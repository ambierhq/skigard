package harness

import "os/exec"

func execLookPath(name string) (string, error) { return exec.LookPath(name) }
