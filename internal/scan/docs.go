package scan

import (
	"fmt"
	"sort"
	"strings"
)

// CatalogMarkdown renders docs/rules.md.
func CatalogMarkdown() string {
	cat := Catalog()
	sort.Slice(cat, func(i, j int) bool { return cat[i][0] < cat[j][0] })
	var b strings.Builder
	b.WriteString("# Scanner rules\n\n")
	b.WriteString("<!-- Generated from internal/scan. Run: go test ./internal/scan -run Docs -update -->\n\n")
	fmt.Fprintf(&b, "Rules version `%s`. The scanner is deterministic and heuristic: it flags, a human decides. It never executes anything.\n\n", RulesVersion)
	b.WriteString("Severity maps to a verdict: any `high` or `critical` finding gives **block**; `medium` (or incomplete coverage) gives **review**; otherwise **pass**.\n")
	b.WriteString("In the prose of Markdown/text files (outside code fences) command-like mentions matching rules marked `(prose-low)` are downgraded to `low`.\n")
	b.WriteString("Accepted findings (`approve --accept REASON`) can suppress `medium` and below, never `high` or `critical`.\n\n")
	b.WriteString("| ID | Name | Family | Severity | What it flags |\n|---|---|---|---|---|\n")
	for _, r := range cat {
		desc := r[4]
		if desc == "" {
			desc = structuralDesc[r[0]]
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n", r[0], r[1], r[2], r[3], desc)
	}
	b.WriteString("\n## Capabilities and chains\n\nFindings imply capabilities (`sensitive_read`, `network_egress`, `dynamic_exec`, `persistence`, `remote_exec`, `destructive_write`, `env_read`, `supply_chain`, `external_instruction`).\n")
	b.WriteString("Dangerous combinations raise their own finding: `sensitive_read` + `network_egress` is `SKG-CHAIN-001` (critical), `persistence` + `dynamic_exec` is `SKG-CHAIN-002` (high).\n")
	return b.String()
}

var structuralDesc = map[string]string{
	"SKG-OBF-001":      "zero-width, bidirectional or tag characters",
	"SKG-OBF-005":      "NUL bytes inside a text file (can hide content from reviewers)",
	"SKG-MANIFEST-001": "SKILL.md missing, no frontmatter, or invalid name/description",
	"SKG-MANIFEST-002": "`allowed-tools` grants broad access (`Bash` without a pattern, or `*`)",
	"SKG-FS-001":       "binary or non-UTF-8 file that cannot be reviewed (makes coverage partial)",
	"SKG-FS-002":       "native executable (ELF, Mach-O, PE)",
	"SKG-FS-003":       "file too large to inspect (makes coverage partial)",
	"SKG-FS-004":       "executable bit set",
	"SKG-FS-005":       "bundled script (reminder to read its code)",
	"SKG-FS-006":       "known binary asset (image, font, pdf); contents not inspected",
	"SKG-FS-007":       "symlink",
	"SKG-FS-008":       "more files than the scanner inspects (makes coverage partial)",
	"SKG-NET-003":      "external URL",
	"SKG-CHAIN-001":    "reads credential locations and has network access",
	"SKG-CHAIN-002":    "installs persistence and executes dynamic code",
}
