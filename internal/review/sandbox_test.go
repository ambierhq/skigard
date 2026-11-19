//go:build !windows

package review

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestMain doubles as the sandboxed child process and as the Landlock helper.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == HelperName {
		os.Exit(SandboxHelper(os.Args[2:]))
	}
	if op := os.Getenv("SKIGARD_TEST_CHILD"); op != "" {
		os.Exit(sandboxChild(op, os.Getenv("SKIGARD_TEST_ARG")))
	}
	os.Exit(m.Run())
}

func sandboxChild(op, arg string) int {
	var err error
	switch op {
	case "read":
		var b []byte
		b, err = os.ReadFile(arg)
		if err == nil {
			fmt.Print(string(b))
		}
	case "write":
		err = os.WriteFile(arg, []byte("x"), 0o600)
	case "connect":
		var c net.Conn
		c, err = net.DialTimeout("tcp", arg, 3*time.Second)
		if err == nil {
			c.Close()
		}
	case "exec":
		err = exec.Command(arg).Run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "denied:", err)
		return 1
	}
	return 0
}

type sbEnv struct {
	pol         Policy
	home, allow string
	work        string
	iso         Isolation
}

func newSBEnv(t *testing.T) *sbEnv {
	t.Helper()
	mode, err := osSandboxAvailable()
	if err != nil {
		if os.Getenv("SKIGARD_REQUIRE_SANDBOX") != "" || os.Getenv("GITHUB_ACTIONS") != "" { // CI must exercise the sandbox, not skip it
			t.Fatalf("an OS sandbox is required here but none works: %v", err)
		}
		t.Skipf("no OS sandbox available on this machine (%v); the sandbox tests did NOT run", err)
	}
	self := selfPath()
	e := &sbEnv{}
	e.home, e.allow, e.work = realTemp(t), realTemp(t), realTemp(t)
	if err := os.MkdirAll(filepath.Join(e.home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	must(t, os.WriteFile(filepath.Join(e.home, ".ssh", "id_rsa"), []byte("PRIVATE KEY"), 0o600))
	must(t, os.WriteFile(filepath.Join(e.allow, "cfg"), []byte("allowed"), 0o600))
	e.pol = Policy{Work: e.work, Read: []string{filepath.Dir(self), e.allow}, Exec: []string{self}, Home: e.home}
	t.Logf("OS sandbox under test: %s", mode)
	return e
}

func selfPath() string {
	p, _ := os.Executable()
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// denied reports that the child ran inside the sandbox and was refused. A
// launch failure must not count as a pass.
func denied(out string, err error) bool {
	return err != nil && strings.Contains(out, "denied:")
}

func realTemp(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	return d
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// run starts this test binary as the sandboxed child.
func (e *sbEnv) run(t *testing.T, sandboxed bool, op, arg string) (string, error) {
	t.Helper()
	self := selfPath()
	path, args := self, []string(nil)
	if sandboxed {
		p, a, iso, cleanup, err := planSandbox(self, nil, e.pol)
		must(t, err)
		defer cleanup()
		path, args, e.iso = p, a, iso
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = e.work
	cmd.Env = []string{"SKIGARD_TEST_CHILD=" + op, "SKIGARD_TEST_ARG=" + arg, "HOME=" + e.home, "TMPDIR=" + e.work, "PATH=/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestSandboxAllowedRead(t *testing.T) {
	e := newSBEnv(t)
	arg := filepath.Join(e.allow, "cfg")
	if out, err := e.run(t, false, "read", arg); err != nil || out != "allowed" {
		t.Fatalf("control run without sandbox failed: %v %q", err, out)
	}
	out, err := e.run(t, true, "read", arg)
	if err != nil || out != "allowed" {
		t.Fatalf("allowed read failed under %s: %v %q", e.iso.Mode, err, out)
	}
}

func TestSandboxDeniesReadOutsideAllowlist(t *testing.T) {
	e := newSBEnv(t)
	key := filepath.Join(e.home, ".ssh", "id_rsa")
	if out, err := e.run(t, false, "read", key); err != nil || out != "PRIVATE KEY" {
		t.Fatalf("control run without sandbox failed: %v %q", err, out)
	}
	out, err := e.run(t, true, "read", key)
	if !denied(out, err) || strings.Contains(out, "PRIVATE KEY") {
		t.Fatalf("read of a file in the fake HOME outside the allowlist succeeded under %s: %q", e.iso.Mode, out)
	}
}

func TestSandboxDeniesWriteOutsideWorkDir(t *testing.T) {
	e := newSBEnv(t)
	outside := filepath.Join(e.home, "pwned")
	if out, err := e.run(t, true, "write", outside); !denied(out, err) {
		t.Fatalf("write outside the temp dir succeeded under %s", e.iso.Mode)
	}
	if _, err := os.Stat(outside); err == nil {
		t.Fatal("file was created outside the temp dir")
	}
	// the read-only allowlist is not writable either
	if out, err := e.run(t, true, "write", filepath.Join(e.allow, "cfg")); !denied(out, err) {
		t.Fatalf("write into a read-only allowlisted dir succeeded under %s", e.iso.Mode)
	}
	inside := filepath.Join(e.work, "ok")
	if out, err := e.run(t, true, "write", inside); err != nil {
		t.Fatalf("write inside the temp dir failed under %s: %v %s", e.iso.Mode, err, out)
	}
}

func TestSandboxDeniesNonHTTPSPort(t *testing.T) {
	e := newSBEnv(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	must(t, err)
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	addr := ln.Addr().String()
	if out, err := e.run(t, false, "connect", addr); err != nil {
		t.Fatalf("control connect without sandbox failed: %v %s", err, out)
	}
	out, err := e.run(t, true, "connect", addr)
	switch {
	case e.iso.Mode == IsoBwrap:
		t.Skipf("bubblewrap shares the host network, so it cannot restrict ports (recorded as a known gap): %s", e.iso.Note)
	case e.iso.Mode == IsoLandlock && landlockABIForTest() < 4:
		t.Skipf("Landlock ABI %d cannot restrict TCP connect (needs 4): %s", landlockABIForTest(), e.iso.Note)
	}
	if !denied(out, err) {
		t.Fatalf("connection to a non-443 port succeeded under %s", e.iso.Mode)
	}
}

func TestSandboxDeniesExecOutsideAllowlist(t *testing.T) {
	e := newSBEnv(t)
	tool, err := exec.LookPath("true")
	must(t, err)
	out, err := e.run(t, true, "exec", tool)
	if !denied(out, err) {
		if e.iso.Mode == IsoBwrap {
			t.Skip("bubblewrap exposes /usr read-only and does not restrict exec (documented gap)")
		}
		t.Fatalf("executing %s was not refused under %s: %v %s", tool, e.iso.Mode, err, out)
	}
}

func TestSandboxUnavailableNeedsPolicyOnlyFlag(t *testing.T) {
	bin := fakeHarness(t, `echo '{"verdict":"pass","summary":"ok","findings":[]}'`)
	d := t.TempDir()
	must(t, os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("---\nname: demo\n---\nhi\n"), 0o600))
	in, _ := BuildInput(d, "demo", nil)
	tpl, _ := TemplateFor("pi")
	env := []string{"PATH=/usr/bin:/bin"}
	_, _, err := Runner{Environ: env, BinOverride: bin, noSandboxForTest: true}.Run(context.Background(), tpl, "", in)
	if err == nil || !strings.Contains(err.Error(), "--allow-policy-only") {
		t.Fatalf("expected a refusal that names --allow-policy-only, got %v", err)
	}
	_, iso, err := Runner{Environ: env, BinOverride: bin, noSandboxForTest: true, AllowPolicyOnly: true}.Run(context.Background(), tpl, "", in)
	if err != nil || iso.Mode != IsoPolicyOnly {
		t.Fatalf("verified template with the flag should run policy-only: %v %+v", err, iso)
	}
	unverified, _ := TemplateFor("gemini-cli")
	_, _, err = Runner{Environ: env, BinOverride: bin, noSandboxForTest: true, AllowPolicyOnly: true}.Run(context.Background(), unverified, "", in)
	if err == nil || !strings.Contains(err.Error(), "unverified") {
		t.Fatalf("an unverified template must not run policy-only, got %v", err)
	}
}

func TestBuildPolicyFollowsShims(t *testing.T) {
	root := realTemp(t)
	pkg := filepath.Join(root, "Cellar", "tool", "1.0")
	must(t, os.MkdirAll(filepath.Join(pkg, "libexec", "bin"), 0o755))
	must(t, os.MkdirAll(filepath.Join(pkg, "bin"), 0o755))
	target := filepath.Join(pkg, "libexec", "bin", "tool")
	must(t, os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755))
	shim := filepath.Join(pkg, "bin", "tool")
	must(t, os.WriteFile(shim, []byte(fmt.Sprintf("#!/bin/bash\nexec %q \"$@\"\n", target)), 0o755))
	link := filepath.Join(root, "bin")
	must(t, os.MkdirAll(link, 0o755))
	must(t, os.Symlink(shim, filepath.Join(link, "tool")))
	home := realTemp(t)
	must(t, os.MkdirAll(filepath.Join(home, ".tool"), 0o755))
	pol := BuildPolicy(filepath.Join(link, "tool"), Template{ConfigPaths: []string{".tool", ".missing"}}, home, "/tmp/w")
	has := func(list []string, p string) bool {
		for _, x := range list {
			if x == p {
				return true
			}
		}
		return false
	}
	if !has(pol.Exec, pkg) || !has(pol.Read, pkg) {
		t.Errorf("install root %s should be readable and executable: %+v", pkg, pol)
	}
	if has(pol.Exec, link) || has(pol.Read, root) {
		t.Errorf("the shared bin dir and its parent must not be granted: %+v", pol)
	}
	if !has(pol.Read, filepath.Join(home, ".tool")) || has(pol.Read, filepath.Join(home, ".missing")) || has(pol.Read, home) {
		t.Errorf("only existing config paths under HOME may be readable: %+v", pol.Read)
	}
	if has(pol.Exec, filepath.Join(home, ".tool")) {
		t.Errorf("config dirs must not be executable")
	}
}

func landlockABIForTest() int {
	if runtime.GOOS != "linux" {
		return 0
	}
	return landlockABIPortable()
}
