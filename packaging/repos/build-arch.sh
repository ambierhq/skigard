#!/usr/bin/env bash
# Build a signed pacman repository "ambierhq" from the .pkg.tar.zst files in PKGDIR.
#   packaging/repos/build-arch.sh PKGDIR OUTDIR    (OUTDIR/arch/{x86_64,aarch64} are created)
# Needs repo-add (pacman-contrib/pacman) and gpg, so run it in an archlinux container (the workflow does).
# Packages and the databases get detached .sig files. Symlinks are replaced by copies
# because Worker static assets do not follow them.
here="$(cd "$(dirname "$0")" && pwd)"; . "$here/common.sh"
[ $# -eq 2 ] || die "usage: $0 PKGDIR OUTDIR"
pkgdir="$1"; arch="$2/arch"
need repo-add; need bsdtar
repos_setup_gpg

rm -rf "$arch"; mkdir -p "$arch"
n=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  info="$(bsdtar -xOf "$f" .PKGINFO)"
  name="$(sed -n 's/^pkgname = //p' <<<"$info")"; ver="$(sed -n 's/^pkgver = //p' <<<"$info")"; a="$(sed -n 's/^arch = //p' <<<"$info")"
  case "$a" in x86_64 | aarch64) ;; *) die "unexpected pacman arch $a in $f" ;; esac
  mkdir -p "$arch/$a"
  dst="$arch/$a/$name-$ver-$a.pkg.tar.zst"
  cp "$f" "$dst"
  repos_gpg --detach-sign --output "$dst.sig" "$dst"
  (cd "$arch/$a" && repo-add --quiet ambierhq.db.tar.zst "$(basename "$dst")")
  n=$((n + 1))
done < <(repos_select "$pkgdir" pkg.tar.zst)
[ "$n" -gt 0 ] || die "no .pkg.tar.zst files in $pkgdir"

for d in "$arch"/*/; do
  # repo-add --sign cannot take a passphrase, so sign the databases ourselves (same detached .sig files).
  for db in ambierhq.db.tar.zst ambierhq.files.tar.zst; do repos_gpg --detach-sign --output "$d$db.sig" "$d$db"; cp "$d$db.sig" "$d${db%.tar.zst}.sig"; done
  for l in "$d"*; do
    [ -L "$l" ] || continue
    t="$(readlink -f "$l")"; rm "$l"; cp "$t" "$l"
  done
done
repos_export_key "$arch/key.gpg"
echo "arch: $n package(s) -> $arch"
