package core

import (
	"os"
	"runtime"

	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// ProtectResult is the outcome for one skill of `lock protect`.
type ProtectResult struct {
	Name    string `json:"name"`
	Changed bool   `json:"changed"` // it was writable and is now read-only
	Note    string `json:"note,omitempty"`
}

// Protect makes approved skills read-only (see util.MakeReadOnly), for skills
// approved before that existed. It removes write bits inside skills/NAME and
// nothing else: no content, hash, lock entry or install changes, so it is safe
// to run in an agent. With no names it covers every locked skill. It is a no-op
// on Windows, where permission bits are synthetic.
func (a *App) Protect(names []string) ([]ProtectResult, error) {
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
	for _, n := range names {
		if err := util.CheckName(n); err != nil {
			return nil, err
		}
		if _, ok := lock.Get(n); !ok {
			return nil, util.Errf("%q is not approved", n)
		}
	}
	var out []ProtectResult
	for _, n := range names {
		r := ProtectResult{Name: n}
		dir := a.Home.SkillDir(n)
		fi, err := os.Lstat(dir)
		switch {
		case runtime.GOOS == "windows":
			r.Note = "not applicable on Windows"
		case err != nil || !fi.IsDir():
			r.Note = "skipped: the approved skill is missing from skills/"
		default:
			w, err := util.TreeWritable(dir)
			if err != nil {
				return out, err
			}
			if w {
				if err := util.MakeReadOnly(dir); err != nil {
					return out, err
				}
				r.Changed = true
			} else {
				r.Note = "already read-only"
			}
		}
		out = append(out, r)
	}
	return out, nil
}
