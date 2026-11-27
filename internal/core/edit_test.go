package core

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/safefs"
	"github.com/ambierhq/skigard/internal/source"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// editWorking appends to the working copy in quarantine, the way a human's editor would.
func editWorking(t *testing.T, a *App, extra string) {
	t.Helper()
	p := filepath.Join(a.Home.QuarantineSkill("alpha"), "SKILL.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(b, []byte(extra)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustEdit(t *testing.T, a *App) *Quarantined {
	t.Helper()
	q, err := a.Edit("alpha")
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func TestEditPreconditions(t *testing.T) {
	a, _ := newApp(t)
	if _, err := a.Edit("nope"); err == nil || !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("%v", err)
	}
	if _, err := a.Edit("../x"); err == nil {
		t.Fatal("bad name accepted")
	}
	a, _ = approvedStrict(t)
	editLocal(t, a, "in place\n")
	if _, err := a.Edit("alpha"); err == nil || !strings.Contains(err.Error(), "edited in place") || !strings.Contains(err.Error(), "skigard adopt alpha") {
		t.Fatalf("drifted: %v", err)
	}
	a, _ = approvedStrict(t)
	os.MkdirAll(a.Home.QuarantineSkill("alpha"), 0o755)
	if _, err := a.Edit("alpha"); err == nil || !strings.Contains(err.Error(), "approve or reject") {
		t.Fatalf("busy: %v", err)
	}
}

func TestEditCopiesAndMarksEditing(t *testing.T) {
	a, _ := approvedStrict(t)
	before := lockEntry(t, a, "alpha")
	lockBytes, _ := os.ReadFile(a.Home.LockPath())
	q := mustEdit(t, a)
	if !q.Meta.Editing || q.Meta.UpstreamHash != before.ContentHash {
		t.Fatalf("%+v", q.Meta)
	}
	m, err := store.LoadMeta(a.Home, "alpha")
	if err != nil || !m.Editing {
		t.Fatalf("%+v %v", m, err)
	}
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(filepath.Join(a.Home.QuarantineSkill("alpha"), "SKILL.md"))
		if fi.Mode().Perm()&0o200 == 0 {
			t.Error("working copy is not writable")
		}
		if fi, _ := os.Stat(a.Home.QuarantineSkill("alpha")); fi.Mode().Perm() != 0o700 {
			t.Errorf("working copy dir is %v", fi.Mode().Perm())
		}
	}
	if now, _ := os.ReadFile(a.Home.LockPath()); string(now) != string(lockBytes) {
		t.Error("edit changed the lock")
	}
	if ps, _ := a.Verify(); HasErrors(ps) {
		t.Fatalf("verify: %+v", ps)
	}
	es, _ := a.List()
	var state string
	for _, e := range es {
		if e.State != "approved" {
			state = e.State
		}
	}
	if state != "editing" {
		t.Errorf("list state %q", state)
	}
	if s, err := a.Show("alpha"); err != nil || !strings.Contains(s, "alpha") {
		t.Fatalf("%q %v", s, err)
	}
}

func TestEditingWorkingCopyBlocksApproveReviewAndUpdate(t *testing.T) {
	a, src := approvedStrict(t)
	mustEdit(t, a)
	editWorking(t, a, "mine\n")
	_, err := a.Approve("alpha", ApproveOpts{Confirm: yes})
	if util.CodeOf(err) != util.ExitPolicy || !strings.Contains(err.Error(), "still an editable working copy") || !strings.Contains(err.Error(), "skigard adopt alpha") {
		t.Fatalf("approve: %v", err)
	}
	if _, err := a.Review(context.Background(), "alpha", ReviewOpts{Harness: "pi"}); err == nil || !strings.Contains(err.Error(), "editable working copy") {
		t.Fatalf("review: %v", err)
	}
	if _, err := a.RecordManual("alpha", "pass", "ok", "me"); err == nil || !strings.Contains(err.Error(), "editable working copy") {
		t.Fatalf("record: %v", err)
	}
	if _, err := a.ManualPrompt("alpha"); err == nil {
		t.Fatal("manual prompt on a working copy")
	}
	// update leaves it alone, even when upstream changed.
	rewriteSkill(src, "alpha", "upstream change\n")
	rs, err := a.Update(context.Background(), []string{"alpha"})
	if err != nil || rs[0].Changed || !strings.Contains(rs[0].Note, "edit in progress") {
		t.Fatalf("%+v %v", rs, err)
	}
	if b, _ := os.ReadFile(filepath.Join(a.Home.QuarantineSkill("alpha"), "SKILL.md")); !strings.Contains(string(b), "mine") {
		t.Error("update clobbered the working copy")
	}
	if m, _ := store.LoadMeta(a.Home, "alpha"); !m.Editing {
		t.Error("update cleared the editing flag")
	}
	// the diff already works on the working copy
	r, err := a.DiffAgainst("alpha")
	if err != nil || !strings.Contains(r.Text, "+ mine") || r.Against != "the approved version in skills/" {
		t.Fatalf("%+v %v", r, err)
	}
	// reject discards it
	if err := a.Reject("alpha"); err != nil {
		t.Fatal(err)
	}
	if inQuarantine(a, "alpha") || len(store.QuarantinedNames(a.Home)) != 0 {
		t.Error("reject left the working copy")
	}
}

