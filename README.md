<p align="center">
  <picture>
    <source media="(prefers-color-scheme: light)" srcset="assets/brand/logo-full-on-light.svg">
    <img alt="skigard" src="assets/brand/logo-full.svg" height="80">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/ambierhq/skigard/releases"><img alt="Release" src="https://img.shields.io/github/v/release/ambierhq/skigard"></a>
  <a href="https://github.com/ambierhq/skigard/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/ambierhq/skigard/ci.yml?branch=main"></a>
  <a href="https://scorecard.dev/viewer/?uri=github.com/ambierhq/skigard"><img alt="OpenSSF Scorecard" src="https://api.scorecard.dev/projects/github.com/ambierhq/skigard/badge"></a>
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/github/license/ambierhq/skigard"></a>
  <a href="https://github.com/ambierhq/skigard/releases"><img alt="Downloads" src="https://img.shields.io/github/downloads/ambierhq/skigard/total"></a>
  <a href="docs/install/README.md"><img alt="Platforms: Linux, macOS, Windows" src="https://img.shields.io/badge/platforms-linux%20%7C%20macOS%20%7C%20windows-informational"></a>
</p>

# skigard: a safer way to manage agent skills for Claude Code, Codex and Cursor (scores, lockfile)

**A safer way to manage agent skills.** skigard scans and scores each agent skill from 1 to 10, pins the exact bytes in a lockfile, and installs in one command; strict mode asks a human first. It lowers risk and shows its reasons; it does not prove a skill safe.

