package diff

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSplitApply(t *testing.T) {
	cases := []struct {
		name, local, up string
		hunks           int
		takeAll         string // result when taking every hunk (must equal upstream)
	}{
		{"identical", "a\nb\n", "a\nb\n", 0, "a\nb\n"},
		{"one change", "a\nb\nc\n", "a\nB\nc\n", 1, "a\nB\nc\n"},
		{"two hunks", "a\nb\nc\nd\ne\n", "a\nB\nc\nD\ne\n", 2, "a\nB\nc\nD\ne\n"},
		{"adjacent changes are one hunk", "a\nb\nc\nd\n", "a\nX\nY\nd\n", 1, "a\nX\nY\nd\n"},
		{"add at start", "b\n", "a\nb\n", 1, "a\nb\n"},
		{"add at end", "a\n", "a\nb\n", 1, "a\nb\n"},
		{"delete at start", "a\nb\n", "b\n", 1, "b\n"},
		{"delete at end", "a\nb\n", "a\n", 1, "a\n"},
		{"empty local", "", "a\n", 1, "a\n"},
		{"empty upstream", "a\n", "", 1, ""},
		{"crlf vs lf", "a\r\nb\r\n", "a\nb\n", 1, "a\nb\n"},
		{"crlf kept elsewhere", "a\r\nb\r\nc\r\n", "a\r\nB\r\nc\r\n", 1, "a\r\nB\r\nc\r\n"},
		{"no trailing newline", "a\nb", "a\nb\n", 1, "a\nb\n"},
		{"no trailing newline both", "a\nb", "a\nB", 1, "a\nB"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			td, err := Split([]byte(c.local), []byte(c.up))
			if err != nil {
				t.Fatal(err)
			}
			hs := td.Hunks()
			if len(hs) != c.hunks {
				t.Fatalf("hunks = %d, want %d: %+v", len(hs), c.hunks, hs)
			}
			if got := string(td.Apply(nil)); got != c.local {
				t.Errorf("keep-all = %q, want local %q", got, c.local)
			}
			take := make([]bool, len(hs))
			for i := range take {
				take[i] = true
			}
			if got := string(td.Apply(take)); got != c.takeAll {
				t.Errorf("take-all = %q, want %q", got, c.takeAll)
			}
		})
	}
}

func TestApplyMixed(t *testing.T) {
	td, _ := Split([]byte("a\nb\nc\nd\ne\n"), []byte("a\nB\nc\nD\ne\n"))
	if got := string(td.Apply([]bool{true, false})); got != "a\nB\nc\nd\ne\n" {
		t.Errorf("first only: %q", got)
	}
	if got := string(td.Apply([]bool{false, true})); got != "a\nb\nc\nD\ne\n" {
		t.Errorf("second only: %q", got)
	}
}

func TestHunkContextAndLines(t *testing.T) {
	td, _ := Split([]byte("1\n2\n3\n4\n5\n6\n7\n8\n"), []byte("1\n2\n3\n4\nX\n6\n7\n8\n"))
	h := td.Hunks()[0]
	if len(h.Before) != 3 || len(h.After) != 3 || h.Before[0] != "2\n" {
		t.Errorf("context: %+v", h)
	}
	ls := h.Lines()
	if ls[3] != "- 5" || ls[4] != "+ X" {
		t.Errorf("lines: %q", ls)
	}
}

func TestSplitTooLarge(t *testing.T) {
	big := make([]byte, 0, 2*(maxLines+1))
	for i := 0; i <= maxLines; i++ {
		big = append(big, 'x', '\n')
	}
	if _, err := Split(big, []byte("a\n")); !errors.Is(err, ErrTooLarge) {
		t.Errorf("err = %v", err)
	}
}

func TestChanges(t *testing.T) {
	l, u := t.TempDir(), t.TempDir()
	w := func(dir, rel, data string) {
		p := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(data), 0o644)
	}
	w(l, "same.md", "x\n")
	w(u, "same.md", "x\n")
	w(l, "SKILL.md", "a\nb\n")
	w(u, "SKILL.md", "a\nc\n")
	w(l, "gone.md", "g\n")
	w(u, "new/n.md", "n\n")
	w(l, "bin.dat", "\x00\x01")
	w(u, "bin.dat", "\x00\x02")
	cs, err := Changes(l, u)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range cs {
		got[c.Path] = c.Kind
	}
	want := map[string]string{"SKILL.md": Modified, "gone.md": Deleted, "new/n.md": Added, "bin.dat": Whole}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %s, want %s", k, got[k], v)
		}
	}
	for _, c := range cs {
		switch c.Kind {
		case Added, Deleted:
			if _, ok := c.Result([]bool{false}); (c.Kind == Added) == ok {
				t.Errorf("%s keep-mine existence wrong", c.Path)
			}
			if _, ok := c.Result([]bool{true}); (c.Kind == Added) != ok {
				t.Errorf("%s take-upstream existence wrong", c.Path)
			}
		case Whole:
			if d, _ := c.Result([]bool{true}); string(d) != "\x00\x02" {
				t.Error("whole take")
			}
		}
	}
}
