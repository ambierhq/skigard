package core

import (
	"context"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
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

// Every refusal added with add/config uses the same detection as approve, including
// the ancestor-process walk, so stripping the environment does not help.
func TestAddAndConfigRefuseUnderAgentAncestor(t *testing.T) {
	a, userHome := newApp(t)
	customHarness(t, a, userHome)
	src := t.TempDir()
	writeSkill(t, src, "alpha", nil)
	a.Env.Getenv = func(string) string { return "" }
	a.AgentAncestor = func() (string, bool) { return "Claude Code", true }
	run := func(o AddInstallOpts) error {
		o.Add.Skill = "alpha"
		o.Install.To = []string{"fake"}
		_, err := a.AddAndInstall(context.Background(), src, o)
		return err
	}
	for name, o := range map[string]AddInstallOpts{"force": {ForceInstall: true}, "threshold": {Threshold: 3}} {
		err := run(o)
		if util.CodeOf(err) != util.ExitPolicy || !strings.Contains(err.Error(), "Claude Code") || strings.Contains(err.Error(), "CLAUDECODE") {
			t.Fatalf("%s: %v", name, err)
		}
	}
	setSettings(t, a, store.Settings{Mode: "strict", Threshold: 8})
	if err := run(AddInstallOpts{Mode: "auto"}); util.CodeOf(err) != util.ExitPolicy {
		t.Fatalf("weaker mode: %v", err)
	}
	called := false
	if _, err := a.ConfigSet("mode", "auto", func(string, string) error { called = true; return nil }); util.CodeOf(err) != util.ExitPolicy || called {
		t.Fatalf("config set: %v called=%v", err, called)
	}
	// strict never installs under an agent even with a confirmer.
	out, err := a.AddAndInstall(context.Background(), src, AddInstallOpts{Add: AddOpts{Skill: "alpha"},
		Confirm: func(string, string) error { called = true; return nil }, Install: InstallOpts{To: []string{"fake"}}})
	if err != nil || out[0].Decision != DecisionNeedsApproval || called {
		t.Fatalf("%+v %v", out, err)
	}
}
