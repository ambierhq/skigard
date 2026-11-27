package term

import (
	"strings"
	"unicode/utf8"
)

// Table lays out rows in aligned columns, two spaces apart, like text/tabwriter,
// but pads by the visible width so a styled cell does not shift the columns.
// color, if set, styles a cell given its row (0-based, header excluded) and column; header is bold when styled.
// The last column is not padded. Cells must already be cleaned of control characters.
func Table(s Style, header []string, rows [][]string, color func(row, col int, cell string) string) string {
	all := append([][]string{header}, rows...)
	widths := make([]int, len(header))
	for _, r := range all {
		for i, c := range r {
			if i < len(widths) {
				widths[i] = max(widths[i], utf8.RuneCountInString(c))
			}
		}
	}
	var b strings.Builder
	for ri, r := range all {
		for i, c := range r {
			shown := c
			switch {
			case ri == 0:
				shown = s.Bold(s.Dim(c))
			case color != nil:
				shown = color(ri-1, i, c)
			}
			b.WriteString(shown)
			if i < len(r)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-utf8.RuneCountInString(c)+2))
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Level styles a status word: good (green), caution (yellow), bad (red) or
// neutral (dim). Unknown words are returned unchanged.
func (s Style) Level(word string) string {
	switch strings.ToLower(strings.TrimSpace(word)) {
	case "pass", "ok", "approved", "installed", "yes", "true", "complete", "auto", "human", "ok:":
		return s.Green(word)
	case "warn", "warning", "configured", "concerns", "review", "quarantined", "caution", "forced", "no", "false", "incomplete":
		return s.Yellow(word)
	case "fail", "error", "block", "drift", "[drift]", "[override]", "critical", "high":
		return s.Red(word)
	case "absent", "-", "info", "low":
		return s.Dim(word)
	}
	return word
}

// Wrap breaks text into lines of at most width runes, each starting with indent.
func Wrap(text string, width int, indent string) string {
	var lines []string
	cur := ""
	for _, w := range strings.Fields(text) {
		if cur != "" && utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(w) > width {
			lines = append(lines, indent+cur)
			cur = w
			continue
		}
		if cur != "" {
			cur += " "
		}
		cur += w
	}
	if cur != "" {
		lines = append(lines, indent+cur)
	}
	return strings.Join(lines, "\n")
}

// Diff styles a unified-style diff line by line: added green, removed red, headers bold.
func Diff(s Style, d string) string {
	if !s.On {
		return d
	}
	lines := strings.Split(strings.TrimRight(d, "\n"), "\n")
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"), strings.HasPrefix(l, "diff "):
			lines[i] = s.Bold(l)
		case strings.HasPrefix(l, "@@"):
			lines[i] = s.Cyan(l)
		case strings.HasPrefix(l, "+"):
			lines[i] = s.Green(l)
		case strings.HasPrefix(l, "-"):
			lines[i] = s.Red(l)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
