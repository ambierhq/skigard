// Package util holds small helpers shared across skigard: user-facing errors,
// name validation, terminal-safe output and atomic file writes.
package util

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Exit codes returned by the CLI.
const (
	ExitOK         = 0
	ExitPolicy     = 1 // findings, drift, refused by policy
	ExitUsage      = 2 // usage or runtime error
	ExitCancelled  = 3
	ExitIncomplete = 4 // coverage incomplete
)

// Error is a user-facing error carrying an exit code.
type Error struct {
	Msg  string
	Code int
}

func (e *Error) Error() string { return e.Msg }

// Errf returns a user error with ExitUsage.
func Errf(format string, a ...any) error {
	return &Error{Msg: fmt.Sprintf(format, a...), Code: ExitUsage}
}

// Policyf returns a user error with ExitPolicy.
func Policyf(format string, a ...any) error {
	return &Error{Msg: fmt.Sprintf(format, a...), Code: ExitPolicy}
}

// Cancelled returns a cancellation error.
func Cancelled(msg string) error { return &Error{Msg: msg, Code: ExitCancelled} }

// CodeOf maps an error to an exit code.
func CodeOf(err error) int {
	if err == nil {
		return ExitOK
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ExitUsage
}

// Now returns the current UTC time in RFC 3339 form.
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

var nameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// CheckName validates a skill or plugin name.
func CheckName(name string) error {
	if len(name) > 64 || !nameRe.MatchString(name) {
		return Errf("invalid name %q: use lowercase letters, digits and single hyphens (max 64 chars)", name)
	}
	if IsWindowsReservedName(name) {
		return Errf("invalid name %q: it is a reserved Windows device name and cannot be a directory there", name)
	}
	return nil
}

// Clean makes untrusted text safe for a terminal: control characters (including
// ESC) and bidi/zero-width characters are replaced by visible escapes.
func Clean(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r == utf8.RuneError || r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
			fmt.Fprintf(&b, "\\x%02x", r&0xff)
		case unicode.Is(unicode.Cf, r) || (r >= 0xe0000 && r <= 0xe007f):
			fmt.Fprintf(&b, "\\u{%x}", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// OneLine is Clean with newlines collapsed, truncated to max runes.
func OneLine(s string, max int) string {
	s = strings.Join(strings.Fields(strings.ReplaceAll(s, "\t", " ")), " ")
	s = Clean(s)
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "..."
	}
	return s
}

var secretRes = []*regexp.Regexp{
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{30,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*["']?[A-Za-z0-9/+_\-]{16,}`),
}

// MaskSecrets hides credential-looking substrings so reports never leak them.
func MaskSecrets(s string) string {
	for _, re := range secretRes {
		s = re.ReplaceAllStringFunc(s, func(m string) string {
			if len(m) <= 8 {
				return "****"
			}
			return m[:4] + "****"
		})
	}
	return s
}

// AtomicWrite writes data to path via a temp file in the same directory.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// RandomHex returns n random bytes as hex.
func RandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// MakeReadOnly removes the write bits of a tree: directories become 0555,
// regular files 0444, or 0555 when they are executable (so the execute-bit set
// that content hash v2 records is unchanged). It is a deterrent against editing
// an approved skill in place, not a security boundary: the lock hash is. It does
// nothing on Windows, where permission bits are synthetic. Symlinks are skipped.
func MakeReadOnly(root string) error {
	return chmodTree(root, func(d fs.DirEntry) (os.FileMode, bool) {
		switch {
		case d.IsDir():
			return 0o555, true
		case d.Type().IsRegular():
			if fi, err := d.Info(); err == nil && fi.Mode().Perm()&0o111 != 0 {
				return 0o555, true
			}
			return 0o444, true
		}
		return 0, false
	})
}

// MakeWritable undoes MakeReadOnly on a tree so it can be removed or replaced
// (directories 0755, files gain the owner write bit). Errors are ignored: the
// caller's next operation reports the real problem.
func MakeWritable(root string) {
	_ = chmodTree(root, func(d fs.DirEntry) (os.FileMode, bool) {
		switch {
		case d.IsDir():
			return 0o755, true
		case d.Type().IsRegular():
			if fi, err := d.Info(); err == nil {
				return fi.Mode().Perm() | 0o200, true
			}
		}
		return 0, false
	})
}

// chmodTree applies the mode pick returns to every directory and regular file
// under root, through an os.Root so a symlink swapped in mid-walk cannot redirect it.
func chmodTree(root string, pick func(fs.DirEntry) (os.FileMode, bool)) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer r.Close()
	return fs.WalkDir(r.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if mode, ok := pick(d); ok {
			return r.Chmod(p, mode)
		}
		return nil
	})
}

// RemoveAll is os.RemoveAll that also works on a read-only tree.
func RemoveAll(p string) error {
	if err := os.RemoveAll(p); err == nil {
		return nil
	}
	MakeWritable(p)
	return os.RemoveAll(p)
}
