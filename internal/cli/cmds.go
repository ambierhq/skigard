package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/ambierhq/skigard/internal/core"
	"github.com/ambierhq/skigard/internal/harness"
	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/review"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/term"
	"github.com/ambierhq/skigard/internal/util"
)

func cmdVersion(c *ctx, args []string) error {
	if _, err := parse(c.flags("version"), args); err != nil {
		return err
	}
	c.printf("skigard %s\n", Version)
	return nil
}

func cmdInit(c *ctx, args []string) error {
	fs := c.flags("init")
	git := fs.Bool("git", false, "also run `git init` in the home")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	if err := c.app.Home.Init(); err != nil {
		return err
	}
	c.printf("skigard home: %s\n", c.app.Home.Root)
	if *git {
		if _, err := os.Stat(c.app.Home.Root + "/.git"); err == nil {
			c.printf("already a git repository\n")
		} else if out, err := exec.CommandContext(c.ctx, "git", "-C", c.app.Home.Root, "init", "-q").CombinedOutput(); err != nil { //nolint:gosec // fixed git arguments; the path is the user's own home
			return util.Errf("git init: %v: %s", err, strings.TrimSpace(string(out)))
		} else {
			c.printf("git repository created (quarantine/ and state/ are ignored)\n")
		}
	}
	return nil
}

func cmdDetect(c *ctx, args []string) error {
	fs := c.flags("detect")
	all := fs.Bool("all", false, "include harnesses that were not found")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	ds, err := c.app.Detect()
	if err != nil {
		return err
	}
	var shown []harness.Detection
	for _, d := range ds {
		if *all || d.Status != harness.Absent {
			shown = append(shown, d)
		}
	}
	if c.json {
		if shown == nil {
			shown = []harness.Detection{}
		}
		return c.printJSON(shown)
	}
	if len(shown) == 0 {
		c.printf("no agent harness detected (try --all, or `skigard harness add`)\n")
		return nil
	}
	st := c.style(false)
	var rows [][]string
	var unv int
	for _, d := range shown {
		v := "yes"
		if !d.Verified {
			v, unv = "no", unv+1
		}
		rows = append(rows, []string{d.Status, d.ID, d.Name, v, d.GlobalAbs})
	}
	c.printf("%s", term.Table(st, []string{"STATUS", "ID", "NAME", "VERIFIED", "SKILLS DIR"}, rows, func(_, col int, cell string) string {
		switch col {
		case 0, 3:
			return st.Level(cell)
		case 1:
			return st.Bold(cell)
		}
		return cell
	}))
	if unv > 0 {
		c.printf("\n%s\n", st.Dim(fmt.Sprintf("%d harness(es) are unverified: listed, but installs need --include-unverified.", unv)))
	}
	return nil
}

func cmdAdd(c *ctx, args []string) error {
	fs := c.flags("add")
	skillName := fs.String("skill", "", "take one skill by name or path")
	all := fs.Bool("all", false, "take every skill found")
	name := fs.String("name", "", "store the skill under this name")
	force := fs.Bool("force", false, "allow replacing a name that is already approved")
	mode := fs.String("mode", "", "strict or auto (default: your setting; inside an agent only a stricter one than the setting is allowed)")
	threshold := fs.Int("threshold", 0, "minimum safety score 1..10 for auto install (default: your setting; inside an agent it cannot be lower)")
	rev := fs.Bool("review", false, "run an advisory review before scoring (sends the skill to a model provider)")
	revHarness := fs.String("review-harness", "", "harness that runs the review (default: your saved choice)")
	forceInstall := fs.Bool("force-install", false, "install whatever the score, recorded as forced (human terminal only, refused inside an agent)")
	quarOnly := fs.Bool("quarantine-only", false, "only fetch and scan; leave the skill in quarantine")
	var io core.InstallOpts
	to := fs.String("to", "", "comma-separated harness ids (default: detected, verified harnesses)")
	fs.StringVar(&io.Scope, "scope", "user", "user or project")
	fs.StringVar(&io.Project, "project", "", "project root for --scope project (default: current directory)")
	fs.BoolVar(&io.Copy, "copy", false, "copy instead of symlink")
	fs.BoolVar(&io.IncludeUnverified, "include-unverified", false, "also install to harnesses whose paths are not verified")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := needArgs(pos, 1, "add SOURCE [--skill N | --all]"); err != nil {
		return err
	}
	if *quarOnly && *forceInstall {
		return util.Errf("--quarantine-only and --force-install contradict each other")
	}
	if err := c.open(); err != nil {
		return err
	}
	ao := core.AddOpts{Skill: *skillName, All: *all, Name: *name, Force: *force}
	if *quarOnly {
		qs, err := c.app.Add(c.ctx, pos[0], ao)
		if err != nil {
			return err
		}
		return printQuarantined(c, qs)
	}
	io.To = split(*to)
	ocs, err := c.app.AddAndInstall(c.ctx, pos[0], core.AddInstallOpts{
		Add: ao, Mode: *mode, Threshold: *threshold, Review: *rev, ReviewOpts: core.ReviewOpts{Harness: *revHarness},
		ForceInstall: *forceInstall, Install: io, Confirm: ttyConfirm,
	})
	if perr := printOutcomes(c, ocs); perr != nil {
		return perr
	}
	if err != nil {
		return err
	}
	return outcomeStatus(ocs)
}

func printQuarantined(c *ctx, qs []*core.Quarantined) error {
	if c.json {
		var out []map[string]any
		for _, q := range qs {
			out = append(out, map[string]any{"name": q.Name, "content_hash": q.Hash, "source": q.Meta.Source, "commit": q.Meta.Commit, "scan": q.Report})
		}
		return c.printJSON(out)
	}
	for _, q := range qs {
		c.printf("quarantined %q from %s\n", q.Name, util.Clean(q.Meta.Source))
		if q.Meta.Commit != "" {
			c.printf("  commit: %s\n", q.Meta.Commit)
		}
		c.writeScan(q.Report)
		c.printf("\nnext: skigard review %s   (optional)\n      skigard approve %s  (in a real terminal)\n\n", q.Name, q.Name)
	}
	return nil
}

// outcomeStatus turns "nothing was installed" into a non-zero exit, so scripts
// and agents can tell a held-back skill from an installed one.
func outcomeStatus(ocs []core.AddOutcome) error {
	held := 0
	for _, o := range ocs {
		switch o.Decision {
		case core.DecisionInstalled, core.DecisionForced:
		default:
			held++
			if o.Err != nil && util.CodeOf(o.Err) == util.ExitCancelled {
				return o.Err
			}
		}
	}
	if held > 0 {
		return util.Policyf("%d skill(s) left in quarantine, not installed", held)
	}
	return nil
}

