// Package source parses skill sources and fetches them into a temp directory
// with strict limits. It never runs git or any code from the source.
package source

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ambierhq/skigard/internal/util"
)

// Kind of source.
const (
	KindGitHub = "github"
	KindLocal  = "local"
)

// Spec is a parsed source reference.
type Spec struct {
	Kind    string
	Owner   string
	Repo    string
	Ref     string // branch, tag or commit; empty = default branch
	Subpath string // slash-separated path inside the repo
	Path    string // local path (Kind == local)
}

var (
	segRe   = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	shaRe   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	ghURLRe = regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+?)(?:\.git)?(?:/(.*))?$`)
	shortRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(/[^@]*)?(@[^@]+)?$`)
)

// IsCommit reports whether s is a full commit SHA.
func IsCommit(s string) bool { return shaRe.MatchString(s) }

// Parse understands:
//
//	github:owner/repo[/path][@ref]
//	owner/repo[/path][@ref]
//	https://github.com/owner/repo[.git][/tree/ref/path]
//	./dir, /abs/dir, ~/dir
func Parse(s string) (Spec, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Spec{}, util.Errf("empty source")
	}
	switch {
	case strings.HasPrefix(s, "github:"):
		return parseShort(strings.TrimPrefix(s, "github:"))
	case strings.HasPrefix(s, "https://github.com/"):
		m := ghURLRe.FindStringSubmatch(s)
		if m == nil {
			return Spec{}, util.Errf("unrecognised GitHub URL %q", s)
		}
		sp := Spec{Kind: KindGitHub, Owner: m[1], Repo: m[2]}
		if rest := strings.Trim(m[3], "/"); rest != "" {
			parts := strings.Split(rest, "/")
			if (parts[0] == "tree" || parts[0] == "blob") && len(parts) >= 2 {
				sp.Ref = parts[1]
				sp.Subpath = strings.Join(parts[2:], "/")
			} else {
				sp.Subpath = rest
			}
		}
		return sp.validate()
	case strings.Contains(s, "://") || strings.HasPrefix(s, "git@"):
		return Spec{}, util.Errf("unsupported source %q (only github.com and local paths are supported)", s)
	}
	if strings.HasPrefix(s, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			s = filepath.Join(home, s[2:])
		}
	}
	isPathLike := strings.HasPrefix(s, ".") || strings.HasPrefix(s, "/") || filepath.IsAbs(s)
	if !isPathLike {
		if info, err := os.Stat(s); err == nil && info.IsDir() {
			isPathLike = true
		}
	}
	if isPathLike {
		abs, err := filepath.Abs(s)
		if err != nil {
			return Spec{}, err
		}
		return Spec{Kind: KindLocal, Path: abs}, nil
	}
	if shortRe.MatchString(s) {
		return parseShort(s)
	}
	return Spec{}, util.Errf("cannot understand source %q", s)
}

func parseShort(s string) (Spec, error) {
	ref := ""
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s, ref = s[:i], s[i+1:]
	}
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) < 2 {
		return Spec{}, util.Errf("source needs owner/repo")
	}
	sp := Spec{Kind: KindGitHub, Owner: parts[0], Repo: strings.TrimSuffix(parts[1], ".git"), Ref: ref}
	sp.Subpath = strings.Join(parts[2:], "/")
	return sp.validate()
}

func (s Spec) validate() (Spec, error) {
	if !segRe.MatchString(s.Owner) || !segRe.MatchString(s.Repo) || s.Owner == "." || s.Owner == ".." || s.Repo == "." || s.Repo == ".." {
		return Spec{}, util.Errf("invalid GitHub owner/repo %q/%q", s.Owner, s.Repo)
	}
	if s.Ref != "" && (strings.HasPrefix(s.Ref, "-") || strings.ContainsAny(s.Ref, " \t\n\\^~:?*[") || strings.Contains(s.Ref, "..")) {
		return Spec{}, util.Errf("invalid ref %q", s.Ref)
	}
	if s.Subpath != "" {
		c, err := CleanSubpath(s.Subpath)
		if err != nil {
			return Spec{}, err
		}
		s.Subpath = c
	}
	return s, nil
}

// CleanSubpath validates a repo-relative path.
func CleanSubpath(p string) (string, error) {
	if p == "" || p == "." {
		return "", nil
	}
	if strings.HasPrefix(p, "/") || strings.Contains(p, `\`) || strings.ContainsRune(p, 0) {
		return "", util.Errf("invalid path %q", p)
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return "", util.Errf("invalid path %q (parent segments not allowed)", p)
		}
	}
	c := path.Clean(p)
	if c == "." {
		return "", nil
	}
	return c, nil
}

// String returns the canonical form of the spec.
func (s Spec) String() string {
	if s.Kind == KindLocal {
		return s.Path
	}
	out := "github:" + s.Owner + "/" + s.Repo
	if s.Subpath != "" {
		out += "/" + s.Subpath
	}
	if s.Ref != "" {
		out += "@" + s.Ref
	}
	return out
}

// Location is the repository URL or local path, without subpath or ref.
func (s Spec) Location() string {
	if s.Kind == KindLocal {
		return s.Path
	}
	return "https://github.com/" + s.Owner + "/" + s.Repo
}
