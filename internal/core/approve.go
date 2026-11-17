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
		if v, ok := harness.InAgent(a.Env.Getenv); ok {
			return nil, util.Policyf("refusing to approve: %s is set, so this looks like it is running inside an agent. Approval is a human step: open a normal terminal and run `skigard approve %s` there", v, name)
		}
	}
	meta, err := store.LoadMeta(a.Home, name)
	if err != nil {
		return nil, err
	}
	dir := a.Home.QuarantineSkill(name)
	cur, err := hash.Dir(dir)
	if err != nil {
		return nil, util.Errf("cannot hash %s: %v", name, err)
	}
	if cur != meta.ContentHash {
		return nil, util.Policyf("%s changed in quarantine after it was fetched (hash %s, expected %s); run `skigard reject %s` and add it again", name, hash.Short(cur, 12), hash.Short(meta.ContentHash, 12), name)
	}
	if m, problems := skill.Read(dir); len(problems) > 0 || m.Name != name {
		return nil, util.Policyf("%s is not a valid skill: %s", name, strings.Join(append(problems, nameMismatch(m, name)...), "; "))
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
	prev, replacing := lock.Get(name)
	oldDir := ""
	if replacing {
		oldDir = a.Home.SkillDir(name)
	}
	d, err := diff.Dirs(oldDir, dir)
	if err != nil {
		return nil, err
	}
	summary := a.summary(meta, rep, rev, stale, d, prev, why, o)
	if err := o.Confirm(summary, name); err != nil {
		return nil, err
	}

	// Stage on the same filesystem, verify, then swap atomically.
	if err := os.MkdirAll(a.Home.SkillsDir(), 0o755); err != nil {
		return nil, err
	}
	stage := filepath.Join(a.Home.SkillsDir(), ".staging-"+util.RandomHex(6))
	defer os.RemoveAll(stage)
	if _, err := safefs.CopyTree(dir, stage, safefs.DefaultLimits()); err != nil {
		return nil, err
	}
	if err := os.Chmod(stage, 0o755); err != nil {
		return nil, err
	}
	if got, err := hash.Dir(stage); err != nil || got != meta.ContentHash {
		return nil, util.Policyf("staged copy does not match the reviewed hash; nothing was approved")
	}
	final := a.Home.SkillDir(name)
	var backup string
	if _, err := os.Lstat(final); err == nil {
		backup = filepath.Join(a.Home.SkillsDir(), ".old-"+util.RandomHex(6))
		if err := os.Rename(final, backup); err != nil {
			return nil, err
		}
	}
	if err := os.Rename(stage, final); err != nil {
		if backup != "" {
			os.Rename(backup, final)
		}
		return nil, err
	}
	approval := store.ApprovalRec{By: by, At: util.Now(), Override: o.Override && len(why) > 0, Reason: strings.TrimSpace(o.Reason), Accepted: accepted}
	bundle := &store.Bundle{Skill: name, Hash: meta.ContentHash, Source: meta.Source, Commit: meta.Commit, Scan: rep, Review: rev, Approval: approval}
	recPath, err := store.SaveBundle(a.Home, bundle, hash.Short(meta.ContentHash, 12))
	if err != nil {
		return nil, err
	}
	ls := store.LockSkill{
		Name: name, Source: meta.Source, Location: meta.Location, Ref: meta.Ref, Subpath: meta.Subpath, Commit: meta.Commit,
		ContentHash: meta.ContentHash,
		Scan:        store.ScanRec{Tool: rep.Tool, Rules: rep.RulesVersion, Verdict: rep.Verdict, Coverage: rep.Coverage, Risk: rep.Risk, Findings: len(rep.Findings)},
		Approval:    approval,
	}
	if rev != nil {
		ls.Review = &store.ReviewRec{Reviewer: rev.Reviewer, Verdict: rev.Verdict, Coverage: rev.Coverage, Record: recPath}
	}
	lock.Put(ls)
	if err := lock.Save(a.Home); err != nil {
		return nil, err
	}
	if backup != "" {
		os.RemoveAll(backup)
	}
	os.RemoveAll(dir)
	os.Remove(a.Home.MetaPath(name))
	return &Approved{Name: name, Hash: meta.ContentHash, Record: recPath, Report: rep}, nil
}

func nameMismatch(m *skill.Meta, name string) []string {
	if m != nil && m.Name != name {
		return []string{fmt.Sprintf("frontmatter name %q does not match %q", util.Clean(m.Name), name)}
	}
	return nil
}

func (a *App) summary(meta *store.Meta, rep *scan.Report, rev *review.Record, stale bool, d string, prev *store.LockSkill, why []string, o ApproveOpts) string {
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
	scan.WriteText(&b, rep)
	switch {
	case rev != nil:
		fmt.Fprintf(&b, "review (%s, advisory): %s, coverage %s\n  %s\n", util.Clean(rev.Reviewer), strings.ToUpper(rev.Verdict), rev.Coverage, util.Clean(rev.Summary))
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
	if d != "" {
		lines := strings.Split(strings.TrimRight(d, "\n"), "\n")
		const max = 300
		b.WriteString("changes:\n")
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

// Diff shows quarantined changes against the approved version.
func (a *App) Diff(name string) (string, error) {
	if err := util.CheckName(name); err != nil {
		return "", err
	}
	if _, err := store.LoadMeta(a.Home, name); err != nil {
		return "", err
	}
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return "", err
	}
	old := ""
	if _, ok := lock.Get(name); ok {
		old = a.Home.SkillDir(name)
	}
	return diff.Dirs(old, a.Home.QuarantineSkill(name))
}
