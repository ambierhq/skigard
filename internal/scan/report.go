package scan

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ambierhq/skigard/internal/term"
	"github.com/ambierhq/skigard/internal/util"
)

// WriteText renders the summarized plain-text report on a 100-column line.
func WriteText(w io.Writer, r *Report) { WriteTextOpts(w, r, RenderOptions{}) }

// WriteTextOpts renders a human-readable report. Low findings of one rule are folded into one
// line with a count; medium and above are printed one by one, with the evidence cut to one line.
// All untrusted text is escaped.
func WriteTextOpts(w io.Writer, r *Report, opt RenderOptions) {
	scripts := 0
	for _, f := range r.Files {
		if f.Script {
			scripts++
		}
	}
	fmt.Fprintf(w, "scan: %s  coverage: %s  risk: %d/100 (higher is riskier)  (%d files, %d scripts)\n",
		strings.ToUpper(r.Verdict), r.Coverage, r.Risk, len(r.Files), scripts)
	if r.Fingerprint != "" {
		fmt.Fprintf(w, "content hash: %s\n", r.Fingerprint)
	}
	if len(r.Capabilities) > 0 {
		fmt.Fprintf(w, "capabilities: %s\n", strings.Join(r.Capabilities, ", "))
	}
	rd := newRenderer(r, opt)
	width := opt.width()
	if sc := severityCounts(rd.counts); sc != "" {
		fmt.Fprintf(w, "findings: %d (%s)\n", len(r.Findings), sc)
	}
	const pad = "             "
	for _, e := range rd.entries {
		f := e.first
		head := fmt.Sprintf("  [%-8s] %-15s %-20s", f.Severity, f.Rule, f.Name)
		if e.total() > 1 {
			fmt.Fprintf(w, "%s %s\n", head, e.countText())
			fmt.Fprintf(w, "%sat %s\n", pad, term.Truncate(e.locText(", "), width-len(pad)-3))
			if ev := rd.evidence(f, min(width-plainEvPrefix, groupEvidence)); ev != "" {
				fmt.Fprintf(w, "%s> %s\n", pad, ev)
			}
			continue
		}
		loc, msg := util.Clean(location(f)), util.Clean(f.Message)
		if line := fmt.Sprintf("%s %s - %s", head, loc, msg); term.DisplayWidth(line) <= width {
			fmt.Fprintln(w, line)
		} else {
			fmt.Fprintf(w, "%s %s\n%s%s\n", head, loc, pad, term.Truncate(msg, width-len(pad)))
		}
		if ev := rd.evidence(f, width-plainEvPrefix); ev != "" {
			fmt.Fprintf(w, "%s> %s\n", pad, ev)
		}
	}
	for _, n := range rd.notes {
		fmt.Fprintf(w, "  note: %s\n", util.Clean(n))
	}
	if len(r.Suppressed) > 0 {
		fmt.Fprintf(w, "  (%d accepted finding(s) suppressed)\n", len(r.Suppressed))
	}
	if f := rd.footer(); f != "" {
		fmt.Fprintf(w, "%s\n", f)
	}
	fmt.Fprintln(w, "note: the scanner is heuristic. A pass is not a safety certificate.")
}

// WriteJSON renders the report as JSON.
func WriteJSON(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteSARIF renders the report as SARIF 2.1.0.
func WriteSARIF(w io.Writer, r *Report) error {
	type loc struct {
		PhysicalLocation map[string]any `json:"physicalLocation"`
	}
	level := func(s Severity) string {
		switch s {
		case Critical, High:
			return "error"
		case Medium:
			return "warning"
		}
		return "note"
	}
	rules := []map[string]any{}
	seen := map[string]bool{}
	results := []map[string]any{}
	for _, f := range r.Findings {
		if !seen[f.Rule] {
			seen[f.Rule] = true
			rules = append(rules, map[string]any{"id": f.Rule, "name": f.Name,
				"shortDescription": map[string]string{"text": f.Message}})
		}
		res := map[string]any{"ruleId": f.Rule, "level": level(f.Severity),
			"message":             map[string]string{"text": f.Message},
			"partialFingerprints": map[string]string{"skigard/key": f.Key}}
		if f.File != "" {
			region := map[string]int{}
			if f.Line > 0 {
				region["startLine"] = f.Line
			}
			pl := map[string]any{"artifactLocation": map[string]string{"uri": f.File}}
			if len(region) > 0 {
				pl["region"] = region
			}
			res["locations"] = []loc{{PhysicalLocation: pl}}
		}
		results = append(results, res)
	}
	doc := map[string]any{
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"version": "2.1.0",
		"runs": []map[string]any{{
			"tool":    map[string]any{"driver": map[string]any{"name": "skigard", "version": r.Tool, "rules": rules}},
			"results": results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
