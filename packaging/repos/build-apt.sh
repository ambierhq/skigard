#!/usr/bin/env bash
# Build a signed apt repository (flat "stable main" suite) from the .deb files in PKGDIR.
#   packaging/repos/build-apt.sh PKGDIR OUTDIR     (OUTDIR/apt is created)
# Needs: dpkg-deb, dpkg-scanpackages (dpkg-dev), gzip, sha256sum or shasum, gpg.
# Layout: apt/pool/main/s/skigard/*.deb, apt/dists/stable/{Release,Release.gpg,InRelease,main/binary-*/Packages*}, apt/key.gpg
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=packaging/repos/common.sh
. "$here/common.sh"
[ $# -eq 2 ] || die "usage: $0 PKGDIR OUTDIR"
pkgdir="$1"; apt="$2/apt"
need dpkg-deb; need dpkg-scanpackages; need gzip
sha() { if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }
repos_setup_gpg

rm -rf "$apt"; pool="$apt/pool/main/s/skigard"; mkdir -p "$pool"
n=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  ver="$(dpkg-deb -f "$f" Version)"; arch="$(dpkg-deb -f "$f" Architecture)"
  cp "$f" "$pool/skigard_${ver}_${arch}.deb"; n=$((n + 1))
done < <(repos_select "$pkgdir" deb)
[ "$n" -gt 0 ] || die "no .deb files in $pkgdir"

suite="$apt/dists/stable"; archs=""
for arch in amd64 arm64; do
  ls "$pool"/*_"$arch".deb >/dev/null 2>&1 || continue
  archs="$archs $arch"
  mkdir -p "$suite/main/binary-$arch"
  (cd "$apt" && dpkg-scanpackages --arch "$arch" pool/ 2>/dev/null) > "$suite/main/binary-$arch/Packages"
  gzip -9nc "$suite/main/binary-$arch/Packages" > "$suite/main/binary-$arch/Packages.gz"
done
archs="${archs# }"

{
  echo "Origin: Ambier"; echo "Label: Ambier"; echo "Suite: stable"; echo "Codename: stable"
  echo "Date: $(LC_ALL=C date -u '+%a, %d %b %Y %H:%M:%S UTC')"
  echo "Architectures: $archs"; echo "Components: main"
  echo "Description: Ambier Oy packages (skigard)"
  echo "SHA256:"
  (cd "$suite" && find main -type f | LC_ALL=C sort) | while IFS= read -r p; do
    printf ' %s %16d %s\n' "$(sha "$suite/$p")" "$(wc -c < "$suite/$p" | tr -d ' ')" "$p"
  done
} > "$suite/Release"
repos_gpg --armor --detach-sign --output "$suite/Release.gpg" "$suite/Release"
repos_gpg --clearsign --output "$suite/InRelease" "$suite/Release"
repos_export_key "$apt/key.gpg"
echo "apt: $n package(s), architectures: $archs -> $apt"
