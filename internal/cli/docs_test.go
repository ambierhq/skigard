package cli

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite generated docs")

func TestDocsUpToDate(t *testing.T) {
	want := CommandsMarkdown()
	path := filepath.Join("..", "..", "docs", "commands.md")
	if *update {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("docs/commands.md is out of date; run: go test ./internal/cli -run Docs -update")
	}
}
