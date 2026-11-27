package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/diff"
	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/review"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

const notesBase = "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"

// forkScenario approves alpha, edits it locally and has `update` stage a changed upstream.
// Local changed notes.md line 2; upstream changed notes.md line 9 and SKILL.md, removed
// gone.md, changed bin.dat and added new.md.
func forkScenario(t *testing.T) *App {
	t.Helper()
	a, _ := newApp(t)
	setSettings(t, a, store.Settings{Mode: "strict"})
	src := t.TempDir()
	d := writeSkill(t, src, "alpha", map[string]string{"notes.md": notesBase, "gone.md": "g\n"})
	addLocal(t, a, src, AddOpts{Skill: "alpha"})
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: yes, Override: true, Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	w := func(dir, rel, data string) {
		util.MakeWritable(dir) // approved skills are read-only; simulate chmod -R u+w
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	local := a.Home.SkillDir("alpha")
	w(local, "bin.dat", "\x00\x01")
	w(local, "notes.md", strings.Replace(notesBase, "l2\n", "MINE2\n", 1))
	w(d, "notes.md", strings.Replace(notesBase, "l9\n", "UP9\n", 1))
	rewriteSkill(src, "alpha", "upstream addition\n")
	w(d, "bin.dat", "\x00\x02")
	w(d, "new.md", "new upstream file\n")
	os.Remove(filepath.Join(d, "gone.md"))
	rs, err := a.Update(context.Background(), []string{"alpha"})
	if err != nil || len(rs) != 1 || !rs[0].Changed {
		t.Fatalf("%+v %v", rs, err)
	}
	return a
}

type chooserFunc func(fc *diff.FileChange) ([]bool, error)

func (f chooserFunc) Choose(fc *diff.FileChange) ([]bool, error) { return f(fc) }

func allOf(v bool) chooserFunc {
	return func(fc *diff.FileChange) ([]bool, error) {
		t := make([]bool, fc.Choices())
		for i := range t {
			t[i] = v
		}
		return t, nil
	}
}

func dirHash(t *testing.T, dir string) string {
	t.Helper()
	h, err := hash.Dir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		return "<missing>"
	}
	return string(b)
}

func TestPickMergeKeepAllTakeAllMixed(t *testing.T) {
	a := forkScenario(t)
	q := a.Home.QuarantineSkill("alpha")
	local := a.Home.SkillDir("alpha")
	pristineV, _ := hash.DirVersion(q, hash.Version)
	localHash := dirHash(t, local)
	lockBytes, _ := os.ReadFile(a.Home.LockPath())

	// Keep mine everywhere: the merge equals the local copy.
	m, err := a.PickMerge("alpha", allOf(false))
	if err != nil {
		t.Fatal(err)
	}
	if got := dirHash(t, q); got != localHash || m.Hash != localHash {
		t.Errorf("keep-all differs from local")
	}
	if m.Meta.UpstreamHash != pristineV || m.Meta.MergeMode != MergePick || m.Meta.MergedBy != "" {
		t.Errorf("%+v", m.Meta)
	}
	if dirHash(t, local) != localHash {
		t.Error("skills/ changed")
	}
	if now, _ := os.ReadFile(a.Home.LockPath()); string(now) != string(lockBytes) {
		t.Error("lock changed")
	}
	if d, _ := a.Diff("alpha"); d != "" {
		t.Errorf("diff of keep-all: %q", d)
	}
	// Approving it records a local fork pinned to the pristine upstream.
	var summary string
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: func(s, w string) error { summary = s; return nil }, Override: true, Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	ls := lockEntry(t, a, "alpha")
	if ls.Approval.Mode != store.ApprovalLocal || ls.UpstreamHash != pristineV {
		t.Errorf("%+v", ls)
	}
	if !strings.Contains(summary, "hand-picked merge") {
		t.Errorf("summary lacks the merge note:\n%s", summary)
	}

	// Take upstream everywhere: the merge equals the pristine upstream.
	a = forkScenario(t)
	q = a.Home.QuarantineSkill("alpha")
	pristine := dirHash(t, q)
	if _, err := a.PickMerge("alpha", allOf(true)); err != nil {
		t.Fatal(err)
	}
	if got := dirHash(t, q); got != pristine {
		t.Errorf("take-all differs from upstream")
	}
	// Taking all upstream is not a fork: approving records mode human.
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: yes, Override: true, Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	if ls := lockEntry(t, a, "alpha"); ls.UpstreamHash != "" || ls.Approval.Mode != store.ApprovalHuman {
		t.Errorf("%+v", ls)
	}

	// Mixed: upstream's notes.md, bin.dat and new.md; mine for SKILL.md and gone.md.
	a = forkScenario(t)
	q = a.Home.QuarantineSkill("alpha")
	var seen []string
	mixed := chooserFunc(func(fc *diff.FileChange) ([]bool, error) {
		seen = append(seen, fc.Path+":"+fc.Kind)
		take := make([]bool, fc.Choices())
		switch fc.Path {
		case "notes.md":
			for i := range take {
				take[i] = true
			}
		case "bin.dat", "new.md":
			take[0] = true
		}
		return take, nil
	})
	if _, err := a.PickMerge("alpha", mixed); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, q, "notes.md"), strings.Replace(notesBase, "l9\n", "UP9\n", 1); got != want {
		t.Errorf("notes.md = %q, want upstream's %q", got, want)
	}
	if readFile(t, q, "bin.dat") != "\x00\x02" || readFile(t, q, "new.md") != "new upstream file\n" || readFile(t, q, "gone.md") != "g\n" {
		t.Errorf("whole-file choices not applied")
	}
	if strings.Join(seen, ",") != "SKILL.md:modified,bin.dat:whole,gone.md:deleted,new.md:added,notes.md:modified" {
		t.Errorf("files offered: %v", seen)
	}
}

