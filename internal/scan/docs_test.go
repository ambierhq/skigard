package scan

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite generated docs")

func TestDocsUpToDate(t *testing.T) {
	want := CatalogMarkdown()
	path := filepath.Join("..", "..", "docs", "rules.md")
	if *update {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("docs/rules.md is out of date; run: go test ./internal/scan -run Docs -update")
	}
}

func TestEveryStructuralRuleIsDocumented(t *testing.T) {
	for id := range structural {
		if structuralDesc[id] == "" {
			t.Errorf("%s has no description", id)
		}
	}
}
