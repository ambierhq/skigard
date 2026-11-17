//go:build !unix

package store

// Lock is a no-op on platforms without flock.
func (h Home) Lock() (func(), error) { return func() {}, nil }
