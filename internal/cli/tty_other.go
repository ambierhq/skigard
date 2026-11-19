//go:build !unix && !windows

package cli

// ttyConfirm has no terminal device to open on this platform, so approval is
// refused. There is no fallback.
func ttyConfirm(_, _ string) error { return noTerminal("not supported on this platform") }

func stdinIsTerminal() bool { return false }
