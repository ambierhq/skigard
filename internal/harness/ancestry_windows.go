//go:build windows

package harness

import (
	"fmt"
	"syscall"
	"unsafe"
)

// SystemProcTable uses a Toolhelp snapshot. Only image names are available;
// environments of other processes are not read on Windows.
func SystemProcTable(pid int) (Proc, error) {
	h, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return Proc{}, err
	}
	defer syscall.CloseHandle(h)
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	if err := syscall.Process32First(h, &e); err != nil {
		return Proc{}, err
	}
	for {
		if int(e.ProcessID) == pid {
			return Proc{PID: pid, PPID: int(e.ParentProcessID), Name: syscall.UTF16ToString(e.ExeFile[:])}, nil
		}
		if err := syscall.Process32Next(h, &e); err != nil {
			return Proc{}, fmt.Errorf("process %d not found", pid)
		}
	}
}
