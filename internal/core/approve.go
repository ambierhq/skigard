package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ambierhq/skigard/internal/diff"
	"github.com/ambierhq/skigard/internal/harness"
	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/review"
	"github.com/ambierhq/skigard/internal/safefs"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/skill"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// Confirmer shows the summary to a human and returns nil only if they
// explicitly confirm by typing want. Production code reads /dev/tty.
type Confirmer func(summary, want string) error

// ApproveOpts control an approval.
type ApproveOpts struct {
	Override      bool
	Reason        string // required with Override
	Accept        string // accept all current low/medium findings with this reason
	AcceptExpires string // optional YYYY-MM-DD
	By            string
	Confirm       Confirmer
}

// Approved is the outcome of an approval.
type Approved struct {
	Name   string
	Hash   string
	Record string
	Report *scan.Report
}

// ReviewInfo returns the review attached to a quarantined skill if it is
// still valid for the current content.
func ReviewInfo(meta *store.Meta) (rec *review.Record, stale bool) {
	if meta.Review == nil {
		return nil, false
	}
	if meta.Review.InputHash != meta.ContentHash {
		return nil, true
	}
	return meta.Review, false
}

// Approve moves a quarantined skill into the approved store after a human
// confirms. It never runs without Confirm and refuses inside an agent harness.
func (a *App) Approve(name string, o ApproveOpts) (*Approved, error) {
	unlock, err := a.Home.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := util.CheckName(name); err != nil {
		return nil, err
	}
	if o.Confirm == nil {
		return nil, util.Errf("approve needs an interactive confirmation")
	}
	if a.RefuseInAgent {
		if h, ok := a.runningUnderAgent(); ok {
			return nil, util.Policyf("refusing to approve: this looks like it is running under an agent harness (%s). Approval is a human step: open a normal terminal and run `skigard approve %s` there", h, name)
		}
	}
	meta, dir, err := a.loadQuarantined(name)
	if err != nil {
		return nil, err
	}
	man, err := store.LoadManifest(a.Home)
	if err != nil {
		return nil, err
	}
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, err
	}
	rep, err := scan.Scan(dir, a.scanOpts(name, nil, nil))
	if err != nil {
		return nil, err
	}
	var accepted []scan.Accepted
	if o.Accept != "" {
		for _, f := range rep.Findings {
			if f.Severity.Rank() <= scan.Medium.Rank() {
				accepted = append(accepted, scan.Accepted{Key: f.Key, Reason: o.Accept, Expires: o.AcceptExpires})
			}
		}
		if rep, err = scan.Scan(dir, a.scanOpts(name, accepted, nil)); err != nil {
			return nil, err
		}
	}
	rev, stale := ReviewInfo(meta)
	var why []string
	if rep.Verdict == scan.VerdictBlock {
		why = append(why, "the scanner verdict is BLOCK")
	}
	if rep.Coverage != scan.CoverageComplete {
		why = append(why, "scan coverage is incomplete")
	}
	if rev != nil && rev.Verdict == review.Fail {
		why = append(why, "the reviewer returned FAIL")
	}
	if len(why) > 0 && (!o.Override || strings.TrimSpace(o.Reason) == "") {
		return nil, util.Policyf("cannot approve %s: %s. If you have read the skill and accept the risk, rerun with --override --reason \"why\"", name, strings.Join(why, "; "))
	}
	if man.Settings.RequireReview && rev == nil {
		return nil, util.Policyf("this home requires a review before approval; run `skigard review %s`", name)
	}
	by := o.By
	if by == "" {
		by = a.User
	}
	if by == store.AutoApprover {
		return nil, util.Errf("%q is reserved for automatic approvals; use another name", store.AutoApprover)
	}
	prev, replacing := lock.Get(name)
	d, err := a.compare(meta, replacing, dir)
	if err != nil {
		return nil, err
	}
	sf := scan.Score(rep, reviewVerdict(rev)).At(man.Settings.EffectiveThreshold())
	summary := a.summary(meta, rep, rev, stale, d, prev, why, o, sf)
	if err := o.Confirm(summary, name); err != nil {
		return nil, err
	}
	approval := store.ApprovalRec{By: by, At: util.Now(), Override: o.Override && len(why) > 0, Reason: strings.TrimSpace(o.Reason), Accepted: accepted,
		Mode: store.ApprovalHuman, Score: sf.Score, ScoreVersion: sf.Version}
	if isFork(meta) {
		approval.Mode = store.ApprovalLocal
	}
	recPath, err := a.promote(meta, dir, rep, rev, approval, lock)
	if err != nil {
		return nil, err
	}
	a.syncMarketplace(man)
	return &Approved{Name: name, Hash: meta.ContentHash, Record: recPath, Report: rep}, nil
}

