package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/core"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/store"
)

// autoApproved returns a home with "demo" auto-approved from a local source.
func autoApproved(t *testing.T) (home, src string) {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	home = filepath.Join(t.TempDir(), "h")
	run(t, home, "init")
	// `config set mode` needs a human terminal; write the settings the way a human would have.
	h := store.Home{Root: home}
	man, err := store.LoadManifest(h)
	if err != nil {
		t.Fatal(err)
	}
	off := false
	man.Settings.Mode, man.Settings.AutoReview = store.ModeAuto, &off
	if err := store.SaveManifest(h, man); err != nil {
		t.Fatal(err)
	}
	addHarness(t, home)
	src = mkSkill(t, "hello\n")
	if code, out, errs := run(t, home, "add", src, "--skill", "demo", "--to", "mine"); code != 0 {
		t.Fatalf("add: %s %s", out, errs)
	}
	return home, src
}

func appendTo(t *testing.T, p, s string) {
	t.Helper()
	os.Chmod(p, 0o644) // approved skills are read-only
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

// nextBlock returns the "next:" hint and its aligned continuation lines.
func nextBlock(out string) string {
	i := strings.LastIndex(out, "next: ")
	if i < 0 {
		return ""
	}
	return out[i:]
}

func TestAdoptOutputStyle(t *testing.T) {
	home, _ := autoApproved(t)
	appendTo(t, filepath.Join(home, "skills", "demo", "SKILL.md"), "mine\n")
	code, out, errs := run(t, home, "adopt", "demo")
	if code != 0 || errs != "" {
		t.Fatalf("%d %q", code, errs)
	}
	for _, want := range []string{
		"✔ staged your local edits of \"demo\" in quarantine\n  (skills/, the lock and installs are unchanged)\nscan: PASS",
		"\n\nnext: skigard diff demo   (your edits vs the original upstream)\n      skigard review demo   (optional)\n      skigard approve demo  (in a real terminal)\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Error("escape codes without a terminal")
	}
	run(t, home, "reject", "demo")
	_, jout, _ := run(t, home, "adopt", "demo", "--json")
	if strings.Contains(jout, "next:") || !strings.HasPrefix(strings.TrimSpace(jout), "{") {
		t.Errorf("json carries human text: %s", jout)
	}
}

func TestUpdateOutputStyle(t *testing.T) {
	home, src := autoApproved(t)
	if _, out, _ := run(t, home, "update"); !strings.Contains(out, "demo: up to date\n") {
		t.Errorf("up to date: %q", out)
	}
	// upstream changes, clean local copy: auto mode approves it
	appendTo(t, filepath.Join(src, "demo", "SKILL.md"), "v2\n")
	_, out, _ := run(t, home, "update")
	if !strings.Contains(out, "\nsafety score: 10/10 (higher is safer; ok; threshold 7)\n  - no findings\n\n✔ approved automatically (recorded in the lock as mode = auto)\n\n") {
		t.Errorf("auto: %s", out)
	}
	if strings.Contains(out, "new commit:") {
		t.Errorf("empty commit printed: %s", out)
	}
	// local edits plus upstream change: held in quarantine with a next block
	appendTo(t, filepath.Join(home, "skills", "demo", "SKILL.md"), "mine\n")
	appendTo(t, filepath.Join(src, "demo", "SKILL.md"), "v3\n")
	_, out, _ = run(t, home, "update")
	want := "● \"demo\" is in quarantine; it differs from your local copy.\n" +
		"next: skigard diff demo   (your copy vs new upstream)\n" +
		"      skigard merge demo --pick   (keep your edits; or --with HARNESS [--note TEXT])\n" +
		"      skigard approve demo  (in a real terminal; takes upstream wholesale and drops your edits)\n\n"
	if !strings.Contains(out, want) {
		t.Errorf("local changes: %s", out)
	}
}

func TestMergeSummaryStyle(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var b bytes.Buffer
	c := &ctx{out: &b, errw: &b}
	m := &core.Merged{
		Quarantined: &core.Quarantined{Name: "demo", Meta: &store.Meta{MergedBy: "pi"}, Report: &scan.Report{Verdict: "pass", Coverage: "complete"}},
		Mode:        core.MergeAgent,
	}
	printMerged(c, m)
	got := b.String()
	for _, want := range []string{
		"✔ staged the merge of \"demo\" from agent pi (isolation: ",
		"in quarantine\n  (skills/, the lock and installs are unchanged)\nscan: PASS",
		"\n\nnext: skigard diff demo   (your current copy vs the merge)\n      skigard review demo   (optional)\n      skigard approve demo  (in a real terminal)\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if nextBlock(got) == "" {
		t.Error("no next block")
	}
}

func TestEditOutputStyleAndWorkflow(t *testing.T) {
	home, _ := autoApproved(t)
	code, out, errs := run(t, home, "edit", "demo")
	if code != 0 || errs != "" {
		t.Fatalf("%d %q", code, errs)
	}
	wc := store.Home{Root: home}.QuarantineSkill("demo")
	for _, want := range []string{
		"✔ editable working copy of \"demo\"\n  " + wc + "\n  (skills/, the lock and installs are unchanged; the approved version stays live)\n",
		"\nnext: edit the files in " + wc + "\n      skigard diff demo     (see your changes)\n      skigard adopt demo    (finalize: hash and scan the edit)\n      skigard approve demo  (in a real terminal)\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Error("escape codes without a terminal")
	}
	if _, lout, _ := run(t, home, "list"); !strings.Contains(lout, "editing") {
		t.Errorf("list: %s", lout)
	}
	// An unfinalized working copy cannot be reviewed or approved.
	if code, _, errs := run(t, home, "review", "demo", "--manual"); code == 0 || !strings.Contains(errs, "editable working copy") {
		t.Errorf("review: %d %s", code, errs)
	}
	// Unchanged: adopt says so and leaves the working copy.
	_, out, _ = run(t, home, "adopt", "demo")
	if !strings.Contains(out, "no changes: the working copy of \"demo\" is identical to the approved version") {
		t.Errorf("no-change adopt: %s", out)
	}
	appendTo(t, filepath.Join(wc, "SKILL.md"), "mine\n")
	code, out, errs = run(t, home, "adopt", "demo")
	if code != 0 || errs != "" {
		t.Fatalf("%d %q", code, errs)
	}
	for _, want := range []string{
		"✔ finalized your edit of \"demo\" in quarantine\n  (skills/, the lock and installs are unchanged)\nscan: PASS",
		"\n\nnext: skigard diff demo   (your edits vs the approved version)\n      skigard review demo   (optional)\n      skigard approve demo  (in a real terminal)\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	_, out, _ = run(t, home, "diff", "demo")
	if !strings.Contains(out, "comparing the approved version in skills/ with your edit in quarantine\n") || !strings.Contains(out, "mine") {
		t.Errorf("diff: %s", out)
	}
	run(t, home, "reject", "demo")
	_, jout, _ := run(t, home, "edit", "demo", "--json")
	if strings.Contains(jout, "next:") || !strings.HasPrefix(strings.TrimSpace(jout), "{") {
		t.Errorf("json carries human text: %s", jout)
	}
}

func TestInPlaceAdoptDiffSaysWhatItCompares(t *testing.T) {
	home, _ := autoApproved(t)
	appendTo(t, filepath.Join(home, "skills", "demo", "SKILL.md"), "mine\n")
	run(t, home, "adopt", "demo")
	_, out, _ := run(t, home, "diff", "demo")
	if !strings.Contains(out, "comparing the original upstream version your edit started from with your edit in quarantine\n") || !strings.Contains(out, "mine") {
		t.Errorf("diff: %s", out)
	}
}

func TestVerifyPointsAtAdoptAndEdit(t *testing.T) {
	home, _ := autoApproved(t)
	appendTo(t, filepath.Join(home, "skills", "demo", "SKILL.md"), "mine\n")
	_, out, _ := run(t, home, "verify")
	if !strings.Contains(out, "skills/ is read-only and must not be edited in place") || !strings.Contains(out, "`skigard adopt demo`") || !strings.Contains(out, "`skigard edit demo`") {
		t.Errorf("verify: %s", out)
	}
}
