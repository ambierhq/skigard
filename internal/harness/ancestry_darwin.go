//go:build darwin

package harness

import (
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"syscall"
	"unsafe"
)

const kinfoPpidOffset = 560 // kinfo_proc.kp_eproc.e_ppid on 64-bit darwin

// sysctl is the raw sysctl(2) call: the syscall package exports no helper for
// numeric MIBs on darwin and a new dependency is not allowed.
func sysctl(mib []int32, buf []byte) (int, error) {
	n := uintptr(len(buf))
	//nolint:gosec // G103: pointers to live Go slices passed straight to sysctl(2)
	_, _, e := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)), 0, 0)
	if e != 0 {
		return 0, e
	}
	return int(n), nil
}

// SystemProcTable uses KERN_PROC_PID and KERN_PROCARGS2 (same-user processes).
func SystemProcTable(pid int) (Proc, error) {
	if pid <= 0 || pid > math.MaxInt32 {
		return Proc{}, fmt.Errorf("bad pid %d", pid)
	}
	pid32 := int32(pid)
	kp := make([]byte, 1024)
	n, err := sysctl([]int32{1, 14, 1, pid32}, kp) // CTL_KERN, KERN_PROC, KERN_PROC_PID
	if err != nil {
		return Proc{}, err
	}
	if n < kinfoPpidOffset+4 {
		return Proc{}, fmt.Errorf("short kinfo_proc for %d", pid)
	}
	p := Proc{PID: pid, PPID: int(binary.LittleEndian.Uint32(kp[kinfoPpidOffset:]))}
	buf := make([]byte, 1<<20)
	if n, err := sysctl([]int32{1, 49, pid32}, buf); err == nil && n > 4 { // KERN_PROCARGS2
		parseProcArgs2(&p, buf[:n])
	}
	return p, nil
}

// parseProcArgs2: int32 argc, exec path, NUL padding, argv[argc], env until an empty string.
func parseProcArgs2(p *Proc, b []byte) {
	argc := int(binary.LittleEndian.Uint32(b))
	parts := splitNul(b[4:])
	if len(parts) == 0 {
		return
	}
	p.Name = filepath.Base(parts[0])
	rest := parts[1:]
	for len(rest) > 0 && rest[0] == "" {
		rest = rest[1:]
	}
	if argc > len(rest) {
		argc = len(rest)
	}
	p.Argv = rest[:argc]
	p.Env = envMap(rest[argc:])
}
