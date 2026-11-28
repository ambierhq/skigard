# Install skigard

skigard is one static binary (no runtime, no telemetry). Pick your system:

- [macOS](macos.md)
- [Linux](linux.md)
- [Windows](windows.md)
- [Other channels: npx, Nix, mise, Go](other.md)

## Channels and status

`v0.4.0` is the latest release. Status of each channel:

| Channel | Command | Platforms | Status |
|---|---|---|---|
| Go | `go install github.com/ambierhq/skigard/cmd/skigard@latest` | all (Go 1.27+) | live (`go install ...@v0.4.0` prints `skigard 0.4.0`) |
| Shell installer | `curl -fsSL https://ambierhq.com/skigard/install.sh \| sh` | macOS, Linux | live |
| PowerShell installer | `irm https://ambierhq.com/skigard/install.ps1 \| iex` | Windows | live (tested in CI on `windows-latest`; not tested by hand on Windows) |
| GitHub Releases | archives, `.deb`, `.rpm`, `.apk`, Arch packages, checksums, SBOMs, cosign signatures | all | live |
| Homebrew | `brew install ambierhq/tap/skigard` | macOS, Linux | live |
| Scoop | `scoop bucket add ambierhq https://github.com/ambierhq/scoop-bucket` then `scoop install skigard` | Windows | live (updated by the release workflow); not tested by hand on Windows |
| mise | `mise use -g github:ambierhq/skigard` | all | live (`github:` backend, checked with v0.1.2; the short name `skigard` needs a registry entry, not submitted) |
| Nix | `nix run github:ambierhq/skigard` | Linux, macOS | the flake builds in CI (Linux and macOS), not tested by hand; a nixpkgs package (`pkgs.skigard`) is submitted and in review ([nixpkgs#590787](https://github.com/NixOS/nixpkgs/pull/590787)), not available yet |
| npx / npm | `npx skigard`, `npm i -g skigard` | all | available (v0.4.0, published by the release workflow with npm provenance) |
| apt, dnf, pacman repositories | not offered; install the `.deb`, `.rpm` or `.pkg.tar.zst` attached to a GitHub Release ([Linux](linux.md)) | Debian, Ubuntu, Fedora, RHEL, Arch | not offered |

Channel details: [npm](npm.md), [Nix](nix.md), [mise](mise.md).
Maintainer view (secrets, switches, registry PRs): [packaging/README.md](../../packaging/README.md).

## Verify what you downloaded

Every release carries `checksums.txt` (SHA-256), a keyless [cosign](https://github.com/sigstore/cosign) signature for it, and an SBOM per archive.
Build provenance attestations (GitHub) are published for every release since `v0.1.1`; check one with `gh attestation verify skigard_0.4.0_linux_amd64.tar.gz --repo ambierhq/skigard`. The npm packages carry npm provenance (SLSA) from `v0.2.0` on (`v0.1.1` was published by hand, without it).
`install.sh` always checks the archive against `checksums.txt`, and also verifies the signature when `cosign` is installed. The installers never use
`sudo` and install to `~/.local/bin` (override with `SKIGARD_INSTALL_DIR`; pin a release with `SKIGARD_VERSION=v0.4.0`).

Download `checksums.txt` and `checksums.txt.sigstore.json` from the release, then (replace `v0.4.0` with the release you downloaded; the
identity names the release workflow at that tag):

```sh
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/ambierhq/skigard/\.github/workflows/release\.yml@refs/tags/v0\.1\.2$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
grep ' skigard_0.4.0_linux_amd64.tar.gz$' checksums.txt | sha256sum --check   # then the archive you downloaded (macOS: shasum -a 256 -c)
```

`Verified OK` means the checksums were produced by `.github/workflows/release.yml` of `ambierhq/skigard` at that tag, in GitHub Actions.
Each archive also has its own `<archive>.sigstore.json` bundle, verified the same way with `verify-blob`.

## Check it works

```sh
skigard version
skigard doctor
```

Then continue with [Getting started](../getting-started.md). Uninstall: remove the binary (and `~/.config/skigard` if you want to drop your
approvals). To remove only what skigard installed into agents, run `skigard uninstall` first.
