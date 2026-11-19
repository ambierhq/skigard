#!/bin/sh
# Render the Homebrew formula from a release's checksums.txt.
#   scripts/homebrew-formula.sh VERSION path/to/checksums.txt > skigard.rb
# VERSION has no leading "v".
set -eu
[ $# -eq 2 ] || { echo "usage: $0 VERSION CHECKSUMS_FILE" >&2; exit 2; }
version="$1"; sums="$2"
here="$(cd "$(dirname "$0")" && pwd)"
sha() {
  v="$(awk -v f="skigard_${version}_$1.tar.gz" '$2 == f {print $1}' "$sums")"
  [ -n "$v" ] || { echo "homebrew-formula: no checksum for $1 in $sums" >&2; exit 1; }
  printf '%s' "$v"
}
sed -e "s|@VERSION@|$version|g" \
  -e "s|@SHA_DARWIN_ARM64@|$(sha darwin_arm64)|" \
  -e "s|@SHA_DARWIN_AMD64@|$(sha darwin_amd64)|" \
  -e "s|@SHA_LINUX_ARM64@|$(sha linux_arm64)|" \
  -e "s|@SHA_LINUX_AMD64@|$(sha linux_amd64)|" \
  "$here/../packaging/homebrew/skigard.rb.tmpl"
