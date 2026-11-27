package cli

import (
	"bytes"
	"strings"
	"testing"
)

// promptOn is driven with in-memory buffers, never with the console, so this cannot block on a terminal.
func TestPromptLayout(t *testing.T) {
	var out bytes.Buffer
	err := promptOn(strings.NewReader("nope\n"), &out, "Approve skill \"x\"\n\n  source  s\n\n", "x")
	if err == nil {
		t.Fatal("wrong answer confirmed")
	}
	want := "\nApprove skill \"x\"\n\n  source  s\n\nType x to confirm, anything else to cancel: "
	if out.String() != want {
		t.Errorf("got %q\nwant %q", out.String(), want)
	}
	if strings.Contains(out.String(), "\n\n\n") {
		t.Error("more than one blank line before the prompt")
	}
}
