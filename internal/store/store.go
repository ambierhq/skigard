// Package store is skigard's on-disk state: the home directory layout, the
// settings file, the lock file, and the quarantine metadata.
package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/ambierhq/skigard/internal/harness"
	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/review"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/util"
)

// Home is a skigard home directory (default ~/.config/skigard; on Windows
// %APPDATA%\skigard plus %LOCALAPPDATA%\skigard).
//
// Root holds the config-like, reviewable files: skigard.toml, skigard.lock,
// approved skills, reviews and the generated marketplace. Local holds the
// untrusted or machine-local parts: quarantine and state (including the lock
// file). They are the same directory everywhere except by default on Windows,
// where Root is in the roaming profile and Local is not. An empty Local means
// "same as Root".
type Home struct{ Root, Local string }

// localRoot is the directory for quarantine and state.
func (h Home) localRoot() string {
	if h.Local != "" {
		return h.Local
	}
	return h.Root
}

// Resolve picks the home: explicit flag, then $SKIGARD_HOME (both put
// everything in one directory), then $XDG_CONFIG_HOME/skigard, then the
// platform default: ~/.config/skigard, or on Windows %APPDATA%\skigard with
// the quarantine and state in %LOCALAPPDATA%\skigard.
func Resolve(flag string) (Home, error) {
	return resolve(flag, runtime.GOOS, os.Getenv, os.UserHomeDir)
}

func resolve(flag, goos string, getenv func(string) string, userHome func() (string, error)) (Home, error) {
	switch {
	case flag != "":
		return abs(flag)
	case getenv("SKIGARD_HOME") != "":
		return abs(getenv("SKIGARD_HOME"))
	case getenv("XDG_CONFIG_HOME") != "":
		return abs(filepath.Join(getenv("XDG_CONFIG_HOME"), "skigard"))
	}
	home, err := userHome()
	if err != nil {
		return Home{}, err
	}
	if goos == "windows" {
		roaming, local := getenv("APPDATA"), getenv("LOCALAPPDATA")
		if roaming == "" {
			roaming = filepath.Join(home, "AppData", "Roaming")
		}
		if local == "" {
			local = filepath.Join(home, "AppData", "Local")
		}
		return Home{Root: filepath.Join(roaming, "skigard"), Local: filepath.Join(local, "skigard")}, nil
	}
	return Home{Root: filepath.Join(home, ".config", "skigard")}, nil
}

func abs(p string) (Home, error) {
	a, err := filepath.Abs(p)
	return Home{Root: a}, err
}

// Paths.
func (h Home) ManifestPath() string            { return filepath.Join(h.Root, "skigard.toml") }
func (h Home) LockPath() string                { return filepath.Join(h.Root, "skigard.lock") }
func (h Home) SkillsDir() string               { return filepath.Join(h.Root, "skills") }
func (h Home) SkillDir(n string) string        { return filepath.Join(h.Root, "skills", n) }
func (h Home) ReviewsDir() string              { return filepath.Join(h.Root, "reviews") }
func (h Home) QuarantineDir() string           { return filepath.Join(h.localRoot(), "quarantine") }
func (h Home) QuarantineSkill(n string) string { return filepath.Join(h.localRoot(), "quarantine", n) }
func (h Home) MetaPath(n string) string {
	return filepath.Join(h.localRoot(), "quarantine", ".meta", n+".json")
}
func (h Home) StateDir() string       { return filepath.Join(h.localRoot(), "state") }
func (h Home) InstallsPath() string   { return filepath.Join(h.StateDir(), "installs.json") }
func (h Home) MarketplaceDir() string { return filepath.Join(h.Root, "marketplace") }

const gitignore = "# skigard: untrusted or machine-local\nquarantine/\nstate/\n"

