// Package safefs copies and extracts untrusted trees without ever following
// symlinks or escaping the destination. All writes go through os.Root, so a
// hostile path cannot leave the destination directory.
package safefs

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ambierhq/skigard/internal/util"
)

// Limits bounds the size of anything we ingest.
type Limits struct {
	MaxArchiveBytes int64 // compressed
	MaxTotalBytes   int64 // extracted / copied
	MaxFileBytes    int64
	MaxEntries      int
	MaxDepth        int
	MaxRatio        int64 // extracted / compressed
}

// DefaultLimits are conservative defaults.
func DefaultLimits() Limits {
	return Limits{
		MaxArchiveBytes: 64 << 20,
		MaxTotalBytes:   128 << 20,
		MaxFileBytes:    8 << 20,
		MaxEntries:      10000,
		MaxDepth:        32,
		MaxRatio:        300,
	}
}

// Result summarises an ingest.
type Result struct {
	Files        int
	Bytes        int64
	SkippedLinks []string // symlinks/hardlinks that were not extracted
	TopDir       string
}

var skipNames = map[string]bool{".git": true, ".DS_Store": true}

// CopyTree copies the regular files of src into the new directory dst.
// Symlinks and special files are an error. ".git" and ".DS_Store" are skipped.
func CopyTree(src, dst string, lim Limits) (*Result, error) {
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dst)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	srcRoot, err := os.OpenRoot(src)
	if err != nil {
		return nil, err
	}
	defer srcRoot.Close()
	res := &Result{}
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if p == src {
			return nil
		}
		if skipNames[d.Name()] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(src, p)
		rel = filepath.ToSlash(rel)
		if strings.Count(rel, "/") >= lim.MaxDepth {
			return fmt.Errorf("path too deep: %s", rel)
		}
		if runtime.GOOS == "windows" {
			// Windows names that exist on disk can still be hostile (\\?\ paths, short names).
			if err := util.CheckPortablePath(rel); err != nil {
				return fmt.Errorf("unsafe file name %q: %w", rel, err)
			}
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0:
			// ModeIrregular is how Go reports Windows junctions (mount points) and other reparse points.
			return fmt.Errorf("symlink or junction not allowed: %s", rel)
		case info.IsDir():
			return root.MkdirAll(rel, 0o755)
		case !info.Mode().IsRegular():
			return fmt.Errorf("special file not allowed: %s", rel)
		}
		res.Files++
		if res.Files > lim.MaxEntries {
			return fmt.Errorf("too many files (limit %d)", lim.MaxEntries)
		}
		if info.Size() > lim.MaxFileBytes {
			return fmt.Errorf("file too large (%d bytes, limit %d): %s", info.Size(), lim.MaxFileBytes, rel)
		}
		res.Bytes += info.Size()
		if res.Bytes > lim.MaxTotalBytes {
			return fmt.Errorf("tree too large (limit %d bytes)", lim.MaxTotalBytes)
		}
		in, err := srcRoot.Open(rel) // rooted: a path swapped for a symlink cannot escape src
		if err != nil {
			return err
		}
		defer in.Close()
		return writeFile(root, rel, in, info.Size(), info.Mode())
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func writeFile(root *os.Root, rel string, r io.Reader, size int64, mode fs.FileMode) error {
	perm := fs.FileMode(0o644)
	if mode&0o111 != 0 {
		perm = 0o755
	}
	if dir := path.Dir(rel); dir != "." {
		if err := root.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	out, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(r, size+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("file size changed while copying: %s", rel)
	}
	return root.Chmod(rel, perm)
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// ExtractTarGz extracts a gzip'd tarball into dst, stripping the single
// top-level directory GitHub archives add. Only directories and regular files
// are written; symlinks and hardlinks are recorded in Result.SkippedLinks.
func ExtractTarGz(r io.Reader, dst string, lim Limits) (*Result, error) {
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dst)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	limited := io.LimitReader(r, lim.MaxArchiveBytes+1)
	cr := &countingReader{r: limited}
	gz, err := gzip.NewReader(cr)
	if err != nil {
		return nil, fmt.Errorf("not a gzip archive: %w", err)
	}
	tr := tar.NewReader(gz)
	res := &Result{}
	seen := map[string]string{} // folded -> original
	entries := 0
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if cr.n > lim.MaxArchiveBytes {
				return nil, fmt.Errorf("archive exceeds %d bytes", lim.MaxArchiveBytes)
			}
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		entries++
		if entries > lim.MaxEntries {
			return nil, fmt.Errorf("archive has too many entries (limit %d)", lim.MaxEntries)
		}
		rel, top, err := cleanEntry(hdr.Name)
		if err != nil {
			return nil, err
		}
		if res.TopDir == "" {
			res.TopDir = top
		} else if top != res.TopDir {
			return nil, fmt.Errorf("archive has multiple top-level directories (%q, %q)", res.TopDir, top)
		}
		if rel == "" {
			continue
		}
		if strings.Count(rel, "/") >= lim.MaxDepth {
			return nil, fmt.Errorf("path too deep: %s", rel)
		}
		if err := recordFolded(seen, rel); err != nil {
			return nil, err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(rel, 0o755); err != nil {
				return nil, err
			}
		case tar.TypeReg:
			if hdr.Size > lim.MaxFileBytes {
				return nil, fmt.Errorf("file too large (%d bytes, limit %d): %s", hdr.Size, lim.MaxFileBytes, rel)
			}
			res.Bytes += hdr.Size
			if res.Bytes > lim.MaxTotalBytes {
				return nil, fmt.Errorf("archive expands beyond %d bytes", lim.MaxTotalBytes)
			}
			if res.Bytes > 1<<20 && cr.n > 0 && res.Bytes/cr.n > lim.MaxRatio {
				return nil, errors.New("archive compression ratio is suspicious")
			}
			res.Files++
			if err := writeFile(root, rel, tr, hdr.Size, fs.FileMode(uint32(hdr.Mode&0o7777))); err != nil {
				return nil, err
			}
		case tar.TypeSymlink, tar.TypeLink:
			res.SkippedLinks = append(res.SkippedLinks, rel)
		default:
			return nil, fmt.Errorf("unsupported archive entry type %q: %s", string(hdr.Typeflag), rel)
		}
	}
	if cr.n > lim.MaxArchiveBytes {
		return nil, fmt.Errorf("archive exceeds %d bytes", lim.MaxArchiveBytes)
	}
	return res, nil
}

