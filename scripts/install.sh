#!/bin/sh
# Install skigard from a GitHub release.
#   curl -fsSL https://raw.githubusercontent.com/ambierhq/skigard/main/scripts/install.sh | sh
# Environment: SKIGARD_VERSION (default: latest), SKIGARD_INSTALL_DIR (default: ~/.local/bin),
#              SKIGARD_BASE_URL (override the download location; used by tests).
set -eu

repo="ambierhq/skigard"
version="${SKIGARD_VERSION:-latest}"
dir="${SKIGARD_INSTALL_DIR:-$HOME/.local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in darwin|linux) ;; *) echo "skigard: unsupported OS $os (try: go install github.com/$repo/cmd/skigard@latest)" >&2; exit 1;; esac
arch="$(uname -m)"
case "$arch" in x86_64|amd64) arch=amd64;; arm64|aarch64) arch=arm64;; *) echo "skigard: unsupported CPU $arch" >&2; exit 1;; esac

if [ -n "${SKIGARD_BASE_URL:-}" ]; then
  base="$SKIGARD_BASE_URL"
elif [ "$version" = latest ]; then
  base="https://github.com/$repo/releases/latest/download"
else
  base="https://github.com/$repo/releases/download/$version"
fi

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"
asset="$(grep "_${os}_${arch}.tar.gz\$" "$tmp/checksums.txt" | head -n1 | awk '{print $2}')"
[ -n "$asset" ] || { echo "skigard: no release asset for ${os}/${arch}" >&2; exit 1; }
curl -fsSL "$base/$asset" -o "$tmp/$asset"

want="$(grep " $asset\$" "$tmp/checksums.txt" | awk '{print $1}')"
if command -v sha256sum >/dev/null 2>&1; then got="$(sha256sum "$tmp/$asset" | awk '{print $1}')"
else got="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"; fi
[ "$want" = "$got" ] || { echo "skigard: checksum mismatch for $asset" >&2; exit 1; }

tar -xzf "$tmp/$asset" -C "$tmp" skigard
mkdir -p "$dir"
install -m 0755 "$tmp/skigard" "$dir/skigard"
echo "skigard installed to $dir/skigard"
case ":$PATH:" in *":$dir:"*) ;; *) echo "add $dir to your PATH";; esac
"$dir/skigard" version
