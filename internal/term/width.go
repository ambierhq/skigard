package term

import (
	"os"
	"unicode"
)

// DefaultWidth is the line width assumed when the terminal width is unknown.
const DefaultWidth = 100

// Width is the column count of f when it is a terminal, else DefaultWidth. It never reads the
// environment, so output is the same wherever a test runs.
func Width(f *os.File) int {
	if f == nil {
		return DefaultWidth
	}
	if fi, err := f.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return DefaultWidth
	}
	if w := terminalColumns(f); w >= 20 {
		return w
	}
	return DefaultWidth
}

// RuneWidth is the number of terminal cells r occupies: 0 for combining marks, 2 for wide East
// Asian characters and emoji, 1 otherwise.
func RuneWidth(r rune) int {
	switch {
	case r == 0 || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r):
		return 0
	case r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe30 && r <= 0xfe6f) || (r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) || (r >= 0x1f300 && r <= 0x1f64f) ||
		(r >= 0x1f900 && r <= 0x1f9ff) || (r >= 0x20000 && r <= 0x3fffd)):
		return 2
	}
	return 1
}

// DisplayWidth is the number of terminal cells s occupies.
func DisplayWidth(s string) int {
	n := 0
	for _, r := range s {
		n += RuneWidth(r)
	}
	return n
}

// Truncate cuts s to at most max cells, ending with an ellipsis when it was cut. It never splits
// a rune. Callers pass text that has been through util.Clean.
func Truncate(s string, max int) string {
	if DisplayWidth(s) <= max {
		return s
	}
	if max < 1 {
		return ""
	}
	n := 0
	var out []rune
	for _, r := range s {
		w := RuneWidth(r)
		if n+w > max-1 {
			break
		}
		n += w
		out = append(out, r)
	}
	return string(out) + "…"
}
