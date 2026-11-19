# Packaging and channels

Everything here is testable against a local `mise run snapshot` (GoReleaser snapshot in `./dist`) and is **gated at release time**:
nothing publishes unless the switch below exists. No R2, no package repositories (apt/rpm/pacman are planned if there is demand):
binaries, `.deb/.rpm/.pkg.tar.zst`, `install.sh` and `install.ps1` are GitHub release assets.

| Channel | Source | Test locally | Release-time switch (owner sets) |
|---|---|---|---|
| npm | `npm/`, `scripts/npm-pack.mjs`, `.github/workflows/release.yml` | `node --test npm/test/`, `scripts/npm-smoke.sh dist VERSION` | repo variable `NPM_PUBLISH=true`, `@ambierhq` npm org, trusted publisher per package (workflow `release.yml`, environment `npm`) |
| Arch (source build) | `packaging/arch`, `.github/workflows/arch.yml` | `docker run archlinux ... packaging/arch/test.sh` (CI does) | none (no AUR package; the release ships the `.pkg.tar.zst`) |
| Nix | `flake.nix`, `packaging/nixpkgs/package.nix`, `.github/workflows/nix.yml` | `nix build` (CI does) | none for the flake; nixpkgs PR is the owner's |
| aqua / mise | `packaging/aqua/registry.yaml`, `packaging/mise/registry-entry.toml` | `mise use github:...[api_url=...]` against `scripts/fake-github-release.py` | PRs below, after v0.1.0 |
| GitHub Action | `ambierhq/skigard-action` | its own CI | tag `v1` after the owner approves |

## Release-time checklist (what the owner enables, in order)

1. Tag `v0.1.0` (release workflow builds everything and uploads `install.sh` / `install.ps1` as assets).
2. npm: create the `@ambierhq` org, add trusted publishers for `skigard` and the six `@ambierhq/skigard-*` packages, set `NPM_PUBLISH=true`, re-run `npm-publish` for the tag. (Trusted publishers can only be configured for an existing package: publish the first version of each package once, manually with 2FA, or use a placeholder.)
3. Redirects for `ambierhq.com/skigard/install.{sh,ps1}` to `releases/latest/download/install.{sh,ps1}` (site Worker).
4. Submit the registry PRs below. Tag `v1` on `skigard-action` after review.

## aqua-registry PR (do not open before v0.1.0 exists)

Title: `feat(ambierhq/skigard): add ambierhq/skigard`

> Adds skigard, a lockfile and review gate for agent skills (https://github.com/ambierhq/skigard).
> Release archives are `skigard_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows) with the binary at the archive root, built by GoReleaser
> for darwin/linux/windows on amd64/arm64. `checksums.txt` is signed with keyless cosign (`checksums.txt.sigstore.json`), configured in `checksum.cosign`.
> Generated with `aqua gr ambierhq/skigard` and tested with `cmdx t ambierhq/skigard` / `aqua i` on Linux, macOS and Windows.
> I am the maintainer of the project.

File: `pkgs/ambierhq/skigard/registry.yaml` (content: `packaging/aqua/registry.yaml`; drop `description` quirks the generator does not emit,
and let `aqua gr` decide). Add the entry to `registry.yaml` via their `cmdx new` / scaffold flow.

## jdx/mise registry PR (do not open before v0.1.0 exists; maintainers may ask for adoption first, then keep `github:` in the docs)

Title: `registry: add skigard (aqua:ambierhq/skigard, github:ambierhq/skigard)`

> Adds `skigard` to the registry: backends `aqua:ambierhq/skigard` (once the aqua-registry PR is merged; otherwise only `github:`), then
> `github:ambierhq/skigard`. `test = skigard version` returns `skigard <version>`. Verified with `mise use github:ambierhq/skigard` on
> Linux, macOS and Windows. Note: the ubi backend is deprecated in current mise, so `ubi:` is not listed.

File: `registry/skigard.toml` (content: `packaging/mise/registry-entry.toml`). Re-read mise's CONTRIBUTING first: the registry format has changed before.

## nixpkgs PR (do not open before v0.1.0 exists and the owner has a maintainer entry)

Title: `skigard: init at 0.1.0`

> Lockfile and review gate for agent skills. `buildGoModule`, one Go dependency, tests run in the Go check phase, shell completions installed.
> Built and tested on x86_64-linux, aarch64-linux, x86_64-darwin, aarch64-darwin. License MIT.

File: `pkgs/by-name/sk/skigard/package.nix` (content: `packaging/nixpkgs/package.nix`), add the maintainer to `maintainers/maintainer-list.nix` in a separate commit.
