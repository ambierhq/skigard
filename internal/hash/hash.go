// Package hash computes skigard's content hash for a skill directory.
//
// Hash version 1: every regular file is framed as
//
//	"F\0" path "\0" mode "\0" length "\0" content
//
// where path is slash-separated and relative, files are sorted by the bytes of
// their path, and mode is "x" if any execute bit is set and "-" otherwise.
// Empty directories are ignored. Symlinks and special files are an error: a
// skill that contains them cannot be hashed (and so cannot be approved).
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// Version identifies the hashing scheme recorded in the lock file.
const Version = 1

// Prefix is prepended to every hash string.
const Prefix = "sha256:"

// Dir returns the content hash of root.
func Dir(root string) (string, error) {
	type entry struct {
		rel, abs string
		mode     fs.FileMode
		size     int64
	}
	var entries []entry
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			return fmt.Errorf("symlink not allowed in skill: %s", rel)
		case info.IsDir():
			return nil
		case !info.Mode().IsRegular():
			return fmt.Errorf("special file not allowed in skill: %s", rel)
		}
		entries = append(entries, entry{rel, p, info.Mode(), info.Size()})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })
	h := sha256.New()
	for _, e := range entries {
		mode := "-"
		if e.mode&0o111 != 0 {
			mode = "x"
		}
		f, err := os.Open(e.abs)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(h, "F\x00"+e.rel+"\x00"+mode+"\x00"+strconv.FormatInt(e.size, 10)+"\x00")
		n, err := io.Copy(h, f)
		_ = f.Close()
		if err != nil {
			return "", err
		}
		if n != e.size {
			return "", fmt.Errorf("file changed while hashing: %s", e.rel)
		}
	}
	return Prefix + hex.EncodeToString(h.Sum(nil)), nil
}

// Short returns the first n hex characters of a hash string.
func Short(h string, n int) string {
	if len(h) > len(Prefix) {
		h = h[len(Prefix):]
	}
	if len(h) > n {
		h = h[:n]
	}
	return h
}
