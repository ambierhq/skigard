package review

import "testing"

func FuzzParse(f *testing.F) {
	in := &Input{files: map[string]string{"SKILL.md": "hello"}}
	for _, s := range []string{`{"verdict":"pass"}`, `{"verdict":"fail","findings":[{"file":"SKILL.md","quote":"hello"}]}`, "{", "}{", `{"verdict":"pass","findings":null}`, ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		rec, err := Parse(s, in)
		if err == nil && (rec == nil || Rank(rec.Verdict) < 0) {
			t.Fatal("bad record")
		}
	})
}
