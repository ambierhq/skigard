package diff

import (
	"bytes"
	"errors"
	"sort"
	"strings"
)

// ErrTooLarge is returned by Split when a file is too big for a line diff;
// callers fall back to a whole-file choice.
var ErrTooLarge = errors.New("file too large to split into hunks")

// ctxLines is the context shown around a hunk.
const ctxLines = 3

// Hunk is one contiguous change between a local and an upstream text file.
// Lines keep their terminator ("\n", "\r\n" or none on a last line), so a
// changed line ending or a missing final newline is a real change.
type Hunk struct {
	Local    []string // lines of the local file the hunk replaces
	Upstream []string // lines of the upstream file that replace them
	Before   []string // up to 3 unchanged lines before, for display only
	After    []string // up to 3 unchanged lines after, for display only
}

// TextDiff is a two-way diff of one text file split into hunks.
type TextDiff struct {
	parts []part
}

type part struct {
	common []string // unchanged lines (hunk == nil)
	hunk   *Hunk
}

// Split diffs local against upstream and groups the differences into hunks.
// Adjacent changed lines form one hunk; hunks are separated by at least one
// unchanged line.
func Split(local, upstream []byte) (*TextDiff, error) {
	a, b := splitLines(local), splitLines(upstream)
	if len(a) > maxLines || len(b) > maxLines {
		return nil, ErrTooLarge
	}
	ops := lcsDiff(a, b)
	td := &TextDiff{}
	var common []string
	flush := func() {
		if len(common) > 0 {
			td.parts = append(td.parts, part{common: common})
			common = nil
		}
	}
	for i := 0; i < len(ops); {
		if ops[i].kind == ' ' {
			common = append(common, ops[i].text)
			i++
			continue
		}
		flush()
		h := &Hunk{}
		for ; i < len(ops) && ops[i].kind != ' '; i++ {
			if ops[i].kind == '-' {
				h.Local = append(h.Local, ops[i].text)
			} else {
				h.Upstream = append(h.Upstream, ops[i].text)
			}
		}
		td.parts = append(td.parts, part{hunk: h})
	}
	flush()
	for i, p := range td.parts {
		if p.hunk == nil {
			continue
		}
		if i > 0 {
			c := td.parts[i-1].common
			p.hunk.Before = append([]string(nil), c[max(0, len(c)-ctxLines):]...)
		}
		if i+1 < len(td.parts) {
			c := td.parts[i+1].common
			p.hunk.After = append([]string(nil), c[:min(len(c), ctxLines)]...)
		}
	}
	return td, nil
}

// Hunks returns the hunks in file order.
func (t *TextDiff) Hunks() []Hunk {
	var out []Hunk
	for _, p := range t.parts {
		if p.hunk != nil {
			out = append(out, *p.hunk)
		}
	}
	return out
}

// Apply builds the merged file. take[i] true takes the upstream side of hunk i;
// false (and any missing entry) keeps the local side.
func (t *TextDiff) Apply(take []bool) []byte {
	var b strings.Builder
	n := 0
	for _, p := range t.parts {
		if p.hunk == nil {
			for _, l := range p.common {
				b.WriteString(l)
			}
			continue
		}
		lines := p.hunk.Local
		if n < len(take) && take[n] {
			lines = p.hunk.Upstream
		}
		n++
		for _, l := range lines {
			b.WriteString(l)
		}
	}
	return []byte(b.String())
}

// Lines renders the hunk for a terminal: context with two spaces, local lines
// with "- " and upstream lines with "+ ". The text is raw; callers must clean it.
func (h Hunk) Lines() []string {
	var out []string
	add := func(prefix string, ls []string) {
		for _, l := range ls {
			out = append(out, prefix+strings.TrimRight(l, "\r\n"))
		}
	}
	add("  ", h.Before)
	add("- ", h.Local)
	add("+ ", h.Upstream)
	add("  ", h.After)
	return out
}

// splitLines splits keeping each line's terminator.
func splitLines(b []byte) []string {
	var out []string
	for len(b) > 0 {
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			out = append(out, string(b))
			break
		}
		out = append(out, string(b[:i+1]))
		b = b[i+1:]
	}
	return out
}

// Change kinds of a FileChange.
const (
	Modified = "modified" // text on both sides; Hunks says where
	Whole    = "whole"    // binary, huge or non-UTF-8: one choice for the file
	Added    = "added"    // only upstream has it
	Deleted  = "deleted"  // only local has it (upstream removed it)
)

// FileChange is one differing file between a local and an upstream tree.
type FileChange struct {
	Path     string
	Kind     string
	Local    []byte // nil when Kind is Added
	Upstream []byte // nil when Kind is Deleted
	Text     bool   // both existing sides are text
	Hunks    []Hunk // Kind Modified only
	// LocalExec and UpstreamExec are the execute bits of the files on each side.
	LocalExec, UpstreamExec bool
	td                      *TextDiff
}

// Choices is how many decisions the file needs (one per hunk, else one).
func (f *FileChange) Choices() int {
	if f.Kind == Modified {
		return len(f.Hunks)
	}
	return 1
}

// Result builds the merged content of the file for the given decisions
// (true = take upstream). exists is false when the file should not exist.
func (f *FileChange) Result(take []bool) (data []byte, exists bool) {
	switch f.Kind {
	case Modified:
		return f.td.Apply(take), true
	case Added:
		if len(take) > 0 && take[0] {
			return f.Upstream, true
		}
		return nil, false
	case Deleted:
		if len(take) > 0 && take[0] {
			return nil, false
		}
		return f.Local, true
	}
	if len(take) > 0 && take[0] {
		return f.Upstream, true
	}
	return f.Local, true
}

// Changes lists the files whose content differs between local and upstream, in
// path order. Mode-only differences are ignored (the local mode wins).
func Changes(localDir, upstreamDir string) ([]FileChange, error) {
	lf, err := list(localDir)
	if err != nil {
		return nil, err
	}
	uf, err := list(upstreamDir)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for k := range lf {
		names[k] = true
	}
	for k := range uf {
		names[k] = true
	}
	var sorted []string
	for k := range names {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	var out []FileChange
	for _, rel := range sorted {
		l, hasL := lf[rel]
		u, hasU := uf[rel]
		switch {
		case !hasL:
			out = append(out, FileChange{Path: rel, Kind: Added, Upstream: u.data, Text: text(u.data), UpstreamExec: u.exec})
		case !hasU:
			out = append(out, FileChange{Path: rel, Kind: Deleted, Local: l.data, Text: text(l.data), LocalExec: l.exec})
		case !bytes.Equal(l.data, u.data):
			fc := FileChange{Path: rel, Kind: Whole, Local: l.data, Upstream: u.data, Text: text(l.data) && text(u.data), LocalExec: l.exec, UpstreamExec: u.exec}
			if fc.Text {
				if td, err := Split(l.data, u.data); err == nil {
					fc.Kind, fc.td, fc.Hunks = Modified, td, td.Hunks()
				}
			}
			out = append(out, fc)
		}
	}
	return out, nil
}
