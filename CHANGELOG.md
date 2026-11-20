# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
- No build provenance attestation for `v0.1.1` (that step was skipped when the publisher failed). Checksums are signed with cosign.
- The npm packages are not public yet.

[Unreleased]: https://github.com/ambierhq/skigard/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/ambierhq/skigard/releases/tag/v0.1.1
