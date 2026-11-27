package core

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/safefs"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/skill"
	"github.com/ambierhq/skigard/internal/source"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// AddOpts selects what to take from a source.
type AddOpts struct {
	Skill string // pick one skill by name or path
	All   bool
	Name  string // override the name (single skill only)
	Force bool   // allow replacing a name that is already approved
}

// Quarantined is the result of adding one skill.
type Quarantined struct {
	Name   string
	Hash   string
	Meta   *store.Meta
	Report *scan.Report
	// NoChanges is set by `adopt` on an editable working copy that still equals the approved
	// skill: nothing was staged and the working copy is left in place.
	NoChanges bool
	// BaselineWarning is set by an in-place `adopt` when the pristine upstream could not be
	// rebuilt, so `diff` and `approve` have nothing to compare the edit with.
	BaselineWarning string
}

// Add fetches a source into quarantine and scans it.
func (a *App) Add(ctx context.Context, src string, o AddOpts) ([]*Quarantined, error) {
	unlock, err := a.Home.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = ctxOrBackground(ctx)
	if err := a.Home.Init(); err != nil {
		return nil, err
	}
	spec, err := source.Parse(src)
	if err != nil {
		return nil, err
	}
	f, err := a.Fetcher.Fetch(ctx, spec)
	if err != nil {
		return nil, err
	}
	defer f.Cleanup()
	return a.quarantine(f, spec, o)
}

func (a *App) quarantine(f *source.Fetched, spec source.Spec, o AddOpts) ([]*Quarantined, error) {
	root := f.Dir
	if spec.Subpath != "" && spec.Kind == source.KindGitHub {
		root = filepath.Join(f.Dir, filepath.FromSlash(spec.Subpath))
	}
	if !isDir(root) {
		return nil, util.Errf("path %q not found in %s", spec.Subpath, spec)
	}
	found, err := skill.Discover(root, 6)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, util.Errf("no SKILL.md found in %s", spec)
	}
	picked := found
	if o.Skill != "" {
		picked = nil
		for _, fd := range found {
			if (fd.Meta != nil && fd.Meta.Name == o.Skill) || path.Base(fd.Rel) == o.Skill || fd.Rel == o.Skill {
				picked = append(picked, fd)
			}
		}
		if len(picked) == 0 {
			return nil, util.Errf("no skill %q in %s; found: %s", o.Skill, spec, describe(found))
		}
	} else if !o.All && len(found) > 1 {
		return nil, util.Errf("%s contains %d skills; choose one with --skill NAME or take all with --all: %s", spec, len(found), describe(found))
	}
	if o.Name != "" && len(picked) != 1 {
		return nil, util.Errf("--name needs exactly one skill")
	}
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, err
	}
	var out []*Quarantined
	for _, fd := range picked {
		name := o.Name
		if name == "" {
			if fd.Meta == nil || fd.Meta.Name == "" || !skill.ValidName(fd.Meta.Name) {
				return nil, util.Errf("skill at %q has no valid name (%s); pass --name", fd.Rel, strings.Join(fd.Bad, "; "))
			}
			name = fd.Meta.Name
		}
		if err := util.CheckName(name); err != nil {
			return nil, err
		}
		if _, ok := lock.Get(name); ok && !o.Force {
			return nil, util.Errf("%q is already approved; use `skigard update %s`, or --force to replace it from this source", name, name)
		}
		repoPath := path.Join(spec.Subpath, fd.Rel)
		if repoPath == "." {
			repoPath = ""
		}
		for _, l := range f.SkippedLinks {
			if repoPath == "" || l == repoPath || strings.HasPrefix(l, repoPath+"/") {
				return nil, util.Errf("%s contains a symlink (%s); skigard refuses to fetch skills with symlinks", name, util.Clean(l))
			}
		}
		srcDir := filepath.Join(root, filepath.FromSlash(fd.Rel))
		full := spec
		full.Subpath = repoPath
		meta := &store.Meta{
			Name: name, Source: full.String(), Kind: spec.Kind, Location: spec.Location(), Ref: spec.Ref,
			Subpath: repoPath, Commit: f.Commit,
		}
		if spec.Kind == source.KindLocal {
			meta.Source = srcDir
			meta.Location = spec.Path
			meta.Subpath = fd.Rel
		}
		q, err := a.place(srcDir, meta, f.ExecFor(repoPath))
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, nil
}

