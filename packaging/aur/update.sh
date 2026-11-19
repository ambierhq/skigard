#!/usr/bin/env bash
# Render the AUR packages (skigard, skigard-bin) for a release into OUTDIR. Writes files only; see push.sh to publish.
#   packaging/aur/update.sh VERSION CHECKSUMS_FILE OUTDIR [SOURCE_TARBALL]
# VERSION has no leading "v". CHECKSUMS_FILE is the release's checksums.txt. The source tarball
# (https://github.com/ambierhq/skigard/archive/refs/tags/vVERSION.tar.gz) is downloaded unless given.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
[ $# -ge 3 ] || { echo "usage: $0 VERSION CHECKSUMS_FILE OUTDIR [SOURCE_TARBALL]" >&2; exit 2; }
version="$1"; sums="$2"; out="$3"; tarball="${4:-}"
case "$version" in [0-9]*.[0-9]*.[0-9]*) ;; *) echo "update: bad version $version" >&2; exit 2 ;; esac
case "$version" in *-*) echo "update: pre-release $version is not published to the AUR" >&2; exit 2 ;; esac

sha_of() { awk -v f="$1" '$2 == f {print $1}' "$sums"; }
sha_file() { if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }

if [ -z "$tarball" ]; then
  tarball="$(mktemp)"; trap 'rm -f "$tarball"' EXIT
  curl -fsSL -o "$tarball" "https://github.com/ambierhq/skigard/archive/refs/tags/v${version}.tar.gz"
fi
src_sha="$(sha_file "$tarball")"
amd="$(sha_of "skigard_${version}_linux_amd64.tar.gz")"; arm="$(sha_of "skigard_${version}_linux_arm64.tar.gz")"
[ -n "$amd" ] && [ -n "$arm" ] || { echo "update: linux checksums for $version missing in $sums" >&2; exit 1; }

rm -rf "$out"; mkdir -p "$out/skigard" "$out/skigard-bin"
sed -e "s|^pkgver=.*|pkgver=$version|" -e "s|^pkgrel=.*|pkgrel=1|" -e "s|^sha256sums=.*|sha256sums=('$src_sha')|" \
  "$here/../arch/PKGBUILD" > "$out/skigard/PKGBUILD"
sed -e "s|^pkgver=.*|pkgver=$version|" -e "s|^pkgrel=.*|pkgrel=1|" \
  -e "s|^sha256sums_x86_64=.*|sha256sums_x86_64=('$amd')|" -e "s|^sha256sums_aarch64=.*|sha256sums_aarch64=('$arm')|" \
  "$here/skigard-bin/PKGBUILD" > "$out/skigard-bin/PKGBUILD"
for p in skigard skigard-bin; do "$here/srcinfo.sh" "$out/$p/PKGBUILD" > "$out/$p/.SRCINFO"; done
echo "rendered skigard and skigard-bin $version into $out"
