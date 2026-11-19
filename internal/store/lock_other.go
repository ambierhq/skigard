//go:build !unix && !windows

package store

// Lock is a no-op on platforms that have neither flock nor LockFileEx. skigard
// does not release for such targets: the release matrix is darwin, linux and windows.
func (h Home) Lock() (func(), error) { return func() {}, nil }