func printOutcomes(c *ctx, ocs []core.AddOutcome) error {
	if c.json {
		out := []map[string]any{}
		for _, o := range ocs {
			m := map[string]any{
				"name": o.Name, "content_hash": o.Hash, "mode": o.Mode, "decision": o.Decision,
				"score": o.Safety, "reasons": o.Reasons, "scan": o.Report, "installed": o.Installed, "notes": o.Notes,
			}
			if o.Review != nil {
				m["review"] = o.Review
			}
			if o.Err != nil {
				m["error"] = util.Clean(o.Err.Error())
			}
			out = append(out, m)
		}
		return c.printJSON(out)
	}
	for _, o := range ocs {
		if o.Decision == core.DecisionFailed {
			c.printf("%q was not installed; it stopped before a decision was made\n\n", o.Name)
			continue
		}
		c.writeScan(o.Report)
		st := c.style(false)
		if o.Review != nil {
			c.printf("review: %s %s\n", verdictStyle(st, o.Review.Verdict), st.Dim("(advisory)"))
		}
		c.printf("\n")
		sf := o.Safety
		sf.Score, sf.Reasons = o.Score, o.Reasons
		scan.WriteScore(c.out, st, sf, o.Mode)
		switch o.Decision {
		case core.DecisionInstalled, core.DecisionForced:
			word := "approved and installed"
			if o.Decision == core.DecisionForced {
				word = "FORCED install (recorded in the lock as mode = forced)"
			} else if o.Mode == store.ModeAuto {
				word = "approved automatically and installed (recorded in the lock as mode = auto, by " + store.AutoApprover + ")"
			}
			c.printf("%s %s %q\n", st.Green("✔"), word, o.Name)
			for _, g := range o.Installed {
				c.printf("  %s %s %s %s %s\n", st.Green("installed"), st.Bold(g.Name), st.Dim("->"), g.Path, st.Dim("("+g.Mode+"; "+strings.Join(g.Harnesses, ", ")+")"))
			}
		case core.DecisionWarned:
			c.printf("%s %q was NOT installed; it is still in quarantine.\n", st.Tag("33", "WARNING"), o.Name)
			c.nextSteps("", "",
				"skigard diff "+o.Name,
				"skigard review "+o.Name+"   (optional)",
				"skigard approve "+o.Name+"  (in a real terminal)",
				"skigard install "+o.Name,
				"or: skigard add ... --force-install (human terminal only)")
		case core.DecisionNeedsApproval:
			c.printf("%s %q is in quarantine and waiting for a human.\n", st.Yellow("●"), o.Name)
			if o.Err != nil {
				c.printf("  (%s)\n", util.Clean(o.Err.Error()))
			}
			c.nextSteps("", "",
				"skigard review "+o.Name+"   (optional)",
				"skigard approve "+o.Name+"  (in a real terminal)",
				"skigard install "+o.Name)
		default:
			c.printf("%q was approved but installing failed: %s\n", o.Name, util.Clean(errText(o.Err)))
		}
		for _, n := range o.Notes {
			c.printf("%s %s\n", st.Yellow("note:"), n)
		}
		c.printf("\n")
	}
	return nil
}

// errText is the text of err, or "unknown error" when it is nil.
func errText(err error) string {
	if err == nil {
		return "unknown error"
	}
	return err.Error()
}

func cmdConfig(c *ctx, args []string) error {
	name := "config"
	for _, a := range args { // name the verb so `config set -h` shows its usage
		if !strings.HasPrefix(a, "-") {
			if a == "get" || a == "set" {
				name += " " + a
			}
			break
		}
	}
	fs := c.flags(name)
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return util.Errf("usage: skigard config %s (see `skigard config -h`)", verbList("config"))
	}
	if pos[0] != "get" && pos[0] != "set" {
		return util.Errf("unknown config command %q (see `skigard config -h`)", pos[0])
	}
	if err := c.open(); err != nil {
		return err
	}
	switch pos[0] {
	case "get":
		keys := core.ConfigKeys
		if len(pos) > 1 {
			keys = pos[1:2]
		}
		vals := map[string]string{}
		for _, k := range keys {
			v, err := c.app.ConfigGet(k)
			if err != nil {
				return err
			}
			vals[k] = v
		}
		if c.json {
			return c.printJSON(vals)
		}
		for _, k := range keys {
			if len(pos) > 1 {
				c.printf("%s\n", vals[k])
			} else {
				note := ""
				if k == "review_harness" {
					switch man, err := store.LoadManifest(c.app.Home); {
					case vals[k] == "":
						vals[k], note = "(none detected)", ""
					case err == nil && man.Settings.ReviewHarness == "":
						note = " (default: first detected reviewer)"
					}
				}
				c.printf("%s = %s%s\n", k, vals[k], note)
			}
		}
		return nil
	case "set":
		if len(pos) != 3 {
			return util.Errf("usage: skigard config set KEY VALUE")
		}
		old, err := c.app.ConfigSet(pos[1], pos[2], ttyConfirm)
		if err != nil {
			return err
		}
		c.printf("%s: %s -> %s\n", pos[1], old, pos[2])
		if pos[1] == "marketplace" {
			if v, _ := strconv.ParseBool(pos[2]); v {
				c.printf("marketplace/ built; it is rebuilt after every approval and plugin change, and verify requires it\n")
			} else {
				c.printf("marketplace/ was left in place; verify checks it only while it exists\n")
			}
		}
		return nil
	}
	return util.Errf("unknown config command %q", pos[0]) // unreachable: validated above
}

