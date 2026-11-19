package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Isolation modes recorded with every automated review.
const (
	IsoSeatbelt   = "seatbelt"
	IsoBwrap      = "bwrap"
	IsoLandlock   = "landlock"
	IsoPolicyOnly = "policy-only"
)

// Isolation says how the reviewer process was confined.
type Isolation struct {
	Mode string // one of the Iso* constants
	// Note lists known gaps, for example "network not restricted to port 443".
	Note string
}

// Policy is what the confined reviewer may touch. Everything not listed is
// denied by the OS sandbox where one is available.
type Policy struct {
	Work string   // temporary working directory: read-write
	Read []string // files or directory trees the reviewer may read
	Exec []string // files or directory trees it may execute (a subset of what it may read)
	// MapExec lists trees it may map as code (shared libraries, native add-ons).
	MapExec []string
	// LockDirs are directories where only creating/removing *.lock entries is allowed.
	LockDirs []string
	// Write lists extra files or trees the reviewer may write (the harness's own databases and logs).
	Write []string
	Home  string
}

// pathSeen tracks de-duplicated existing paths.
type pathSet struct {
	seen map[string]bool
	list []string
}

func (s *pathSet) add(p string) {
	if p == "" || !filepath.IsAbs(p) {
		return
	}
	p = filepath.Clean(p)
	if _, err := os.Lstat(p); err != nil {
		return
	}
	if s.seen == nil {
		s.seen = map[string]bool{}
	}
	if !s.seen[p] {
		s.seen[p] = true
		s.list = append(s.list, p)
	}
	// also the real path, because kernels match resolved paths
	if r, err := filepath.EvalSymlinks(p); err == nil && r != p && !s.seen[r] {
		s.seen[r] = true
		s.list = append(s.list, r)
	}
}

// tooBroad reports roots that must never be granted as a whole just because a
// binary lives under them.
func tooBroad(p string, home string) bool {
	switch filepath.Clean(p) {
	case "/", "/usr", "/usr/local", "/opt", "/opt/homebrew", "/bin", "/sbin", "/usr/bin", "/usr/sbin",
		"/usr/local/bin", "/opt/homebrew/bin", "/home", "/Users", "/var", "/etc", "/tmp", "/private", "/Library":
		return true
	}
	return home != "" && filepath.Clean(p) == filepath.Clean(home)
}

// installRoot is the directory tree that holds a resolved harness binary: its
// own directory, or the parent when that directory is named bin.
func installRoot(resolved, home string) string {
	dir := filepath.Dir(resolved)
	if filepath.Base(dir) == "bin" {
		if up := filepath.Dir(dir); !tooBroad(up, home) {
			return up
		}
	}
	if tooBroad(dir, home) {
		return resolved // a single file
	}
	return dir
}

// scriptDeps returns programs a script starts: its shebang interpreter (and
// the program named after env) and absolute paths it quotes, such as the
// target of a shim's `exec "/opt/.../bin/tool"`.
func scriptDeps(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	buf := make([]byte, 2048)
	n, _ := f.Read(buf)
	text := string(buf[:n])
	if !strings.HasPrefix(text, "#!") {
		return nil
	}
	line, rest, _ := strings.Cut(text, "\n")
	fields := strings.Fields(strings.TrimPrefix(line, "#!"))
	var out []string
	if len(fields) > 0 {
		out = append(out, fields[0])
		if filepath.Base(fields[0]) == "env" {
			for _, a := range fields[1:] {
				if strings.HasPrefix(a, "-") || strings.Contains(a, "=") {
					continue
				}
				if p, err := exec.LookPath(a); err == nil {
					out = append(out, p)
				}
				break
			}
		}
	}
	for _, q := range quotedAbs.FindAllStringSubmatch(rest, 8) {
		if fi, err := os.Stat(q[1]); err == nil && fi.Mode().IsRegular() {
			out = append(out, q[1])
		}
	}
	return out
}

var quotedAbs = regexp.MustCompile(`["']((?:/[A-Za-z0-9._+@-]+)+)["']`)

// BuildPolicy computes the allowlist for one reviewer run. bin is the path
// found on PATH; home is the reviewer's HOME.
func BuildPolicy(bin string, t Template, home, work string) Policy {
	var read, ex, mapx pathSet
	seen := map[string]bool{}
	var addProgram func(p string, depth int)
	addProgram = func(p string, depth int) {
		if seen[p] || depth > 4 {
			return
		}
		seen[p] = true
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			resolved = p
		}
		for _, b := range []string{p, resolved} {
			ex.add(b)
			read.add(b)
		}
		root := installRoot(resolved, home)
		ex.add(root)
		read.add(root)
		mapx.add(root)
		for _, d := range append(scriptDeps(p), scriptDeps(resolved)...) {
			addProgram(d, depth+1)
		}
	}
	addProgram(bin, 0)
	if home != "" {
		for _, rel := range t.ConfigPaths {
			read.add(filepath.Join(home, rel))
			mapx.add(filepath.Join(home, rel))
		}
	}
	var locks, write pathSet
	if home != "" {
		for _, rel := range t.WritePaths {
			m, _ := filepath.Glob(filepath.Join(home, rel))
			for _, x := range m {
				write.add(x)
				read.add(x)
			}
		}
		for _, rel := range t.LockDirs {
			locks.add(filepath.Join(home, rel))
		}
	}
	return Policy{Work: work, Read: read.list, Exec: ex.list, MapExec: mapx.list, LockDirs: locks.list, Write: write.list, Home: home}
}

// preferLandlock makes the Linux planSandbox use Landlock even when bwrap works (tests).
var preferLandlock bool
