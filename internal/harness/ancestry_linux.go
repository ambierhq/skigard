//go:build linux

package harness

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SystemProcTable reads /proc. Unreadable files leave the field empty.
func SystemProcTable(pid int) (Proc, error) {
	dir := "/proc/" + strconv.Itoa(pid) + "/"
	stat, err := os.ReadFile(dir + "stat")
	if err != nil {
		return Proc{}, err
	}
	// pid (comm) state ppid ...; comm may contain spaces and parentheses.
	i, j := bytes.IndexByte(stat, '('), bytes.LastIndexByte(stat, ')')
	if i < 0 || j < i {
		return Proc{}, fmt.Errorf("bad stat for %d", pid)
	}
	f := strings.Fields(string(stat[j+1:]))
	if len(f) < 2 {
		return Proc{}, fmt.Errorf("bad stat for %d", pid)
	}
	ppid, err := strconv.Atoi(f[1])
	if err != nil {
		return Proc{}, err
	}
	p := Proc{PID: pid, PPID: ppid, Name: string(stat[i+1 : j])}
	if b, err := os.ReadFile(dir + "cmdline"); err == nil {
		p.Argv = splitNul(b)
	}
	if b, err := os.ReadFile(dir + "environ"); err == nil {
		p.Env = envMap(splitNul(b))
	}
	return p, nil
}