// recordFolded notes rel and each of its parent directories in seen (folded ->
// original spelling) and fails when two entries differ only by case, which
// would be the same file on Windows and on default macOS volumes.
func recordFolded(seen map[string]string, rel string) error {
	parts := strings.Split(rel, "/")
	for i := range parts {
		p := strings.Join(parts[:i+1], "/")
		fold := strings.ToLower(p)
		if prev, ok := seen[fold]; ok && prev != p {
			return fmt.Errorf("case-insensitive path collision: %q and %q", prev, p)
		}
		seen[fold] = p
	}
	return nil
}

// cleanEntry validates an archive path and splits off the top-level directory.
func cleanEntry(name string) (rel, top string, err error) {
	if name == "" || strings.ContainsRune(name, 0) || strings.Contains(name, `\`) {
		return "", "", fmt.Errorf("unsafe archive path %q", name)
	}
	if strings.HasPrefix(name, "/") {
		return "", "", fmt.Errorf("absolute archive path %q", name)
	}
	for _, part := range strings.Split(strings.TrimSuffix(name, "/"), "/") {
		if part == ".." {
			return "", "", fmt.Errorf("path traversal in archive: %q", name)
		}
		// Reject names Windows treats specially, on every platform, so an archive
		// behaves the same wherever it is extracted (see util.CheckPortableComponent).
		if part != "" && part != "." {
			if err := util.CheckPortableComponent(part); err != nil {
				return "", "", fmt.Errorf("unsafe archive path %q: %w", name, err)
			}
		}
	}
	clean := path.Clean(name)
	parts := strings.SplitN(clean, "/", 2)
	top = parts[0]
	if len(parts) == 2 {
		rel = parts[1]
	}
	if top == "." || top == "" {
		return "", "", fmt.Errorf("unsafe archive path %q", name)
	}
	return rel, top, nil
}

// RemoveAll removes a path without following symlinks out of it.
func RemoveAll(p string) error { return os.RemoveAll(p) }
