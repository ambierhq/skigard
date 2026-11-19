package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/safefs"
	"github.com/ambierhq/skigard/internal/util"
)

// Target is a directory skills get installed into. Several harnesses can share
// one directory (for example ~/.agents/skills), so installs are per directory.
type Target struct {
	Dir        string   `json:"dir"`
	Harnesses  []string `json:"harnesses"`
	Unverified []string `json:"unverified,omitempty"`
}

// Targets groups harnesses by installation directory.
func Targets(ds []Detection, scope, projectRoot string, env Env) []Target {
	byDir := map[string]*Target{}
	var order []string
	for _, d := range ds {
		dir := d.GlobalAbs
		if scope == "project" {
			dir = d.ProjectDir(projectRoot)
		}
		if dir == "" {
			continue
		}
		t, ok := byDir[dir]
		if !ok {
			t = &Target{Dir: dir}
			byDir[dir] = t
			order = append(order, dir)
		}
		t.Harnesses = append(t.Harnesses, d.ID)
		if !d.Verified {
			t.Unverified = append(t.Unverified, d.ID)
		}
	}
	sort.Strings(order)
	out := make([]Target, 0, len(order))
	for _, dir := range order {
		out = append(out, *byDir[dir])
	}
	return out
}

// Record remembers one installed skill so it can be verified and removed.
type Record struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Mode      string   `json:"mode"` // symlink | copy
	Store     string   `json:"store"`
	Hash      string   `json:"hash"`
	Scope     string   `json:"scope"`
	Harnesses []string `json:"harnesses"`
	At        string   `json:"at"`
}

// Modes.
const (
	ModeSymlink = "symlink"
	ModeCopy    = "copy"
)

// InstallSkill links or copies the approved skill at store into dir/name.
func InstallSkill(dir, name, store, mode string, id hash.Identity, force bool) (*Record, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dest := filepath.Join(dir, name)
	if filepath.Dir(dest) != filepath.Clean(dir) {
		return nil, util.Errf("unsafe install path %q", dest)
	}
	if fi, err := os.Lstat(dest); err == nil {
		if mode == ModeSymlink && fi.Mode()&os.ModeSymlink != 0 {
			if cur, _ := os.Readlink(dest); cur == store {
				return record(name, dest, mode, store, id.Content), nil
			}
		}
		if !force {
			return nil, util.Errf("%s already exists and is not managed by skigard (use --force to replace it; the old one is kept as a backup)", dest)
		}
		backup := fmt.Sprintf("%s.skigard-backup-%d", dest, time.Now().Unix())
		if err := os.Rename(dest, backup); err != nil {
			return nil, err
		}
	}
	switch mode {
	case ModeSymlink:
		if err := os.Symlink(store, dest); err != nil {
			return nil, fmt.Errorf("symlink: %w (try --copy)", err)
		}
	case ModeCopy:
		stage, err := os.MkdirTemp(dir, ".skigard-stage-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(stage)
		tmp := filepath.Join(stage, name)
		if _, err := safefs.CopyTree(store, tmp, safefs.DefaultLimits()); err != nil {
			return nil, err
		}
		if st, err := hash.Check(tmp, id); err != nil || !st.OK() {
			return nil, util.Errf("staged copy of %s does not match its approved hash", name)
		}
		if err := os.Chmod(tmp, 0o755); err != nil {
			return nil, err
		}
		if err := os.Rename(tmp, dest); err != nil {
			return nil, err
		}
	default:
		return nil, util.Errf("unknown install mode %q", mode)
	}
	return record(name, dest, mode, store, id.Content), nil
}

func record(name, dest, mode, store, h string) *Record {
	return &Record{Name: name, Path: dest, Mode: mode, Store: store, Hash: h, At: util.Now()}
}

// Check verifies one install record. It returns a list of problems.
func Check(r Record, want hash.Identity) []string {
	var out []string
	fi, err := os.Lstat(r.Path)
	if err != nil {
		return []string{"missing: " + r.Path}
	}
	switch r.Mode {
	case ModeSymlink:
		if fi.Mode()&os.ModeSymlink == 0 {
			return []string{"not a symlink any more: " + r.Path}
		}
		if cur, _ := os.Readlink(r.Path); cur != r.Store {
			out = append(out, fmt.Sprintf("symlink %s points to %s, expected %s", r.Path, cur, r.Store))
		}
		if _, err := os.Stat(r.Path); err != nil {
			out = append(out, "broken symlink: "+r.Path)
		}
	case ModeCopy:
		st, err := hash.Check(r.Path, want)
		switch {
		case err != nil:
			out = append(out, "cannot hash "+r.Path+": "+err.Error())
		case !st.ContentOK:
			out = append(out, "content differs from approved hash: "+r.Path)
		case !st.ExecOK:
			out = append(out, "executable files differ from those approved: "+r.Path)
		}
	}
	return out
}

// Remove deletes an installed skill, refusing to touch anything that no
// longer matches what skigard installed (unless force).
func Remove(r Record, force bool) error {
	fi, err := os.Lstat(r.Path)
	if err != nil {
		return nil
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		if cur, _ := os.Readlink(r.Path); cur != r.Store && !force {
			return util.Errf("%s is a symlink to %s, not to skigard's store (use --force)", r.Path, cur)
		}
		return os.Remove(r.Path)
	}
	if r.Mode == ModeCopy {
		if got, err := hash.Dir(r.Path); (err != nil || got != r.Hash) && !force {
			return util.Errf("%s was modified after install (use --force to delete it)", r.Path)
		}
		return os.RemoveAll(r.Path)
	}
	if !force {
		return util.Errf("%s is no longer a symlink (use --force)", r.Path)
	}
	return os.RemoveAll(r.Path)
}

// State is the persisted list of installs (stored outside any skills dir).
type State struct {
	Installs []Record `json:"installs"`
}

// LoadState reads path; a missing file is an empty state.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &s, nil
}

// Save writes the state atomically.
func (s *State) Save(path string) error {
	sort.Slice(s.Installs, func(i, j int) bool { return s.Installs[i].Path < s.Installs[j].Path })
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return util.AtomicWrite(path, append(data, '\n'), 0o600)
}

// Upsert adds or replaces the record for r.Path.
func (s *State) Upsert(r Record) {
	for i := range s.Installs {
		if s.Installs[i].Path == r.Path {
			s.Installs[i] = r
			return
		}
	}
	s.Installs = append(s.Installs, r)
}

// Drop removes the record for path.
func (s *State) Drop(path string) {
	out := s.Installs[:0]
	for _, r := range s.Installs {
		if r.Path != path {
			out = append(out, r)
		}
	}
	s.Installs = out
}
