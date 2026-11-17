package skill

import "testing"

func FuzzFrontmatter(f *testing.F) {
	for _, s := range []string{"---\nname: a\n---\n", "---\n", "", "---\nname: >\n  x\n---", "\ufeff---\na: b\n---\nbody"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		fields, _, ok := ParseFrontmatter(s)
		if !ok && fields != nil {
			t.Fatal("fields without ok")
		}
	})
}
