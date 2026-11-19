package safefs

import (
	"path"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/util"
)

func FuzzCleanEntry(f *testing.F) {
	for _, s := range []string{"a/b", "a/../b", "/abs", "a//b", "a/./b", "..", "a/..", "a\\b", "", "a/b/", "./a",
		"r/NUL", "r/con.txt", "r/a.txt:stream", "r/a.txt::$DATA", "C:foo", "C:/foo", `//srv/share/x`, `\\srv\share`, "r/a.", "r/a ", "r/PROGRA~1", "r/COM1", "r/lpt9.md", "r/x*y", "r/a\x00b"} {
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
		for _, c := range strings.Split(rel, "/") {
			if c != "" && util.CheckPortableComponent(c) != nil {
				t.Fatalf("accepted non-portable component %q from %q", c, name)
			}
		}
		if rel != "" && (path.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") || strings.HasSuffix(rel, "/..")) {
			t.Fatalf("escaping rel %q from %q", rel, name)
		}
	})
}
