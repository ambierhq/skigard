package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const tk = "skigard-tok"

func mw(parts ...string) string {
	return "=== SKIGARD MERGE " + tk + " BEGIN ===\n" + strings.Join(parts, "") + "=== SKIGARD MERGE " + tk + " END ===\n"
}

func mf(p, body string) string {
	return "=== FILE " + tk + " path=" + p + " ===\n" + body + "=== END FILE " + tk + " ===\n"
}

func TestParseMergeValid(t *testing.T) {
	raw := "Sure!\n```\n" + mw(mf("SKILL.md", "a\r\nb\n"), "\n", mf("dir/x.txt", ""), "=== DELETE "+tk+" path=old.md ===\n") + "bye"
	out, err := ParseMerge(raw, tk)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Files) != 2 || out.Files[0].Path != "SKILL.md" || string(out.Files[0].Data) != "a\r\nb\n" || len(out.Files[1].Data) != 0 {
		t.Errorf("%+v", out.Files)
	}
	if len(out.Delete) != 1 || out.Delete[0] != "old.md" {
		t.Errorf("%+v", out.Delete)
	}
	if out, err := ParseMerge(mw(), tk); err != nil || len(out.Files) != 0 {
		t.Errorf("an empty merge is valid: %v", err)
	}
}

func TestParseMergeRefuses(t *testing.T) {
	for name, raw := range map[string]string{
		"empty":           "",
		"traversal":       mw(mf("../x", "a\n")),
		"absolute":        mw(mf("/x", "a\n")),
		"backslash":       mw(mf(`a\b`, "a\n")),
		"dot":             mw(mf("./a", "a\n")),
		"dup":             mw(mf("a", "1\n"), mf("a", "2\n")),
		"dup delete":      mw(mf("a", "1\n"), "=== DELETE "+tk+" path=a ===\n"),
		"no end":          "=== SKIGARD MERGE " + tk + " BEGIN ===\n" + mf("a", "1\n"),
		"stray line":      mw("hello\n"),
		"newline in path": mw("=== FILE " + tk + " path=a\nb ===\n"),
		"invisible":       mw(mf("a\u200bb", "x\n")),
		"too deep":        mw(mf(strings.Repeat("d/", 20)+"f", "x\n")),
		"bad utf8":        mw(mf("a", "\xff\n")),
		"ds_store":        mw(mf("d/.DS_Store", "x\n")),
	} {
		if _, err := ParseMerge(raw, tk); err == nil || !strings.Contains(err.Error(), "merge output was invalid") {
			t.Errorf("%s: %v", name, err)
		}
	}
	var many strings.Builder
	for i := 0; i <= MaxMergeFiles; i++ {
		many.WriteString(mf("f"+strings.Repeat("a", i%7)+string(rune('A'+i%26))+string(rune('a'+i/26)), "x\n"))
	}
	if _, err := ParseMerge(mw(many.String()), tk); err == nil {
		t.Error("too many files accepted")
	}
	if _, err := ParseMerge(mw(), ""); err == nil {
		t.Error("empty token accepted")
	}
}

func TestBuildMergeInput(t *testing.T) {
	l, u := t.TempDir(), t.TempDir()
	w := func(d, rel, data string) {
		os.MkdirAll(filepath.Dir(filepath.Join(d, rel)), 0o755)
		os.WriteFile(filepath.Join(d, rel), []byte(data), 0o644)
	}
	w(l, "SKILL.md", "LOCAL-BODY\n")
	w(u, "SKILL.md", "UP-BODY\n")
	w(l, "same.md", "SAME-BODY\n")
	w(u, "same.md", "SAME-BODY\n")
	w(l, "b.bin", "\x00\x01")
	w(u, "b.bin", "\x00\x02")
	in, err := BuildMergeInput(l, u, "demo", "my note \x00\x1b[31m"+strings.Repeat("n", 3000))
	if err != nil {
		t.Fatal(err)
	}
	p := in.Prompt
	for _, s := range []string{"LOCAL-BODY", "UP-BODY", "my note", "[note truncated]", "USER NOTE " + in.Token, "Token for the output markers: " + in.Token} {
		if !strings.Contains(p, s) {
			t.Errorf("missing %q", s)
		}
	}
	if strings.Contains(p, "SAME-BODY") || !strings.Contains(p, "identical on both sides") {
		t.Error("identical files must be listed, not shown")
	}
	if !strings.Contains(p, "NOT shown") || !strings.Contains(p, "b.bin") || strings.ContainsAny(p, "\x00\x1b") {
		t.Errorf("binary handling or sanitising wrong:\n%s", p)
	}
	if strings.Contains(in.System, "LOCAL-BODY") || strings.Contains(in.System, "my note") {
		t.Error("system prompt carries content")
	}
	if len(in.Prompt) > maxPromptBytes+4096 {
		t.Errorf("prompt too large: %d", len(in.Prompt))
	}
}
