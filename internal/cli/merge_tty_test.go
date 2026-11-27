package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/diff"
	"github.com/ambierhq/skigard/internal/term"
	"github.com/ambierhq/skigard/internal/util"
)

// These drive the prompt logic with in-memory streams. The terminal-backed
// chooser (ttyChooser) is never called from a test.
func choose(t *testing.T, answers string, fcs ...diff.FileChange) ([][]bool, string, error) {
	t.Helper()
	var out bytes.Buffer
	p := newPromptChooser(strings.NewReader(answers), &out, term.Style{})
	var res [][]bool
	for i := range fcs {
		r, err := p.Choose(&fcs[i])
		if err != nil {
			return res, out.String(), err
		}
		res = append(res, r)
	}
	return res, out.String(), nil
}

func modified(t *testing.T, path, local, up string) diff.FileChange {
	t.Helper()
	td, err := diff.Split([]byte(local), []byte(up))
	if err != nil {
		t.Fatal(err)
	}
	return diff.FileChange{Path: path, Kind: diff.Modified, Text: true, Hunks: td.Hunks()}
}

func TestPromptChooser(t *testing.T) {
	two := modified(t, "a.md", "1\n2\n3\n4\n5\n6\n7\n8\n9\n", "1\nX\n3\n4\n5\n6\n7\n8\nY\n")
	if len(two.Hunks) != 2 {
		t.Fatal("setup")
	}
	whole := diff.FileChange{Path: "b.bin", Kind: diff.Whole, Local: []byte{0, 1}, Upstream: []byte{0, 2}}
	cases := []struct {
		name, answers string
		want          [][]bool
	}{
		{"enter keeps mine", "\n\n\n", [][]bool{{false, false}, {false}}},
		{"take then keep", "t\nk\nt\n", [][]bool{{true, false}, {true}}},
		{"all upstream for the file", "a\nt\n", [][]bool{{true, true}, {true}}},
		{"mine for the rest", "t\nm\n", [][]bool{{true, false}, {false}}},
		{"unknown answer asks again", "zz\nt\nt\nk\n", [][]bool{{true, true}, {false}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, out, err := choose(t, c.answers, two, whole)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("%v", got)
			}
			for i := range got {
				for j := range got[i] {
					if got[i][j] != c.want[i][j] {
						t.Errorf("%v want %v\n%s", got, c.want, out)
					}
				}
			}
		})
	}
	if _, _, err := choose(t, "t\nq\n", two); util.CodeOf(err) != util.ExitCancelled {
		t.Errorf("quit: %v", err)
	}
	if _, _, err := choose(t, "", two); util.CodeOf(err) != util.ExitCancelled {
		t.Errorf("EOF must cancel: %v", err)
	}
}

func TestPromptChooserCleansUntrustedText(t *testing.T) {
	fc := modified(t, "e\x1b[31mvil.md", "a\n", "b\x1b]0;pwn\x07 token=abcdefghijklmnopqrstuvwx\n")
	_, out, err := choose(t, "\n", fc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out, "\x1b\x07") || strings.Contains(out, "abcdefghijklmnopqrstuvwx") {
		t.Errorf("unsafe output: %q", out)
	}
}
