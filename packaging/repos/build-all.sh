#!/usr/bin/env bash
# Build the static tree served at https://releases.ambierhq.com (Cloudflare Worker `ambier-releases`, static assets).
#   packaging/repos/build-all.sh PKGDIR OUTDIR [apt] [rpm] [arch]     (default: all three)
# PKGDIR holds GoReleaser's .deb/.rpm/.pkg.tar.zst files for the versions to publish (the newest ${KEEP:-3} are kept).
# The arch step needs repo-add, so in CI it runs in an archlinux container: call this script once per format.
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=packaging/repos/common.sh
. "$here/common.sh"
[ $# -ge 2 ] || die "usage: $0 PKGDIR OUTDIR [apt] [rpm] [arch]"
pkgdir="$1"; out="$2"; shift 2
[ $# -gt 0 ] || set -- apt rpm arch
mkdir -p "$out"
for f in "$@"; do
  case "$f" in apt | rpm | arch) "$here/build-$f.sh" "$pkgdir" "$out" ;; *) die "unknown format $f" ;; esac
done
cat > "$out/index.html" <<'HTML'
<!doctype html><meta charset="utf-8"><title>Ambier package repositories</title>
<p>Package repositories for Ambier tools: <a href="https://github.com/ambierhq/skigard/tree/main/docs/install">install instructions</a>.
HTML
repos_check_limits "$out"
