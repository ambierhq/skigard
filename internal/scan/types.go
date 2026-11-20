// Package scan is skigard's deterministic static scanner. It is heuristic:
// it flags, a human decides. Nothing in a scanned tree is ever executed.
package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Severity of a finding.
type Severity string

const (
	Info     Severity = "info"
	Low      Severity = "low"
	Medium   Severity = "medium"
	High     Severity = "high"
	Critical Severity = "critical"
)

// Rank orders severities.
func (s Severity) Rank() int {
	switch s {
	case Low:
		return 1
	case Medium:
		return 2
	case High:
		return 3
	case Critical:
		return 4
	}
	return 0
}

// Verdicts.
const (
	VerdictPass   = "pass"
	VerdictReview = "review"
	VerdictBlock  = "block"
)

// Coverage values.
const (
	CoverageComplete = "complete"
	CoveragePartial  = "partial"
)

// SchemaVersion of the JSON report.
const SchemaVersion = 1

// RulesVersion changes whenever the rule set changes.
const RulesVersion = "2026.3"

// Finding is a single scanner signal.
type Finding struct {
	Rule     string   `json:"rule"`
	Name     string   `json:"name"`
	Family   string   `json:"family"`
	Severity Severity `json:"severity"`
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
	Message  string   `json:"message"`
	Evidence string   `json:"evidence,omitempty"`
	Key      string   `json:"key"`
}

// FileInfo describes one inspected (or not) file.
type FileInfo struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Kind      string `json:"kind"` // text, binary, asset, exec
	Inspected bool   `json:"inspected"`
	Script    bool   `json:"script,omitempty"`
}

// Accepted is a reviewed finding the user chose to accept.
type Accepted struct {
	Key     string `toml:"key" json:"key"`
	Reason  string `toml:"reason" json:"reason"`
	Expires string `toml:"expires,omitempty" json:"expires,omitempty"`
}

// Report is the result of scanning one skill.
type Report struct {
	SchemaVersion int        `json:"schema_version"`
	Tool          string     `json:"tool"`
	RulesVersion  string     `json:"rules_version"`
	Skill         string     `json:"skill,omitempty"`
	Fingerprint   string     `json:"fingerprint,omitempty"`
	Verdict       string     `json:"verdict"`
	Coverage      string     `json:"coverage"`
	Risk          int        `json:"risk_score"`
	Safety        string     `json:"safety_claim"`
	Capabilities  []string   `json:"capabilities"`
	Files         []FileInfo `json:"files"`
	Findings      []Finding  `json:"findings"`
	Suppressed    []Finding  `json:"suppressed,omitempty"`
	Notes         []string   `json:"notes,omitempty"`
}

// Options tune a scan.
type Options struct {
	ToolVersion string
	Skill       string
	Accepted    []Accepted
	Now         time.Time
	// SkippedLinks are symlinks dropped while fetching (relative to the skill).
	SkippedLinks []string
}

func findingKey(rule, file, evidence string) string {
	sum := sha256.Sum256([]byte(rule + "\x00" + file + "\x00" + strings.TrimSpace(evidence)))
	return hex.EncodeToString(sum[:])
}
