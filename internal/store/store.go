// Package store is skigard's on-disk state: the home directory layout, the
// settings file, the lock file, and the quarantine metadata.
package store

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"

	"github.com/ambierhq/skigard/internal/harness"
	"github.com/ambierhq/skigard/internal/review"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/util"
)

// Home is a skigard home directory (default ~/.config/skigard).
type Home struct{ Root string }

// Resolve picks the home: explicit flag, then $SKIGARD_HOME, then
// $XDG_CONFIG_HOME/skigard, then ~/.config/skigard.
func Resolve(flag string) (Home, error) {
	switch {
	case flag != "":
		return abs(flag)
	case os.Getenv("SKIGARD_HOME") != "":
		return abs(os.Getenv("SKIGARD_HOME"))
	case os.Getenv("XDG_CONFIG_HOME") != "":
		return abs(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "skigard"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Home{}, err
	}
	return Home{filepath.Join(home, ".config", "skigard")}, nil
}

func abs(p string) (Home, error) {
	a, err := filepath.Abs(p)
	return Home{a}, err
}

// Paths.
func (h Home) ManifestPath() string            { return filepath.Join(h.Root, "skigard.toml") }
func (h Home) LockPath() string                { return filepath.Join(h.Root, "skigard.lock") }
func (h Home) SkillsDir() string               { return filepath.Join(h.Root, "skills") }
func (h Home) SkillDir(n string) string        { return filepath.Join(h.Root, "skills", n) }
func (h Home) ReviewsDir() string              { return filepath.Join(h.Root, "reviews") }
func (h Home) QuarantineDir() string           { return filepath.Join(h.Root, "quarantine") }
func (h Home) QuarantineSkill(n string) string { return filepath.Join(h.Root, "quarantine", n) }
func (h Home) MetaPath(n string) string {
	return filepath.Join(h.Root, "quarantine", ".meta", n+".json")
}
func (h Home) StateDir() string       { return filepath.Join(h.Root, "state") }
func (h Home) InstallsPath() string   { return filepath.Join(h.StateDir(), "installs.json") }
func (h Home) MarketplaceDir() string { return filepath.Join(h.Root, "marketplace") }

const gitignore = "# skigard: untrusted or machine-local\nquarantine/\nstate/\n"

// Init creates the layout. It is safe to call repeatedly.
func (h Home) Init() error {
	for _, d := range []string{h.Root, h.SkillsDir(), h.ReviewsDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	for _, d := range []string{h.QuarantineDir(), filepath.Join(h.QuarantineDir(), ".meta"), h.StateDir()} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
		os.Chmod(d, 0o700)
	}
	if _, err := os.Stat(filepath.Join(h.Root, ".gitignore")); os.IsNotExist(err) {
		if err := os.WriteFile(filepath.Join(h.Root, ".gitignore"), []byte(gitignore), 0o644); err != nil {
			return err
		}
	}
	if _, err := os.Stat(h.ManifestPath()); os.IsNotExist(err) {
		if err := SaveManifest(h, &Manifest{Version: 1}); err != nil {
			return err
		}
	}
	return nil
}

// Exists reports whether the home has been initialised.
func (h Home) Exists() bool {
	_, err := os.Stat(h.ManifestPath())
	return err == nil
}

// ---- manifest ------------------------------------------------------------

// Settings are user preferences.
type Settings struct {
	ReviewHarness   string   `toml:"review_harness,omitempty"`
	InstallTo       []string `toml:"install_to,omitempty"`
	Owner           string   `toml:"owner,omitempty"`
	MarketplaceName string   `toml:"marketplace_name,omitempty"`
	Description     string   `toml:"description,omitempty"`
	RequireReview   bool     `toml:"require_review,omitempty"`
}

// Plugin is a named group of approved skills.
type Plugin struct {
	Description string   `toml:"description,omitempty"`
	Skills      []string `toml:"skills"`
}

// Manifest is skigard.toml: settings, plugins and custom harnesses.
type Manifest struct {
	Version  int               `toml:"version"`
	Settings Settings          `toml:"settings"`
	Plugins  map[string]Plugin `toml:"plugins,omitempty"`
	Harness  []harness.Entry   `toml:"harness,omitempty"`
}

// LoadManifest reads skigard.toml; a missing file yields defaults.
func LoadManifest(h Home) (*Manifest, error) {
	m := &Manifest{Version: 1}
	data, err := os.ReadFile(h.ManifestPath())
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, m); err != nil {
		return nil, util.Errf("%s: %v", h.ManifestPath(), err)
	}
	for pn, p := range m.Plugins {
		if err := util.CheckName(pn); err != nil {
			return nil, util.Errf("%s: plugin: %v", h.ManifestPath(), err)
		}
		for _, sn := range p.Skills {
			if err := util.CheckName(sn); err != nil {
				return nil, util.Errf("%s: plugin %s: %v", h.ManifestPath(), pn, err)
			}
		}
	}
	return m, nil
}

