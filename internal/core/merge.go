package core

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ambierhq/skigard/internal/diff"
	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/review"
	"github.com/ambierhq/skigard/internal/safefs"
	"github.com/ambierhq/skigard/internal/skill"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// Merge modes recorded in the quarantine metadata.
const (
	MergePick  = "pick"
	MergeAgent = "agent"
)

// Chooser decides, for one differing file, which side to take. It is injected
// (like Confirmer) so tests drive it; the CLI passes a terminal-backed one.
type Chooser interface {
	// Choose returns one decision per fc.Choices() (true = take upstream, false = keep mine).
	// Returning an error (for example util.Cancelled) stages nothing.
	Choose(fc *diff.FileChange) ([]bool, error)
}

// AgentRunner runs a confined agent harness; review.Runner implements it.
type AgentRunner interface {
	Run(ctx context.Context, t review.Template, model string, in *review.Input) (string, review.Isolation, error)
}

// MergeOpts configures an agent merge.
type MergeOpts struct {
	Harness string
	Model   string
	Note    string
	// AllowPolicyOnly has the same meaning as for review: accept isolation by policy when no OS sandbox works.
	AllowPolicyOnly bool
}

// Merged is the result of staging a merge.
type Merged struct {
	*Quarantined
	Isolation review.Isolation
	Mode      string
}

type mergeSrc struct {
	meta  *store.Meta
	local string
	up    string
}

// mergePrepare checks the preconditions common to both merge modes. The caller holds the lock.
func (a *App) mergePrepare(name string) (*mergeSrc, []diff.FileChange, error) {
	if err := util.CheckName(name); err != nil {
		return nil, nil, err
	}
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, nil, err
	}
	if _, ok := lock.Get(name); !ok {
		return nil, nil, util.Errf("%q is not approved; merge brings a new upstream version into an approved skill", name)
	}
	local := a.Home.SkillDir(name)
	if !isDir(local) {
		return nil, nil, util.Errf("%q is missing from skills/; nothing to merge into", name)
	}
	meta, err := store.LoadMeta(a.Home, name)
	up := a.Home.QuarantineSkill(name)
	if err != nil || !isDir(up) {
		return nil, nil, util.Errf("there is no new upstream version of %s in quarantine; run: skigard update %s", name, name)
	}
	if meta.UpstreamHash != "" || meta.MergeMode != "" {
		return nil, nil, util.Errf("the quarantined %s is already a staged merge or adoption, not a fresh upstream version; reject it and run: skigard update %s", name, name)
	}
	changes, err := diff.Changes(local, up)
	if err != nil {
		return nil, nil, err
	}
	if len(changes) == 0 {
		return nil, nil, util.Policyf("nothing to merge: skills/%s and the quarantined upstream version are identical", name)
	}
	return &mergeSrc{meta: meta, local: local, up: up}, changes, nil
}

// PickMerge stages a hand-picked merge of the new upstream version into the
// local copy. Every decision comes from the injected Chooser, which defaults to
// "keep mine" in the CLI. It changes only quarantine; approval stays a separate human step.
func (a *App) PickMerge(name string, ch Chooser) (*Merged, error) {
	if ch == nil {
		return nil, util.Errf("merge --pick needs a chooser")
	}
	if a.RefuseInAgent {
		if h, ok := a.runningUnderAgent(); ok {
			return nil, util.Policyf("refusing to pick merge hunks: this looks like it is running under an agent harness (%s). Picking is a human step; to have an agent merge, use `skigard merge %s --with HARNESS`", h, name)
		}
	}
	unlock, err := a.Home.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	src, changes, err := a.mergePrepare(name)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "skigard-merge-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	tree := filepath.Join(tmp, "tree")
	if _, err := safefs.CopyTree(src.local, tree, a.Fetcher.Limits); err != nil {
		return nil, util.Errf("cannot read skills/%s: %v", name, err)
	}
	root, err := os.OpenRoot(tree)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	for i := range changes {
		fc := &changes[i]
		take, err := ch.Choose(fc)
		if err != nil {
			return nil, err
		}
		if len(take) != fc.Choices() {
			return nil, util.Errf("internal error: %d decisions for %d choices in %s", len(take), fc.Choices(), util.Clean(fc.Path))
		}
		data, exists := fc.Result(take)
		exec := fc.LocalExec
		if fc.Kind == diff.Added {
			exec = fc.UpstreamExec
		}
		if err := applyFile(root, fc.Path, data, exists, exec); err != nil {
			return nil, util.Errf("cannot stage %s: %v", util.Clean(fc.Path), err)
		}
	}
	q, err := a.stageMerge(src, tree, MergePick, "", review.Isolation{})
	if err != nil {
		return nil, err
	}
	return &Merged{Quarantined: q, Mode: MergePick}, nil
}

