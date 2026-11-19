package store

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const lockHelperEnv = "SKIGARD_TEST_LOCK_HELPER_HOME"

// TestLockHelperProcess is not a test: it is the child process of
// TestLockTwoProcesses. It takes the lock, reports, and holds it until its
// stdin closes.
func TestLockHelperProcess(t *testing.T) {
	root := os.Getenv(lockHelperEnv)
	if root == "" {
		t.Skip("helper process only")
	}
	unlock, err := Home{Root: root}.Lock()
	if err != nil {
		os.Stdout.WriteString("error: " + err.Error() + "\n")
		os.Exit(3)
	}
	os.Stdout.WriteString("locked\n")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n') // blocks until the parent closes stdin
	unlock()
}

func TestLockTwoProcesses(t *testing.T) {
	h := Home{Root: t.TempDir()}
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	cmd.Env = append(os.Environ(), lockHelperEnv+"="+h.Root)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := false
	finish := func() {
		if !done {
			done = true
			_ = stdin.Close()
			_ = cmd.Wait()
		}
	}
	defer finish()
	line, _ := bufio.NewReader(stdout).ReadString('\n')
	if strings.TrimSpace(line) != "locked" {
		t.Fatalf("helper did not take the lock: %q", line)
	}
	if unlock, err := h.Lock(); err == nil {
		unlock()
		t.Fatal("second process got the lock while the first held it")
	} else if !strings.Contains(err.Error(), "another skigard process") {
		t.Fatalf("unclear error: %v", err)
	}
	finish()
	var lastErr error
	for i := 0; i < 50; i++ {
		unlock, err := h.Lock()
		if err == nil {
			unlock()
			return
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("lock was not released after the holder exited: %v", lastErr)
}