// SaveManifest writes skigard.toml atomically.
func SaveManifest(h Home, m *Manifest) error {
	var buf bytes.Buffer
	buf.WriteString("# skigard settings, plugins and custom harnesses. The skills themselves are recorded in skigard.lock.\n")
	if err := toml.NewEncoder(&buf).Encode(m); err != nil {
		return err
	}
	return util.AtomicWrite(h.ManifestPath(), buf.Bytes(), 0o644)
}

// ---- lock ----------------------------------------------------------------

// ScanRec is the scan summary pinned in the lock.
type ScanRec struct {
	Tool     string `toml:"tool"`
	Rules    string `toml:"rules"`
	Verdict  string `toml:"verdict"`
	Coverage string `toml:"coverage"`
	Risk     int    `toml:"risk"`
	Findings int    `toml:"findings"`
}

// ReviewRec is the review summary pinned in the lock.
type ReviewRec struct {
	Reviewer string `toml:"reviewer"`
	Verdict  string `toml:"verdict"`
	Coverage string `toml:"coverage"`
	Record   string `toml:"record"`
}

// ApprovalRec records the human decision.
type ApprovalRec struct {
	By       string          `toml:"by"`
	At       string          `toml:"at"`
	Override bool            `toml:"override,omitempty"`
	Reason   string          `toml:"reason,omitempty"`
	Migrated string          `toml:"migrated_from,omitempty"`
	Accepted []scan.Accepted `toml:"accepted,omitempty"`
}

// LockSkill is one approved skill.
type LockSkill struct {
	Name        string      `toml:"name"`
	Source      string      `toml:"source"`
	Location    string      `toml:"location"`
	Ref         string      `toml:"ref,omitempty"`
	Subpath     string      `toml:"subpath,omitempty"`
	Commit      string      `toml:"commit,omitempty"`
	ContentHash string      `toml:"content_hash"`
	Scan        ScanRec     `toml:"scan"`
	Review      *ReviewRec  `toml:"review,omitempty"`
	Approval    ApprovalRec `toml:"approval"`
}

// Lock is skigard.lock.
type Lock struct {
	Version     int         `toml:"version"`
	HashVersion int         `toml:"hash_version"`
	Skills      []LockSkill `toml:"skill"`
}

// LoadLock reads skigard.lock; a missing file is an empty lock.
func LoadLock(h Home) (*Lock, error) {
	l := &Lock{Version: 1, HashVersion: 1}
	data, err := os.ReadFile(h.LockPath())
	if os.IsNotExist(err) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, l); err != nil {
		return nil, util.Errf("%s: %v", h.LockPath(), err)
	}
	if l.Version != 1 {
		return nil, util.Errf("unsupported lock version %d", l.Version)
	}
	seen := map[string]bool{}
	for _, s := range l.Skills {
		if err := util.CheckName(s.Name); err != nil {
			return nil, util.Errf("%s: %v", h.LockPath(), err)
		}
		if seen[s.Name] {
			return nil, util.Errf("%s: %q appears twice", h.LockPath(), s.Name)
		}
		seen[s.Name] = true
	}
	return l, nil
}

