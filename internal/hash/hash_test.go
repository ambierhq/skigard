package hash

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func write(t *testing.T, root, rel, content string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	os.Chmod(p, mode)
}

// Test vector: pins hash version 1 (legacy, includes the execute bit) so the scheme cannot change silently.
func TestVectorV1(t *testing.T) {
	d := t.TempDir()
	write(t, d, "SKILL.md", "hello\n", 0o644)
	write(t, d, "scripts/run.sh", "echo hi\n", 0o755)
	got, err := DirVersion(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := "sha256:b1a18ee21c6d18ec9d1472a89236ace9501b5c187a48bd24f643988b611e90c9"
	if runtime.GOOS == "windows" {
		// Windows files carry no execute bit, so run.sh is hashed with mode "-".
		want = "sha256:8b96a0fa86430a7ce70cde994a67d8d24c43c467a5fb5e5de5bf3b962697701e"
	}
	if want != got {
		t.Fatalf("v1 hash vector changed: got %s want %s", got, want)
	}
}

// Test vector: pins hash version 2. The value is the same on every OS and was
// cross-checked against an independent Python implementation of the framing.
func TestVectorV2(t *testing.T) {
	d := t.TempDir()
	write(t, d, "SKILL.md", "hello\n", 0o644)
	write(t, d, "scripts/run.sh", "echo hi\n", 0o755)
	got, err := Dir(d)
	if err != nil {
		t.Fatal(err)
	}
	const want = "sha256:1dbfa1cf29367d433069f3615ffd1b94d245220fcb439a97ee65dee17f879e2d"
	if got != want {
		t.Fatalf("v2 hash vector changed: got %s want %s", got, want)
	}
	if again, _ := Dir(d); again != got {
		t.Fatal("hash not deterministic")
	}
	// The exec digest of {scripts/run.sh} and of the empty set are pinned too.
	if g := ExecDigest([]string{"scripts/run.sh"}); g != "sha256:388b536eaed60046b2f7db41cac7551cb02775de97fccda838553ef39dd81316" {
		t.Fatalf("exec digest vector changed: %s", g)
	}
	if g := ExecDigest(nil); g != "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("empty exec digest changed: %s", g)
	}
}

func TestV2IgnoresModeV1DoesNot(t *testing.T) {
	mk := func(mode os.FileMode, v int) string {
		d := t.TempDir()
		write(t, d, "f", "a", mode)
		h, err := DirVersion(d, v)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	if mk(0o644, 2) != mk(0o755, 2) || mk(0o644, 2) != mk(0o600, 2) || mk(0o644, 2) != mk(0o700, 2) {
		t.Fatal("v2 content hash depends on file mode")
	}
	if runtime.GOOS != "windows" && mk(0o644, 1) == mk(0o755, 1) {
		t.Fatal("v1 hash should still depend on the execute bit")
	}
}

func TestSensitiveToContentLengthNameAndOrder(t *testing.T) {
	mk := func(files map[string]string) string {
		d := t.TempDir()
		for n, c := range files {
			write(t, d, n, c, 0o644)
		}
		h, err := Dir(d)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	base := mk(map[string]string{"a": "xy", "b": "z"})
	for name, files := range map[string]map[string]string{
		"one byte":       {"a": "xY", "b": "z"},
		"name":           {"a": "xy", "c": "z"},
		"length":         {"a": "xyz", "b": ""}, // same concatenated content, different split
		"content swap":   {"a": "z", "b": "xy"}, // same bytes, different order
		"extra file":     {"a": "xy", "b": "z", "c": ""},
		"missing file":   {"a": "xy"},
		"prefix framing": {"a": "xy", "b": "z", "bz": ""},
		"nested":         {"a": "xy", "b/": ""}, // placeholder replaced below
	} {
		if name == "nested" {
			files = map[string]string{"a": "xy", "b/b": "z"}
		}
		if mk(files) == base {
			t.Errorf("hash did not change: %s", name)
		}
	}
}

func TestSensitiveToContentAndPath(t *testing.T) {
	mk := func(content, name string) string {
		d := t.TempDir()
		write(t, d, name, content, 0o644)
		h, err := Dir(d)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	base := mk("a", "f")
	if mk("b", "f") == base || mk("a", "g") == base {
		t.Error("hash did not change with content or path")
	}
}

func TestOrderIndependentOfCreation(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	write(t, a, "a-b", "1", 0o644)
	write(t, a, "a/b", "2", 0o644)
	write(t, b, "a/b", "2", 0o644)
	write(t, b, "a-b", "1", 0o644)
	ha, _ := Dir(a)
	hb, _ := Dir(b)
	if ha != hb {
		t.Fatal("order matters")
	}
}

func TestRejectsSymlink(t *testing.T) {
	d := t.TempDir()
	write(t, d, "f", "x", 0o644)
	if err := os.Symlink("f", filepath.Join(d, "l")); err != nil {
		t.Skip(err)
	}
	if _, err := Dir(d); err == nil {
		t.Fatal("expected symlink error")
	}
}
