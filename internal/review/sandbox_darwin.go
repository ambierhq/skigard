//go:build darwin

package review

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const sandboxExec = "/usr/bin/sandbox-exec"

var (
	probeOnce sync.Once
	probeErr  error
)

// osSandboxAvailable checks that sandbox-exec exists and can apply a profile
// here (it cannot when skigard already runs inside another Seatbelt sandbox).
func osSandboxAvailable() (string, error) {
	probeOnce.Do(func() {
		if _, err := os.Stat(sandboxExec); err != nil {
			probeErr = fmt.Errorf("%s not found", sandboxExec)
			return
		}
		out, err := exec.CommandContext(context.Background(), sandboxExec, "-p", "(version 1)(allow default)", "/usr/bin/true").CombinedOutput()
		if err != nil {
			probeErr = fmt.Errorf("sandbox-exec cannot apply a profile here (already inside a sandbox?): %s", strings.TrimSpace(string(out)))
		}
	})
	return IsoSeatbelt, probeErr
}

// sbQuote returns s as a Seatbelt string literal.
func sbQuote(s string) (string, error) {
	if strings.ContainsAny(s, "\x00\n\r") {
		return "", fmt.Errorf("path %q cannot be put in a sandbox profile", s)
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`, nil
}

// Profile generates the Seatbelt profile for a policy. It starts from
// (deny default) and lists what is allowed. It contains paths only, never
// skill content.
func Profile(p Policy) (string, error) {
	var b strings.Builder
	b.WriteString("(version 1)\n(deny default)\n")
	// Process basics. Mach lookups are allowed in general: the OS services a
	// CLI needs (dns, trust, notify, logging) are many and unstable across macOS versions.
	b.WriteString(`(allow process-fork)
(allow signal (target self))
(allow sysctl-read)
(allow mach-lookup)
(allow ipc-posix-shm)
(allow file-read-metadata)
(allow file-read-data (literal "/"))
(allow system-socket)
(allow user-preference-read)
(allow file-ioctl (literal "/dev/dtracehelper"))
`)
	sub := func(op string, paths []string, filter string) error {
		for _, x := range paths {
			q, err := sbQuote(x)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, "(allow %s (%s %s))\n", op, filter, q)
		}
		return nil
	}
	both := func(op string, paths []string) error {
		for _, x := range paths {
			q, err := sbQuote(x)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, "(allow %s (subpath %s) (literal %s))\n", op, q, q)
		}
		return nil
	}
	// System files a process needs to run at all.
	system := []string{"/usr/lib", "/usr/share", "/System", "/Library/Frameworks", "/private/var/db/dyld",
		"/private/var/db/timezone", "/private/etc/ssl", "/private/etc/resolv.conf", "/private/etc/hosts",
		"/private/etc/localtime", "/private/etc/protocols", "/private/etc/services", "/usr/bin/env", "/bin", "/usr/bin",
		"/opt/homebrew/lib", "/opt/homebrew/opt", "/opt/homebrew/Cellar", "/usr/local/lib", "/usr/local/opt", "/usr/local/Cellar",
		"/Library/Preferences/com.apple.security.plist", "/private/var/db/mds"}
	for _, g := range []string{"/opt/homebrew/etc/openssl*", "/usr/local/etc/openssl*", "/opt/homebrew/etc/ca-certificates"} {
		m, _ := filepath.Glob(g)
		system = append(system, m...)
	}
	var sys []string
	for _, s := range system {
		if _, err := os.Lstat(s); err == nil {
			sys = append(sys, s)
		}
	}
	if err := both("file-read*", sys); err != nil {
		return "", err
	}
	if err := both("file-read*", p.Read); err != nil {
		return "", err
	}
	if err := both("file-map-executable", append(append([]string{}, sys...), p.MapExec...)); err != nil {
		return "", err
	}
	// Only the harness and its runtime may be executed.
	exe := append([]string{"/bin/sh", "/bin/bash", "/usr/bin/env"}, p.Exec...)
	if err := both("process-exec*", exe); err != nil {
		return "", err
	}
	// Writes: the temporary working directory and a few device nodes.
	if err := both("file-read* file-write*", append([]string{p.Work}, p.Write...)); err != nil {
		return "", err
	}
	for _, d := range []string{"/dev/null", "/dev/dtracehelper", "/dev/tty"} {
		if err := sub("file-read* file-write*", []string{d}, "literal"); err != nil {
			return "", err
		}
	}
	if err := sub("file-read*", []string{"/dev/urandom", "/dev/random", "/dev/zero"}, "literal"); err != nil {
		return "", err
	}
	// Lock entries (name.lock) next to credentials: create and remove only.
	for _, d := range p.LockDirs {
		// #"..." is a raw literal: backslashes are regex escapes, a quote cannot be escaped.
		if strings.ContainsAny(d, "\x00\n\r\"") {
			return "", fmt.Errorf("path %q cannot be put in a sandbox profile", d)
		}
		fmt.Fprintf(&b, "(allow file-write* (regex #\"^%s/[^/]+\\.lock(/.*)?$\"))\n", regexp.QuoteMeta(d))
	}
	// Network: HTTPS and name resolution only.
	b.WriteString(`(allow network-outbound (remote tcp "*:443"))
(allow network-outbound (literal "/private/var/run/mDNSResponder"))
(allow network-outbound (remote udp "*:53"))
`)
	return b.String(), nil
}

// planSandbox wraps a command in sandbox-exec. The profile is written to a
// private (0700) temp directory; the returned cleanup removes it.
func planSandbox(path string, args []string, p Policy) (cmdPath string, cmdArgs []string, iso Isolation, cleanup func(), err error) {
	if _, err := osSandboxAvailable(); err != nil {
		return "", nil, Isolation{}, nil, err
	}
	prof, err := Profile(p)
	if err != nil {
		return "", nil, Isolation{}, nil, err
	}
	dir, err := os.MkdirTemp("", "skigard-sb-")
	if err != nil {
		return "", nil, Isolation{}, nil, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	file := filepath.Join(dir, "profile.sb")
	if err := os.WriteFile(file, []byte(prof), 0o600); err != nil {
		cleanup()
		return "", nil, Isolation{}, nil, err
	}
	a := append([]string{"-f", file, path}, args...)
	return sandboxExec, a, Isolation{Mode: IsoSeatbelt, Note: "mach services are not restricted; UDP 53 and the system resolver are allowed"}, cleanup, nil
}
