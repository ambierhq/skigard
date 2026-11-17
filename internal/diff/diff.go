// Package diff produces a readable unified-style diff between two skill
// directories. It is for human review, so every line is terminal-escaped.
package diff

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ambierhq/skigard/internal/util"
)

const maxLines = 4000

// Dirs compares oldDir (may be empty or missing) with newDir.
func Dirs(oldDir, newDir string) (string, error) {
	oldFiles, err := list(oldDir)
	if err != nil {
		return "", err
	}
	newFiles, err := list(newDir)
	if err != nil {
		return "", err
	}
	names := map[string]bool{}
	for k := range oldFiles {
		names[k] = true
	}
	for k := range newFiles {
		names[k] = true
	}
	var sorted []string
	for k := range names {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	var out strings.Builder
	for _, rel := range sorted {
		o, hasO := oldFiles[rel]
		n, hasN := newFiles[rel]
		switch {
		case !hasO:
			fmt.Fprintf(&out, "+++ added: %s\n", util.Clean(rel))
			writeBody(&out, "+", n)
		case !hasN:
			fmt.Fprintf(&out, "--- removed: %s\n", util.Clean(rel))
		case !bytes.Equal(o.data, n.data) || o.exec != n.exec:
			if o.exec != n.exec {
				fmt.Fprintf(&out, "~~~ mode changed: %s (exec %v -> %v)\n", util.Clean(rel), o.exec, n.exec)
			}
			if bytes.Equal(o.data, n.data) {
				continue
			}
			if !text(o.data) || !text(n.data) {
				fmt.Fprintf(&out, "~~~ binary changed: %s\n", util.Clean(rel))
				continue
			}
			fmt.Fprintf(&out, "--- approved/%s\n+++ candidate/%s\n", util.Clean(rel), util.Clean(rel))
			out.WriteString(unified(lines(o.data), lines(n.data)))
		}
	}
	return out.String(), nil
}

type file struct {
	data []byte
	exec bool
}

func list(dir string) (map[string]file, error) {
	out := map[string]file{}
	if dir == "" {
		return out, nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return out, nil
	}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			out[filepath.ToSlash(rel)] = file{data: []byte("<" + info.Mode().String() + ">")}
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = file{data: data, exec: info.Mode()&0o111 != 0}
		return nil
	})
	return out, err
}

func text(b []byte) bool { return bytes.IndexByte(b, 0) < 0 && utf8.Valid(b) }

func lines(b []byte) []string {
	s := strings.TrimSuffix(string(b), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func writeBody(out *strings.Builder, prefix string, f file) {
	if !text(f.data) {
		out.WriteString(prefix + "<binary>\n")
		return
	}
	for i, l := range lines(f.data) {
		if i >= 200 {
			out.WriteString(prefix + "... (truncated)\n")
			break
		}
		out.WriteString(prefix + util.Clean(l) + "\n")
	}
}

type op struct {
	kind byte // ' ', '-', '+'
	text string
}

func unified(a, b []string) string {
	if len(a) > maxLines || len(b) > maxLines {
		return "(file too large to diff line by line)\n"
	}
	ops := lcsDiff(a, b)
	const ctx = 3
	keep := make([]bool, len(ops))
	for i, o := range ops {
		if o.kind != ' ' {
			for j := i - ctx; j <= i+ctx; j++ {
				if j >= 0 && j < len(ops) {
					keep[j] = true
				}
			}
		}
	}
	var out strings.Builder
	skipping := false
	for i, o := range ops {
		if !keep[i] {
			if !skipping {
				out.WriteString("  ...\n")
				skipping = true
			}
			continue
		}
		skipping = false
		out.WriteString(string(o.kind) + " " + util.Clean(o.text) + "\n")
	}
	return out.String()
}

func lcsDiff(a, b []string) []op {
	n, m := len(a), len(b)
	dp := make([][]int32, n+1)
	for i := range dp {
		dp[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var ops []op
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, op{' ', a[i]})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			ops = append(ops, op{'-', a[i]})
			i++
		default:
			ops = append(ops, op{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, op{'-', a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, op{'+', b[j]})
	}
	return ops
}
