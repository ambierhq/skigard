package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/safefs"
	"github.com/ambierhq/skigard/internal/source"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// Edit copies an approved skill into quarantine as an editable working copy.
// The human edits that copy; `adopt` then finalizes it for review and approval.
// skills/, the lock and installs are never touched, so the approved version stays
// live and pristine until a human approves the edit.
func (a *App) Edit(name string) (*Quarantined, error) {
	unlock, err := a.Home.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := util.CheckName(name); err != nil {
		return nil, err
	}
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, err
	}
	ls, ok := lock.Get(name)
	if !ok {
		return nil, util.Errf("%q is not approved; there is nothing to edit", name)
	}
	dir := a.Home.SkillDir(name)
	if !isDir(dir) {
		return nil, util.Errf("%q is missing from skills/; nothing to edit", name)
	}
	st, err := hash.Check(dir, ls.Identity())
	if err != nil {
		return nil, util.Errf("cannot hash %s: %v", name, err)
	}
	if !st.OK() {
		return nil, util.Errf("skills/%s was already edited in place (it no longer matches the lock); run `skigard adopt %s` to stage that edit instead", name, name)
	}
	if ls.HashVersion != hash.Version {
		return nil, util.Errf("%s is pinned with hash version %d; run `skigard lock upgrade %s` before editing", name, ls.HashVersion, name)
	}
	if err := a.quarantineFree(name); err != nil {
		return nil, err
	}
	if err := a.Home.Init(); err != nil {
		return nil, err
	}
	meta := forkMeta(ls)
	meta.Editing = true
	return a.place(dir, meta, nil)
}

// Adopt stages local changes of an approved skill into quarantine so a human can
// approve them (recorded as mode local). With an editable working copy from `edit`
// it finalizes that copy; otherwise it stages an edit made in place in skills/ and
// tries to rebuild the pristine upstream to compare it with. It changes neither
// skills/, the lock nor any install: approval stays a separate human step.
func (a *App) Adopt(ctx context.Context, name string) (*Quarantined, error) {
	unlock, err := a.Home.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := util.CheckName(name); err != nil {
		return nil, err
	}
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, err
	}
	ls, ok := lock.Get(name)
	if !ok {
		return nil, util.Errf("%q is not approved", name)
	}
	if meta, err := store.LoadMeta(a.Home, name); err == nil && meta.Editing && isDir(a.Home.QuarantineSkill(name)) {
		return a.finalizeEdit(ls, meta)
	}
	dir := a.Home.SkillDir(name)
	if !isDir(dir) {
		return nil, util.Errf("%q is missing from skills/; nothing to adopt", name)
	}
	st, err := hash.Check(dir, ls.Identity())
	if err != nil {
		return nil, util.Errf("cannot hash %s: %v", name, err)
	}
	if st.OK() {
		return nil, util.Errf("nothing to adopt: %s matches the lock (to change it, run `skigard edit %s`)", name, name)
	}
	if ls.HashVersion != hash.Version {
		return nil, util.Errf("%s is pinned with hash version %d; run `skigard lock upgrade %s` (on a copy of the approved content) before adopting edits", name, ls.HashVersion, name)
	}
	if err := a.quarantineFree(name); err != nil {
		return nil, err
	}
	meta := forkMeta(ls)
	meta.InPlace = true
	q, err := a.place(dir, meta, nil)
	if err != nil {
		return nil, err
	}
	if err := a.rebuildBase(ctx, ls); err != nil {
		q.BaselineWarning = util.Clean(err.Error())
	}
	return q, nil
}

// quarantineFree errors when name already has anything in quarantine.
func (a *App) quarantineFree(name string) error {
	if _, err := os.Lstat(a.Home.QuarantineSkill(name)); err == nil {
		return util.Errf("%s is already in quarantine; approve or reject it first (skigard approve %s | skigard reject %s)", name, name, name)
	}
	if _, err := os.Lstat(a.Home.MetaPath(name)); err == nil {
		return util.Errf("%s has quarantine metadata; reject it first (skigard reject %s)", name, name)
	}
	return nil
}

