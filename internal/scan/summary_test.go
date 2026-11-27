package scan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/term"
)

func manyReport() *Report {
	r := &Report{Skill: "pw", Verdict: "review", Coverage: "complete", Risk: 10, Fingerprint: "sha256:abc"}
	for i := 1; i <= 12; i++ {
		r.Findings = append(r.Findings, Finding{Rule: "SKG-NET-003", Name: "external-url", Severity: Low,
			File: fmt.Sprintf("references/r%d.md", i), Line: i, Message: "external URL", Evidence: fmt.Sprintf("https://example.com/%d", i)})
	}
	r.Findings = append(r.Findings,
		Finding{Rule: "SKG-MANIFEST-002", Name: "broad-tools", Severity: Medium, File: "SKILL.md", Line: 3,
			Message: "allowed-tools grants broad access", Evidence: "Bash(" + strings.Repeat("playwright-cli ", 30) + ")"},
		Finding{Rule: "SKG-EXEC-004", Name: "dynamic-exec", Severity: High, File: "run.sh", Line: 7,
			Message: "dynamic code", Evidence: "eval \x1b[31m中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文"},
	)
	return r
}

func render(r *Report, opt RenderOptions) string {
	var b bytes.Buffer
	WriteTextOpts(&b, r, opt)
	return b.String()
}

func TestTextSummaryGolden(t *testing.T) {
	got := render(manyReport(), RenderOptions{Width: 100})
	want := strings.Join([]string{
		"scan: REVIEW  coverage: complete  risk: 10/100 (higher is riskier)  (0 files, 0 scripts)",
		"content hash: sha256:abc",
		"findings: 14 (1 high, 1 medium, 12 low)",
		"  [high    ] SKG-EXEC-004    dynamic-exec         run.sh:7 - dynamic code",
		"             > eval \\x1b[31m中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中文中…",
		"  [medium  ] SKG-MANIFEST-002 broad-tools          SKILL.md:3 - allowed-tools grants broad access",
		"             > Bash(playwright-cli playwright-cli playwright-cli playwright-cli playwright-cli play…",
		"  [low     ] SKG-NET-003     external-url         x12",
		"             at references/r1.md:1, references/r2.md:2, references/r3.md:3 ... and 9 more",
		"             > https://example.com/1",
		"showing a summary; run `skigard scan pw --all` for every finding, or --json",
		"note: the scanner is heuristic. A pass is not a safety certificate.",
		"",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestTextAllPrintsEveryFinding(t *testing.T) {
	got := render(manyReport(), RenderOptions{All: true, Width: 100})
	if n := strings.Count(got, "SKG-NET-003"); n != 12 {
		t.Errorf("--all printed %d external-url findings, want 12:\n%s", n, got)
	}
	if strings.Contains(got, "showing a summary") {
		t.Errorf("--all must not claim a summary:\n%s", got)
	}
	if !strings.Contains(got, "use --json") {
		t.Errorf("--all with cut evidence needs the --json hint:\n%s", got)
	}
}

func TestNoLineExceedsWidthAndNoEscapes(t *testing.T) {
	for _, w := range []int{40, 80, 100, 160} {
		for name, out := range map[string]string{
			"plain":  render(manyReport(), RenderOptions{Width: w}),
			"all":    render(manyReport(), RenderOptions{All: true, Width: w}),
			"styled": styledString(manyReport(), RenderOptions{Width: w}),
		} {
			if strings.Contains(out, "\x1b[31m") && name == "plain" {
				t.Errorf("untrusted escape reached %s output", name)
			}
			for _, line := range strings.Split(out, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), ">") || strings.Contains(line, "allowed-tools grants") {
					if got := term.DisplayWidth(stripSGR(line)); got > w {
						t.Errorf("%s width %d: line is %d cells: %q", name, w, got, line)
					}
				}
			}
		}
	}
}

func styledString(r *Report, opt RenderOptions) string {
	var b bytes.Buffer
	WriteStyledOpts(&b, r, term.Style{}, opt)
	return b.String()
}

func stripSGR(s string) string {
	for {
		i := strings.Index(s, "\x1b[")
		if i < 0 {
			return s
		}
		j := strings.IndexByte(s[i:], 'm')
		if j < 0 {
			return s
		}
		s = s[:i] + s[i+j+1:]
	}
}

func TestEveryMediumPlusStillPrinted(t *testing.T) {
	r := manyReport()
	for i := 0; i < 5; i++ {
		r.Findings = append(r.Findings, Finding{Rule: "SKG-X-001", Name: "x", Severity: Medium, File: fmt.Sprintf("m%d.md", i), Line: i + 1, Message: "m"})
	}
	for name, out := range map[string]string{"text": render(r, RenderOptions{}), "styled": styledString(r, RenderOptions{})} {
		for _, f := range r.Findings {
			if f.Severity.Rank() >= Medium.Rank() && !strings.Contains(out, location(f)) {
				t.Errorf("%s: %s %s not printed:\n%s", name, f.Rule, location(f), out)
			}
		}
	}
}

func TestStyledSummary(t *testing.T) {
	var b bytes.Buffer
	WriteStyledOpts(&b, manyReport(), term.Style{On: true}, RenderOptions{Width: 100})
	out := b.String()
	for _, want := range []string{"x12", "and 9 more", "run `skigard scan pw --all`", "\x1b[33m"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
	if strings.Contains(out, "\x1b[31m中") {
		t.Errorf("untrusted escape in styled output")
	}
}

func TestJSONKeepsEveryFinding(t *testing.T) {
	r := manyReport()
	var b bytes.Buffer
	if err := WriteJSON(&b, r); err != nil {
		t.Fatal(err)
	}
	var back Report
	if err := json.Unmarshal(b.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Findings) != len(r.Findings) || back.Findings[0].Evidence != r.Findings[0].Evidence ||
		back.Findings[12].Evidence != r.Findings[12].Evidence {
		t.Errorf("json changed: %d findings", len(back.Findings))
	}
}

func TestTruncateWide(t *testing.T) {
	if got := term.Truncate("中文中文", 5); got != "中文…" || term.DisplayWidth(got) > 5 {
		t.Errorf("got %q", got)
	}
}
