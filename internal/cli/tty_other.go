//go:build !unix && !windows

package cli

import "io"

// ttyConfirm has no terminal device to open on this platform, so approval is
// refused. There is no fallback.
func ttyConfirm(_, _ string) error { return noTerminal("not supported on this platform") }

func stdinIsTerminal() bool { return false }

func openTerminal() (io.Reader, io.Writer, func(), error) {
	return nil, nil, nil, noTerminal("not supported on this platform")
}
