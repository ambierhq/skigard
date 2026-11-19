package safefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyTreeRejectsSymlink(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(src, "SKILL.md"), filepath.Join(src, "link")); err != nil {
		t.Skipf("cannot create symlinks here (Windows needs privilege): %v", err)
	}
	if _, err := CopyTree(src, filepath.Join(t.TempDir(), "out"), DefaultLimits()); err == nil {
		t.Fatal("symlink in source was copied")
	}
}
