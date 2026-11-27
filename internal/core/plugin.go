package core

import (
	"fmt"
	"sort"

	"github.com/ambierhq/skigard/internal/marketplace"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// PluginCreate defines an empty plugin.
func (a *App) PluginCreate(name, desc string) error {
	unlock, err := a.Home.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	if err := util.CheckName(name); err != nil {
		return err
	}
	if err := a.Home.Init(); err != nil {
		return err
	}
	m, err := store.LoadManifest(a.Home)
	if err != nil {
		return err
	}
	if _, ok := m.Plugins[name]; ok {
		return util.Errf("plugin %q already exists", name)
	}
	if m.Plugins == nil {
		m.Plugins = map[string]store.Plugin{}
	}
	m.Plugins[name] = store.Plugin{Description: desc, Skills: []string{}}
	if err := store.SaveManifest(a.Home, m); err != nil {
		return err
	}
	a.syncMarketplace(m)
	return nil
}

// PluginDelete removes a plugin definition.
func (a *App) PluginDelete(name string) error {
	unlock, err := a.Home.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	m, err := store.LoadManifest(a.Home)
	if err != nil {
		return err
	}
	if _, ok := m.Plugins[name]; !ok {
		return util.Errf("no plugin %q", name)
	}
	delete(m.Plugins, name)
	if err := store.SaveManifest(a.Home, m); err != nil {
		return err
	}
	a.syncMarketplace(m)
	return nil
}

// PluginAdd adds approved skills to a plugin.
func (a *App) PluginAdd(name string, skills []string) error {
	unlock, err := a.Home.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	m, err := store.LoadManifest(a.Home)
	if err != nil {
		return err
	}
	p, ok := m.Plugins[name]
	if !ok {
		return util.Errf("no plugin %q", name)
	}
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return err
	}
	set := map[string]bool{}
	for _, s := range p.Skills {
		set[s] = true
	}
	for _, s := range skills {
		if _, ok := lock.Get(s); !ok {
			return util.Errf("%q is not approved", s)
		}
		set[s] = true
	}
	p.Skills = keys(set)
	m.Plugins[name] = p
	if err := store.SaveManifest(a.Home, m); err != nil {
		return err
	}
	a.syncMarketplace(m)
	return nil
}

// PluginRemove removes skills from a plugin.
func (a *App) PluginRemove(name string, skills []string) error {
	unlock, err := a.Home.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	m, err := store.LoadManifest(a.Home)
	if err != nil {
		return err
	}
	p, ok := m.Plugins[name]
	if !ok {
		return util.Errf("no plugin %q", name)
	}
	set := map[string]bool{}
	for _, s := range p.Skills {
		set[s] = true
	}
	for _, s := range skills {
		if !set[s] {
			return util.Errf("%q is not in plugin %q", s, name)
		}
		delete(set, s)
	}
	p.Skills = keys(set)
	m.Plugins[name] = p
	if err := store.SaveManifest(a.Home, m); err != nil {
		return err
	}
	a.syncMarketplace(m)
	return nil
}

func keys(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// MarketplaceBuild regenerates the marketplace directory.
func (a *App) MarketplaceBuild() error {
	unlock, err := a.Home.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	return a.marketplaceBuild()
}

func (a *App) marketplaceBuild() error { return marketplace.Build(a.Home, a.User) }

// syncMarketplace rebuilds marketplace/ when settings.marketplace is on. The caller holds the
// home lock and has already saved what changed (an approval, a plugin edit). A failed rebuild
// never undoes that: it is reported as a warning on a.Err, and verify flags the drift.
func (a *App) syncMarketplace(man *store.Manifest) {
	var err error
	if man == nil {
		if man, err = store.LoadManifest(a.Home); err != nil {
			a.warnMarketplace(err)
			return
		}
	}
	if !man.Settings.EffectiveMarketplace() {
		return
	}
	if err := a.marketplaceBuild(); err != nil {
		a.warnMarketplace(err)
	}
}

func (a *App) warnMarketplace(err error) {
	if a.Err != nil {
		fmt.Fprintf(a.Err, "warning: could not rebuild marketplace/: %s; the change itself succeeded. Run: skigard marketplace build (verify reports the drift)\n", util.Clean(err.Error()))
	}
}

// MarketplaceCheck reports drift.
func (a *App) MarketplaceCheck() ([]string, error) { return marketplace.Check(a.Home, a.User) }
