package util

import (
	"strings"
	"testing"
)

func TestCleanEscapesControlAndBidi(t *testing.T) {
	got := Clean("a\x1b[31mred\x00b\u202ec\u200bd\tok\n")
	for _, bad := range []string{"\x1b", "\x00", "\u202e", "\u200b"} {
		if strings.Contains(got, bad) {
			t.Errorf("%q survived: %q", bad, got)
		}
	}
	if !strings.Contains(got, "\t") || !strings.Contains(got, "\n") {
		t.Error("tab/newline should be kept")
	}
}

func TestMaskSecrets(t *testing.T) {
	for _, s := range []string{"AKIAABCDEFGHIJKLMNOP", "ghp_" + strings.Repeat("a", 36), "sk-" + strings.Repeat("b", 30), "api_key = abcdefghijklmnop1234"} {
		if got := MaskSecrets("x " + s + " y"); strings.Contains(got, s) {
			t.Errorf("not masked: %q", got)
		}
	}
}

func TestCheckName(t *testing.T) {
	for _, ok := range []string{"a", "my-skill", "a1-b2"} {
		if CheckName(ok) != nil {
			t.Errorf("%q rejected", ok)
		}
	}
	for _, bad := range []string{"", "A", "a_b", "-a", "a-", "a--b", "../x", "a/b", strings.Repeat("a", 65)} {
		if CheckName(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestCodeOf(t *testing.T) {
	if CodeOf(nil) != 0 || CodeOf(Errf("x")) != ExitUsage || CodeOf(Policyf("x")) != ExitPolicy || CodeOf(Cancelled("x")) != ExitCancelled {
		t.Fatal("exit codes wrong")
	}
}

func TestOneLine(t *testing.T) {
	if got := OneLine("a \n\t b   c", 100); got != "a b c" {
		t.Errorf("%q", got)
	}
	if got := OneLine(strings.Repeat("x", 50), 10); !strings.HasSuffix(got, "...") || len(got) != 13 {
		t.Errorf("%q", got)
	}
}
