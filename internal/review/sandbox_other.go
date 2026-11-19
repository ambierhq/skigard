//go:build !darwin && !linux

package review

import "fmt"

func osSandboxAvailable() (string, error) {
	return "", fmt.Errorf("no OS sandbox is implemented for this platform")
}

func planSandbox(string, []string, Policy) (string, []string, Isolation, func(), error) {
	return "", nil, Isolation{}, nil, fmt.Errorf("no OS sandbox is implemented for this platform")
}
