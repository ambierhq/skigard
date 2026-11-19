package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
)

// ExecApplicable reports whether this OS can represent execute bits. On
// Windows it cannot, so exec sets are not checked there.
func ExecApplicable() bool { return runtime.GOOS != "windows" }

// ExecSet returns the sorted slash-separated paths of the regular files under
// root that have any execute bit set. It reads the local filesystem, so it is
// only meaningful where ExecApplicable is true; elsewhere it returns nil.
func ExecSet(root string) ([]string, error) {
	var out []string
	if !ExecApplicable() {
		return out, nil
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || p == root {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// ExecDigest hashes an exec set: each path, sorted by bytes and de-duplicated,
// is framed as "E\0" length "\0" path "\0". The empty set has a digest too, so a lock entry
// can always say "no executable files" explicitly.
func ExecDigest(set []string) string {
	s := append([]string(nil), set...)
	sort.Strings(s)
	h := sha256.New()
	prev := ""
	for i, p := range s {
		if i > 0 && p == prev {
			continue
		}
		prev = p
		_, _ = h.Write([]byte("E\x00" + strconv.Itoa(len(p)) + "\x00" + p + "\x00"))
	}
	return Prefix + hex.EncodeToString(h.Sum(nil))
}

// Identity is what a lock entry (or quarantine record) pins about a skill.
type Identity struct {
	Version  int
	Content  string   // content hash under Version
	Exec     []string // version 2 only: execute-bit paths captured at fetch time
	ExecHash string   // version 2 only: ExecDigest(Exec)
}

// Status is the outcome of Check.
type Status struct {
	Got       string   // content hash computed from the directory
	ContentOK bool     // Got equals Identity.Content
	ExecOK    bool     // exec set matches (or does not apply)
	ExecNA    bool     // exec set not checked: version 1 (mode is in the hash) or Windows
	ExecGot   []string // exec set found on disk when it was checked
}

// OK reports whether the directory matches the identity completely.
func (s Status) OK() bool { return s.ContentOK && s.ExecOK }

// Check compares dir with id. An error means the directory could not be read
// (a symlink inside it, for example), never a mismatch.
func Check(dir string, id Identity) (Status, error) {
	got, err := DirVersion(dir, id.Version)
	if err != nil {
		return Status{}, err
	}
	st := Status{Got: got, ContentOK: got == id.Content, ExecOK: true}
	if id.Version < 2 || !ExecApplicable() {
		st.ExecNA = true
		return st, nil
	}
	set, err := ExecSet(dir)
	if err != nil {
		return Status{}, err
	}
	st.ExecGot = set
	st.ExecOK = ExecDigest(set) == id.ExecHash && ExecDigest(id.Exec) == id.ExecHash
	return st, nil
}
