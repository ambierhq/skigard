//go:build !linux && !darwin && !windows

package harness

import "errors"

// SystemProcTable is unavailable here; the ancestor check fails open.
func SystemProcTable(int) (Proc, error) { return Proc{}, errors.New("process table not supported") }
