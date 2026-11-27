package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// manyURLSkill has a dozen external URLs over three files: a low finding repeated, which text
// output folds. (The scanner caps matches per file.)
func manyURLSkill(t *testing.T) string {
	dir := filepath.Join(mkSkill(t, "See https://example.com/p0\n"), "demo")
	for f := 1; f <= 3; f++ {
		var b strings.Builder
		for i := 1; i <= 4; i++ {
			fmt.Fprintf(&b, "See https://example.com/f%dp%d\n", f, i)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("r%d.md", f)), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestScanTextSummarizesAndAllShowsEverything(t *testing.T) {
	home := filepath.Join(t.TempDir(), "h")
	dir := manyURLSkill(t)
	_, sum, _ := run(t, home, "scan", dir, "--fail-on", "none")
	if strings.Count(sum, "SKG-NET-003") != 1 || !strings.Contains(sum, "x13") || !strings.Contains(sum, "and 10 more") ||
		!strings.Contains(sum, "skigard scan") || !strings.Contains(sum, "--all") {
		t.Errorf("summary view wrong:\n%s", sum)
	}
	_, all, _ := run(t, home, "scan", dir, "--fail-on", "none", "--all")
	if strings.Count(all, "SKG-NET-003") != 13 || strings.Contains(all, "showing a summary") {
		t.Errorf("--all view wrong:\n%s", all)
	}
}

func TestScanJSONUnchangedByView(t *testing.T) {
	home := filepath.Join(t.TempDir(), "h")
	dir := manyURLSkill(t)
	for _, extra := range [][]string{nil, {"--all"}} {
		args := append([]string{"scan", dir, "--fail-on", "none", "--format", "json"}, extra...)
		_, out, _ := run(t, home, args...)
		var rep struct {
			Findings []struct{ Rule string } `json:"findings"`
		}
		if err := json.Unmarshal([]byte(out), &rep); err != nil || len(rep.Findings) != 13 {
			t.Errorf("json %v: %d findings, err %v", extra, len(rep.Findings), err)
		}
	}
}
