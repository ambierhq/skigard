//go:build windows && e2e

package e2e

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestPublishedSmoke drives a PUBLISHED skigard.exe (not one built from source) through the quick start.
// It is used by .github/workflows/windows-smoke.yml via scripts/windows-smoke.ps1, and skips unless
// SKIGARD_SMOKE_BIN is set:
//
//	SKIGARD_SMOKE_BIN      path to the released skigard.exe
//	SKIGARD_SMOKE_VERSION  expected version, e.g. 0.1.2 (optional)
//	SKIGARD_SMOKE_SKILL    path to a directory holding a harmless skill (default ../../examples/hello-skill)
//	SKIGARD_SMOKE_REMOTE   "1" also fetches github:ambierhq/skigard/examples/hello-skill (needs GITHUB_TOKEN)
func TestPublishedSmoke(t *testing.T) {
	bin := os.Getenv("SKIGARD_SMOKE_BIN")
	if bin == "" {
		t.Skip("SKIGARD_SMOKE_BIN not set")
	}
	skill := os.Getenv("SKIGARD_SMOKE_SKILL")
	if skill == "" {
		skill = filepath.Join("..", "..", "examples", "hello-skill")
	}
	skill, err := filepath.Abs(skill)
	if err != nil {
		t.Fatal(err)
	}
	// Short, space-free work directory: it is also passed through cmd.exe below.
	work, err := os.MkdirTemp("", "sksmoke")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(work) })
	appdata := filepath.Join(work, "appdata")
	local := filepath.Join(work, "local")
	user := filepath.Join(work, "user")
	for _, d := range []string{appdata, local, user} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// No SKIGARD_HOME: this exercises the documented default layout, with APPDATA and LOCALAPPDATA redirected.
	env := cleanEnv(map[string]string{"APPDATA": appdata, "LOCALAPPDATA": local, "USERPROFILE": user, "HOME": user, "SKIGARD_HOME": "", "XDG_CONFIG_HOME": ""})
	env = dropEmpty(env, "SKIGARD_HOME", "XDG_CONFIG_HOME")
	r := &rig{t: t, bin: bin, work: work, env: env}

	t.Run("version", func(t *testing.T) {
		out := r.mustRun("version")
		t.Log(strings.TrimSpace(out))
		if want := os.Getenv("SKIGARD_SMOKE_VERSION"); want != "" && !regexp.MustCompile(`^skigard v?`+regexp.QuoteMeta(strings.TrimPrefix(want, "v"))+`(\s|$)`).MatchString(out) {
			t.Fatalf("version output %q does not match %s", out, want)
		}
	})

	home := filepath.Join(appdata, "skigard")
	state := filepath.Join(local, "skigard")

	t.Run("add local path lands in the split layout", func(t *testing.T) {
		r.mustRun("init")
		out := r.mustRun("add", skill)
		if !strings.Contains(out, "hello-skill") {
			t.Fatalf("add output: %s", out)
		}
		mustExist(t, filepath.Join(state, "quarantine", "hello-skill", "SKILL.md"))
		mustNotExist(t, filepath.Join(home, "quarantine"))
		mustExist(t, filepath.Join(home, "skigard.toml"))
		t.Log(r.mustRun("status"))
	})

	var localHash string
	t.Run("scan passes", func(t *testing.T) {
		out := r.mustRun("scan", "hello-skill")
		if !strings.Contains(out, "PASS") {
			t.Fatalf("scan: %s", out)
		}
		localHash = regexp.MustCompile(`sha256:[0-9a-f]{64}`).FindString(out)
		if localHash == "" {
			t.Fatalf("no content hash in: %s", out)
		}
	})

	t.Run("approve is refused when CLAUDECODE=1", func(t *testing.T) {
		e := append(append([]string{}, r.env...), "CLAUDECODE=1")
		code, out := r.run(e, "approve", "hello-skill")
		if code == 0 || !strings.Contains(out, "agent") {
			t.Fatalf("no console: exit %d: %s", code, out)
		}
		// With a real console too, and the correct answer waiting on stdin: still refused.
		s, err := startInPTY(r.bin, []string{"approve", "hello-skill"}, e)
		if err != nil {
			t.Fatal(err)
		}
		defer s.close()
		_, _ = s.in.Write([]byte("hello-skill\r"))
		c, err := s.wait(30 * time.Second)
		// Exit 1 is the policy refusal. The message goes to stderr, which is not the pseudo console, so it is not in s.output().
		if err != nil || c != 1 {
			t.Fatalf("pty: exit %d (want 1) err %v output %q", c, err, s.output())
		}
		assertNotApproved(t, r)
	})

	t.Run("approve is refused under an agent-named ancestor", func(t *testing.T) {
		// Ancestor-process detection shipped in v0.1.2 (#38). Older releases would approve here, so do not run it.
		if !atLeast(os.Getenv("SKIGARD_SMOKE_VERSION"), 0, 1, 2) {
			t.Skipf("ancestor detection arrived in 0.1.2; testing %s", os.Getenv("SKIGARD_SMOKE_VERSION"))
		}
		cmd := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
		fake := filepath.Join(work, "claude.exe")
		copyFile(t, cmd, fake)
		line := r.bin + " approve hello-skill"
		s, err := startInPTY(fake, []string{"/c", line}, r.env)
		if err != nil {
			t.Fatal(err)
		}
		defer s.close()
		_, _ = s.in.Write([]byte("hello-skill\r"))
		c, err := s.wait(30 * time.Second)
		if err != nil || c != 1 {
			t.Fatalf("exit %d (want 1, policy refusal) err %v output %q", c, err, s.output())
		}
		assertNotApproved(t, r)
	})

	t.Run("conpty: wrong answer cancels (exit 3), right answer approves", func(t *testing.T) {
		if code, out := r.approveInPTY("hello-skill", "nope"); code != 3 {
			t.Fatalf("wrong answer: exit %d, want 3; output %q", code, out)
		}
		assertNotApproved(t, r)
		if code, out := r.approveInPTY("hello-skill", "hello-skill"); code != 0 {
			t.Fatalf("right answer: exit %d, want 0; output %q", code, out)
		}
		if out := r.mustRun("list"); !strings.Contains(out, "approved") {
			t.Fatalf("not approved:\n%s", out)
		}
		mustExist(t, filepath.Join(home, "skigard.lock"))
		mustExist(t, filepath.Join(home, "skills", "hello-skill", "SKILL.md"))
		mustNotExist(t, filepath.Join(state, "skigard.lock"))
		mustExist(t, filepath.Join(state, "state"))
		t.Log(r.mustRun("status"))
	})

	t.Run("verify --strict", func(t *testing.T) {
		t.Log(r.mustRun("verify", "--strict"))
	})

	t.Run("install into a temp skills dir", func(t *testing.T) {
		dir := filepath.Join(work, "agent", "skills")
		r.mustRun("harness", "add", "--id", "smoke", "--name", "Smoke", "--global", dir)
		t.Log(r.mustRun("install", "hello-skill", "--to", "smoke"))
		mustExist(t, filepath.Join(dir, "hello-skill", "SKILL.md"))
		t.Log(r.mustRun("verify", "--strict"))
	})

	t.Run("remote add has the same content hash", func(t *testing.T) {
		if os.Getenv("SKIGARD_SMOKE_REMOTE") != "1" {
			t.Skip("SKIGARD_SMOKE_REMOTE not 1")
		}
		if os.Getenv("GITHUB_TOKEN") == "" {
			t.Fatal("GITHUB_TOKEN is required for the remote add")
		}
		e := append(append([]string{}, r.env...), "GITHUB_TOKEN="+os.Getenv("GITHUB_TOKEN"))
		rhome := filepath.Join(work, "remote-home")
		e = append(e, "SKIGARD_HOME="+rhome)
		code, out := r.run(e, "add", "github:ambierhq/skigard/examples/hello-skill")
		if code != 0 {
			t.Fatalf("remote add exit %d: %s", code, out)
		}
		code, out = r.run(e, "scan", "hello-skill")
		if code != 0 {
			t.Fatalf("remote scan exit %d: %s", code, out)
		}
		if got := regexp.MustCompile(`sha256:[0-9a-f]{64}`).FindString(out); got != localHash {
			t.Fatalf("remote hash %q != local hash %q (the checked-out example may differ from main: informational if so)", got, localHash)
		}
	})
}

// atLeast reports whether version "X.Y.Z[-suffix]" is >= the given one. An empty or unparsable version counts as new.
func atLeast(version string, major, minor, patch int) bool {
	m := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`).FindStringSubmatch(version)
	if m == nil {
		return true
	}
	have := [3]int{}
	for i := range have {
		have[i], _ = strconv.Atoi(m[i+1])
	}
	want := [3]int{major, minor, patch}
	for i := range have {
		if have[i] != want[i] {
			return have[i] > want[i]
		}
	}
	return true
}

func dropEmpty(env []string, keys ...string) []string {
	var out []string
outer:
	for _, kv := range env {
		for _, k := range keys {
			if strings.EqualFold(kv, k+"=") {
				continue outer
			}
		}
		out = append(out, kv)
	}
	return out
}

func mustExist(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected %s: %v", p, err)
	}
}

func mustNotExist(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Stat(p); err == nil {
		t.Fatalf("did not expect %s", p)
	}
}

func assertNotApproved(t *testing.T, r *rig) {
	t.Helper()
	if out := r.mustRun("list"); strings.Contains(out, "approved") {
		t.Fatalf("skill was approved:\n%s", out)
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	in, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}
