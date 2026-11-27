package core

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/review"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/term"
)

var (
	hashRE  = regexp.MustCompile(`sha256:[0-9a-f]{64}`)
	shortRE = regexp.MustCompile(`\b[0-9a-f]{12}\b`)
	srcRE   = regexp.MustCompile(`(?m)^(  source  +).*$`)
)

// normalize masks what differs between runs (temp paths, hashes) so the screen can be compared whole.
func normalize(s string) string {
	s = hashRE.ReplaceAllString(s, "sha256:HASH")
	s = shortRE.ReplaceAllString(s, "SHORT12345678")
	return srcRE.ReplaceAllString(s, "${1}SRC")
}

func approveSummary(t *testing.T, a *App, name string, o ApproveOpts) string {
	t.Helper()
	var summary string
	o.Confirm = func(s, _ string) error { summary = s; return nil }
	if _, err := a.Approve(name, o); err != nil {
		t.Fatal(err)
	}
	return summary
}

func TestSummaryGoldenLocalFork(t *testing.T) {
	a, _ := approvedStrict(t)
	editLocal(t, a, "mine\n")
	if _, err := a.Adopt(context.Background(), "alpha"); err != nil {
		t.Fatal(err)
	}
	got := normalize(approveSummary(t, a, "alpha", ApproveOpts{}))
	want := strings.Join([]string{
		"Approve skill \"alpha\"",
		"",
		"  source    SRC",
		"  hash      sha256:HASH",
		"  replaces  approved version SHORT12345678",
		"",
		"LOCAL FORK  your edited copy of an approved skill",
		"  baseline  upstream SHORT12345678",
		"  recorded  as mode local",
		"",
		"  scan: PASS  coverage: complete  risk: 0/100 (higher is riskier)  (1 files, 0 scripts)",
		"  note: the scanner is heuristic. A pass is not a safety certificate.",
		"",
		"safety score: 10/10 (higher is safer; ok; threshold 7; mode local)",
		"  - no findings",
		"",
		"review: none (run `skigard review` first for a second opinion)",
		"",
		"changes  comparing the original upstream version your edit started from with your edit in quarantine",
		"  --- approved/SKILL.md",
		"  +++ candidate/SKILL.md",
		""}, "\n")
	if !strings.HasPrefix(got, want) {
		t.Fatalf("got:\n%s\nwant prefix:\n%s", got, want)
	}
	if strings.HasSuffix(got, "\n") || strings.Contains(got, "\n\n\n") || strings.Count(got, "sha256:") != 1 {
		t.Errorf("stray blank lines or repeated hash:\n%s", got)
	}
}

func TestSummaryGoldenNewSkillWithReview(t *testing.T) {
	a, _ := newApp(t)
	setSettings(t, a, store.Settings{Mode: "strict"})
	src := t.TempDir()
	writeSkill(t, src, "alpha", nil)
	addLocal(t, a, src, AddOpts{Skill: "alpha"})
	meta, err := store.LoadMeta(a.Home, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	meta.Review = &review.Record{Reviewer: "pi", Verdict: "concerns", Coverage: "complete", Summary: "Fetches a URL.",
		InputHash: meta.ContentHash, Isolation: "seatbelt", Findings: []review.Finding{{Severity: "low", File: "SKILL.md", Line: 4, Why: "remote url"}}}
	if err := meta.Save(a.Home); err != nil {
		t.Fatal(err)
	}
	got := normalize(approveSummary(t, a, "alpha", ApproveOpts{}))
	want := strings.Join([]string{
		"Approve skill \"alpha\"",
		"",
		"  source  SRC",
		"  hash    sha256:HASH",
		"",
		"  scan: PASS  coverage: complete  risk: 0/100 (higher is riskier)  (1 files, 0 scripts)",
		"  note: the scanner is heuristic. A pass is not a safety certificate.",
		"",
		"safety score: 8/10 (higher is safer; ok; threshold 7; mode human)",
		"  - the reviewer raised concerns",
		"",
		"review: CONCERNS (pi, advisory; coverage complete)",
		"  Fetches a URL.",
		"  isolation  seatbelt (OS sandbox)",
		"  [low] SKILL.md:4 remote url",
		"",
		"changes  a new skill: every file is shown as added",
		""}, "\n")
	if !strings.HasPrefix(got, want) {
		t.Fatalf("got:\n%s\nwant prefix:\n%s", got, want)
	}
}

func TestSummaryStyledKeepsTextAndEscapesOnlyOnStyle(t *testing.T) {
	a, _ := approvedStrict(t)
	editLocal(t, a, "mine\n")
	if _, err := a.Adopt(context.Background(), "alpha"); err != nil {
		t.Fatal(err)
	}
	a.Style = term.Style{On: true}
	got := approveSummary(t, a, "alpha", ApproveOpts{})
	if !strings.Contains(got, "\x1b[") || !strings.Contains(got, "LOCAL FORK") {
		t.Errorf("no styling:\n%q", got)
	}
}

// diff and approve word the comparison the same way.
func TestComparePhraseSharedByDiffAndSummary(t *testing.T) {
	a, _ := approvedStrict(t)
	editLocal(t, a, "mine\n")
	if _, err := a.Adopt(context.Background(), "alpha"); err != nil {
		t.Fatal(err)
	}
	d, err := a.DiffAgainst("alpha")
	if err != nil || d.Phrase != "comparing the original upstream version your edit started from with your edit in quarantine" {
		t.Fatalf("%+v %v", d, err)
	}
	if got := approveSummary(t, a, "alpha", ApproveOpts{}); !strings.Contains(got, "changes  "+d.Phrase+"\n") {
		t.Errorf("approve does not use the diff phrase:\n%s", got)
	}
}
