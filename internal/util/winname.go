package util

import (
	"fmt"
	"strings"
	"unicode"
)

// Windows file-name rules. skigard applies them on every platform to anything
// that came from an untrusted source or that becomes a directory name, so a skill
// that is accepted on Linux can always be installed on Windows and cannot rely on
// Windows quirks (device names, alternate data streams, 8.3 aliases, trailing
// dots) to land somewhere unexpected.

var winReserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true, "CONIN$": true, "CONOUT$": true, "CLOCK$": true,
}

// IsWindowsReservedName reports whether name (a single path component) is a
// Windows device name. Windows reserves the name with any extension ("con.txt",
// "NUL.tar.gz") and ignores trailing spaces before the extension.
func IsWindowsReservedName(name string) bool {
	base, _, _ := strings.Cut(name, ".")
	base = strings.ToUpper(strings.TrimRight(base, " "))
	if winReserved[base] {
		return true
	}
	// COM1-COM9 and LPT1-LPT9, including the superscript digits Windows also treats as digits.
	if len([]rune(base)) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		switch d := []rune(base)[3]; {
		case d >= '0' && d <= '9', d == '¹', d == '²', d == '³':
			return true
		}
	}
	return false
}

// CheckPortableComponent rejects one path component that Windows would
// interpret specially or reject. It does not look at "." or ".." (callers
// handle traversal separately).
func CheckPortableComponent(c string) error {
	switch {
	case c == "":
		return fmt.Errorf("empty path component")
	case c == "." || c == "..":
		return nil
	case strings.ContainsAny(c, `<>:"|?*\`):
		// ':' covers alternate data streams (file:stream) and drive-relative paths (C:foo)
		return fmt.Errorf("path component %q has a character Windows does not allow", c)
	case strings.IndexFunc(c, unicode.IsControl) >= 0:
		return fmt.Errorf("path component %q has a control character", c)
	case strings.HasSuffix(c, ".") || strings.HasSuffix(c, " "):
		return fmt.Errorf("path component %q ends with a dot or space (Windows strips it)", c)
	case IsWindowsReservedName(c):
		return fmt.Errorf("path component %q is a reserved Windows device name", c)
	case hasShortNameTilde(c):
		return fmt.Errorf("path component %q looks like a Windows 8.3 short name", c)
	}
	return nil
}

// hasShortNameTilde detects "NAME~1" or "NAME~1.EXT": the form NTFS generates
// for 8.3 aliases, which can alias a different, longer file name.
func hasShortNameTilde(c string) bool {
	base, _, _ := strings.Cut(c, ".")
	i := strings.LastIndexByte(base, '~')
	if i <= 0 || i == len(base)-1 {
		return false
	}
	for _, r := range base[i+1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// CheckPortablePath validates every component of a slash-separated path.
func CheckPortablePath(p string) error {
	for _, c := range strings.Split(p, "/") {
		if err := CheckPortableComponent(c); err != nil {
			return err
		}
	}
	return nil
}
