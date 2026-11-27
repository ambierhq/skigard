package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ambierhq/skigard/internal/harness"
	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/skill"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// Entry is one row of `list`.
type Entry struct {
	Name     string `json:"name"`
	State    string `json:"state"` // approved | quarantined
	Source   string `json:"source"`
	Commit   string `json:"commit,omitempty"`
	Hash     string `json:"content_hash"`
	Scan     string `json:"scan,omitempty"`
	Review   string `json:"review,omitempty"`
	Override bool   `json:"override,omitempty"`
	// Mode (human, auto or forced), Score and Threshold come from the approval record.
	Mode      string `json:"mode,omitempty"`
	Score     int    `json:"score,omitempty"`
	Threshold int    `json:"threshold,omitempty"`
	Drift     bool   `json:"drift,omitempty"`
	// Merge is set for a quarantined merge result: "pick" or "agent:<harness>".
	Merge    string   `json:"merge,omitempty"`
	Installs []string `json:"installs,omitempty"`
}

// List returns approved and quarantined skills.
func (a *App) List() ([]Entry, error) {
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, err
	}
	st, err := harness.LoadState(a.Home.InstallsPath())
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, s := range lock.Skills {
		e := Entry{Name: s.Name, State: "approved", Source: s.Source, Commit: s.Commit, Hash: s.ContentHash, Scan: s.Scan.Verdict, Override: s.Approval.Override,
			Mode: s.Approval.EffectiveMode(), Score: s.Approval.Score, Threshold: s.Approval.Threshold}
		if s.Review != nil {
			e.Review = s.Review.Verdict
		}
		if st, err := hash.Check(a.Home.SkillDir(s.Name), s.Identity()); err != nil || !st.OK() {
			e.Drift = true
		}
		for _, r := range st.Installs {
			if r.Name == s.Name {
				e.Installs = append(e.Installs, r.Path)
			}
		}
		out = append(out, e)
	}
	for _, n := range store.QuarantinedNames(a.Home) {
		m, err := store.LoadMeta(a.Home, n)
		if err != nil {
			continue
		}
		state := "quarantined"
		if m.Editing {
			state = "editing"
		}
		e := Entry{Name: n, State: state, Source: m.Source, Commit: m.Commit, Hash: m.ContentHash}
		if rec, _ := ReviewInfo(m); rec != nil {
			e.Review = rec.Verdict
		}
		if m.MergeMode != "" {
			e.Merge = m.MergeMode
			if m.MergedBy != "" {
				e.Merge += ":" + m.MergedBy
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// Show returns SKILL.md and the file list of a skill.
func (a *App) Show(name string) (string, error) {
	d, err := a.ShowDoc(name)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s: %s\nfiles: %s\n\n%s", d.Name, d.State, strings.Join(d.Files, ", "), d.Body), nil
}

// SkillDoc is a skill's SKILL.md and file list, with all text already cleaned for a terminal.
type SkillDoc struct {
	Name, State string
	Files       []string
	Body        string
}

// ShowDoc reads a skill (quarantine first, then approved) for display.
func (a *App) ShowDoc(name string) (*SkillDoc, error) {
	if err := util.CheckName(name); err != nil {
		return nil, err
	}
	dir := a.Home.QuarantineSkill(name)
	state := "quarantined (UNTRUSTED)"
	if m, err := store.LoadMeta(a.Home, name); err == nil && isDir(dir) {
		if m.Editing {
			state = "editing (working copy, UNTRUSTED; `skigard adopt " + name + "` finalizes it)"
		}
		if mergeNote(m) != "" {
			state += ", " + mergeNote(m)
		}
	}
	if !isDir(dir) {
		dir = a.Home.SkillDir(name)
		state = "approved"
		if !isDir(dir) {
			return nil, util.Errf("no skill named %q", name)
		}
		if lock, err := store.LoadLock(a.Home); err == nil {
			if ls, ok := lock.Get(name); ok {
				state += ", approved by " + ls.Approval.EffectiveMode()
				if ls.Approval.Score > 0 {
					state += fmt.Sprintf(", score %d/10 (threshold %d)", ls.Approval.Score, ls.Approval.Threshold)
				}
			}
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil, err
	}
	var files []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	for i := range files {
		files[i] = util.Clean(files[i])
	}
	return &SkillDoc{Name: name, State: state, Files: files, Body: util.Clean(string(data))}, nil
}

// TokenRow is one skill's context cost.
type TokenRow struct {
	Name     string `json:"name"`
	Resident int    `json:"resident_tokens"`
	Body     int    `json:"body_tokens"`
}

// Tokens estimates how much prompt each approved skill costs (about 4 chars per token).
// "Resident" is what sits in every prompt (name + description); "body" loads only when the skill fires.
func (a *App) Tokens() ([]TokenRow, error) {
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, err
	}
	var rows []TokenRow
	for _, s := range lock.Skills {
		m, probs := skill.Read(a.Home.SkillDir(s.Name))
		if m == nil || len(probs) > 0 && m.Name == "" {
			continue
		}
		rows = append(rows, TokenRow{Name: s.Name, Resident: (len(m.Name)+len(m.Description)+40)/4 + 1, Body: len(m.Body)/4 + 1})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Resident > rows[j].Resident })
	return rows, nil
}

// Check is one doctor result.
type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pass | warn | fail
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

// Doctor checks the environment.
func (a *App) Doctor() []Check {
	var cs []Check
	add := func(name, status, msg, fix string) { cs = append(cs, Check{name, status, msg, fix}) }
	if a.Home.Exists() {
		add("home", "pass", a.Home.Root, "")
	} else {
		add("home", "warn", a.Home.Root+" is not initialised", "skigard init")
	}
	if _, err := store.LoadManifest(a.Home); err != nil {
		add("manifest", "fail", err.Error(), "fix or delete skigard.toml")
	} else {
		add("manifest", "pass", "skigard.toml parses", "")
	}
	if _, err := store.LoadLock(a.Home); err != nil {
		add("lock", "fail", err.Error(), "restore skigard.lock from git")
	} else {
		add("lock", "pass", "skigard.lock parses", "")
	}
	if ps, err := a.Verify(); err != nil {
		add("verify", "fail", err.Error(), "")
	} else if HasErrors(ps) {
		n := 0
		for _, p := range ps {
			if p.Level == "error" {
				n++
			}
		}
		add("verify", "fail", fmt.Sprintf("%d problem(s)", n), "skigard verify")
	} else {
		add("verify", "pass", "lock, store and installs agree", "")
	}
	if p, err := exec.LookPath("git"); err == nil {
		add("git", "pass", p, "")
	} else {
		add("git", "warn", "git not found (only needed if you version your home)", "install git")
	}
	ds, err := a.Detect()
	if err != nil {
		add("harnesses", "fail", err.Error(), "")
		return cs
	}
	var inst, conf int
	for _, d := range ds {
		switch d.Status {
		case harness.Installed:
			inst++
		case harness.Configured:
			conf++
		}
	}
	if inst+conf == 0 {
		add("harnesses", "warn", "no agent harness detected", "install one, or define it in skigard.toml")
	} else {
		add("harnesses", "pass", fmt.Sprintf("%d installed, %d configured", inst, conf), "")
	}
	rv, _ := a.Reviewers()
	usable := 0
	for _, r := range rv {
		if r.Installed && r.Verified {
			usable++
		}
	}
	if usable == 0 {
		add("reviewer", "warn", "no verified headless reviewer found (claude-code, pi, omp)", "install one, or use `skigard review NAME --manual`")
	} else {
		add("reviewer", "pass", fmt.Sprintf("%d verified reviewer(s) available", usable), "")
	}
	if _, in := a.runningUnderAgent(); in {
		add("agent-env", "warn", "running inside an agent: approve will refuse here", "approve from a normal terminal")
	}
	// harness dirs: broken links into our store
	st, _ := harness.LoadState(a.Home.InstallsPath())
	broken := 0
	for _, r := range st.Installs {
		if r.Mode == harness.ModeSymlink {
			if _, err := os.Stat(r.Path); err != nil {
				broken++
			}
		}
	}
	if broken > 0 {
		add("installs", "fail", fmt.Sprintf("%d broken symlink(s)", broken), "skigard sync")
	} else {
		add("installs", "pass", fmt.Sprintf("%d install(s) recorded", len(st.Installs)), "")
	}
	return cs
}
