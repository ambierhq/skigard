package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/ambierhq/skigard/internal/review"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/source"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// Decisions of AddAndInstall for one skill.
const (
	DecisionInstalled      = "installed"      // approved (human or auto) and installed
	DecisionNeedsApproval  = "needs-approval" // strict mode: left in quarantine for a human
	DecisionWarned         = "warned"         // auto mode: below threshold or a hard stop; left in quarantine
	DecisionForced         = "forced"         // installed with --force-install
	DecisionApprovedOnly   = "approved"       // approved but installing failed; see AddOutcome.Err
	DecisionFailed         = "failed"         // an error stopped the skill before any decision; see AddOutcome.Err
	reasonForceInstallNote = "forced install (--force-install)"
)

// AddInstallOpts control the one-step add: fetch, scan, score, decide, install.
type AddInstallOpts struct {
	Add AddOpts
	// Mode is "strict" or "auto"; empty uses the stored setting.
	Mode string
	// Threshold is 1..10; 0 uses the stored setting.
	Threshold int
	// Review runs the advisory reviewer (it also runs when settings.auto_review is on).
	Review     bool
	ReviewOpts ReviewOpts
	// ForceInstall installs whatever the score. It needs a human: refused inside an agent.
	ForceInstall bool
	// Install says where to install (Names and Plugin are ignored).
	Install InstallOpts
	// Confirm is the human confirmation used in strict mode. nil, or running
	// inside an agent, leaves the skill in quarantine (needs-approval).
	Confirm Confirmer
	By      string
}

// AddOutcome is the result for one skill.
type AddOutcome struct {
	Name      string
	Hash      string
	Score     int
	Tier      string
	Safety    scan.Safety
	Mode      string // the mode applied: strict or auto
	Decision  string
	Reasons   []string
	Report    *scan.Report
	Review    *review.Record
	Installed []Installed
	Notes     []string
	// Err is set when a human declined the strict prompt (util.Cancelled) or the install step failed after approval.
	Err error
}