// loadQuarantined returns a quarantined skill's metadata and directory after
// checking that its bytes still match what was recorded at fetch time and that
// it is a valid skill.
func (a *App) loadQuarantined(name string) (*store.Meta, string, error) {
	meta, err := store.LoadMeta(a.Home, name)
	if err != nil {
		return nil, "", err
	}
	if err := refuseEditing(meta); err != nil {
		return nil, "", err
	}
	dir := a.Home.QuarantineSkill(name)
	st, err := hash.Check(dir, meta.Identity())
	if err != nil {
		return nil, "", util.Errf("cannot hash %s: %v", name, err)
	}
	if !st.ContentOK {
		return nil, "", util.Policyf("%s changed in quarantine after it was fetched (hash %s, expected %s); run `skigard reject %s` and add it again", name, hash.Short(st.Got, 12), hash.Short(meta.ContentHash, 12), name)
	}
	if !st.ExecOK {
		return nil, "", util.Policyf("%s changed in quarantine after it was fetched (executable files differ from those recorded at fetch time); run `skigard reject %s` and add it again", name, name)
	}
	if m, problems := skill.Read(dir); len(problems) > 0 || m.Name != name {
		return nil, "", util.Policyf("%s is not a valid skill: %s", name, strings.Join(append(problems, nameMismatch(m, name)...), "; "))
	}
	return meta, dir, nil
}

// promote is the shared tail of every approval path (human, auto, forced): it
// stages the quarantined skill, re-verifies the hash, swaps it into skills/,
// writes the record bundle and the lock entry, and clears quarantine. The
// caller holds the home lock and has already made the decision.
func (a *App) promote(meta *store.Meta, dir string, rep *scan.Report, rev *review.Record, approval store.ApprovalRec, lock *store.Lock) (string, error) {
	name := meta.Name
	// Stage on the same filesystem, verify, then swap atomically.
	if err := os.MkdirAll(a.Home.SkillsDir(), 0o755); err != nil {
		return "", err
	}
	stage := filepath.Join(a.Home.SkillsDir(), ".staging-"+util.RandomHex(6))
	defer func() { _ = util.RemoveAll(stage) }()
	if _, err := safefs.CopyTree(dir, stage, safefs.DefaultLimits()); err != nil {
		return "", err
	}
	if err := os.Chmod(stage, 0o755); err != nil {
		return "", err
	}
	if st, err := hash.Check(stage, meta.Identity()); err != nil || !st.OK() {
		return "", util.Policyf("staged copy does not match the reviewed hash; nothing was approved")
	}
	final := a.Home.SkillDir(name)
	var backup string
	if _, err := os.Lstat(final); err == nil {
		backup = filepath.Join(a.Home.SkillsDir(), ".old-"+util.RandomHex(6))
		if err := os.Rename(final, backup); err != nil {
			return "", err
		}
	}
	if err := os.Rename(stage, final); err != nil {
		if backup != "" {
			_ = os.Rename(backup, final) // best-effort rollback; the original error is returned
		}
		return "", err
	}
	// A deterrent against editing the approved copy in place (installs are symlinks into
	// it); the lock hash is the real control. Best effort: failing to chmod is not fatal.
	_ = util.MakeReadOnly(final)
	bundle := &store.Bundle{Skill: name, Hash: meta.ContentHash, Source: meta.Source, Commit: meta.Commit, Scan: rep, Review: rev, Approval: approval}
	recPath, err := store.SaveBundle(a.Home, bundle, hash.Short(meta.ContentHash, 12))
	if err != nil {
		return "", err
	}
	ls := store.LockSkill{
		Name: name, Source: meta.Source, Location: meta.Location, Ref: meta.Ref, Subpath: meta.Subpath, Commit: meta.Commit,
		Scan:     store.ScanRec{Tool: rep.Tool, Rules: rep.RulesVersion, Verdict: rep.Verdict, Coverage: rep.Coverage, Risk: rep.Risk, Findings: len(rep.Findings)},
		Approval: approval,
	}
	ls.SetIdentity(meta.Identity())
	if isFork(meta) {
		ls.UpstreamHash = meta.UpstreamHash
	}
	if rev != nil {
		ls.Review = &store.ReviewRec{Reviewer: rev.Reviewer, Verdict: rev.Verdict, Coverage: rev.Coverage, Record: recPath}
	}
	lock.Put(ls)
	if err := lock.Save(a.Home); err != nil {
		return "", err
	}
	if backup != "" {
		_ = util.RemoveAll(backup) // the old approved tree is read-only
	}
	os.RemoveAll(dir)
	os.RemoveAll(a.Home.BasePath(name))
	os.Remove(a.Home.MetaPath(name))
	return recPath, nil
}