func cmdScan(c *ctx, args []string) error {
	fs := c.flags("scan")
	format := fs.String("format", "text", "text, json or sarif")
	failOn := fs.String("fail-on", "high", "exit 1 at this severity or above: low, medium, high, critical, none")
	incomplete := fs.Bool("fail-on-incomplete", false, "exit 4 when scan coverage is incomplete")
	out := fs.String("output", "", "write the report to a file")
	plain := fs.Bool("plain", false, "text format: no colours (the default when not on a terminal)")
	all := fs.Bool("all", false, "text format: show every finding instead of a summary")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := needArgs(pos, 1, "scan NAME|PATH"); err != nil {
		return err
	}
	if c.json {
		*format = "json"
	}
	if err := c.open(); err != nil {
		return err
	}
	rep, _, err := c.app.Scan(pos[0])
	if err != nil {
		return err
	}
	w := c.out
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	switch *format {
	case "text":
		if st := c.style(*plain); st.On && *out == "" {
			scan.WriteStyledOpts(w, rep, st, c.scanOptions(*all))
		} else {
			opt := c.scanOptions(*all)
			if *out != "" {
				opt.Width = term.DefaultWidth
			}
			scan.WriteTextOpts(w, rep, opt)
		}
	case "json":
		err = scan.WriteJSON(w, rep)
	case "sarif":
		err = scan.WriteSARIF(w, rep)
	default:
		return util.Errf("unknown --format %q", *format)
	}
	if err != nil {
		return err
	}
	if *failOn != "none" {
		threshold := scan.Severity(*failOn)
		if threshold.Rank() == 0 {
			return util.Errf("unknown --fail-on %q", *failOn)
		}
		for _, f := range rep.Findings {
			if f.Severity.Rank() >= threshold.Rank() {
				return util.Policyf("%d finding(s) at or above %s", len(rep.Findings), *failOn)
			}
		}
	}
	if *incomplete && rep.Coverage != scan.CoverageComplete {
		return &util.Error{Msg: "scan coverage is incomplete", Code: util.ExitIncomplete}
	}
	return nil
}

func cmdReview(c *ctx, args []string) error {
	if len(args) > 0 && args[0] == groups["review"][0].name {
		return cmdReviewRecord(c, args[1:])
	}
	fs := c.flags("review")
	harnessID := fs.String("harness", "", "harness to run as the reviewer (default: your saved choice, or you pick)")
	model := fs.String("model", "", "model for the reviewer, if it supports --model")
	manual := fs.Bool("manual", false, "print the review prompt instead of running a harness")
	unverified := fs.Bool("allow-unverified", false, "allow reviewer templates that have not been verified")
	policyOnly := fs.Bool("allow-policy-only", false, "if no OS sandbox works here, run a verified reviewer isolated by policy only (advisory review; has no effect on approval)")
	list := fs.Bool("list", false, "list harnesses that can review, then exit")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	rv, err := c.app.Reviewers()
	if err != nil {
		return err
	}
	if *list {
		if c.json {
			return c.printJSON(rv)
		}
		for _, r := range rv {
			c.printf("%-12s installed=%-5v verified=%-5v %s\n", r.ID, r.Installed, r.Verified, r.Note)
		}
		return nil
	}
	if err := needArgs(pos, 1, "review NAME [--harness ID | --manual]"); err != nil {
		return err
	}
	name := pos[0]
	if *manual {
		p, err := c.app.ManualPrompt(name)
		if err != nil {
			return err
		}
		c.printf("%s\n\n---\nWhen done, record your verdict (this does not approve anything):\n  skigard review record %s --verdict pass|concerns|fail --summary \"...\"\n", p, name)
		return nil
	}
	if err := review.Supported(); err != nil {
		return err
	}
	id := *harnessID
	if id == "" {
		man, err := store.LoadManifest(c.app.Home)
		if err != nil {
			return err
		}
		id = c.app.DefaultReviewHarness(man.Settings)
	}
	if id == "" {
		if id, err = pickReviewer(c, rv); err != nil {
			return err
		}
	}
	iso, isoErr := review.SandboxStatus()
	if isoErr != nil {
		c.printf("note: no OS sandbox is available here (%s)\n", util.OneLine(isoErr.Error(), 200))
	}
	c.printf("reviewing %q with %s (tools disabled, isolation: %s, advisory). This uses your own %s plan/credits once.\n", name, id, iso.Mode, id)
	rec, err := c.app.Review(c.ctx, name, core.ReviewOpts{Harness: id, Model: *model, AllowUnverified: *unverified, AllowPolicyOnly: *policyOnly})
	if err != nil {
		return err
	}
	if c.json {
		return c.printJSON(rec)
	}
	if st := c.style(false); st.On {
		printReviewStyled(c, st, name, rec)
		return nil
	}
	c.printf("review verdict: %s (coverage %s, advisory, isolation %s)\n  %s\n", strings.ToUpper(rec.Verdict), rec.Coverage, rec.Isolation, util.Clean(rec.Summary))
	for _, f := range rec.Findings {
		c.printf("  [%s] %s:%d %s\n", f.Severity, util.Clean(f.File), f.Line, util.Clean(f.Why))
	}
	if rec.Dropped > 0 {
		c.printf("  (%d finding(s) dropped: their quotes were not found in the files)\n", rec.Dropped)
	}
	c.printf("next: skigard approve %s\n", name)
	return nil
}

func printReviewStyled(c *ctx, st term.Style, name string, rec *review.Record) {
	c.printf("\n%s  %s  %s\n", st.Bold(util.Clean(name)), st.Bold("review"), verdictStyle(st, rec.Verdict))
	c.printf("%s %s   %s %s   %s\n\n", st.Dim("coverage"), rec.Coverage, st.Dim("isolation"), rec.Isolation, st.Dim("advisory: it can only raise concern"))
	c.printf("%s\n", term.Wrap(util.Clean(rec.Summary), 88, "  "))
	if len(rec.Findings) > 0 {
		c.printf("\n%s  %s\n", st.Bold("FINDINGS"), st.Dim(fmt.Sprint(len(rec.Findings))))
	}
	for _, f := range rec.Findings {
		loc := util.Clean(f.File)
		if f.Line > 0 {
			loc += fmt.Sprintf(":%d", f.Line)
		}
		c.printf("\n  %s %s\n%s\n", sevLabelPad(st, f.Severity), st.Cyan(loc), term.Wrap(util.Clean(f.Why), 84, "    "))
	}
	if rec.Dropped > 0 {
		c.printf("\n  %s\n", st.Dim(fmt.Sprintf("(%d finding(s) dropped: their quotes were not found in the files)", rec.Dropped)))
	}
	c.printf("\n%s skigard approve %s\n", st.Dim("next:"), name)
}

func verdictStyle(st term.Style, v string) string { return st.Verdict(util.Clean(v)) }

func sevLabelPad(st term.Style, sev string) string {
	return sevStyle(st, fmt.Sprintf("%-8s", sev))
}

