//go:build !windows

package review

// Supported reports whether automated review can run on this platform.
func Supported() error { return nil }