// isFork reports whether a quarantined skill is a local fork staged by adopt:
// its content differs from the upstream baseline it records.
func isFork(meta *store.Meta) bool {
	return meta.UpstreamHash != "" && meta.UpstreamHash != meta.ContentHash
}

// hasLocalEdits reports whether skills/<name> exists and differs from the lock identity.
func (a *App) hasLocalEdits(ls *store.LockSkill) bool {
	dir := a.Home.SkillDir(ls.Name)
	if !isDir(dir) {
		return false
	}
	st, err := hash.Check(dir, ls.Identity())
	return err == nil && !st.OK()
}

// reviewVerdict is the verdict of a review record, or "" when there is none.
func reviewVerdict(rev *review.Record) string {
	if rev == nil {
		return ""
	}
	return rev.Verdict
}

func nameMismatch(m *skill.Meta, name string) []string {
	if m != nil && m.Name != name {
		return []string{fmt.Sprintf("frontmatter name %q does not match %q", util.Clean(m.Name), name)}
	}
	return nil
}

func (a *App) summary(meta *store.Meta, rep *scan.Report, rev *review.Record, stale bool, d comparison, prev *store.LockSkill, why []string, o ApproveOpts, sf scan.Safety) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Approve skill %q\n", meta.Name)
	fmt.Fprintf(&b, "  source:  %s\n", util.Clean(meta.Source))
	if meta.Commit != "" {
		fmt.Fprintf(&b, "  commit:  %s\n", meta.Commit)
	}
	fmt.Fprintf(&b, "  hash:    %s\n", meta.ContentHash)
	if prev != nil {
		fmt.Fprintf(&b, "  replaces approved version %s\n", hash.Short(prev.ContentHash, 12))
	}
	if prev != nil && isFork(meta) {
		fmt.Fprintf(&b, "LOCAL FORK  your edited copy of an approved skill; baseline upstream %s, recorded as mode local\n", hash.Short(meta.UpstreamHash, 12))
	} else if prev != nil && (prev.UpstreamHash != "" || a.hasLocalEdits(prev)) {
		b.WriteString("WARNING  approving replaces your local edits to this skill with the new upstream version\n")
	}
	if d.None {
		b.WriteString("NO COMPARISON  this is your in-place edit of an approved skill and the original upstream could not be rebuilt (see `skigard adopt`); read every file in quarantine before you approve\n")
	}
	if n := mergeNote(meta); n != "" {
		fmt.Fprintf(&b, "  merge:   %s\n", util.Clean(n))
	}
	if meta.MergeMode == MergeAgent {
		b.WriteString("WARNING  the merge agent's output is untrusted; read the diff against skills/ below\n")
	}
	scan.WriteText(&b, rep)
	fmt.Fprintf(&b, "safety score: %d/10 (%s; threshold %d): %s\n", sf.Score, sf.Tier, sf.Threshold, strings.Join(sf.Reasons, "; "))
	switch {
	case rev != nil:
		fmt.Fprintf(&b, "review (%s, advisory): %s, coverage %s\n  %s\n", util.Clean(rev.Reviewer), strings.ToUpper(rev.Verdict), rev.Coverage, util.Clean(rev.Summary))
		fmt.Fprintf(&b, "  isolation: %s\n", isolationLine(rev))
		for _, f := range rev.Findings {
			fmt.Fprintf(&b, "    [%s] %s:%d %s\n", f.Severity, util.Clean(f.File), f.Line, util.Clean(f.Why))
		}
	case stale:
		b.WriteString("review: a review exists but is for different content (ignored)\n")
	default:
		b.WriteString("review: none (run `skigard review` first for a second opinion)\n")
	}
	if len(why) > 0 {
		fmt.Fprintf(&b, "OVERRIDE: %s\n  reason given: %s\n", strings.Join(why, "; "), util.Clean(o.Reason))
	}
	if o.Accept != "" {
		fmt.Fprintf(&b, "accepting current low/medium findings: %s\n", util.Clean(o.Accept))
	}
	if d.Text != "" {
		lines := strings.Split(strings.TrimRight(d.Text, "\n"), "\n")
		const max = 300
		fmt.Fprintf(&b, "changes  %s\n", d.Phrase())
		for i, l := range lines {
			if i >= max {
				fmt.Fprintf(&b, "  ... %d more lines (run `skigard diff %s`)\n", len(lines)-max, meta.Name)
				break
			}
			b.WriteString("  " + l + "\n")
		}
	}
	return b.String()
}