func TestPickMergePerHunk(t *testing.T) {
	a := forkScenario(t)
	// notes.md has two hunks vs upstream: line 2 (mine only) and line 9 (upstream only).
	var hunks int
	m, err := a.PickMerge("alpha", chooserFunc(func(fc *diff.FileChange) ([]bool, error) {
		take := make([]bool, fc.Choices())
		if fc.Path == "notes.md" {
			hunks = len(fc.Hunks)
			for i, h := range fc.Hunks {
				take[i] = strings.Contains(strings.Join(h.Upstream, ""), "UP9")
			}
		}
		return take, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(strings.Replace(notesBase, "l2\n", "MINE2\n", 1), "l9\n", "UP9\n", 1)
	if got := readFile(t, a.Home.QuarantineSkill("alpha"), "notes.md"); hunks != 2 || got != want {
		t.Errorf("hunks=%d notes.md=%q want %q", hunks, got, want)
	}
	_ = m
}

func TestPickMergePreconditionsAndCancel(t *testing.T) {
	// Not approved.
	a, _ := newApp(t)
	if _, err := a.PickMerge("nope", allOf(false)); err == nil || !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("%v", err)
	}
	// Approved but nothing in quarantine: points to update.
	a, _ = approvedStrict(t)
	if _, err := a.PickMerge("alpha", allOf(false)); err == nil || !strings.Contains(err.Error(), "skigard update alpha") {
		t.Fatalf("%v", err)
	}
	// Identical to upstream: a policy refusal.
	if _, err := a.Update(context.Background(), []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	a.Home.Init()
	src := a.Home.SkillDir("alpha")
	if err := os.MkdirAll(a.Home.QuarantineSkill("alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyDir(t, src, a.Home.QuarantineSkill("alpha"))
	(&store.Meta{Name: "alpha", Source: "x", Kind: "local", Location: "x"}).Save(a.Home)
	_, err := a.PickMerge("alpha", allOf(false))
	if err == nil || util.CodeOf(err) != util.ExitPolicy || !strings.Contains(err.Error(), "nothing to merge") {
		t.Fatalf("%v", err)
	}

	// A chooser error (quit) stages nothing and leaves the upstream version intact.
	a = forkScenario(t)
	q := a.Home.QuarantineSkill("alpha")
	pristine := dirHash(t, q)
	_, err = a.PickMerge("alpha", chooserFunc(func(*diff.FileChange) ([]bool, error) { return nil, util.Cancelled("q") }))
	if err == nil || dirHash(t, q) != pristine {
		t.Fatalf("%v", err)
	}
	if m, _ := store.LoadMeta(a.Home, "alpha"); m.MergeMode != "" || m.UpstreamHash != "" {
		t.Errorf("%+v", m)
	}
	// Wrong number of decisions is refused.
	if _, err := a.PickMerge("alpha", chooserFunc(func(*diff.FileChange) ([]bool, error) { return nil, nil })); err == nil {
		t.Error("accepted a chooser that decided nothing")
	}
	// A staged merge is not a fresh upstream: merging again is refused.
	if _, err := a.PickMerge("alpha", allOf(false)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PickMerge("alpha", allOf(false)); err == nil || !strings.Contains(err.Error(), "already a staged") {
		t.Fatalf("%v", err)
	}
}

func TestPickMergeRefusedInAgent(t *testing.T) {
	a := forkScenario(t)
	a.AgentAncestor = func() (string, bool) { return "some-agent", true }
	called := false
	_, err := a.PickMerge("alpha", chooserFunc(func(*diff.FileChange) ([]bool, error) { called = true; return nil, nil }))
	if err == nil || called || util.CodeOf(err) != util.ExitPolicy {
		t.Fatalf("%v called=%v", err, called)
	}
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		rel, _ := filepath.Rel(src, p)
		if err != nil || fi.IsDir() {
			return nil
		}
		b, _ := os.ReadFile(p)
		os.MkdirAll(filepath.Dir(filepath.Join(dst, rel)), 0o755)
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
}

// ---- agent merge -------------------------------------------------------------

type fakeAgent struct {
	reply func(token string) string
	err   error
	// recorded
	argv   []string
	prompt string
	system string
	calls  int
	iso    review.Isolation
}

func (f *fakeAgent) Run(_ context.Context, t review.Template, model string, in *review.Input) (string, review.Isolation, error) {
	f.calls++
	system := review.SystemPrompt
	if in.System != "" {
		system = in.System
	}
	f.argv, f.prompt, f.system = t.Args(system, model), in.Prompt, system
	if f.err != nil {
		return "", review.Isolation{}, f.err
	}
	token := ""
	for _, l := range strings.Split(in.Prompt, "\n") {
		if v, ok := strings.CutPrefix(l, "Token for the output markers: "); ok {
			token = v
		}
	}
	iso := f.iso
	if iso.Mode == "" {
		iso = review.Isolation{Mode: review.IsoSeatbelt}
	}
	return f.reply(token), iso, nil
}

func mergedSkill(name string) string {
	return fmt.Sprintf("---\nname: %s\ndescription: does %s things\n---\n# %s\nmerged\n", name, name, name)
}

func block(tok, p, body string) string {
	return fmt.Sprintf("=== FILE %s path=%s ===\n%s=== END FILE %s ===\n", tok, p, body, tok)
}

func wrap(tok string, parts ...string) string {
	return "=== SKIGARD MERGE " + tok + " BEGIN ===\n" + strings.Join(parts, "") + "=== SKIGARD MERGE " + tok + " END ===\n"
}

func TestAgentMergeValid(t *testing.T) {
	a := forkScenario(t)
	q := a.Home.QuarantineSkill("alpha")
	pristineV, _ := hash.DirVersion(q, hash.Version)
	localHash := dirHash(t, a.Home.SkillDir("alpha"))
	lockBytes, _ := os.ReadFile(a.Home.LockPath())
	f := &fakeAgent{reply: func(tok string) string {
		return "Here you go:\n" + wrap(tok, block(tok, "SKILL.md", mergedSkill("alpha")), block(tok, "extra/new.txt", "x\ny\n"), "=== DELETE "+tok+" path=gone.md ===\n")
	}}
	a.MergeRunner = f
	m, err := a.AgentMerge(context.Background(), "alpha", MergeOpts{Harness: "pi", Note: "use mise"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Meta.MergedBy != "pi" || m.Meta.MergeMode != MergeAgent || m.Meta.MergeIsolation != review.IsoSeatbelt || m.Meta.UpstreamHash != pristineV {
		t.Errorf("%+v", m.Meta)
	}
	if m.Report == nil {
		t.Error("merge output was not scanned")
	}
	// Fallback: files the agent did not emit keep local, else upstream; DELETE removes.
	if got := readFile(t, q, "notes.md"); got != strings.Replace(notesBase, "l2\n", "MINE2\n", 1) {
		t.Errorf("notes.md should keep the local version: %q", got)
	}
	if readFile(t, q, "new.md") != "new upstream file\n" {
		t.Error("an upstream-only file should be taken from upstream")
	}
	if readFile(t, q, "gone.md") != "<missing>" {
		t.Error("DELETE did not remove gone.md")
	}
	if readFile(t, q, "bin.dat") != "\x00\x01" {
		t.Error("a binary file should keep the local version")
	}
	if readFile(t, q, "extra/new.txt") != "x\ny\n" || !strings.Contains(readFile(t, q, "SKILL.md"), "merged") {
		t.Error("emitted files missing")
	}
	if dirHash(t, a.Home.SkillDir("alpha")) != localHash {
		t.Error("skills/ changed")
	}
	if now, _ := os.ReadFile(a.Home.LockPath()); string(now) != string(lockBytes) {
		t.Error("lock changed")
	}
	d, err := a.Diff("alpha")
	if err != nil || !strings.Contains(d, "merged") {
		t.Errorf("diff is not local vs merged: %q %v", d, err)
	}
	// Approval stays a human step and records a local fork with the merge note.
	var summary string
	if _, err := a.Approve("alpha", ApproveOpts{Confirm: func(s, w string) error { summary = s; return nil }, Override: true, Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	if ls := lockEntry(t, a, "alpha"); ls.Approval.Mode != store.ApprovalLocal || ls.UpstreamHash != pristineV {
		t.Errorf("%+v", ls)
	}
	if !strings.Contains(summary, "merged by agent pi (isolation seatbelt)") {
		t.Errorf("summary:\n%s", summary)
	}
}

func TestAgentMergeRefusesBadOutput(t *testing.T) {
	big := strings.Repeat("xxxxxxxx\n", 40000)
	cases := map[string]func(tok string) string{
		"path traversal": func(tok string) string {
			return wrap(tok, block(tok, "SKILL.md", mergedSkill("alpha")), block(tok, "../evil.md", "x\n"))
		},
		"nested traversal": func(tok string) string { return wrap(tok, block(tok, "a/../../evil.md", "x\n")) },
		"absolute path":    func(tok string) string { return wrap(tok, block(tok, "/etc/passwd", "x\n")) },
		"windows name":     func(tok string) string { return wrap(tok, block(tok, "CON", "x\n")) },
		"git dir":          func(tok string) string { return wrap(tok, block(tok, ".git/config", "x\n")) },
		"duplicate": func(tok string) string {
			return wrap(tok, block(tok, "SKILL.md", mergedSkill("alpha")), block(tok, "SKILL.md", mergedSkill("alpha")))
		},
		"case-insensitive duplicate": func(tok string) string {
			return wrap(tok, block(tok, "a.md", "x\n"), block(tok, "A.md", "y\n"))
		},
		"missing SKILL.md": func(tok string) string { return wrap(tok, "=== DELETE "+tok+" path=SKILL.md ===\n") },
		"wrong name":       func(tok string) string { return wrap(tok, block(tok, "SKILL.md", mergedSkill("beta"))) },
		"delete unknown":   func(tok string) string { return wrap(tok, "=== DELETE "+tok+" path=nothing.md ===\n") },
		"oversized":        func(tok string) string { return wrap(tok, block(tok, "big.md", big)) },
		"no end marker": func(tok string) string {
			return "=== SKIGARD MERGE " + tok + " BEGIN ===\n" + block(tok, "SKILL.md", mergedSkill("alpha"))
		},
		"unclosed file": func(tok string) string {
			return "=== SKIGARD MERGE " + tok + " BEGIN ===\n=== FILE " + tok + " path=SKILL.md ===\nhi\n=== SKIGARD MERGE " + tok + " END ===\n"
		},
		"prose inside": func(tok string) string { return wrap(tok, "sure, here is the merge\n") },
		"wrong token": func(tok string) string {
			return wrap("skigard-forged", block("skigard-forged", "SKILL.md", mergedSkill("alpha")))
		},
		"no markers":      func(tok string) string { return mergedSkill("alpha") },
		"binary content":  func(tok string) string { return wrap(tok, block(tok, "x.bin", "a\x00b\n")) },
		"two BEGIN":       func(tok string) string { return wrap(tok) + wrap(tok) },
		"file under file": func(tok string) string { return wrap(tok, block(tok, "notes.md/x", "x\n")) },
		"nested marker": func(tok string) string {
			return wrap(tok, "=== FILE "+tok+" path=a.md ===\n=== FILE "+tok+" path=b.md ===\n")
		},
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			a := forkScenario(t)
			q := a.Home.QuarantineSkill("alpha")
			pristine := dirHash(t, q)
			metaBefore, _ := os.ReadFile(a.Home.MetaPath("alpha"))
			a.MergeRunner = &fakeAgent{reply: reply}
			if _, err := a.AgentMerge(context.Background(), "alpha", MergeOpts{Harness: "pi"}); err == nil || !strings.Contains(err.Error(), "merge output was invalid") {
				t.Fatalf("expected an invalid-output refusal, got %v", err)
			}
			if dirHash(t, q) != pristine {
				t.Error("quarantine changed")
			}
			if after, _ := os.ReadFile(a.Home.MetaPath("alpha")); string(after) != string(metaBefore) {
				t.Error("meta changed")
			}
			ents, _ := os.ReadDir(a.Home.QuarantineDir())
			for _, e := range ents {
				if strings.Contains(e.Name(), ".upstream-") {
					t.Errorf("leftover %s", e.Name())
				}
			}
		})
	}
}

func TestAgentMergeRunnerAndTemplateErrors(t *testing.T) {
	a := forkScenario(t)
	if _, err := a.AgentMerge(context.Background(), "alpha", MergeOpts{Harness: "nonexistent"}); err == nil {
		t.Error("unknown harness accepted")
	}
	f := &fakeAgent{reply: func(string) string { return "" }}
	a.MergeRunner = f
	if _, err := a.AgentMerge(context.Background(), "alpha", MergeOpts{Harness: "gemini-cli"}); err == nil || f.calls != 0 {
		t.Errorf("a harness without a template must be refused before running: %v", err)
	}
	f.err = util.Errf("pi failed")
	if _, err := a.AgentMerge(context.Background(), "alpha", MergeOpts{Harness: "pi"}); err == nil {
		t.Error("runner failure was swallowed")
	}
	if m, _ := store.LoadMeta(a.Home, "alpha"); m.MergeMode != "" {
		t.Error("a failed run staged something")
	}
}

func TestAgentMergePromptSeparatesNoteAndTrees(t *testing.T) {
	a := forkScenario(t)
	local := a.Home.SkillDir("alpha")
	os.WriteFile(filepath.Join(local, "notes.md"), []byte("LOCALMARKER-ZZ\n"), 0o644)
	os.WriteFile(filepath.Join(a.Home.QuarantineSkill("alpha"), "notes.md"), []byte("UPSTREAMMARKER-ZZ\n"), 0o644)
	f := &fakeAgent{reply: func(tok string) string { return wrap(tok, block(tok, "SKILL.md", mergedSkill("alpha"))) }}
	a.MergeRunner = f
	if _, err := a.AgentMerge(context.Background(), "alpha", MergeOpts{Harness: "claude-code", Note: "NOTE-SECRET-42 use mise run playwright"}); err != nil {
		t.Fatal(err)
	}
	argv := strings.Join(f.argv, "\x00") + f.system
	for _, s := range []string{"NOTE-SECRET-42", "LOCALMARKER-ZZ", "UPSTREAMMARKER-ZZ", "MINE2", "upstream addition"} {
		if strings.Contains(argv, s) {
			t.Errorf("%q leaked into argv/system prompt", s)
		}
		if s != "MINE2" && !strings.Contains(f.prompt, s) {
			t.Errorf("%q missing from stdin prompt", s)
		}
	}
	if !strings.Contains(f.prompt, "USER NOTE") || !strings.Contains(f.system, "UNTRUSTED DATA") {
		t.Error("prompt framing missing")
	}
	if strings.Contains(f.prompt, "\x00") {
		t.Error("NUL in prompt")
	}
}

func TestAgentMergeThroughRealRunnerUsesStdinOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stand-in")
	}
	a := forkScenario(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "fakepi")
	script := `#!/bin/sh
printf '%s\n' "$@" > "` + dir + `/argv"
cat > "` + dir + `/stdin"
tok=$(sed -n 's/^Token for the output markers: //p' "` + dir + `/stdin")
printf '=== SKIGARD MERGE %s BEGIN ===\n=== FILE %s path=SKILL.md ===\n---\nname: alpha\ndescription: d\n---\nmerged\n=== END FILE %s ===\n=== SKIGARD MERGE %s END ===\n' "$tok" "$tok" "$tok" "$tok"
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	a.Runner = review.Runner{Environ: []string{"PATH=/usr/bin:/bin", "GITHUB_TOKEN=topsecret"}, BinOverride: bin, NoOSSandbox: true}
	if _, err := exec.LookPath("sed"); err != nil {
		t.Skip("no sed")
	}
	m, err := a.AgentMerge(context.Background(), "alpha", MergeOpts{Harness: "pi", Note: "NOTE-REAL-77"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Isolation.Mode != review.IsoPolicyOnly {
		t.Errorf("isolation = %q", m.Isolation.Mode)
	}
	argv, _ := os.ReadFile(filepath.Join(dir, "argv"))
	stdin, _ := os.ReadFile(filepath.Join(dir, "stdin"))
	if strings.Contains(string(argv), "NOTE-REAL-77") || strings.Contains(string(argv), "MINE2") || !strings.Contains(string(argv), "--no-tools") {
		t.Errorf("argv: %s", argv)
	}
	if !strings.Contains(string(stdin), "NOTE-REAL-77") || !strings.Contains(string(stdin), "MINE2") {
		t.Errorf("stdin lacks the note or the local tree")
	}
}
