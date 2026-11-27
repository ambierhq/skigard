package core

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/ambierhq/skigard/internal/harness"
	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/marketplace"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// InstallOpts select what is installed where.
type InstallOpts struct {
	Names             []string
	Plugin            string
	To                []string // harness ids; empty = configured/detected
	IncludeUnverified bool
	Scope             string // user | project
	Project           string // project root for scope=project
	Copy              bool
	Force             bool
}

// Installed is one placed skill.
type Installed struct {
	Name      string
	Path      string
	Mode      string
	Harnesses []string
}

// Targets resolves where to install.
func (a *App) Targets(o InstallOpts) ([]harness.Target, []string, error) {
	reg, man, err := a.Registry()
	if err != nil {
		return nil, nil, err
	}
	var picked []harness.Detection
	var notes []string
	all := harness.Detect(reg, a.Env)
	byID := map[string]harness.Detection{}
	for _, d := range all {
		byID[d.ID] = d
	}
	ids := o.To
	if len(ids) == 0 {
		ids = man.Settings.InstallTo
	}
	if len(ids) > 0 {
		for _, id := range ids {
			d, ok := byID[id]
			if !ok {
				return nil, nil, util.Errf("unknown harness %q (see: skigard harness list)", id)
			}
			if !d.Verified && !o.IncludeUnverified {
				return nil, nil, util.Errf("harness %q is not verified yet; pass --include-unverified if you want to install there anyway", id)
			}
			picked = append(picked, d)
		}
	} else {
		var skipped []string
		for _, d := range all {
			if d.Status == harness.Absent {
				continue
			}
			if !d.Verified && !o.IncludeUnverified {
				skipped = append(skipped, d.ID)
				continue
			}
			picked = append(picked, d)
		}
		if len(skipped) > 0 {
			notes = append(notes, fmt.Sprintf("detected but unverified (skipped; use --include-unverified): %s", strings.Join(skipped, ", ")))
		}
	}
	scope := o.Scope
	if scope == "" {
		scope = "user"
	}
	if scope != "user" && scope != "project" {
		return nil, nil, util.Errf("--scope must be user or project")
	}
	proj := o.Project
	if scope == "project" {
		if proj == "" {
			proj, _ = os.Getwd()
		}
		if proj, err = filepath.Abs(proj); err != nil {
			return nil, nil, err
		}
	}
	ts := harness.Targets(picked, scope, proj, a.Env)
	if len(ts) == 0 {
		return nil, notes, util.Errf("no target harness found. Run `skigard detect`, or choose with --to")
	}
	return ts, notes, nil
}

// Install places approved skills into harness directories.
func (a *App) Install(o InstallOpts) ([]Installed, []string, error) {
	unlock, err := a.Home.Lock()
	if err != nil {
		return nil, nil, err
	}
	defer unlock()
	return a.install(o)
}

