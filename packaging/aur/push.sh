#!/usr/bin/env bash
# Push rendered AUR packages (from update.sh) to aur.archlinux.org. NEVER run this by hand without the owner's go.
#   AUR_SSH_KEY=path/to/private/key packaging/aur/push.sh OUTDIR VERSION
# Refuses to run without AUR_SSH_KEY. The key must belong to the single AUR maintainer account.
set -euo pipefail
[ $# -eq 2 ] || { echo "usage: AUR_SSH_KEY=file $0 OUTDIR VERSION" >&2; exit 2; }
out="$1"; version="$2"
[ -n "${AUR_SSH_KEY:-}" ] && [ -f "$AUR_SSH_KEY" ] || { echo "push: AUR_SSH_KEY (path to the private key) is required" >&2; exit 1; }
export GIT_SSH_COMMAND="ssh -i $AUR_SSH_KEY -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=${AUR_KNOWN_HOSTS:-$HOME/.ssh/known_hosts_aur}"
work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
for p in skigard skigard-bin; do
  git clone --quiet "ssh://aur@aur.archlinux.org/$p.git" "$work/$p"
  cp "$out/$p/PKGBUILD" "$out/$p/.SRCINFO" "$work/$p/"
  git -C "$work/$p" add PKGBUILD .SRCINFO
  if git -C "$work/$p" diff --cached --quiet; then echo "$p: already up to date"; continue; fi
  git -C "$work/$p" -c user.name="Ambier release bot" -c user.email="skigard@ambierhq.com" commit --quiet -m "upgpkg: $p $version-1"
  git -C "$work/$p" push --quiet origin HEAD:master
  echo "$p: pushed $version"
done
