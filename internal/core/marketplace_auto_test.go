package core

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// marketplaceApp is an app with settings.marketplace on, a warning buffer and one approved skill in plugin "dev".
func marketplaceApp(t *testing.T) (*App, *bytes.Buffer) {
	t.Helper()
	a, _ := newApp(t)
	var warn bytes.Buffer
	a.Err = &warn
	src := t.TempDir()
	writeSkill(t, src, "alpha", nil)
	approveLocal(t, a, src, "alpha")
	if err := a.PluginCreate("dev", "dev"); err != nil {
		t.Fatal(err)
	}
	if err := a.PluginAdd("dev", []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ConfigSet("marketplace", "true", nil); err != nil {
		t.Fatal(err)
	}
	return a, &warn
}

func mustBeInSync(t *testing.T, a *App, when string) {
	t.Helper()
	if ps, _ := a.Verify(); HasErrors(ps) {
		t.Fatalf("%s: %+v", when, ps)
	}
}

func TestVerifyMarketplaceSetting(t *testing.T) {
	a, _ := newApp(t)
	src := t.TempDir()
	writeSkill(t, src, "alpha", nil)
	approveLocal(t, a, src, "alpha")
	mustBeInSync(t, a, "disabled and missing")
	on := true
	man, _ := store.LoadManifest(a.Home)
	man.Settings.Marketplace = &on
	if err := store.SaveManifest(a.Home, man); err != nil {
		t.Fatal(err)
	}
	ps, _ := a.Verify()
	found := false
	for _, p := range ps {
		if p.Level == "error" && strings.Contains(p.Msg, "marketplace is enabled in settings but marketplace/ is missing; run: skigard marketplace build") {
			found = true
		}
	}
	if !found {
		t.Fatalf("enabled and missing not reported: %+v", ps)
	}
	if err := a.MarketplaceBuild(); err != nil {
		t.Fatal(err)
	}
	mustBeInSync(t, a, "enabled and built")
	os.WriteFile(filepath.Join(a.Home.MarketplaceDir(), "junk"), []byte("x"), 0o644)
	if ps, _ := a.Verify(); !HasErrors(ps) {
		t.Fatal("enabled marketplace with drift must be checked")
	}
}

func TestMarketplaceAutoRebuild(t *testing.T) {
	a, warn := marketplaceApp(t)
	mustBeInSync(t, a, "after set true")

	// Approve a new skill; add it to the plugin; remove it again. Each leaves marketplace/ in sync.
	src := t.TempDir()
	writeSkill(t, src, "beta", nil)
	addLocal(t, a, src, AddOpts{Skill: "beta"})
	if _, err := a.Approve("beta", ApproveOpts{Confirm: yes}); err != nil {
		t.Fatal(err)
	}
	mustBeInSync(t, a, "after approve")
	if err := a.PluginAdd("dev", []string{"beta"}); err != nil {
		t.Fatal(err)
	}
	mustBeInSync(t, a, "after plugin add")
	if _, err := os.Stat(filepath.Join(a.Home.MarketplaceDir(), "plugins", "dev", "skills", "beta", "SKILL.md")); err != nil {
		t.Fatalf("plugin add not rendered: %v", err)
	}
	if err := a.PluginRemove("dev", []string{"beta"}); err != nil {
		t.Fatal(err)
	}
	mustBeInSync(t, a, "after plugin remove")
	if _, err := os.Stat(filepath.Join(a.Home.MarketplaceDir(), "plugins", "dev", "skills", "beta")); err == nil {
		t.Fatal("plugin remove not rendered")
	}
	if err := a.PluginCreate("ops", "ops"); err != nil {
		t.Fatal(err)
	}
	if err := a.PluginAdd("ops", []string{"beta"}); err != nil {
		t.Fatal(err)
	}
	if err := a.PluginDelete("ops"); err != nil {
		t.Fatal(err)
	}
	mustBeInSync(t, a, "after plugin delete")
	if warn.Len() != 0 {
		t.Fatalf("unexpected warnings: %s", warn.String())
	}
}

func TestMarketplaceNoAutoRebuildWhenDisabled(t *testing.T) {
	a, _ := newApp(t)
	src := t.TempDir()
	writeSkill(t, src, "alpha", nil)
	approveLocal(t, a, src, "alpha")
	if err := a.PluginCreate("dev", ""); err != nil {
		t.Fatal(err)
	}
	if err := a.PluginAdd("dev", []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.Home.MarketplaceDir()); err == nil {
		t.Fatal("marketplace built without the setting")
	}
}

func TestMarketplaceRebuildFailureIsAWarning(t *testing.T) {
	a, warn := marketplaceApp(t)
	// Tamper with the approved skill the plugin renders, so the rebuild refuses.
	util.MakeWritable(a.Home.SkillDir("alpha"))
	if err := os.WriteFile(filepath.Join(a.Home.SkillDir("alpha"), "extra.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	writeSkill(t, src, "beta", nil)
	addLocal(t, a, src, AddOpts{Skill: "beta"})
	if _, err := a.Approve("beta", ApproveOpts{Confirm: yes}); err != nil {
		t.Fatalf("approval must survive a rebuild failure: %v", err)
	}
	lock, _ := store.LoadLock(a.Home)
	if _, ok := lock.Get("beta"); !ok {
		t.Fatal("approval was undone")
	}
	if _, err := os.Stat(a.Home.SkillDir("beta")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn.String(), "warning: could not rebuild marketplace/") {
		t.Fatalf("no warning: %q", warn.String())
	}
	if ps, _ := a.Verify(); !HasErrors(ps) {
		t.Fatal("verify must flag it")
	}
}
