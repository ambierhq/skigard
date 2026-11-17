package harness

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite generated docs")

func TestDocsUpToDate(t *testing.T) {
	reg, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	want := Markdown(reg)
	path := filepath.Join("..", "..", "docs", "harnesses.md")
	if *update {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("docs/harnesses.md is out of date; run: go test ./internal/harness -run Docs -update")
	}
}
