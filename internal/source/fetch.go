package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ambierhq/skigard/internal/safefs"
	"github.com/ambierhq/skigard/internal/util"
)

// Fetched is a source materialised on disk.
type Fetched struct {
	Dir          string // tree root: a 0700 temp dir, or the original directory for local sources
	temp         string // temp dir to remove on Cleanup ("" for local sources)
	Commit       string // resolved commit (GitHub) or empty (local)
	SkippedLinks []string
	Spec         Spec
}

// Cleanup removes the temp directory.
func (f *Fetched) Cleanup() {
	if f != nil && f.temp != "" {
		os.RemoveAll(f.temp)
	}
}

// Fetcher downloads sources. The zero value is not usable; call NewFetcher.
type Fetcher struct {
	APIBase      string
	CodeloadBase string
	Client       *http.Client
	Limits       safefs.Limits
	Token        string
}

// NewFetcher returns a hardened fetcher for github.com.
func NewFetcher() *Fetcher {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	return &Fetcher{
		APIBase:      "https://api.github.com",
		CodeloadBase: "https://codeload.github.com",
		Client:       HardenedClient(),
		Limits:       safefs.DefaultLimits(),
		Token:        token,
	}
}

var allowedHosts = map[string]bool{
	"api.github.com":      true,
	"codeload.github.com": true,
	"github.com":          true,
}

// PublicAddr reports whether an IP is a routable public address.
func PublicAddr(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
		a.IsMulticast() || a.IsUnspecified() || a.IsInterfaceLocalMulticast() {
		return false
	}
	if a.Is4() {
		b := a.As4()
		if b[0] == 100 && b[1]&0xc0 == 64 { // CGNAT 100.64.0.0/10
			return false
		}
	}
	return true
}

// CheckRedirect enforces https, a host allowlist and a redirect cap.
func CheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	if req.URL.Scheme != "https" || !allowedHosts[req.URL.Hostname()] {
		return fmt.Errorf("refusing redirect to %s", req.URL.Redacted())
	}
	return nil
}

// HardenedClient refuses private addresses (checked after DNS resolution, at
// connect time), non-https redirects and hosts outside github.com.
func HardenedClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 15 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil {
				return err
			}
			if !PublicAddr(ap.Addr()) {
				return fmt.Errorf("refusing to connect to non-public address %s", ap.Addr())
			}
			return nil
		},
	}
	return &http.Client{
		Timeout:       5 * time.Minute,
		CheckRedirect: CheckRedirect,
		Transport: &http.Transport{
			DialContext:         dialer.DialContext,
			TLSHandshakeTimeout: 15 * time.Second,
			Proxy:               nil,
		},
	}
}

func (f *Fetcher) get(ctx context.Context, url, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "skigard")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if f.Token != "" {
		req.Header.Set("Authorization", "Bearer "+f.Token)
	}
	return f.Client.Do(req)
}

// ResolveCommit turns a ref into a full commit SHA.
func (f *Fetcher) ResolveCommit(ctx context.Context, sp Spec) (string, error) {
	if IsCommit(sp.Ref) {
		return sp.Ref, nil
	}
	ref := sp.Ref
	if ref == "" {
		ref = "HEAD"
	}
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s", f.APIBase, sp.Owner, sp.Repo, ref)
	resp, err := f.get(ctx, url, "application/vnd.github.sha")
	if err != nil {
		return "", util.Errf("resolve %s: %v", sp, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return "", util.Errf("%s/%s@%s not found (private repository? set GITHUB_TOKEN)", sp.Owner, sp.Repo, ref)
	case http.StatusForbidden, http.StatusTooManyRequests:
		return "", util.Errf("GitHub refused the request (rate limit?); set GITHUB_TOKEN and retry")
	default:
		return "", util.Errf("resolve %s: HTTP %d", sp, resp.StatusCode)
	}
	sha := string(body)
	if !IsCommit(sha) {
		return "", util.Errf("resolve %s: unexpected response", sp)
	}
	return sha, nil
}

// Fetch materialises the source into a private temp directory.
func (f *Fetcher) Fetch(ctx context.Context, sp Spec) (*Fetched, error) {
	if sp.Kind == KindLocal {
		// Local sources are read in place; callers copy only the skill they select.
		info, err := os.Stat(sp.Path)
		if err != nil || !info.IsDir() {
			return nil, util.Errf("%s is not a directory", sp.Path)
		}
		return &Fetched{Dir: sp.Path, Spec: sp}, nil
	}
	tmp, err := os.MkdirTemp("", "skigard-fetch-")
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(tmp, "src")
	out := &Fetched{Dir: dir, temp: tmp, Spec: sp}
	fail := func(err error) (*Fetched, error) {
		os.RemoveAll(tmp)
		return nil, err
	}
	sha, err := f.ResolveCommit(ctx, sp)
	if err != nil {
		return fail(err)
	}
	out.Commit = sha
	url := fmt.Sprintf("%s/%s/%s/tar.gz/%s", f.CodeloadBase, sp.Owner, sp.Repo, sha)
	resp, err := f.get(ctx, url, "")
	if err != nil {
		return fail(util.Errf("download %s: %v", sp, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fail(util.Errf("download %s: HTTP %d", sp, resp.StatusCode))
	}
	if resp.ContentLength > f.Limits.MaxArchiveBytes {
		return fail(util.Errf("archive is %d bytes (limit %d)", resp.ContentLength, f.Limits.MaxArchiveBytes))
	}
	res, err := safefs.ExtractTarGz(resp.Body, dir, f.Limits)
	if err != nil {
		return fail(util.Errf("extract %s: %v", sp, err))
	}
	out.SkippedLinks = res.SkippedLinks
	return out, nil
}
