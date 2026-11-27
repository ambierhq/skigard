# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-11-27

### Changed

- **BREAKING: `auto_review` is now on by default.** Every `skigard add` and `skigard update` runs the advisory agent review unless you run
  `skigard config set auto_review false` (or use `add --quarantine-only`), which **sends the skill to the model provider of the reviewer harness**.
  The reviewer is the saved `review_harness`, else the first detected, verified reviewer (computed on every run, never written to `skigard.toml`).
  With no verified reviewer installed the review is skipped. Homes that set `auto_review` explicitly keep their choice.
- **BREAKING: a default review that fails to run blocks the automatic install.** `add` leaves the skill in quarantine (exit 1) and `update` keeps the
  changed version in quarantine, with `review did not run: ...` among the reasons. Fix the harness, pick another with `config set review_harness`, or turn reviews off.
- **BREAKING: the risk and score formula changed (`score_version` 2, `ScoreVersion` 2).** The scanner's risk (0 to 100) now counts each rule once, at
  the weight of its worst finding, plus up to three small bumps for repeats, so many identical lines no longer pile up. Safety scores go up for noisy skills, so skills
  that stayed in quarantine before can now clear the threshold and install on their own in `auto` mode. Old locks keep their recorded score and version.
- **BREAKING: scanner rules `2026.4`.** `tool eval "..."` (for example `playwright-cli eval`) is no longer the shell builtin `SKG-EXEC-004`, `npx --no-install`
  is no longer a package install (`SKG-SUPPLY-001`), and `obj.fetch()` is no longer the global `fetch` (`SKG-NET-002`). Skills that were flagged for these now pass them.
- Output labels say which way is better: risk is 0 to 100 and **higher is riskier**; the safety score is 1 to 10 and **higher is safer**.
- On a terminal, output is styled: colour, grouped `scan` findings, Markdown rendering for `show`, styled `review`, `add` outcomes, `diff`, tables for `list` and
  `plugin list`, and `doctor`, `status` and `verify`. Pipes, files, `--json` and `NO_COLOR` stay plain (new package `internal/term`).
- `plugin create|add|remove|delete` print a confirmation line; `plugin list` prints a table.
- claude-code now reviews under the macOS Seatbelt sandbox: its scratch directory is redirected with `CLAUDE_CODE_TMPDIR`, `~/.config/anthropic` is readable and
  `*.lock` entries can be created there. `docs/review.md` lists the per-harness needs.

### Added

- `skigard allow add|list|remove`: a human-only, per-source allow list for `medium` and `low` scanner rules (`[[allow]]` in `skigard.toml`). It never hides `high` or
  `critical` rules, is refused inside an agent, needs `allow` typed at the terminal, and suppressed findings stay counted in the scan output.
- `skigard config get|set review_harness` (`auto` clears the saved choice). `config get auto_review` and `review_harness` show the effective value.
- `show --plain` and `scan --plain` print the raw text with no colours or rendering.
- A spinner with the elapsed time on stderr while a reviewer runs (terminals only, not with `--json`).

### Fixed

- Repeated matches of one rule no longer inflate the risk number, so large, repetitive skills are no longer scored down for the same habit many times.

### Security

- `allow` is a human step with the same agent refusal as `approve`; an agent cannot widen what the scanner reports. `approve`, `--force-install` and `config set mode|threshold` are unchanged.
- Styled output only touches text that has already been escaped with `util.Clean`, so untrusted skill text cannot inject terminal escape sequences.
- The default review sends the skill to a model provider; see the BREAKING note above and the [threat model](docs/threat-model.md#reviewing-agents-what-leaves-your-machine).

## [0.2.0] - 2026-11-20

### Changed

- **BREAKING: `skigard add` now installs by default in `auto` mode.** Scripts and agents that relied on `add` only fetching and scanning will now
  get a skill installed if it clears the threshold. Use `add --quarantine-only` for the old behaviour, or `skigard config set mode strict` to
  require a human first. It fetches to quarantine, scans, scores the skill from 1 to 10 (10 = no concern) with reasons, and in the
  default `auto` mode approves and installs it if the score is at or above the threshold (default 7) and there is no hard stop. Otherwise it stays in
  quarantine with a warning and the commands to continue. The old behaviour (fetch and scan only) is `add --quarantine-only`. `review`, `approve`,
  `reject` and `install` work unchanged.
- skigard is now described as "A safer way to manage agent skills": it scans and scores each skill from 1 to 10, pins the exact bytes in a
  lockfile and installs in one command. It no longer claims that "nothing reaches your agents until a human approves"; a human still approves in
  `strict` mode and for any skill below the threshold. It never claims to prove a skill safe.
- Automatic approvals record `by = "skigard (auto)"` instead of the OS user; `mode = "auto"` is unchanged. That value is reserved and a lock that uses it with another mode fails to load.
- `update` in `auto` mode installs a changed skill only if it scores at or above the threshold, has no hard stop and does not score lower than the
  approved version; otherwise it goes back to quarantine with the diff. `strict` mode is unchanged.
- Docs and site: install status, pinned versions (`v0.1.2`), provenance wording and test coverage statements brought in line with what is verified (winget #484914 and nixpkgs #590787 are in review; Windows is CI-tested only; the Linux sandboxed review was not run by hand). No code change.
- Release workflow: the winget job updates our open PR while the package is not in microsoft/winget-pkgs yet, instead of opening a second one.

### Fixed

- `skigard add SRC --name X` with a name that does not match the skill no longer prints `"X" was approved but installing failed: <nil>` before the real error.

### Added

- `add` flags `--mode strict|auto`, `--threshold N`, `--review`, `--force-install` and `--quarantine-only`.
- `skigard config get|set mode|threshold|auto_review` (`skigard.toml` `[settings]`). `set` of `mode` and `threshold` needs a human terminal.
- Safety score with tiers `ok`, `caution` and `blocked`, and hard stops (scanner `block`, reviewer `fail`) that are never auto-installed. `ScoreVersion` is recorded in the lock.
- Lock approvals record `mode` (`human`, `auto`, `forced`), `score`, `threshold` and `score_version`. Older locks read as `human`; the lock `version` stays 1.
- `skigard verify --require-human` fails if any approval is `auto` or `forced`. `list`, `show` and `status` show mode and score.

### Security

- Inside a detected agent, `--force-install` and `config set mode|threshold` are refused, and `--mode`/`--threshold` can only be stricter than the stored setting.
  `approve` is unchanged (terminal only, no bypass). Detection is best effort (environment and ancestor processes). In `auto` mode an agent that runs `add` can install skills that clear your threshold; this is recorded as `mode = "auto"`. Set `mode` to `strict` to prevent it. See the [threat model](docs/threat-model.md).

## [0.1.2] - 2026-11-20

### Added

- Best-effort agent detection: `approve` also refuses when an ancestor process is a known agent harness (by binary name or environment marker), which catches the `script`/pty one-liner even when the agent variable was stripped. The refusal no longer names the variable. It is evadable by re-parenting (documented in the threat model).
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

[Unreleased]: https://github.com/ambierhq/skigard/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/ambierhq/skigard/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/ambierhq/skigard/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/ambierhq/skigard/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/ambierhq/skigard/releases/tag/v0.1.1