// AddAndInstall fetches a source into quarantine, scans it, optionally reviews
// it, scores it and then, depending on mode, either asks a human (strict) or
// installs what clears the threshold (auto). It takes the home lock once.
//
// A hard stop (scanner BLOCK or reviewer FAIL) is never installed unless
// ForceInstall is set. Inside an agent it refuses ForceInstall and any mode or
// threshold weaker than the stored settings, and strict mode never prompts.
func (a *App) AddAndInstall(ctx context.Context, src string, o AddInstallOpts) ([]AddOutcome, error) {
	unlock, err := a.Home.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = ctxOrBackground(ctx)
	if err := a.Home.Init(); err != nil {
		return nil, err
	}
	man, err := store.LoadManifest(a.Home)
	if err != nil {
		return nil, err
	}
	mode, th, err := a.effectivePolicy(man, o)
	if err != nil {
		return nil, err
	}
	_, inAgent := a.agentVar()
	spec, err := source.Parse(src)
	if err != nil {
		return nil, err
	}
	f, err := a.Fetcher.Fetch(ctx, spec)
	if err != nil {
		return nil, err
	}
	defer f.Cleanup()
	qs, err := a.quarantine(f, spec, o.Add)
	if err != nil {
		return nil, err
	}
	var out []AddOutcome
	for _, q := range qs {
		oc, err := a.decide(ctx, q, man, mode, th, inAgent, o)
		if err != nil && oc.Decision == "" {
			oc.Decision, oc.Err = DecisionFailed, err
		}
		out = append(out, oc)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// agentVar reports which agent harness skigard detects it is running under
// (environment markers, registry env, then ancestor processes), when
// RefuseInAgent is on. It is the same best-effort check `approve` uses.
func (a *App) agentVar() (string, bool) {
	if !a.RefuseInAgent {
		return "", false
	}
	return a.runningUnderAgent()
}

// effectivePolicy resolves mode and threshold and enforces that an agent can
// only make them stricter than the stored settings.
func (a *App) effectivePolicy(man *store.Manifest, o AddInstallOpts) (mode string, th int, err error) {
	storedMode, storedTh := man.Settings.EffectiveMode(), man.Settings.EffectiveThreshold()
	mode, th = storedMode, storedTh
	if o.Mode != "" {
		if o.Mode != store.ModeStrict && o.Mode != store.ModeAuto {
			return "", 0, util.Errf("--mode must be strict or auto")
		}
		mode = o.Mode
	}
	if o.Threshold != 0 {
		if o.Threshold < 1 || o.Threshold > 10 {
			return "", 0, util.Errf("--threshold must be 1..10")
		}
		th = o.Threshold
	}
	if v, ok := a.agentVar(); ok {
		switch {
		case o.ForceInstall:
			return "", 0, util.Policyf("refusing --force-install: this looks like it is running under an agent harness (%s). Forcing an install is a human step: run it in a normal terminal", v)
		case storedMode == store.ModeStrict && mode == store.ModeAuto:
			return "", 0, util.Policyf("refusing --mode auto: running under an agent harness (%s) and the stored mode is strict. An agent cannot weaken the mode; a human can run `skigard config set mode auto`", v)
		case th < storedTh:
			return "", 0, util.Policyf("refusing --threshold %d: running under an agent harness (%s) and the stored threshold is %d. An agent cannot lower the bar", th, v, storedTh)
		}
	}
	return mode, th, nil
}

func (a *App) decide(ctx context.Context, q *Quarantined, man *store.Manifest, mode string, th int, inAgent bool, o AddInstallOpts) (AddOutcome, error) {
	name := q.Name
	oc := AddOutcome{Name: name, Hash: q.Hash, Mode: mode, Report: q.Report}
	var reviewNote string
	if o.Review || man.Settings.EffectiveAutoReview() {
		ro := o.ReviewOpts
		if ro.Harness == "" {
			ro.Harness = a.DefaultReviewHarness(man.Settings)
		}
		if ro.Harness == "" {
			if o.Review || man.Settings.AutoReview != nil {
				reviewNote = "review did not run: no reviewer harness set (use --review-harness or `skigard config`)"
			}
		} else if _, err := a.Review(ctx, name, ro); err != nil {
			reviewNote = "review did not run: " + util.Clean(err.Error())
		}
	}
	meta, dir, err := a.loadQuarantined(name)
	if err != nil {
		return oc, err
	}
	rev, stale := ReviewInfo(meta)
	oc.Review = rev
	rep := q.Report
	sf := scan.Score(rep, reviewVerdict(rev)).At(th)
	oc.Safety, oc.Score, oc.Tier = sf, sf.Score, sf.Tier
	oc.Reasons = append([]string(nil), sf.Reasons...)
	if reviewNote != "" {
		oc.Reasons = append(oc.Reasons, reviewNote)
	}
	needsReview := man.Settings.RequireReview && rev == nil

	forced := o.ForceInstall
	var decision string
	switch {
	case forced:
		decision = DecisionForced
	case sf.HardStop:
		oc.Reasons = append(oc.Reasons, "hard stop: only --force-install or a human `skigard approve --override` can install this")
	case needsReview:
		oc.Reasons = append(oc.Reasons, "this home requires a review before approval (settings.require_review)")
	case mode == store.ModeStrict:
		// handled below: a human must confirm
	case reviewNote != "":
		oc.Reasons = append(oc.Reasons, "not auto-installed because the requested review did not run")
	case sf.Score < th:
		oc.Reasons = append(oc.Reasons, fmt.Sprintf("score %d is below the threshold %d", sf.Score, th))
	default:
		decision = DecisionInstalled
	}

	approval := store.ApprovalRec{By: o.By, At: util.Now(), Score: sf.Score, Threshold: th, ScoreVersion: sf.Version}
	if approval.By == "" {
		approval.By = a.User
	}
	switch {
	case decision == DecisionForced:
		approval.Mode, approval.Override, approval.Reason = store.ApprovalForced, sf.HardStop || sf.Tier != scan.TierOK, reasonForceInstallNote
	case decision == DecisionInstalled:
		approval.Mode, approval.By = store.ApprovalAuto, store.AutoApprover
	case mode == store.ModeStrict && !sf.HardStop && !needsReview:
		// A human decides. Never prompt inside an agent, or without a confirmer.
		if o.Confirm == nil || inAgent {
			oc.Decision = DecisionNeedsApproval
			oc.Reasons = append(oc.Reasons, "strict mode: a human must confirm with `skigard approve "+name+"`")
			return oc, nil
		}
		lock, err := store.LoadLock(a.Home)
		if err != nil {
			return oc, err
		}
		prev, replacing := lock.Get(name)
		d, err := a.compare(meta, replacing, dir)
		if err != nil {
			return oc, err
		}
		summary := a.summary(meta, rep, rev, stale, d, prev, nil, ApproveOpts{}, sf)
		if err := o.Confirm(summary, name); err != nil {
			oc.Decision, oc.Err = DecisionNeedsApproval, err
			oc.Reasons = append(oc.Reasons, "not confirmed; still in quarantine")
			return oc, nil
		}
		approval.Mode = store.ApprovalHuman
		decision = DecisionInstalled
	default:
		if mode == store.ModeStrict {
			oc.Decision = DecisionNeedsApproval
		} else {
			oc.Decision = DecisionWarned
		}
		return oc, nil
	}

	// Resolve targets before approving so a missing harness leaves the skill in quarantine.
	iopts := o.Install
	iopts.Names, iopts.Plugin = []string{name}, ""
	if _, _, err := a.Targets(iopts); err != nil {
		return oc, err
	}
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return oc, err
	}
	if _, err := a.promote(meta, dir, rep, rev, approval, lock); err != nil {
		return oc, err
	}
	a.syncMarketplace(nil)
	oc.Decision = DecisionApprovedOnly
	inst, notes, err := a.install(iopts)
	oc.Installed, oc.Notes = inst, notes
	if err != nil {
		oc.Err = err
		return oc, err
	}
	oc.Decision = decision
	return oc, nil
}

// updateAuto applies auto-mode update rules to a changed skill that is now in
// quarantine. It sets r.Applied when the new version was approved, otherwise it
// leaves the skill in quarantine and says why in r.Note. The caller holds the home lock.
func (a *App) updateAuto(ctx context.Context, man *store.Manifest, ls *store.LockSkill, q *Quarantined, r *UpdateResult) {
	if ls.UpstreamHash != "" || a.hasLocalEdits(ls) {
		return // never replace local edits or a local fork without a human
	}
	th := man.Settings.EffectiveThreshold()
	var note []string
	if man.Settings.EffectiveAutoReview() {
		if h := a.DefaultReviewHarness(man.Settings); h != "" {
			if _, err := a.Review(ctx, ls.Name, ReviewOpts{Harness: h}); err != nil {
				note = append(note, "review did not run: "+util.Clean(err.Error()))
			}
		}
	}
	meta, dir, err := a.loadQuarantined(ls.Name)
	var rev *review.Record
	if err == nil {
		rev, _ = ReviewInfo(meta)
	}
	sf := scan.Score(q.Report, reviewVerdict(rev)).At(th)
	r.Safety = sf
	prevScore := previousScore(ls)
	switch {
	case err != nil:
		note = append(note, util.Clean(err.Error()))
	case sf.HardStop:
		note = append(note, "hard stop: "+strings.Join(sf.Reasons, "; "))
	case sf.Score < th:
		note = append(note, fmt.Sprintf("score %d is below the threshold %d", sf.Score, th))
	case sf.Score < prevScore:
		note = append(note, fmt.Sprintf("score %d is lower than the approved version's %d", sf.Score, prevScore))
	case man.Settings.RequireReview && rev == nil:
		note = append(note, "this home requires a review before approval")
	case len(note) > 0:
		note = append(note, "not auto-approved because the requested review did not run")
	}
	if err == nil && len(note) == 0 {
		lock, lerr := store.LoadLock(a.Home)
		if lerr == nil {
			ap := store.ApprovalRec{By: store.AutoApprover, At: util.Now(), Mode: store.ApprovalAuto, Score: sf.Score, Threshold: th, ScoreVersion: sf.Version}
			if _, lerr = a.promote(meta, dir, q.Report, rev, ap, lock); lerr == nil {
				a.syncMarketplace(man)
				r.Applied = true
				r.Note = fmt.Sprintf("updated and approved automatically (score %d/10, threshold %d); copy installs need `skigard sync`", sf.Score, th)
				return
			}
		}
		note = append(note, util.Clean(lerr.Error()))
	}
	r.Note = fmt.Sprintf("changed upstream; left in quarantine (%s). Review with: skigard diff %s, then skigard approve %s", strings.Join(note, "; "), ls.Name, ls.Name)
}

// previousScore is the score recorded at approval, or one reconstructed from the
// scan summary for locks written before scores existed.
func previousScore(ls *store.LockSkill) int {
	if ls.Approval.Score > 0 {
		return ls.Approval.Score
	}
	rv := ""
	if ls.Review != nil {
		rv = ls.Review.Verdict
	}
	return scan.Score(&scan.Report{Risk: ls.Scan.Risk, Verdict: ls.Scan.Verdict, Coverage: ls.Scan.Coverage}, rv).Score
}
