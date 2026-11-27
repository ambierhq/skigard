package core

import (
	"context"
	"os"
	"strings"

	"github.com/ambierhq/skigard/internal/harness"
	"github.com/ambierhq/skigard/internal/review"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// Reviewer is a harness that can review skills headlessly.
type Reviewer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Verified  bool   `json:"verified"`
	Running   bool   `json:"running_skigard"`
	Note      string `json:"note,omitempty"`
}

// Reviewers lists harnesses that have a reviewer template and are present.
func (a *App) Reviewers() ([]Reviewer, error) {
	ds, err := a.Detect()
	if err != nil {
		return nil, err
	}
	reg, _, _ := a.Registry()
	running := harness.Running(reg, a.Env.Getenv)
	var out []Reviewer
	for _, d := range ds {
		t, ok := review.TemplateFor(d.ID)
		if !ok {
			continue
		}
		r := Reviewer{ID: d.ID, Name: d.Name, Installed: d.Status == harness.Installed, Verified: t.Verified, Note: t.Note}
		if running != nil && running.ID == d.ID {
			r.Running = true
		}
		if err := review.Supported(); err != nil {
			r.Note = "automated review is disabled on this platform; use --manual"
		}
		out = append(out, r)
	}
	return out, nil
}

// DefaultReviewHarness is the reviewer used when none is chosen: the saved
// settings.review_harness, else the first installed, verified reviewer in
// registry order. Empty when there is none. It is never written to the settings.
func (a *App) DefaultReviewHarness(s store.Settings) string {
	if s.ReviewHarness != "" {
		return s.ReviewHarness
	}
	rv, err := a.Reviewers()
	if err != nil {
		return ""
	}
	for _, r := range rv {
		if r.Installed && r.Verified {
			return r.ID
		}
	}
	return ""
}

// ReviewOpts selects the reviewer.
type ReviewOpts struct {
	Harness         string
	Model           string
	AllowUnverified bool
	// AllowPolicyOnly accepts isolation by policy when no OS sandbox works. Advisory review only; approval never reads it.
	AllowPolicyOnly bool
}

// Review runs a harness as a tool-less reviewer (isolated by policy, not by an OS sandbox) and stores the result.
func (a *App) Review(ctx context.Context, name string, o ReviewOpts) (*review.Record, error) {
	ctx = ctxOrBackground(ctx)
	if err := review.Supported(); err != nil {
		return nil, err
	}
	if err := util.CheckName(name); err != nil {
		return nil, err
	}
	meta, err := store.LoadMeta(a.Home, name)
	if err != nil {
		return nil, err
	}
	t, ok := review.TemplateFor(o.Harness)
	if !ok {
		return nil, util.Errf("%q cannot run reviews headlessly yet. Supported: claude-code, pi, omp, gemini-cli. Or use --manual", o.Harness)
	}
	if !t.Verified && !o.AllowUnverified {
		return nil, util.Errf("the %s reviewer template is unverified (%s); pass --allow-unverified to use it anyway", t.ID, t.Note)
	}
	dir := a.Home.QuarantineSkill(name)
	rep, err := scan.Scan(dir, a.scanOpts(name, nil, nil))
	if err != nil {
		return nil, err
	}
	in, err := review.BuildInput(dir, name, rep)
	if err != nil {
		return nil, err
	}
	r := a.Runner
	if r.Environ == nil {
		r.Environ = os.Environ()
	}
	r.AllowPolicyOnly = o.AllowPolicyOnly
	if a.ReviewProgress != nil {
		defer a.ReviewProgress(name, t.ID)()
	}
	raw, iso, err := r.Run(ctx, t, o.Model, in)
	if err != nil {
		return nil, err
	}
	rec, err := review.Parse(raw, in)
	if err != nil {
		return nil, err
	}
	rec.Reviewer, rec.Model, rec.InputHash, rec.PromptHash = t.ID, o.Model, meta.ContentHash, in.PromptHash
	rec.Isolation, rec.IsolationNote = iso.Mode, iso.Note
	rec.At = util.Now()
	rec.Raw = util.Clean(truncate(raw, 32<<10))
	meta.Review = rec
	return rec, meta.Save(a.Home)
}

// RecordManual stores a review made by a person or an agent working by hand.
func (a *App) RecordManual(name, verdict, summary, by string) (*review.Record, error) {
	meta, err := store.LoadMeta(a.Home, name)
	if err != nil {
		return nil, err
	}
	switch verdict {
	case review.Pass, review.Concerns, review.Fail:
	default:
		return nil, util.Errf("--verdict must be pass, concerns or fail")
	}
	if strings.TrimSpace(summary) == "" {
		return nil, util.Errf("--summary is required")
	}
	if by == "" {
		by = a.User
	}
	rec := &review.Record{Reviewer: "manual", By: by, Verdict: verdict, Summary: util.OneLine(summary, 1200),
		Coverage: scan.CoverageComplete, InputHash: meta.ContentHash, At: util.Now(), Advisory: true}
	meta.Review = rec
	return rec, meta.Save(a.Home)
}

// ManualPrompt returns the instructions and material for reviewing by hand.
func (a *App) ManualPrompt(name string) (string, error) {
	if _, err := store.LoadMeta(a.Home, name); err != nil {
		return "", err
	}
	dir := a.Home.QuarantineSkill(name)
	rep, err := scan.Scan(dir, a.scanOpts(name, nil, nil))
	if err != nil {
		return "", err
	}
	in, err := review.BuildInput(dir, name, rep)
	if err != nil {
		return "", err
	}
	return review.SystemPrompt + "\n\n" + in.Prompt, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
