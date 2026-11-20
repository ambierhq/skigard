#!/bin/sh
# Render the winget manifests (schema 1.28) for a release from its checksums.txt.
#   scripts/winget-manifests.sh VERSION path/to/checksums.txt OUTDIR [RELEASE_DATE]
# VERSION has no leading "v". RELEASE_DATE is YYYY-MM-DD (optional; omitted from the manifest when empty).
# Writes OUTDIR/manifests/a/Ambier/Skigard/VERSION/Ambier.Skigard{,.installer,.locale.en-US}.yaml, the layout of
# microsoft/winget-pkgs. The installers are the release's Windows zip archives with a nested portable skigard.exe.
# The content matches what GoReleaser's winget publisher produced for v0.1.1 (microsoft/winget-pkgs#484409).
set -eu
{ [ $# -eq 3 ] || [ $# -eq 4 ]; } || { echo "usage: $0 VERSION CHECKSUMS_FILE OUTDIR [RELEASE_DATE]" >&2; exit 2; }
version="$1"; sums="$2"; out="$3"; date="${4:-}"
case "$version" in
  *[!0-9A-Za-z.+_-]*|"") echo "winget-manifests: invalid version '$version'" >&2; exit 2 ;;
esac
case "$date" in
  ""|[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]) ;;
  *) echo "winget-manifests: invalid release date '$date'" >&2; exit 2 ;;
esac
id=Ambier.Skigard
base="https://github.com/ambierhq/skigard/releases/download/v${version}"
# winget wants upper-case SHA-256.
sha() {
  v="$(awk -v f="skigard_${version}_windows_$1.zip" '$2 == f {print toupper($1)}' "$sums")"
  [ -n "$v" ] || { echo "winget-manifests: no checksum for windows_$1 in $sums" >&2; exit 1; }
  printf '%s' "$v"
}
sha_arm64="$(sha arm64)"
sha_amd64="$(sha amd64)"
dir="$out/manifests/a/Ambier/Skigard/$version"
mkdir -p "$dir"

cat > "$dir/$id.yaml" <<YAML
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.version.1.28.0.schema.json
PackageIdentifier: $id
PackageVersion: $version
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.28.0
YAML

{
  # shellcheck disable=SC2016 # the schema URL line is literal
  echo '# yaml-language-server: $schema=https://aka.ms/winget-manifest.installer.1.28.0.schema.json'
  echo "PackageIdentifier: $id"
  echo "PackageVersion: $version"
  echo "InstallerLocale: en-US"
  echo "InstallerType: zip"
  [ -z "$date" ] || echo "ReleaseDate: $date"
  echo "Installers:"
  for pair in "arm64 arm64 $sha_arm64" "x64 amd64 $sha_amd64"; do
    # shellcheck disable=SC2086 # intentional split into three fields
    set -- $pair
    cat <<YAML
  - Architecture: $1
    NestedInstallerType: portable
    NestedInstallerFiles:
      - RelativeFilePath: skigard.exe
        PortableCommandAlias: skigard
    InstallerUrl: $base/skigard_${version}_windows_$2.zip
    InstallerSha256: $3
    UpgradeBehavior: uninstallPrevious
YAML
  done
  echo "ManifestType: installer"
  echo "ManifestVersion: 1.28.0"
} > "$dir/$id.installer.yaml"

cat > "$dir/$id.locale.en-US.yaml" <<YAML
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.defaultLocale.1.28.0.schema.json
PackageIdentifier: $id
PackageVersion: $version
PackageLocale: en-US
Publisher: Ambier Oy
PublisherUrl: https://ambierhq.com
PublisherSupportUrl: https://github.com/ambierhq/skigard/issues
PackageName: skigard
PackageUrl: https://github.com/ambierhq/skigard
License: MIT
LicenseUrl: https://github.com/ambierhq/skigard/blob/main/LICENSE
ShortDescription: A safer way to manage agent skills
Moniker: skigard
ManifestType: defaultLocale
ManifestVersion: 1.28.0
YAML
echo "$dir"
