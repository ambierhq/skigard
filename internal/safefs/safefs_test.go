package safefs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type ent struct {
	name, body, link string
	typ              byte
	mode             int64
}

func tarball(t *testing.T, ents ...ent) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range ents {
		typ := e.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		h := &tar.Header{Name: e.name, Typeflag: typ, Mode: mode, Linkname: e.link}
		if typ == tar.TypeReg {
			h.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if typ == tar.TypeReg {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestExtractStripsTopDirAndSkipsLinks(t *testing.T) {
	data := tarball(t,
		ent{name: "repo-abc/", typ: tar.TypeDir},
		ent{name: "repo-abc/SKILL.md", body: "hi"},
		ent{name: "repo-abc/run.sh", body: "#!/bin/sh", mode: 0o755},
		ent{name: "repo-abc/link", typ: tar.TypeSymlink, link: "/etc/passwd"},
	)
	dst := filepath.Join(t.TempDir(), "x")
	res, err := ExtractTarGz(bytes.NewReader(data), dst, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if res.TopDir != "repo-abc" || len(res.SkippedLinks) != 1 || res.SkippedLinks[0] != "link" {
		t.Fatalf("unexpected result %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dst, "link")); err == nil {
		t.Fatal("symlink was extracted")
	}
	info, _ := os.Stat(filepath.Join(dst, "run.sh"))
	if info.Mode()&0o111 == 0 {
		t.Fatal("exec bit lost")
	}
}

func TestExtractRejectsHostile(t *testing.T) {
	cases := map[string][]ent{
		"traversal":  {{name: "r/../../evil", body: "x"}},
		"absolute":   {{name: "/etc/evil", body: "x"}},
		"backslash":  {{name: `r/a\b`, body: "x"}},
		"collision":  {{name: "r/A", body: "x"}, {name: "r/a", body: "y"}},
		"two tops":   {{name: "r/a", body: "x"}, {name: "s/b", body: "y"}},
		"device":     {{name: "r/dev", typ: tar.TypeChar}},
		"big file":   {{name: "r/big", body: strings.Repeat("a", 2048)}},
		"parent dir": {{name: "r/a/../../b", body: "x"}},
		// Windows hostile names, rejected on every platform
		"device name":        {{name: "r/NUL", body: "x"}},
		"device with ext":    {{name: "r/scripts/con.txt", body: "x"}},
		"com port":           {{name: "r/COM1", body: "x"}},
		"alternate stream":   {{name: "r/SKILL.md:hidden", body: "x"}},
		"default stream":     {{name: "r/a.txt::$DATA", body: "x"}},
		"drive relative":     {{name: "r/C:evil", body: "x"}},
		"drive in top dir":   {{name: "C:/evil", body: "x"}},
		"unc":                {{name: `//server/share/x`, body: "x"}},
		"unc backslash":      {{name: `\\server\share\x`, body: "x"}},
		"trailing dot":       {{name: "r/run.sh.", body: "x"}},
		"trailing space":     {{name: "r/run.sh ", body: "x"}},
		"short name":         {{name: "r/SKILL~1.MD", body: "x"}},
		"wildcard":           {{name: "r/a*b", body: "x"}},
		"dir case collision": {{name: "r/Dir/a", body: "x"}, {name: "r/dir/b", body: "y"}},
		"file dir collision": {{name: "r/Sub", body: "x"}, {name: "r/sub/b", body: "y"}},
		"collision unicode":  {{name: "r/É", body: "x"}, {name: "r/é", body: "y"}},
	}
	lim := DefaultLimits()
	lim.MaxFileBytes = 1024
	for name, ents := range cases {
		t.Run(name, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), "x")
			if _, err := ExtractTarGz(bytes.NewReader(tarball(t, ents...)), dst, lim); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestExtractLimits(t *testing.T) {
	lim := DefaultLimits()
	lim.MaxEntries = 2
	data := tarball(t, ent{name: "r/a", body: "1"}, ent{name: "r/b", body: "2"}, ent{name: "r/c", body: "3"})
	if _, err := ExtractTarGz(bytes.NewReader(data), filepath.Join(t.TempDir(), "x"), lim); err == nil {
		t.Fatal("expected entry limit error")
	}
	lim = DefaultLimits()
	lim.MaxArchiveBytes = 10
	if _, err := ExtractTarGz(bytes.NewReader(data), filepath.Join(t.TempDir(), "y"), lim); err == nil {
		t.Fatal("expected archive size error")
	}
}

func TestCopyTree(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, ".git"), 0o755)
	os.WriteFile(filepath.Join(src, ".git", "config"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(src, "sub"), 0o755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(src, "sub", "x.sh"), []byte("b"), 0o755)
	dst := filepath.Join(t.TempDir(), "out")
	res, err := CopyTree(src, dst, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if res.Files != 2 {
		t.Fatalf("files = %d", res.Files)
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); err == nil {
		t.Fatal(".git copied")
	}
	if err := os.Symlink("SKILL.md", filepath.Join(src, "ln")); err != nil {
		t.Skip(err)
	}
	if _, err := CopyTree(src, filepath.Join(t.TempDir(), "o2"), DefaultLimits()); err == nil {
		t.Fatal("symlink should be rejected")
	}
}

func TestExtractAcceptsPlainNames(t *testing.T) {
	data := tarball(t,
		ent{name: "r/", typ: tar.TypeDir},
		ent{name: "r/SKILL.md", body: "x"},
		ent{name: "r/scripts/run.sh", body: "y"},
		ent{name: "r/.github/ci.yml", body: "z"},
		ent{name: "r/docs/com10.md", body: "w"},
		ent{name: "r/a b/c d.md", body: "v"},
	)
	if _, err := ExtractTarGz(bytes.NewReader(data), filepath.Join(t.TempDir(), "x"), DefaultLimits()); err != nil {
		t.Fatal(err)
	}
}
