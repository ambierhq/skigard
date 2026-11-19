package hash

import (
	"os"
	"path/filepath"
	"testing"
)

// crlfFixtureHash pins the content hash of testdata/crlf-skill. The fixture has
// CRLF, LF and mixed line endings and no executable files (Windows cannot
// represent the execute bit), so the same value must come out on Linux, macOS
// and Windows runners. If a checkout rewrote line endings it would change.
const crlfFixtureHash = "sha256:a3d5808a6d40fad1f23a89e9f1e54c96f820befb32effe179f86b7e2f8b5a005"

func TestCRLFFixtureHashIsPinnedAcrossPlatforms(t *testing.T) {
	got, err := Dir(filepath.Join("testdata", "crlf-skill"))
	if err != nil {
		t.Fatal(err)
	}
	if got != crlfFixtureHash {
		t.Fatalf("fixture hash = %s, want %s (was the fixture's line endings changed by a checkout, or the hash scheme?)", got, crlfFixtureHash)
	}
}

// The hash covers raw bytes: the same text with LF and with CRLF line endings
// must hash differently. Line endings are deliberately not normalised.
func TestLineEndingsAreHashedAsGiven(t *testing.T) {
	text := "---\nname: x\ndescription: y\n---\nbody\nmore\n"
	crlf := ""
	for _, r := range text {
		if r == '\n' {
			crlf += "\r"
		}
		crlf += string(r)
	}
	mk := func(content string) string {
		d := t.TempDir()
		if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		h, err := Dir(d)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	if mk(text) == mk(crlf) {
		t.Fatal("LF and CRLF content hash the same: line endings were normalised")
	}
	if mk(crlf) != mk(crlf) {
		t.Fatal("hash not deterministic")
	}
}
