---
title: Install
description: Install skigard on macOS, Linux or Windows with a shell installer, npx, apt, pacman, Homebrew, Nix or go install.
---

skigard is one static binary. Every installer verifies the SHA-256 checksum, and the shell installer also checks the
cosign signature when `cosign` is installed. None of them uses `sudo` except the system package managers.

:::note
No release is published yet. Until `v0.1.0`, install from source with `go install`. The channel commands below are
what the release pipeline is set up to publish.
:::

## macOS and Linux

```sh
curl -fsSL https://ambierhq.com/skigard/install.sh | sh
```

`-L` is required: the URL redirects to the latest GitHub release asset. Set `SKIGARD_VERSION` to pin a release and
`SKIGARD_INSTALL_DIR` to choose the target (default `~/.local/bin`).

## Windows

PowerShell 5.1 or 7, no administrator rights:

```powershell
irm https://ambierhq.com/skigard/install.ps1 | iex
```

See [Windows notes](/skigard/docs/windows/) for file locations and what differs.

## npx

```sh
npx skigard detect
```

## Debian and Ubuntu

```sh
sudo apt install ./skigard_<version>_amd64.deb   # from the GitHub release
```

A signed apt repository is planned; this page will list the repository lines when it is live.

## Arch Linux

```sh
sudo pacman -U https://github.com/ambierhq/skigard/releases/download/v<version>/skigard_<version>_linux_amd64.pkg.tar.zst
```

## Homebrew

```sh
brew install ambierhq/tap/skigard
```

## Nix

There is no Nix package yet. Use `go install` or the release binary.

## From source

```sh
go install github.com/ambierhq/skigard/cmd/skigard@latest   # Go 1.27+
```

## Verify your download

Each release publishes `checksums.txt`, a cosign keyless signature for it and an SBOM. Check them before you run
the binary:

```sh
sha256sum -c checksums.txt --ignore-missing
cosign verify-blob --certificate-identity-regexp 'https://github.com/ambierhq/skigard/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --bundle checksums.txt.sigstore.json checksums.txt
```
