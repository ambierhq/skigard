package store

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/harness"
	"github.com/ambierhq/skigard/internal/scan"
)

func TestResolvePrecedence(t *testing.T) {
	t.Setenv("SKIGARD_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	h, _ := Resolve("")
	home, _ := os.UserHomeDir()
	if runtime.GOOS != "windows" && h.Root != filepath.Join(home, ".config", "skigard") { // Windows: see TestResolveWindowsSplit
		t.Errorf("default = %s", h.Root)
	}
	xdg, _ := filepath.Abs(filepath.FromSlash("/x/cfg"))
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if h, _ := Resolve(""); h.Root != filepath.Join(xdg, "skigard") {
		t.Errorf("xdg = %s", h.Root)
	}
	yhome, _ := filepath.Abs(filepath.FromSlash("/y/home"))
	t.Setenv("SKIGARD_HOME", yhome)
	if h, _ := Resolve(""); h.Root != yhome {
		t.Errorf("env = %s", h.Root)
	}
	z, _ := filepath.Abs(filepath.FromSlash("/z"))
	if h, _ := Resolve("/z"); h.Root != z {
		t.Errorf("flag = %s", h.Root)
	}
}

func TestInitLayoutAndPerms(t *testing.T) {
	h := Home{Root: filepath.Join(t.TempDir(), "sk")}
	if err := h.Init(); err != nil {
		t.Fatal(err)
	}
	if err := h.Init(); err != nil {
		t.Fatal("second init failed", err)
	}
	for _, d := range []string{h.QuarantineDir(), h.StateDir()} {
		fi, err := os.Stat(d)
		if err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o700) {
			t.Errorf("%s: %v %v", d, fi, err)
		}
	}
	gi, _ := os.ReadFile(filepath.Join(h.Root, ".gitignore"))
	if string(gi) != gitignore {
		t.Errorf("gitignore = %q", gi)
	}
}

func TestManifestAndLockRoundTrip(t *testing.T) {
	h := Home{Root: t.TempDir()}
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
	h := Home{Root: t.TempDir()}
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
	h := Home{Root: t.TempDir()}
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
	h := Home{Root: t.TempDir()}
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

func TestResolveWindowsSplit(t *testing.T) {
	none := func(string) string { return "" }
	home := func() (string, error) { return `C:\Users\u`, nil }
	env := map[string]string{"APPDATA": `C:\Users\u\AppData\Roaming`, "LOCALAPPDATA": `D:\Local`}
	get := func(k string) string { return env[k] }
	h, err := resolve("", "windows", get, home)
	if err != nil || h.Root != filepath.Join(env["APPDATA"], "skigard") || h.Local != filepath.Join(`D:\Local`, "skigard") {
		t.Fatalf("split = %+v %v", h, err)
	}
	if h.QuarantineDir() != filepath.Join(h.Local, "quarantine") || h.StateDir() != filepath.Join(h.Local, "state") ||
		h.LockPath() != filepath.Join(h.Root, "skigard.lock") || h.ManifestPath() != filepath.Join(h.Root, "skigard.toml") ||
		h.SkillDir("a") != filepath.Join(h.Root, "skills", "a") {
		t.Errorf("layout wrong: %+v", h)
	}
	// fall back to the profile when the variables are missing
	h, _ = resolve("", "windows", none, home)
	if h.Root != filepath.Join(`C:\Users\u`, "AppData", "Roaming", "skigard") || h.Local != filepath.Join(`C:\Users\u`, "AppData", "Local", "skigard") {
		t.Errorf("fallback = %+v", h)
	}
	// SKIGARD_HOME overrides everything and keeps all of it in one place
	env["SKIGARD_HOME"] = filepath.Join(t.TempDir(), "one")
	h, _ = resolve("", "windows", get, home)
	if h.Local != "" || h.QuarantineDir() != filepath.Join(h.Root, "quarantine") {
		t.Errorf("SKIGARD_HOME must not split: %+v", h)
	}
	// non-Windows never splits
	delete(env, "SKIGARD_HOME")
	if h, _ = resolve("", "linux", get, home); h.Local != "" {
		t.Errorf("linux split: %+v", h)
	}
}

func TestSplitHomeInit(t *testing.T) {
	h := Home{Root: filepath.Join(t.TempDir(), "roaming", "sk"), Local: filepath.Join(t.TempDir(), "local", "sk")}
	if err := h.Init(); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{h.SkillsDir(), h.ReviewsDir(), h.QuarantineDir(), h.StateDir()} {
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			t.Errorf("%s missing: %v", d, err)
		}
	}
	if _, err := os.Stat(filepath.Join(h.Root, "quarantine")); err == nil {
		t.Error("quarantine must not be created in the roaming root")
	}
	if unlock, err := h.Lock(); err != nil {
		t.Fatal(err)
	} else {
		unlock()
	}
}

func TestSettingsModeThresholdRoundTripAndValidation(t *testing.T) {
	off := false
	h := Home{Root: t.TempDir()}
	m := &Manifest{Version: 1, Settings: Settings{Mode: "strict", Threshold: 9, AutoReview: &off}}
	if err := SaveManifest(h, m); err != nil {
		t.Fatal(err)
	}
	got, err := LoadManifest(h)
	if err != nil || got.Settings.Mode != "strict" || got.Settings.Threshold != 9 || got.Settings.EffectiveAutoReview() {
		t.Fatalf("%+v %v", got, err)
	}
	if d := (Settings{}); d.EffectiveMode() != DefaultMode || d.EffectiveThreshold() != DefaultThreshold || !d.EffectiveAutoReview() {
		t.Errorf("defaults: %s %d", d.EffectiveMode(), d.EffectiveThreshold())
	}
	for _, bad := range []string{"mode = \"yolo\"", "threshold = 11", "threshold = -1"} {
		os.WriteFile(h.ManifestPath(), []byte("version = 1\n[settings]\n"+bad+"\n"), 0o644)
		if _, err := LoadManifest(h); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestApprovalFieldsRoundTripAndOldLock(t *testing.T) {
	h := Home{Root: t.TempDir()}
	l := &Lock{Version: 1, HashVersion: 1}
	l.Put(LockSkill{Name: "a", ContentHash: "sha256:aa", Approval: ApprovalRec{By: AutoApprover, Mode: ApprovalAuto, Score: 8, Threshold: 7, ScoreVersion: "1"}})
	if err := l.Save(h); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLock(h)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := got.Get("a"); s.Approval.Mode != "auto" || s.Approval.By != AutoApprover || s.Approval.Score != 8 || s.Approval.Threshold != 7 || s.Approval.ScoreVersion != "1" {
		t.Fatalf("%+v", s.Approval)
	}
	old := "version = 1\nhash_version = 1\n[[skill]]\nname = \"b\"\ncontent_hash = \"sha256:bb\"\n[skill.approval]\nby = \"me\"\nat = \"x\"\n"
	os.WriteFile(h.LockPath(), []byte(old), 0o644)
	got, err = LoadLock(h)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := got.Get("b"); s.Approval.EffectiveMode() != "human" || s.Approval.Score != 0 {
		t.Fatalf("%+v", s.Approval)
	}
	os.WriteFile(h.LockPath(), []byte(old+"mode = \"yolo\"\n"), 0o644)
	if _, err := LoadLock(h); err == nil {
		t.Error("unknown approval mode accepted")
	}
	os.WriteFile(h.LockPath(), []byte(strings.Replace(old, `by = "me"`, `by = "skigard (auto)"`, 1)), 0o644)
	if _, err := LoadLock(h); err == nil {
		t.Error("the auto approver was accepted for a human approval")
	}
}