func pickReviewer(c *ctx, rv []core.Reviewer) (string, error) {
	var usable []core.Reviewer
	for _, r := range rv {
		if r.Installed {
			usable = append(usable, r)
		}
	}
	if len(usable) == 0 {
		return "", util.Errf("no harness that can review headlessly was found (claude-code, pi, omp). Use --manual, or install one")
	}
	if !stdinIsTerminal() {
		var ids []string
		for _, r := range usable {
			ids = append(ids, r.ID)
		}
		return "", util.Errf("choose a reviewer with --harness (available: %s) or save one with `skigard review NAME` in a terminal", strings.Join(ids, ", "))
	}
	fmt.Fprintln(os.Stderr, "Which harness should review this skill?")
	for i, r := range usable {
		tag := ""
		if r.Running {
			tag += " (running this command)"
		}
		if !r.Verified {
			tag += " (unverified)"
		}
		fmt.Fprintf(os.Stderr, "  %d) %s%s\n", i+1, r.ID, tag)
	}
	n, err := strconv.Atoi(ask("number: "))
	if err != nil || n < 1 || n > len(usable) {
		return "", util.Cancelled("no reviewer chosen")
	}
	id := usable[n-1].ID
	if strings.HasPrefix(strings.ToLower(ask("remember this choice? [y/N]: ")), "y") {
		man, err := store.LoadManifest(c.app.Home)
		if err == nil {
			man.Settings.ReviewHarness = id
			if err := store.SaveManifest(c.app.Home, man); err != nil {
				fmt.Fprintf(os.Stderr, "could not save your choice: %v\n", err)
			}
		}
	}
	return id, nil
}

