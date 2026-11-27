// Package cli is skigard's command-line front end. It is a thin layer over
// internal/core: parse flags, call a workflow, print the result.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/ambierhq/skigard/internal/core"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/term"
	"github.com/ambierhq/skigard/internal/util"
)

// testAppHook is set only from tests (unexported, so no flag or env can reach
// it). It must never touch the confirmation path.
var testAppHook func(*core.App)

// Version is set at build time with -ldflags "-X .../internal/cli.Version=...".
var Version = "dev"

type command struct {
	name, args, summary string
	run                 func(c *ctx, args []string) error
}

type ctx struct {
	app  *core.App
	out  io.Writer
	errw io.Writer
	home string
	json bool
	ctx  context.Context
}

var commands []command

func init() {
	commands = []command{
		{"init", "[--git]", "create the skigard home directory", cmdInit},
		{"detect", "[--all]", "list agent harnesses found on this machine", cmdDetect},
		{"add", "SOURCE [--skill N | --all] [--mode strict|auto] [--threshold N] [--review] [--force-install] [--quarantine-only]", "fetch, scan, score and (per mode) install a skill; --quarantine-only stops after the scan", cmdAdd},
		{"scan", "NAME|PATH [--format text|json|sarif]", "scan a skill", cmdScan},
		{"review", "NAME [--harness ID | --manual]", "have an agent harness review a quarantined skill (advisory)", cmdReview},
		{"diff", "NAME", "show quarantined changes against the approved version", cmdDiff},
		{"approve", "NAME", "HUMAN ONLY: approve a quarantined skill", cmdApprove},
		{"reject", "NAME", "delete a quarantined skill", cmdReject},
		{"update", "[NAME...]", "re-fetch approved skills; changes go to quarantine", cmdUpdate},
		{"install", "NAME... | --plugin P", "install approved skills into harnesses", cmdInstall},
		{"uninstall", "[NAME...]", "remove skills installed by skigard", cmdUninstall},
		{"sync", "", "install every approved skill into the default harnesses", cmdSync},
		{"config", "get [KEY] | set KEY VALUE", "read or change mode, threshold and auto_review (set of mode/threshold is human only)", cmdConfig},
		{"verify", "[--strict] [--require-human]", "check lock, store, installs and marketplace (CI-friendly)", cmdVerify},
		{"lock", "upgrade [NAME...]", "upgrade lock entries to hash version 2 (only if the old hash still matches)", cmdLock},
		{"init-ci", "[--version X.Y.Z] [--action-sha SHA]", "write a GitHub workflow that runs skigard verify", cmdInitCI},
		{"list", "", "list approved and quarantined skills", cmdList},
		{"show", "NAME", "print a skill's SKILL.md and file list", cmdShow},
		{"status", "", "summary of this home", cmdStatus},
		{"doctor", "", "check the environment", cmdDoctor},
		{"stats", "--tokens", "estimate the context cost of approved skills", cmdStats},
		{"allow", "add SOURCE|SKILL RULE... --reason R | list | remove SOURCE [RULE...]", "HUMAN ONLY: stop reporting medium/low scanner rules for one source", cmdAllow},
		{"plugin", "create|delete|add|remove|show|list", "manage plugins (groups of approved skills)", cmdPlugin},
		{"marketplace", "build|check", "generate or check the Claude/OMP marketplace", cmdMarketplace},
		{"harness", "list|add", "inspect or extend the harness registry", cmdHarness},
		{"rules", "", "list scanner rules", cmdRules},
		{"migrate", "--from-skm PATH", "import approved skills from an skm repository", cmdMigrate},
		{"completion", "bash|zsh|fish|powershell", "print a shell completion script", cmdCompletion},
		{"version", "", "print the version", cmdVersion},
	}
}

