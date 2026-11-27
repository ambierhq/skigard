package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

func writableApproved(t *testing.T) (*App, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are synthetic on Windows")
	}
	a, _ := newApp(t)
	src := t.TempDir()
	writeSkill(t, src, "alpha", map[string]string{"run.sh": "#!/bin/sh\n"})
	os.Chmod(filepath.Join(src, "alpha", "run.sh"), 0o755)
	approveLocal(t, a, src, "alpha")
	dir := a.Home.SkillDir("alpha")
	t.Cleanup(func() { util.MakeWritable(dir) })
	return a, dir
}

func writableWarns(t *testing.T, a *App) bool {
	t.Helper()
	ps, err := a.Verify()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		if strings.Contains(p.Msg, "skills/alpha is writable") {
			if p.Level != "warn" {
				t.Fatalf("level %s", p.Level)
			}
			return true
		}
	}
	if HasErrors(ps) {
		t.Fatalf("%+v", ps)
	}
	return false
}

func TestVerifyWarnsWritableSkill(t *testing.T) {
	a, dir := writableApproved(t)
	if writableWarns(t, a) {
		t.Fatal("read-only skill warned")
	}
	util.MakeWritable(dir)
	if !writableWarns(t, a) {
		t.Fatal("writable skill did not warn")
	}
}

func TestProtect(t *testing.T) {
	a, dir := writableApproved(t)
	util.MakeWritable(dir)
	outside := filepath.Join(t.TempDir(), "target")
	os.WriteFile(outside, []byte("x"), 0o666)
	os.Chmod(outside, 0o666)
	// a symlink inside the tree must be left alone (this changes the hash, so do it after Check below)
	ls, _ := store.LoadLock(a.Home)
	sk, _ := ls.Get("alpha")
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Skip(err)
	}
	res, err := a.Protect(nil)
	if err != nil || len(res) != 1 || !res[0].Changed {
		t.Fatalf("%+v %v", res, err)
	}
	if fi, _ := os.Stat(outside); fi.Mode().Perm() != 0o666 {
		t.Fatalf("symlink target chmod'ed: %v", fi.Mode())
	}
	os.Remove(filepath.Join(dir, "link")) // read-only dir: remove needs write
	if w, _ := util.TreeWritable(dir); w {
		t.Fatal("still writable")
	}
	util.MakeWritable(dir)
	os.Remove(filepath.Join(dir, "link"))
	util.MakeWritable(dir)
	if _, err := a.Protect([]string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	st, err := hash.Check(dir, sk.Identity())
	if err != nil || !st.ContentOK || !st.ExecOK {
		t.Fatalf("%+v %v", st, err)
	}
	if fi, _ := os.Stat(filepath.Join(dir, "run.sh")); fi.Mode().Perm() != 0o555 {
		t.Fatalf("exec bit: %v", fi.Mode())
	}
	again, err := a.Protect(nil)
	if err != nil || again[0].Changed || again[0].Note != "already read-only" {
		t.Fatalf("%+v %v", again, err)
	}
	if writableWarns(t, a) {
		t.Fatal("still warns")
	}
}

func TestProtectRejectsNames(t *testing.T) {
	a, _ := writableApproved(t)
	for _, n := range []string{"nope", "../x", ""} {
		if _, err := a.Protect([]string{n}); err == nil {
			t.Errorf("accepted %q", n)
		}
	}
}
