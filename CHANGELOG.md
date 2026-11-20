# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- npm packages published: `skigard` and the six `@ambierhq/skigard-<os>-<cpu>` packages (`npx skigard`, `npm i -g skigard`).

### Changed

- Wording: approval is described as needing a human at a terminal, with the pseudo-terminal limitation documented (threat model, README, FAQ, SECURITY.md). The one-line pitch no longer claims that nothing can reach an agent without approval. Quick start uses a small passing sample (`examples/hello-skill`).
- Scanner rules `2026.3` (37 rules). New: `SKG-OBF-006` encoded PowerShell command, `SKG-SUPPLY-003` indirect "fetch the real
  instructions" without a URL, `SKG-SUPPLY-004` package install hooks. Extended: `SKG-EXEC-002` (python `pty.spawn` shell,
  socket `dup2`), `SKG-EXEC-003` (`rm -rf --no-preserve-root /`), `SKG-PROMPT-001/002/003` (disregard/forget synonyms, "the
  person you work for", auto-approve). `SKG-OBF-003` no longer fires on `data:font/` and `data:image/` URIs.

### Fixed

- `skigard version` now reports the module version for `go install ...@vX.Y.Z` builds (previously `dev`), and `dev+<commit>` (`-dirty` if modified) for local VCS builds. GoReleaser ldflags still win.

## [0.1.1] - 2026-11-19

Use this release. `v0.1.0` was tagged a few minutes earlier; its release assets were published, but the release workflow failed in the
publisher step (a templating error in the token settings, fixed in #26), so the Scoop bucket and the Homebrew tap were not updated.

### Added

- Quarantine-first fetching from GitHub (as archives, never `git`) and local paths, with hostile-archive handling.
- Deterministic scanner with 34 rules and capability chains; text, JSON and SARIF output.
- Advisory agent review through installed harnesses, tools off, inside an OS sandbox on macOS and Linux.
- Human-only `approve` (terminal device, no bypass), hash-pinned `skigard.lock`, `install`, `sync`, `update`, `verify`.
- Content hash v2: identical on every OS, with a separate exec set, and `skigard lock upgrade` for older locks.
- Registry of 78 agent harnesses with detection and installs, plugins and a generated Claude Code / OMP marketplace.
- Windows support (amd64, arm64), including `approve` through the console.
- Release pipeline: signed, SBOM-ed archives and Linux packages, shell and PowerShell installers.
- Documentation, brand assets and a generated command reference.

### Known issues

- The winget manifest was submitted by hand ([winget-pkgs#484409](https://github.com/microsoft/winget-pkgs/pull/484409)) after the workflow's
  winget step timed out; the Homebrew formula was pushed by hand too. The workflow now does both in separate jobs.
- The `v0.1.1` build provenance attestations were added after the release (`gh attestation verify ... --repo ambierhq/skigard` works). Checksums are signed with cosign.

[Unreleased]: https://github.com/ambierhq/skigard/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/ambierhq/skigard/releases/tag/v0.1.1
