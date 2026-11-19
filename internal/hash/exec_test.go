package hash

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// goldenHashV2 pins testdata/golden-skill: LF, CRLF, binary and empty files, a
// nested tree and names that sort differently as "a-b" and "a/b". The fixture is
// covered by `-text` in .gitattributes, so this runs and passes unchanged on
// Linux, macOS and Windows runners. It does not depend on the execute bit.
const goldenHashV2 = "sha256:db210e5da6785e27bb6903921e9e49d2f30fbbbeb86a6310c21d51c353a77532"

func TestGoldenFixtureV2AcrossPlatforms(t *testing.T) {
	dir := filepath.Join("testdata", "golden-skill")
	got, err := Dir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != goldenHashV2 {
		t.Fatalf("golden v2 hash = %s, want %s (checkout rewrote bytes, or the scheme changed)", got, goldenHashV2)
	}
	set, err := ExecSet(dir)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if len(set) != 0 || ExecApplicable() {
			t.Fatalf("exec set must be not applicable on Windows, got %v", set)
		}
		return
	}
	// git records scripts/run.sh as 100755 and a unix checkout keeps it.
	if want := []string{"scripts/run.sh"}; !reflect.DeepEqual(set, want) {
		t.Fatalf("exec set = %v, want %v", set, want)
	}
}

func mkIdentity(t *testing.T, dir string) Identity {
	t.Helper()
	h, err := Dir(dir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := ExecSet(dir)
	if err != nil {
		t.Fatal(err)
	}
	return Identity{Version: 2, Content: h, Exec: set, ExecHash: ExecDigest(set)}
}

func TestCheckDetectsContentAndExecChanges(t *testing.T) {
	d := t.TempDir()
	write(t, d, "SKILL.md", "x", 0o644)
	write(t, d, "run.sh", "y", 0o755)
	id := mkIdentity(t, d)
	st, err := Check(d, id)
	if err != nil || !st.OK() {
		t.Fatalf("fresh dir should match: %+v %v", st, err)
	}
	if runtime.GOOS == "windows" {
		if !st.ExecNA {
			t.Fatal("exec must be reported not applicable on Windows")
		}
		return
	}
	// Tamper: drop the execute bit. Content hash is unchanged, the exec set is not.
	if err := os.Chmod(filepath.Join(d, "run.sh"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, _ = Check(d, id)
	if !st.ContentOK || st.ExecOK || st.OK() {
		t.Fatalf("lost exec bit not detected: %+v", st)
	}
	// Tamper: add an execute bit to another file.
	if err := os.Chmod(filepath.Join(d, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(d, "SKILL.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	st, _ = Check(d, id)
	if !st.ContentOK || st.ExecOK {
		t.Fatalf("added exec bit not detected: %+v", st)
	}
	// Tamper: edit exec_set in the entry but not its digest.
	if err := os.Chmod(filepath.Join(d, "SKILL.md"), 0o644); err != nil {
		t.Fatal(err)
	}
	forged := id
	forged.Exec = nil
	if st, _ = Check(d, forged); st.ExecOK {
		t.Fatal("exec_set not bound to exec_hash")
	}
	// Tamper: content.
	write(t, d, "SKILL.md", "X", 0o644)
	if st, _ = Check(d, id); st.ContentOK {
		t.Fatal("content change not detected")
	}
}

func TestCheckV1UsesModeInHash(t *testing.T) {
	d := t.TempDir()
	write(t, d, "run.sh", "y", 0o755)
	h, err := DirVersion(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	st, err := Check(d, Identity{Version: 1, Content: h})
	if err != nil || !st.OK() || !st.ExecNA {
		t.Fatalf("%+v %v", st, err)
	}
}

func TestExecDigestIsOrderAndDuplicateIndependent(t *testing.T) {
	if ExecDigest([]string{"b", "a"}) != ExecDigest([]string{"a", "b", "a"}) {
		t.Fatal("digest depends on order or duplicates")
	}
	if ExecDigest([]string{"a", "b"}) == ExecDigest([]string{"a"}) || ExecDigest([]string{"a"}) == ExecDigest(nil) {
		t.Fatal("digest ignores members")
	}
	// "a\0E\0b" must not collide with the two paths "a" and "b".
	if ExecDigest([]string{"a\x00E\x00b"}) == ExecDigest([]string{"a", "b"}) {
		t.Fatal("framing collision")
	}
}

func FuzzDirVersionDeterministic(f *testing.F) {
	f.Add("SKILL.md", "hello\n", "run.sh", "echo\n")
	f.Add("a", "", "b", "\x00\r\n")
	f.Fuzz(func(t *testing.T, n1, c1, n2, c2 string) {
		if !validFuzzName(n1) || !validFuzzName(n2) || n1 == n2 {
			t.Skip()
		}
		mk := func(order [2]int) string {
			d := t.TempDir()
			files := [2][2]string{{n1, c1}, {n2, c2}}
			for _, i := range order {
				if err := os.WriteFile(filepath.Join(d, files[i][0]), []byte(files[i][1]), 0o644); err != nil {
					t.Skip()
				}
			}
			h, err := Dir(d)
			if err != nil {
				t.Skip()
			}
			return h
		}
		if mk([2]int{0, 1}) != mk([2]int{1, 0}) {
			t.Fatal("hash depends on creation order")
		}
	})
}

func validFuzzName(n string) bool {
	if n == "" || len(n) > 32 {
		return false
	}
	for _, r := range n {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
