package harness

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

type fakeTable map[int]Proc

func (f fakeTable) lookup(pid int) (Proc, error) {
	p, ok := f[pid]
	if !ok {
		return Proc{}, errors.New("unreadable")
	}
	return p, nil
}

// chain builds pid 100 (skigard) <- 101 <- ... from the given parents, each parent of the next.
func chain(ps ...Proc) fakeTable {
	t := fakeTable{}
	for i := range ps {
		ps[i].PID = 100 + i
		if i+1 < len(ps) {
			ps[i].PPID = 101 + i
		} else {
			ps[i].PPID = 1
		}
		t[ps[i].PID] = ps[i]
	}
	return t
}

func pr(name string, argv ...string) Proc { return Proc{Name: name, Argv: argv} }

func withEnv(p Proc, kv ...string) Proc {
	p.Env = map[string]string{}
	for _, e := range kv {
		k, v, _ := strings.Cut(e, "=")
		p.Env[k] = v
	}
	return p
}

func TestAgentAncestor(t *testing.T) {
	reg, err := Registry(nil)
	if err != nil {
		t.Fatal(err)
	}
	skig := pr("skigard", "skigard", "approve", "x")
	deep := []Proc{skig}
	for i := 0; i < 24; i++ {
		deep = append(deep, pr("bash", "bash"))
	}
	deep = append(deep, pr("claude", "claude"))
	unreadableMid := chain(skig, pr("script", "script"), pr("claude", "claude"))
	delete(unreadableMid, 101)

	tests := []struct {
		name  string
		table fakeTable
		want  string // harness name, "" for none
	}{
		{"direct env marker on parent", chain(skig, withEnv(pr("bash", "bash"), "CLAUDECODE=1")), "Claude Code"},
		{"env stripped for child, agent ancestor by binary", chain(skig, pr("env", "env", "-u", "CLAUDECODE", "script"), pr("bash", "bash"), pr("claude", "claude")), "Claude Code"},
		{"env stripped for child, ancestor carries marker", chain(skig, withEnv(pr("python3", "python3", "pty.py"), "PATH=/bin"), withEnv(pr("bash", "bash", "-c", "x"), "PI_CODING_AGENT=1")), "Pi"},
		{"script between agent and skigard", chain(skig, pr("script", "script", "-q", "/dev/null", "skigard"), pr("sh", "sh", "-c", "x"), pr("node", "node", "/usr/lib/node_modules/@x/pi/bin/pi.js")), "Pi"},
		{"expect between agent and skigard", chain(skig, pr("expect", "expect", "-c", "spawn skigard"), pr("codex", "codex")), "Codex"},
		{"python pty helper under cursor-agent", chain(skig, pr("python3", "python3", "pty.py"), pr("cursor-agent", "cursor-agent")), "Cursor"},
		{"windows image name", chain(skig, pr("cmd.exe"), pr("claude.exe")), "Claude Code"},
		{"human in tmux", chain(skig, pr("zsh", "-zsh"), pr("tmux: server", "tmux", "new"), pr("launchd", "/sbin/launchd")), ""},
		{"human over ssh", chain(skig, pr("bash", "-bash"), pr("sshd", "sshd: me@pts/0"), pr("sshd", "sshd")), ""},
		{"human in VS Code terminal", chain(skig, withEnv(pr("zsh", "-zsh"), "TERM_PROGRAM=vscode", "VSCODE_PID=1"), pr("Code Helper", "Code Helper (Plugin)"), pr("Electron", "/Applications/Visual Studio Code.app/Contents/MacOS/Electron")), ""},
		{"human in Cursor terminal pane", chain(skig, withEnv(pr("zsh", "-zsh"), "TERM_PROGRAM=vscode", "CURSOR_TRACE_ID=abc"), pr("Cursor Helper", "Cursor Helper (Plugin)"), pr("Cursor", "/Applications/Cursor.app/Contents/MacOS/Cursor")), ""},
		{"human in Zed terminal", chain(skig, pr("zsh", "-zsh"), pr("zed", "/usr/bin/zed")), ""},
		{"human, unrelated node script", chain(skig, pr("node", "node", "/home/me/build.js")), ""},
		{"agent deeper than depth limit", chain(deep...), ""},
		{"unreadable entry fails open", unreadableMid, ""},
		{"empty-valued marker ignored", chain(skig, withEnv(pr("bash", "bash"), "CLAUDECODE=")), ""},
		{"unreadable env still matches binary", chain(skig, Proc{Name: "bash"}, pr("goose", "goose")), "Goose"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := AgentAncestor(reg, tc.table.lookup, 101, MaxAncestry)
			if tc.want == "" && ok || tc.want != "" && (!ok || got != tc.want) {
				t.Fatalf("got (%q, %v), want %q", got, ok, tc.want)
			}
		})
	}
}

func TestAgentAncestorCycleAndMissing(t *testing.T) {
	reg, _ := Registry(nil)
	cyc := fakeTable{5: {PID: 5, PPID: 6, Name: "a"}, 6: {PID: 6, PPID: 5, Name: "b"}}
	if _, ok := AgentAncestor(reg, cyc.lookup, 5, MaxAncestry); ok {
		t.Fatal("cycle matched")
	}
	if _, ok := AgentAncestor(reg, fakeTable{}.lookup, 5, MaxAncestry); ok {
		t.Fatal("empty table matched")
	}
}

func TestEveryEnvMarkerIsCaught(t *testing.T) {
	reg, _ := Registry(nil)
	for k := range AgentEnvMarkers(reg) {
		tbl := chain(pr("skigard"), withEnv(pr("bash", "bash"), k+"=1"))
		if _, ok := AgentAncestor(reg, tbl.lookup, 101, MaxAncestry); !ok {
			t.Errorf("marker %s not caught", k)
		}
	}
	for _, e := range reg {
		for _, b := range e.Bins {
			if ancestorSkip[e.ID] {
				continue
			}
			tbl := chain(pr("skigard"), pr(b, b))
			if got, ok := AgentAncestor(reg, tbl.lookup, 101, MaxAncestry); !ok || got != e.Name {
				t.Errorf("bin %s: got %q %v want %s", b, got, ok, e.Name)
			}
		}
	}
}

// The real table must at least describe this process and its parent, and
// must never panic on a pid that is gone.
func TestSystemProcTable(t *testing.T) {
	p, err := SystemProcTable(os.Getpid())
	if err != nil {
		t.Skipf("process table unavailable: %v", err)
	}
	if p.PPID != os.Getppid() {
		t.Fatalf("ppid %d, want %d", p.PPID, os.Getppid())
	}
	if _, err := SystemProcTable(1 << 30); err == nil {
		t.Fatal("expected error for missing pid")
	}
	_ = fmt.Sprint(p)
}
