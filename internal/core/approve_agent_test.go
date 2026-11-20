package core

import (
	"strings"
	"testing"
)

func TestApproveRefusesUnderAgentAncestor(t *testing.T) {
	a, _ := newApp(t)
	called := false
	a.AgentAncestor = func() (string, bool) { return "Claude Code", true }
	_, err := a.Approve("alpha", ApproveOpts{Confirm: func(string, string) error { called = true; return nil }})
	if err == nil || !strings.Contains(err.Error(), "agent harness (Claude Code)") {
		t.Fatalf("want refusal naming the harness, got %v", err)
	}
	if called {
		t.Fatal("confirmer must not run")
	}
	for _, v := range []string{"CLAUDECODE", "PI_CODING_AGENT", "CODEX_SANDBOX"} {
		if strings.Contains(err.Error(), v) {
			t.Fatalf("refusal names the variable %s: %v", v, err)
		}
	}
}

func TestApproveRefusalDoesNotNameEnvVar(t *testing.T) {
	a, _ := newApp(t)
	a.Env.Getenv = func(k string) string {
		if k == "PI_CODING_AGENT" {
			return "1"
		}
		return ""
	}
	_, err := a.Approve("alpha", ApproveOpts{Confirm: func(string, string) error { return nil }})
	if err == nil || !strings.Contains(err.Error(), "agent harness (Pi)") || strings.Contains(err.Error(), "PI_CODING_AGENT") {
		t.Fatalf("got %v", err)
	}
}

func TestApproveAllowsHumanWithNoAgentAncestor(t *testing.T) {
	a, _ := newApp(t)
	a.AgentAncestor = func() (string, bool) { return "", false }
	if h, ok := a.runningUnderAgent(); ok {
		t.Fatalf("false positive: %s", h)
	}
}
