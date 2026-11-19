#!/usr/bin/env bash
# Print a .SRCINFO for a PKGBUILD without makepkg (which only exists on Arch). CI cross-checks this against
# `makepkg --printsrcinfo` in an archlinux container, so the two cannot drift.
#   packaging/aur/srcinfo.sh path/to/PKGBUILD > .SRCINFO
# Supports the fields our PKGBUILDs use: scalar fields, arrays and the _x86_64/_aarch64 variants of source/sha256sums.
# shellcheck disable=SC2154 # the fields come from the sourced PKGBUILD
set -euo pipefail
pkgbuild="${1:?usage: srcinfo.sh PKGBUILD}"
# Plain bash (3.2 compatible, no namerefs) so it also runs on macOS.
emit() { # KEY ARRAYNAME
  local n
  eval "n=\${#$2[@]}"
  [ "$n" -gt 0 ] || return 0
  eval "printf '\\t$1 = %s\\n' \"\${$2[@]}\""
}
# shellcheck disable=SC1090 # sourcing the PKGBUILD is the point; it is our own file, parsed in a subshell
(
  set +u
  . "$pkgbuild"
  : "${pkgrel:=1}"
  echo "pkgbase = ${pkgbase:-$pkgname}"
  printf '\tpkgdesc = %s\n\tpkgver = %s\n\tpkgrel = %s\n\turl = %s\n' "$pkgdesc" "$pkgver" "$pkgrel" "$url"
  emit arch arch; emit license license; emit makedepends makedepends; emit depends depends
  emit provides provides; emit conflicts conflicts; emit options options
  for sfx in "" _x86_64 _aarch64; do emit "source$sfx" "source$sfx"; done
  for sfx in "" _x86_64 _aarch64; do emit "sha256sums$sfx" "sha256sums$sfx"; done
  echo
  echo "pkgname = $pkgname"
)
