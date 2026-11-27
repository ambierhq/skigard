package core

import (
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/store"
)

func ruleCount(rep *scan.Report, id string) int {
	n := 0
	for _, f := range rep.Findings {
		if f.Rule == id {
			n++
		}
	}
	return n
}

func TestAllowListSuppressesMediumRulesForOneSource(t *testing.T) {
	a, _ := newApp(t)
	src := t.TempDir()
	writeSkill(t, src, "alpha", map[string]string{"run.py": "import subprocess\nsubprocess.run(['ls'])\n"})
	writeSkill(t, src, "beta", map[string]string{"run.py": "import subprocess\nsubprocess.run(['ls'])\n"})
	qs := addLocal(t, a, src, AddOpts{All: true})
	if len(qs) != 2 {
		t.Fatalf("want 2 skills, got %d", len(qs))
	}
	before, _, err := a.Scan("alpha")
	if err != nil || ruleCount(before, "SKG-EXEC-004") == 0 {
		t.Fatalf("the sample must trigger SKG-EXEC-004: %v", err)
	}
	var alphaSource string
	for _, q := range qs {
		if q.Name == "alpha" {
			alphaSource = q.Meta.Source
		}
	}
	// the allowance matches alpha's recorded source, with or without a ref
	if err := a.AllowAdd(alphaSource+"@main", []string{"SKG-EXEC-004"}, "sample uses subprocess on purpose", yes); err != nil {
		t.Fatal(err)
	}
	after, _, _ := a.Scan("alpha")
	if ruleCount(after, "SKG-EXEC-004") != 0 || len(after.Suppressed) == 0 {
		t.Fatalf("not suppressed: %+v", after.Findings)
	}
	// both skills live in one source directory here, so the second one is covered too: the
	// allowance is per source, not per skill. A different source must stay loud.
	other := t.TempDir()
	writeSkill(t, other, "gamma", map[string]string{"run.py": "import subprocess\nsubprocess.run(['ls'])\n"})
	addLocal(t, a, other, AddOpts{Skill: "gamma"})
	g, _, _ := a.Scan("gamma")
	if ruleCount(g, "SKG-EXEC-004") == 0 {
		t.Fatal("an allowance for one source hid a finding from another")
	}
}

func TestAllowNeverHidesHighRulesAndNeedsAHuman(t *testing.T) {
	a, _ := newApp(t)
	if err := a.AllowAdd("github:o/r/p", []string{"SKG-EXEC-001"}, "x", yes); err == nil || !strings.Contains(err.Error(), "only medium and low") {
		t.Fatalf("high rule allowed: %v", err)
	}
	if err := a.AllowAdd("github:o/r/p", []string{"SKG-NOPE-001"}, "x", yes); err == nil {
		t.Fatal("unknown rule allowed")
	}
	if err := a.AllowAdd("github:o/r/p", []string{"SKG-EXEC-004"}, "", yes); err == nil {
		t.Fatal("allowed without a reason")
	}
	if err := a.AllowAdd("*", []string{"SKG-EXEC-004"}, "x", yes); err == nil {
		t.Fatal("bare wildcard source accepted")
	}
	if err := a.AllowAdd("github:o/r/p", []string{"SKG-EXEC-004"}, "x", nil); err == nil {
		t.Fatal("changed without a confirmation")
	}
	a.RefuseInAgent = true
	inAgent(a)
	if err := a.AllowAdd("github:o/r/p", []string{"SKG-EXEC-004"}, "x", yes); err == nil || !strings.Contains(err.Error(), "agent") {
		t.Fatalf("an agent changed the allow list: %v", err)
	}
	if es, _ := a.AllowList(); len(es) != 0 {
		t.Fatalf("nothing should be stored: %+v", es)
	}
}

func TestAllowRemoveAndPrefixSources(t *testing.T) {
	a, _ := newApp(t)
	if err := a.AllowAdd("github:o/r/*", []string{"SKG-EXEC-004", "SKG-NET-002"}, "docs", yes); err != nil {
		t.Fatal(err)
	}
	man, _ := store.LoadManifest(a.Home)
	if got := man.AllowedRules("github:o/r/skills/x"); len(got) != 2 {
		t.Fatalf("prefix did not match: %v", got)
	}
	if got := man.AllowedRules("github:o/other/x"); got != nil {
		t.Fatalf("prefix matched another repo: %v", got)
	}
	if err := a.AllowRemove("github:o/r/*", []string{"SKG-NET-002"}, yes); err != nil {
		t.Fatal(err)
	}
	if es, _ := a.AllowList(); len(es) != 1 || len(es[0].Rules) != 1 {
		t.Fatalf("%+v", es)
	}
	if err := a.AllowRemove("github:o/r/*", nil, yes); err != nil {
		t.Fatal(err)
	}
	if es, _ := a.AllowList(); len(es) != 0 {
		t.Fatalf("%+v", es)
	}
}
