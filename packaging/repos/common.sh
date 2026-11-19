# shellcheck shell=bash
# Shared helpers for the package repository builders. Source it; do not run it.
#
# Signing key (all optional helpers read the environment, nothing is written to the repo):
#   GPG_PRIVATE_KEY   ASCII-armored private key to import into a throwaway GNUPGHOME (CI)
#   GPG_PASSPHRASE    its passphrase, if any
#   GPG_KEY_ID        fingerprint/keyid to sign with (default: the first secret key in the keyring)
#   GNUPGHOME         use an existing keyring instead of importing (local tests)
set -euo pipefail

die() { echo "repos: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "missing tool: $1"; }

# repos_setup_gpg: imports GPG_PRIVATE_KEY if set, resolves GPG_KEY_ID, exports GPG_PASS_FILE.
repos_setup_gpg() {
  need gpg
  if [ -n "${GPG_PRIVATE_KEY:-}" ]; then
    GNUPGHOME="$(mktemp -d)"; chmod 700 "$GNUPGHOME"; export GNUPGHOME
    printf '%s\n' "$GPG_PRIVATE_KEY" | gpg --batch --quiet --import
  fi
  if [ -z "${GPG_KEY_ID:-}" ]; then
    GPG_KEY_ID="$(gpg --batch --list-secret-keys --with-colons | awk -F: '$1=="fpr"{print $10; exit}')"
  fi
  [ -n "$GPG_KEY_ID" ] || die "no signing key: set GPG_PRIVATE_KEY or use a GNUPGHOME with a secret key"
  export GPG_KEY_ID
  GPG_PASS_FILE="$(mktemp)"; chmod 600 "$GPG_PASS_FILE"; printf '%s' "${GPG_PASSPHRASE:-}" > "$GPG_PASS_FILE"
  export GPG_PASS_FILE
}

# repos_gpg ARGS...: gpg in batch mode with the key and passphrase from repos_setup_gpg.
repos_gpg() {
  gpg --batch --yes --pinentry-mode loopback --passphrase-file "$GPG_PASS_FILE" --local-user "$GPG_KEY_ID" "$@"
}

repos_export_key() { # DEST: armored public key
  gpg --batch --armor --export "$GPG_KEY_ID" > "$1"
}

# repos_keep PKGDIR GLOB_SUFFIX: print package paths limited to the newest ${KEEP:-3} versions.
# Versions are taken from skigard_<version>_<os>_<arch>.<ext> (the GoReleaser file names).
repos_select() { # PKGDIR EXT
  local dir="$1" ext="$2" keep="${KEEP:-3}" versions
  versions="$(find "$dir" -maxdepth 1 -name "skigard_*_linux_*.$ext" -print |
    sed -E 's|.*/skigard_(.*)_linux_[a-z0-9]+\..*|\1|' | sort -uV | tail -n "$keep")"
  local v f
  for v in $versions; do
    for f in "$dir"/skigard_"${v}"_linux_*."$ext"; do [ -e "$f" ] && echo "$f"; done
  done
  return 0
}

# repos_check_limits DIR: Cloudflare Workers static assets allow 25 MiB per file and 20,000 files.
repos_check_limits() {
  local n big
  n="$(find "$1" -type f | wc -l | tr -d ' ')"
  [ "$n" -le 20000 ] || die "$n files exceeds the 20,000 file limit of Worker static assets"
  big="$(find "$1" -type f -size +25M -print | head -n 3)"
  [ -z "$big" ] || die "file(s) over 25 MiB (Worker static assets limit): $big"
}
