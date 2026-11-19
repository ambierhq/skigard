//go:build linux

package review

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// HelperName is the hidden subcommand that applies Landlock and execs the reviewer.
const HelperName = "__sandbox-exec"

// Landlock syscall numbers are the same on every architecture (kernel >= 5.13).
const (
	sysLandlockCreateRuleset = 444
	sysLandlockAddRule       = 445
	sysLandlockRestrictSelf  = 446

	prSetNoNewPrivs = 38

	llCreateRulesetVersion = 1 << 0
	llRulePathBeneath      = 1
	llRuleNetPort          = 2

	llExecute    = 1 << 0
	llWriteFile  = 1 << 1
	llReadFile   = 1 << 2
	llReadDir    = 1 << 3
	llRemoveDir  = 1 << 4
	llRemoveFile = 1 << 5
	llMakeChar   = 1 << 6
	llMakeDir    = 1 << 7
	llMakeReg    = 1 << 8
	llMakeSock   = 1 << 9
	llMakeFifo   = 1 << 10
	llMakeBlock  = 1 << 11
	llMakeSym    = 1 << 12
	llRefer      = 1 << 13 // ABI 2
	llTruncate   = 1 << 14 // ABI 3
	llIoctlDev   = 1 << 15 // ABI 5

	oPath = 0x200000 // O_PATH on amd64, arm64 and riscv64

	llNetBindTCP    = 1 << 0 // ABI 4
	llNetConnectTCP = 1 << 1 // ABI 4
)

var (
	probeOnce  sync.Once
	probeIso   string
	probeNote  string
	probeErr   error
	probeBwrap string
)

func landlockABI() int {
	v, _, errno := syscall.Syscall(sysLandlockCreateRuleset, 0, 0, llCreateRulesetVersion)
	if errno != 0 {
		return 0
	}
	return int(v)
}

// osSandboxAvailable picks the best working mechanism: bubblewrap, then Landlock.
func osSandboxAvailable() (string, error) {
	probeOnce.Do(func() {
		var why []string
		if bw, err := exec.LookPath("bwrap"); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			tr, _ := exec.LookPath("true")
			if tr == "" {
				tr = "/usr/bin/true"
			}
			out, err := exec.CommandContext(ctx, bw, "--unshare-all", "--share-net", "--die-with-parent", "--ro-bind", "/", "/", tr).CombinedOutput() //nolint:gosec // fixed arguments
			if err == nil {
				probeIso, probeBwrap = IsoBwrap, bw
				probeNote = "network is not restricted to port 443 (bubblewrap shares the host network); system tools under /usr stay executable"
				return
			}
			why = append(why, "bwrap present but unusable: "+strings.TrimSpace(string(out)))
		}
		if abi := landlockABI(); abi >= 1 {
			probeIso = IsoLandlock
			if abi < 4 {
				probeNote = fmt.Sprintf("network is not restricted (Landlock ABI %d < 4)", abi)
			}
			return
		} else {
			why = append(why, "Landlock is not available in this kernel")
		}
		probeErr = fmt.Errorf("%s", strings.Join(why, "; "))
	})
	return probeIso, probeErr
}

func planSandbox(path string, args []string, p Policy) (cmdPath string, cmdArgs []string, iso Isolation, cleanup func(), err error) {
	mode, err := osSandboxAvailable()
	if err != nil {
		return "", nil, Isolation{}, nil, err
	}
	noop := func() {}
	if preferLandlock && landlockABI() >= 1 {
		mode = IsoLandlock
		probeNote = ""
		if abi := landlockABI(); abi < 4 {
			probeNote = fmt.Sprintf("network is not restricted (Landlock ABI %d < 4)", abi)
		}
	}
	if mode == IsoBwrap {
		a := bwrapArgs(p)
		a = append(a, "--", path)
		return probeBwrap, append(a, args...), Isolation{Mode: IsoBwrap, Note: probeNote}, noop, nil
	}
	self, err := os.Executable()
	if err != nil {
		return "", nil, Isolation{}, nil, err
	}
	a := []string{HelperName}
	for _, r := range p.Read {
		a = append(a, "-r", r)
	}
	for _, x := range p.Exec {
		a = append(a, "-x", x)
	}
	for _, l := range p.LockDirs {
		a = append(a, "-l", l)
	}
	for _, w := range append([]string{p.Work}, p.Write...) {
		a = append(a, "-w", w)
	}
	a = append(a, "--", path)
	return self, append(a, args...), Isolation{Mode: IsoLandlock, Note: probeNote}, noop, nil
}

