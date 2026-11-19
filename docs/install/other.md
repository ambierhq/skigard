# Other install channels

All of these arrive with v0.1.0 unless marked live. See the [status table](README.md#channels-and-status).

## npx and npm (coming with v0.1.0)

```sh
npx skigard detect            # run without installing
npm install --global skigard  # or install
```

The npm package is a small launcher that selects the right prebuilt binary for your platform (through per-platform `optionalDependencies`); it runs no
install scripts and downloads nothing at install time.

## Nix (coming with v0.1.0)

```sh
nix run github:ambierhq/skigard -- detect
```

## Docker (coming with v0.1.0)

```sh
docker run --rm ghcr.io/ambierhq/skigard --help
```

The image is distroless, non-root and contains only the binary. `approve` needs a terminal, so use `docker run -it` and mount a home
directory if you want to keep state: `-v "$HOME/.config/skigard:/home/nonroot/.config/skigard"`. The image is most useful for
`skigard verify` in CI.

## mise (coming with v0.1.0)

```sh
mise use -g skigard
```

Until it is in the registry: `mise use -g ubi:ambierhq/skigard` or `github:ambierhq/skigard` once a release exists.

## Go (live)

```sh
go install github.com/ambierhq/skigard/cmd/skigard@latest
```

Needs Go 1.27 or later and builds from source with no cgo.

## From source

```sh
git clone https://github.com/ambierhq/skigard && cd skigard
mise install && mise run build      # writes ./dist/skigard
```
