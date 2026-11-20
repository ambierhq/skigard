# Other install channels

Status of each channel is in the [status table](README.md#channels-and-status): npm, mise, Go and Nix are available now.

## npx and npm

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
