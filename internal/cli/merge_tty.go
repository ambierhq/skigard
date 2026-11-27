package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/ambierhq/skigard/internal/core"
	"github.com/ambierhq/skigard/internal/diff"
	"github.com/ambierhq/skigard/internal/term"
	"github.com/ambierhq/skigard/internal/util"
)

const (
	maxShownLines = 60
	maxShownWidth = 200
)

// ttyChooser returns a Chooser that reads its decisions from the controlling
// terminal only (never stdin, never a flag or an environment variable), like ttyConfirm.
func ttyChooser(st term.Style) (core.Chooser, func(), error) {
	in, out, closeFn, err := openTerminal()
	if err != nil {
		return nil, nil, err
	}
	return newPromptChooser(in, out, st), closeFn, nil
}

// promptChooser asks per hunk on in/out. Tests give it in-memory streams; the CLI gives it the terminal.
type promptChooser struct {
	in       *bufio.Reader
	out      io.Writer
	st       term.Style
	restMine bool
}

func newPromptChooser(in io.Reader, out io.Writer, st term.Style) *promptChooser {
	return &promptChooser{in: bufio.NewReader(in), out: out, st: st}
}

// Choose implements core.Chooser. The default answer (Enter) is "keep mine".
func (p *promptChooser) Choose(fc *diff.FileChange) ([]bool, error) {
	n := fc.Choices()
	take := make([]bool, n)
	if p.restMine {
		return take, nil
	}
	fmt.Fprintf(p.out, "\n%s\n", p.st.Bold(util.Clean(fc.Path)))
	switch fc.Kind {
	case diff.Added:
		fmt.Fprintf(p.out, "  new file in upstream (%d bytes)\n%s", len(fc.Upstream), p.preview("+ ", fc.Upstream, fc.Text))
	case diff.Deleted:
		fmt.Fprintf(p.out, "  removed upstream; you have it (%d bytes)\n%s", len(fc.Local), p.preview("- ", fc.Local, fc.Text))
	case diff.Whole:
		fmt.Fprintf(p.out, "  differs as a whole file (%d bytes yours, %d upstream; binary or too large for hunks)\n", len(fc.Local), len(fc.Upstream))
	}
	for i := 0; i < n; i++ {
		if fc.Kind == diff.Modified {
			fmt.Fprintf(p.out, "\n  hunk %d/%d\n%s", i+1, n, p.show(fc.Hunks[i].Lines()))
		}
		menu := "[k]eep mine / [t]ake upstream / [a]ll upstream for this file / [m]ine for the rest / [q]uit"
		if n == 1 {
			menu = "[k]eep mine / [t]ake upstream / [m]ine for the rest / [q]uit"
		}
		for answered := false; !answered; {
			fmt.Fprintf(p.out, "  %s [k]: ", menu)
			line, err := p.in.ReadString('\n')
			if err != nil && line == "" {
				return nil, util.Cancelled("cancelled: nothing was staged")
			}
			switch strings.ToLower(strings.TrimSpace(line)) {
			case "", "k":
				answered = true
			case "t":
				take[i], answered = true, true
			case "a":
				for j := i; j < n; j++ {
					take[j] = true
				}
				answered = true
				i = n
			case "m":
				p.restMine = true
				for j := i; j < n; j++ {
					take[j] = false
				}
				return take, nil
			case "q":
				return nil, util.Cancelled("cancelled: nothing was staged")
			}
		}
	}
	return take, nil
}

// show styles hunk lines. Every line is untrusted: cleaned and secret-masked.
func (p *promptChooser) show(lines []string) string {
	var b strings.Builder
	for i, l := range lines {
		if i >= maxShownLines {
			b.WriteString("  ... (truncated)\n")
			break
		}
		b.WriteString(p.safe(l) + "\n")
	}
	return term.Diff(p.st, b.String())
}

func (p *promptChooser) preview(prefix string, data []byte, text bool) string {
	if !text {
		return prefix + "<binary>\n"
	}
	var ls []string
	for _, l := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		ls = append(ls, prefix+strings.TrimRight(l, "\r"))
	}
	return p.show(ls)
}

func (p *promptChooser) safe(l string) string {
	l = util.Clean(util.MaskSecrets(l))
	if r := []rune(l); len(r) > maxShownWidth {
		l = string(r[:maxShownWidth]) + "..."
	}
	return l
}
