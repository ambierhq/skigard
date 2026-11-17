package scan

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ambierhq/skigard/internal/hash"
	"github.com/ambierhq/skigard/internal/skill"
	"github.com/ambierhq/skigard/internal/util"
)

const (
	maxTextBytes  = 2 << 20
	maxTotalBytes = 64 << 20
	maxFiles      = 10000
	perRuleCap    = 5
	safetyClaim   = "not-proven-safe"
)

var scriptExt = map[string]bool{".sh": true, ".bash": true, ".zsh": true, ".py": true, ".js": true, ".mjs": true,
	".cjs": true, ".ts": true, ".rb": true, ".pl": true, ".ps1": true, ".bat": true, ".php": true, ".lua": true}

var assetExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true,
	".bmp": true, ".pdf": true, ".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".mp3": true, ".mp4": true}

var textExt = map[string]bool{".md": true, ".markdown": true, ".txt": true, ".json": true, ".yaml": true, ".yml": true,
	".toml": true, ".html": true, ".htm": true, ".css": true, ".svg": true, ".xml": true, ".mjs": true, ".jsx": true,
	".tsx": true, ".sh": true, ".bash": true, ".zsh": true, ".py": true, ".js": true, ".cjs": true, ".ts": true,
	".rb": true, ".pl": true, ".go": true, ".rs": true, ".lua": true, ".php": true}

var docExt = map[string]bool{".md": true, ".markdown": true, ".txt": true, ".rst": true}

var urlRe = regexp.MustCompile(`https?://[^\s)>"'\x60]+`)
var broadToolsRe = regexp.MustCompile(`\bBash\b([^(]|$)`)

type state struct {
	rep    *Report
	counts map[string]int
	caps   map[string]bool
	more   map[string]int
}

func (s *state) add(id, file string, line int, msg, evidence string, sev Severity, name, family string) {
	k := id + "\x00" + file
	s.counts[k]++
	if s.counts[k] > perRuleCap {
		s.more[id]++
		return
	}
	ev := util.OneLine(util.MaskSecrets(evidence), 160)
	s.rep.Findings = append(s.rep.Findings, Finding{
		Rule: id, Name: name, Family: family, Severity: sev, File: file, Line: line,
		Message: msg, Evidence: ev, Key: findingKey(id, file, ev),
	})
}

func (s *state) addStructural(id, file string, line int, msg, evidence string) {
	d := structural[id]
	s.add(id, file, line, msg, evidence, d.Sev, d.Name, d.Family)
}

