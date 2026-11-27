package term

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Tag is a short coloured label for a warning block heading: an inverted badge on a terminal, the
// bare word otherwise ("WARNING", "LOCAL FORK"). code is as for Badge.
func (s Style) Tag(code, t string) string {
	if !s.On {
		return t
	}
	return s.Badge(code, t)
}

// Verdict is a scan or review verdict as a badge ("PASS" green, "FAIL" red, anything else yellow).
func (s Style) Verdict(v string) string {
	if !s.On {
		return strings.ToUpper(v)
	}
	switch strings.ToLower(v) {
	case "pass":
		return s.Badge("32", "PASS")
	case "fail":
		return s.Badge("31", "FAIL")
	}
	return s.Badge("33", strings.ToUpper(v))
}

// Score colours "N/10" by the same bands as the safety tiers: 7 and up green, 4 to 6 yellow, below red.
func (s Style) Score(cell string) string {
	n, err := strconv.Atoi(strings.TrimSuffix(cell, "/10"))
	switch {
	case err != nil:
		return s.Dim(cell)
	case n >= 7:
		return s.Green(cell)
	case n >= 4:
		return s.Yellow(cell)
	}
	return s.Red(cell)
}

// KV is one row of a key/value block.
type KV struct{ Key, Value string }

// KVBlock prints rows with the values aligned in one column, keys dim, every line indented. Rows
// with an empty Value are skipped; a row with an empty Key continues the previous value.
func KVBlock(s Style, indent string, rows []KV) string {
	w := 0
	for _, r := range rows {
		if r.Value != "" && utf8.RuneCountInString(r.Key) > w {
			w = utf8.RuneCountInString(r.Key)
		}
	}
	var b strings.Builder
	for _, r := range rows {
		if r.Value == "" {
			continue
		}
		pad := strings.Repeat(" ", w-utf8.RuneCountInString(r.Key))
		fmt.Fprintf(&b, "%s%s%s  %s\n", indent, s.Dim(r.Key), pad, r.Value)
	}
	return b.String()
}

// Indent prefixes every non-empty line of t with indent.
func Indent(t, indent string) string {
	lines := strings.Split(strings.TrimRight(t, "\n"), "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = indent + l
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