// AgentMerge has a confined agent harness merge the new upstream version into
// the local copy. The agent's output is untrusted: it is parsed strictly,
// validated, written through os.Root and scanned like any quarantined skill.
func (a *App) AgentMerge(ctx context.Context, name string, o MergeOpts) (*Merged, error) {
	ctx = ctxOrBackground(ctx)
	if err := review.Supported(); err != nil {
		return nil, err
	}
	t, ok := review.TemplateFor(o.Harness)
	if !ok {
		return nil, util.Errf("%q cannot run merges headlessly. Supported: claude-code, pi, omp. Or use --pick", o.Harness)
	}
	if !t.Verified {
		return nil, util.Errf("the %s template is unverified (%s); it cannot be used for merges. Use a verified harness or --pick", t.ID, t.Note)
	}
	unlock, err := a.Home.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	src, changes, err := a.mergePrepare(name)
	if err != nil {
		return nil, err
	}
	in, err := review.BuildMergeInput(src.local, src.up, name, o.Note)
	if err != nil {
		return nil, err
	}
	r := a.MergeRunner
	if r == nil {
		rr := a.Runner
		if rr.Environ == nil {
			rr.Environ = os.Environ()
		}
		rr.AllowPolicyOnly = o.AllowPolicyOnly
		r = rr
	}
	if a.ReviewProgress != nil {
		defer a.ReviewProgress(name, t.ID)()
	}
	raw, iso, err := r.Run(ctx, t, o.Model, in.Input)
	if err != nil {
		return nil, err
	}
	return a.finishAgentMerge(name, t, src, changes, in.Token, raw, iso)
}

// finishAgentMerge parses, validates and stages the agent's raw output. It
// runs nothing: the output is as untrusted here as it is from a real run, and
// the caller holds the lock.
func (a *App) finishAgentMerge(name string, t review.Template, src *mergeSrc, changes []diff.FileChange, token, raw string, iso review.Isolation) (*Merged, error) {
	out, err := review.ParseMerge(raw, token)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "skigard-merge-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	tree := filepath.Join(tmp, "tree")
	if err := a.buildAgentTree(src, changes, out, tree); err != nil {
		return nil, err
	}
	if m, bad := skill.Read(tree); m == nil {
		return nil, util.Errf("the merge output was invalid: %s", util.OneLine(strings.Join(bad, "; "), 200))
	} else if m.Name != name {
		return nil, util.Errf("the merge output was invalid: SKILL.md frontmatter name is %q, not %q", util.OneLine(m.Name, 60), name)
	}
	q, err := a.stageMerge(src, tree, MergeAgent, t.ID, iso)
	if err != nil {
		return nil, err
	}
	return &Merged{Quarantined: q, Isolation: iso, Mode: MergeAgent}, nil
}