// Scan inspects the skill directory root.
func Scan(root string, opts Options) (*Report, error) {
	rep := &Report{
		SchemaVersion: SchemaVersion, Tool: "skigard " + opts.ToolVersion, RulesVersion: RulesVersion,
		Skill: opts.Skill, Coverage: CoverageComplete, Safety: safetyClaim,
	}
	st := &state{rep: rep, counts: map[string]int{}, caps: map[string]bool{}, more: map[string]int{}}

	if _, problems := skill.Read(root); len(problems) > 0 {
		for _, p := range problems {
			st.addStructural("SKG-MANIFEST-001", "SKILL.md", 0, p, "")
		}
	}
	for _, l := range opts.SkippedLinks {
		st.addStructural("SKG-FS-007", l, 0, "symlink in the source was not fetched", l)
	}

	var total int64
	nfiles := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if p == root || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			target, _ := os.Readlink(p)
			st.addStructural("SKG-FS-007", rel, 0, "symlink in skill: "+util.Clean(target), "")
			rep.Files = append(rep.Files, FileInfo{Path: rel, Kind: "link"})
			rep.Coverage = CoveragePartial
			return nil
		}
		nfiles++
		if nfiles > maxFiles {
			if nfiles == maxFiles+1 {
				st.addStructural("SKG-FS-008", "", 0, "more files than the scanner will inspect", "")
				rep.Coverage = CoveragePartial
			}
			return nil
		}
		fi := FileInfo{Path: rel, Size: info.Size(), Kind: "text"}
		ext := strings.ToLower(filepath.Ext(rel))
		if scriptExt[ext] {
			fi.Script = true
			st.addStructural("SKG-FS-005", rel, 0, "bundled script: review its code", "")
		}
		if info.Mode()&0o111 != 0 {
			st.addStructural("SKG-FS-004", rel, 0, "file has the executable bit set", "")
		}
		total += info.Size()
		if info.Size() > maxTextBytes || total > maxTotalBytes {
			fi.Kind = "large"
			st.addStructural("SKG-FS-003", rel, 0, "file too large to inspect", "")
			rep.Coverage = CoveragePartial
			rep.Files = append(rep.Files, fi)
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		nulInText := textExt[ext] && utf8.Valid(data) && bytes.IndexByte(data, 0) >= 0 && binaryKind(data) == "binary"
		if nulInText {
			// Source files occasionally contain NULs legitimately; flag it, but still read the text.
			st.addStructural("SKG-OBF-005", rel, 0, "NUL bytes inside a text file (can hide content from reviewers)", "")
		}
		if kind := binaryKind(data); kind != "" && !nulInText {
			fi.Kind = kind
			switch {
			case kind == "exec":
				st.addStructural("SKG-FS-002", rel, 0, "native executable binary", "")
				rep.Coverage = CoveragePartial
			case assetExt[ext]:
				fi.Kind = "asset"
				st.addStructural("SKG-FS-006", rel, 0, "binary asset, contents not inspected", "")
			default:
				st.addStructural("SKG-FS-001", rel, 0, "binary or non-UTF-8 file cannot be reviewed", "")
				rep.Coverage = CoveragePartial
			}
			rep.Files = append(rep.Files, fi)
			return nil
		}
		fi.Inspected = true
		rep.Files = append(rep.Files, fi)
		st.scanText(rel, ext, string(data))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(rep.Files, func(i, j int) bool { return rep.Files[i].Path < rep.Files[j].Path })

	for id, n := range st.more {
		rep.Notes = append(rep.Notes, id+": "+itoa(n)+" further matches not listed")
	}
	sort.Strings(rep.Notes)
	st.chains()
	if h, err := hash.Dir(root); err == nil {
		rep.Fingerprint = h
	} else {
		rep.Notes = append(rep.Notes, "fingerprint unavailable: "+err.Error())
		rep.Coverage = CoveragePartial
	}
	finalize(rep, st, opts)
	return rep, nil
}

func itoa(n int) string { return strconv.Itoa(n) }

func binaryKind(data []byte) string {
	head := data
	if len(head) > 8000 {
		head = head[:8000]
	}
	if len(head) >= 4 {
		m := head[:4]
		if bytes.Equal(m, []byte("\x7fELF")) || bytes.Equal(m, []byte{0xfe, 0xed, 0xfa, 0xce}) ||
			bytes.Equal(m, []byte{0xfe, 0xed, 0xfa, 0xcf}) || bytes.Equal(m, []byte{0xce, 0xfa, 0xed, 0xfe}) ||
			bytes.Equal(m, []byte{0xcf, 0xfa, 0xed, 0xfe}) || bytes.Equal(m, []byte{0xca, 0xfe, 0xba, 0xbe}) ||
			bytes.HasPrefix(head, []byte("MZ")) {
			return "exec"
		}
	}
	if bytes.IndexByte(head, 0) >= 0 || !utf8.Valid(data) {
		return "binary"
	}
	return ""
}

func invisible(r rune) bool {
	switch {
	case r >= 0x200b && r <= 0x200f, r >= 0x202a && r <= 0x202e, r >= 0x2060 && r <= 0x2064,
		r >= 0x2066 && r <= 0x2069, r >= 0xe0000 && r <= 0xe007f, r == 0x00ad, r == 0x180e:
		return true
	}
	return false
}

