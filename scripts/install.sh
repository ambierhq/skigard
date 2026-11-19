#!/bin/sh
# Install skigard from a GitHub release. Never uses sudo.
#   curl -fsSL https://raw.githubusercontent.com/ambierhq/skigard/main/scripts/install.sh | sh
#
# Environment:
#   SKIGARD_VERSION      release to install, with or without a leading "v" (default: latest)
#   SKIGARD_INSTALL_DIR  where to put the binary (default: ~/.local/bin)
#   SKIGARD_BASE_URL     override the download location (used by tests)
#   SKIGARD_SKIP_COSIGN  set to 1 to skip the cosign signature check even if cosign is installed
#
# The archive is always checked against checksums.txt (SHA-256). If `cosign` is installed, checksums.txt
# is also verified against its keyless signature from this repository's release workflow.
set -eu

repo="ambierhq/skigard"
version="${SKIGARD_VERSION:-latest}"

die() { echo "skigard-install: $*" >&2; exit 1; }
say() { echo "skigard-install: $*"; }

if [ -n "${SKIGARD_INSTALL_DIR:-}" ]; then
  dir="$SKIGARD_INSTALL_DIR"
elif [ -n "${HOME:-}" ]; then
  dir="$HOME/.local/bin"
else
  die "HOME is not set; set SKIGARD_INSTALL_DIR"
fi

case "$version" in
  latest) ;;
  *[!0-9A-Za-z.+_-]*|"") die "invalid SKIGARD_VERSION '$version'" ;;
  v*) ;;
  *) version="v$version" ;;
esac

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  darwin|linux) ;;
  *) die "unsupported OS '$os' (on Windows use install.ps1; or: go install github.com/$repo/cmd/skigard@latest)" ;;
esac
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) die "unsupported CPU '$arch' (supported: amd64, arm64)" ;;
esac

if [ -n "${SKIGARD_BASE_URL:-}" ]; then
  base="${SKIGARD_BASE_URL%/}"
elif [ "$version" = latest ]; then
  base="https://github.com/$repo/releases/latest/download"
else
  base="https://github.com/$repo/releases/download/$version"
fi

if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL --retry 3 -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -q -O "$2" "$1"; }
else
  die "need curl or wget"
fi
download() { # URL DEST
  fetch "$1" "$2" || die "download failed: $1 (is '$version' a published release with a ${os}/${arch} build?)"
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then openssl dgst -sha256 "$1" | awk '{print $NF}'
  else die "need sha256sum, shasum or openssl to verify the download"; fi
}

tmp="$(mktemp -d)" || die "cannot create a temporary directory"
trap 'rm -rf "$tmp"' EXIT INT TERM

download "$base/checksums.txt" "$tmp/checksums.txt"

if [ "${SKIGARD_SKIP_COSIGN:-0}" != 1 ] && command -v cosign >/dev/null 2>&1; then
  download "$base/checksums.txt.sigstore.json" "$tmp/checksums.txt.sigstore.json"
  cosign verify-blob \
    --bundle "$tmp/checksums.txt.sigstore.json" \
    --certificate-identity-regexp "^https://github.com/$repo/\\.github/workflows/release\\.yml@refs/tags/v" \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com \
    "$tmp/checksums.txt" >/dev/null 2>"$tmp/cosign.err" \
    || { cat "$tmp/cosign.err" >&2; die "cosign signature verification of checksums.txt failed"; }
  say "cosign signature verified"
fi

asset="$(awk -v suffix="_${os}_${arch}.tar.gz" 'index($2, suffix) && substr($2, length($2) - length(suffix) + 1) == suffix {print $2; exit}' "$tmp/checksums.txt")"
[ -n "$asset" ] || die "no release asset for ${os}/${arch} in checksums.txt"
case "$asset" in */*|*..*) die "unexpected asset name '$asset'" ;; esac
download "$base/$asset" "$tmp/$asset"

want="$(awk -v f="$asset" '$2 == f {print $1; exit}' "$tmp/checksums.txt")"
got="$(sha256 "$tmp/$asset")"
[ -n "$want" ] && [ "$want" = "$got" ] || die "checksum mismatch for $asset (want '$want', got '$got')"

mkdir "$tmp/x"
tar -xzf "$tmp/$asset" -C "$tmp/x" skigard || die "could not extract skigard from $asset"
mkdir -p "$dir" 2>/dev/null || true
[ -d "$dir" ] && [ -w "$dir" ] || die "cannot write to $dir (set SKIGARD_INSTALL_DIR to a directory you own; this script never uses sudo)"
install -m 0755 "$tmp/x/skigard" "$dir/skigard" || die "could not install to $dir/skigard"

say "installed to $dir/skigard"
case ":${PATH:-}:" in *":$dir:"*) ;; *) say "add $dir to your PATH" ;; esac
"$dir/skigard" version
