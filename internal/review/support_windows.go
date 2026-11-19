//go:build windows

package review

import "github.com/ambierhq/skigard/internal/util"

// Supported reports whether automated review can run on this platform.
//
// On Unix the reviewer is isolated by policy rather than by an OS sandbox: the
// harness CLI is started with its tool, MCP, extension and session flags
// disabled, in an empty temporary directory, with a scrubbed environment, the
// skill on stdin only and bounded output. None of that has been verified against
// the Windows builds of these CLIs (npm installs them as .cmd shims, which go
// through cmd.exe argument parsing), and Windows offers no equivalent that
// skigard can apply from the standard library (restricted tokens and job
// objects are not a substitute for the CLI's own tool controls). So review
// fails closed on Windows instead of running an unverified reviewer.
func Supported() error {
	return util.Errf("automated review is not available on Windows yet: skigard will not run an unverified reviewer. " +
		"Use `skigard review <name> --manual` (it prints the material to review by hand) " +
		"and record the result with `skigard review record`")
}
