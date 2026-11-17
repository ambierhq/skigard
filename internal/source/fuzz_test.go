package source

import (
	"strings"
	"testing"
)

func FuzzParse(f *testing.F) {
	for _, s := range []string{"github:o/r", "o/r/a/b@v1", "https://github.com/o/r/tree/main/x", "./x", "../..//", "github:o/r/../../etc", "", "@@", "o/r@", "github:/"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		sp, err := Parse(s)
		if err != nil || sp.Kind != KindGitHub {
			return
		}
		// Whatever Parse accepts must be safe to use in URLs and paths.
		for _, part := range []string{sp.Owner, sp.Repo} {
			if part == "" || strings.ContainsAny(part, "/\\ \x00?#") || part == "." || part == ".." {
				t.Fatalf("unsafe owner/repo %q from %q", part, s)
			}
		}
		for _, seg := range strings.Split(sp.Subpath, "/") {
			if seg == ".." {
				t.Fatalf("subpath traversal from %q", s)
			}
		}
		if strings.HasPrefix(sp.Subpath, "/") || strings.Contains(sp.Subpath, "\\") {
			t.Fatalf("unsafe subpath %q from %q", sp.Subpath, s)
		}
		if strings.HasPrefix(sp.Ref, "-") || strings.Contains(sp.Ref, "..") {
			t.Fatalf("unsafe ref %q from %q", sp.Ref, s)
		}
	})
}
