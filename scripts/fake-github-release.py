#!/usr/bin/env python3
"""Serve a local directory of release assets through a minimal GitHub REST API, for testing installers offline.

usage: scripts/fake-github-release.py DIST VERSION PORT [OWNER/REPO]
DIST is a GoReleaser output dir. Files named skigard_<anything>_<os>_<arch>.<ext> are exposed as release v<VERSION>,
renamed to skigard_<VERSION>_<os>_<arch>.<ext> (and checksums.txt is regenerated), so a snapshot dist can stand in
for a real release. Point tools at it with, for example, mise's `api_url=http://127.0.0.1:PORT`.
"""
import hashlib, json, os, re, shutil, sys, tempfile
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

dist, version, port = sys.argv[1], sys.argv[2], int(sys.argv[3])
repo = sys.argv[4] if len(sys.argv) > 4 else "ambierhq/skigard"
work = tempfile.mkdtemp()
base = f"http://127.0.0.1:{port}"
sums = []
for f in sorted(os.listdir(dist)):
    m = re.fullmatch(r"skigard_[^_]+_(linux|darwin|windows)_(amd64|arm64)\.(tar\.gz|zip)", f)
    if not m:
        continue
    name = f"skigard_{version}_{m[1]}_{m[2]}.{m[3]}"
    shutil.copy(os.path.join(dist, f), os.path.join(work, name))
    sums.append(f"{hashlib.sha256(open(os.path.join(work, name), 'rb').read()).hexdigest()}  {name}")
open(os.path.join(work, "checksums.txt"), "w").write("\n".join(sums) + "\n")
assets = sorted(os.listdir(work))


def asset(i, a):
    path = os.path.join(work, a)
    return {
        "id": i + 10,
        "name": a,
        "size": os.path.getsize(path),
        "content_type": "application/octet-stream",
        "state": "uploaded",
        "digest": "sha256:" + hashlib.sha256(open(path, "rb").read()).hexdigest(),
        "browser_download_url": f"{base}/download/{a}",
        "url": f"{base}/api/v3/repos/{repo}/releases/assets/{i + 10}",
    }


def release():
    return {
        "id": 1, "tag_name": f"v{version}", "name": f"v{version}", "draft": False, "prerelease": False,
        "created_at": "2026-01-01T00:00:00Z", "published_at": "2026-01-01T00:00:00Z",
        "html_url": f"https://github.com/{repo}/releases/tag/v{version}",
        "assets": [asset(i, a) for i, a in enumerate(assets)],
    }


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        sys.stderr.write("fake-github: " + (a[0] % a[1:]) + "\n")

    def send(self, code, body, ctype):
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        p = self.path.split("?")[0]
        for prefix in ("/api/v3", ""):
            q = p[len(prefix):] if p.startswith(prefix) else None
            if q is None:
                continue
            if q == f"/repos/{repo}/releases/latest" or q == f"/repos/{repo}/releases/tags/v{version}":
                return self.send(200, json.dumps(release()).encode(), "application/json")
            if q == f"/repos/{repo}/releases":
                return self.send(200, json.dumps([release()]).encode(), "application/json")
            m = re.fullmatch(rf"/repos/{repo}/releases/assets/(\d+)", q)
            if m and 10 <= int(m[1]) < 10 + len(assets):
                a = assets[int(m[1]) - 10]
                return self.send(200, open(os.path.join(work, a), "rb").read(), "application/octet-stream")
        if p.startswith("/download/") and os.path.basename(p) in assets:
            return self.send(200, open(os.path.join(work, os.path.basename(p)), "rb").read(), "application/octet-stream")
        self.send(404, b"{}", "application/json")


print(f"fake GitHub API for {repo} v{version} on {base}", flush=True)
ThreadingHTTPServer(("127.0.0.1", port), H).serve_forever()
