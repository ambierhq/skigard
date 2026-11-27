package scan

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/term"
)

func TestWriteStyledGroupsAndEscapes(t *testing.T) {
	r := &Report{Skill: "demo", Verdict: "review", Coverage: "complete", Risk: 43,
		Capabilities: []string{"dynamic_exec"},
		Findings: []Finding{
			{Rule: "SKG-EXEC-004", Name: "dynamic-exec", Severity: Medium, File: "SKILL.md", Line: 5, Message: "dynamic code", Evidence: "eval a"},
			{Rule: "SKG-EXEC-004", Name: "dynamic-exec", Severity: Medium, File: "SKILL.md", Line: 9, Message: "dynamic code", Evidence: "eval b"},
			{Rule: "SKG-NET-003", Name: "external-url", Severity: Low, File: "a\x1b[31m.md", Line: 1, Message: "external URL"},
		}}
	var plain, styled bytes.Buffer
	WriteStyled(&plain, r, term.Style{})
	WriteStyled(&styled, r, term.Style{On: true})
	for _, want := range []string{"demo", "REVIEW", "43/100", "x2", "SKILL.md:5,9", "1 low", "2 medium"} {
		if !strings.Contains(plain.String(), want) {
			t.Errorf("missing %q in\n%s", want, plain.String())
		}
	}
	if strings.Contains(plain.String(), "\x1b") {
		t.Errorf("plain style emitted escapes: %q", plain.String())
	}
	// untrusted text may never add its own escape sequence
	if strings.Contains(styled.String(), "a\x1b[31m") {
		t.Errorf("untrusted escape reached the terminal: %q", styled.String())
	}
}
