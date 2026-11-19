# Install on macOS

Apple silicon (arm64) and Intel (amd64) are both supported.

## Homebrew

```sh
brew install ambierhq/tap/skigard
```

## Shell installer

```sh
curl -fsSL https://ambierhq.com/skigard/install.sh | sh
```

Downloads the latest release archive for your architecture, checks it against `checksums.txt` (and the cosign signature if `cosign` is
installed), and puts `skigard` in `~/.local/bin`. Make sure that directory is on your `PATH`. Options: `SKIGARD_VERSION`, `SKIGARD_INSTALL_DIR`,
`SKIGARD_SKIP_COSIGN=1`.

## From a release archive

Download `skigard_<version>_darwin_<arch>.tar.gz` from [GitHub Releases](https://github.com/ambierhq/skigard/releases), check it against
`checksums.txt`, extract it and move `skigard` onto your `PATH`.

## Go

```sh
go install github.com/ambierhq/skigard/cmd/skigard@latest
```

## Notes

- Review runs inside a Seatbelt sandbox (`/usr/bin/sandbox-exec`) when it is available. See [review](../review.md).
- `approve` reads `/dev/tty`. It works over SSH sessions that have a terminal and refuses without one.
- Files: `~/.config/skigard` (or `$XDG_CONFIG_HOME/skigard`, or `$SKIGARD_HOME`).
- Shell completions: `skigard completion zsh > "${fpath[1]}/_skigard"` (also `bash`, `fish`).

Other channels: [npx, Nix, Docker, mise](other.md). Next: [Getting started](../getting-started.md).
