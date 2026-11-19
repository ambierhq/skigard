package cli

import (
	_ "embed"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ambierhq/skigard/internal/util"
)

const ciWorkflowPath = ".github/workflows/skigard.yml"

var (
	semverRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)
	sha40Re  = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

//go:embed ci-workflow.yml
var ciWorkflowTmpl string

// ciWorkflow renders the workflow. Unpinned actions get an obvious placeholder so the file cannot be used by accident.
func ciWorkflow(version, actionSHA string) string {
	if actionSHA == "" {
		actionSHA = "REPLACE_WITH_FULL_COMMIT_SHA"
	}
	return strings.NewReplacer("@VERSION@", version, "@ACTION_SHA@", actionSHA).Replace(ciWorkflowTmpl)
}

func cmdInitCI(c *ctx, args []string) error {
	fs := c.flags("init-ci")
	version := fs.String("version", "", "skigard version to pin in the workflow (default: this binary's version)")
	sha := fs.String("action-sha", "", "full commit SHA of ambierhq/skigard-action to pin (default: a placeholder you must fill in)")
	force := fs.Bool("force", false, "overwrite an existing "+ciWorkflowPath)
	if _, err := parse(fs, args); err != nil {
		return err
	}
	v := strings.TrimPrefix(*version, "v")
	if v == "" {
		v = strings.TrimPrefix(Version, "v")
	}
	if !semverRe.MatchString(v) {
		return util.Errf("cannot pin version %q: pass --version X.Y.Z", v)
	}
	if *sha != "" && !sha40Re.MatchString(*sha) {
		return util.Errf("--action-sha must be a full 40-character lowercase commit SHA")
	}
	if _, err := os.Lstat(ciWorkflowPath); err == nil && !*force {
		return util.Errf("%s already exists (use --force to overwrite)", ciWorkflowPath)
	}
	if err := util.AtomicWrite(filepath.FromSlash(ciWorkflowPath), []byte(ciWorkflow(v, *sha)), 0o644); err != nil {
		return util.Errf("write %s: %v", ciWorkflowPath, err)
	}
	c.printf("wrote %s (skigard %s)\n", ciWorkflowPath, v)
	if *sha == "" {
		c.printf("next: replace REPLACE_WITH_FULL_COMMIT_SHA with full commit SHAs of actions/checkout and ambierhq/skigard-action\n")
	}
	return nil
}