// Save writes the lock atomically, sorted by name.
func (l *Lock) Save(h Home) error {
	sort.Slice(l.Skills, func(i, j int) bool { return l.Skills[i].Name < l.Skills[j].Name })
	var buf bytes.Buffer
	buf.WriteString("# Generated by skigard. Each entry is an approved skill pinned to a content hash.\n# Review changes to this file like code.\n")
	if err := toml.NewEncoder(&buf).Encode(l); err != nil {
		return err
	}
	return util.AtomicWrite(h.LockPath(), buf.Bytes(), 0o644)
}

// Get returns the lock entry for name.
func (l *Lock) Get(name string) (*LockSkill, bool) {
	for i := range l.Skills {
		if l.Skills[i].Name == name {
			return &l.Skills[i], true
		}
	}
	return nil, false
}

// Put adds or replaces an entry.
func (l *Lock) Put(s LockSkill) {
	for i := range l.Skills {
		if l.Skills[i].Name == s.Name {
			l.Skills[i] = s
			return
		}
	}
	l.Skills = append(l.Skills, s)
}

// Delete removes an entry.
func (l *Lock) Delete(name string) {
	out := l.Skills[:0]
	for _, s := range l.Skills {
		if s.Name != name {
			out = append(out, s)
		}
	}
	l.Skills = out
}

// ---- quarantine metadata ---------------------------------------------------

// Meta describes a quarantined skill. It lives next to, not inside, the skill
// so it never changes the content hash.
type Meta struct {
	Name         string         `json:"name"`
	Source       string         `json:"source"`
	Kind         string         `json:"kind"`
	Location     string         `json:"location"`
	Ref          string         `json:"ref,omitempty"`
	Subpath      string         `json:"subpath,omitempty"`
	Commit       string         `json:"commit,omitempty"`
	FetchedAt    string         `json:"fetched_at"`
	ContentHash  string         `json:"content_hash"`
	SkippedLinks []string       `json:"skipped_links,omitempty"`
	Review       *review.Record `json:"review,omitempty"`
}

// LoadMeta reads the metadata for a quarantined skill.
func LoadMeta(h Home, name string) (*Meta, error) {
	if err := util.CheckName(name); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(h.MetaPath(name))
	if os.IsNotExist(err) {
		return nil, util.Errf("%q is not in quarantine (run: skigard add ...)", name)
	}
	if err != nil {
		return nil, err
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, util.Errf("%s: %v", h.MetaPath(name), err)
	}
	return &m, nil
}

// Save writes the metadata.
func (m *Meta) Save(h Home) error {
	if err := util.CheckName(m.Name); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return util.AtomicWrite(h.MetaPath(m.Name), append(data, '\n'), 0o600)
}

// QuarantinedNames lists skills currently in quarantine.
func QuarantinedNames(h Home) []string {
	entries, _ := os.ReadDir(filepath.Join(h.QuarantineDir(), ".meta"))
	var out []string
	for _, e := range entries {
		if n := e.Name(); filepath.Ext(n) == ".json" {
			out = append(out, n[:len(n)-5])
		}
	}
	sort.Strings(out)
	return out
}

// ---- approval bundle ---------------------------------------------------------

// Bundle is the evidence stored under reviews/<name>/ for one approval.
type Bundle struct {
	Skill    string         `json:"skill"`
	Hash     string         `json:"content_hash"`
	Source   string         `json:"source"`
	Commit   string         `json:"commit,omitempty"`
	Scan     *scan.Report   `json:"scan"`
	Review   *review.Record `json:"review,omitempty"`
	Approval ApprovalRec    `json:"approval"`
}

// SaveBundle writes reviews/<name>/<hash12>.json and returns its relative path.
func SaveBundle(h Home, b *Bundle, short string) (string, error) {
	rel := filepath.ToSlash(filepath.Join("reviews", b.Skill, short+".json"))
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return "", err
	}
	return rel, util.AtomicWrite(filepath.Join(h.Root, filepath.FromSlash(rel)), append(data, '\n'), 0o644)
}
