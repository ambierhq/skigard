package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/safefs"
	"github.com/ambierhq/skigard/internal/scan"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// MigrateResult summarises an import from an skm repository.
type MigrateResult struct {
	Migrated []string
	Skipped  map[string]string
	Plugins  []string
}

type skmState struct {
	Approved *struct {
		At       string `json:"at"`
		Hash     string `json:"hash"`
		Override bool   `json:"override"`
		Origin   struct {
			Commit   string `json:"commit"`
			Location string `json:"location"`
			Ref      string `json:"ref"`
			Spec     string `json:"spec"`
			Subpath  string `json:"subpath"`
			Type     string `json:"type"`
		} `json:"origin"`
	} `json:"approved"`
}

// skmHash reproduces skm's content hash so we can prove the bytes we import
// are exactly the bytes that were approved there.
func skmHash(root string) (string, error) {
	type ent struct {
		rel, abs string
		link     bool
	}
	var ents []ent
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "__pycache__" || name == ".DS_Store" {
				return filepath.SkipDir
			}
			return nil
		}
		if name == ".DS_Store" || filepath.Ext(name) == ".pyc" {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		ents = append(ents, ent{filepath.ToSlash(rel), p, d.Type()&fs.ModeSymlink != 0})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(ents, func(i, j int) bool { return ents[i].rel < ents[j].rel })
	h := sha256.New()
	for _, e := range ents {
		if e.link {
			t, _ := os.Readlink(e.abs)
			h.Write([]byte("L\x00" + e.rel + "\x00" + t + "\x00"))
			continue
		}
		info, err := os.Stat(e.abs)
		if err != nil {
			return "", err
		}
		mode := "-"
		if info.Mode()&0o100 != 0 {
			mode = "x"
		}
		data, err := os.ReadFile(e.abs)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(data)
		h.Write([]byte("F\x00" + e.rel + "\x00" + mode + "\x00"))
		h.Write(sum[:])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MigrateSkm imports approved skills, plugins and settings from an skm repository.
// A skill is only imported when its bytes still match the hash skm approved.
func (a *App) MigrateSkm(repo string) (*MigrateResult, error) {
	unlock, err := a.Home.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := a.Home.Init(); err != nil {
		return nil, err
	}
	res := &MigrateResult{Skipped: map[string]string{}}
	entries, err := os.ReadDir(filepath.Join(repo, "skills"))
	if err != nil {
		return nil, util.Errf("%s does not look like an skm repository (no skills/): %v", repo, err)
	}
	lock, err := store.LoadLock(a.Home)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || name[0] == '.' {
			continue
		}
		if _, ok := lock.Get(name); ok {
			res.Skipped[name] = "already approved in skigard"
			continue
		}
		data, err := os.ReadFile(filepath.Join(repo, ".review", name+".json"))
		if err != nil {
			res.Skipped[name] = "no approval record in .review/"
			continue
		}
		var st skmState
		if err := json.Unmarshal(data, &st); err != nil || st.Approved == nil {
			res.Skipped[name] = "not approved in skm"
			continue
		}
		src := filepath.Join(repo, "skills", name)
		cur, err := skmHash(src)
		if err != nil || cur != st.Approved.Hash {
			res.Skipped[name] = "content differs from what skm approved; re-add and review it"
			continue
		}
		dest := a.Home.SkillDir(name)
		if err := os.RemoveAll(dest); err != nil {
			return res, err
		}
		if _, err := safefs.CopyTree(src, dest, safefs.DefaultLimits()); err != nil {
			os.RemoveAll(dest)
			res.Skipped[name] = "cannot import: " + err.Error()
			continue
		}
		if err := os.Chmod(dest, 0o755); err != nil {
			os.RemoveAll(dest)
			res.Skipped[name] = err.Error()
			continue
		}
		h, err := hash.Dir(dest)
		if err != nil {
			os.RemoveAll(dest)
			res.Skipped[name] = err.Error()
			continue
		}
		// The import is a local directory, so its filesystem modes are the source modes.
		exec, err := hash.ExecSet(dest)
		if err != nil {
			os.RemoveAll(dest)
			res.Skipped[name] = err.Error()
			continue
		}
		rep, err := scan.Scan(dest, a.scanOpts(name, nil, nil))
		if err != nil {
			return res, err
		}
		o := st.Approved.Origin
		source := o.Spec
		if source == "" {
			source = o.Location
		}
		if o.Subpath != "" && o.Type == "git" {
			source += "/" + o.Subpath
		}
		reason := "migrated from skm; bytes match the hash skm approved"
		ls := store.LockSkill{
			Name: name, Source: source, Location: trimGit(o.Location), Ref: o.Ref, Subpath: o.Subpath, Commit: o.Commit,
			Scan:     store.ScanRec{Tool: rep.Tool, Rules: rep.RulesVersion, Verdict: rep.Verdict, Coverage: rep.Coverage, Risk: rep.Risk, Findings: len(rep.Findings)},
			Approval: store.ApprovalRec{By: "skm", At: st.Approved.At, Override: st.Approved.Override, Reason: reason, Migrated: "skm"},
		}
		ls.SetIdentity(hash.Identity{Version: hash.Version, Content: h, Exec: exec, ExecHash: hash.ExecDigest(exec)})
		lock.Put(ls)
		res.Migrated = append(res.Migrated, name)
	}
	if err := lock.Save(a.Home); err != nil {
		return res, err
	}
	man, err := store.LoadManifest(a.Home)
	if err != nil {
		return res, err
	}
	if data, err := os.ReadFile(filepath.Join(repo, "skm.config.json")); err == nil {
		var cfg struct {
			Marketplace string `json:"marketplace_name"`
			Description string `json:"description"`
			Owner       struct {
				Name string `json:"name"`
			} `json:"owner"`
		}
		if json.Unmarshal(data, &cfg) == nil {
			man.Settings.MarketplaceName, man.Settings.Description, man.Settings.Owner = cfg.Marketplace, cfg.Description, cfg.Owner.Name
		}
	}
	if pe, err := os.ReadDir(filepath.Join(repo, "plugins")); err == nil {
		for _, e := range pe {
			data, err := os.ReadFile(filepath.Join(repo, "plugins", e.Name()))
			if err != nil || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			var p struct {
				Name, Description string
				Skills            []string
			}
			if json.Unmarshal(data, &p) != nil || p.Name == "" {
				continue
			}
			var keep []string
			for _, s := range p.Skills {
				if _, ok := lock.Get(s); ok {
					keep = append(keep, s)
				}
			}
			if man.Plugins == nil {
				man.Plugins = map[string]store.Plugin{}
			}
			man.Plugins[p.Name] = store.Plugin{Description: p.Description, Skills: append([]string{}, keep...)}
			res.Plugins = append(res.Plugins, p.Name)
		}
	}
	if err := store.SaveManifest(a.Home, man); err != nil {
		return res, err
	}
	if len(man.Plugins) > 0 {
		if err := a.marketplaceBuild(); err != nil {
			return res, util.Errf("migrated, but could not build the marketplace: %v", err)
		}
	}
	return res, nil
}

func trimGit(s string) string {
	if len(s) > 4 && s[len(s)-4:] == ".git" {
		return s[:len(s)-4]
	}
	return s
}
