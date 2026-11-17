//go:build unix

package store

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/ambierhq/skigard/internal/util"
)

// Lock takes an exclusive advisory lock on the home so two skigard processes
// cannot rewrite the lock file or the skills directory at the same time.
func (h Home) Lock() (func(), error) {
	if err := os.MkdirAll(h.StateDir(), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(h.StateDir(), ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, util.Errf("another skigard process is using %s; try again in a moment", h.Root)
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
