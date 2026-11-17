// Command skigard is a review-gated, lockfile-based manager for agent skills.
package main

import (
	"os"

	"github.com/ambierhq/skigard/internal/cli"
)

func main() { os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr)) }
