package safefs

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ambierhq/skigard/internal/hash"
)

// Skills are hashed as raw bytes, so no code path may rewrite line endings
// (git's autocrlf does; archives and os.Root copies must not).
var lineEndingFiles = map[string]string{
	"SKILL.md":         "---\r\nname: x\r\ndescription: y\r\n---\r\nbody\r\n",
	"scripts/lf.sh":    "#!/bin/sh\necho lf\n",
	"scripts/mixed.sh": "echo a\r\necho b\necho c\r\n",
	"no-newline.txt":   "ends without newline",
	"lone-cr.txt":      "a\rb\r",
	"bom.txt":          "\ufeffbom\r\n",
}

func assertBytes(t *testing.T, dir string) {
	t.Helper()
	for rel, want := range lineEndingFiles {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, []byte(want)) {
			t.Errorf("%s was rewritten: got %q want %q", rel, got, want)
		}
	}
}

func TestExtractAndCopyPreserveLineEndingsAndHash(t *testing.T) {
	src := t.TempDir()
	ents := []ent{}
	for rel, body := range lineEndingFiles {
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		ents = append(ents, ent{name: "repo-1/" + rel, body: body})
	}
	ents = append(ents, ent{name: "repo-1/", typ: tar.TypeDir})
	want, err := hash.Dir(src)
	if err != nil {
		t.Fatal(err)
	}

	extracted := filepath.Join(t.TempDir(), "x")
	if _, err := ExtractTarGz(bytes.NewReader(tarball(t, ents...)), extracted, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	assertBytes(t, extracted)
	if got, _ := hash.Dir(extracted); got != want {
		t.Errorf("extracted hash %s != source hash %s", got, want)
	}

	copied := filepath.Join(t.TempDir(), "c")
	if _, err := CopyTree(src, copied, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	assertBytes(t, copied)
	if got, _ := hash.Dir(copied); got != want {
		t.Errorf("copied hash %s != source hash %s", got, want)
	}

	// a second generation (quarantine -> approved store -> harness) is still identical
	again := filepath.Join(t.TempDir(), "again")
	if _, err := CopyTree(copied, again, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	if got, _ := hash.Dir(again); got != want {
		t.Errorf("second copy hash %s != %s", got, want)
	}
}