// place copies srcDir into quarantine/<name>, records metadata and scans it.
// fetched is the execute-bit set recorded by the source archive, when it has one
// (see source.Fetched.ExecFor); otherwise the source files' own modes are used.
func (a *App) place(srcDir string, meta *store.Meta, fetched *source.ExecSet) (*Quarantined, error) {
	dest := a.Home.QuarantineSkill(meta.Name)
	if err := os.RemoveAll(dest); err != nil {
		return nil, err
	}
	if !meta.InPlace {
		os.RemoveAll(a.Home.BasePath(meta.Name)) // a stale baseline belongs to a replaced in-place adoption
	}
	res, err := safefs.CopyTree(srcDir, dest, a.Fetcher.Limits)
	if err != nil {
		os.RemoveAll(dest)
		return nil, util.Errf("cannot take %s: %v", meta.Name, err)
	}
	exec := res.Exec
	if fetched != nil {
		// Archive modes are authoritative (they survive a Windows destination). Keep only
		// files that were actually copied: .git and .DS_Store are skipped.
		exec = nil
		for _, p := range fetched.Paths {
			if fi, err := os.Lstat(filepath.Join(dest, filepath.FromSlash(p))); err == nil && fi.Mode().IsRegular() {
				exec = append(exec, p)
			}
		}
	}
	sort.Strings(exec)
	h, err := hash.Dir(dest)
	if err != nil {
		os.RemoveAll(dest)
		return nil, err
	}
	meta.SetIdentity(hash.Identity{Version: hash.Version, Content: h, Exec: exec, ExecHash: hash.ExecDigest(exec)})
	meta.FetchedAt = util.Now()
	if err := meta.Save(a.Home); err != nil {
		return nil, err
	}
	rep, err := scan.Scan(dest, a.scanOpts(meta.Name, nil, nil))
	if err != nil {
		return nil, err
	}
	return &Quarantined{Name: meta.Name, Hash: h, Meta: meta, Report: rep}, nil
}

func describe(found []skill.Found) string {
	var parts []string
	for _, f := range found {
		n := path.Base(f.Rel)
		if f.Meta != nil && f.Meta.Name != "" {
			n = f.Meta.Name
		}
		parts = append(parts, util.Clean(n))
	}
	return strings.Join(parts, ", ")
}

// UpdateResult describes one update check.
type UpdateResult struct {
	Name    string
	Changed bool
	Note    string
	Report  *scan.Report
	OldHash string
	NewHash string
	NewSHA  string
	// Applied is set when auto mode approved the changed version; otherwise a
	// changed skill is in quarantine. Safety is the score of the changed content.
	Applied bool
	Safety  scan.Safety
	// LocalChanges is set when the approved skill is a local fork or has
	// unadopted local edits, and upstream changed: the changed version is in
	// quarantine and is never auto-approved.
	LocalChanges bool `json:"local_changes,omitempty"`
}

// Update re-fetches approved skills; changed ones go to quarantine for review.
func (a *App) Update(ctx context.Context, names []string) ([]UpdateResult, error) {
	unlock, err := a.Home.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = ctxOrBackground(ctx)
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		for _, s := range lock.Skills {
			names = append(names, s.Name)
		}
	}
	if err := a.Home.Init(); err != nil {
		return nil, err
	}
	man, err := store.LoadManifest(a.Home)
	if err != nil {
		return nil, err
	}
	var out []UpdateResult
	for _, n := range names {
		ls, ok := lock.Get(n)
		if !ok {
			return nil, util.Errf("%q is not approved", n)
		}
		out = append(out, a.updateOne(ctx, man, ls))
	}
	return out, nil
}

// specFor is the source of a lock entry at ref (empty for the default branch).
func specFor(ls *store.LockSkill, ref string) (source.Spec, error) {
	if !strings.HasPrefix(ls.Location, "https://github.com/") {
		return source.Spec{Kind: source.KindLocal, Path: ls.Location}, nil
	}
	s := "github:" + strings.TrimPrefix(ls.Location, "https://github.com/")
	if ref != "" {
		s += "@" + ref
	}
	return source.Parse(s)
}