// systemDirs are bound read-only (symlinks such as /bin -> usr/bin are recreated).
var systemDirs = []string{"/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64"}

var etcFiles = []string{"/etc/resolv.conf", "/etc/hosts", "/etc/nsswitch.conf", "/etc/host.conf", "/etc/ssl",
	"/etc/ca-certificates", "/etc/pki", "/etc/ca-certificates.conf", "/etc/ld.so.cache", "/etc/ld.so.conf",
	"/etc/ld.so.conf.d", "/etc/localtime", "/etc/alternatives", "/etc/gai.conf"}

func bwrapArgs(p Policy) []string {
	a := []string{"--unshare-all", "--share-net", "--die-with-parent", "--new-session", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp"}
	for _, d := range systemDirs {
		if t, err := os.Readlink(d); err == nil {
			a = append(a, "--symlink", t, d)
		} else if _, err := os.Stat(d); err == nil {
			a = append(a, "--ro-bind", d, d)
		}
	}
	for _, f := range etcFiles {
		a = append(a, "--ro-bind-try", f, f)
	}
	if p.Home != "" && !strings.HasPrefix(p.Home, "/tmp/") {
		a = append(a, "--tmpfs", p.Home)
	}
	read := append([]string{}, p.Read...)
	sort.Strings(read) // parents before children
	for _, r := range read {
		if underAny(r, systemDirs) {
			continue
		}
		a = append(a, "--ro-bind-try", r, r)
	}
	// Lock directories: an empty tmpfs with the existing entries bound read-only
	// on top, so the harness can create name.lock entries (they vanish with the
	// sandbox) without being able to change what is there.
	for _, l := range p.LockDirs {
		a = append(a, "--tmpfs", l)
		if ents, err := os.ReadDir(l); err == nil {
			for _, e := range ents {
				x := l + "/" + e.Name()
				a = append(a, "--ro-bind-try", x, x)
			}
		}
	}
	for _, w := range p.Write {
		a = append(a, "--bind-try", w, w)
	}
	return append(a, "--bind", p.Work, p.Work, "--chdir", p.Work)
}

func underAny(p string, dirs []string) bool {
	for _, d := range dirs {
		if p == d || strings.HasPrefix(p, d+"/") {
			return true
		}
	}
	return false
}

// ---- Landlock helper -------------------------------------------------

type rulesetAttr struct {
	fs  uint64
	net uint64
}

type pathBeneath struct {
	access uint64
	fd     int32
}

type netPort struct {
	access uint64
	port   uint64
}

func fsRights(abi int) uint64 {
	r := uint64(1<<13 - 1)
	if abi >= 2 {
		r |= llRefer
	}
	if abi >= 3 {
		r |= llTruncate
	}
	if abi >= 5 {
		r |= llIoctlDev
	}
	return r
}

const (
	rightsRead = llReadFile | llReadDir
	rightsFile = llExecute | llWriteFile | llReadFile | llTruncate | llIoctlDev
)

// SandboxHelper is the re-exec entry point: `skigard __sandbox-exec -r P -x P -w P -l P -- cmd args`.
// It applies Landlock to this thread with PR_SET_NO_NEW_PRIVS and execs the
// command, so the command inherits the restrictions. It only ever removes
// access. It returns a process exit status and is only used when it fails.
func SandboxHelper(argv []string) int {
	var reads, execs, writes, locks []string
	i := 0
loop:
	for i < len(argv) {
		switch argv[i] {
		case "-r", "-x", "-w", "-l":
			if i+1 >= len(argv) {
				fmt.Fprintln(os.Stderr, "skigard: bad sandbox helper arguments")
				return 2
			}
			switch argv[i] {
			case "-r":
				reads = append(reads, argv[i+1])
			case "-x":
				execs = append(execs, argv[i+1])
			case "-l":
				locks = append(locks, argv[i+1])
			default:
				writes = append(writes, argv[i+1])
			}
			i += 2
		case "--":
			i++
			break loop
		default:
			fmt.Fprintln(os.Stderr, "skigard: bad sandbox helper arguments")
			return 2
		}
	}
	if i >= len(argv) {
		fmt.Fprintln(os.Stderr, "skigard: sandbox helper: no command")
		return 2
	}
	cmd := argv[i:]
	runtime.LockOSThread()
	if err := applyLandlock(reads, execs, writes, locks); err != nil {
		fmt.Fprintf(os.Stderr, "skigard: cannot apply Landlock: %v\n", err)
		return 126
	}
	err := syscall.Exec(cmd[0], cmd, os.Environ()) //nolint:gosec // the command is chosen by skigard, never by skill content
	fmt.Fprintf(os.Stderr, "skigard: exec %s: %v\n", cmd[0], err)
	return 127
}

func applyLandlock(reads, execs, writes, locks []string) error {
	abi := landlockABI()
	if abi < 1 {
		return fmt.Errorf("kernel has no Landlock")
	}
	handled := fsRights(abi)
	attr := rulesetAttr{fs: handled}
	size := 8
	if abi >= 4 {
		attr.net = llNetBindTCP | llNetConnectTCP
		size = 16
	}
	fd, _, errno := syscall.Syscall(sysLandlockCreateRuleset, uintptr(unsafe.Pointer(&attr)), uintptr(size), 0) //nolint:gosec // kernel ABI struct
	if errno != 0 {
		return errno
	}
	rfd := int(fd)
	defer func() { _ = syscall.Close(rfd) }()

	add := func(path string, rights uint64) {
		f, err := syscall.Open(path, oPath|syscall.O_CLOEXEC, 0)
		if err != nil {
			return // a missing path simply grants nothing
		}
		defer func() { _ = syscall.Close(f) }()
		var st syscall.Stat_t
		if syscall.Fstat(f, &st) != nil {
			return
		}
		rights &= handled
		if st.Mode&syscall.S_IFMT != syscall.S_IFDIR {
			rights &= rightsFile
		}
		if rights == 0 {
			return
		}
		pb := pathBeneath{access: rights, fd: int32(f)}                                                              //nolint:gosec // fd fits in int32
		syscall.Syscall6(sysLandlockAddRule, uintptr(rfd), llRulePathBeneath, uintptr(unsafe.Pointer(&pb)), 0, 0, 0) //nolint:gosec,errcheck // kernel ABI struct; failure only narrows access
	}
	// System files a dynamically linked program needs.
	for _, d := range []string{"/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64"} {
		add(d, rightsRead)
	}
	for _, d := range []string{"/usr/lib", "/usr/lib64", "/usr/libexec", "/lib", "/lib32", "/lib64"} {
		add(d, llExecute)
	}
	for _, f := range []string{"/bin/sh", "/bin/bash", "/usr/bin/env", "/usr/bin/sh", "/usr/bin/bash", "/usr/bin/dash"} {
		add(f, llExecute)
	}
	for _, f := range etcFiles {
		add(f, rightsRead)
	}
	add("/proc", rightsRead)
	add("/sys/devices/system/cpu", rightsRead)
	add("/dev/null", llReadFile|llWriteFile|llTruncate|llIoctlDev)
	for _, f := range []string{"/dev/urandom", "/dev/random", "/dev/zero"} {
		add(f, llReadFile)
	}
	for _, r := range reads {
		add(r, rightsRead)
	}
	for _, x := range execs {
		add(x, llExecute|rightsRead)
	}
	for _, w := range writes {
		add(w, handled)
	}
	for _, l := range locks {
		add(l, llMakeDir|llRemoveDir) // creating and removing name.lock directories only
	}
	if abi >= 4 {
		np := netPort{access: llNetConnectTCP, port: 443}
		syscall.Syscall6(sysLandlockAddRule, uintptr(rfd), llRuleNetPort, uintptr(unsafe.Pointer(&np)), 0, 0, 0) //nolint:gosec,errcheck // kernel ABI struct
	}
	if _, _, e := syscall.RawSyscall(syscall.SYS_PRCTL, prSetNoNewPrivs, 1, 0); e != 0 {
		return fmt.Errorf("PR_SET_NO_NEW_PRIVS: %w", e)
	}
	if _, _, e := syscall.RawSyscall(sysLandlockRestrictSelf, uintptr(rfd), 0, 0); e != 0 {
		return fmt.Errorf("landlock_restrict_self: %w", e)
	}
	return nil
}

func landlockABIPortable() int { return landlockABI() }