// Run executes skigard and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage(stdout)
		return util.ExitOK
	}
	if args[0] == "-v" || args[0] == "--version" {
		fmt.Fprintln(stdout, "skigard", Version)
		return util.ExitOK
	}
	var cmd *command
	for i := range commands {
		if commands[i].name == args[0] {
			cmd = &commands[i]
		}
	}
	if cmd == nil {
		fmt.Fprintf(stderr, "skigard: unknown command %q\n\n", args[0])
		usage(stderr)
		return util.ExitUsage
	}
	c := &ctx{out: stdout, errw: stderr}
	cx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	c.ctx = cx
	if err := cmd.run(c, args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return util.ExitOK
		}
		fmt.Fprintf(stderr, "skigard: %s\n", util.Clean(err.Error()))
		return util.CodeOf(err)
	}
	return util.ExitOK
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "skigard "+Version+": a safer way to manage agent skills")
	fmt.Fprintln(w, "\nUsage: skigard COMMAND [ARGS]\n\nCommands:")
	for _, c := range commands {
		fmt.Fprintf(w, "  %-12s %s\n", c.name, c.summary)
	}
	fmt.Fprintln(w, "\nTypical flow:  add (fetch, scan, score, install if it clears your threshold)")
	fmt.Fprintln(w, "Step by step:  add --quarantine-only -> review -> approve (in a real terminal) -> install")
	fmt.Fprintln(w, "Run `skigard COMMAND -h` for a command's flags. Home: $SKIGARD_HOME or ~/.config/skigard.")
}

// flags creates a FlagSet with the flags every command shares.
func (c *ctx) flags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet("skigard "+name, flag.ContinueOnError)
	fs.SetOutput(c.errw)
	fs.StringVar(&c.home, "home", "", "skigard home directory (default $SKIGARD_HOME, ~/.config/skigard, or %APPDATA%\\skigard on Windows)")
	fs.BoolVar(&c.json, "json", false, "machine-readable output")
	return fs
}

// parse allows flags and positional arguments in any order.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

// open builds the App once flags are parsed.
func (c *ctx) open() error {
	h, err := store.Resolve(c.home)
	if err != nil {
		return err
	}
	c.app = core.New(h, Version, c.out, c.errw)
	if !c.json && c.errw == io.Writer(os.Stderr) && term.StderrIsTerminal(os.Getenv) {
		c.app.ReviewProgress = c.reviewProgress
	}
	if testAppHook != nil {
		testAppHook(c.app)
	}
	return nil
}

// reviewProgress shows a spinner with the elapsed time while a reviewer runs,
// so a slow model call is visibly alive. It writes to stderr only.
func (c *ctx) reviewProgress(skill, harness string) func() {
	label := fmt.Sprintf("reviewing %s with %s (this can take a minute or two)", util.Clean(skill), util.Clean(harness))
	sp := term.StartSpinner(c.errw, label)
	return func() {
		took := sp.Stop()
		fmt.Fprintf(c.errw, "review finished in %s\n", term.Elapsed(took))
	}
}

func (c *ctx) printJSON(v any) error {
	enc := json.NewEncoder(c.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// style is the output style for human-readable text: colours only on a real
// terminal, never with --json, --plain or NO_COLOR, so pipes and files stay plain.
func (c *ctx) style(plain bool) term.Style {
	if plain || c.json || c.out != io.Writer(os.Stdout) {
		return term.Style{}
	}
	return term.Style{On: term.Enabled(os.Stdout, os.Getenv)}
}

// writeScan prints a scan report, styled on a terminal and as plain text otherwise.
func (c *ctx) writeScan(r *scan.Report) {
	if st := c.style(false); st.On {
		scan.WriteStyled(c.out, r, st)
		return
	}
	scan.WriteText(c.out, r)
}

func (c *ctx) printf(format string, args ...any) { fmt.Fprintf(c.out, format, args...) }

func split(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func needArgs(pos []string, n int, usage string) error {
	if len(pos) < n {
		return util.Errf("usage: skigard %s", usage)
	}
	return nil
}
