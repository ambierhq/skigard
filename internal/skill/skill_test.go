package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFrontmatter(t *testing.T) {
	f, body, ok := ParseFrontmatter("---\nname: demo\ndescription: >\n  A long\n  description\nallowed-tools: \"Bash(git:*)\"\n---\n# Body\n")
	if !ok || f["name"] != "demo" || f["description"] != "A long description" || f["allowed-tools"] != "Bash(git:*)" {
		t.Fatalf("bad parse: %v %v", f, ok)
	}
	if body != "# Body\n" {
		t.Fatalf("body = %q", body)
	}
	if _, _, ok := ParseFrontmatter("# no frontmatter"); ok {
		t.Fatal("expected !ok")
	}
}

func TestReadAndDiscover(t *testing.T) {
	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	mk("a/SKILL.md", "---\nname: a\ndescription: x\n---\n")
	mk("a/nested/SKILL.md", "---\nname: nested\ndescription: x\n---\n")
	mk("b/SKILL.md", "---\nname: Bad Name\n---\n")
	mk(".git/SKILL.md", "---\nname: g\ndescription: x\n---\n")
	found, err := Discover(root, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 || found[0].Rel != "a" || found[1].Rel != "b" {
		t.Fatalf("found = %+v", found)
	}
	if len(found[0].Bad) != 0 || len(found[1].Bad) == 0 {
		t.Fatalf("problems: %v / %v", found[0].Bad, found[1].Bad)
	}
}
