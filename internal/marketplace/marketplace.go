// Package marketplace renders approved skills and plugins as a Claude Code /
// Oh My Pi compatible plugin marketplace.
package marketplace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/safefs"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// Render writes the marketplace tree into dest.
func Render(h store.Home, dest, defaultOwner string) error {
	man, err := store.LoadManifest(h)
	if err != nil {
		return err
	}
	lock, err := store.LoadLock(h)
	if err != nil {
		return err
	}
	name := man.Settings.MarketplaceName
	if name == "" {
		name = "skigard"
	}
	owner := man.Settings.Owner
	if owner == "" {
		owner = defaultOwner
	}
	desc := man.Settings.Description
	if desc == "" {
		desc = "Reviewed skills managed by skigard"
	}
	var pnames []string
	for p := range man.Plugins {
		pnames = append(pnames, p)
	}
	sort.Strings(pnames)
	entries := []map[string]any{}
	for _, pname := range pnames {
		p := man.Plugins[pname]
		if len(p.Skills) == 0 {
			continue
		}
		pdir := filepath.Join(dest, "plugins", pname)
		digest := sha256.New()
		skills := append([]string(nil), p.Skills...)
		sort.Strings(skills)
		for _, s := range skills {
			ls, ok := lock.Get(s)
			if !ok {
				return util.Errf("plugin %q lists %q, which is not approved", pname, s)
			}
			cur, err := hash.Dir(h.SkillDir(s))
			if err != nil || cur != ls.ContentHash {
				return util.Errf("skill %q does not match its approved hash; run: skigard verify", s)
			}
			if _, err := safefs.CopyTree(h.SkillDir(s), filepath.Join(pdir, "skills", s), safefs.DefaultLimits()); err != nil {
				return err
			}
			digest.Write([]byte(ls.ContentHash))
		}
		version := "1.0.0+" + hex.EncodeToString(digest.Sum(nil))[:8]
		desc := p.Description
		if desc == "" {
			desc = pname
		}
		pj, _ := json.MarshalIndent(map[string]string{"name": pname, "description": desc, "version": version}, "", "  ")
		if err := util.AtomicWrite(filepath.Join(pdir, ".claude-plugin", "plugin.json"), append(pj, '\n'), 0o644); err != nil {
			return err
		}
		entries = append(entries, map[string]any{"name": pname, "description": desc, "version": version, "source": "./plugins/" + pname})
	}
	catalog := map[string]any{
		"name":     name,
		"owner":    map[string]string{"name": owner},
		"metadata": map[string]string{"description": desc},
		"plugins":  entries,
	}
	cj, _ := json.MarshalIndent(catalog, "", "  ")
	return util.AtomicWrite(filepath.Join(dest, ".claude-plugin", "marketplace.json"), append(cj, '\n'), 0o644)
}

// Build regenerates home/marketplace.
func Build(h store.Home, defaultOwner string) error {
	tmp, err := os.MkdirTemp(h.Root, ".marketplace-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	out := filepath.Join(tmp, "m")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if err := Render(h, out, defaultOwner); err != nil {
		return err
	}
	old := h.MarketplaceDir() + ".old"
	os.RemoveAll(old)
	if _, err := os.Stat(h.MarketplaceDir()); err == nil {
		if err := os.Rename(h.MarketplaceDir(), old); err != nil {
			return err
		}
	}
	if err := os.Rename(out, h.MarketplaceDir()); err != nil {
		return err
	}
	return os.RemoveAll(old)
}

// Check returns drift problems between the lock/manifest and home/marketplace.
func Check(h store.Home, defaultOwner string) ([]string, error) {
	tmp, err := os.MkdirTemp("", "skigard-mkt-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := Render(h, tmp, defaultOwner); err != nil {
		return nil, err
	}
	want, err := hash.Dir(tmp)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(h.MarketplaceDir()); err != nil {
		man, _ := store.LoadManifest(h)
		used := false
		for _, p := range man.Plugins {
			if len(p.Skills) > 0 {
				used = true
			}
		}
		if !used {
			return nil, nil
		}
		return []string{"marketplace/ is missing; run: skigard marketplace build"}, nil
	}
	got, err := hash.Dir(h.MarketplaceDir())
	if err != nil {
		return []string{"marketplace/ is unreadable: " + err.Error()}, nil
	}
	if got != want {
		return []string{"marketplace/ is out of date; run: skigard marketplace build"}, nil
	}
	return nil, nil
}
