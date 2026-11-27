package scan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ambierhq/skigard/internal/term"
	"github.com/ambierhq/skigard/internal/util"
)

const (
	maxGroupLocs   = 3  // file:line entries shown for a folded group
	groupEvidence  = 80 // widest evidence sample shown for a folded group
	minLineWidth   = 40
	plainEvPrefix  = 15 // "             > "
	styledEvPrefix = 6  // "    > "
)

// RenderOptions controls how a report is written as text. The zero value is the summarized view
// on a 100-column line.
type RenderOptions struct {
	// All prints every finding on its own (no folding of repeated low findings).
	All bool
	// Width is the terminal width in columns; 0 means term.DefaultWidth.
	Width int
}

func (o RenderOptions) width() int {
	w := o.Width
	if w == 0 {
		w = term.DefaultWidth
	}
	return max(w, minLineWidth)
}

// entry is one printed finding or one folded group of same-rule, same-severity findings.
type entry struct {
	first Finding
	count int // findings listed in the report
	more  int // matches the scanner counted but did not list (per-file cap)
	locs  []string
	hosts map[string]int // external-url rule only: host -> count
}

func (e *entry) total() int { return e.count + e.more }

// renderer carries what the text writers share: the entries to print and whether anything was
// folded or cut, which decides the footer.
type renderer struct {
	opt       RenderOptions
	entries   []*entry
	notes     []string
	counts    map[Severity]int
	folded    bool
	truncated bool
	skill     string
}

// newRenderer folds low and info findings by rule and severity. Medium and above are never
// folded, and with All nothing is.
func newRenderer(r *Report, opt RenderOptions) *renderer {
	rd := &renderer{opt: opt, counts: map[Severity]int{}, skill: r.Skill}
	extra, notes := map[string]int{}, r.Notes
	if !opt.All {
		extra, notes = furtherMatches(r.Notes)
	}
	rd.notes = notes
	byKey := map[string]*entry{}
	for _, f := range r.Findings {
		rd.counts[f.Severity]++
		var e *entry
		fold := !opt.All && f.Severity.Rank() < Medium.Rank()
		if fold {
			key := f.Rule + "\x00" + string(f.Severity)
			e = byKey[key]
		}
		if e == nil {
			e = &entry{first: f}
			if n := extra[f.Rule]; n > 0 { // the scanner's uncounted matches go to the first group of the rule
				e.more = n
				delete(extra, f.Rule)
			}
			rd.entries = append(rd.entries, e)
			if fold {
				byKey[f.Rule+"\x00"+string(f.Severity)] = e
			}
		}
		e.count++
		e.locs = append(e.locs, location(f))
		if f.Rule == "SKG-NET-003" {
			if e.hosts == nil {
				e.hosts = map[string]int{}
			}
			e.hosts[urlHost(f.Evidence)]++
		}
	}
	sort.SliceStable(rd.entries, func(i, j int) bool {
		a, b := rd.entries[i], rd.entries[j]
		if a.first.Severity.Rank() != b.first.Severity.Rank() {
			return a.first.Severity.Rank() > b.first.Severity.Rank()
		}
		return a.total() > b.total()
	})
	for _, e := range rd.entries {
		if e.total() > 1 {
			rd.folded = true
		}
	}
	return rd
}

// location is file:line, or just the file, or "(skill)" for a finding about the whole skill.
func location(f Finding) string {
	switch {
	case f.File == "":
		return "(skill)"
	case f.Line > 0:
		return fmt.Sprintf("%s:%d", f.File, f.Line)
	}
	return f.File
}

// locText lists up to maxGroupLocs locations, then "... and N more".
func (e *entry) locText(sep string) string {
	shown := e.locs
	if len(shown) > maxGroupLocs {
		shown = shown[:maxGroupLocs]
	}
	out := make([]string, len(shown))
	for i, l := range shown {
		out[i] = util.Clean(l)
	}
	t := strings.Join(out, sep)
	if n := e.total() - len(shown); n > 0 {
		t += fmt.Sprintf(" ... and %d more", n)
	}
	return t
}

// evidence is the one-line, secret-masked, escaped evidence cut to max cells. It records a cut.
func (rd *renderer) evidence(f Finding, max int) string {
	if f.Evidence == "" {
		return ""
	}
	t := strings.Join(strings.Fields(util.Clean(util.MaskSecrets(f.Evidence))), " ")
	if term.DisplayWidth(t) > max {
		rd.truncated = true
		return term.Truncate(t, max)
	}
	return t
}

// footer is the line that says the output is not complete, or "" when it is.
func (rd *renderer) footer() string {
	name := "NAME"
	if rd.skill != "" {
		name = util.Clean(rd.skill)
	}
	switch {
	case !rd.opt.All && (rd.folded || rd.truncated):
		return fmt.Sprintf("showing a summary; run `skigard scan %s --all` for every finding, or --json", name)
	case rd.truncated:
		return "long evidence is cut to the line width; use --json for the full text"
	}
	return ""
}

// severityCounts is "14 low, 1 medium" style text, worst first.
func severityCounts(counts map[Severity]int) string {
	var parts []string
	for _, sv := range []Severity{Critical, High, Medium, Low, Info} {
		if counts[sv] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[sv], sv))
		}
	}
	return strings.Join(parts, ", ")
}

func (e *entry) countText() string {
	t := fmt.Sprintf("x%d", e.total())
	if e.more > 0 {
		t += fmt.Sprintf(" (%d more not listed)", e.more)
	}
	return t
}
