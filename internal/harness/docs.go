package harness

import (
	"fmt"
	"sort"
	"strings"
)

// Markdown renders the registry as the table in docs/harnesses.md.
func Markdown(reg []Entry) string {
	rows := append([]Entry(nil), reg...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Verified != rows[j].Verified {
			return rows[i].Verified
		}
		return rows[i].ID < rows[j].ID
	})
	var b strings.Builder
	b.WriteString("# Harness registry\n\n")
	b.WriteString("<!-- Generated from internal/harness/registry.toml. Run: go test ./internal/harness -run Docs -update -->\n\n")
	fmt.Fprintf(&b, "%d harnesses. **Verified** means skigard's authors confirmed the skills directory against the harness's own\n", len(rows))
	b.WriteString("documentation or a machine that runs it. Unverified entries are detected and listed, but skigard only installs to\n")
	b.WriteString("them when you pass `--include-unverified`. Please open a PR to verify or fix an entry.\n\n")
	b.WriteString("| ID | Name | Verified | User-scope dir | Project dir | Detected by |\n|---|---|---|---|---|---|\n")
	for _, e := range rows {
		v := "no"
		if e.Verified {
			v = "yes"
		}
		var sig []string
		for _, x := range e.Bins {
			sig = append(sig, "`"+x+"`")
		}
		for _, x := range e.Apps {
			sig = append(sig, "`"+x+"`")
		}
		dirs := e.Dirs
		if dirs == nil && e.Global != "" {
			dirs = []string{dirOf(e.Global)}
		}
		for _, x := range dirs {
			sig = append(sig, "`"+x+"`")
		}
		glob := "`" + e.Global + "`"
		if e.EnvHome != "" {
			glob += " (or `$" + e.EnvHome + "/" + e.EnvSub + "`)"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | `%s` | %s |\n", e.ID, e.Name, v, glob, e.Project, strings.Join(sig, ", "))
	}
	b.WriteString("\n`~` is your home directory and `{config}` is `$XDG_CONFIG_HOME`, or `~/.config` when that is unset.\n")
	b.WriteString("Harnesses that share a directory (for example `~/.agents/skills`) are installed to once.\n")
	return b.String()
}

func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i > 0 {
		return p[:i]
	}
	return p
}
