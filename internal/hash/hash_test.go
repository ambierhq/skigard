package hash

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func write(t *testing.T, root, rel, content string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	os.Chmod(p, mode)
}

// Test vector: pins hash version 1 so the scheme cannot change silently.
func TestVector(t *testing.T) {
	d := t.TempDir()
	write(t, d, "SKILL.md", "hello\n", 0o644)
	write(t, d, "scripts/run.sh", "echo hi\n", 0o755)
	got, err := Dir(d)
	if err != nil {
		t.Fatal(err)
	}
	want := "sha256:b1a18ee21c6d18ec9d1472a89236ace9501b5c187a48bd24f643988b611e90c9"
	if runtime.GOOS == "windows" {
		// Windows files carry no execute bit, so run.sh is hashed with mode "-".
		want = "sha256:8b96a0fa86430a7ce70cde994a67d8d24c43c467a5fb5e5de5bf3b962697701e"
	}
	if want != got {
		t.Fatalf("hash vector changed: got %s want %s", got, want)
	}
	again, _ := Dir(d)
	if again != got {
		t.Fatal("hash not deterministic")
	}
}

func TestSensitiveToContentModeAndPath(t *testing.T) {
	mk := func(content string, mode os.FileMode, name string) string {
		d := t.TempDir()
		write(t, d, name, content, mode)
		h, err := Dir(d)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	base := mk("a", 0o644, "f")
	variants := map[string]string{
		"content": mk("b", 0o644, "f"),
		"path":    mk("a", 0o644, "g"),
	}
	if runtime.GOOS != "windows" { // Windows files have no execute bit
		variants["mode"] = mk("a", 0o755, "f")
	}
	for name, h := range variants {
		if h == base {
			t.Errorf("hash did not change with %s", name)
		}
	}
}

func TestOrderIndependentOfCreation(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	write(t, a, "a-b", "1", 0o644)
	write(t, a, "a/b", "2", 0o644)
	write(t, b, "a/b", "2", 0o644)
	write(t, b, "a-b", "1", 0o644)
	ha, _ := Dir(a)
	hb, _ := Dir(b)
	if ha != hb {
		t.Fatal("order matters")
	}
}

func TestRejectsSymlink(t *testing.T) {
	d := t.TempDir()
	write(t, d, "f", "x", 0o644)
	if err := os.Symlink("f", filepath.Join(d, "l")); err != nil {
		t.Skip(err)
	}
	if _, err := Dir(d); err == nil {
		t.Fatal("expected symlink error")
	}
}
