package scan

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ambierhq/skigard/internal/term"
	"github.com/ambierhq/skigard/internal/util"
)

const (
	maxGroupFiles = 4
	maxEvidence   = 110
	riskBarWidth  = 20
)

type findingGroup struct {
	first Finding
	count int
	more  int // matches the scanner counted but did not list (per-file cap)
	bySev map[Severity]int
	locs  map[string][]int // file -> lines
	files []string         // file order of first appearance
	hosts map[string]int   // external-url rule only: host -> count
}

func (g *findingGroup) countText() string {
	t := fmt.Sprintf("x%d", g.count+g.more)
	var parts []string
	for _, sv := range []Severity{Critical, High, Medium, Low, Info} {
		if n := g.bySev[sv]; n > 0 && len(g.bySev) > 1 {
			parts = append(parts, fmt.Sprintf("%d %s", n, sv))
		}
	}
	if g.more > 0 {
		parts = append(parts, fmt.Sprintf("%d more not listed", g.more))
	}
	if len(parts) > 0 {
		t += " (" + strings.Join(parts, ", ") + ")"
	}
	return t
}

func (g *findingGroup) hostText() string {
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

// WriteStyled renders a report for a terminal: the verdict and risk first, then
// findings grouped by rule with their locations. It shows the same facts as
// WriteText. All untrusted text is escaped before it is styled.
func WriteStyled(w io.Writer, r *Report, s term.Style) {
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

	extra, notes := furtherMatches(r.Notes)
	groups := groupFindings(r.Findings, extra)
	counts := map[Severity]int{}
	for _, f := range r.Findings {
		counts[f.Severity]++
	}
	fmt.Fprintf(w, "\n%s  %s\n", s.Bold("FINDINGS"), severitySummary(counts, s))
	if len(groups) == 0 {
		fmt.Fprintln(w, "  "+s.Green("none"))
	}
	for _, g := range groups {
		f := g.first
		fmt.Fprintf(w, "\n  %s %s %s", sevLabel(f.Severity, s), s.Bold(f.Rule), s.Dim(f.Name))
		if total := g.count + g.more; total > 1 {
			fmt.Fprintf(w, "  %s", s.Dim(g.countText()))
		}
		fmt.Fprintf(w, "\n    %s\n", util.Clean(f.Message))
		if g.hosts != nil {
			fmt.Fprintf(w, "    %s %s\n", s.Dim("hosts"), g.hostText())
		} else if loc := locations(g); loc != "" {
			fmt.Fprintf(w, "    %s %s\n", s.Dim("at"), loc)
		}
		if f.Evidence != "" && g.hosts == nil {
			fmt.Fprintf(w, "    %s %s\n", s.Dim(">"), s.Cyan(truncate(util.Clean(f.Evidence), maxEvidence)))
		}
	}
	if len(notes) > 0 {
		fmt.Fprintln(w)
	}
	for _, n := range notes {
		fmt.Fprintf(w, "  note: %s\n", util.Clean(n))
	}
	if len(r.Suppressed) > 0 {
		fmt.Fprintf(w, "  %s\n", s.Dim(fmt.Sprintf("(%d accepted or allowed finding(s) suppressed)", len(r.Suppressed))))
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

// groupFindings merges findings by rule: one entry per rule, at its worst
// severity, with the count per severity. Order: severity, then how often it hit.
func groupFindings(fs []Finding, more map[string]int) []*findingGroup {
	byRule := map[string]*findingGroup{}
	var out []*findingGroup
	for _, f := range fs {
		g := byRule[f.Rule]
		if g == nil {
			g = &findingGroup{first: f, locs: map[string][]int{}, bySev: map[Severity]int{}, more: more[f.Rule]}
			byRule[f.Rule] = g
			out = append(out, g)
		}
		if f.Severity.Rank() > g.first.Severity.Rank() {
			first := f
			g.first = first
		}
		g.count++
		g.bySev[f.Severity]++
		file := f.File
		if file == "" {
			file = "(skill)"
		}
		if _, seen := g.locs[file]; !seen {
			g.files = append(g.files, file)
		}
		if f.Line > 0 {
			g.locs[file] = append(g.locs[file], f.Line)
		} else if _, ok := g.locs[file]; !ok {
			g.locs[file] = nil
		}
		if f.Rule == "SKG-NET-003" {
			if g.hosts == nil {
				g.hosts = map[string]int{}
			}
			g.hosts[urlHost(f.Evidence)]++
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.first.Severity.Rank() != b.first.Severity.Rank() {
			return a.first.Severity.Rank() > b.first.Severity.Rank()
		}
		return a.count+a.more > b.count+b.more
	})
	return out
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

func locations(g *findingGroup) string {
	var parts []string
	for i, file := range g.files {
		if i == maxGroupFiles {
			parts = append(parts, fmt.Sprintf("+%d more file(s)", len(g.files)-maxGroupFiles))
			break
		}
		lines := g.locs[file]
		if len(lines) == 0 {
			parts = append(parts, util.Clean(file))
			continue
		}
		nums := make([]string, 0, len(lines))
		for j, n := range lines {
			if j == 6 {
				nums = append(nums, fmt.Sprintf("+%d", len(lines)-6))
				break
			}
			nums = append(nums, fmt.Sprint(n))
		}
		parts = append(parts, util.Clean(file)+":"+strings.Join(nums, ","))
	}
	return strings.Join(parts, "  ")
}

func truncate(t string, n int) string {
	t = strings.Join(strings.Fields(t), " ")
	if r := []rune(t); len(r) > n {
		return string(r[:n]) + "…"
	}
	return t
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
