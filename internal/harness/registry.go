// Package harness knows where each agent harness keeps its skills, detects
// which harnesses are present on this machine, and installs approved skills
// into them.
package harness

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/ambierhq/skigard/internal/util"
)

//go:embed registry.toml
var registryTOML []byte

// Entry describes one harness.
type Entry struct {
	ID       string   `toml:"id" json:"id"`
	Name     string   `toml:"name" json:"name"`
	Project  string   `toml:"project" json:"project_dir"`
	Global   string   `toml:"global" json:"global_dir"`
	EnvHome  string   `toml:"env_home,omitempty" json:"env_home,omitempty"`
	EnvSub   string   `toml:"env_sub,omitempty" json:"-"`
	Bins     []string `toml:"bins,omitempty" json:"bins,omitempty"`
	Dirs     []string `toml:"dirs,omitempty" json:"-"`
	Apps     []string `toml:"apps,omitempty" json:"-"`
	Env      []string `toml:"env,omitempty" json:"-"`
	Verified bool     `toml:"verified,omitempty" json:"verified"`
	Custom   bool     `toml:"-" json:"custom,omitempty"`
}

// Env abstracts the machine so detection is testable.
type Env struct {
	Home     string
	Getenv   func(string) string
	LookPath func(string) (string, error)
	Exists   func(string) bool
	Extra    []string // extra directories searched for binaries
	OS       string   // GOOS; empty means the running platform (tests set it)
}

func (env Env) goos() string {
	if env.OS != "" {
		return env.OS
	}
	return runtime.GOOS
}

// RealEnv is the current machine.
func RealEnv() Env {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		appData, local := os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		if local == "" {
			local = filepath.Join(home, "AppData", "Local")
		}
		return Env{
			Home:     home,
			Getenv:   os.Getenv,
			LookPath: lookPath,
			Exists:   func(p string) bool { _, err := os.Stat(p); return err == nil },
			Extra: []string{
				filepath.Join(home, ".local", "bin"), filepath.Join(appData, "npm"), filepath.Join(home, ".bun", "bin"),
				filepath.Join(home, "go", "bin"), filepath.Join(local, "Microsoft", "WinGet", "Links"),
				filepath.Join(local, "Programs"),
			},
		}
	}
	return Env{
		Home:     home,
		Getenv:   os.Getenv,
		LookPath: lookPath,
		Exists:   func(p string) bool { _, err := os.Stat(p); return err == nil },
		Extra: []string{
			filepath.Join(home, ".local", "bin"), "/opt/homebrew/bin", "/usr/local/bin",
			filepath.Join(home, ".bun", "bin"), filepath.Join(home, ".npm-global", "bin"), filepath.Join(home, "go", "bin"),
		},
	}
}

func lookPath(name string) (string, error) { return execLookPath(name) }

// Builtin returns the embedded registry.
func Builtin() ([]Entry, error) {
	var doc struct {
		Harness []Entry `toml:"harness"`
	}
	if err := toml.Unmarshal(registryTOML, &doc); err != nil {
		return nil, err
	}
	return doc.Harness, nil
}

// Registry merges user-defined entries over the builtin ones (by id).
func Registry(custom []Entry) ([]Entry, error) {
	base, err := Builtin()
	if err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for i, e := range base {
		idx[e.ID] = i
	}
	for _, c := range custom {
		c.Custom = true
		if i, ok := idx[c.ID]; ok {
			base[i] = c
		} else {
			base = append(base, c)
		}
	}
	return base, nil
}