// install is Install without taking the home lock; the caller holds it.
func (a *App) install(o InstallOpts) ([]Installed, []string, error) {
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, nil, err
	}
	names := append([]string(nil), o.Names...)
	if o.Plugin != "" {
		man, err := store.LoadManifest(a.Home)
		if err != nil {
			return nil, nil, err
		}
		p, ok := man.Plugins[o.Plugin]
		if !ok {
			return nil, nil, util.Errf("no plugin %q", o.Plugin)
		}
		names = append(names, p.Skills...)
	}
	if len(names) == 0 {
		return nil, nil, util.Errf("name at least one skill (or --plugin P, or use `skigard sync`)")
	}
	ts, notes, err := a.Targets(o)
	if err != nil {
		return nil, notes, err
	}
	scope := o.Scope
	if scope == "" {
		scope = "user"
	}
	mode := harness.ModeSymlink
	// Windows installs by copy: creating symlinks there needs a privilege most users do not have.
	if o.Copy || scope == "project" || runtime.GOOS == "windows" {
		mode = harness.ModeCopy
	}
	st, err := harness.LoadState(a.Home.InstallsPath())
	if err != nil {
		return nil, notes, err
	}
	var out []Installed
	for _, n := range names {
		ls, ok := lock.Get(n)
		if !ok {
			return out, notes, util.Errf("%q is not approved (see: skigard list)", n)
		}
		dir := a.Home.SkillDir(n)
		if st, err := hash.Check(dir, ls.Identity()); err != nil || !st.OK() {
			return out, notes, util.Policyf("%q does not match its approved hash; refusing to install (run: skigard verify; an intentional in-place edit goes through `skigard adopt %s`)", n, n)
		}
		for _, t := range ts {
			rec, err := harness.InstallSkill(t.Dir, n, dir, mode, ls.Identity(), o.Force)
			if err != nil {
				return out, notes, err
			}
			rec.Scope, rec.Harnesses = scope, t.Harnesses
			st.Upsert(*rec)
			if err := st.Save(a.Home.InstallsPath()); err != nil {
				return out, notes, err
			}
			out = append(out, Installed{Name: n, Path: rec.Path, Mode: mode, Harnesses: t.Harnesses})
		}
	}
	return out, notes, nil
}

// Uninstall removes installs recorded by skigard. With no names, removes all of them.
func (a *App) Uninstall(names []string, force bool) ([]string, error) {
	unlock, err := a.Home.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	st, err := harness.LoadState(a.Home.InstallsPath())
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var removed []string
	for _, r := range append([]harness.Record(nil), st.Installs...) {
		if len(want) > 0 && !want[r.Name] {
			continue
		}
		if err := harness.Remove(r, force); err != nil {
			return removed, err
		}
		st.Drop(r.Path)
		removed = append(removed, r.Path)
	}
	if len(want) > 0 && len(removed) == 0 {
		return nil, util.Errf("none of those skills are installed by skigard")
	}
	return removed, st.Save(a.Home.InstallsPath())
}

// Sync installs every approved skill to the default targets.
func (a *App) Sync(o InstallOpts) ([]Installed, []string, error) {
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, nil, err
	}
	for _, s := range lock.Skills {
		o.Names = append(o.Names, s.Name)
	}
	if len(o.Names) == 0 {
		return nil, nil, util.Errf("nothing is approved yet")
	}
	return a.Install(o)
}

// Problem is one verification finding.
type Problem struct {
	Level string `json:"level"` // error | warn | info
	Msg   string `json:"message"`
}

