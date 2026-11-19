package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/safefs"
	"github.com/ambierhq/skigard/internal/source"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

func unixOnly(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs execute bits")
	}
}

// seedSkillWithScript writes an executable script next to SKILL.md.
func seedSkillWithScript(t *testing.T, dir, name string) string {
	t.Helper()
	d := writeSkill(t, dir, name, map[string]string{"scripts/run.sh": "#!/bin/sh\necho hi\n"})
	if err := os.Chmod(filepath.Join(d, "scripts", "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestApproveWritesHashV2WithExecSet(t *testing.T) {
	unixOnly(t)
	a, _ := newApp(t)
	src := t.TempDir()
	seedSkillWithScript(t, src, "alpha")
	approveLocal(t, a, src, "alpha")
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		t.Fatal(err)
	}
	ls, _ := lock.Get("alpha")
	if ls.HashVersion != 2 || !reflect.DeepEqual(ls.ExecSet, []string{"scripts/run.sh"}) || ls.ExecHash != hash.ExecDigest(ls.ExecSet) {
		t.Fatalf("%+v", ls)
	}
	raw, _ := os.ReadFile(a.Home.LockPath())
	if !strings.Contains(string(raw), "hash_version = 2") || !strings.Contains(string(raw), "exec_hash") {
		t.Fatalf("lock text:\n%s", raw)
	}
	if ps, _ := a.Verify(); HasErrors(ps) {
		t.Fatalf("verify: %+v", ps)
	}
}

func TestExecBitTamperIsCaughtByVerifyInstallAndApprove(t *testing.T) {
	unixOnly(t)
	a, userHome := newApp(t)
	customHarness(t, a, userHome)
	src := t.TempDir()
	seedSkillWithScript(t, src, "alpha")
	// Tamper in quarantine, before approval.
	addLocal(t, a, src, AddOpts{Skill: "alpha"})
	q := filepath.Join(a.Home.QuarantineSkill("alpha"), "scripts", "run.sh")
	os.Chmod(q, 0o644)
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: yes}); err == nil || !strings.Contains(err.Error(), "executable files") {
		t.Fatalf("quarantine exec tamper not refused: %v", err)
	}
	if _, err := os.Stat(a.Home.SkillDir("alpha")); err == nil {
		t.Fatal("tampered skill was approved")
	}
	os.Chmod(q, 0o755)
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: yes}); err != nil {
		t.Fatal(err)
	}
	// Tamper after approval: content hash is unchanged, only the mode.
	p := filepath.Join(a.Home.SkillDir("alpha"), "scripts", "run.sh")
	os.Chmod(p, 0o644)
	ps, _ := a.Verify()
	if !HasErrors(ps) || !strings.Contains(fmt.Sprint(ps), "executable files changed") {
		t.Fatalf("verify missed exec tamper: %+v", ps)
	}
	if _, _, err := a.Install(InstallOpts{Names: []string{"alpha"}, To: []string{"fake"}}); util.CodeOf(err) != util.ExitPolicy {
		t.Fatalf("install should refuse, got %v", err)
	}
	es, _ := a.List()
	if len(es) != 1 || !es[0].Drift {
		t.Fatalf("list should flag drift: %+v", es)
	}
	os.Chmod(p, 0o755)
	if ps, _ := a.Verify(); HasErrors(ps) {
		t.Fatalf("restored mode should verify: %+v", ps)
	}
}

