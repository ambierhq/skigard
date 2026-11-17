package harness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ambierhq/skigard/internal/hash"
)

func fakeEnv(home string, bins map[string]string, env map[string]string) Env {
	return Env{
		Home:   home,
		Getenv: func(k string) string { return env[k] },
		LookPath: func(n string) (string, error) {
			if p, ok := bins[n]; ok {
				return p, nil
			}
			return "", os.ErrNotExist
		},
		Exists: func(p string) bool { _, err := os.Stat(p); return err == nil },
	}
}

func TestRegistryParses(t *testing.T) {
	reg, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg) < 70 {
		t.Fatalf("only %d entries", len(reg))
	}
	ids := map[string]bool{}
	for _, e := range reg {
		if ids[e.ID] {
			t.Errorf("duplicate id %s", e.ID)
		}
		ids[e.ID] = true
		if e.Name == "" || (e.Global == "" && e.Project == "") && e.ID != "eve" {
			t.Errorf("incomplete entry %+v", e)
		}
	}
	for _, id := range []string{"claude-code", "pi", "omp", "codex", "cursor", "gemini-cli", "opencode"} {
		if !ids[id] {
			t.Errorf("missing %s", id)
		}
	}
}

func TestDetect(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	os.MkdirAll(filepath.Join(home, ".goose-nothing"), 0o755)
	os.MkdirAll(filepath.Join(home, ".config", "goose"), 0o755)
	reg, _ := Builtin()
	env := fakeEnv(home, map[string]string{"pi": "/usr/bin/pi"}, map[string]string{"CODEX_HOME": "/x/codex"})
	got := map[string]Detection{}
	for _, d := range Detect(reg, env) {
		got[d.ID] = d
	}
	if got["claude-code"].Status != Configured {
		t.Errorf("claude-code: %+v", got["claude-code"])
	}
	if got["pi"].Status != Installed || got["pi"].Bin != "/usr/bin/pi" {
		t.Errorf("pi: %+v", got["pi"])
	}
	if got["goose"].Status != Configured {
		t.Errorf("goose: %+v", got["goose"])
	}
	if got["codex"].Status != Configured || got["codex"].GlobalAbs != "/x/codex/skills" {
		t.Errorf("codex env home: %+v", got["codex"])
	}
	if got["cursor"].Status != Absent {
		t.Errorf("cursor: %+v", got["cursor"])
	}
}

func TestExpandXDG(t *testing.T) {
	env := fakeEnv("/home/u", nil, map[string]string{"XDG_CONFIG_HOME": "/cfg"})
	if got := env.Expand("{config}/opencode/skills"); got != "/cfg/opencode/skills" {
		t.Error(got)
	}
	env = fakeEnv("/home/u", nil, nil)
	if got := env.Expand("{config}/x"); got != "/home/u/.config/x" {
		t.Error(got)
	}
	if got := env.Expand("~/a"); got != "/home/u/a" {
		t.Error(got)
	}
}

func TestTargetsDedupe(t *testing.T) {
	env := fakeEnv("/home/u", nil, nil)
	reg, _ := Builtin()
	var picked []Detection
	for _, d := range Detect(reg, env) {
		if d.ID == "pi" || d.ID == "omp" || d.ID == "claude-code" || d.ID == "cursor" {
			picked = append(picked, d)
		}
	}
	ts := Targets(picked, "user", "", env)
	if len(ts) != 3 {
		t.Fatalf("targets = %+v", ts)
	}
	for _, tg := range ts {
		if tg.Dir == "/home/u/.agents/skills" && len(tg.Harnesses) != 2 {
			t.Errorf("shared dir not merged: %+v", tg)
		}
	}
	ps := Targets(picked, "project", "/proj", env)
	for _, tg := range ps {
		if filepath.Dir(filepath.Dir(tg.Dir)) != "/proj" && filepath.Dir(tg.Dir) != "/proj" {
			t.Errorf("project dir %s", tg.Dir)
		}
	}
}

func TestRunning(t *testing.T) {
	reg, _ := Builtin()
	e := Running(reg, func(k string) string {
		if k == "PI_CODING_AGENT" {
			return "1"
		}
		return ""
	})
	if e == nil || e.ID != "pi" {
		t.Fatalf("%v", e)
	}
	if Running(reg, func(string) string { return "" }) != nil {
		t.Error("false positive")
	}
}

func mkStore(t *testing.T) (string, string) {
	store := filepath.Join(t.TempDir(), "skills", "demo")
	os.MkdirAll(store, 0o755)
	os.WriteFile(filepath.Join(store, "SKILL.md"), []byte("---\nname: demo\ndescription: d\n---\n"), 0o644)
	h, err := hash.Dir(store)
	if err != nil {
		t.Fatal(err)
	}
	return store, h
}

func TestInstallSymlinkAndCopy(t *testing.T) {
	store, h := mkStore(t)
	dir := filepath.Join(t.TempDir(), "skills")
	r, err := InstallSkill(dir, "demo", store, ModeSymlink, h, false)
	if err != nil {
		t.Fatal(err)
	}
	if problems := Check(*r, h); len(problems) != 0 {
		t.Fatal(problems)
	}
	if _, err := InstallSkill(dir, "demo", store, ModeSymlink, h, false); err != nil {
		t.Fatalf("idempotent install failed: %v", err)
	}
	if err := Remove(*r, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(r.Path); err == nil {
		t.Fatal("not removed")
	}
	rc, err := InstallSkill(dir, "demo", store, ModeCopy, h, false)
	if err != nil {
		t.Fatal(err)
	}
	if problems := Check(*rc, h); len(problems) != 0 {
		t.Fatal(problems)
	}
	os.WriteFile(filepath.Join(rc.Path, "SKILL.md"), []byte("tampered"), 0o644)
	if len(Check(*rc, h)) == 0 {
		t.Error("tampering not detected")
	}
	if Remove(*rc, false) == nil {
		t.Error("modified copy removed without force")
	}
}

func TestInstallRefusesForeignAndBacksUp(t *testing.T) {
	store, h := mkStore(t)
	dir := filepath.Join(t.TempDir(), "skills")
	os.MkdirAll(filepath.Join(dir, "demo"), 0o755)
	os.WriteFile(filepath.Join(dir, "demo", "mine.txt"), []byte("x"), 0o644)
	if _, err := InstallSkill(dir, "demo", store, ModeSymlink, h, false); err == nil {
		t.Fatal("overwrote foreign dir")
	}
	if _, err := InstallSkill(dir, "demo", store, ModeSymlink, h, true); err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "demo.skigard-backup-*"))
	if len(matches) != 1 {
		t.Fatalf("backup missing: %v", matches)
	}
}

func TestStateRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s", "installs.json")
	s := &State{}
	s.Upsert(Record{Name: "a", Path: "/x/a"})
	s.Upsert(Record{Name: "a", Path: "/x/a", Mode: "copy"})
	s.Upsert(Record{Name: "b", Path: "/x/b"})
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(p)
	if err != nil || len(got.Installs) != 2 || got.Installs[0].Mode != "copy" {
		t.Fatalf("%+v %v", got, err)
	}
	got.Drop("/x/a")
	if len(got.Installs) != 1 {
		t.Fatal("drop failed")
	}
}
