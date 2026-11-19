#!/usr/bin/env bash
# Build a signed rpm repository (one per arch, for $basearch) from the .rpm files in PKGDIR.
#   packaging/repos/build-rpm.sh PKGDIR OUTDIR     (OUTDIR/rpm/{x86_64,aarch64} are created)
# Needs: rpm (rpmsign), createrepo_c, gpg. Packages and repomd.xml are both signed.
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=packaging/repos/common.sh
. "$here/common.sh"
[ $# -eq 2 ] || die "usage: $0 PKGDIR OUTDIR"
pkgdir="$1"; rpmdir="$2/rpm"
need rpmsign; need createrepo_c
repos_setup_gpg

rm -rf "$rpmdir"; mkdir -p "$rpmdir"
n=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  arch="$(rpm -qp --qf '%{ARCH}' "$f" 2>/dev/null)"
  case "$arch" in x86_64 | aarch64) ;; *) die "unexpected rpm arch $arch in $f" ;; esac
  mkdir -p "$rpmdir/$arch"
  cp "$f" "$rpmdir/$arch/$(rpm -qp --qf '%{NAME}-%{VERSION}-%{RELEASE}.%{ARCH}.rpm' "$f" 2>/dev/null)"; n=$((n + 1))
done < <(repos_select "$pkgdir" rpm)
[ "$n" -gt 0 ] || die "no .rpm files in $pkgdir"

for d in "$rpmdir"/*/; do
  rpmsign --addsign \
    --define "_gpg_name $GPG_KEY_ID" \
    --define "__gpg $(command -v gpg)" \
    --define "_gpg_sign_cmd_extra_args --batch --pinentry-mode loopback --passphrase-file $GPG_PASS_FILE" \
    "$d"*.rpm >/dev/null
  createrepo_c --quiet "$d"
  repos_gpg --armor --detach-sign --output "$d/repodata/repomd.xml.asc" "$d/repodata/repomd.xml"
done
repos_export_key "$rpmdir/key.gpg"
echo "rpm: $n package(s) -> $rpmdir"
