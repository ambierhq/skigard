// Package core implements skigard's workflows: add, scan, review, approve,
// update, install, verify and friends. The CLI is a thin layer over it.
package core

import (
	"context"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ambierhq/skigard/internal/harness"
	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/review"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/source"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// App holds everything a workflow needs, so tests can substitute any of it.
type App struct {
	Home          store.Home
	Env           harness.Env
	Fetcher       *source.Fetcher
	Runner        review.Runner
	Out, Err      io.Writer
	Version       string
	User          string
	RefuseInAgent bool // approve refuses when an agent harness is detected
}

// New builds an App for the real machine.
func New(home store.Home, version string, out, errw io.Writer) *App {
	u := os.Getenv("USER")
	if cu, err := user.Current(); err == nil && cu.Username != "" {
		u = cu.Username
	}
	return &App{
		Home: home, Env: harness.RealEnv(), Fetcher: source.NewFetcher(),
		Runner: review.Runner{Environ: os.Environ()},
		Out:    out, Err: errw, Version: version, User: u, RefuseInAgent: true,
	}
}

// Registry returns the builtin harnesses plus user-defined ones.
func (a *App) Registry() ([]harness.Entry, *store.Manifest, error) {
	man, err := store.LoadManifest(a.Home)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range man.Harness {
		if err := c.Validate(); err != nil {
			return nil, nil, err
		}
	}
	reg, err := harness.Registry(man.Harness)
	return reg, man, err
}

// Detect probes the machine for harnesses.
func (a *App) Detect() ([]harness.Detection, error) {
	reg, _, err := a.Registry()
	if err != nil {
		return nil, err
	}
	return harness.Detect(reg, a.Env), nil
}

// ScanOptions builds scan options for the tool version.
func (a *App) scanOpts(name string, accepted []scan.Accepted, links []string) scan.Options {
	return scan.Options{ToolVersion: a.Version, Skill: name, Accepted: accepted, SkippedLinks: links}
}

// Scan scans a skill by name (quarantine first, then approved) or a directory path.
func (a *App) Scan(target string) (*scan.Report, string, error) {
	if fi, err := os.Stat(target); err == nil && fi.IsDir() && (strings.ContainsRune(target, filepath.Separator) || strings.HasPrefix(target, ".")) {
		rep, err := scan.Scan(target, a.scanOpts(filepath.Base(target), nil, nil))
		return rep, target, err
	}
	if err := util.CheckName(target); err != nil {
		return nil, "", err
	}
	if dir := a.Home.QuarantineSkill(target); isDir(dir) {
		meta, err := store.LoadMeta(a.Home, target)
		if err != nil {
			return nil, "", err
		}
		rep, err := scan.Scan(dir, a.scanOpts(target, nil, meta.SkippedLinks))
		return rep, dir, err
	}
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, "", err
	}
	if ls, ok := lock.Get(target); ok {
		dir := a.Home.SkillDir(target)
		var acc []scan.Accepted
		if cur, err := hash.Dir(dir); err == nil && cur == ls.ContentHash {
			acc = ls.Approval.Accepted
		}
		rep, err := scan.Scan(dir, a.scanOpts(target, acc, nil))
		return rep, dir, err
	}
	return nil, "", util.Errf("no skill named %q (not in quarantine or approved)", target)
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// Reject deletes a quarantined skill.
func (a *App) Reject(name string) error {
	unlock, err := a.Home.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	if err := util.CheckName(name); err != nil {
		return err
	}
	if _, err := store.LoadMeta(a.Home, name); err != nil {
		return err
	}
	if err := os.RemoveAll(a.Home.QuarantineSkill(name)); err != nil {
		return err
	}
	return os.Remove(a.Home.MetaPath(name))
}

// ctxOrBackground avoids nil contexts from callers.
func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
