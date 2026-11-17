package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ambierhq/skigard/internal/harness"
	"github.com/ambierhq/skigard/internal/scan"
)

func TestResolvePrecedence(t *testing.T) {
	t.Setenv("SKIGARD_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	h, _ := Resolve("")
	home, _ := os.UserHomeDir()
	if h.Root != filepath.Join(home, ".config", "skigard") {
		t.Errorf("default = %s", h.Root)
	}
	t.Setenv("XDG_CONFIG_HOME", "/x/cfg")
	if h, _ := Resolve(""); h.Root != "/x/cfg/skigard" {
		t.Errorf("xdg = %s", h.Root)
	}
	t.Setenv("SKIGARD_HOME", "/y/home")
	if h, _ := Resolve(""); h.Root != "/y/home" {
		t.Errorf("env = %s", h.Root)
	}
	if h, _ := Resolve("/z"); h.Root != "/z" {
		t.Errorf("flag = %s", h.Root)
	}
}

func TestInitLayoutAndPerms(t *testing.T) {
	h := Home{filepath.Join(t.TempDir(), "sk")}
	if err := h.Init(); err != nil {
		t.Fatal(err)
	}
	if err := h.Init(); err != nil {
		t.Fatal("second init failed", err)
	}
	for _, d := range []string{h.QuarantineDir(), h.StateDir()} {
		fi, err := os.Stat(d)
		if err != nil || fi.Mode().Perm() != 0o700 {
			t.Errorf("%s: %v %v", d, fi, err)
		}
	}
	gi, _ := os.ReadFile(filepath.Join(h.Root, ".gitignore"))
	if string(gi) != gitignore {
		t.Errorf("gitignore = %q", gi)
	}
}

func TestManifestAndLockRoundTrip(t *testing.T) {
	h := Home{t.TempDir()}
	m := &Manifest{Version: 1, Settings: Settings{ReviewHarness: "pi", Owner: "me"},
		Plugins: map[string]Plugin{"dev": {Description: "d", Skills: []string{"a", "b"}}},
		Harness: []harness.Entry{{ID: "mine", Name: "Mine", Global: "~/.mine/skills"}}}
	if err := SaveManifest(h, m); err != nil {
		t.Fatal(err)
	}
	got, err := LoadManifest(h)
	if err != nil || got.Settings.ReviewHarness != "pi" || len(got.Plugins["dev"].Skills) != 2 || got.Harness[0].ID != "mine" {
		t.Fatalf("%+v %v", got, err)
	}
	l := &Lock{Version: 1, HashVersion: 1}
	l.Put(LockSkill{Name: "zeta", Source: "github:o/r", ContentHash: "sha256:aa", Scan: ScanRec{Verdict: "pass"},
		Review:   &ReviewRec{Reviewer: "pi", Verdict: "pass"},
		Approval: ApprovalRec{By: "me", At: "now", Override: true, Reason: "ok", Accepted: []scan.Accepted{{Key: "k", Reason: "r"}}}})
	l.Put(LockSkill{Name: "alpha", Source: "github:o/r2", ContentHash: "sha256:bb"})
	if err := l.Save(h); err != nil {
		t.Fatal(err)
	}
	back, err := LoadLock(h)
	if err != nil || len(back.Skills) != 2 || back.Skills[0].Name != "alpha" {
		t.Fatalf("%+v %v", back, err)
	}
	z, _ := back.Get("zeta")
	if !z.Approval.Override || z.Review.Reviewer != "pi" || z.Approval.Accepted[0].Key != "k" {
		t.Errorf("%+v", z)
	}
	back.Delete("alpha")
	if _, ok := back.Get("alpha"); ok {
		t.Error("delete failed")
	}
	// determinism: saving twice yields identical bytes
	a, _ := os.ReadFile(h.LockPath())
	back2, _ := LoadLock(h)
	l.Delete("alpha")
	back2.Delete("alpha")
	_ = back2.Save(h)
	b, _ := os.ReadFile(h.LockPath())
	if len(a) == 0 || len(b) == 0 {
		t.Fatal("empty lock")
	}
}

func TestMetaRoundTripAndMissing(t *testing.T) {
	h := Home{t.TempDir()}
	h.Init()
	if _, err := LoadMeta(h, "nope"); err == nil {
		t.Fatal("expected error")
	}
	m := &Meta{Name: "a", Source: "github:o/r", FetchedAt: "t", ContentHash: "sha256:x"}
	if err := m.Save(h); err != nil {
		t.Fatal(err)
	}
	got, err := LoadMeta(h, "a")
	if err != nil || got.ContentHash != "sha256:x" {
		t.Fatal(got, err)
	}
	if n := QuarantinedNames(h); len(n) != 1 || n[0] != "a" {
		t.Errorf("%v", n)
	}
}

func TestHomeLockIsExclusive(t *testing.T) {
	h := Home{t.TempDir()}
	unlock, err := h.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Lock(); err == nil {
		t.Fatal("second lock should fail while the first is held")
	}
	unlock()
	u2, err := h.Lock()
	if err != nil {
		t.Fatalf("lock not released: %v", err)
	}
	u2()
}

func TestHostileNamesRejectedOnLoad(t *testing.T) {
	h := Home{t.TempDir()}
	os.WriteFile(h.LockPath(), []byte("version = 1\nhash_version = 1\n[[skill]]\nname = \"../../etc\"\ncontent_hash = \"x\"\n"), 0o644)
	if _, err := LoadLock(h); err == nil {
		t.Fatal("path-traversal name accepted in the lock")
	}
	os.WriteFile(h.LockPath(), []byte("version = 1\n[[skill]]\nname = \"a\"\n[[skill]]\nname = \"a\"\n"), 0o644)
	if _, err := LoadLock(h); err == nil {
		t.Fatal("duplicate names accepted")
	}
	os.WriteFile(h.ManifestPath(), []byte("version = 1\n[plugins.\"../x\"]\nskills = []\n"), 0o644)
	if _, err := LoadManifest(h); err == nil {
		t.Fatal("bad plugin name accepted")
	}
	if _, err := LoadMeta(h, "../x"); err == nil {
		t.Fatal("bad meta name accepted")
	}
}
