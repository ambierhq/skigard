// Command skigard is a safer way to manage agent skills.
package main

import (
	"os"

	"github.com/ambierhq/skigard/internal/cli"
	"github.com/ambierhq/skigard/internal/review"
)

func main() {
	// Hidden re-exec entry used by the Linux Landlock reviewer sandbox. It can
	// only remove access from the command it then starts.
	if len(os.Args) > 1 && os.Args[1] == review.HelperName {
		os.Exit(review.SandboxHelper(os.Args[2:]))
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
