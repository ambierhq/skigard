package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/core"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
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

// setMode writes the settings the way a human would have (`config set` needs a terminal).
func setMode(t *testing.T, home, mode string, threshold int) {
	t.Helper()
	h := store.Home{Root: home}
	man, err := store.LoadManifest(h)
	if err != nil {
		t.Fatal(err)
	}
	man.Settings.Mode, man.Settings.Threshold = mode, threshold
	if err := store.SaveManifest(h, man); err != nil {
		t.Fatal(err)
	}
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
	setMode(t, home, store.ModeStrict, 0)
	appendTo(t, filepath.Join(home, "skills", "demo", "SKILL.md"), "mine\n")
	code, out, errs := run(t, home, "adopt", "demo")
	if code != 0 || errs != "" {
		t.Fatalf("%d %q", code, errs)
	}
	for _, want := range []string{
		"✔ staged your local edits of \"demo\" in quarantine\n  (skills/, the lock and installs are unchanged)\nscan: PASS",
		"\nsafety score: 10/10 (higher is safer; ok; threshold 7; mode strict)\n  - no findings\n  - strict mode: a human must confirm with `skigard approve demo`\n\n● \"demo\" is in quarantine and waiting for a human.\nnext: skigard diff demo   (your edits vs the original upstream)\n      skigard review demo   (optional)\n      skigard approve demo  (in a real terminal)\n",
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
	setMode(t, home, store.ModeStrict, 0)
	code, out, errs := run(t, home, "edit", "demo")
	if code != 0 || errs != "" {
		t.Fatalf("%d %q", code, errs)
	}
	wc := store.Home{Root: home}.QuarantineSkill("demo")
	for _, want := range []string{
		"✔ editable working copy of \"demo\"\n  " + wc + "\n  (skills/, the lock and installs are unchanged; the approved version stays live)\n",
		"\nnext: edit the files in " + wc + "\n      skigard diff demo     (see your changes)\n      skigard adopt demo    (finalize; approves automatically when your mode and threshold allow)\n      skigard approve demo  (in a real terminal, if adopt left it staged)\n",
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
		"\n● \"demo\" is in quarantine and waiting for a human.\nnext: skigard diff demo   (your edits vs the approved version)\n      skigard review demo   (optional)\n      skigard approve demo  (in a real terminal)\n",
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
	setMode(t, home, store.ModeStrict, 0)
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

func TestLockProtectOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are synthetic on Windows")
	}
	home, _ := autoApproved(t)
	dir := filepath.Join(home, "skills", "demo")
	util.MakeWritable(dir) // an old-style approval
	code, out, errs := run(t, home, "verify")
	if code != 0 || errs != "" || !strings.Contains(out, "warn  demo: skills/demo is writable; edit it with `skigard edit demo`, and run `skigard lock protect` to make approved skills read-only\n") {
		t.Fatalf("%d %q %q", code, out, errs)
	}
	if code, _, _ := run(t, home, "verify", "--strict"); code != 1 {
		t.Fatalf("--strict code %d", code)
	}
	code, out, errs = run(t, home, "lock", "protect")
	if code != 0 || errs != "" {
		t.Fatalf("%d %q", code, errs)
	}
	if !strings.HasPrefix(out, "✔ demo is now read-only\n") || !strings.Contains(out, "(permissions only: content, hashes, the lock and installs are unchanged)\n") || nextBlock(out) == "" {
		t.Fatalf("%q", out)
	}
	code, out, _ = run(t, home, "lock", "protect")
	if code != 0 || out != "1 already read-only\n" {
		t.Fatalf("%d %q", code, out)
	}
	if code, out, _ := run(t, home, "verify", "--strict"); code != 0 || strings.Contains(out, "writable") {
		t.Fatalf("%d %q", code, out)
	}
	if code, out, _ := run(t, home, "lock", "protect", "demo", "--json"); code != 0 || !strings.Contains(out, `"note": "already read-only"`) {
		t.Fatalf("%d %q", code, out)
	}
	if code, _, _ := run(t, home, "lock", "protect", "nope"); code != util.ExitUsage {
		t.Fatalf("unknown name: %d", code)
	}
}

func TestLockProtectHelp(t *testing.T) {
	home := filepath.Join(t.TempDir(), "h")
	code, out, _ := run(t, home, "lock", "protect", "-h")
	if code != 0 || !strings.Contains(out, "skigard lock protect [NAME...]") || !strings.Contains(out, "Make approved skills read-only") {
		t.Fatalf("%d %q", code, out)
	}
}

func TestAdoptAutoApprovedOutput(t *testing.T) {
	home, _ := autoApproved(t)
	appendTo(t, filepath.Join(home, "skills", "demo", "SKILL.md"), "mine\n")
	code, out, errs := run(t, home, "adopt", "demo")
	// Installs are symlinks on POSIX (refreshed) and copies on Windows (stale).
	wantErr := "note: run `skigard sync` to update the install records of \"demo\"\n"
	wants := []string{
		"\nsafety score: 10/10 (higher is safer; ok; threshold 7; mode auto)\n  - no findings\n\n✔ approved automatically (recorded in the lock as mode = auto)\n  refreshed ",
		"(symlink to skills/; shows the new version)\n\n",
	}
	if runtime.GOOS == "windows" {
		wantErr = "note: copy installs of \"demo\" still hold the old version; run `skigard sync`\n"
		wants = []string{
			"\nsafety score: 10/10 (higher is safer; ok; threshold 7; mode auto)\n  - no findings\n\n✔ approved automatically (recorded in the lock as mode = auto)\n  stale ",
			"(copy install)\n\n",
		}
	}
	if code != 0 || errs != wantErr {
		t.Fatalf("%d %q", code, errs)
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b") || strings.Contains(out, "next:") || strings.Contains(out, "staged") {
		t.Errorf("unexpected text:\n%s", out)
	}
	if _, lout, _ := run(t, home, "list"); strings.Contains(lout, "quarantined") {
		t.Errorf("still quarantined: %s", lout)
	}
	// verify --require-human flags the automatic approval; the fork baseline is kept.
	if code, vout, verr := run(t, home, "verify", "--require-human"); code == 0 || !strings.Contains(vout+verr, "auto") {
		t.Errorf("require-human: %d %s %s", code, vout, verr)
	}
	ls, err := store.LoadLock(store.Home{Root: home})
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ls.Get("demo")
	if e.Approval.Mode != store.ApprovalAuto || e.UpstreamHash == "" || e.UpstreamHash == e.ContentHash {
		t.Errorf("lock: %+v", e)
	}
}

func TestAdoptBelowThresholdStaysStaged(t *testing.T) {
	home, _ := autoApproved(t)
	appendTo(t, filepath.Join(home, "skills", "demo", "SKILL.md"), "mine\n")
	code, out, errs := run(t, home, "adopt", "demo", "--threshold", "10", "--review")
	_ = code
	for _, want := range []string{"review did not run", "not auto-installed because the requested review did not run", "WARNING \"demo\" was NOT approved; it is still in quarantine.\nnext: skigard diff demo"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s%s", want, out, errs)
		}
	}
	if code != 0 {
		t.Errorf("exit %d", code)
	}
	if _, lout, _ := run(t, home, "list"); !strings.Contains(lout, "quarantined") {
		t.Errorf("list: %s", lout)
	}
}