// forkMeta is the metadata of a local fork of an approved skill: the lock entry's
// source, with the upstream baseline the edits are measured against.
func forkMeta(ls *store.LockSkill) *store.Meta {
	m := &store.Meta{Name: ls.Name, Source: ls.Source, Kind: kindOf(ls), Location: ls.Location, Ref: ls.Ref,
		Subpath: ls.Subpath, Commit: ls.Commit, UpstreamHash: ls.ContentHash}
	if ls.UpstreamHash != "" {
		m.UpstreamHash = ls.UpstreamHash
	}
	return m
}

// finalizeEdit hashes, scans and unlocks an editable working copy. The tree is
// copied through safefs (limits, symlink refusal) like any other input. If nothing
// differs from the approved skill the working copy is left in place.
func (a *App) finalizeEdit(ls *store.LockSkill, old *store.Meta) (*Quarantined, error) {
	name := ls.Name
	if ls.HashVersion != hash.Version {
		return nil, util.Errf("%s is pinned with hash version %d; run `skigard lock upgrade %s` first", name, ls.HashVersion, name)
	}
	dest := a.Home.QuarantineSkill(name)
	aside := filepath.Join(a.Home.QuarantineDir(), ".edit-"+util.RandomHex(6))
	if err := os.Rename(dest, aside); err != nil {
		return nil, err
	}
	restore := func() {
		os.RemoveAll(dest)
		_ = os.Rename(aside, dest)
		_ = old.Save(a.Home)
	}
	fresh := *old
	fresh.Editing, fresh.Review = false, nil
	q, err := a.place(aside, &fresh, nil)
	if err != nil {
		restore()
		return nil, err
	}
	got, want := q.Meta.Identity(), ls.Identity()
	if got.Content == want.Content && got.ExecHash == want.ExecHash {
		restore()
		q.Meta, q.NoChanges = old, true
		return q, nil
	}
	os.RemoveAll(aside)
	return q, nil
}

// rebuildBase re-reads the original upstream of an approved skill (the lock's pinned
// commit for GitHub, the path for a local source) and keeps it as the baseline an
// in-place edit is compared with, but only if it hashes to the entry's baseline.
func (a *App) rebuildBase(ctx context.Context, ls *store.LockSkill) error {
	if a.Fetcher == nil {
		return util.Errf("no fetcher available")
	}
	ref := ls.Ref
	if ls.Commit != "" && strings.HasPrefix(ls.Location, "https://github.com/") {
		ref = ls.Commit
	}
	spec, err := specFor(ls, ref)
	if err != nil {
		return err
	}
	f, err := a.Fetcher.Fetch(ctxOrBackground(ctx), spec)
	if err != nil {
		return util.Errf("could not re-fetch the original upstream: %v", err)
	}
	defer f.Cleanup()
	src := filepath.Join(f.Dir, filepath.FromSlash(ls.Subpath))
	if !isDir(src) {
		return util.Errf("the skill is no longer at %q upstream", ls.Subpath)
	}
	base := a.Home.BasePath(ls.Name)
	tmp := base + ".tmp-" + util.RandomHex(4)
	defer os.RemoveAll(tmp)
	if _, err := safefs.CopyTree(src, tmp, a.Fetcher.Limits); err != nil {
		return util.Errf("cannot take the original upstream: %v", err)
	}
	want := ls.ContentHash
	if ls.UpstreamHash != "" {
		want = ls.UpstreamHash
	}
	got, err := hash.DirVersion(tmp, ls.Identity().Version)
	if err != nil {
		return err
	}
	if got != want {
		return util.Errf("the upstream no longer matches what was approved (hash %s, expected %s)", hash.Short(got, 12), hash.Short(want, 12))
	}
	if err := os.RemoveAll(base); err != nil {
		return err
	}
	return os.Rename(tmp, base)
}

// kindOf is the source kind of a lock entry, as updateOne derives it.
func kindOf(ls *store.LockSkill) string {
	if strings.HasPrefix(ls.Location, "https://github.com/") {
		return source.KindGitHub
	}
	return source.KindLocal
}
