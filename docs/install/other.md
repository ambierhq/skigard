# Other install channels

Status of each channel is in the [status table](README.md#channels-and-status): mise, Go and Nix are available now; npm and Docker are not yet.

## npx and npm (not published yet)

```sh
npx skigard detect            # run without installing
npm install --global skigard  # or install
```

The npm package is a small launcher that selects the right prebuilt binary for your platform (through per-platform `optionalDependencies`); it runs no
install scripts and downloads nothing at install time.

## Nix (builds in CI, not tested by hand)

```sh
nix run github:ambierhq/skigard -- detect
```

## Docker (not yet public)

The release workflow builds and signs `ghcr.io/ambierhq/skigard`, but the package is not public yet, so `docker pull` is denied for now.
Until then, install the binary another way. The commands below are what will work.

```sh
docker run --rm ghcr.io/ambierhq/skigard --help
```

The image is distroless, non-root and contains only the binary. `approve` needs a terminal, so use `docker run -it` and mount a home
directory if you want to keep state: `-v "$HOME/.config/skigard:/home/nonroot/.config/skigard"`. The image is most useful for
`skigard verify` in CI.

## mise

```sh
mise use -g github:ambierhq/skigard
```

The short name `mise use -g skigard` needs a mise registry entry (not submitted). Recent mise versions skip releases younger than their
`minimum_release_age` (24 hours), so a brand-new release may need `mise use -g github:ambierhq/skigard@<version>` first.

## Go

```sh
go install github.com/ambierhq/skigard/cmd/skigard@latest
```

Needs Go 1.27 or later and builds from source with no cgo. A `go install pkg@vX.Y.Z` build reports that module version; a local build reports `dev+<commit>` (`-dirty` if modified).

## From source

```sh
git clone https://github.com/ambierhq/skigard && cd skigard
mise install && mise run build      # writes ./dist/skigard
```
