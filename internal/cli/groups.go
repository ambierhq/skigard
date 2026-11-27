package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/ambierhq/skigard/internal/core"
	"github.com/ambierhq/skigard/internal/util"
)

// subcommand is one verb of a command group, for example `plugin create`.
type subcommand struct{ name, args, summary string }

// groups is the single source of truth for commands that take a verb as their
// first argument. Usage, -h, completion and docs/commands.md all read it.
var groups = map[string][]subcommand{
	"config": {
		{"get", "[KEY]", "print one setting, or all of them (keys: " + strings.Join(core.ConfigKeys, ", ") + ")"},
		{"set", "KEY VALUE", "change a setting (mode and threshold: human terminal only)"},
	},
	"lock": {
		{"upgrade", "[NAME...]", "upgrade lock entries to hash version 2 (only if the old hash still matches)"},
		{"protect", "[NAME...]", "make approved skills read-only (changes permissions only, never content, hash or lock)"},
	},
	"allow": {
		{"add", "SOURCE|SKILL RULE... --reason R", "HUMAN ONLY: stop reporting medium/low rules for one source"},
		{"list", "", "list allowed rules"},
		{"remove", "SOURCE [RULE...]", "HUMAN ONLY: remove allowed rules"},
	},
	"plugin": {
		{"create", "NAME [--description D]", "create an empty plugin"},
		{"delete", "NAME", "delete a plugin"},
		{"add", "NAME SKILL...", "add approved skills to a plugin"},
		{"remove", "NAME SKILL...", "remove skills from a plugin"},
		{"show", "NAME", "show a plugin's skills"},
		{"list", "", "list plugins"},
	},
	"marketplace": {
		{"build", "", "write the marketplace/ directory"},
		{"check", "", "report drift between marketplace/ and the approved skills"},
	},
	"harness": {
		{"list", "", "list known harnesses and whether they were detected"},
		{"add", "--id ID [--name N] [--global DIR] [--project DIR] [--bin B]", "add a harness to skigard.toml"},
	},
	"review": {
		{"record", "NAME --verdict V --summary S", "record a manual review (advisory)"},
	},
}

func isHelp(a string) bool { return a == "-h" || a == "--help" || a == "-help" }

// verb picks the sub-command of group name from args, or returns an error.
// -h/--help prints the group's usage to stdout and returns flag.ErrHelp; no
// home is read and nothing runs. optional means a missing verb is allowed.
func (c *ctx) verb(name string, args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, util.Errf("usage: skigard %s %s (see `skigard %s -h`)", name, verbList(name), name)
	}
	if isHelp(args[0]) {
		c.groupUsage(c.out, name)
		return "", nil, flag.ErrHelp
	}
	for _, s := range groups[name] {
		if s.name == args[0] {
			return s.name, args[1:], nil
		}
	}
	return "", nil, util.Errf("unknown %s command %q (see `skigard %s -h`)", name, args[0], name)
}

func verbList(name string) string {
	var v []string
	for _, s := range groups[name] {
		v = append(v, s.name)
	}
	return strings.Join(v, "|")
}

func (c *ctx) groupUsage(w io.Writer, name string) {
	cm := lookup(name)
	fmt.Fprintf(w, "Usage: skigard %s SUBCOMMAND [ARGS]\n\n%s.\n\nSubcommands:\n", name, capitalise(cm.summary))
	for _, s := range groups[name] {
		fmt.Fprintf(w, "  %-9s %s\n", s.name, s.summary)
	}
	fmt.Fprintf(w, "\nRun `skigard %s SUBCOMMAND -h` for its flags and arguments.\n", name)
}

func lookup(name string) command {
	for _, cm := range commands {
		if cm.name == name {
			return cm
		}
	}
	return command{}
}

// usageFor returns the usage line and summary for a flag set named like
// "skigard plugin create".
func usageFor(fsName string) (line, summary string) {
	parts := strings.Fields(strings.TrimPrefix(fsName, "skigard "))
	if len(parts) == 0 {
		return "skigard", ""
	}
	if len(parts) > 1 {
		for _, s := range groups[parts[0]] {
			if s.name == parts[1] {
				return strings.TrimSpace("skigard " + parts[0] + " " + s.name + " " + s.args), s.summary
			}
		}
	}
	cm := lookup(parts[0])
	return strings.TrimSpace("skigard " + parts[0] + " " + cm.args), cm.summary
}