**The default mode is `auto`** (threshold 7): a skill that scores 7 or more with no hard stop installs without a prompt, and the lock records `mode = "auto"`. To require a human for every install, run `skigard config set mode strict` (see [Configuration](#configuration)).

<p>
  <a href="#install">Install</a> ·
  <a href="#quick-start">Quick start</a> ·
  <a href="docs/getting-started.md">Docs</a> ·
  <a href="https://ambierhq.com/skigard">Website</a>
</p>

## The problem

- Agent skills are instructions and scripts that run with **your agent's permissions**. Copying one from the internet is running someone else's code.
- Skills are fetched from moving branches: what you read yesterday is not what installs today. There is no pinning.
- Nobody reviews them, and the agent that installs a skill is the same agent a bad skill can talk into installing more.
- A prompt-injected agent can add skills on its own, so "the agent approved it" is not a control.

skigard puts a check in front of that: **fetch to quarantine, scan, agent review (on by default when a reviewer is installed), safety score, decision (auto-install or ask a human), hash-pinned lockfile, install.** It scores and warns; it does not claim to stop every attack (see the [threat model](docs/threat-model.md)).

| Without skigard | With skigard |
|---|---|
| `git clone` or copy a skill straight into `~/.claude/skills` | Fetched as an archive into a private quarantine, never executed |
| Whatever is on the branch today | Pinned to a commit and a SHA-256 content hash in `skigard.lock` |
| You skim it, maybe | A deterministic scan, an agent review (on by default, advisory), and a diff against what you approved before |
| An agent can install more skills | `approve`, `--force-install` and changing `mode`/`threshold` need a human at a terminal, and agents skigard can detect are refused. In `auto` mode an agent's `add` can still install skills that clear your threshold, and the lock records it |
| You want to change a skill | `skigard edit` copies it to a working copy, `adopt` scans and records your edit as a local fork, and `update` never replaces it; `merge` brings new upstream changes in ([guide](docs/customizing-skills.md)) |
| A skill changes silently on update | Changes are re-scored; in `auto` mode only a skill that clears the threshold and does not score lower than before is installed, otherwise it goes back to quarantine with a diff |
| Each agent has its own skills folder | Installs once into every detected harness (Claude Code, Pi, Codex, Cursor, Gemini CLI, OpenCode and 72 more of the 78 in the registry; only 3 are verified) |
| No way to notice drift | `skigard verify` fails CI if the lock, the store or the installs change |

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/diagrams/pipeline-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="assets/diagrams/pipeline-light.svg">
  <img alt="skigard pipeline: add to quarantine, scan, agent review, safety score, auto-install above the threshold or human approve in a real terminal, skigard.lock, install; update loops back to quarantine and verify checks drift from the lock" src="assets/diagrams/pipeline-dark.svg" width="880">
</picture>

<sub>Made with [Archify](https://github.com/tt-a1i/archify); the Mermaid sources stay in [assets/diagrams](assets/diagrams). `verify` and `update` branch off the lock.</sub>

<p align="center">
  <img alt="skigard adding, scanning, approving and installing a skill" src="assets/media/demo.gif" width="800">
</p>

- **One static binary**, no runtime, no telemetry, one dependency (a TOML parser).
- **Approve, force and mode changes need a human at a terminal.** `approve` reads the terminal device only: there is no `--yes`, no stdin path and no environment variable, and skigard refuses `approve`, `--force-install` and `config set mode|threshold` when it detects an agent harness in its environment or among its parent processes (best effort, evadable by re-parenting). A process that creates its own pseudo-terminal can still type the answer, so this stops accidents and ordinary prompt injection, not a determined agent ([details](docs/threat-model.md#the-pseudo-terminal-case)). In `auto` mode an agent's `add` can install skills that clear your threshold; use `--mode strict` to prevent it. Signed approvals (a key in your OS keychain or a passkey) are planned.
- **Reproducible.** `skigard.lock` records source, commit, content hash, scan result, review, score and how it was approved (`human`, `local`, `auto` or `forced`). Commit it.
- **Portable.** The content hash is identical on Linux, macOS and Windows.

> The scanner is heuristic and an agent review can be fooled. A high score is not a safety certificate. Read the skill,
> especially bundled scripts, or use `--mode strict`. See the [threat model](docs/threat-model.md) and
> [what skigard does not protect against](#what-skigard-does-not-protect-against).

## Install

`v0.4.0` is the latest release ([GitHub Releases](https://github.com/ambierhq/skigard/releases)). Status of each channel:

| Channel | Command | Status |
|---|---|---|
| Go | `go install github.com/ambierhq/skigard/cmd/skigard@latest` | live (`go install ...@v0.4.0` prints `skigard 0.4.0`) |
| Shell installer (macOS, Linux) | `curl -fsSL https://ambierhq.com/skigard/install.sh \| sh` | live |
| PowerShell (Windows) | `irm https://ambierhq.com/skigard/install.ps1 \| iex` | live (tested in CI on `windows-latest`; not tested by hand on Windows) |
| GitHub Releases | archives, `.deb`, `.rpm`, `.apk`, `.pkg.tar.zst`, checksums, SBOMs, cosign signatures | live |
| Homebrew | `brew install ambierhq/tap/skigard` | live |
| winget | `winget install Ambier.Skigard` | submitted, in review at Microsoft ([winget-pkgs#484914](https://github.com/microsoft/winget-pkgs/pull/484914)); not installable yet |
| Scoop | `scoop bucket add ambierhq https://github.com/ambierhq/scoop-bucket && scoop install skigard` | live (the bucket is updated by the release workflow); not tested by hand on Windows |
| mise | `mise use -g github:ambierhq/skigard` | live (`github:` backend, checked with v0.1.2; the short name `skigard` needs a registry entry, not submitted) |
| Nix | `nix run github:ambierhq/skigard` | the flake builds in CI (Linux and macOS), not tested by hand. A nixpkgs package (`pkgs.skigard`) is submitted and in review ([nixpkgs#590787](https://github.com/NixOS/nixpkgs/pull/590787)); it does not exist yet |
| npx / npm | `npx skigard` or `npm i -g skigard` | available (v0.4.0, published by the release workflow with npm provenance) |
| apt, dnf, pacman repositories | not offered. Install the `.deb`, `.rpm` or `.pkg.tar.zst` attached to a GitHub Release (see [Linux](docs/install/linux.md)) | not offered |

Per-system pages: [macOS](docs/install/macos.md) · [Linux](docs/install/linux.md) · [Windows](docs/install/windows.md) ·
[other channels](docs/install/other.md). Releases are checksummed, SBOM-ed, signed with cosign and carry GitHub build attestations
([verify them](docs/install/README.md#verify-what-you-downloaded)). The installers never use `sudo`.

### Supported platforms

| OS | Architectures | `approve` | Automated `review` | Installs as |
|---|---|---|---|---|
| Linux | amd64, arm64 | yes (`/dev/tty`) | yes (bubblewrap or Landlock) | symlinks (or `--copy`) |
| macOS | amd64, arm64 | yes (`/dev/tty`) | yes (Seatbelt) | symlinks (or `--copy`) |
| Windows 10/11 | amd64, arm64 | yes (console, `CONIN$`) | no, use `review --manual` or WSL | copies |

How much of this was tested by hand: macOS (arm64) end to end, including the Seatbelt-sandboxed review. Linux (Ubuntu 26.04 x86_64, Landlock, no
bubblewrap): the installer, the `.deb`, the quick start and `approve` through a pty by hand; the OS-sandboxed `review` was **not** run there. Windows is covered by CI only (tests, the installer, an `approve` test through a ConPTY), not tested by hand. macOS amd64 and Linux arm64 are cross-compiled in CI and not tested by hand.

## Quick start

`examples/hello-skill` is a tiny skill that passes the scanner, so the whole flow runs without an override:

```sh
skigard detect                                                         # which agent harnesses are on this machine?
skigard add github:ambierhq/skigard/examples/hello-skill               # fetch, scan, score 10/10, and install (auto mode, threshold 7)
skigard verify                                                         # CI-friendly: lock vs store vs installs
```

`add` runs the whole pipeline in one step. What happens next depends on the mode and the score (below). If a skill is held in
quarantine you get a warning with the reasons and the exact commands to continue (or use `--quarantine-only` to stop after the scan):

```sh
skigard review hello-skill                                             # second opinion from pi / claude / omp (asks which; `add` already ran it by default)
skigard approve hello-skill                                            # in a real terminal: shows scan, review and diff; you type the name
skigard install hello-skill                                            # link into every detected, verified harness
```

What a block looks like: this repository's own `skills/review-skill` talks *about* credentials and `curl | sh`, so the scanner blocks it
(four high or critical findings, each with file, line and the matching text). `add` leaves it in quarantine, and `approve` refuses until you have read it and record why:

```sh
skigard add github:ambierhq/skigard/skills/review-skill
skigard approve review-skill --override --reason "reviewed by hand: it only describes risks"
```

Large skills can produce hundreds of findings; read the verdict and the counts first, then the files. A step-by-step version is in
[Getting started](docs/getting-started.md); changing a skill you approved is covered in [Customizing a skill](docs/customizing-skills.md).

Update later: `skigard update` re-fetches approved skills; in `auto` mode a changed skill that clears the threshold (and does not score lower than before) is installed, anything else goes back to quarantine with a diff.
New machine: clone your home repository to `~/.config/skigard` and run `skigard sync`.

### Modes, threshold and the safety score

`skigard add SOURCE [--skill N | --all] [--mode strict|auto] [--threshold N] [--review] [--force-install] [--quarantine-only] [--to IDS] [--scope user|project] [--copy] [--json]`

| Mode | What `add` does |
|---|---|
| `strict` | Shows the scan, the score and the diff, then asks for human confirmation on `/dev/tty` (the `approve` screen), always. Without a terminal, or inside an agent, the skill stays in quarantine and next steps are printed. |
| `auto` (default) | If `score >= threshold` and there is no hard stop, approves and installs with no prompt. Otherwise the skill stays in quarantine with a warning, the reasons and the exact commands to continue. |
| any mode + `--force-install` | Installs no matter what, recorded in the lock as `mode = "forced"`. Needs a human at a terminal: refused inside a detected agent. |

The default threshold is `7`. Set them with `skigard config set mode strict`, `skigard config set threshold 8`, `skigard config set auto_review false`
(`config get` reads them; `auto_review` is on by default, so a review sends the skill to the model provider of the first detected verified reviewer harness; turn it off to stop that). `config set` of `mode` and `threshold` needs a human terminal and is refused in an agent.
Inside an agent, `--mode` and `--threshold` may only make things stricter than the stored setting; a human may pass anything.

The scanner prints a **risk** from 0 to 100 (**higher is riskier**); the **safety score** runs from 1 to 10, where **10 means no concern** (**higher is safer**). It is a heuristic and is always printed with its reasons, never alone.
It starts from the scanner's risk score (`10 - ceil(risk / 12)`), then applies caps: a scanner `block` at most 3, incomplete coverage at most 6, a failing review at most 3, a review with concerns minus 2.
A review can only lower the score. Tiers: `ok` (at or above the threshold), `caution`, `blocked`.

**Hard stops** are never auto-installed, whatever the threshold: a scanner verdict of `block` and a review verdict of `fail`. Only `--force-install` or a human `approve --override` passes them.
`--quarantine-only` is the old `add` (fetch and scan only), and `review`, `approve`, `reject` and `install` work as before.

### Configuration

Settings live in the `[settings]` table of `skigard.toml` and are read and changed with `skigard config`:

| Setting | Values | Default | Meaning |
|---|---|---|---|
| `mode` | `auto`, `strict` | `auto` | `auto` installs at or above the threshold; `strict` always asks a human |
| `threshold` | `1` to `10` | `7` | minimum safety score for an automatic install |
| `marketplace` | `true`, `false` | `false` | keep `marketplace/` rebuilt after every approval and plugin change, and make `verify` require it (`config set marketplace true` builds it now; `false` leaves the directory) |
| `auto_review` | `true`, `false` | `true` | run the agent review on every `add` and `update` with the saved `review_harness`, else the first detected verified reviewer (sends the skill to a model provider) |
| `review_harness` | `auto` or a harness id (`claude-code`, `pi`, `omp`, ...) | `auto` | the reviewer: `auto` uses the first detected, verified one; `config get review_harness` shows what would run |

```sh
skigard config get                      # show all settings
skigard config set mode strict          # human approval for everything
skigard config set threshold 8
skigard config set auto_review false
```

Output is coloured and tidied (tables, grouped scan findings, rendered `show`) only on a terminal. Pipes, files, `--json`, `--plain` (on `show` and `scan`) and `NO_COLOR` stay plain text, and everything from a skill is escaped first. While a reviewer runs on a terminal, a spinner with the elapsed time is drawn on stderr.

Per run, `add` takes `--mode`, `--threshold`, `--review`, `--quarantine-only` and `--force-install` (human terminal only). `config set mode|threshold` also needs a human terminal.

## How to use

| Command | What it does |
|---|---|
| `init [--git]`, `detect [--all]` | create the home; list harnesses (`installed`, `configured`, `absent`) |
| `add SOURCE [--skill N \| --all] [--mode M] [--threshold N] [--review] [--force-install] [--quarantine-only]` | fetch, scan, score and (per mode) install; `--quarantine-only` stops after the scan. `SOURCE`: `github:owner/repo[/path][@ref]`, `owner/repo`, a github.com URL, or a local path |
| `scan NAME\|PATH [--format text\|json\|sarif] [--fail-on LEVEL] [--plain] [--all]` | deterministic scan; repeated low findings are folded into one line with a count (every medium and above is listed), coloured on a terminal; `--all` lists every finding, `--json` is always complete |
| `review NAME [--harness ID \| --manual]` | advisory agent review ([how it works](docs/review.md)) |
| `edit NAME` | copy an approved skill into quarantine as an editable working copy (`skills/` is read-only and untouched) |
| `adopt NAME [--review] [--mode M] [--threshold N]` | finalize the working copy from `edit` (or an edit made in place): scan and score it; in `auto` mode it is approved when it clears the threshold (`mode = "auto"`), otherwise it stays in quarantine for a human `approve` (`mode = "local"`). Merge results always need a human |
| `diff NAME` | quarantined vs approved (for an edit: against the approved version, or the rebuilt upstream for an in-place edit; it says which) |
| `approve NAME [--override --reason R] [--accept R]` | **human only** (records `mode = "human"`, or `local` for an adopted edit) |
| `merge NAME [--pick \| --with HARNESS] [--note TEXT]` | bring the new upstream version of a locally edited skill into your copy, staged in quarantine; `--pick` is a hunk picker on `/dev/tty` (human only), `--with` has a verified reviewer harness (claude-code, pi, omp) merge, confined like a review and sending the files to its provider. Never auto-approved ([guide](docs/customizing-skills.md)) |
| `allow add SOURCE\|SKILL RULE... --reason R`, `allow list`, `allow remove SOURCE [RULE...]` | **human only**: stop reporting medium/low scanner rules for one source; high and critical are never hidden |
| `config get\|set mode\|threshold\|auto_review\|review_harness\|marketplace` | read or change settings; `set` of `mode`/`threshold` is human only |
| `reject NAME` | delete from quarantine |
| `update [NAME...]` | re-fetch approved skills; changes go to quarantine (a locally edited skill is compared with its `upstream_hash` and never auto-approved over your edits) |
| `install NAME... [--to IDS] [--scope user\|project] [--copy]`, `uninstall`, `sync` | put approved skills into harnesses, remove them, or install everything approved |
| `verify [--strict] [--require-human]` | lock, store, installs and marketplace (if `marketplace/` exists) agree (exit 1 if not); `--strict` also fails on warnings, including a still-writable approved skill; `--require-human` also fails on any `auto` or `forced` approval |
| `list`, `show [--plain]`, `status`, `doctor`, `stats --tokens` | inspect (`list`, `show` and `status` show approval mode and score; `show` renders the Markdown on a terminal, `--plain` prints it raw) |
| `plugin create\|add\|remove\|delete\|show\|list`, `marketplace build\|check` | group skills (`install --plugin NAME`); optionally generate a Claude Code / OMP marketplace to publish them (opt-in: `config set marketplace true` keeps it in sync) |
| `harness list\|add`, `rules` | the harness registry and the scanner rules |
| `lock upgrade`, `lock protect [NAME...]` | move older lock entries to hash v2, only where the old hash still matches; make approved skills read-only (permissions only, safe in an agent) |
| `init-ci` | write a GitHub workflow that runs `skigard verify` |
| `completion bash\|zsh\|fish\|powershell`, `migrate --from-skm PATH` | shell completions; import from the predecessor tool |

Every command takes `--json`, `--home PATH` and `-h`/`--help` (`skigard help COMMAND [SUB]` works too). Exit codes: `0` ok, `1` policy/drift/findings, `2` usage or error, `3` cancelled, `4` incomplete coverage.
Full reference: [docs/commands.md](docs/commands.md) (generated from the CLI).

### How skigard decides

1. **Scan**: [37 rules](docs/rules.md) plus capability chains. Any `high`/`critical` finding is a **block**; `medium` or incomplete coverage is a **review**.
2. **Review** (on by default via `auto_review` when a verified reviewer harness is installed; `--review` forces it, `config set auto_review false` turns it off): an installed harness reads the files with tools off, inside an OS sandbox where one works. It can only add doubt and only lower the score.
3. **Score** 1 to 10 with reasons. A `block` verdict or a failing review is a hard stop.
4. **Decide**: `strict` asks you on `/dev/tty`; `auto` installs at or above the threshold with no hard stop and otherwise keeps the skill in quarantine. `--force-install` overrides, for a human only.
   With `approve`, you see the scan, the review and the diff, and type the skill name; a block or failing review needs `--override --reason "..."`, which is recorded forever.
5. **Install** refuses anything whose hash no longer matches the lock. The lock records the mode (`human`, `auto`, `forced`), score and threshold.

### Where things live

`~/.config/skigard` (`%APPDATA%\skigard` on Windows; override with `$SKIGARD_HOME`). It is designed to be a private git repository: `skigard init --git`.
Layout, formats and the Windows split are in [concepts](docs/concepts.md), [lockfile](docs/lockfile.md) and [Windows notes](docs/windows.md).

## CI usage

```yaml
- run: curl -fsSL https://github.com/ambierhq/skigard/releases/download/v0.4.0/install.sh | SKIGARD_VERSION=v0.4.0 sh
- run: skigard verify --strict        # with SKIGARD_HOME=./.skigard committed to the repository
```

Pin the release, never `main`. See the [CI guide](docs/ci.md) and [`examples/verify-workflow.yml`](examples/verify-workflow.yml). Add `--require-human` to fail on any `auto` or `forced` approval. A reusable [`skigard-action`](https://github.com/ambierhq/skigard-action) (`v1`) wraps `skigard verify` for GitHub Actions.

## Supported harnesses

[78 harnesses](docs/harnesses.md) are in the registry. Three are verified so far (Claude Code, Pi, Oh My Pi); the rest are detected and listed but installed to
only with `--include-unverified`, until someone confirms the path. Missing yours? `skigard harness add --id mytool --name MyTool --global '~/.mytool/skills' --bin mytool`, or send a PR.

## FAQ

More in [docs/faq.md](docs/faq.md).

<details><summary><b>Does a high score or "pass" mean a skill is safe?</b></summary>

No. The scanner is heuristic and a review can be fooled. A 10 means the rules found nothing, not that the skill is harmless. The score is a recommendation printed with its reasons. Use `--mode strict` and read the skill when it matters; `verify --require-human` fails CI if anything was installed without a human.
</details>

<details><summary><b>Does an agent review replace reading the skill?</b></summary>

No. It is a second opinion that can only raise concern. It runs on `add` and `update` by default but nothing else depends on it; `skigard config set auto_review false` turns it off.
</details>

<details><summary><b>Why can't my agent approve skills?</b></summary>

An injected agent installing more skills is the threat. `approve` reads the terminal device only, has no `--yes`, stdin path or env bypass, and skigard refuses inside the agent harnesses it can detect (environment and parent processes, best effort); so do `--force-install` and changing `mode` or `threshold`. A process that creates its own pseudo-terminal can still type the answer, so the gate stops accidents and ordinary prompt injection, not a determined agent ([the pseudo-terminal case](docs/threat-model.md#the-pseudo-terminal-case)). What a detected agent can still do is run `skigard add` in `auto` mode, which installs only skills that clear the threshold you chose, with no hard stop. That is recorded as `mode = "auto"` in the lock. If you do not want it, set `mode` to `strict`. Signed approvals (a key in your OS keychain or a passkey) are planned.
</details>

<details><summary><b>Does it phone home?</b></summary>

No telemetry and no update check. It talks to GitHub only when you `add` or `update`. `review` (which `add` and `update` run by default, see `auto_review`) sends the skill to the provider of the reviewer harness; use `--manual`, or `skigard config set auto_review false`, to avoid that.
</details>

<details><summary><b>How is it different from vercel-labs/skills?</b></summary>

[`vercel-labs/skills`](https://github.com/vercel-labs/skills) (`npx skills`) is the popular installer and discovery CLI: `add`, `find`, `use`, `check`, `update`, `remove`, `init`, `list`, a `skills-lock.json` with a content hash, and the [skills.sh](https://skills.sh) directory.

skigard is now close to a drop-in for `add`: one `skigard add SOURCE` fetches, scans, scores and installs what clears your threshold. It also covers `update`, `remove` (`uninstall`) and `list`, and adds the checks: quarantine, a scan, a safety score, an optional review, a hash-pinned lock, optional human approval (`--mode strict`) and `verify` for drift in CI.

What you give up today: discovery and search (`skills find`, the skills.sh directory), the `skills init` template, and one-off `skills use`. Many people will find skills on skills.sh and add them through skigard.
The lock file is different (`skigard.lock`), and an importer for `skills-lock.json` is planned, not built (`skigard migrate` only reads `skm` repositories).
Sources are GitHub (public, or private with `GITHUB_TOKEN`) and local directories; other hosts such as GitLab are not supported yet.
</details>

<details><summary><b>How is it different from asm (luongnv89/asm)?</b></summary>

[`asm`](https://github.com/luongnv89/asm) is a scriptable skill manager with a large catalog, a TUI, `--json`/`--yes` automation and a pre-install scan, built to be driven by agents.
skigard differs on one point: `approve`, forcing an install and lowering your threshold need a human at a terminal (a process that builds its own pseudo-terminal can still type the answer; see the [threat model](docs/threat-model.md#the-pseudo-terminal-case)). Pick asm for catalog and convenience, skigard for a score, a lockfile and an auditable record of how each skill was approved.
</details>

<details><summary><b>Does it work on Windows?</b></summary>

Yes, on amd64 and arm64, including `approve`. Installs are copies, and automated review is disabled (use `review --manual` or WSL). See [docs/windows.md](docs/windows.md).
</details>

<details><summary><b>What happens when a skill updates?</b></summary>

`update` re-fetches; unchanged content is "up to date". In `auto` mode, changed content that scores at or above the threshold, has no hard stop and does not score lower than the approved version is approved and installed; otherwise it returns to quarantine with a warning and a diff, and the installed copy stays at the approved version. In `strict` mode changed content always returns to quarantine.
</details>

<details><summary><b>Why is my security-review skill blocked?</b></summary>

The scanner matches text, not intent, so a skill that *discusses* `~/.ssh` or "ignore previous instructions" looks like one that does it. Read it (`add` leaves it in quarantine), then `approve --override --reason "..."`. This repository's own
`skills/review-skill` is such a skill.
</details>

<details><summary><b>Does it need an agent installed?</b></summary>

No. Scan, score, approve, lock and verify need no agent. Review does.
</details>

<details><summary><b>How do I uninstall, and how do I report a malicious skill?</b></summary>

`skigard uninstall`, then delete the binary (and the home to drop approvals; approved skills are read-only, so `chmod -R u+w` the home's `skills/` first if the delete is refused). Report malicious skills to GitHub; if one gets past a rule, open an issue or PR with a defanged fixture. Flaws in skigard itself: see [SECURITY.md](SECURITY.md).
</details>

## What skigard does not protect against

- **Anyone who can write to your home as you.** Write access to `~/.config/skigard` or to a harness skills directory (`~/.claude/skills`, `~/.agents/skills`, ...) can forge an approval or add a skill without skigard being involved. skigard is a gate for the supported install path, not a sandbox for your account.
- **Forgery after the fact.** The answer is `skigard verify --strict` in CI with the home committed to a repository: a changed lock shows up as a reviewable diff ([CI guide](docs/ci.md)). Signed approvals are planned.
- **A determined agent with its own pseudo-terminal.** See [the pseudo-terminal case](docs/threat-model.md#the-pseudo-terminal-case).
- **Malicious code in a skill you approved anyway.** The scanner is heuristic and a reviewing agent can be fooled. skigard pins the bytes you approved; it does not make them safe, and it does not sandbox the skill when an agent later uses it.

## Security

Report vulnerabilities privately; see [SECURITY.md](SECURITY.md). What skigard promises, and what it does not, is in the [threat model](docs/threat-model.md).
Releases are signed (cosign, keyless), come with SBOMs and GitHub build attestations (npm packages carry npm provenance), and CI is pinned and hardened (CodeQL, govulncheck, dependency review, OpenSSF Scorecard).

## Roadmap

- Install channels still to land: winget (in review, #484914), nixpkgs (in review, #590787) and the mise registry.
- `skigard find` (#28): search skills.sh or a catalog (discovery is the main thing skigard lacks today).
- An importer for `skills-lock.json` (#29) from vercel-labs/skills.
- Verify more harnesses (each one needs a person to confirm its skills directory).
- Automated review on Windows, once the reviewer CLIs have been verified there.
- Signed approvals (approver key in the OS keychain or a passkey, signature stored in `skigard.lock`, checked by `verify --strict`).

Ideas and votes: [Discussions](https://github.com/ambierhq/skigard/discussions). Changes so far: [CHANGELOG.md](CHANGELOG.md).

## Contributing

Issues and PRs are welcome. Start with [CONTRIBUTING.md](CONTRIBUTING.md) (adding a harness, a scanner rule or a reviewer template) and the [code of conduct](CODE_OF_CONDUCT.md).
Questions: [SUPPORT.md](SUPPORT.md).

```sh
mise install && mise run check && mise run e2e
```

## Acknowledgements

skigard stands on the shoulders of others. We learned from these projects and reimplemented the ideas; no code was copied. Please go and look at them:

- [SkillGuardrail](https://github.com/T-Zevin/SkillGuardrail) by T-Zevin: quarantine-first fetching, hostile-archive handling, hash-bound baselines, atomic staged installs, capability chains.
- [SkillSpector](https://github.com/NVIDIA/SkillSpector) by NVIDIA: rule IDs, SARIF, "coverage incomplete", and the capability-stripped way of using a local agent CLI as a reviewer.
- [agent-skill-manager (asm)](https://github.com/luongnv89/asm) by luongnv89: `doctor`, `--json` everywhere, context cost reporting, harness definitions as data.
- [vercel-labs/skills](https://github.com/vercel-labs/skills) by Vercel: the map of the agent-harness landscape, the universal `.agents/skills` directory, and a deterministic content hash for a skill folder.
- [skillpin](https://github.com/nbsp1221/skillpin) by nbsp1221: a manifest and lockfile pinned to exact commits.
- The [Agent Skills specification](https://agentskills.io) for the skill format, and [BurntSushi/toml](https://github.com/BurntSushi/toml) (MIT), the one library inside the binary.

Full notices: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## License

[MIT](LICENSE), Copyright (c) Ambier Oy.

<p align="center">Built by <a href="https://ambierhq.com">Ambier Oy</a></p>