// comparison is what an approval or `diff` compares a quarantined skill with.
type comparison struct {
	Text     string // the unified diff
	Label    string // what Text is against; empty for an ordinary new or updated skill
	None     bool   // no baseline is available: an empty Text means nothing was compared
	Approved bool   // an approved copy exists in skills/ (an update, not a new skill)
	Edit     bool   // the quarantined side is the user's own edit rather than a fetched version
}

// Phrase says what is compared with what. `diff` and the approval screen both use it, so they
// cannot word it differently.
func (c comparison) Phrase() string {
	switch {
	case c.Label != "" && c.Edit:
		return "comparing " + c.Label + " with your edit in quarantine"
	case c.Label != "":
		return "comparing " + c.Label + " with the new version in quarantine"
	case c.Approved:
		return "comparing the approved version in skills/ with the new version in quarantine"
	}
	return "a new skill: every file is shown as added"
}

// compare diffs the quarantined dir against the right baseline: the pristine
// approved copy in skills/ for an edit or a fork, the rebuilt upstream for an
// in-place adopt, and skills/ (when approved) for everything else.
func (a *App) compare(meta *store.Meta, approved bool, dir string) (comparison, error) {
	c := comparison{Approved: approved, Edit: meta.Editing || meta.InPlace || (isFork(meta) && meta.MergeMode == "")}
	old := ""
	if approved {
		old = a.Home.SkillDir(meta.Name)
	}
	switch {
	case meta.InPlace:
		base := a.Home.BasePath(meta.Name)
		if !isDir(base) {
			c.None = true
			return c, nil
		}
		old, c.Label = base, "the original upstream version your edit started from"
	case approved && (meta.Editing || (isFork(meta) && meta.MergeMode == "")):
		c.Label = "the approved version in skills/"
	}
	var err error
	c.Text, err = diff.Dirs(old, dir)
	return c, err
}

// DiffResult is `diff NAME`: the unified diff and what it is against.
type DiffResult struct {
	Text string
	// Against names the baseline for an edit or local fork ("" for an ordinary update).
	Against string
	// Phrase is the one-line "comparing X with Y" description shared with the approval screen.
	Phrase string
	// NoBaseline is set when the baseline could not be rebuilt for an in-place edit: Text
	// is empty because nothing was compared, not because nothing changed.
	NoBaseline bool
	// Approved is set when skills/ holds an approved copy the text was compared with.
	Approved bool
}

// DiffAgainst shows quarantined changes against the right baseline and says which.
func (a *App) DiffAgainst(name string) (*DiffResult, error) {
	if err := util.CheckName(name); err != nil {
		return nil, err
	}
	meta, err := store.LoadMeta(a.Home, name)
	if err != nil {
		return nil, err
	}
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, err
	}
	_, ok := lock.Get(name)
	c, err := a.compare(meta, ok, a.Home.QuarantineSkill(name))
	if err != nil {
		return nil, err
	}
	return &DiffResult{Text: c.Text, Against: c.Label, Phrase: c.Phrase(), Approved: c.Approved, NoBaseline: c.None}, nil
}

// Diff shows quarantined changes against the approved version.
func (a *App) Diff(name string) (string, error) {
	r, err := a.DiffAgainst(name)
	if err != nil {
		return "", err
	}
	return r.Text, nil
}

// refuseEditing stops approval-side flows on a working copy that `adopt` has not finalized.
func refuseEditing(meta *store.Meta) error {
	if meta.Editing {
		return util.Policyf("%s is still an editable working copy; run `skigard adopt %s` to finalize it, or `skigard reject %s` to discard", meta.Name, meta.Name, meta.Name)
	}
	return nil
}

// isolationLine describes how the reviewer was confined, for the approval screen.
func isolationLine(r *review.Record) string {
	switch r.Isolation {
	case "":
		return "not recorded (manual review, or recorded before isolation was tracked)"
	case review.IsoPolicyOnly:
		return "policy-only (WEAKER: tools disabled by flag, but no OS sandbox; the reviewer could read your files and reach the network)"
	}
	s := util.Clean(r.Isolation) + " (OS sandbox)"
	if r.IsolationNote != "" {
		s += "; " + util.Clean(r.IsolationNote)
	}
	return s
}

// runningUnderAgent combines the environment check with the ancestor walk.
// Both are best effort; see docs/threat-model.md.
func (a *App) runningUnderAgent() (string, bool) {
	if reg, err := harness.Registry(nil); err == nil {
		if e := harness.Running(reg, a.Env.Getenv); e != nil {
			return e.Name, true
		}
	}
	if _, ok := harness.InAgent(a.Env.Getenv); ok {
		return "an agent", true
	}
	if a.AgentAncestor != nil {
		return a.AgentAncestor()
	}
	return "", false
}