// buildAgentTree starts from the local tree, adds upstream-only files, then
// applies the agent's FILE and DELETE entries. Files the agent did not mention
// keep the local version, else the upstream one.
func (a *App) buildAgentTree(src *mergeSrc, changes []diff.FileChange, out *review.MergeOutput, tree string) error {
	if _, err := safefs.CopyTree(src.local, tree, a.Fetcher.Limits); err != nil {
		return util.Errf("cannot read the local copy: %v", err)
	}
	root, err := os.OpenRoot(tree)
	if err != nil {
		return err
	}
	defer root.Close()
	byPath := map[string]*diff.FileChange{}
	for i := range changes {
		fc := &changes[i]
		byPath[fc.Path] = fc
		if fc.Kind == diff.Added {
			if err := applyFile(root, fc.Path, fc.Upstream, true, fc.UpstreamExec); err != nil {
				return util.Errf("the merge output was invalid: %v", err)
			}
		}
	}
	for _, f := range out.Files {
		exec := false
		if fc := byPath[f.Path]; fc != nil {
			exec = fc.LocalExec || (fc.Kind == diff.Added && fc.UpstreamExec)
		} else if fi, err := os.Lstat(filepath.Join(src.local, filepath.FromSlash(f.Path))); err == nil {
			exec = fi.Mode()&0o111 != 0
		}
		if err := applyFile(root, f.Path, f.Data, true, exec); err != nil {
			return util.Errf("the merge output was invalid: cannot write %s: %v", util.OneLine(f.Path, 80), err)
		}
	}
	for _, p := range out.Delete {
		if _, err := root.Lstat(p); err != nil {
			return util.Errf("the merge output was invalid: DELETE of %s, which does not exist", util.OneLine(p, 80))
		}
		if err := applyFile(root, p, nil, false, false); err != nil {
			return util.Errf("the merge output was invalid: cannot delete %s: %v", util.OneLine(p, 80), err)
		}
	}
	return nil
}

// stageMerge replaces the quarantined upstream tree with the merged tree. The
// upstream hash is taken from the pristine upstream before it is replaced, so
// approval can pin it as upstream_hash. On any failure the upstream version is restored.
func (a *App) stageMerge(src *mergeSrc, tree, mode, by string, iso review.Isolation) (*Quarantined, error) {
	up, err := hash.DirVersion(src.up, hash.Version)
	if err != nil {
		return nil, util.Errf("cannot hash the upstream version: %v", err)
	}
	dest := a.Home.QuarantineSkill(src.meta.Name)
	aside := dest + ".upstream-" + util.RandomHex(4)
	if err := os.Rename(dest, aside); err != nil {
		return nil, err
	}
	meta := *src.meta
	meta.Review = nil
	meta.UpstreamHash, meta.MergeMode, meta.MergedBy, meta.MergeIsolation = up, mode, by, ""
	if mode == MergeAgent {
		meta.MergeIsolation = iso.Mode
	}
	q, err := a.place(tree, &meta, nil)
	if err != nil {
		os.RemoveAll(dest)
		if rerr := os.Rename(aside, dest); rerr == nil {
			_ = src.meta.Save(a.Home)
		}
		return nil, err
	}
	os.RemoveAll(aside)
	return q, nil
}

// applyFile writes (or, with exists false, removes) one file through the root.
func applyFile(root *os.Root, rel string, data []byte, exists, exec bool) error {
	if !exists {
		if err := root.Remove(rel); err != nil && !os.IsNotExist(err) {
			return err
		}
		for dir := path.Dir(rel); dir != "."; dir = path.Dir(dir) {
			if root.Remove(dir) != nil { // only empty directories go
				break
			}
		}
		return nil
	}
	if dir := path.Dir(rel); dir != "." {
		if err := root.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := root.Remove(rel); err != nil && !os.IsNotExist(err) {
		return err
	}
	perm := os.FileMode(0o644)
	if exec {
		perm = 0o755
	}
	f, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return root.Chmod(rel, perm)
}

// mergeNote describes a staged merge for the approval screen and info.
func mergeNote(m *store.Meta) string {
	switch m.MergeMode {
	case MergePick:
		return "hand-picked merge of the new upstream version into your local copy"
	case MergeAgent:
		return fmt.Sprintf("merged by agent %s (isolation %s)", util.Clean(m.MergedBy), util.Clean(m.MergeIsolation))
	}
	return ""
}
