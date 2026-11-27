//go:build linux || darwin || freebsd

package term

import (
	"os"
	"syscall"
	"unsafe"
)

func terminalColumns(f *os.File) int {
	var ws struct{ Row, Col, X, Y uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws))) //nolint:gosec // TIOCGWINSZ needs a pointer to a winsize struct
	if errno != 0 {
		return 0
	}
	return int(ws.Col)
}
