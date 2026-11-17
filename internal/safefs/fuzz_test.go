package safefs

import (
	"path"
	"strings"
	"testing"
)

func FuzzCleanEntry(f *testing.F) {
	for _, s := range []string{"a/b", "a/../b", "/abs", "a//b", "a/./b", "..", "a/..", "a\\b", "", "a/b/", "./a"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		rel, top, err := cleanEntry(name)
		if err != nil {
			return
		}
		if top == "" || top == "." || top == ".." || strings.Contains(top, "/") {
			t.Fatalf("bad top %q from %q", top, name)
		}
		if rel != "" && (path.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") || strings.HasSuffix(rel, "/..")) {
			t.Fatalf("escaping rel %q from %q", rel, name)
		}
	})
}
