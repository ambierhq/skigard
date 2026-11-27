// Package term styles human-readable output with ANSI escapes.
//
// Styling is only for text that has already been through util.Clean: Clean
// escapes every control character, so untrusted input can never inject its own
// escape sequences. A zero Style is plain: every method returns its input.
package term

import (
	"os"
	"runtime"
)

// Style turns text into styled text. The zero value styles nothing.
type Style struct{ On bool }

// Enabled reports whether f is a terminal that should get colour: not
// redirected, not NO_COLOR, not TERM=dumb. Windows consoles stay plain.
func Enabled(f *os.File, getenv func(string) string) bool {
	if f == nil || runtime.GOOS == "windows" {
		return false
	}
	if getenv("NO_COLOR") != "" || getenv("TERM") == "dumb" {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func (s Style) wrap(code, t string) string {
	if !s.On || t == "" {
		return t
	}
	return "\x1b[" + code + "m" + t + "\x1b[0m"
}

// Bold, Dim and the colours below wrap t in one style.
func (s Style) Bold(t string) string    { return s.wrap("1", t) }
func (s Style) Dim(t string) string     { return s.wrap("2", t) }
func (s Style) Red(t string) string     { return s.wrap("31", t) }
func (s Style) Green(t string) string   { return s.wrap("32", t) }
func (s Style) Yellow(t string) string  { return s.wrap("33", t) }
func (s Style) Magenta(t string) string { return s.wrap("35", t) }
func (s Style) Cyan(t string) string    { return s.wrap("36", t) }

// Badge is t as a bold, inverted label in the given colour code ("31" red, "32" green, "33" yellow).
func (s Style) Badge(code, t string) string { return s.wrap("1;7;"+code, " "+t+" ") }
