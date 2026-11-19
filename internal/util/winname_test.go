package util

import "testing"

func TestWindowsReservedNames(t *testing.T) {
	for _, n := range []string{"con", "CON", "Con.txt", "nul", "NUL.tar.gz", "aux", "prn", "com1", "COM9", "lpt1.md", "LPT9",
		"com¹", "lpt³.txt", "conin$", "CONOUT$", "con .txt", "nul  "} {
		if !IsWindowsReservedName(n) {
			t.Errorf("%q should be reserved", n)
		}
	}
	for _, n := range []string{"console", "com", "com10", "com0x", "lpt", "aux-tools", "config", "nulled.txt", "a.con", "x-con", ""} {
		if IsWindowsReservedName(n) {
			t.Errorf("%q should not be reserved", n)
		}
	}
}

func TestCheckNameRejectsDeviceNames(t *testing.T) {
	for _, n := range []string{"con", "nul", "com1", "lpt9", "aux", "prn"} {
		if CheckName(n) == nil {
			t.Errorf("CheckName(%q) should fail: it cannot be a directory on Windows", n)
		}
	}
	for _, n := range []string{"console", "com10", "aux-tools", "my-skill"} {
		if err := CheckName(n); err != nil {
			t.Errorf("CheckName(%q): %v", n, err)
		}
	}
}

func TestCheckPortablePath(t *testing.T) {
	bad := map[string]string{
		"reserved":        "scripts/NUL",
		"reserved ext":    "con.txt",
		"nested reserved": "a/aux/b.md",
		"ads":             "file.txt:stream",
		"ads default":     "file.txt::$DATA",
		"drive relative":  "C:foo",
		"drive":           "C:/foo",
		"trailing dot":    "a/readme.md.",
		"trailing space":  "a/readme.md ",
		"wildcard":        "a/b*.md",
		"angle":           "a/<b>",
		"pipe":            "a|b",
		"quote":           `a"b`,
		"question":        "a?b",
		"backslash":       `a\b`,
		"unc":             `\\server\share\x`,
		"control":         "a\x01b",
		"short name":      "PROGRA~1",
		"short name ext":  "docs/SKILL~1.MD",
		"empty":           "a//b",
	}
	for name, p := range bad {
		if CheckPortablePath(p) == nil {
			t.Errorf("%s: %q should be rejected", name, p)
		}
	}
	for _, p := range []string{"SKILL.md", "scripts/run.sh", ".github/workflows/x.yml", "a b/c d.md", "notes~.md", "~tmp", "a~b.txt", "x.tar.gz", "a/.hidden", "com10.txt", "naïve.md"} {
		if err := CheckPortablePath(p); err != nil {
			t.Errorf("%q: %v", p, err)
		}
	}
}