func TestFinalizeEditStagesAndApproveRecordsLocalAndReadOnly(t *testing.T) {
	a, _ := approvedStrict(t)
	before := lockEntry(t, a, "alpha")
	mustEdit(t, a)
	editWorking(t, a, "mine\n")
	q, err := a.Adopt(context.Background(), "alpha")
	if err != nil || q.NoChanges || q.Report == nil {
		t.Fatalf("%+v %v", q, err)
	}
	if q.Meta.Editing || q.Meta.UpstreamHash != before.ContentHash || q.Hash == before.ContentHash {
		t.Fatalf("%+v", q.Meta)
	}
	if st, err := hash.Check(a.Home.QuarantineSkill("alpha"), q.Meta.Identity()); err != nil || !st.OK() {
		t.Fatalf("recorded hash does not match: %+v %v", st, err)
	}
	// skills/ still holds the pristine approved version.
	if ps, _ := a.Verify(); HasErrors(ps) {
		t.Fatalf("verify: %+v", ps)
	}
	d, err := a.DiffAgainst("alpha")
	if err != nil || !strings.Contains(d.Text, "+ mine") || d.Against == "" || d.NoBaseline {
		t.Fatalf("%+v %v", d, err)
	}
	var summary string
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: func(s, w string) error { summary = s; return nil }}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "changes  comparing the approved version in skills/ with your edit in quarantine\n") || !strings.Contains(summary, "+ mine") {
		t.Errorf("summary:\n%s", summary)
	}
	ls := lockEntry(t, a, "alpha")
	if ls.Approval.Mode != store.ApprovalLocal || ls.UpstreamHash != before.ContentHash || ls.ContentHash != q.Hash {
		t.Fatalf("%+v", ls)
	}
	assertReadOnly(t, a.Home.SkillDir("alpha"))
	if ps, _ := a.Verify(); HasErrors(ps) {
		t.Fatalf("verify: %+v", ps)
	}
	// A second round works over the read-only tree and replaces it.
	mustEdit(t, a)
	editWorking(t, a, "again\n")
	if _, err := a.Adopt(context.Background(), "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: yes}); err != nil {
		t.Fatal(err)
	}
	if ls2 := lockEntry(t, a, "alpha"); ls2.UpstreamHash != before.ContentHash || ls2.ContentHash == ls.ContentHash {
		t.Fatalf("%+v", ls2)
	}
	assertReadOnly(t, a.Home.SkillDir("alpha"))
	olds, _ := filepath.Glob(filepath.Join(a.Home.SkillsDir(), ".old-*"))
	stage, _ := filepath.Glob(filepath.Join(a.Home.SkillsDir(), ".staging-*"))
	if len(olds)+len(stage) != 0 {
		t.Errorf("leftovers: %v %v", olds, stage)
	}
}

func TestFinalizeEditWithoutChangesLeavesWorkingCopy(t *testing.T) {
	a, _ := approvedStrict(t)
	mustEdit(t, a)
	q, err := a.Adopt(context.Background(), "alpha")
	if err != nil || !q.NoChanges {
		t.Fatalf("%+v %v", q, err)
	}
	m, err := store.LoadMeta(a.Home, "alpha")
	if err != nil || !m.Editing || !inQuarantine(a, "alpha") {
		t.Fatalf("working copy not left in place: %+v %v", m, err)
	}
	if ents, _ := filepath.Glob(filepath.Join(a.Home.QuarantineDir(), ".edit-*")); len(ents) != 0 {
		t.Errorf("leftover %v", ents)
	}
	// ...and still finalizes once there is a change.
	editWorking(t, a, "now changed\n")
	if q, err := a.Adopt(context.Background(), "alpha"); err != nil || q.NoChanges || q.Meta.Editing {
		t.Fatalf("%+v %v", q, err)
	}
}