func TestAdoptJSON(t *testing.T) {
	home, _ := autoApproved(t)
	appendTo(t, filepath.Join(home, "skills", "demo", "SKILL.md"), "mine\n")
	_, jout, errs := run(t, home, "adopt", "demo", "--json")
	var m map[string]any
	if err := json.Unmarshal([]byte(jout), &m); err != nil || errs != "" {
		t.Fatalf("%v %q %s", err, errs, jout)
	}
	if m["decision"] != "adopted" || m["mode"] != "auto" || m["reasons"] == nil || m["score"] == nil || m["upstream_hash"] == "" {
		t.Errorf("%v", m)
	}
	// unchanged (nothing to adopt now): in strict mode a staged edit reports its decision.
	setMode(t, home, store.ModeStrict, 0)
	appendTo(t, filepath.Join(home, "skills", "demo", "SKILL.md"), "again\n")
	_, jout, _ = run(t, home, "adopt", "demo", "--json")
	m = map[string]any{}
	if err := json.Unmarshal([]byte(jout), &m); err != nil || m["decision"] != "needs-approval" || m["mode"] != "strict" {
		t.Errorf("%v %s", err, jout)
	}
}

func TestAdoptAndEditHelp(t *testing.T) {
	for _, c := range []string{"adopt", "edit"} {
		_, out, errs := run(t, filepath.Join(t.TempDir(), "h"), c, "-h")
		got := out + errs
		for _, want := range map[string][]string{
			"adopt": {"-mode", "-threshold", "-review", "-review-harness"},
			"edit":  {"edit NAME"},
		}[c] {
			if !strings.Contains(got, want) {
				t.Errorf("%s -h missing %q:\n%s", c, want, got)
			}
		}
		if strings.Contains(got, "force-install") {
			t.Errorf("%s -h mentions force-install", c)
		}
	}
}

// A declined strict prompt is reported once, by the final "skigard: cancelled" line, not
// again inside the outcome block; other errors still show there.
func TestNeedsApprovalReportsCancelOnce(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	render := func(err error) string {
		var b bytes.Buffer
		c := &ctx{out: &b, errw: &b}
		o := core.AddOutcome{Name: "demo", Mode: store.ModeStrict, Decision: core.DecisionNeedsApproval, Err: err,
			Report: &scan.Report{Verdict: "pass", Coverage: "complete"}}
		if e := printOutcomes(c, []core.AddOutcome{o}); e != nil {
			t.Fatal(e)
		}
		return b.String()
	}
	if got := render(util.Cancelled("cancelled: nothing was approved")); strings.Contains(got, "cancelled") {
		t.Errorf("cancel repeated in the outcome block:\n%s", got)
	}
	if got := render(util.Errf("boom")); !strings.Contains(got, "(boom)") {
		t.Errorf("a non-cancel error should still be shown:\n%s", got)
	}
}

// verify only claims the marketplace agrees once it has been built.
func TestVerifyOKNamesTheMarketplaceOnlyWhenBuilt(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	home := filepath.Join(t.TempDir(), "h")
	run(t, home, "init")
	if code, out, errs := run(t, home, "verify"); code != 0 || !strings.Contains(out, "ok lock, store and installs agree") || strings.Contains(out, "marketplace") {
		t.Fatalf("no marketplace: %d %q %q", code, out, errs)
	}
	if code, out, errs := run(t, home, "marketplace", "build"); code != 0 {
		t.Fatalf("build: %d %q %q", code, out, errs)
	}
	if code, out, errs := run(t, home, "verify"); code != 0 || !strings.Contains(out, "lock, store, installs and marketplace agree") {
		t.Fatalf("built: %d %q %q", code, out, errs)
	}
}
