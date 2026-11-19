# Install skigard

skigard is one static binary (no runtime, no telemetry). Pick your system:

- [macOS](macos.md)
- [Linux](linux.md)
- [Windows](windows.md)
- [Other channels: npx, Nix, Docker, mise, Go](other.md)

## Channels and status

The first release (`v0.1.0`) has not been published yet. Channels marked "coming with v0.1.0" are built and tested in CI but go live when the
release is tagged. Until then, the only working route is Go (or building from source).

| Channel | Command | Platforms | Status |
|---|---|---|---|
| Go | `go install github.com/ambierhq/skigard/cmd/skigard@latest` | all (Go 1.27+) | live |
| Shell installer | `curl -fsSL https://raw.githubusercontent.com/ambierhq/skigard/main/scripts/install.sh \| sh` | macOS, Linux | coming with v0.1.0 (needs a release to download) |
| PowerShell installer | `irm https://raw.githubusercontent.com/ambierhq/skigard/main/scripts/install.ps1 \| iex` | Windows | coming with v0.1.0 |
| GitHub Releases | archives, `.deb`, `.rpm`, `.apk`, Arch packages, checksums, SBOMs, cosign signatures | all | coming with v0.1.0 |
| Homebrew | `brew install ambierhq/tap/skigard` | macOS, Linux | coming with v0.1.0 |
| npx / npm | `npx skigard`, `npm i -g skigard` | all | coming with v0.1.0 |
| apt | `sudo apt install skigard` | Debian, Ubuntu | coming with v0.1.0 |
| dnf | `sudo dnf install skigard` | Fedora, RHEL | coming with v0.1.0 |
| pacman | `sudo pacman -S skigard` | Arch | coming with v0.1.0 |
| winget | `winget install Ambier.Skigard` | Windows | coming with v0.1.0 (first submission is reviewed by Microsoft) |
| Scoop | `scoop bucket add ambierhq https://github.com/ambierhq/scoop-bucket` then `scoop install skigard` | Windows | coming with v0.1.0 |
| Nix | `nix run github:ambierhq/skigard` | Linux, macOS | coming with v0.1.0 |
| Docker | `docker run ghcr.io/ambierhq/skigard` | Linux | coming with v0.1.0 |
| mise | `mise use -g skigard` | all | coming with v0.1.0 |

## Verify what you downloaded

Every release carries `checksums.txt` (SHA-256), a keyless [cosign](https://github.com/sigstore/cosign) signature for it, and an SBOM per archive.
`install.sh` always checks the archive against `checksums.txt`, and also verifies the signature when `cosign` is installed. The installers never use
`sudo` and install to `~/.local/bin` (override with `SKIGARD_INSTALL_DIR`; pin a release with `SKIGARD_VERSION=v0.1.0`).

```sh
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/ambierhq/skigard/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
```

## Check it works

```sh
skigard version
skigard doctor
```

Then continue with [Getting started](../getting-started.md). Uninstall: remove the binary (and `~/.config/skigard` if you want to drop your
approvals). To remove only what skigard installed into agents, run `skigard uninstall` first.
