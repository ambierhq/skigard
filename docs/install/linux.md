# Install on Linux

`amd64` and `arm64` are supported. Packages are built for the common distributions; all of them install a single binary at
`/usr/bin/skigard` plus shell completions.

## Shell installer (coming with v0.1.0)

```sh
curl -fsSL https://raw.githubusercontent.com/ambierhq/skigard/main/scripts/install.sh | sh
```

Installs to `~/.local/bin` without `sudo`. Options: `SKIGARD_VERSION`, `SKIGARD_INSTALL_DIR`, `SKIGARD_SKIP_COSIGN=1`. The archive is checked against
`checksums.txt`, and its signature too when `cosign` is installed.

## Homebrew on Linux (coming with v0.1.0)

```sh
brew install ambierhq/tap/skigard
```

## Packages from a GitHub Release (coming with v0.1.0)

Each GitHub Release has a `.deb`, `.rpm`, `.apk` and `.pkg.tar.zst` attached. There are no apt, dnf or pacman repositories yet
(planned if there is demand). Download the package from the [release](https://github.com/ambierhq/skigard/releases) and install it with your package manager:

```sh
sudo apt install ./skigard_*.deb          # Debian, Ubuntu
sudo dnf install ./skigard_*.rpm          # Fedora, RHEL
sudo pacman -U ./skigard-*.pkg.tar.zst    # Arch
```

Verify the download against `checksums.txt` first (see [Install](README.md)). Updating means installing the newer file.

## Alpine (coming with v0.1.0)

An `.apk` is attached to each release.

## Go

```sh
go install github.com/ambierhq/skigard/cmd/skigard@latest
```

## Notes

- Review uses bubblewrap (`bwrap`) when it is installed, otherwise Landlock (Linux 5.13+), and otherwise runs under policy only. See
  [review](../review.md) for what each one confines and the `--allow-policy-only` switch.
- `approve` needs a controlling terminal (`/dev/tty`). It refuses in containers and CI jobs without one, by design.
- Files: `~/.config/skigard` (or `$XDG_CONFIG_HOME/skigard`, or `$SKIGARD_HOME`).
- WSL counts as Linux and gets the Linux sandbox.
- Shell completions: `skigard completion bash > ~/.local/share/bash-completion/completions/skigard` (also `zsh`, `fish`).

Other channels: [npx, Nix, Docker, mise](other.md). Next: [Getting started](../getting-started.md).
