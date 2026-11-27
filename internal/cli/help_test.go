package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/util"
)

// helpPaths lists every command and every subcommand as an argv prefix.
func helpPaths() [][]string {
	var paths [][]string
	for _, cm := range commands {
		paths = append(paths, []string{cm.name})
		for _, s := range groups[cm.name] {
			paths = append(paths, []string{cm.name, s.name})
		}
	}
	return paths
}

func TestHelpEverywhere(t *testing.T) {
	for _, p := range helpPaths() {
		for _, flagName := range []string{"-h", "--help"} {
			name := strings.Join(p, " ") + " " + flagName
			t.Run(name, func(t *testing.T) {
				// The home does not exist: -h must not create, lock or read it.
				home := filepath.Join(t.TempDir(), "never-created")
				args := append(append([]string{}, p...), flagName, "--home", home)
				code, out, errs := run(t, home, args...)
				if code != util.ExitOK {
					t.Fatalf("exit %d, stderr %q", code, errs)
				}
				if errs != "" {
					t.Fatalf("stderr should be empty: %q", errs)
				}
				if !strings.Contains(out, "Usage: skigard "+strings.Join(p, " ")) {
					t.Fatalf("usage missing the command name:\n%s", out)
				}
				if _, err := os.Stat(home); !os.IsNotExist(err) {
					t.Fatalf("home was touched: %v", err)
				}
			})
		}
	}
}

func TestGroupHelpListsSubcommands(t *testing.T) {
	home := filepath.Join(t.TempDir(), "h")
	for name, subs := range groups {
		_, out, _ := run(t, home, name, "-h")
		for _, s := range subs {
			if !strings.Contains(out, s.name) || !strings.Contains(out, s.summary) {
				t.Errorf("%s -h misses %q:\n%s", name, s.name, out)
			}
		}
	}
}

func TestHelpCommand(t *testing.T) {
	home := filepath.Join(t.TempDir(), "h")
	if code, out, _ := run(t, home, "help", "plugin", "create"); code != 0 || !strings.Contains(out, "Usage: skigard plugin create") {
		t.Fatalf("help plugin create: %d %q", code, out)
	}
	if code, _, errs := run(t, home, "help", "bogus"); code != util.ExitUsage || !strings.Contains(errs, "unknown command") {
		t.Fatalf("help bogus: %d %q", code, errs)
	}
}

func TestGroupUsageErrors(t *testing.T) {
	home := filepath.Join(t.TempDir(), "h")
	for name := range groups {
		if name == "review" { // review is also a command with its own flags
			continue
		}
		code, _, errs := run(t, home, name)
		if code != util.ExitUsage || !strings.Contains(errs, "skigard "+name+" -h") {
			t.Errorf("bare %s: %d %q", name, code, errs)
		}
		code, _, errs = run(t, home, name, "bogus")
		if code != util.ExitUsage || !strings.Contains(errs, "unknown "+name+" command") {
			t.Errorf("%s bogus: %d %q", name, code, errs)
		}
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("home was touched: %v", err)
	}
}
