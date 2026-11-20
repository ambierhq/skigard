package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/util"
)

func TestConfigSet(t *testing.T) {
	a, _ := newApp(t)
	asked := 0
	yes := func(summary, want string) error { asked++; return nil }
	no := func(summary, want string) error { return util.Cancelled("no") }

	if v, err := a.ConfigGet("mode"); err != nil || v != "auto" {
		t.Fatalf("default mode: %q %v", v, err)
	}
	if _, err := a.ConfigSet("mode", "strict", nil); err == nil {
		t.Fatal("mode changed without a confirmer")
	}
	if _, err := a.ConfigSet("mode", "strict", no); err == nil {
		t.Fatal("mode changed after a declined prompt")
	}
	if v, _ := a.ConfigGet("mode"); v != "auto" {
		t.Fatalf("mode changed anyway: %s", v)
	}
	if old, err := a.ConfigSet("mode", "strict", yes); err != nil || old != "auto" || asked != 1 {
		t.Fatalf("set mode: %q %v %d", old, err, asked)
	}
	if _, err := a.ConfigSet("threshold", "0", yes); err == nil {
		t.Fatal("threshold 0 accepted")
	}
	if _, err := a.ConfigSet("threshold", "8", yes); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.ConfigGet("threshold"); v != "8" {
		t.Fatalf("threshold %s", v)
	}
	// auto_review needs no human
	if _, err := a.ConfigSet("auto_review", "true", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ConfigSet("bogus", "1", yes); err == nil {
		t.Fatal("unknown key accepted")
	}
}

func TestConfigSetRefusedInAgent(t *testing.T) {
	a, _ := newApp(t)
	inAgent(a)
	called := false
	confirm := func(string, string) error { called = true; return nil }
	for _, kv := range [][2]string{{"mode", "strict"}, {"threshold", "1"}} {
		_, err := a.ConfigSet(kv[0], kv[1], confirm)
		var ue *util.Error
		if !errors.As(err, &ue) || ue.Code != util.ExitPolicy || !strings.Contains(err.Error(), "inside an agent") {
			t.Fatalf("%s: %v", kv[0], err)
		}
	}
	if called {
		t.Fatal("the prompt ran inside an agent")
	}
}
