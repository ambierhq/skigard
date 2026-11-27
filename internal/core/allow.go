package core

import (
	"strings"

	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// allowFor returns the scanner rules a human allowed for the source of the
// skill called name (in quarantine or approved). Any error means "no allowances":
// failing open would hide findings, failing closed only shows more of them.
func (a *App) allowFor(name string) map[string]string {
	man, err := store.LoadManifest(a.Home)
	if err != nil || len(man.Allow) == 0 {
		return nil
	}
	src, ok := a.sourceOf(name)
	if !ok {
		return nil
	}
	return man.AllowedRules(src)
}

// sourceOf is the recorded source of a quarantined or approved skill.
func (a *App) sourceOf(name string) (string, bool) {
	if meta, err := store.LoadMeta(a.Home, name); err == nil && meta.Source != "" {
		return meta.Source, true
	}
	if lock, err := store.LoadLock(a.Home); err == nil {
		if ls, ok := lock.Get(name); ok && ls.Source != "" {
			return ls.Source, true
		}
	}
	return "", false
}

// AllowList returns the allow entries.
func (a *App) AllowList() ([]store.AllowRule, error) {
	man, err := store.LoadManifest(a.Home)
	if err != nil {
		return nil, err
	}
	return man.Allow, nil
}

// ResolveAllowSource turns a skill name into its recorded source; anything
// that already looks like a source (it has ":" or "/" or "*") is returned as is.
func (a *App) ResolveAllowSource(arg string) (string, error) {
	if strings.ContainsAny(arg, ":/*") {
		return arg, nil
	}
	if err := util.CheckName(arg); err != nil {
		return "", err
	}
	src, ok := a.sourceOf(arg)
	if !ok {
		return "", util.Errf("no skill named %q to take a source from; pass the source (github:owner/repo/path) instead", arg)
	}
	return src, nil
}

// allowGuard is the human-only gate shared by every change to the allow list.
func (a *App) allowGuard(summary string, confirm Confirmer) error {
	if v, ok := a.agentVar(); ok {
		return util.Policyf("refusing to change the allow list: this looks like it is running inside an agent harness (%s). Allowing scanner rules is a human step: run it in a normal terminal", v)
	}
	if confirm == nil {
		return util.Errf("changing the allow list needs an interactive confirmation")
	}
	return confirm(summary, "allow")
}

// AllowAdd lets a human stop the scanner reporting medium-or-lower rules for
// skills from source. It is refused inside an agent and needs a typed confirmation.
func (a *App) AllowAdd(source string, rules []string, reason string, confirm Confirmer) error {
	entry := store.AllowRule{Source: source, Rules: rules, Reason: strings.TrimSpace(reason), By: a.User, At: util.Now()}
	if err := entry.Validate(); err != nil {
		return util.Errf("%v", err)
	}
	for _, r := range rules {
		sev, ok := scan.RuleSeverity(r)
		if !ok {
			return util.Errf("unknown rule %q (see: skigard rules)", util.Clean(r))
		}
		if sev.Rank() > scan.Medium.Rank() {
			return util.Errf("%s is %s: only medium and low rules can be allowed", r, sev)
		}
	}
	unlock, err := a.Home.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	if err := a.Home.Init(); err != nil {
		return err
	}
	man, err := store.LoadManifest(a.Home)
	if err != nil {
		return err
	}
	summary := "Allow " + strings.Join(rules, ", ") + " for skills from " + util.Clean(source) +
		"\nreason: " + util.Clean(entry.Reason) +
		"\n(the scanner will stop reporting these rules for that source; high and critical findings are never hidden)"
	if err := a.allowGuard(summary, confirm); err != nil {
		return err
	}
	for i := range man.Allow {
		if man.Allow[i].Source == source {
			man.Allow[i].Rules = mergeRules(man.Allow[i].Rules, rules)
			man.Allow[i].Reason, man.Allow[i].By, man.Allow[i].At = entry.Reason, entry.By, entry.At
			return store.SaveManifest(a.Home, man)
		}
	}
	man.Allow = append(man.Allow, entry)
	return store.SaveManifest(a.Home, man)
}

// AllowRemove deletes the given rules for source, or the whole entry when rules is empty.
func (a *App) AllowRemove(source string, rules []string, confirm Confirmer) error {
	unlock, err := a.Home.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	man, err := store.LoadManifest(a.Home)
	if err != nil {
		return err
	}
	idx := -1
	for i, e := range man.Allow {
		if e.Source == source {
			idx = i
		}
	}
	if idx < 0 {
		return util.Errf("no allow entry for %q", util.Clean(source))
	}
	what := "every allowed rule"
	if len(rules) > 0 {
		what = strings.Join(rules, ", ")
	}
	if err := a.allowGuard("Stop allowing "+what+" for skills from "+util.Clean(source), confirm); err != nil {
		return err
	}
	if len(rules) == 0 {
		man.Allow = append(man.Allow[:idx], man.Allow[idx+1:]...)
	} else {
		var keep []string
		for _, r := range man.Allow[idx].Rules {
			drop := false
			for _, d := range rules {
				drop = drop || d == r
			}
			if !drop {
				keep = append(keep, r)
			}
		}
		if len(keep) == 0 {
			man.Allow = append(man.Allow[:idx], man.Allow[idx+1:]...)
		} else {
			man.Allow[idx].Rules = keep
		}
	}
	return store.SaveManifest(a.Home, man)
}

func mergeRules(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range append(append([]string{}, a...), b...) {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}
