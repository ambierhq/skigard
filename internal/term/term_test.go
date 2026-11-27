package term

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPlainStyleChangesNothing(t *testing.T) {
	src := "# Title\n\n- **a** `b`\n"
	if got := Markdown(src, Style{}); got != src {
		t.Fatalf("plain markdown changed: %q", got)
	}
	if (Style{}).Red("x") != "x" {
		t.Fatal("plain style added escapes")
	}
}

func TestMarkdownStyled(t *testing.T) {
	s := Style{On: true}
	got := Markdown("---\nname: x\n---\n# Title\n\n- **bold** and `code` [t](http://u)\n\n```sh\nls\n```\n> quote\n", s)
	for _, want := range []string{"front matter", "TITLE", "•", "\x1b[1mbold\x1b[0m", "\x1b[36mcode\x1b[0m", "(http://u)", "┌─ sh", "▎"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "```") || strings.Contains(got, "**") {
		t.Errorf("markers left in %q", got)
	}
}

func TestCodeSpansAreNotReinterpreted(t *testing.T) {
	got := Markdown("`**not bold**`\n", Style{On: true})
	if strings.Contains(got, "\x1b[1m") {
		t.Fatalf("bold applied inside a code span: %q", got)
	}
}

func TestTableMatchesTabwriterWhenPlain(t *testing.T) {
	got := Table(Style{}, []string{"NAME", "STATE"}, [][]string{{"alpha", "approved"}, {"b", "quarantined"}}, nil)
	want := "NAME   STATE\nalpha  approved\nb      quarantined\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTableStyledKeepsColumnsAligned(t *testing.T) {
	s := Style{On: true}
	got := Table(s, []string{"A", "B"}, [][]string{{"x", "ok"}, {"yyyy", "no"}}, func(_, col int, cell string) string {
		if col == 1 {
			return s.Level(cell)
		}
		return cell
	})
	plain := stripANSI(got)
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
	for _, l := range lines {
		if strings.Index(l, l[len(l)-2:]) < 6 && !strings.HasPrefix(l, "A") {
			t.Errorf("column drifted: %q", l)
		}
	}
}

func stripANSI(t string) string {
	var b strings.Builder
	for i := 0; i < len(t); i++ {
		if t[i] == 0x1b {
			for i < len(t) && t[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(t[i])
	}
	return b.String()
}

func TestWrapAndDiff(t *testing.T) {
	if got := Wrap("aaa bbb ccc", 7, "> "); got != "> aaa bbb\n> ccc" {
		t.Fatalf("%q", got)
	}
	if d := "+a\n-b\n"; Diff(Style{}, d) != d {
		t.Fatal("plain diff changed")
	}
	if !strings.Contains(Diff(Style{On: true}, "+a\n-b\n"), "\x1b[32m+a") {
		t.Fatal("added line not green")
	}
}

func TestSpinnerDrawsAndErases(t *testing.T) {
	var b strings.Builder
	var mu sync.Mutex
	w := lockedWriter{&b, &mu}
	sp := startSpinner(w, "reviewing x", time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	sp.Stop()
	mu.Lock()
	defer mu.Unlock()
	got := b.String()
	if !strings.Contains(got, "reviewing x 0:00") || !strings.HasSuffix(got, "\r\x1b[K") {
		t.Fatalf("spinner output %q", got)
	}
}

type lockedWriter struct {
	b  *strings.Builder
	mu *sync.Mutex
}

func (l lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func TestElapsed(t *testing.T) {
	for d, want := range map[time.Duration]string{0: "0:00", 75 * time.Second: "1:15", 3725 * time.Second: "1:02:05"} {
		if got := Elapsed(d); got != want {
			t.Errorf("%v: %s want %s", d, got, want)
		}
	}
}

func TestBlocks(t *testing.T) {
	plain, on := Style{}, Style{On: true}
	if plain.Tag("33", "WARNING") != "WARNING" || plain.Verdict("concerns") != "CONCERNS" {
		t.Error("plain tags carry padding or escapes")
	}
	if !strings.Contains(on.Tag("33", "WARNING"), "\x1b[") || !strings.Contains(on.Verdict("pass"), "PASS") {
		t.Error("styled tags lack escapes")
	}
	got := KVBlock(plain, "  ", []KV{{"source", "a"}, {"commit", ""}, {"replaces", "b"}})
	if got != "  source    a\n  replaces  b\n" {
		t.Errorf("KVBlock %q", got)
	}
	if got := Indent("a\n\nb\n", "  "); got != "  a\n\n  b\n" {
		t.Errorf("Indent %q", got)
	}
	if plain.Score("9/10") != "9/10" || on.Score("9/10") == "9/10" {
		t.Error("Score")
	}
}
