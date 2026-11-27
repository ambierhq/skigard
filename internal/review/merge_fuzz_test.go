package review

import "testing"

func FuzzParseMerge(f *testing.F) {
	const tok = "skigard-test"
	for _, s := range []string{
		"=== SKIGARD MERGE skigard-test BEGIN ===\n=== FILE skigard-test path=SKILL.md ===\nhi\n=== END FILE skigard-test ===\n=== SKIGARD MERGE skigard-test END ===\n",
		"=== SKIGARD MERGE skigard-test BEGIN ===\n=== DELETE skigard-test path=a/b.md ===\n=== SKIGARD MERGE skigard-test END ===",
		"=== SKIGARD MERGE skigard-test BEGIN ===\n=== FILE skigard-test path=../x ===\n", "", "=== SKIGARD MERGE skigard-test BEGIN ===",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out, err := ParseMerge(s, tok)
		if err != nil {
			return
		}
		seen := map[string]bool{}
		for _, p := range append(out.Delete, paths(out.Files)...) {
			if ValidMergePath(p) != nil || seen[p] {
				t.Fatalf("accepted bad or duplicate path %q", p)
			}
			seen[p] = true
		}
	})
}

func paths(fs []MergeFile) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Path)
	}
	return out
}