func cmdReviewRecord(c *ctx, args []string) error {
	fs := c.flags("review record")
	verdict := fs.String("verdict", "", "pass, concerns or fail")
	summary := fs.String("summary", "", "2-4 sentences: what it does, what you checked, residual risks")
	by := fs.String("by", "", "who reviewed (default: you)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := needArgs(pos, 1, "review record NAME --verdict V --summary S"); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	rec, err := c.app.RecordManual(pos[0], *verdict, *summary, *by)
	if err != nil {
		return err
	}
	c.printf("recorded %s review for %q (advisory; nothing was approved)\n", rec.Verdict, pos[0])
	return nil
}

func cmdDiff(c *ctx, args []string) error {
	fs := c.flags("diff")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := needArgs(pos, 1, "diff NAME"); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	d, err := c.app.DiffAgainst(pos[0])
	if err != nil {
		return err
	}
	st := c.style(false)
	if d.NoBaseline {
		c.printf("%s no comparison is available: the original upstream of this in-place edit could not be rebuilt.\n  Read the files in quarantine instead (skigard show %s).\n", st.Yellow("●"), pos[0])
		return nil
	}
	c.printf("%s\n", st.Dim(d.Phrase))
	if d.Text == "" {
		c.printf("no differences\n")
	}
	c.printf("%s", term.Diff(st, d.Text))
	return nil
}

func cmdApprove(c *ctx, args []string) error {
	fs := c.flags("approve")
	override := fs.Bool("override", false, "approve despite a BLOCK verdict, incomplete coverage or a FAIL review")
	reason := fs.String("reason", "", "why the override is acceptable (required with --override)")
	accept := fs.String("accept", "", "accept all current low/medium findings, with this reason")
	expires := fs.String("accept-expires", "", "expiry date (YYYY-MM-DD) for --accept")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := needArgs(pos, 1, "approve NAME"); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	res, err := c.app.Approve(pos[0], core.ApproveOpts{Override: *override, Reason: *reason, Accept: *accept, AcceptExpires: *expires, Confirm: ttyConfirm})
	if err != nil {
		return err
	}
	st := c.style(false)
	c.printf("%s approved %q (%s)\n%s %s\n%s skigard install %s\n", st.Green("✔"), res.Name, hash.Short(res.Hash, 12), st.Dim("evidence:"), res.Record, st.Dim("next:"), res.Name)
	return nil
}

func cmdAdopt(c *ctx, args []string) error {
	fs := c.flags("adopt")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := needArgs(pos, 1, "adopt NAME"); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	q, err := c.app.Adopt(c.ctx, pos[0])
	if err != nil {
		return err
	}
	if c.json {
		return c.printJSON(map[string]any{"name": q.Name, "content_hash": q.Hash, "upstream_hash": q.Meta.UpstreamHash, "no_changes": q.NoChanges, "baseline_warning": q.BaselineWarning, "report": q.Report})
	}
	st := c.style(false)
	if q.NoChanges {
		c.printf("%s no changes: the working copy of %q is identical to the approved version\n  %s\n", st.Yellow("●"), q.Name, st.Dim("(it is still open: edit "+c.app.Home.QuarantineSkill(q.Name)+" or discard it with `skigard reject "+q.Name+"`)"))
		return nil
	}
	verb := "finalized your edit of"
	if q.Meta.InPlace {
		verb = "staged your local edits of"
	}
	c.printf("%s %s %q in quarantine\n  %s\n", st.Green("✔"), verb, q.Name, st.Dim("(skills/, the lock and installs are unchanged)"))
	c.writeScan(q.Report)
	if q.BaselineWarning != "" {
		c.printf("%s no comparison is available for this in-place edit: %s\n  `skigard diff` and the approve screen cannot show what changed; read every file in quarantine.\n", st.Yellow("●"), q.BaselineWarning)
	}
	against := "your edits vs the approved version"
	if q.Meta.InPlace {
		against = "your edits vs the original upstream"
	}
	c.nextSteps("\n", "",
		fmt.Sprintf("skigard diff %s   (%s)", q.Name, against),
		fmt.Sprintf("skigard review %s   (optional)", q.Name),
		fmt.Sprintf("skigard approve %s  (in a real terminal)", q.Name))
	return nil
}

func cmdEdit(c *ctx, args []string) error {
	fs := c.flags("edit")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := needArgs(pos, 1, "edit NAME"); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	q, err := c.app.Edit(pos[0])
	if err != nil {
		return err
	}
	dir := c.app.Home.QuarantineSkill(q.Name)
	if c.json {
		return c.printJSON(map[string]any{"name": q.Name, "path": dir, "content_hash": q.Hash})
	}
	st := c.style(false)
	c.printf("%s editable working copy of %q\n  %s\n  %s\n", st.Green("✔"), q.Name, util.Clean(dir), st.Dim("(skills/, the lock and installs are unchanged; the approved version stays live)"))
	c.nextSteps("\n", "",
		fmt.Sprintf("edit the files in %s", util.Clean(dir)),
		fmt.Sprintf("skigard diff %s     (see your changes)", q.Name),
		fmt.Sprintf("skigard adopt %s    (finalize: hash and scan the edit)", q.Name),
		fmt.Sprintf("skigard approve %s  (in a real terminal)", q.Name))
	return nil
}

func cmdMerge(c *ctx, args []string) error {
	fs := c.flags("merge")
	pick := fs.Bool("pick", false, "choose hunk by hunk in your terminal what to take from upstream (default for every hunk: keep mine)")
	with := fs.String("with", "", "have this harness (a reviewer template id: claude-code, pi, omp) merge, confined like a reviewer")
	note := fs.String("note", "", "guidance for the merge agent, for example \"use mise run playwright\" (sent to the agent; max 2000 bytes)")
	model := fs.String("model", "", "model for the merge agent, if it supports --model")
	policyOnly := fs.Bool("allow-policy-only", false, "if no OS sandbox works here, run the merge agent isolated by policy only (same meaning as for review; has no effect on approval)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := needArgs(pos, 1, "merge NAME [--pick | --with HARNESS] [--note TEXT]"); err != nil {
		return err
	}
	if *pick && *with != "" {
		return util.Errf("choose one of --pick and --with")
	}
	if *note != "" && *with == "" {
		return util.Errf("--note is for the merge agent; use it with --with HARNESS")
	}
	if err := c.open(); err != nil {
		return err
	}
	name := pos[0]
	if !*pick && *with == "" {
		if !stdinIsTerminal() || c.json || c.out != io.Writer(os.Stdout) {
			return util.Errf("choose how to merge: --pick (you choose hunk by hunk) or --with HARNESS (an agent merges, then you review and approve)")
		}
		switch strings.ToLower(ask("merge by [p]ick hunks yourself or with an [a]gent? [p/a]: ")) {
		case "p", "pick":
			*pick = true
		case "a", "agent":
			rv, err := c.app.Reviewers()
			if err != nil {
				return err
			}
			if *with, err = pickReviewer(c, rv); err != nil {
				return err
			}
		default:
			return util.Cancelled("no merge mode chosen")
		}
	}
	var m *core.Merged
	if *pick {
		ch, closeFn, err := ttyChooser(c.style(false))
		if err != nil {
			return err
		}
		defer closeFn()
		if m, err = c.app.PickMerge(name, ch); err != nil {
			return err
		}
	} else {
		iso, isoErr := review.SandboxStatus()
		if isoErr != nil {
			fmt.Fprintf(c.errw, "note: no OS sandbox is available here (%s)\n", util.OneLine(isoErr.Error(), 200))
		}
		fmt.Fprintf(c.errw, "merging %q with %s (tools disabled, isolation: %s). This sends your local copy, the new upstream version and your note to its model provider, and uses your own %s plan/credits once.\n", name, *with, iso.Mode, *with)
		if m, err = c.app.AgentMerge(c.ctx, name, core.MergeOpts{Harness: *with, Model: *model, Note: *note, AllowPolicyOnly: *policyOnly}); err != nil {
			return err
		}
	}
	if c.json {
		return c.printJSON(map[string]any{"name": m.Name, "mode": m.Mode, "merged_by": m.Meta.MergedBy, "isolation": m.Isolation.Mode,
			"content_hash": m.Hash, "upstream_hash": m.Meta.UpstreamHash, "report": m.Report})
	}
	printMerged(c, m)
	return nil
}

// printMerged is the human summary of a staged merge.
func printMerged(c *ctx, m *core.Merged) {
	how := "your picks"
	if m.Mode == core.MergeAgent {
		how = fmt.Sprintf("agent %s (isolation: %s)", util.Clean(m.Meta.MergedBy), m.Isolation.Mode)
	}
	c.printf("%s staged the merge of %q from %s in quarantine\n  %s\n", c.style(false).Green("✔"), m.Name, how, c.style(false).Dim("(skills/, the lock and installs are unchanged)"))
	c.writeScan(m.Report)
	c.nextSteps("\n", "",
		fmt.Sprintf("skigard diff %s   (your current copy vs the merge)", m.Name),
		fmt.Sprintf("skigard review %s   (optional)", m.Name),
		fmt.Sprintf("skigard approve %s  (in a real terminal)", m.Name))
}

func cmdReject(c *ctx, args []string) error {
	fs := c.flags("reject")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := needArgs(pos, 1, "reject NAME"); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	if err := c.app.Reject(pos[0]); err != nil {
		return err
	}
	c.printf("%s removed %q from quarantine\n", c.style(false).Green("✔"), pos[0])
	return nil
}

func cmdUpdate(c *ctx, args []string) error {
	fs := c.flags("update")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	rs, err := c.app.Update(c.ctx, pos)
	if err != nil {
		return err
	}
	if c.json {
		return c.printJSON(rs)
	}
	if len(rs) == 0 {
		c.printf("nothing is approved yet\n")
	}
	st := c.style(false)
	for _, r := range rs {
		c.printf("%s: %s\n", st.Bold(util.Clean(r.Name)), util.Clean(r.Note))
		if r.Changed && r.Report != nil {
			if r.NewSHA != "" {
				c.printf("  new commit: %s\n", r.NewSHA)
			}
			c.printf("\n")
			c.writeScan(r.Report)
			if r.Safety.Score > 0 {
				c.printf("\n")
				scan.WriteScore(c.out, st, r.Safety, "")
			}
			c.printf("\n")
			if r.Applied {
				c.printf("%s approved automatically (recorded in the lock as mode = auto)\n\n", st.Green("✔"))
				continue
			}
			if r.LocalChanges {
				c.printf("%s %q is in quarantine; it differs from your local copy.\n", st.Yellow("●"), r.Name)
				c.nextSteps("", "",
					fmt.Sprintf("skigard diff %s   (your copy vs new upstream)", r.Name),
					fmt.Sprintf("skigard merge %s --pick   (keep your edits; or --with HARNESS [--note TEXT])", r.Name),
					fmt.Sprintf("skigard approve %s  (in a real terminal; takes upstream wholesale and drops your edits)", r.Name))
				c.printf("\n")
				continue
			}
			c.printf("%s %q is in quarantine.\n", st.Yellow("●"), r.Name)
			c.nextSteps("", "",
				fmt.Sprintf("skigard diff %s", r.Name),
				fmt.Sprintf("skigard review %s   (optional)", r.Name),
				fmt.Sprintf("skigard approve %s  (in a real terminal)", r.Name))
			c.printf("\n")
		}
	}
	return nil
}

func installFlags(c *ctx, fs interface {
	StringVar(*string, string, string, string)
	BoolVar(*bool, string, bool, string)
}, o *core.InstallOpts, to *string) {
	fs.StringVar(to, "to", "", "comma-separated harness ids (default: detected, verified harnesses)")
	fs.StringVar(&o.Scope, "scope", "user", "user or project")
	fs.StringVar(&o.Project, "project", "", "project root for --scope project (default: current directory)")
	fs.BoolVar(&o.Copy, "copy", false, "copy instead of symlink")
	fs.BoolVar(&o.Force, "force", false, "replace an existing unmanaged skill (the old one is kept as a backup)")
	fs.BoolVar(&o.IncludeUnverified, "include-unverified", false, "also install to harnesses whose paths are not verified")
}

func printInstalled(c *ctx, got []core.Installed, notes []string) error {
	if c.json {
		return c.printJSON(map[string]any{"installed": got, "notes": notes})
	}
	st := c.style(false)
	for _, g := range got {
		c.printf("%s installed %s %s %s %s\n", st.Green("✔"), st.Bold(g.Name), st.Dim("->"), g.Path, st.Dim("("+g.Mode+"; "+strings.Join(g.Harnesses, ", ")+")"))
	}
	for _, n := range notes {
		c.printf("%s %s\n", st.Yellow("note:"), n)
	}
	return nil
}

func cmdInstall(c *ctx, args []string) error {
	fs := c.flags("install")
	var o core.InstallOpts
	var to string
	installFlags(c, fs, &o, &to)
	fs.StringVar(&o.Plugin, "plugin", "", "install every skill in this plugin")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	o.Names, o.To = pos, split(to)
	got, notes, err := c.app.Install(o)
	if err != nil {
		for _, n := range notes {
			fmt.Fprintf(c.errw, "note: %s\n", n)
		}
		return err
	}
	return printInstalled(c, got, notes)
}

func cmdSync(c *ctx, args []string) error {
	fs := c.flags("sync")
	var o core.InstallOpts
	var to string
	installFlags(c, fs, &o, &to)
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	o.To = split(to)
	got, notes, err := c.app.Sync(o)
	if err != nil {
		return err
	}
	return printInstalled(c, got, notes)
}

func cmdUninstall(c *ctx, args []string) error {
	fs := c.flags("uninstall")
	force := fs.Bool("force", false, "remove even if the install was modified")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	removed, err := c.app.Uninstall(pos, *force)
	if err != nil {
		return err
	}
	for _, r := range removed {
		c.printf("removed %s\n", r)
	}
	return nil
}

func cmdVerify(c *ctx, args []string) error {
	fs := c.flags("verify")
	strict := fs.Bool("strict", false, "treat warnings (overrides, non-pass reviews) as failures")
	requireHuman := fs.Bool("require-human", false, "fail if any approval was made automatically (mode auto) or forced")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	ps, err := c.app.Verify()
	if err != nil {
		return err
	}
	if *requireHuman {
		lock, err := store.LoadLock(c.app.Home)
		if err != nil {
			return err
		}
		for _, sk := range lock.Skills {
			// local is a human approval of the user's own edits (flagged in list/status as local); it is allowed here.
			if m := sk.Approval.EffectiveMode(); m != store.ApprovalHuman && m != store.ApprovalLocal {
				ps = append(ps, core.Problem{Level: "error", Msg: fmt.Sprintf("%s: approved with mode %s, not by a human (--require-human)", sk.Name, m)})
			}
		}
	}
	st := c.style(false)
	bad := core.HasErrors(ps)
	if *strict {
		for _, p := range ps {
			if p.Level == "warn" {
				bad = true
			}
		}
	}
	if c.json {
		if ps == nil {
			ps = []core.Problem{}
		}
		if err := c.printJSON(map[string]any{"ok": !bad, "problems": ps}); err != nil {
			return err
		}
	} else {
		for _, p := range ps {
			c.printf("%s %s\n", st.Level(fmt.Sprintf("%-5s", p.Level)), util.Clean(p.Msg))
		}
		if !bad {
			c.printf("%s lock, store, installs and marketplace agree\n", st.Green("✔ ok"))
		}
	}
	if bad {
		return util.Policyf("verification failed")
	}
	return nil
}

func cmdList(c *ctx, args []string) error {
	fs := c.flags("list")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	es, err := c.app.List()
	if err != nil {
		return err
	}
	if c.json {
		if es == nil {
			es = []core.Entry{}
		}
		return c.printJSON(es)
	}
	if len(es) == 0 {
		c.printf("no skills yet. Start with: skigard add SOURCE\n")
		return nil
	}
	st := c.style(false)
	var rows [][]string
	for _, e := range es {
		flags := ""
		if e.Override {
			flags += " [override]"
		}
		if e.Drift {
			flags += " [DRIFT]"
		}
		if e.Merge != "" {
			flags += " [merged: " + util.Clean(e.Merge) + "]"
		}
		score := "-"
		if e.Score > 0 {
			score = fmt.Sprintf("%d/10", e.Score)
		}
		rows = append(rows, []string{e.Name, e.State, dash(e.Mode), score, dash(e.Scan), dash(e.Review), hash.Short(e.Hash, 8), util.Clean(e.Source) + flags})
	}
	c.printf("%s", term.Table(st, []string{"NAME", "STATE", "MODE", "SCORE", "SCAN", "REVIEW", "HASH", "SOURCE"}, rows, func(_, col int, cell string) string {
		switch col {
		case 0:
			return st.Bold(cell)
		case 1, 2, 4, 5:
			return st.Level(cell)
		case 3:
			return scoreStyle(st, cell)
		case 6:
			return st.Dim(cell)
		}
		return strings.NewReplacer("[override]", st.Yellow("[override]"), "[DRIFT]", st.Red("[DRIFT]")).Replace(cell)
	}))
	return nil
}

// confirmed prints a one-line success message after an otherwise silent command succeeded.
func confirmed(c *ctx, err error, format string, args ...any) error {
	if err == nil && !c.json {
		c.printf("%s %s\n", c.style(false).Green("✔"), fmt.Sprintf(format, args...))
	}
	return err
}

func sevStyle(st term.Style, sev string) string {
	switch sev {
	case "critical", "high":
		return st.Red(sev)
	case "medium":
		return st.Yellow(sev)
	}
	return st.Dim(sev)
}

// scoreStyle colours "N/10" by the same bands as the safety tiers.
func scoreStyle(st term.Style, cell string) string { return st.Score(cell) }

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func cmdShow(c *ctx, args []string) error {
	fs := c.flags("show")
	plain := fs.Bool("plain", false, "print the raw text, with no colours or Markdown rendering")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := needArgs(pos, 1, "show NAME"); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	st := c.style(*plain)
	if !st.On {
		s, err := c.app.Show(pos[0])
		if err != nil {
			return err
		}
		c.printf("%s", s)
		return nil
	}
	d, err := c.app.ShowDoc(pos[0])
	if err != nil {
		return err
	}
	c.printf("%s  %s\n", st.Bold(d.Name), st.Dim(d.State))
	c.printf("%s %s\n\n", st.Dim("files"), strings.Join(d.Files, st.Dim(" · ")))
	c.printf("%s", term.Markdown(d.Body, st))
	return nil
}

func cmdStatus(c *ctx, args []string) error {
	fs := c.flags("status")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	es, err := c.app.List()
	if err != nil {
		return err
	}
	ps, err := c.app.Verify()
	if err != nil {
		return err
	}
	var appr, quar, over int
	modes := map[string]int{}
	for _, e := range es {
		if e.State == "approved" {
			appr++
			modes[e.Mode]++
			if e.Override {
				over++
			}
		} else {
			quar++
		}
	}
	errs := 0
	for _, p := range ps {
		if p.Level == "error" {
			errs++
		}
	}
	if c.json {
		return c.printJSON(map[string]any{"home": c.app.Home.Root, "approved": appr, "quarantined": quar, "overrides": over, "approved_by": modes, "errors": errs, "problems": ps})
	}
	st := c.style(false)
	c.printf("%s %s\n%s %s (%d with override)\n", st.Dim("home:       "), c.app.Home.Root, st.Dim("approved:   "), st.Green(strconv.Itoa(appr)), over)
	if appr > 0 {
		c.printf("%s %d human, %d local, %d auto, %d forced\n", st.Dim("  by mode:  "), modes[store.ApprovalHuman], modes[store.ApprovalLocal], modes[store.ApprovalAuto], modes[store.ApprovalForced])
	}
	qs := strconv.Itoa(quar)
	if quar > 0 {
		qs = st.Yellow(qs)
	}
	es2 := fmt.Sprintf("%d error(s)", errs)
	if errs > 0 {
		es2 = st.Red(es2)
	} else {
		es2 = st.Green(es2)
	}
	c.printf("%s %s\n%s %s\n", st.Dim("quarantined:"), qs, st.Dim("problems:   "), es2)
	for _, p := range ps {
		if p.Level != "info" {
			c.printf("  %s %s\n", st.Level(fmt.Sprintf("%-5s", p.Level)), util.Clean(p.Msg))
		}
	}
	return nil
}

func cmdDoctor(c *ctx, args []string) error {
	fs := c.flags("doctor")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	cs := c.app.Doctor()
	st := c.style(false)
	failed := false
	if c.json {
		if err := c.printJSON(cs); err != nil {
			return err
		}
	}
	for _, ch := range cs {
		if ch.Status == "fail" {
			failed = true
		}
		if c.json {
			continue
		}
		mark := map[string]string{"pass": "✔", "warn": "!", "fail": "✘"}[ch.Status]
		if mark == "" || !st.On {
			mark = ch.Status
		}
		c.printf("%s %s %s\n", st.Level(fmt.Sprintf("%-5s", mark)), st.Bold(fmt.Sprintf("%-10s", ch.Name)), util.Clean(ch.Message))
		if ch.Fix != "" && ch.Status != "pass" {
			c.printf("      %s %s\n", st.Dim("fix:"), ch.Fix)
		}
	}
	if failed {
		return util.Policyf("doctor found problems")
	}
	return nil
}

func cmdStats(c *ctx, args []string) error {
	fs := c.flags("stats")
	tokens := fs.Bool("tokens", true, "estimate token cost of approved skills")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	_ = tokens
	if err := c.open(); err != nil {
		return err
	}
	rows, err := c.app.Tokens()
	if err != nil {
		return err
	}
	if c.json {
		if rows == nil {
			rows = []core.TokenRow{}
		}
		return c.printJSON(rows)
	}
	var res, body int
	st := c.style(false)
	var trs [][]string
	for _, r := range rows {
		trs = append(trs, []string{r.Name, fmt.Sprintf("~%d", r.Resident), fmt.Sprintf("~%d", r.Body)})
		res += r.Resident
		body += r.Body
	}
	trs = append(trs, []string{"TOTAL", fmt.Sprintf("~%d", res), fmt.Sprintf("~%d", body)})
	last := len(trs) - 1
	c.printf("%s", term.Table(st, []string{"SKILL", "RESIDENT (~tokens in every prompt)", "BODY (~tokens when it fires)"}, trs, func(row, _ int, cell string) string {
		if row == last {
			return st.Bold(cell)
		}
		return cell
	}))
	c.printf("%s\n", st.Dim("estimates assume ~4 characters per token"))
	return nil
}

func cmdPlugin(c *ctx, args []string) error {
	sub, args, err := c.verb("plugin", args)
	if err != nil {
		return err
	}
	fs := c.flags("plugin " + sub)
	desc := fs.String("description", "", "plugin description (create)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	a := c.app
	switch sub {
	case "create":
		if err := needArgs(pos, 1, "plugin create NAME [--description D]"); err != nil {
			return err
		}
		return confirmed(c, a.PluginCreate(pos[0], *desc), "created plugin %q", pos[0])
	case "delete":
		if err := needArgs(pos, 1, "plugin delete NAME"); err != nil {
			return err
		}
		return confirmed(c, a.PluginDelete(pos[0]), "deleted plugin %q", pos[0])
	case "add":
		if err := needArgs(pos, 2, "plugin add NAME SKILL..."); err != nil {
			return err
		}
		return confirmed(c, a.PluginAdd(pos[0], pos[1:]), "added %s to plugin %q", strings.Join(pos[1:], ", "), pos[0])
	case "remove":
		if err := needArgs(pos, 2, "plugin remove NAME SKILL..."); err != nil {
			return err
		}
		return confirmed(c, a.PluginRemove(pos[0], pos[1:]), "removed %s from plugin %q", strings.Join(pos[1:], ", "), pos[0])
	case "list", "show":
		man, err := store.LoadManifest(a.Home)
		if err != nil {
			return err
		}
		var names []string
		for n := range man.Plugins {
			names = append(names, n)
		}
		sort.Strings(names)
		if sub == "show" {
			if err := needArgs(pos, 1, "plugin show NAME"); err != nil {
				return err
			}
			p, ok := man.Plugins[pos[0]]
			if !ok {
				return util.Errf("no plugin %q", pos[0])
			}
			if c.json {
				return c.printJSON(p)
			}
			st := c.style(false)
			c.printf("%s  %s\n", st.Bold(pos[0]), st.Dim(p.Description))
			for _, sk := range p.Skills {
				c.printf("  %s %s\n", st.Cyan("•"), sk)
			}
			if len(p.Skills) == 0 {
				c.printf("  %s\n", st.Dim("(no skills yet: skigard plugin add "+pos[0]+" SKILL...)"))
			}
			return nil
		}
		if c.json {
			return c.printJSON(man.Plugins)
		}
		st := c.style(false)
		var rows [][]string
		for _, n := range names {
			rows = append(rows, []string{n, strings.Join(man.Plugins[n].Skills, ", ")})
		}
		if len(rows) == 0 {
			c.printf("no plugins yet. Create one: skigard plugin create NAME\n")
			return nil
		}
		c.printf("%s", term.Table(st, []string{"PLUGIN", "SKILLS"}, rows, func(_, col int, cell string) string {
			if col == 0 {
				return st.Bold(cell)
			}
			return cell
		}))
		return nil
	}
	return util.Errf("unknown plugin command %q", sub) // unreachable: verb() validated it
}

func cmdMarketplace(c *ctx, args []string) error {
	sub, args, err := c.verb("marketplace", args)
	if err != nil {
		return err
	}
	fs := c.flags("marketplace " + sub)
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	switch sub {
	case "build":
		if err := c.app.MarketplaceBuild(); err != nil {
			return err
		}
		c.printf("marketplace written to %s\nClaude Code: /plugin marketplace add %s\n", c.app.Home.MarketplaceDir(), c.app.Home.MarketplaceDir())
		return nil
	case "check":
		drift, err := c.app.MarketplaceCheck()
		if err != nil {
			return err
		}
		for _, d := range drift {
			c.printf("%s\n", d)
		}
		if len(drift) > 0 {
			return util.Policyf("marketplace is out of date")
		}
		c.printf("marketplace is up to date\n")
		return nil
	}
	return util.Errf("unknown marketplace command %q", sub)
}

func cmdHarness(c *ctx, args []string) error {
	sub, args, err := c.verb("harness", args)
	if err != nil {
		return err
	}
	fs := c.flags("harness " + sub)
	id := fs.String("id", "", "harness id (add)")
	name := fs.String("name", "", "display name (add)")
	global := fs.String("global", "", "user-scope skills dir, e.g. ~/.mytool/skills (add)")
	project := fs.String("project", "", "project-scope skills dir, e.g. .mytool/skills (add)")
	bin := fs.String("bin", "", "executable name used for detection (add)")
	_, err = parse(fs, args)
	if err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	switch sub {
	case "list":
		return cmdDetect2(c)
	case "add":
		e := harness.Entry{ID: *id, Name: *name, Global: *global, Project: *project, Verified: true}
		if *bin != "" {
			e.Bins = []string{*bin}
		}
		if e.Name == "" {
			e.Name = e.ID
		}
		if err := e.Validate(); err != nil {
			return err
		}
		if err := c.app.Home.Init(); err != nil {
			return err
		}
		man, err := store.LoadManifest(c.app.Home)
		if err != nil {
			return err
		}
		out := man.Harness[:0]
		for _, h := range man.Harness {
			if h.ID != e.ID {
				out = append(out, h)
			}
		}
		man.Harness = append(append([]harness.Entry{}, out...), e)
		if err := store.SaveManifest(c.app.Home, man); err != nil {
			return err
		}
		c.printf("harness %q saved to skigard.toml\n", e.ID)
		return nil
	}
	return util.Errf("unknown harness command %q", sub)
}

func cmdDetect2(c *ctx) error {
	ds, err := c.app.Detect()
	if err != nil {
		return err
	}
	if c.json {
		return c.printJSON(ds)
	}
	st := c.style(false)
	var rows [][]string
	for _, d := range ds {
		rows = append(rows, []string{d.ID, d.Name, d.Status, fmt.Sprint(d.Verified), d.GlobalAbs, d.Project})
	}
	c.printf("%s", term.Table(st, []string{"ID", "NAME", "STATUS", "VERIFIED", "GLOBAL DIR", "PROJECT DIR"}, rows, func(_, col int, cell string) string {
		switch col {
		case 0:
			return st.Bold(cell)
		case 2, 3:
			return st.Level(cell)
		}
		return cell
	}))
	return nil
}

func cmdRules(c *ctx, args []string) error {
	fs := c.flags("rules")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	cat := scan.Catalog()
	sort.Slice(cat, func(i, j int) bool { return cat[i][0] < cat[j][0] })
	if c.json {
		return c.printJSON(cat)
	}
	st := c.style(false)
	var rows [][]string
	for _, r := range cat {
		rows = append(rows, []string{r[0], r[1], r[2], r[3], r[4]})
	}
	c.printf("%s", term.Table(st, []string{"ID", "NAME", "FAMILY", "SEVERITY", "DESCRIPTION"}, rows, func(_, col int, cell string) string {
		switch col {
		case 0:
			return st.Bold(cell)
		case 3:
			return sevStyle(st, cell)
		}
		return cell
	}))
	c.printf("%s\n", st.Dim("rules version "+scan.RulesVersion))
	return nil
}

func cmdMigrate(c *ctx, args []string) error {
	fs := c.flags("migrate")
	from := fs.String("from-skm", "", "path to an skm repository")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if *from == "" {
		return util.Errf("usage: skigard migrate --from-skm PATH")
	}
	if err := c.open(); err != nil {
		return err
	}
	res, err := c.app.MigrateSkm(*from)
	if err != nil {
		return err
	}
	for _, n := range res.Migrated {
		c.printf("migrated %s\n", n)
	}
	var skipped []string
	for n := range res.Skipped {
		skipped = append(skipped, n)
	}
	sort.Strings(skipped)
	for _, n := range skipped {
		c.printf("skipped  %s: %s\n", n, res.Skipped[n])
	}
	for _, p := range res.Plugins {
		c.printf("plugin   %s\n", p)
	}
	c.printf("next: skigard verify && skigard sync\n")
	return nil
}

func cmdLock(c *ctx, args []string) error {
	_, args, err := c.verb("lock", args)
	if err != nil {
		return err
	}
	fs := c.flags("lock upgrade")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	res, err := c.app.LockUpgrade(pos)
	if err != nil {
		return err
	}
	if c.json {
		if res == nil {
			res = []core.UpgradeResult{}
		}
		return c.printJSON(res)
	}
	if len(res) == 0 {
		c.printf("no approved skills\n")
	}
	for _, r := range res {
		c.printf("%s: %s\n", util.Clean(r.Name), r.Note)
		if r.Upgraded {
			c.printf("  content_hash %s -> %s\n  exec_set: %d executable file(s)\n", hash.Short(r.OldHash, 12), hash.Short(r.NewHash, 12), len(r.Exec))
		}
	}
	return nil
}
