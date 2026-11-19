//go:build !linux

package review

import (
	"fmt"
	"os"
)

// HelperName is the hidden subcommand used by the Landlock sandbox on Linux.
const HelperName = "__sandbox-exec"

// SandboxHelper is only implemented on Linux.
func SandboxHelper([]string) int {
	fmt.Fprintln(os.Stderr, "skigard: the sandbox helper is only available on Linux")
	return 2
}

func landlockABIPortable() int { return 0 }

var preferLandlock bool