// Find returns the entry with the given id.
func Find(reg []Entry, id string) (Entry, bool) {
	for _, e := range reg {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// Expand resolves "~" and "{config}" in p. "~" is the user's home
// (%USERPROFILE% on Windows). "{config}" is $XDG_CONFIG_HOME when set, else
// ~/.config, except on Windows where it is %APPDATA%.
func (env Env) Expand(p string) string {
	if p == "" {
		return ""
	}
	cfg := env.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(env.Home, ".config")
		if env.goos() == "windows" {
			if cfg = env.Getenv("APPDATA"); cfg == "" {
				cfg = filepath.Join(env.Home, "AppData", "Roaming")
			}
		}
	}
	if strings.Contains(p, "{config}") {
		// the registry writes slashes; make the result a native path
		return filepath.Clean(filepath.FromSlash(strings.ReplaceAll(p, "{config}", cfg)))
	}
	if p == "~" {
		return env.Home
	}
	if strings.HasPrefix(p, "~/") || (env.goos() == "windows" && strings.HasPrefix(p, `~\`)) {
		return filepath.Join(env.Home, filepath.FromSlash(p[2:]))
	}
	return p
}

// GlobalDir is the user-scope skills directory for the harness.
func (e Entry) GlobalDir(env Env) string {
	if e.EnvHome != "" {
		if v := strings.TrimSpace(env.Getenv(e.EnvHome)); v != "" && e.EnvSub != "" {
			return filepath.Join(v, e.EnvSub)
		}
	}
	return env.Expand(e.Global)
}

// ProjectDir is the project-scope skills directory.
func (e Entry) ProjectDir(projectRoot string) string {
	if e.Project == "" {
		return ""
	}
	return filepath.Join(projectRoot, filepath.FromSlash(e.Project))
}

// Validate checks a custom entry.
func (e Entry) Validate() error {
	if e.ID == "" || e.Name == "" {
		return util.Errf("harness needs id and name")
	}
	if e.Global == "" && e.Project == "" {
		return util.Errf("harness %q needs a global or project dir", e.ID)
	}
	return nil
}

// Status of a harness on this machine.
const (
	Installed  = "installed"  // binary or app found
	Configured = "configured" // only a config directory found
	Absent     = "absent"
)

// Detection is the result of probing one harness.
type Detection struct {
	Entry
	Status    string   `json:"status"`
	Signals   []string `json:"signals,omitempty"`
	Bin       string   `json:"binary,omitempty"`
	GlobalAbs string   `json:"global_path"`
}

// Detect probes every harness.
func Detect(reg []Entry, env Env) []Detection {
	out := make([]Detection, 0, len(reg))
	for _, e := range reg {
		d := Detection{Entry: e, Status: Absent, GlobalAbs: e.GlobalDir(env)}
		for _, b := range e.Bins {
			if p := findBin(env, b); p != "" {
				d.Status, d.Bin = Installed, p
				d.Signals = append(d.Signals, "binary:"+p)
				break
			}
		}
		for _, a := range e.Apps {
			if env.Exists(a) {
				d.Status = Installed
				d.Signals = append(d.Signals, "app:"+a)
			}
		}
		dirs := e.Dirs
		if dirs == nil && e.Global != "" {
			dirs = []string{filepath.Dir(e.Global)}
		}
		for _, dir := range dirs {
			if p := env.Expand(dir); p != "" && env.Exists(p) {
				if d.Status == Absent {
					d.Status = Configured
				}
				d.Signals = append(d.Signals, "dir:"+p)
			}
		}
		if e.EnvHome != "" && strings.TrimSpace(env.Getenv(e.EnvHome)) != "" {
			d.Signals = append(d.Signals, "env:"+e.EnvHome)
			if d.Status == Absent {
				d.Status = Configured
			}
		}
		out = append(out, d)
	}
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i].Status) < rank(out[j].Status) })
	return out
}

func rank(s string) int {
	switch s {
	case Installed:
		return 0
	case Configured:
		return 1
	}
	return 2
}

func findBin(env Env, name string) string {
	if p, err := env.LookPath(name); err == nil {
		return p
	}
	for _, dir := range env.Extra {
		for _, c := range binCandidates(name, env.goos(), env.Getenv("PATHEXT")) {
			if p := filepath.Join(dir, c); env.Exists(p) {
				return p
			}
		}
	}
	return ""
}

// binCandidates lists the file names an executable called name may have. On
// Windows that is name plus each PATHEXT extension (default .COM;.EXE;.BAT;.CMD,
// plus .ps1 for npm-style shims); elsewhere just name.
func binCandidates(name, goos, pathext string) []string {
	if goos != "windows" {
		return []string{name}
	}
	if pathext == "" {
		pathext = ".COM;.EXE;.BAT;.CMD"
	}
	out := []string{}
	seen := map[string]bool{}
	add := func(n string) {
		if k := strings.ToLower(n); !seen[k] {
			seen[k] = true
			out = append(out, n)
		}
	}
	if ext := filepath.Ext(name); ext != "" && strings.Contains(";"+strings.ToLower(pathext)+";.ps1;", ";"+strings.ToLower(ext)+";") {
		add(name) // already has an executable extension
	}
	for _, ext := range strings.Split(pathext+";.ps1", ";") {
		if ext = strings.TrimSpace(ext); ext != "" {
			add(name + strings.ToLower(ext))
		}
	}
	return out
}

// Running returns the harness skigard appears to be running inside, if any.
// These signals are environment variables a harness sets for the commands it
// runs. A hit means the caller must not be allowed to approve anything.
func Running(reg []Entry, getenv func(string) string) *Entry {
	for i := range reg {
		for _, name := range reg[i].Env {
			if getenv(name) != "" {
				return &reg[i]
			}
		}
	}
	return nil
}

// GenericAgentEnv are agent-agnostic signals.
var GenericAgentEnv = []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CODEX_SANDBOX", "CODEX_THREAD_ID",
	"GEMINI_CLI", "OPENCODE", "CURSOR_AGENT", "PI_CODING_AGENT", "GOOSE_TERMINAL", "AMP_CURRENT_THREAD_ID"}

// InAgent reports whether any agent signal is present.
func InAgent(getenv func(string) string) (string, bool) {
	for _, n := range GenericAgentEnv {
		if getenv(n) != "" {
			return n, true
		}
	}
	return "", false
}
