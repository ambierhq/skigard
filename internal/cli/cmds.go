package cli

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/ambierhq/skigard/internal/core"
	"github.com/ambierhq/skigard/internal/harness"
	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

func cmdVersion(c *ctx, args []string) error {
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
	w := tabwriter.NewWriter(c.out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tID\tNAME\tVERIFIED\tSKILLS DIR")
	var unv int
	for _, d := range shown {
		v := "yes"
		if !d.Verified {
			v, unv = "no", unv+1
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", d.Status, d.ID, d.Name, v, d.GlobalAbs)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if unv > 0 {
		c.printf("\n%d harness(es) are unverified: listed, but installs need --include-unverified.\n", unv)
	}
	return nil
}

func cmdAdd(c *ctx, args []string) error {
	fs := c.flags("add")
	skillName := fs.String("skill", "", "take one skill by name or path")
	all := fs.Bool("all", false, "take every skill found")
	name := fs.String("name", "", "store the skill under this name")
	force := fs.Bool("force", false, "allow replacing a name that is already approved")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if err := needArgs(pos, 1, "add SOURCE [--skill N | --all]"); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	qs, err := c.app.Add(c.ctx, pos[0], core.AddOpts{Skill: *skillName, All: *all, Name: *name, Force: *force})
	if err != nil {
		return err
	}
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
		scan.WriteText(c.out, q.Report)
		c.printf("\nnext: skigard review %s   (optional)\n      skigard approve %s  (in a real terminal)\n\n", q.Name, q.Name)
	}
	return nil
}

func cmdScan(c *ctx, args []string) error {
	fs := c.flags("scan")
	format := fs.String("format", "text", "text, json or sarif")
	failOn := fs.String("fail-on", "high", "exit 1 at this severity or above: low, medium, high, critical, none")
	incomplete := fs.Bool("fail-on-incomplete", false, "exit 4 when scan coverage is incomplete")
	out := fs.String("output", "", "write the report to a file")
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
		scan.WriteText(w, rep)
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
	if len(args) > 0 && args[0] == "record" {
		return cmdReviewRecord(c, args[1:])
	}
	fs := c.flags("review")
	harnessID := fs.String("harness", "", "harness to run as the reviewer (default: your saved choice, or you pick)")
	model := fs.String("model", "", "model for the reviewer, if it supports --model")
	manual := fs.Bool("manual", false, "print the review prompt instead of running a harness")
	unverified := fs.Bool("allow-unverified", false, "allow reviewer templates that have not been verified")
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
	id := *harnessID
	if id == "" {
		man, err := store.LoadManifest(c.app.Home)
		if err != nil {
			return err
		}
		id = man.Settings.ReviewHarness
	}
	if id == "" {
		if id, err = pickReviewer(c, rv); err != nil {
			return err
		}
	}
	c.printf("reviewing %q with %s (read-only, no tools, advisory). This uses your own %s plan/credits once.\n", name, id, id)
	rec, err := c.app.Review(c.ctx, name, core.ReviewOpts{Harness: id, Model: *model, AllowUnverified: *unverified})
	if err != nil {
		return err
	}
	if c.json {
		return c.printJSON(rec)
	}
	c.printf("review verdict: %s (coverage %s, advisory)\n  %s\n", strings.ToUpper(rec.Verdict), rec.Coverage, util.Clean(rec.Summary))
	for _, f := range rec.Findings {
		c.printf("  [%s] %s:%d %s\n", f.Severity, util.Clean(f.File), f.Line, util.Clean(f.Why))
	}
	if rec.Dropped > 0 {
		c.printf("  (%d finding(s) dropped: their quotes were not found in the files)\n", rec.Dropped)
	}
	c.printf("next: skigard approve %s\n", name)
	return nil
}

func pickReviewer(c *ctx, rv []core.Reviewer) (string, error) {
	var usable []core.Reviewer
	for _, r := range rv {
		if r.Installed {
			usable = append(usable, r)
		}
	}
	if len(usable) == 0 {
		return "", util.Errf("no harness that can review headlessly was found (claude-code, pi, omp, gemini-cli). Use --manual, or install one")
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
	d, err := c.app.Diff(pos[0])
	if err != nil {
		return err
	}
	if d == "" {
		d = "no differences from the approved version\n"
	}
	c.printf("%s", d)
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
	c.printf("approved %q (%s)\nevidence: %s\nnext: skigard install %s\n", res.Name, hash.Short(res.Hash, 12), res.Record, res.Name)
	return nil
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
	c.printf("removed %q from quarantine\n", pos[0])
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
	for _, r := range rs {
		c.printf("%s: %s\n", r.Name, util.Clean(r.Note))
		if r.Changed && r.Report != nil {
			c.printf("  new commit: %s\n", r.NewSHA)
			scan.WriteText(c.out, r.Report)
			c.printf("  next: skigard diff %s; skigard review %s; skigard approve %s\n", r.Name, r.Name, r.Name)
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
	for _, g := range got {
		c.printf("installed %s -> %s (%s; %s)\n", g.Name, g.Path, g.Mode, strings.Join(g.Harnesses, ", "))
	}
	for _, n := range notes {
		c.printf("note: %s\n", n)
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
			c.printf("%-5s %s\n", p.Level, util.Clean(p.Msg))
		}
		if !bad {
			c.printf("ok: lock, store, installs and marketplace agree\n")
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
	w := tabwriter.NewWriter(c.out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATE\tSCAN\tREVIEW\tHASH\tSOURCE")
	for _, e := range es {
		flags := ""
		if e.Override {
			flags += " [override]"
		}
		if e.Drift {
			flags += " [DRIFT]"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s%s\n", e.Name, e.State, dash(e.Scan), dash(e.Review), hash.Short(e.Hash, 8), util.Clean(e.Source), flags)
	}
	return w.Flush()
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func cmdShow(c *ctx, args []string) error {
	fs := c.flags("show")
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
	s, err := c.app.Show(pos[0])
	if err != nil {
		return err
	}
	c.printf("%s", s)
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
	for _, e := range es {
		if e.State == "approved" {
			appr++
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
		return c.printJSON(map[string]any{"home": c.app.Home.Root, "approved": appr, "quarantined": quar, "overrides": over, "errors": errs, "problems": ps})
	}
	c.printf("home:        %s\napproved:    %d (%d with override)\nquarantined: %d\nproblems:    %d error(s)\n", c.app.Home.Root, appr, over, quar, errs)
	for _, p := range ps {
		if p.Level != "info" {
			c.printf("  %-5s %s\n", p.Level, util.Clean(p.Msg))
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
		c.printf("%-5s %-10s %s\n", ch.Status, ch.Name, util.Clean(ch.Message))
		if ch.Fix != "" && ch.Status != "pass" {
			c.printf("      fix: %s\n", ch.Fix)
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
	w := tabwriter.NewWriter(c.out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "SKILL\tRESIDENT (~tokens in every prompt)\tBODY (~tokens when it fires)")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t~%d\t~%d\n", r.Name, r.Resident, r.Body)
		res += r.Resident
		body += r.Body
	}
	fmt.Fprintf(w, "TOTAL\t~%d\t~%d\n", res, body)
	if err := w.Flush(); err != nil {
		return err
	}
	c.printf("estimates assume ~4 characters per token\n")
	return nil
}

func cmdPlugin(c *ctx, args []string) error {
	if len(args) == 0 {
		return util.Errf("usage: skigard plugin create|delete|add|remove|show|list ...")
	}
	sub := args[0]
	fs := c.flags("plugin " + sub)
	desc := fs.String("description", "", "plugin description (create)")
	pos, err := parse(fs, args[1:])
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
		return a.PluginCreate(pos[0], *desc)
	case "delete":
		if err := needArgs(pos, 1, "plugin delete NAME"); err != nil {
			return err
		}
		return a.PluginDelete(pos[0])
	case "add":
		if err := needArgs(pos, 2, "plugin add NAME SKILL..."); err != nil {
			return err
		}
		return a.PluginAdd(pos[0], pos[1:])
	case "remove":
		if err := needArgs(pos, 2, "plugin remove NAME SKILL..."); err != nil {
			return err
		}
		return a.PluginRemove(pos[0], pos[1:])
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
			c.printf("%s: %s\n  skills: %s\n", pos[0], p.Description, strings.Join(p.Skills, ", "))
			return nil
		}
		if c.json {
			return c.printJSON(man.Plugins)
		}
		for _, n := range names {
			c.printf("%s\t%s\n", n, strings.Join(man.Plugins[n].Skills, ", "))
		}
		return nil
	}
	return util.Errf("unknown plugin command %q", sub)
}

func cmdMarketplace(c *ctx, args []string) error {
	if len(args) == 0 {
		return util.Errf("usage: skigard marketplace build|check")
	}
	fs := c.flags("marketplace " + args[0])
	if _, err := parse(fs, args[1:]); err != nil {
		return err
	}
	if err := c.open(); err != nil {
		return err
	}
	switch args[0] {
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
	return util.Errf("unknown marketplace command %q", args[0])
}

func cmdHarness(c *ctx, args []string) error {
	if len(args) == 0 {
		return util.Errf("usage: skigard harness list|add")
	}
	sub := args[0]
	fs := c.flags("harness " + sub)
	id := fs.String("id", "", "harness id (add)")
	name := fs.String("name", "", "display name (add)")
	global := fs.String("global", "", "user-scope skills dir, e.g. ~/.mytool/skills (add)")
	project := fs.String("project", "", "project-scope skills dir, e.g. .mytool/skills (add)")
	bin := fs.String("bin", "", "executable name used for detection (add)")
	pos, err := parse(fs, args[1:])
	_ = pos
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
	w := tabwriter.NewWriter(c.out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tVERIFIED\tGLOBAL DIR\tPROJECT DIR")
	for _, d := range ds {
		fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%s\t%s\n", d.ID, d.Name, d.Status, d.Verified, d.GlobalAbs, d.Project)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(c.out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tFAMILY\tSEVERITY\tDESCRIPTION")
	for _, r := range cat {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r[0], r[1], r[2], r[3], r[4])
	}
	if err := w.Flush(); err != nil {
		return err
	}
	c.printf("rules version %s\n", scan.RulesVersion)
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
