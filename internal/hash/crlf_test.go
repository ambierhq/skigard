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
const crlfFixtureHashV1 = "sha256:a3d5808a6d40fad1f23a89e9f1e54c96f820befb32effe179f86b7e2f8b5a005"
const crlfFixtureHashV2 = "sha256:5d2d81f5066c79fae12fc219999009451ca6bbdf157ab541b71b25e36de49b2b"

func TestCRLFFixtureHashIsPinnedAcrossPlatforms(t *testing.T) {
	for v, want := range map[int]string{1: crlfFixtureHashV1, 2: crlfFixtureHashV2} {
		got, err := DirVersion(filepath.Join("testdata", "crlf-skill"), v)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("v%d fixture hash = %s, want %s (was the fixture's line endings changed by a checkout, or the hash scheme?)", v, got, want)
		}
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
	a, b := mk(crlf), mk(crlf)
	if a != b {
		t.Fatal("hash not deterministic")
	}
}