func (s *state) scanText(rel, ext, text string) {
	isDoc := docExt[ext]
	isMD := ext == ".md" || ext == ".markdown" || ext == ".html" || ext == ".htm"
	inFence := false
	lines := strings.Split(text, "\n")
	for i, raw := range lines {
		n := i + 1
		line := strings.TrimSuffix(raw, "\r")
		if i == 0 {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		t := strings.TrimSpace(line)
		if isDoc && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")) {
			inFence = !inFence
		}
		for _, r := range line {
			if invisible(r) {
				s.addStructural("SKG-OBF-001", rel, n, "zero-width, bidi or tag characters", line)
				break
			}
		}
		prose := isDoc && !inFence
		lower := strings.ToLower(line)
		for _, r := range lineRules {
			if r.MarkdownOnly && !isMD {
				continue
			}
			if !r.maybe(lower) || !r.Re.MatchString(line) {
				continue
			}
			sev := r.Sev
			downgraded := false
			if r.DocLow && prose {
				sev, downgraded = Low, true
			}
			if !downgraded {
				for _, c := range r.Caps {
					s.caps[c] = true
				}
			}
			s.add(r.ID, rel, n, r.Msg, line, sev, r.Name, r.Family)
		}
		for _, m := range urlRe.FindAllString(line, 1) {
			if !strings.Contains(m, "//localhost") && !strings.Contains(m, "//127.0.0.1") {
				s.addStructural("SKG-NET-003", rel, n, "external URL", m)
			}
		}
	}
	if rel == "SKILL.md" {
		if fm, _, ok := skill.ParseFrontmatter(text); ok {
			if tools := fm["allowed-tools"]; tools != "" && (broadToolsRe.MatchString(tools) || strings.Contains(tools, "*")) {
				s.addStructural("SKG-MANIFEST-002", rel, 0, "allowed-tools grants broad access: "+util.OneLine(tools, 80), "")
			}
		}
	}
}

func (s *state) chains() {
	if s.caps["sensitive_read"] && s.caps["network_egress"] {
		s.addStructural("SKG-CHAIN-001", "", 0, "reads credential locations and also has network access (possible exfiltration)", "")
	}
	if s.caps["persistence"] && s.caps["dynamic_exec"] {
		s.addStructural("SKG-CHAIN-002", "", 0, "installs persistence and executes dynamic code", "")
	}
}

var weights = map[Severity]int{Critical: 40, High: 20, Medium: 5, Low: 1}

func finalize(rep *Report, st *state, opts Options) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	accepted := map[string]Accepted{}
	for _, a := range opts.Accepted {
		if a.Expires != "" {
			if t, err := time.Parse("2006-01-02", a.Expires); err == nil && now.After(t.Add(24*time.Hour)) {
				continue
			}
			if t, err := time.Parse(time.RFC3339, a.Expires); err == nil && now.After(t) {
				continue
			}
		}
		accepted[a.Key] = a
	}
	var active []Finding
	for _, f := range rep.Findings {
		if _, ok := accepted[f.Key]; ok && f.Severity.Rank() <= Medium.Rank() {
			rep.Suppressed = append(rep.Suppressed, f)
			continue
		}
		active = append(active, f)
	}
	sort.SliceStable(active, func(i, j int) bool {
		a, b := active[i], active[j]
		if a.Severity.Rank() != b.Severity.Rank() {
			return a.Severity.Rank() > b.Severity.Rank()
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	rep.Findings = active
	max := 0
	risk := 0
	perRule := map[string]int{}
	for _, f := range active {
		if f.Severity.Rank() > max {
			max = f.Severity.Rank()
		}
		if perRule[f.Rule] < 3 {
			risk += weights[f.Severity]
		}
		perRule[f.Rule]++
	}
	if risk > 100 {
		risk = 100
	}
	rep.Risk = risk
	switch {
	case max >= High.Rank():
		rep.Verdict = VerdictBlock
	case max >= Medium.Rank() || rep.Coverage != CoverageComplete:
		rep.Verdict = VerdictReview
	default:
		rep.Verdict = VerdictPass
	}
	for c := range st.caps {
		rep.Capabilities = append(rep.Capabilities, c)
	}
	sort.Strings(rep.Capabilities)
	if rep.Capabilities == nil {
		rep.Capabilities = []string{}
	}
	if rep.Findings == nil {
		rep.Findings = []Finding{}
	}
	if rep.Files == nil {
		rep.Files = []FileInfo{}
	}
}
