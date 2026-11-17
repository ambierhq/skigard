package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, dir, rel, body string, mode os.FileMode) {
	p := filepath.Join(dir, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(body), mode)
	os.Chmod(p, mode)
}

func TestDirs(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	put(t, a, "SKILL.md", "one\ntwo\nthree\n", 0o644)
	put(t, b, "SKILL.md", "one\nTWO\nthree\n", 0o644)
	put(t, a, "old.txt", "x", 0o644)
	put(t, b, "new.sh", "echo \x1b[31mhi", 0o755)
	got, err := Dirs(a, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--- approved/SKILL.md", "- two", "+ TWO", "--- removed: old.txt", "+++ added: new.sh"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\x1b") {
		t.Error("terminal escape leaked")
	}
}

func TestNewSkillAndIdentical(t *testing.T) {
	b := t.TempDir()
	put(t, b, "SKILL.md", "x\n", 0o644)
	got, _ := Dirs(filepath.Join(t.TempDir(), "missing"), b)
	if !strings.Contains(got, "+++ added: SKILL.md") {
		t.Errorf("%s", got)
	}
	if got, _ := Dirs(b, b); got != "" {
		t.Errorf("identical dirs produced %q", got)
	}
}

func TestModeChange(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	put(t, a, "run.sh", "x", 0o644)
	put(t, b, "run.sh", "x", 0o755)
	got, _ := Dirs(a, b)
	if !strings.Contains(got, "mode changed") {
		t.Errorf("%s", got)
	}
}
