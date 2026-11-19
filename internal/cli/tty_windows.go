//go:build windows

package cli

import (
	"os"
	"syscall"
)

// openConsole opens a console device by name ("CONIN$" or "CONOUT$"). These
// attach to the console of the process even when stdin and stdout are
// redirected, which is the Windows analogue of /dev/tty. It fails when the
// process has no console (detached, or started with no console window).
func openConsole(name string) (*os.File, error) {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, err
	}
	var mode uint32
	if err := syscall.GetConsoleMode(h, &mode); err != nil {
		_ = syscall.CloseHandle(h)
		return nil, err
	}
	return os.NewFile(uintptr(h), name), nil
}

// ttyConfirm shows the summary on the console and requires the human to type
// want. It deliberately has no flag, environment variable or stdin fallback:
// if there is no console, there is no approval.
func ttyConfirm(summary, want string) error {
	in, err := openConsole("CONIN$")
	if err != nil {
		return noTerminal("no console found")
	}
	defer in.Close()
	out, err := openConsole("CONOUT$")
	if err != nil {
		return noTerminal("no console found")
	}
	defer out.Close()
	return promptOn(in, out, summary, want)
}

// stdinIsTerminal reports whether prompts can be answered interactively.
func stdinIsTerminal() bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(os.Stdin.Fd()), &mode) == nil
}
