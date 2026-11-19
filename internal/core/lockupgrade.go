package core

import (
	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// UpgradeResult is the outcome for one lock entry.
type UpgradeResult struct {
	Name     string   `json:"name"`
	Upgraded bool     `json:"upgraded"`
	Note     string   `json:"note"`
	OldHash  string   `json:"old_hash,omitempty"`
	NewHash  string   `json:"new_hash,omitempty"`
	Exec     []string `json:"exec_set,omitempty"`
}

// LockUpgrade re-derives hash version 2 for approved skills that are still on
// version 1. It changes an entry only when the version 1 hash still matches the
// skill on disk, so it can never make something trusted that was not already
// approved: the same bytes, with the same execute bits (version 1 covered them),
// that the human approved. The approval, scan and review records are untouched;
// the old hash is kept in upgraded_from. With no names it considers every entry.
func (a *App) LockUpgrade(names []string) ([]UpgradeResult, error) {
	unlock, err := a.Home.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		for _, s := range lock.Skills {
			names = append(names, s.Name)
		}
	}
	var out []UpgradeResult
	changed := false
	for _, n := range names {
		if err := util.CheckName(n); err != nil {
			return nil, err
		}
		ls, ok := lock.Get(n)
		if !ok {
			return nil, util.Errf("%q is not approved", n)
		}
		r := UpgradeResult{Name: n, OldHash: ls.ContentHash}
		if ls.HashVersion >= 2 {
			r.Note = "already hash version 2"
			out = append(out, r)
			continue
		}
		dir := a.Home.SkillDir(n)
		st, err := hash.Check(dir, ls.Identity())
		switch {
		case err != nil && !isDir(dir):
			r.Note = "not upgraded: the approved skill is missing from skills/"
		case err != nil:
			r.Note = "not upgraded: cannot hash: " + err.Error()
		case !st.ContentOK:
			r.Note = "not upgraded: the skill on disk does not match its approved version 1 hash (" + hash.Short(st.Got, 12) + ", approved " + hash.Short(ls.ContentHash, 12) + "); nothing was trusted. Run `skigard verify`"
			if !hash.ExecApplicable() {
				r.Note += ". Version 1 hashes include the execute bit, which Windows cannot represent, so a skill with executable scripts can only be upgraded on Linux or macOS"
			}
		default:
			// The version 1 hash matched, so the bytes and the execute bits are exactly the approved ones.
			// On Windows a match proves there were no execute bits (v1 always saw "-" there).
			v2, err := hash.Dir(dir)
			if err != nil {
				return out, err
			}
			exec, err := hash.ExecSet(dir)
			if err != nil {
				return out, err
			}
			ls.UpgradedFrom = ls.ContentHash
			ls.SetIdentity(hash.Identity{Version: hash.Version, Content: v2, Exec: exec, ExecHash: hash.ExecDigest(exec)})
			r.Upgraded, r.NewHash, r.Exec, changed = true, v2, exec, true
			r.Note = "upgraded to hash version 2 (version 1 hash verified first; approval unchanged)"
		}
		out = append(out, r)
	}
	if changed {
		if err := lock.Save(a.Home); err != nil {
			return out, err
		}
	}
	return out, nil
}