// Verify checks the lock against what is on disk, what is installed and the marketplace.
func (a *App) Verify() ([]Problem, error) {
	var ps []Problem
	add := func(level, format string, args ...any) { ps = append(ps, Problem{level, fmt.Sprintf(format, args...)}) }
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, err
	}
	man, err := store.LoadManifest(a.Home)
	if err != nil {
		return nil, err
	}
	locked := map[string]*store.LockSkill{}
	for i := range lock.Skills {
		s := &lock.Skills[i]
		locked[s.Name] = s
		st, err := hash.Check(a.Home.SkillDir(s.Name), s.Identity())
		switch {
		case os.IsNotExist(err) || (err != nil && !isDir(a.Home.SkillDir(s.Name))):
			add("error", "%s: approved skill is missing from skills/", s.Name)
		case err != nil:
			add("error", "%s: cannot hash: %v", s.Name, err)
		case !st.ContentOK:
			add("error", "%s: content changed since approval (%s, approved %s); skills/ is read-only and must not be edited in place. If this is an intentional edit already made, run `skigard adopt %s`; to make future edits safely use `skigard edit %s`", s.Name, hash.Short(st.Got, 12), hash.Short(s.ContentHash, 12), s.Name, s.Name)
		case !st.ExecOK:
			add("error", "%s: executable files changed since approval (now %s, approved %s); skills/ is read-only and must not be edited in place. If this is an intentional edit already made, run `skigard adopt %s`; to make future edits safely use `skigard edit %s`", s.Name, execList(st.ExecGot), execList(s.ExecSet), s.Name, s.Name)
		}
		if err == nil && st.ExecNA && s.HashVersion >= 2 {
			add("info", "%s: exec-bit check not applicable on this OS (%d executable file(s) recorded at approval)", s.Name, len(s.ExecSet))
		}
		if runtime.GOOS != "windows" && isDir(a.Home.SkillDir(s.Name)) {
			if w, werr := util.TreeWritable(a.Home.SkillDir(s.Name)); werr == nil && w {
				add("warn", "%s: skills/%s is writable; edit it with `skigard edit %s`, and run `skigard lock protect` to make approved skills read-only", s.Name, s.Name, s.Name)
			}
		}
		if s.Approval.Override {
			add("warn", "%s: approved with an override (%s)", s.Name, util.Clean(s.Approval.Reason))
		}
		if s.Review == nil {
			add("info", "%s: no agent review recorded", s.Name)
		} else if s.Review.Verdict != "pass" {
			add("warn", "%s: review verdict is %s", s.Name, s.Review.Verdict)
		}
	}
	if entries, err := os.ReadDir(a.Home.SkillsDir()); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if _, ok := locked[e.Name()]; !ok {
				add("error", "%s: present in skills/ but not approved in skigard.lock", e.Name())
			}
		}
	}
	for pn, p := range man.Plugins {
		for _, s := range p.Skills {
			if _, ok := locked[s]; !ok {
				add("error", "plugin %s lists %s, which is not approved", pn, s)
			}
		}
	}
	st, err := harness.LoadState(a.Home.InstallsPath())
	if err != nil {
		return nil, err
	}
	for _, r := range st.Installs {
		ls, ok := locked[r.Name]
		if !ok {
			add("error", "installed but no longer approved: %s", r.Path)
			continue
		}
		for _, p := range harness.Check(r, ls.Identity()) {
			add("error", "%s", util.Clean(p))
		}
		if r.Hash != ls.ContentHash {
			add("warn", "%s was installed from an older approved version; run `skigard sync`", r.Path)
		}
	}
	// Windows reports synthetic permission bits (0o777 for directories), so only check on Unix.
	if fi, err := os.Stat(a.Home.QuarantineDir()); err == nil && runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
		add("warn", "quarantine/ is accessible to other users (mode %o)", fi.Mode().Perm())
	}
	// The marketplace is opt-in: it is checked only once `marketplace build` has created it.
	// Installs never read it (they use the lock and approved store), so its absence hides nothing.
	// With settings.marketplace on it is required instead.
	_, serr := os.Stat(a.Home.MarketplaceDir())
	if serr != nil && man.Settings.EffectiveMarketplace() {
		add("error", "marketplace is enabled in settings but marketplace/ is missing; run: skigard marketplace build")
	}
	if serr == nil && (len(man.Plugins) > 0 || man.Settings.EffectiveMarketplace()) {
		drift, err := marketplace.Check(a.Home, a.User)
		if err != nil {
			add("error", "marketplace: %v", err)
		}
		for _, d := range drift {
			add("error", "%s", d)
		}
	}
	sort.SliceStable(ps, func(i, j int) bool { return levelRank(ps[i].Level) < levelRank(ps[j].Level) })
	return ps, nil
}

func levelRank(l string) int {
	switch l {
	case "error":
		return 0
	case "warn":
		return 1
	}
	return 2
}

// HasErrors reports whether any problem is an error.
func HasErrors(ps []Problem) bool {
	for _, p := range ps {
		if p.Level == "error" {
			return true
		}
	}
	return false
}

func execList(set []string) string {
	if len(set) == 0 {
		return "none"
	}
	out := make([]string, len(set))
	for i, p := range set {
		out[i] = util.Clean(p)
	}
	return strings.Join(out, ", ")
}
