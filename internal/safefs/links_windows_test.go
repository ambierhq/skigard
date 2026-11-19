//go:build windows

package safefs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A junction needs no privilege, so this runs on every Windows runner.
func TestCopyTreeRejectsJunction(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(src, "junc"), outside).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v: %s", err, out)
	}
	dst := filepath.Join(t.TempDir(), "out")
	if _, err := CopyTree(src, dst, DefaultLimits()); err == nil {
		t.Fatal("junction in source was followed or copied")
	}
	if _, err := os.Stat(filepath.Join(dst, "junc", "secret.txt")); err == nil {
		t.Fatal("junction target leaked into the copy")
	}
}