func TestFinalizeEditRefusesSymlinkInWorkingCopy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks")
	}
	a, _ := approvedStrict(t)
	mustEdit(t, a)
	if err := os.Symlink("/etc/passwd", filepath.Join(a.Home.QuarantineSkill("alpha"), "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Adopt(context.Background(), "alpha"); err == nil {
		t.Fatal("symlink accepted")
	}
	if m, err := store.LoadMeta(a.Home, "alpha"); err != nil || !m.Editing || !inQuarantine(a, "alpha") {
		t.Fatalf("working copy not restored: %+v %v", m, err)
	}
}

// ---- in-place adopt: rebuilding the baseline --------------------------------

type ghFake struct {
	body string
	fail bool
	srv  *httptest.Server
	a    *App
}

func newGHApp(t *testing.T) *ghFake {
	t.Helper()
	const sha = "1111111111111111111111111111111111111111"
	g := &ghFake{body: fmt.Sprintf(goodSkill, "gh", "gh", "gh")}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case g.fail:
			http.Error(w, "down", http.StatusInternalServerError)
		case r.URL.Path == "/repos/o/r/commits/HEAD":
			w.Write([]byte(sha))
		case strings.HasPrefix(r.URL.Path, "/o/r/tar.gz/"):
			w.Write(tgz(map[string]string{"r-abc/skills/gh/SKILL.md": g.body}))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(g.srv.Close)
	a, _ := newApp(t)
	setSettings(t, a, store.Settings{Mode: "strict"})
	a.Fetcher = &source.Fetcher{APIBase: g.srv.URL, CodeloadBase: g.srv.URL, Client: g.srv.Client(), Limits: safefs.DefaultLimits()}
	g.a = a
	if _, err := a.Add(context.Background(), "github:o/r/skills/gh", AddOpts{}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Approve("gh", ApproveOpts{Confirm: yes}); err != nil {
		t.Fatal(err)
	}
	util.MakeWritable(a.Home.SkillDir("gh"))
	p := filepath.Join(a.Home.SkillDir("gh"), "SKILL.md")
	b, _ := os.ReadFile(p)
	os.WriteFile(p, append(b, []byte("mine\n")...), 0o644)
	return g
}

func TestInPlaceAdoptRebuildsBaselineFromPinnedCommit(t *testing.T) {
	g := newGHApp(t)
	a := g.a
	q, err := a.Adopt(context.Background(), "gh")
	if err != nil || q.BaselineWarning != "" || !q.Meta.InPlace {
		t.Fatalf("%+v %v", q, err)
	}
	if !isDir(a.Home.BasePath("gh")) {
		t.Fatal("no baseline stored")
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(a.Home.BasePath("gh")); fi.Mode().Perm() != 0o700 {
			t.Errorf("baseline dir is %v", fi.Mode().Perm())
		}
	}
	d, err := a.DiffAgainst("gh")
	if err != nil || d.NoBaseline || !strings.Contains(d.Against, "original upstream") || !strings.Contains(d.Text, "+ mine") {
		t.Fatalf("%+v %v", d, err)
	}
	var summary string
	if _, err := a.Approve("gh", ApproveOpts{Confirm: func(s, w string) error { summary = s; return nil }}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "comparing the original upstream version your edit started from with your edit in quarantine") || !strings.Contains(summary, "+ mine") {
		t.Errorf("summary:\n%s", summary)
	}
	if isDir(a.Home.BasePath("gh")) {
		t.Error("approve left the baseline")
	}
	if ls := lockEntry(t, a, "gh"); ls.Approval.Mode != store.ApprovalLocal {
		t.Fatalf("%+v", ls)
	}
}

func TestInPlaceAdoptWithoutBaselineWarnsAndSummarySaysSo(t *testing.T) {
	for name, tweak := range map[string]func(*ghFake){
		"hash mismatch": func(g *ghFake) { g.body += "force-pushed over\n" },
		"fetch failure": func(g *ghFake) { g.fail = true },
	} {
		t.Run(name, func(t *testing.T) {
			g := newGHApp(t)
			tweak(g)
			q, err := g.a.Adopt(context.Background(), "gh")
			if err != nil || q.BaselineWarning == "" {
				t.Fatalf("%+v %v", q, err)
			}
			if isDir(g.a.Home.BasePath("gh")) {
				t.Error("a baseline was stored")
			}
			if ents, _ := filepath.Glob(g.a.Home.BasePath("gh") + ".tmp-*"); len(ents) != 0 {
				t.Errorf("leftover %v", ents)
			}
			d, err := g.a.DiffAgainst("gh")
			if err != nil || !d.NoBaseline || d.Text != "" {
				t.Fatalf("%+v %v", d, err)
			}
			var summary string
			if _, err := g.a.Approve("gh", ApproveOpts{Confirm: func(s, w string) error { summary = s; return nil }}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(summary, "NO COMPARISON  ") || strings.Contains(summary, "changes  ") {
				t.Errorf("summary:\n%s", summary)
			}
		})
	}
}

func TestBaselineRemovedByReject(t *testing.T) {
	g := newGHApp(t)
	if _, err := g.a.Adopt(context.Background(), "gh"); err != nil {
		t.Fatal(err)
	}
	if err := g.a.Reject("gh"); err != nil {
		t.Fatal(err)
	}
	if isDir(g.a.Home.BasePath("gh")) || inQuarantine(g.a, "gh") {
		t.Error("reject left files")
	}
}

func TestInPlaceAdoptLocalSourceRebuildsFromPath(t *testing.T) {
	a, _ := approvedStrict(t)
	editLocal(t, a, "mine\n")
	if q, err := a.Adopt(context.Background(), "alpha"); err != nil || q.BaselineWarning != "" || !isDir(a.Home.BasePath("alpha")) {
		t.Fatalf("%+v %v", q, err)
	}
}

// ---- read-only approved skills ----------------------------------------------

func assertReadOnly(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		perm := fi.Mode().Perm()
		switch {
		case d.IsDir() && perm != 0o555:
			t.Errorf("dir %s is %v, want 0555", p, perm)
		case fi.Mode().IsRegular() && perm != 0o444 && perm != 0o555:
			t.Errorf("file %s is %v, want 0444 or 0555", p, perm)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPromoteMakesSkillReadOnlyKeepingExecBits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are synthetic on Windows")
	}
	a, _ := newApp(t)
	setSettings(t, a, store.Settings{Mode: "strict"})
	src := t.TempDir()
	d := writeSkill(t, src, "alpha", map[string]string{"scripts/run.sh": "#!/bin/sh\necho hi\n", "refs/a.md": "a\n"})
	os.Chmod(filepath.Join(d, "scripts", "run.sh"), 0o755)
	addLocal(t, a, src, AddOpts{Skill: "alpha"})
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: yes, Override: true, Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	dir := a.Home.SkillDir("alpha")
	assertReadOnly(t, dir)
	if fi, _ := os.Stat(filepath.Join(dir, "scripts", "run.sh")); fi.Mode().Perm() != 0o555 {
		t.Errorf("exec file is %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(filepath.Join(dir, "SKILL.md")); fi.Mode().Perm() != 0o444 {
		t.Errorf("plain file is %v", fi.Mode().Perm())
	}
	if st, err := hash.Check(dir, lockEntry(t, a, "alpha").Identity()); err != nil || !st.OK() {
		t.Fatalf("hash of the read-only tree: %+v %v", st, err)
	}
	if ps, _ := a.Verify(); HasErrors(ps) {
		t.Fatalf("verify: %+v", ps)
	}
	if es, err := a.List(); err != nil || es[0].Drift {
		t.Fatalf("%+v %v", es, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("x"), 0o644); err == nil && os.Geteuid() != 0 {
		t.Error("the approved copy is writable")
	}
}

func TestUtilRemoveAndReplaceReadOnlyTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are synthetic on Windows")
	}
	root := t.TempDir()
	tree := filepath.Join(root, "t")
	os.MkdirAll(filepath.Join(tree, "sub"), 0o755)
	os.WriteFile(filepath.Join(tree, "sub", "f"), []byte("x"), 0o644)
	if err := util.MakeReadOnly(tree); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { util.MakeWritable(tree) })
	if err := os.RemoveAll(tree); err == nil && os.Geteuid() != 0 {
		t.Fatal("plain RemoveAll was expected to fail on a read-only tree")
	}
	if err := util.RemoveAll(tree); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(tree); !os.IsNotExist(err) {
		t.Fatal("tree still there")
	}
}

func TestApproveReplacesReadOnlySkillAndCleansBackup(t *testing.T) {
	// Replacing an approved (read-only) skill must also remove its read-only .old- backup.
	a, src := approvedStrict(t)
	rewriteSkill(src, "alpha", "v2\n")
	if _, err := a.Update(context.Background(), []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: yes}); err != nil {
		t.Fatal(err)
	}
	assertReadOnly(t, a.Home.SkillDir("alpha"))
	if olds, _ := filepath.Glob(filepath.Join(a.Home.SkillsDir(), ".old-*")); len(olds) != 0 {
		t.Errorf("backup not cleaned: %v", olds)
	}
}
