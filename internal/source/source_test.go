package source

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/ambierhq/skigard/internal/safefs"
)

func TestParse(t *testing.T) {
	cases := map[string]Spec{
		"github:tt-a1i/archify":                       {Kind: KindGitHub, Owner: "tt-a1i", Repo: "archify"},
		"tt-a1i/archify/archify@v1":                   {Kind: KindGitHub, Owner: "tt-a1i", Repo: "archify", Subpath: "archify", Ref: "v1"},
		"https://github.com/o/r.git":                  {Kind: KindGitHub, Owner: "o", Repo: "r"},
		"https://github.com/o/r/tree/main/skills/foo": {Kind: KindGitHub, Owner: "o", Repo: "r", Ref: "main", Subpath: "skills/foo"},
	}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "ssh://github.com/o/r", "git@github.com:o/r.git", "github:o", "github:o/r/../x", "github:o/r@-x", "just-a-word", "https://evil.com/o/r"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) should fail", bad)
		}
	}
	l, err := Parse("./x")
	if err != nil || l.Kind != KindLocal || !filepath.IsAbs(l.Path) {
		t.Errorf("local: %+v %v", l, err)
	}
	if s := (Spec{Kind: KindGitHub, Owner: "o", Repo: "r", Subpath: "a/b", Ref: "v1"}).String(); s != "github:o/r/a/b@v1" {
		t.Errorf("String = %s", s)
	}
}

func TestPublicAddr(t *testing.T) {
	for ip, want := range map[string]bool{
		"8.8.8.8": true, "140.82.112.3": true, "127.0.0.1": false, "10.0.0.1": false, "192.168.1.1": false,
		"172.16.0.1": false, "169.254.169.254": false, "::1": false, "fe80::1": false, "0.0.0.0": false,
		"100.64.0.1": false, "::ffff:127.0.0.1": false, "224.0.0.1": false,
	} {
		if got := PublicAddr(netip.MustParseAddr(ip)); got != want {
			t.Errorf("PublicAddr(%s) = %v", ip, got)
		}
	}
}

func TestCheckRedirect(t *testing.T) {
	mk := func(u string) *http.Request { r, _ := http.NewRequest("GET", u, nil); return r }
	if err := CheckRedirect(mk("https://codeload.github.com/x"), nil); err != nil {
		t.Error(err)
	}
	for _, u := range []string{"http://codeload.github.com/x", "https://evil.example/x", "https://169.254.169.254/"} {
		if CheckRedirect(mk(u), nil) == nil {
			t.Errorf("redirect to %s allowed", u)
		}
	}
	if CheckRedirect(mk("https://github.com/x"), make([]*http.Request, 5)) == nil {
		t.Error("redirect cap not enforced")
	}
}

func tarGz(files map[string]string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))})
		tw.Write([]byte(body))
	}
	tw.WriteHeader(&tar.Header{Name: "repo-sha/evil", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"})
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestFetchGitHub(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/repos/o/r/commits/HEAD":
			w.Write([]byte(sha))
		case "/repos/o/r/commits/missing":
			http.NotFound(w, r)
		case "/o/r/tar.gz/" + sha:
			w.Write(tarGz(map[string]string{"repo-sha/skills/a/SKILL.md": "---\nname: a\ndescription: d\n---\n"}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	f := &Fetcher{APIBase: srv.URL, CodeloadBase: srv.URL, Client: srv.Client(), Limits: safefs.DefaultLimits(), Token: "tok"}
	got, err := f.Fetch(context.Background(), Spec{Kind: KindGitHub, Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatal(err)
	}
	defer got.Cleanup()
	if got.Commit != sha || len(got.SkippedLinks) != 1 {
		t.Fatalf("%+v", got)
	}
	if _, err := os.Stat(filepath.Join(got.Dir, "skills", "a", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if _, err := f.Fetch(context.Background(), Spec{Kind: KindGitHub, Owner: "o", Repo: "r", Ref: "missing"}); err == nil {
		t.Error("expected not-found error")
	}
	// a commit SHA ref must not hit the API
	if c, err := f.ResolveCommit(context.Background(), Spec{Kind: KindGitHub, Owner: "o", Repo: "r", Ref: sha}); err != nil || c != sha {
		t.Errorf("ResolveCommit(sha) = %q, %v", c, err)
	}
}

func TestFetchLocal(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("x"), 0o644)
	f := NewFetcher()
	got, err := f.Fetch(context.Background(), Spec{Kind: KindLocal, Path: src})
	if err != nil {
		t.Fatal(err)
	}
	defer got.Cleanup()
	if _, err := os.Stat(filepath.Join(got.Dir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Fetch(context.Background(), Spec{Kind: KindLocal, Path: filepath.Join(src, "nope")}); err == nil {
		t.Error("expected error for missing dir")
	}
}
