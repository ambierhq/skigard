# Packaging and channels

Everything here is testable against a local `mise run snapshot` (GoReleaser snapshot in `./dist`) and is **gated at release time**:
nothing publishes unless the switch below exists. No R2, no package repositories (no apt, dnf or pacman repository, no AUR package, no Docker image, and none is planned for now):
binaries, `.deb/.rpm/.pkg.tar.zst`, `install.sh` and `install.ps1` are GitHub release assets.

| Channel | Source | Test locally | Release-time switch (owner sets) |
|---|---|---|---|
| npm | `npm/`, `scripts/npm-pack.mjs`, `.github/workflows/release.yml` | `node --test npm/test/`, `scripts/npm-smoke.sh dist VERSION` | live: repo variable `NPM_PUBLISH=true`, `@ambierhq` npm org, trusted publisher per package (workflow `release.yml`, environment `npm`); since 0.1.2 the release workflow publishes with provenance |
| Arch (source build) | `packaging/arch`, `.github/workflows/arch.yml` | `docker run archlinux ... packaging/arch/test.sh` (CI does) | none (no AUR package; the release ships the `.pkg.tar.zst`) |
| Nix | `flake.nix`, `packaging/nixpkgs/package.nix`, `.github/workflows/nix.yml` | `nix build` (CI does) | none for the flake; nixpkgs PR submitted ([#590787](https://github.com/NixOS/nixpkgs/pull/590787), in review) |
| aqua / mise | `packaging/aqua/registry.yaml`, `packaging/mise/registry-entry.toml` | `mise use github:...[api_url=...]` against `scripts/fake-github-release.py` | not submitted and not planned for now (the drafts below are kept for later) |
| GitHub Action | `ambierhq/skigard-action` | its own CI | tag `v1` after the owner approves |

## Release-time checklist (what the owner enables, in order)

1. Done: `v0.4.0` is released (the release workflow builds everything, signs it, attests it and uploads `install.sh` / `install.ps1` as assets). The Homebrew tap and Scoop bucket are updated by the release workflow. winget is submitted ([winget-pkgs#484914](https://github.com/microsoft/winget-pkgs/pull/484914), `v0.1.1`'s #484409 was closed as superseded): `winget install Ambier.Skigard` does not work until it is merged. There is no Docker image.
2. Done: npm. `skigard` and the six `@ambierhq/skigard-*` packages are at 0.4.0, published by the release workflow through npm trusted publishing with provenance (0.1.1 was published by hand, without provenance).
3. Done: `ambierhq.com/skigard/install.{sh,ps1}` redirect to `releases/latest/download/install.{sh,ps1}` (site Worker).
4. Not planned for now: the registry PRs below. `skigard-action` has its `v1` tag.

## aqua-registry PR (draft, not submitted)

Title: `feat(ambierhq/skigard): add ambierhq/skigard`

> Adds skigard, a safer way to manage agent skills (https://github.com/ambierhq/skigard).
> Release archives are `skigard_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows) with the binary at the archive root, built by GoReleaser
> for darwin/linux/windows on amd64/arm64. `checksums.txt` is signed with keyless cosign (`checksums.txt.sigstore.json`), configured in `checksum.cosign`.
> Generated with `aqua gr ambierhq/skigard` and tested with `cmdx t ambierhq/skigard` / `aqua i` on Linux, macOS and Windows (draft text: run these before submitting; not done yet).
> I am the maintainer of the project.

File: `pkgs/ambierhq/skigard/registry.yaml` (content: `packaging/aqua/registry.yaml`; drop `description` quirks the generator does not emit,
and let `aqua gr` decide). Add the entry to `registry.yaml` via their `cmdx new` / scaffold flow.

## jdx/mise registry PR (draft, not submitted; maintainers may ask for adoption first, then keep `github:` in the docs)

Title: `registry: add skigard (aqua:ambierhq/skigard, github:ambierhq/skigard)`

> Adds `skigard` to the registry: backends `aqua:ambierhq/skigard` (once the aqua-registry PR is merged; otherwise only `github:`), then
> `github:ambierhq/skigard`. `test = skigard version` returns `skigard <version>`. Verified with `mise use github:ambierhq/skigard` on
> Linux, macOS and Windows (draft text: so far checked on macOS only). Note: the ubi backend is deprecated in current mise, so `ubi:` is not listed.

File: `registry/skigard.toml` (content: `packaging/mise/registry-entry.toml`). Re-read mise's CONTRIBUTING first: the registry format has changed before.

## nixpkgs PR (submitted as [#590787](https://github.com/NixOS/nixpkgs/pull/590787), `skigard: init at 0.1.1`, in review)

Title: `skigard: init at 0.1.1`

> A safer way to manage agent skills. `buildGoModule`, one Go dependency, tests run in the Go check phase, shell completions installed.
> Built on the platforms nixpkgs CI covers (draft text: confirm what was actually built before claiming more). License MIT.

File: `pkgs/by-name/sk/skigard/package.nix` (content: `packaging/nixpkgs/package.nix`), add the maintainer to `maintainers/maintainer-list.nix` in a separate commit.
