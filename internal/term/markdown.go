package term

import (
	"regexp"
	"strings"
)

var (
	headingRe = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	bulletRe  = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])\s+(.*)$`)
	fenceRe   = regexp.MustCompile("^\\s*(```+|~~~+)\\s*(\\S*)")
	linkRe    = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	boldRe    = regexp.MustCompile(`\*\*([^*]+)\*\*|__([^_]+)__`)
	codeRe    = regexp.MustCompile("`([^`]+)`")
)

// Markdown renders a small, readable subset of Markdown for a terminal:
// front matter, headings, lists, quotes, rules, fenced code, tables, and inline
// code, bold and links. It never executes or fetches anything. With a plain
// Style it returns src unchanged. src must already be cleaned of control characters.
func Markdown(src string, s Style) string {
	if !s.On {
		return src
	}
	lines := strings.Split(strings.TrimRight(src, "\n"), "\n")
	var out []string
	i := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		end := -1
		for j := 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "---" {
				end = j
				break
			}
		}
		if end > 0 {
			out = append(out, s.Dim("┌─ front matter"))
			for _, l := range lines[1:end] {
				if k, v, ok := strings.Cut(l, ":"); ok && !strings.HasPrefix(l, " ") {
					out = append(out, s.Dim("│ ")+s.Bold(k+":")+v)
				} else {
					out = append(out, s.Dim("│ ")+l)
				}
			}
			out = append(out, s.Dim("└─"), "")
			i = end + 1
		}
	}
	var fence string
	for ; i < len(lines); i++ {
		l := lines[i]
		if m := fenceRe.FindStringSubmatch(l); m != nil {
			switch {
			case fence == "":
				fence = m[1][:1]
				label := "code"
				if m[2] != "" {
					label = m[2]
				}
				out = append(out, s.Dim("┌─ "+label))
			case strings.HasPrefix(m[1], fence):
				fence = ""
				out = append(out, s.Dim("└─"))
			default:
				out = append(out, s.Dim("│ ")+s.Cyan(l))
			}
			continue
		}
		if fence != "" {
			out = append(out, s.Dim("│ ")+s.Cyan(l))
			continue
		}
		out = append(out, mdLine(l, s))
	}
	return collapseBlank(strings.Join(out, "\n")) + "\n"
}

// collapseBlank drops leading blank lines and squeezes runs of blank lines to one.
func collapseBlank(t string) string {
	var out []string
	for _, l := range strings.Split(t, "\n") {
		if strings.TrimSpace(l) == "" && (len(out) == 0 || out[len(out)-1] == "") {
			continue
		}
		out = append(out, strings.TrimRight(l, " "))
	}
	return strings.Join(out, "\n")
}

func mdLine(l string, s Style) string {
	if m := headingRe.FindStringSubmatch(l); m != nil {
		text := inline(m[2], s)
		switch len(m[1]) {
		case 1:
			return "\n" + s.Bold(s.Magenta(strings.ToUpper(m[2]))) + "\n" + s.Dim(strings.Repeat("═", min(len([]rune(m[2])), 60)))
		case 2:
			return "\n" + s.Bold(s.Cyan(m[2])) + "\n" + s.Dim(strings.Repeat("─", min(len([]rune(m[2])), 60)))
		default:
			return "\n" + s.Bold(text)
		}
	}
	if isRule(l) {
		return s.Dim(strings.Repeat("─", 40))
	}
	if m := bulletRe.FindStringSubmatch(l); m != nil {
		mark := "•"
		if m[2][0] >= '0' && m[2][0] <= '9' {
			mark = m[2]
		}
		return m[1] + s.Cyan(mark) + " " + inline(m[3], s)
	}
	if t, ok := strings.CutPrefix(l, ">"); ok {
		return s.Dim("▎") + " " + s.Dim(inline(strings.TrimPrefix(t, " "), s))
	}
	if strings.HasPrefix(strings.TrimSpace(l), "|") {
		return strings.ReplaceAll(inline(l, s), "|", s.Dim("│"))
	}
	return inline(l, s)
}

// inline styles `code`, **bold** and [text](url) inside one line. Code spans
// are cut out first so their contents are never re-interpreted.
func inline(t string, s Style) string {
	var b strings.Builder
	rest := t
	for {
		loc := codeRe.FindStringSubmatchIndex(rest)
		if loc == nil {
			b.WriteString(prose(rest, s))
			return b.String()
		}
		b.WriteString(prose(rest[:loc[0]], s))
		b.WriteString(s.Cyan(rest[loc[2]:loc[3]]))
		rest = rest[loc[1]:]
	}
}

func prose(t string, s Style) string {
	t = linkRe.ReplaceAllStringFunc(t, func(m string) string {
		p := linkRe.FindStringSubmatch(m)
		return s.Bold(p[1]) + " " + s.Dim("("+p[2]+")")
	})
	return boldRe.ReplaceAllStringFunc(t, func(m string) string {
		p := boldRe.FindStringSubmatch(m)
		return s.Bold(p[1] + p[2])
	})
}

// isRule reports a thematic break: three or more of -, * or _ (spaces allowed) and nothing else.
func isRule(l string) bool {
	t := strings.ReplaceAll(strings.TrimSpace(l), " ", "")
	if len(t) < 3 || strings.Trim(t, t[:1]) != "" {
		return false
	}
	return t[0] == '-' || t[0] == '*' || t[0] == '_'
}
