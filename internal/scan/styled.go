package scan

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ambierhq/skigard/internal/term"
	"github.com/ambierhq/skigard/internal/util"
)

const riskBarWidth = 20

func (g *entry) hostText() string {
	type kv struct {
		h string
		n int
	}
	var list []kv
	for h, n := range g.hosts {
		list = append(list, kv{h, n})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].h < list[j].h
	})
	var out []string
	for i, e := range list {
		if i == 5 {
			out = append(out, fmt.Sprintf("+%d more", len(list)-5))
			break
		}
		out = append(out, fmt.Sprintf("%s (%d)", util.Clean(e.h), e.n))
	}
	return strings.Join(out, ", ")
}

// WriteStyled renders a report for a terminal with the default options (summarized, 100 columns).
func WriteStyled(w io.Writer, r *Report, s term.Style) { WriteStyledOpts(w, r, s, RenderOptions{}) }

// WriteStyledOpts renders a report for a terminal: the verdict and risk first, then findings.
// Low findings of one rule are folded into one group with a count; medium and above are listed one
// by one. It shows the same facts as WriteTextOpts. All untrusted text is escaped before it is
// styled.
func WriteStyledOpts(w io.Writer, r *Report, s term.Style, opt RenderOptions) {
	scripts := 0
	for _, f := range r.Files {
		if f.Script {
			scripts++
		}
	}
	title := "scan"
	if r.Skill != "" {
		title = util.Clean(r.Skill)
	}
	fmt.Fprintf(w, "%s  %s\n", s.Bold(title), verdictBadge(r.Verdict, s))
	fmt.Fprintf(w, "risk      %s %d/100  %s\n", riskBar(r.Risk, s), r.Risk, s.Dim("0 = nothing found, higher is riskier"))
	fmt.Fprintf(w, "coverage  %s   %d files, %d scripts\n", r.Coverage, len(r.Files), scripts)
	if r.Fingerprint != "" {
		fmt.Fprintf(w, "hash      %s\n", s.Dim(r.Fingerprint))
	}
	if len(r.Capabilities) > 0 {
		fmt.Fprintf(w, "powers    %s\n", s.Yellow(strings.Join(r.Capabilities, "  ")))
	}

	rd := newRenderer(r, opt)
	width := opt.width()
	fmt.Fprintf(w, "\n%s  %s\n", s.Bold("FINDINGS"), severitySummary(rd.counts, s))
	if len(rd.entries) == 0 {
		fmt.Fprintln(w, "  "+s.Green("none"))
	}
	for _, g := range rd.entries {
		f := g.first
		fmt.Fprintf(w, "\n  %s %s %s", sevLabel(f.Severity, s), s.Bold(f.Rule), s.Dim(f.Name))
		group := g.total() > 1
		if group {
			fmt.Fprintf(w, "  %s", s.Dim(g.countText()))
		}
		fmt.Fprintf(w, "\n    %s\n", term.Truncate(util.Clean(f.Message), width-4))
		switch {
		case g.hosts != nil && group:
			fmt.Fprintf(w, "    %s %s\n", s.Dim("hosts"), g.hostText())
			fmt.Fprintf(w, "    %s %s\n", s.Dim("at"), g.locText("  "))
		default:
			fmt.Fprintf(w, "    %s %s\n", s.Dim("at"), g.locText("  "))
			evw := width - styledEvPrefix
			if group {
				evw = min(evw, groupEvidence)
			}
			if ev := rd.evidence(f, evw); ev != "" {
				fmt.Fprintf(w, "    %s %s\n", s.Dim(">"), s.Cyan(ev))
			}
		}
	}
	if len(rd.notes) > 0 {
		fmt.Fprintln(w)
	}
	for _, n := range rd.notes {
		fmt.Fprintf(w, "  note: %s\n", util.Clean(n))
	}
	if len(r.Suppressed) > 0 {
		fmt.Fprintf(w, "  %s\n", s.Dim(fmt.Sprintf("(%d accepted or allowed finding(s) suppressed)", len(r.Suppressed))))
	}
	if f := rd.footer(); f != "" {
		fmt.Fprintf(w, "\n%s\n", s.Dim(f))
	}
	fmt.Fprintln(w, "\n"+s.Dim("the scanner is heuristic. A pass is not a safety certificate."))
}

// furtherMatches splits the "ID: N further matches not listed" notes from the rest.
func furtherMatches(notes []string) (map[string]int, []string) {
	more := map[string]int{}
	var rest []string
	for _, n := range notes {
		id, tail, ok := strings.Cut(n, ": ")
		var c int
		if ok && strings.HasSuffix(tail, " further matches not listed") {
			if _, err := fmt.Sscanf(tail, "%d further", &c); err == nil {
				more[id] += c
				continue
			}
		}
		rest = append(rest, n)
	}
	return more, rest
}

func urlHost(u string) string {
	_, rest, ok := strings.Cut(u, "://")
	if !ok {
		return u
	}
	if i := strings.IndexAny(rest, "/?#:"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func verdictBadge(v string, s term.Style) string {
	label := strings.ToUpper(util.Clean(v))
	switch strings.ToLower(v) {
	case "pass":
		return s.Badge("32", label)
	case "block":
		return s.Badge("31", label)
	}
	return s.Badge("33", label)
}

func riskBar(risk int, s term.Style) string {
	n := min(max(risk, 0), 100) * riskBarWidth / 100
	bar := strings.Repeat("█", n) + strings.Repeat("░", riskBarWidth-n)
	switch {
	case risk >= 60:
		return s.Red(bar)
	case risk >= 30:
		return s.Yellow(bar)
	}
	return s.Green(bar)
}

func sevLabel(sv Severity, s term.Style) string {
	label := fmt.Sprintf("%-8s", sv)
	switch sv {
	case Critical, High:
		return s.Red(s.Bold(label))
	case Medium:
		return s.Yellow(label)
	}
	return s.Dim(label)
}

func severitySummary(counts map[Severity]int, s term.Style) string {
	var parts []string
	for _, sv := range []Severity{Critical, High, Medium, Low, Info} {
		if counts[sv] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[sv], sv))
		}
	}
	if len(parts) == 0 {
		return s.Dim("0")
	}
	return s.Dim(strings.Join(parts, " · "))
}

// WriteScore prints the safety score line and its reasons, the same way for add, update and
// approve. mode is the approval mode shown after the threshold; "" leaves it out.
func WriteScore(w io.Writer, s term.Style, sf Safety, mode string) {
	detail := fmt.Sprintf("higher is safer; %s; threshold %d", sf.Tier, sf.Threshold)
	if mode != "" {
		detail += "; mode " + mode
	}
	fmt.Fprintf(w, "%s %s %s\n", s.Bold("safety score:"), s.Score(fmt.Sprintf("%d/10", sf.Score)), s.Dim("("+detail+")"))
	for _, r := range sf.Reasons {
		fmt.Fprintf(w, "  %s %s\n", s.Dim("-"), util.Clean(r))
	}
}
