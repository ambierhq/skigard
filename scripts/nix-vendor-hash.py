#!/usr/bin/env python3
"""Compute the buildGoModule vendorHash for this module WITHOUT nix.

usage: scripts/nix-vendor-hash.py [repo-root]     (needs `go` and network/GOMODCACHE for `go mod vendor`)
Runs `go mod vendor` in a scratch copy of go.mod/go.sum/sources, serialises the vendor directory as a NAR archive
and prints its SHA-256 in SRI form, which is what nixpkgs' `buildGoModule` fixed-output derivation hashes.
CI (nix.yml) rebuilds with nix and fails if this value is stale, so it cannot silently rot.
"""
import base64, hashlib, os, shutil, struct, subprocess, sys, tempfile


def pad(b):
    return b + b"\0" * (-len(b) % 8)


def s(x):
    b = x.encode() if isinstance(x, str) else x
    return struct.pack("<Q", len(b)) + pad(b)


def nar(path, h):
    st = os.lstat(path)
    h.update(s("("))
    if os.path.islink(path):
        h.update(s("type") + s("symlink") + s("target") + s(os.readlink(path)))
    elif os.path.isdir(path):
        h.update(s("type") + s("directory"))
        for name in sorted(os.listdir(path)):
            h.update(s("entry") + s("(") + s("name") + s(name) + s("node"))
            nar(os.path.join(path, name), h)
            h.update(s(")"))
    else:
        h.update(s("type") + s("regular"))
        if st.st_mode & 0o111:
            h.update(s("executable") + s(""))
        h.update(s("contents") + struct.pack("<Q", st.st_size))
        with open(path, "rb") as f:
            n = 0
            while chunk := f.read(1 << 20):
                h.update(chunk)
                n += len(chunk)
        h.update(b"\0" * (-n % 8))
    h.update(s(")"))


def main():
    root = os.path.abspath(sys.argv[1] if len(sys.argv) > 1 else os.path.join(os.path.dirname(__file__), ".."))
    with tempfile.TemporaryDirectory() as tmp:
        for item in ("go.mod", "go.sum", "cmd", "internal"):
            src = os.path.join(root, item)
            (shutil.copytree if os.path.isdir(src) else shutil.copy)(src, os.path.join(tmp, item))
        env = {**os.environ, "GOFLAGS": "-mod=mod", "GOTOOLCHAIN": "local"}
        subprocess.run(["go", "mod", "vendor"], cwd=tmp, env=env, check=True)
        h = hashlib.sha256()
        h.update(s("nix-archive-1"))
        nar(os.path.join(tmp, "vendor"), h)
        print("sha256-" + base64.b64encode(h.digest()).decode())


main()