func (a *App) updateOne(ctx context.Context, man *store.Manifest, ls *store.LockSkill) UpdateResult {
	r := UpdateResult{Name: ls.Name, OldHash: ls.ContentHash}
	fail := func(format string, args ...any) UpdateResult {
		r.Note = fmt.Sprintf(format, args...)
		return r
	}
	if m, err := store.LoadMeta(a.Home, ls.Name); err == nil && m.Editing {
		return fail("you have an edit in progress; run `skigard adopt %s` to finalize it or `skigard reject %s` to discard it first", ls.Name, ls.Name)
	}
	spec, err := specFor(ls, ls.Ref)
	if err != nil {
		return fail("cannot update: %v", err)
	}
	f, err := a.Fetcher.Fetch(ctx, spec)
	if err != nil {
		return fail("fetch failed: %v", err)
	}
	defer f.Cleanup()
	dir := filepath.Join(f.Dir, filepath.FromSlash(ls.Subpath))
	if !isDir(dir) {
		return fail("the skill no longer exists at %q upstream", ls.Subpath)
	}
	for _, l := range f.SkippedLinks {
		if ls.Subpath == "" || l == ls.Subpath || strings.HasPrefix(l, ls.Subpath+"/") {
			return fail("upstream now contains a symlink (%s); refusing", util.Clean(l))
		}
	}
	meta := &store.Meta{Name: ls.Name, Source: ls.Source, Kind: spec.Kind, Location: ls.Location, Ref: ls.Ref,
		Subpath: ls.Subpath, Commit: f.Commit}
	q, err := a.place(dir, meta, f.ExecFor(ls.Subpath))
	if err != nil {
		return fail("%v", err)
	}
	r.NewHash, r.NewSHA = q.Hash, f.Commit
	fork, dirty := ls.UpstreamHash != "", a.hasLocalEdits(ls)
	if same, err := upstreamUnchanged(a.Home.QuarantineSkill(ls.Name), q.Meta, ls); err == nil && same {
		os.RemoveAll(a.Home.QuarantineSkill(ls.Name))
		os.Remove(a.Home.MetaPath(ls.Name))
		note := "up to date"
		if fork {
			note += " (local fork)"
		}
		if dirty {
			note += "; local edits are not adopted, run: skigard adopt " + ls.Name
		}
		return fail("%s", note)
	}
	r.Changed, r.Report = true, q.Report
	r.Safety = scan.Score(q.Report, "").At(man.Settings.EffectiveThreshold())
	r.Note = "changed upstream; review with: skigard diff " + ls.Name
	if fork || dirty {
		r.LocalChanges = true
		r.Note = "changed upstream; you have local changes. Compare with: skigard diff " + ls.Name
	}
	if m, _ := skill.Read(a.Home.QuarantineSkill(ls.Name)); m != nil && m.Name != ls.Name {
		r.Note += fmt.Sprintf(" (warning: the skill is now named %q)", util.Clean(m.Name))
	}
	if man.Settings.EffectiveMode() == store.ModeAuto && !r.LocalChanges {
		a.updateAuto(ctx, man, ls, q, &r)
	}
	return r
}

// upstreamUnchanged reports whether the freshly fetched skill in dir equals the
// upstream the entry was approved from. For a local fork the baseline is
// upstream_hash and only content is compared (the exec set is not recorded for
// upstream); otherwise it is sameAsApproved.
func upstreamUnchanged(dir string, fresh *store.Meta, ls *store.LockSkill) (bool, error) {
	if ls.UpstreamHash == "" {
		return sameAsApproved(dir, fresh, ls)
	}
	got, err := hash.DirVersion(dir, ls.Identity().Version)
	return err == nil && got == ls.UpstreamHash, err
}

// sameAsApproved reports whether the freshly fetched skill in dir is identical to
// the approved one: same bytes under the approved entry's hash version and, for
// version 2, the same execute-bit set. A version 1 entry is compared with the
// version 1 hash so an unchanged skill is not reported as changed.
func sameAsApproved(dir string, fresh *store.Meta, ls *store.LockSkill) (bool, error) {
	id := ls.Identity()
	got, err := hash.DirVersion(dir, id.Version)
	if err != nil || got != id.Content {
		return false, err
	}
	return id.Version < 2 || fresh.ExecHash == id.ExecHash, nil
}
