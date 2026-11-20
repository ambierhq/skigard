package scan

import "testing"

func TestScoreTable(t *testing.T) {
	cases := []struct {
		name    string
		rep     Report
		verdict string
		score   int
		tier    string
		hard    bool
	}{
		{"clean", Report{Verdict: VerdictPass, Coverage: CoverageComplete}, "", 10, TierOK, false},
		{"risk 12", Report{Risk: 12, Verdict: VerdictReview, Coverage: CoverageComplete}, "", 9, TierOK, false},
		{"risk 13", Report{Risk: 13, Verdict: VerdictReview, Coverage: CoverageComplete}, "", 8, TierOK, false},
		{"risk 36 is 7", Report{Risk: 36, Verdict: VerdictReview, Coverage: CoverageComplete}, "", 7, TierOK, false},
		{"risk 37 is 6", Report{Risk: 37, Verdict: VerdictReview, Coverage: CoverageComplete}, "", 6, TierCaution, false},
		{"risk 100", Report{Risk: 100, Verdict: VerdictReview, Coverage: CoverageComplete}, "", 1, TierCaution, false},
		{"block caps at 3", Report{Risk: 0, Verdict: VerdictBlock, Coverage: CoverageComplete}, "", 3, TierBlocked, true},
		{"block high risk", Report{Risk: 100, Verdict: VerdictBlock, Coverage: CoverageComplete}, "", 1, TierBlocked, true},
		{"partial caps at 6", Report{Verdict: VerdictPass, Coverage: CoveragePartial}, "", 6, TierCaution, false},
		{"reviewer fail", Report{Verdict: VerdictPass, Coverage: CoverageComplete}, "fail", 3, TierBlocked, true},
		{"reviewer concerns", Report{Verdict: VerdictPass, Coverage: CoverageComplete}, "concerns", 8, TierOK, false},
		{"concerns + partial", Report{Verdict: VerdictPass, Coverage: CoveragePartial}, "concerns", 4, TierCaution, false},
		{"reviewer pass never raises", Report{Risk: 37, Verdict: VerdictReview, Coverage: CoverageComplete}, "pass", 6, TierCaution, false},
		{"concerns floor", Report{Risk: 100, Verdict: VerdictReview, Coverage: CoverageComplete}, "concerns", 1, TierCaution, false},
	}
	for _, c := range cases {
		got := Score(&c.rep, c.verdict)
		if got.Score != c.score || got.Tier != c.tier || got.HardStop != c.hard || got.Version != ScoreVersion || len(got.Reasons) == 0 {
			t.Errorf("%s: got %+v, want score %d tier %s hard %v", c.name, got, c.score, c.tier, c.hard)
		}
	}
}

func TestSafetyAt(t *testing.T) {
	s := Score(&Report{Risk: 37, Verdict: VerdictReview, Coverage: CoverageComplete}, "")
	if s.At(6).Tier != TierOK || s.At(7).Tier != TierCaution {
		t.Errorf("%+v", s)
	}
	h := Score(&Report{Verdict: VerdictBlock, Coverage: CoverageComplete}, "")
	if h.At(1).Tier != TierBlocked {
		t.Error("a hard stop must stay blocked at any threshold")
	}
}
