package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/store"
)

// approvedStrict approves "alpha" from a fresh local source in strict mode.
func approvedStrict(t *testing.T) (*App, string) {
	t.Helper()
	a, _ := newApp(t)
	setSettings(t, a, store.Settings{Mode: "strict"})
	src := t.TempDir()
	writeSkill(t, src, "alpha", nil)
	addLocal(t, a, src, AddOpts{Skill: "alpha"})
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: yes}); err != nil {
		t.Fatal(err)
	}
	return a, src
}

func editLocal(t *testing.T, a *App, extra string) {
	t.Helper()
	p := filepath.Join(a.Home.SkillDir("alpha"), "SKILL.md")
	b, _ := os.ReadFile(p)
	os.Chmod(p, 0o644) // approved skills are read-only; this simulates the user's chmod/force-save
	if err := os.WriteFile(p, append(b, []byte(extra)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func adoptAndApprove(t *testing.T, a *App) *store.LockSkill {
	t.Helper()
	if _, err := a.Adopt(context.Background(), "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: yes}); err != nil {
		t.Fatal(err)
	}
	return lockEntry(t, a, "alpha")
}

func TestAdoptPreconditions(t *testing.T) {
	a, _ := newApp(t)
	if _, err := a.Adopt(context.Background(), "nope"); err == nil || !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("%v", err)
	}
	a, _ = approvedStrict(t)
	if _, err := a.Adopt(context.Background(), "alpha"); err == nil || !strings.Contains(err.Error(), "nothing to adopt") {
		t.Fatalf("%v", err)
	}
	editLocal(t, a, "mine\n")
	// Something already in quarantine blocks adopt.
	os.MkdirAll(a.Home.QuarantineSkill("alpha"), 0o755)
	if _, err := a.Adopt(context.Background(), "alpha"); err == nil || !strings.Contains(err.Error(), "quarantine") {
		t.Fatalf("%v", err)
	}
}

func TestAdoptStagesOnlyAndApproveRecordsLocal(t *testing.T) {
	a, _ := approvedStrict(t)
	before := lockEntry(t, a, "alpha")
	editLocal(t, a, "mine\n")
	lockBytes, _ := os.ReadFile(a.Home.LockPath())
	q, err := a.Adopt(context.Background(), "alpha")
	if err != nil || q.Report == nil {
		t.Fatalf("%v", err)
	}
	if q.Meta.UpstreamHash != before.ContentHash {
		t.Errorf("baseline = %s, want %s", q.Meta.UpstreamHash, before.ContentHash)
	}
	if now, _ := os.ReadFile(a.Home.LockPath()); string(now) != string(lockBytes) {
		t.Error("adopt changed the lock")
	}
	if b, _ := os.ReadFile(filepath.Join(a.Home.SkillDir("alpha"), "SKILL.md")); !strings.Contains(string(b), "mine") {
		t.Error("adopt changed skills/")
	}
	// The in-place edit is compared with the rebuilt upstream, so the diff is not empty.
	d, err := a.Diff("alpha")
	if err != nil || !strings.Contains(d, "+ mine") {
		t.Fatalf("diff of an in-place adopt: %q %v", d, err)
	}
	// An adopted copy is approved through the normal path and recorded as local.
	ls := adoptAndApprove2(t, a)
	if ls.Approval.Mode != store.ApprovalLocal || ls.UpstreamHash != before.ContentHash || ls.ContentHash == before.ContentHash {
		t.Fatalf("%+v", ls)
	}
	if a.hasLocalEdits(ls) || inQuarantine(a, "alpha") {
		t.Error("state after approve is not clean")
	}
	// A second edit adopts against the same upstream baseline.
	editLocal(t, a, "more\n")
	if ls2 := adoptAndApprove(t, a); ls2.UpstreamHash != before.ContentHash {
		t.Fatalf("%+v", ls2)
	}
}

// adoptAndApprove2 approves the already-staged quarantine copy.
func adoptAndApprove2(t *testing.T, a *App) *store.LockSkill {
	t.Helper()
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: yes}); err != nil {
		t.Fatal(err)
	}
	return lockEntry(t, a, "alpha")
}

func TestApprovedAdoptWithSameContentIsNotAFork(t *testing.T) {
	a, _ := approvedStrict(t)
	ls := lockEntry(t, a, "alpha")
	meta := &store.Meta{Name: "alpha", ContentHash: ls.ContentHash, UpstreamHash: ls.ContentHash}
	if isFork(meta) {
		t.Error("identical content counted as a fork")
	}
}
