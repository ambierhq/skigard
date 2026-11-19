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

## apt: Debian, Ubuntu (coming with v0.1.0)

```sh
sudo apt install skigard
```

This needs the signed apt repository that goes live with the first release. Until then you can install the `.deb` from a release with
`sudo apt install ./skigard_<version>_linux_<arch>.deb` once one is published.

## dnf: Fedora, RHEL (coming with v0.1.0)

```sh
sudo dnf install skigard
```

## pacman: Arch (coming with v0.1.0)

```sh
sudo pacman -S skigard
```

The Arch package is first published through the repository on `releases.ambierhq.com`; the goal is the official `extra` repository.

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
