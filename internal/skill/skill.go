// Package skill parses and validates Agent Skills (a directory with SKILL.md).
package skill

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ambierhq/skigard/internal/util"
)

var nameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var keyRe = regexp.MustCompile(`^([A-Za-z0-9_-]+)\s*:\s*(.*)$`)

// Meta is the parsed frontmatter of a SKILL.md.
type Meta struct {
	Name        string
	Description string
	Fields      map[string]string
	Body        string
}

// ParseFrontmatter parses the minimal YAML subset skills use: top-level
// scalars and block scalars. ok is false when there is no frontmatter.
func ParseFrontmatter(text string) (fields map[string]string, body string, ok bool) {
	text = strings.TrimPrefix(text, "\ufeff")
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, text, false
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, text, false
	}
	fields = map[string]string{}
	for i := 1; i < end; {
		m := keyRe.FindStringSubmatch(lines[i])
		if m == nil {
			i++
			continue
		}
		key, val := m[1], strings.TrimSpace(m[2])
		switch val {
		case ">", "|", ">-", "|-":
			var parts []string
			i++
			for i < end && (strings.HasPrefix(lines[i], " ") || strings.TrimSpace(lines[i]) == "") {
				if t := strings.TrimSpace(lines[i]); t != "" {
					parts = append(parts, t)
				}
				i++
			}
			fields[key] = strings.Join(parts, " ")
			continue
		}
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		fields[key] = val
		i++
	}
	return fields, strings.Join(lines[end+1:], "\n"), true
}

// ValidName reports whether name is a valid skill name.
func ValidName(name string) bool {
	return len(name) <= 64 && nameRe.MatchString(name) && !util.IsWindowsReservedName(name)
}

// Read parses dir/SKILL.md and returns the metadata and any problems found.
func Read(dir string) (*Meta, []string) {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil, []string{"SKILL.md is missing"}
	}
	if !utf8.Valid(data) {
		return nil, []string{"SKILL.md is not valid UTF-8"}
	}
	fields, body, ok := ParseFrontmatter(string(data))
	if !ok {
		return nil, []string{"SKILL.md has no frontmatter"}
	}
	m := &Meta{Name: fields["name"], Description: fields["description"], Fields: fields, Body: body}
	var problems []string
	switch {
	case m.Name == "":
		problems = append(problems, "frontmatter 'name' is missing")
	case !ValidName(m.Name):
		problems = append(problems, "frontmatter name "+quote(m.Name)+" is not a valid skill name")
	}
	switch {
	case m.Description == "":
		problems = append(problems, "frontmatter 'description' is missing")
	case len(m.Description) > 1024:
		problems = append(problems, "description is longer than 1024 characters")
	}
	return m, problems
}

func quote(s string) string { return "\"" + strings.ReplaceAll(s, "\"", "'") + "\"" }

// Found is a skill located inside a tree.
type Found struct {
	Rel  string // slash-separated path relative to the tree root; "." for the root itself
	Meta *Meta
	Bad  []string
}

// Discover finds directories that contain a SKILL.md, without following symlinks.
func Discover(root string, maxDepth int) ([]Found, error) {
	var out []Found
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.Name() == ".git" || d.Name() == "node_modules" {
			return filepath.SkipDir
		}
		if rel != "." && strings.Count(rel, "/") >= maxDepth {
			return filepath.SkipDir
		}
		if info, err := os.Lstat(filepath.Join(p, "SKILL.md")); err == nil && info.Mode().IsRegular() {
			m, bad := Read(p)
			out = append(out, Found{Rel: rel, Meta: m, Bad: bad})
			return filepath.SkipDir
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, err
}