// writeV1Lock writes skills/<name> and a version 1 lock entry for it, the way a
// release before hash v2 did.
func writeV1Lock(t *testing.T, a *App, src string, names ...string) {
	t.Helper()
	if err := a.Home.Init(); err != nil {
		t.Fatal(err)
	}
	lock := &store.Lock{Version: 1, HashVersion: 1}
	for _, n := range names {
		dest := a.Home.SkillDir(n)
		if _, err := safefs.CopyTree(filepath.Join(src, n), dest, safefs.DefaultLimits()); err != nil {
			t.Fatal(err)
		}
		h, err := hash.DirVersion(dest, 1)
		if err != nil {
			t.Fatal(err)
		}
		lock.Put(store.LockSkill{Name: n, Source: "local", Location: src, ContentHash: h, HashVersion: 1,
			Approval: store.ApprovalRec{By: "old", At: "2026-01-01T00:00:00Z"}})
	}
	if err := lock.Save(a.Home); err != nil {
		t.Fatal(err)
	}
	// Save stamps the current default; rewrite as a real v1 file (no per-entry version).
	raw, _ := os.ReadFile(a.Home.LockPath())
	s := strings.ReplaceAll(string(raw), "    hash_version = 1\n", "")
	s = strings.ReplaceAll(s, "  hash_version = 1\n", "")
	s = strings.Replace(s, "hash_version = 2", "hash_version = 1", 1)
	if err := os.WriteFile(a.Home.LockPath(), []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestV1LockStillVerifiesThenUpgrades(t *testing.T) {
	a, _ := newApp(t)
	src := t.TempDir()
	if runtime.GOOS == "windows" {
		writeSkill(t, src, "alpha", map[string]string{"scripts/run.sh": "echo"})
	} else {
		seedSkillWithScript(t, src, "alpha")
	}
	writeSkill(t, src, "beta", nil)
	writeV1Lock(t, a, src, "alpha", "beta")
	raw, _ := os.ReadFile(a.Home.LockPath())
	if strings.Contains(string(raw), "exec_hash") || !strings.Contains(string(raw), "hash_version = 1") {
		t.Fatalf("fixture is not a v1 lock:\n%s", raw)
	}
	if ps, _ := a.Verify(); HasErrors(ps) {
		t.Fatalf("v1 lock must keep verifying: %+v", ps)
	}
	before, _ := store.LoadLock(a.Home)
	old, _ := before.Get("alpha")
	oldHash := old.ContentHash

	res, err := a.LockUpgrade(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if !r.Upgraded {
			t.Fatalf("%+v", r)
		}
	}
	after, _ := store.LoadLock(a.Home)
	ls, _ := after.Get("alpha")
	if ls.HashVersion != 2 || ls.UpgradedFrom != oldHash || ls.ContentHash == oldHash || ls.Approval.By != "old" {
		t.Fatalf("%+v", ls)
	}
	if runtime.GOOS != "windows" && !reflect.DeepEqual(ls.ExecSet, []string{"scripts/run.sh"}) {
		t.Fatalf("exec set = %v", ls.ExecSet)
	}
	if b, _ := after.Get("beta"); b.HashVersion != 2 || len(b.ExecSet) != 0 || b.ExecHash != hash.ExecDigest(nil) {
		t.Fatalf("%+v", b)
	}
	if ps, _ := a.Verify(); HasErrors(ps) {
		t.Fatalf("verify after upgrade: %+v", ps)
	}
	again, _ := a.LockUpgrade(nil)
	if again[0].Upgraded || !strings.Contains(again[0].Note, "already") {
		t.Fatalf("%+v", again)
	}
}

func TestUpgradeRefusesWhenV1HashNoLongerMatches(t *testing.T) {
	a, _ := newApp(t)
	src := t.TempDir()
	writeSkill(t, src, "alpha", nil)
	writeSkill(t, src, "beta", nil)
	writeV1Lock(t, a, src, "alpha", "beta")
	os.WriteFile(filepath.Join(a.Home.SkillDir("alpha"), "SKILL.md"), []byte("tampered"), 0o644)
	lockBefore, _ := os.ReadFile(a.Home.LockPath())

	res, err := a.LockUpgrade([]string{"alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Upgraded || !strings.Contains(res[0].Note, "does not match") {
		t.Fatalf("%+v", res[0])
	}
	if after, _ := os.ReadFile(a.Home.LockPath()); !bytes.Equal(lockBefore, after) {
		t.Fatal("lock was rewritten although nothing was upgraded")
	}
	// A skill that is missing, or unknown, is never upgraded or invented.
	os.RemoveAll(a.Home.SkillDir("beta"))
	if res, _ := a.LockUpgrade([]string{"beta"}); res[0].Upgraded {
		t.Fatalf("%+v", res[0])
	}
	if _, err := a.LockUpgrade([]string{"ghost"}); err == nil {
		t.Fatal("unknown name should fail")
	}
	if lk, _ := store.LoadLock(a.Home); len(lk.Skills) != 2 || lk.Skills[0].HashVersion != 1 {
		t.Fatalf("%+v", lk.Skills)
	}
}

func TestUpgradeRefusesV1ExecTamper(t *testing.T) {
	unixOnly(t)
	a, _ := newApp(t)
	src := t.TempDir()
	seedSkillWithScript(t, src, "alpha")
	writeV1Lock(t, a, src, "alpha")
	// v1 bound the execute bit: an added bit must stop the upgrade, or it would be blessed silently.
	os.Chmod(filepath.Join(a.Home.SkillDir("alpha"), "SKILL.md"), 0o755)
	res, _ := a.LockUpgrade(nil)
	if res[0].Upgraded {
		t.Fatalf("upgrade blessed a changed mode: %+v", res[0])
	}
}

func TestLoadLockRejectsInconsistentExecFields(t *testing.T) {
	a, _ := newApp(t)
	src := t.TempDir()
	writeSkill(t, src, "alpha", nil)
	approveLocal(t, a, src, "alpha")
	raw, _ := os.ReadFile(a.Home.LockPath())
	bad := strings.Replace(string(raw), "  exec_hash =", "  exec_set = [\"evil.sh\"]\n  exec_hash =", 1)
	os.WriteFile(a.Home.LockPath(), []byte(bad), 0o644)
	if _, err := store.LoadLock(a.Home); err == nil || !strings.Contains(err.Error(), "exec_hash") {
		t.Fatalf("forged exec_set accepted: %v", err)
	}
}

func tgzModes(files map[string]int64, bodies map[string]string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for n, mode := range files {
		b := bodies[n]
		tw.WriteHeader(&tar.Header{Name: n, Typeflag: tar.TypeReg, Mode: mode, Size: int64(len(b))})
		tw.Write([]byte(b))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

// The exec set comes from the archive's modes, so it is the same on every OS,
// including Windows where the bit cannot be stored on disk.
func TestGitHubExecSetComesFromArchiveModes(t *testing.T) {
	const sha = "1111111111111111111111111111111111111111"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/o/r/commits/HEAD":
			w.Write([]byte(sha))
		case strings.HasPrefix(r.URL.Path, "/o/r/tar.gz/"):
			w.Write(tgzModes(map[string]int64{
				"r-abc/skills/gh/SKILL.md":       0o644,
				"r-abc/skills/gh/scripts/run.sh": 0o755,
				"r-abc/skills/gh/.DS_Store":      0o755, // skipped on copy: must not appear in the set
				"r-abc/skills/other/tool":        0o755, // another skill: must not appear
				"r-abc/README.md":                0o755,
			}, map[string]string{"r-abc/skills/gh/SKILL.md": fmt.Sprintf(goodSkill, "gh", "gh", "gh"), "r-abc/skills/gh/scripts/run.sh": "#!/bin/sh\n"}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	a, _ := newApp(t)
	a.Fetcher = &source.Fetcher{APIBase: srv.URL, CodeloadBase: srv.URL, Client: srv.Client(), Limits: safefs.DefaultLimits()}
	qs, err := a.Add(context.Background(), "github:o/r/skills/gh", AddOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if got := qs[0].Meta.ExecSet; !reflect.DeepEqual(got, []string{"scripts/run.sh"}) || qs[0].Meta.HashVersion != 2 {
		t.Fatalf("meta exec set = %v (%+v)", got, qs[0].Meta)
	}
	if _, err := a.Approve("gh", ApproveOpts{Confirm: yes}); err != nil {
		t.Fatal(err)
	}
	lock, _ := store.LoadLock(a.Home)
	ls, _ := lock.Get("gh")
	if !reflect.DeepEqual(ls.ExecSet, []string{"scripts/run.sh"}) {
		t.Fatalf("lock exec set = %v", ls.ExecSet)
	}
	if ps, _ := a.Verify(); HasErrors(ps) {
		t.Fatalf("verify: %+v", ps)
	}
	// Unchanged upstream: update reports "up to date" for a v2 entry.
	rs, err := a.Update(context.Background(), nil)
	if err != nil || rs[0].Changed {
		t.Fatalf("%+v %v", rs, err)
	}
}
