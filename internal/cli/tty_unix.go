//go:build unix

package cli

import (
	"io"
	"os"
)

// ttyConfirm shows the summary on the controlling terminal and requires the
// human to type want. It deliberately has no flag, environment variable or
// stdin fallback: if there is no terminal, there is no approval.
func ttyConfirm(summary, want string) error {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return noTerminal("no controlling terminal found")
	}
	defer tty.Close()
	return promptOn(tty, tty, summary, want)
}

// stdinIsTerminal reports whether prompts can be answered interactively.
func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// openTerminal opens the controlling terminal for prompts that must come from a human.
func openTerminal() (io.Reader, io.Writer, func(), error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, nil, noTerminal("no controlling terminal found")
	}
	return tty, tty, func() { tty.Close() }, nil
}
