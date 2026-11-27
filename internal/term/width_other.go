//go:build !(linux || darwin || freebsd)

package term

import "os"

func terminalColumns(*os.File) int { return 0 }
