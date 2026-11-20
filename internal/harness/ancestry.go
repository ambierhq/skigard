package harness

import (
	"path/filepath"
	"strings"
)

// Best-effort detection of an agent harness above this process.
//
// An agent can allocate its own pty and type the answer, and it can strip
// environment variables for its child. It cannot, without extra work, change
// who its ancestors are. So approve also walks the parent chain and refuses if
// any ancestor is a known agent binary or carries an agent environment marker.
//
// This is not a security boundary: a process that re-parents itself (double
// fork, daemonize, systemd-run, launchctl) leaves the chain. It raises the cost
// of the one-line bypass, nothing more.

// MaxAncestry is how many parents are examined.
const MaxAncestry = 20

// Proc is one process as seen by the ancestor walk.
type Proc struct {
	PID, PPID int
	Name      string            // image name (comm or exe base); may be truncated or generic
	Argv      []string          // nil when unreadable
	Env       map[string]string // nil when unreadable (other user, Windows, hardened /proc)
}

// ProcTable looks up a process. Any error ends the walk and fails open: the
// other checks (environment, tty) still apply.
type ProcTable func(pid int) (Proc, error)

// ancestorSkip lists registry entries whose binary is also an editor or
// terminal that hosts a human shell. Matching them would block humans.
var ancestorSkip = map[string]bool{"zed": true}

var interpreters = map[string]bool{"node": true, "nodejs": true, "bun": true, "deno": true,
	"python": true, "python3": true, "ruby": true, "tsx": true, "npx": true}

// normExe lowercases and strips the extensions that wrap an agent binary.
func normExe(s string) string {
	s = strings.ToLower(filepath.Base(strings.ReplaceAll(s, `\`, "/")))
	for _, ext := range []string{".exe", ".cmd", ".bat", ".ps1", ".js", ".mjs", ".cjs", ".py"} {
		s = strings.TrimSuffix(s, ext)
	}
	return s
}

// exeNames returns the names a process may be known by: its image name, argv[0]
// and, for an interpreter, the script it runs (`node /x/bin/pi`).
func exeNames(p Proc) []string {
	var out []string
	if p.Name != "" {
		out = append(out, normExe(p.Name))
	}
	if len(p.Argv) > 0 {
		a0 := normExe(p.Argv[0])
		out = append(out, a0)
		if strings.HasPrefix(a0, "python") || interpreters[a0] {
			for _, a := range p.Argv[1:] {
				if a != "" && !strings.HasPrefix(a, "-") {
					out = append(out, normExe(a))
					break
				}
			}
		}
	}
	return out
}

// AgentEnvMarkers maps every environment variable that signals an agent to the
// harness that sets it.
func AgentEnvMarkers(reg []Entry) map[string]string {
	m := map[string]string{}
	for _, n := range GenericAgentEnv {
		m[n] = "an agent"
	}
	for _, e := range reg {
		for _, n := range e.Env {
			m[n] = e.Name
		}
	}
	return m
}

// AgentAncestor walks up from pid (pid itself is examined too) and returns the name of the first agent harness found.
func AgentAncestor(reg []Entry, table ProcTable, pid, maxDepth int) (string, bool) {
	bins := map[string]string{}
	for _, e := range reg {
		if ancestorSkip[e.ID] {
			continue
		}
		for _, b := range e.Bins {
			bins[normExe(b)] = e.Name
		}
	}
	markers := AgentEnvMarkers(reg)
	seen := map[int]bool{}
	for depth := 0; depth < maxDepth && pid > 1 && !seen[pid]; depth++ {
		seen[pid] = true
		p, err := table(pid)
		if err != nil {
			return "", false
		}
		for _, n := range exeNames(p) {
			if h, ok := bins[n]; ok {
				return h, true
			}
		}
		for k, v := range p.Env {
			if h, ok := markers[k]; ok && v != "" {
				return h, true
			}
		}
		pid = p.PPID
	}
	return "", false
}
