//go:build windows

package store

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/ambierhq/skigard/internal/util"
)

const (
	lockfileFailImmediately = 0x1
	lockfileExclusiveLock   = 0x2
	errorLockViolation      = syscall.Errno(33)
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

// lockWait is how long Lock keeps retrying before it gives up. It is a var so
// tests can shorten it; there is no user-facing knob.
var lockWait = 10 * time.Second

func lockFileEx(h syscall.Handle, flags uint32) error {
	var ol syscall.Overlapped
	r, _, e := procLockFileEx.Call(uintptr(h), uintptr(flags), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		return e
	}
	return nil
}

func unlockFileEx(h syscall.Handle) {
	var ol syscall.Overlapped
	_, _, _ = procUnlockFileEx.Call(uintptr(h), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
}

// Lock takes an exclusive lock (LockFileEx on the first byte of .lock) so two
// skigard processes cannot rewrite the lock file or the skills directory at the
// same time. It retries for up to ten seconds, then fails with a clear message.
// The lock is released when the process exits, even if it crashes.
func (h Home) Lock() (func(), error) {
	if err := os.MkdirAll(h.StateDir(), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(h.StateDir(), ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	handle := syscall.Handle(f.Fd())
	deadline := time.Now().Add(lockWait)
	for {
		err = lockFileEx(handle, lockfileExclusiveLock|lockfileFailImmediately)
		if err == nil {
			return func() { unlockFileEx(handle); _ = f.Close() }, nil
		}
		if !errors.Is(err, errorLockViolation) || !time.Now().Before(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = f.Close()
	if errors.Is(err, errorLockViolation) {
		return nil, util.Errf("another skigard process is using %s; try again in a moment", h.Root)
	}
	return nil, util.Errf("cannot lock %s: %v", h.Root, err)
}