// Init creates the layout. It is safe to call repeatedly.
func (h Home) Init() error {
	for _, d := range []string{h.Root, h.localRoot(), h.SkillsDir(), h.ReviewsDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	for _, d := range []string{h.QuarantineDir(), filepath.Join(h.QuarantineDir(), ".meta"), h.StateDir()} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(d, 0o700); err != nil {
			return err
		}
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
	// Mode is "strict" (a human confirms every add) or "auto" (skills that clear
	// Threshold install without a prompt). Empty means DefaultMode.
	Mode string `toml:"mode,omitempty"`
	// Threshold is the minimum safety score (1..10) for auto install. 0 means DefaultThreshold.
	Threshold int `toml:"threshold,omitempty"`
	// AutoReview runs the advisory reviewer during add and update. Nil means on (see EffectiveAutoReview).
	// Review sends the skill to the model provider of the reviewer harness; set it to false to stop that.
	AutoReview *bool `toml:"auto_review,omitempty"`
	// Marketplace keeps marketplace/ in sync automatically and makes verify require it. Nil means off.
	Marketplace *bool `toml:"marketplace,omitempty"`
}

// Modes and defaults for Settings.
const (
	ModeStrict       = "strict"
	ModeAuto         = "auto"
	DefaultMode      = ModeAuto
	DefaultThreshold = scan.DefaultThreshold
)

// Approval modes recorded in the lock.
const (
	ApprovalHuman  = "human"
	ApprovalAuto   = "auto"
	ApprovalForced = "forced"
)

// AutoApprover is the `by` recorded for a machine decision (mode "auto"), so the
// lock never names a person who did not decide. No other mode may use it.
const AutoApprover = "skigard (auto)"

// EffectiveMode is Mode, or DefaultMode when unset.
func (s Settings) EffectiveMode() string {
	if s.Mode == "" {
		return DefaultMode
	}
	return s.Mode
}

// EffectiveThreshold is Threshold, or DefaultThreshold when unset.
func (s Settings) EffectiveThreshold() int {
	if s.Threshold == 0 {
		return DefaultThreshold
	}
	return s.Threshold
}

// EffectiveAutoReview is AutoReview, or true when unset.
func (s Settings) EffectiveAutoReview() bool {
	return s.AutoReview == nil || *s.AutoReview
}

// EffectiveMarketplace is Marketplace, or false when unset.
func (s Settings) EffectiveMarketplace() bool {
	return s.Marketplace != nil && *s.Marketplace
}

// Validate checks mode and threshold.
func (s Settings) Validate() error {
	if s.Mode != "" && s.Mode != ModeStrict && s.Mode != ModeAuto {
		return fmt.Errorf("settings.mode must be %q or %q, got %q", ModeStrict, ModeAuto, util.Clean(s.Mode))
	}
	if s.Threshold != 0 && (s.Threshold < 1 || s.Threshold > 10) {
		return fmt.Errorf("settings.threshold must be 1..10, got %d", s.Threshold)
	}
	return nil
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
	// Allow lists scanner rules a human accepted for skills from one source.
	Allow []AllowRule `toml:"allow,omitempty"`
}

// AllowRule lets the scanner stay quiet about some medium-or-lower rules for
// skills that come from Source. It never applies to high or critical findings.
type AllowRule struct {
	// Source is a skill source as recorded at add time, or a prefix ending in "/*".
	Source string   `toml:"source"`
	Rules  []string `toml:"rules"`
	Reason string   `toml:"reason"`
	By     string   `toml:"by,omitempty"`
	At     string   `toml:"at,omitempty"`
}

var allowRuleID = regexp.MustCompile(`^SKG-[A-Z]+-[0-9]{3}$`)

// Validate checks an allow entry that came from disk.
func (a AllowRule) Validate() error {
	if a.Source == "" || a.Source == "*" || (strings.HasSuffix(a.Source, "*") && !strings.Contains(strings.TrimSuffix(a.Source, "*"), "/")) {
		return fmt.Errorf("allow: source %q is too broad", util.Clean(a.Source))
	}
	if len(a.Rules) == 0 || strings.TrimSpace(a.Reason) == "" {
		return fmt.Errorf("allow %q: needs rules and a reason", util.Clean(a.Source))
	}
	for _, r := range a.Rules {
		if !allowRuleID.MatchString(r) {
			return fmt.Errorf("allow %q: bad rule id %q", util.Clean(a.Source), util.Clean(r))
		}
	}
	return nil
}

// NormalizeSource drops a trailing @ref so an allow entry survives a ref change.
func NormalizeSource(s string) string {
	if i := strings.LastIndex(s, "@"); i > strings.LastIndex(s, "/") {
		return s[:i]
	}
	return s
}

// Matches reports whether source is covered by this entry.
func (a AllowRule) Matches(source string) bool {
	src, want := NormalizeSource(source), NormalizeSource(a.Source)
	if p, ok := strings.CutSuffix(want, "*"); ok {
		return strings.HasPrefix(src, p)
	}
	return src == want
}

// AllowedRules returns the rules allowed for source, with their reasons.
func (m *Manifest) AllowedRules(source string) map[string]string {
	var out map[string]string
	for _, a := range m.Allow {
		if !a.Matches(source) {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		for _, r := range a.Rules {
			out[r] = a.Reason
		}
	}
	return out
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
	if err := m.Settings.Validate(); err != nil {
		return nil, util.Errf("%s: %v", h.ManifestPath(), err)
	}
	for _, a := range m.Allow {
		if err := a.Validate(); err != nil {
			return nil, util.Errf("%s: %v", h.ManifestPath(), err)
		}
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
	// Mode is human, auto or forced. Empty (older locks) reads as human.
	Mode         string `toml:"mode,omitempty"`
	Score        int    `toml:"score,omitempty"`
	Threshold    int    `toml:"threshold,omitempty"`
	ScoreVersion string `toml:"score_version,omitempty"`
}

// EffectiveMode is Mode, or human for records written before modes existed.
func (r ApprovalRec) EffectiveMode() string {
	if r.Mode == "" {
		return ApprovalHuman
	}
	return r.Mode
}

// LockSkill is one approved skill.
type LockSkill struct {
	Name        string `toml:"name"`
	Source      string `toml:"source"`
	Location    string `toml:"location"`
	Ref         string `toml:"ref,omitempty"`
	Subpath     string `toml:"subpath,omitempty"`
	Commit      string `toml:"commit,omitempty"`
	ContentHash string `toml:"content_hash"`
	// HashVersion is the scheme of ContentHash. Entries written before it
	// existed take the lock's default, so they stay version 1.
	HashVersion int `toml:"hash_version,omitempty"`
	// ExecSet and ExecHash (hash version 2): the paths that carried an execute
	// bit at fetch time, and their digest. ExecHash is present even when the
	// set is empty.
	ExecSet      []string    `toml:"exec_set,omitempty"`
	ExecHash     string      `toml:"exec_hash,omitempty"`
	UpgradedFrom string      `toml:"upgraded_from,omitempty"` // the version 1 content hash this entry was upgraded from
	Scan         ScanRec     `toml:"scan"`
	Review       *ReviewRec  `toml:"review,omitempty"`
	Approval     ApprovalRec `toml:"approval"`
}

// Identity returns what this entry pins about the skill's bytes.
func (s LockSkill) Identity() hash.Identity {
	v := s.HashVersion
	if v == 0 {
		v = hash.Legacy
	}
	return hash.Identity{Version: v, Content: s.ContentHash, Exec: s.ExecSet, ExecHash: s.ExecHash}
}

// SetIdentity records id in the entry.
func (s *LockSkill) SetIdentity(id hash.Identity) {
	s.ContentHash, s.HashVersion, s.ExecSet, s.ExecHash = id.Content, id.Version, id.Exec, id.ExecHash
}

// Lock is skigard.lock. HashVersion is the default scheme for entries that do
// not carry their own hash_version (every entry written by this version does).
type Lock struct {
	Version     int         `toml:"version"`
	HashVersion int         `toml:"hash_version"`
	Skills      []LockSkill `toml:"skill"`
}

// LoadLock reads skigard.lock; a missing file is an empty lock.
func LoadLock(h Home) (*Lock, error) {
	l := &Lock{Version: 1, HashVersion: hash.Legacy}
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
	for i := range l.Skills {
		if err := l.Skills[i].normalize(l.HashVersion); err != nil {
			return nil, util.Errf("%s: %s: %v", h.LockPath(), l.Skills[i].Name, err)
		}
	}
	return l, nil
}

// normalize resolves the entry's hash version and checks that the exec set and
// its digest agree. It is a consistency check on the file, not a trust root.
func (s *LockSkill) normalize(def int) error {
	switch s.Approval.Mode {
	case "", ApprovalHuman, ApprovalAuto, ApprovalForced:
	default:
		return util.Errf("unknown approval mode %q", util.Clean(s.Approval.Mode))
	}
	if s.Approval.By == AutoApprover && s.Approval.Mode != ApprovalAuto {
		return util.Errf("approval by %q is only valid with mode %q", AutoApprover, ApprovalAuto)
	}
	if s.Approval.Score < 0 || s.Approval.Score > 10 || s.Approval.Threshold < 0 || s.Approval.Threshold > 10 {
		return util.Errf("approval score/threshold out of range")
	}
	if s.HashVersion == 0 {
		s.HashVersion = def
		if def == 0 {
			s.HashVersion = hash.Legacy
		}
	}
	switch s.HashVersion {
	case 1:
		if len(s.ExecSet) > 0 || s.ExecHash != "" {
			return util.Errf("exec_set is not part of hash version 1")
		}
	case 2:
		for _, p := range s.ExecSet {
			if p == "" || strings.HasPrefix(p, "/") || strings.ContainsRune(p, 0) || strings.Contains(p, "\\") || slices.Contains(strings.Split(p, "/"), "..") {
				return util.Errf("bad exec_set path %q", p)
			}
		}
		if s.ExecHash == "" || hash.ExecDigest(s.ExecSet) != s.ExecHash {
			return util.Errf("exec_hash does not match exec_set")
		}
	default:
		return util.Errf("unsupported hash_version %d", s.HashVersion)
	}
	return nil
}

// Save writes the lock atomically, sorted by name.
func (l *Lock) Save(h Home) error {
	sort.Slice(l.Skills, func(i, j int) bool { return l.Skills[i].Name < l.Skills[j].Name })
	l.HashVersion = hash.Version // entries always carry their own hash_version
	for i := range l.Skills {
		if l.Skills[i].HashVersion == 0 {
			l.Skills[i].HashVersion = hash.Legacy
		}
	}
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
	HashVersion  int            `json:"hash_version,omitempty"` // 0 means 1 (quarantined before hash v2)
	ExecSet      []string       `json:"exec_set,omitempty"`
	ExecHash     string         `json:"exec_hash,omitempty"`
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

// Identity returns what the metadata pins about the quarantined bytes.
func (m *Meta) Identity() hash.Identity {
	v := m.HashVersion
	if v == 0 {
		v = hash.Legacy
	}
	return hash.Identity{Version: v, Content: m.ContentHash, Exec: m.ExecSet, ExecHash: m.ExecHash}
}

// SetIdentity records id in the metadata.
func (m *Meta) SetIdentity(id hash.Identity) {
	m.ContentHash, m.HashVersion, m.ExecSet, m.ExecHash = id.Content, id.Version, id.Exec, id.ExecHash
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
