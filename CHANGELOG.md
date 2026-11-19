# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

First public release (`v0.1.0`) in preparation.

### Added

- Quarantine-first fetching from GitHub (as archives, never `git`) and local paths, with hostile-archive handling.
- Deterministic scanner with 34 rules and capability chains; text, JSON and SARIF output.
- Advisory agent review through installed harnesses, tools off, inside an OS sandbox on macOS and Linux.
- Human-only `approve` (terminal device, no bypass), hash-pinned `skigard.lock`, `install`, `sync`, `update`, `verify`.
- Content hash v2: identical on every OS, with a separate exec set, and `skigard lock upgrade` for older locks.
- Registry of 78 agent harnesses with detection and installs, plugins and a generated Claude Code / OMP marketplace.
- Windows support (amd64, arm64), including `approve` through the console.
- Release pipeline: signed, SBOM-ed archives and Linux packages, container image, shell and PowerShell installers.
- Documentation, brand assets and a generated command reference.

[Unreleased]: https://github.com/ambierhq/skigard/commits/main
