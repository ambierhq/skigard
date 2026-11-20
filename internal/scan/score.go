package scan

import "fmt"

// ScoreVersion changes whenever the scoring formula changes. It is recorded in
// the lock next to each score.
const ScoreVersion = "1"

// DefaultThreshold is the score at or above which a skill counts as "ok".
const DefaultThreshold = 7

// Tiers of a safety score.
const (
	TierOK      = "ok"      // score >= threshold and no hard stop
	TierCaution = "caution" // below the threshold
	TierBlocked = "blocked" // a hard stop: never auto-installed
)

// Reviewer verdicts understood by Score (same strings as the review package,
// which scan must not import).
const (
	reviewConcerns = "concerns"
	reviewFail     = "fail"
)

// Safety is a 1..10 heuristic score (10 = no concern). It is always shown with
// its Reasons, never alone, and it is a recommendation, not a guarantee.
type Safety struct {
	Score int `json:"score"`
	// Tier is blocked for a hard stop, otherwise ok or caution measured against
	// Threshold (DefaultThreshold unless At was used).
	Tier      string   `json:"tier"`
	Threshold int      `json:"threshold"`
	HardStop  bool     `json:"hard_stop"`
	Reasons   []string `json:"reasons"`
	Version   string   `json:"score_version"`
}

// At returns s judged against threshold.
func (s Safety) At(threshold int) Safety {
	s.Threshold = threshold
	switch {
	case s.HardStop:
		s.Tier = TierBlocked
	case s.Score >= threshold:
		s.Tier = TierOK
	default:
		s.Tier = TierCaution
	}
	return s
}

// Score turns a scan report and an optional reviewer verdict ("", "pass",
// "concerns" or "fail") into a Safety. It is pure. The reviewer can only lower
// the score, never raise it.
func Score(rep *Report, reviewVerdict string) Safety {
	s := Safety{Version: ScoreVersion}
	risk := 0
	if rep != nil {
		risk = rep.Risk
	}
	if risk < 0 {
		risk = 0
	}
	score := 10 - (risk+11)/12
	if risk > 0 {
		s.Reasons = append(s.Reasons, fmt.Sprintf("scanner risk %d/100", risk))
	}
	capTo := func(max int, why string) {
		if score > max {
			score = max
		}
		s.Reasons = append(s.Reasons, why)
	}
	if rep != nil && rep.Verdict == VerdictBlock {
		s.HardStop = true
		capTo(3, "scanner verdict is BLOCK (high or critical finding)")
	}
	if rep != nil && rep.Coverage != CoverageComplete {
		capTo(6, "scan coverage is incomplete")
	}
	switch reviewVerdict {
	case reviewFail:
		s.HardStop = true
		capTo(3, "the reviewer returned FAIL")
	case reviewConcerns:
		score -= 2
		s.Reasons = append(s.Reasons, "the reviewer raised concerns")
	}
	if score < 1 {
		score = 1
	}
	if score > 10 {
		score = 10
	}
	s.Score = score
	if len(s.Reasons) == 0 {
		s.Reasons = []string{"no findings"}
	}
	return s.At(DefaultThreshold)
}
