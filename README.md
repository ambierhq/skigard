# skigard

**A lockfile and review gate for agent skills.** Nothing reaches your agents until a human has approved those exact bytes.

skigard fetches skills (a folder with a `SKILL.md`) from GitHub or disk into **quarantine**, scans them, optionally has an
agent you already use review them, makes **you** approve them in a real terminal, pins the approval to a **content hash**
in a lockfile, and then installs them into every agent harness it finds on your machine (Claude Code, Pi, Codex, Cursor,
Gemini CLI, OpenCode, and about 75 more).

```
add ──► quarantine ──► scan ──► review (agent, advisory) ──► approve (human, /dev/tty) ──► skigard.lock ──► install
 ▲                                                                                              │
 └──────────────────────────── update: re-fetch, diff, review again ◄──────────────────────────┘
```

- **One static binary**, no runtime, no telemetry, one dependency (a TOML parser).
- **Agents cannot approve.** `approve` reads `/dev/tty`, has no `--yes`, and refuses inside a detected agent harness.
- **Reproducible.** `skigard.lock` records source, commit, content hash, scan result, review and approver. Commit it.
- **Drift is caught.** `skigard verify` fails if the approved store, the installs or the generated marketplace change.
- **Auto-detects harnesses** and installs once per shared directory (`~/.agents/skills`).

> The scanner is heuristic and an agent review can be fooled. "Pass" is not a safety certificate. Read the skill,
> especially bundled scripts. See the [threat model](docs/threat-model.md).

## Install

```sh
# macOS / Linux (Homebrew tap and release binaries are published from GitHub Releases)
curl -fsSL https://raw.githubusercontent.com/ambierhq/skigard/main/scripts/install.sh | sh

# Windows (PowerShell 5.1 or 7, no administrator rights)
irm https://raw.githubusercontent.com/ambierhq/skigard/main/scripts/install.ps1 | iex

# from source (Go 1.27+)
go install github.com/ambierhq/skigard/cmd/skigard@latest
```

Windows: everything works, including `approve`, which reads the console (`CONIN$`) the way Unix reads `/dev/tty`.
See [docs/windows.md](docs/windows.md) for file locations and what differs (automated review is disabled; use `review --manual`).

## Quick start

```sh
skigard detect                              # which agent harnesses are on this machine?
skigard add github:tt-a1i/archify --skill archify   # fetch into quarantine and scan
skigard review archify                      # optional second opinion from pi / claude / omp (asks which)
skigard approve archify                     # in a real terminal: shows scan, review and diff, you type the name
skigard install archify                     # symlink into every detected, verified harness
skigard verify                              # CI-friendly: lock vs store vs installs
```

Update later: `skigard update` re-fetches approved skills; anything that changed goes back to quarantine with a diff,
and has to be reviewed and approved again.

New machine: clone your home repo to `~/.config/skigard`, then `skigard sync`.

## What lives where

Everything is under `~/.config/skigard` (override with `$SKIGARD_HOME`; honours `$XDG_CONFIG_HOME`). It is designed to
be a private git repository: `skigard init --git`.

```
~/.config/skigard/
├── skigard.toml        settings, plugins, custom harnesses
├── skigard.lock        approved skills pinned to hashes   (commit)
├── skills/<name>/      approved content                    (commit)
├── reviews/<name>/     evidence for each approval          (commit)
├── marketplace/        generated Claude Code / OMP marketplace (commit)
├── quarantine/         UNTRUSTED, mode 0700                (gitignored)
└── state/              install records, lock file          (gitignored)
```

Installs are symlinks from each harness's skills directory into `skills/`, or copies with `--copy` (and always for
`--scope project`, so a repository is self-contained).

## Commands

| Command | |
|---|---|
| `init [--git]` | create the home |
| `detect [--all]` | list harnesses: `installed` (binary/app), `configured` (config dir only), `absent` |
| `add SOURCE [--skill N \| --all]` | fetch to quarantine, scan. `SOURCE`: `github:owner/repo[/path][@ref]`, `owner/repo`, a github.com URL, or a local path |
| `scan NAME\|PATH [--format text\|json\|sarif] [--fail-on LEVEL] [--fail-on-incomplete]` | deterministic scan |
| `review NAME [--harness ID \| --manual]` | advisory agent review ([how it works](docs/review.md)) |
| `diff NAME` | quarantined vs approved |
| `approve NAME [--override --reason R] [--accept R]` | **human only** |
| `reject NAME` | delete from quarantine |
| `update [NAME...]` | re-fetch approved skills; changes go to quarantine |
| `install NAME... [--plugin P] [--to IDS] [--scope user\|project] [--copy] [--include-unverified]` | install approved skills |
| `uninstall [NAME...]`, `sync` | remove what skigard installed / install everything approved |
| `verify [--strict]` | lock, store, installs and marketplace agree (exit 1 if not) |
| `list`, `show NAME`, `status`, `doctor`, `stats --tokens` | inspect |
| `plugin create\|add\|remove\|show\|list`, `marketplace build\|check` | group skills; generate a Claude Code / OMP marketplace |
| `harness list\|add`, `rules` | the harness registry and the scanner rules |
| `completion bash\|zsh\|fish\|powershell` | print a shell completion script |
| `migrate --from-skm PATH` | import from the predecessor tool |

Every command takes `--json` and `--home PATH`. Exit codes: `0` ok, `1` policy/drift/findings, `2` usage or error,
`3` cancelled, `4` incomplete scan coverage.

## How the gate decides

1. **Scan** (`skigard scan`): [34 rules](docs/rules.md) plus capability chains. Any `high`/`critical` finding is
   a **block**; `medium` or incomplete coverage is a **review**. Binary or oversized files make coverage *partial*.
2. **Review** (optional): an installed harness reads the files with all tools disabled. It can only add doubt.
3. **Approve**: you see the scan, the review and the diff against what you approved before, and type the skill name.
   A block, partial coverage or a failing review needs `--override --reason "..."`, which is recorded and shown by
   `status` and `verify` forever. `--accept REASON` can accept low/medium findings (never high/critical).
4. **Install** refuses anything whose hash no longer matches the lock.

## Harness support

[78 harnesses](docs/harnesses.md) are in the registry. Three are verified so far (Claude Code, Pi, Oh My Pi); the rest are
detected and listed but installed to only with `--include-unverified`, until someone confirms the path. Missing yours?
`skigard harness add --id mytool --name MyTool --global '~/.mytool/skills' --bin mytool`, or send a PR.

## Using it in CI and projects

```yaml
# .github/workflows/skills.yml  (see examples/)
- run: skigard verify --strict
```

`skigard install ... --scope project` copies approved skills into the repository's harness directories. Commit
`./.skigard/skigard.lock` with `SKIGARD_HOME=./.skigard`.

## FAQ

**Why is my security-review skill (or `archify`) blocked?** The scanner matches text, not intent. A skill that *talks
about* `~/.ssh`, `curl | sh` or "ignore previous instructions" (a checklist, a deny-list regex) looks like one that does
it. High and critical findings cannot be accepted away; read the skill, then approve with `--override --reason "..."`.
The override is recorded and `status`/`verify` keep showing it. This repository's own `skills/review-skill` is such a skill.

**Can an agent run `skigard review`?** Yes, review is advisory and costs your own plan. It cannot approve.

**Does it work without an agent installed?** Yes. Scan, approve, lock and verify need no agent at all.

**Which models see my skill?** Only if you run `review`: the harness you choose sends it to its provider. Use `--manual` to avoid that.

## Development

```sh
mise install          # go 1.27
mise run check        # golangci-lint + editorconfig + tests
mise run fmt          # format
mise run e2e          # builds the binary and drives approve through a real pty
mise run fuzz         # short fuzz runs of the parsers
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).

## Credits

skigard stands on the shoulders of others. We learned from these projects and reimplemented the ideas; no code was
copied. Please go and look at them:

- [SkillGuardrail](https://github.com/T-Zevin/SkillGuardrail) by T-Zevin: quarantine-first fetching, hostile-archive
  handling, hash-bound baselines, atomic staged installs, receipts outside the skill tree, capability chains.
- [SkillSpector](https://github.com/NVIDIA/SkillSpector) by NVIDIA: rule IDs, SARIF, "coverage incomplete", and the
  capability-stripped way of using a local agent CLI as a reviewer.
- [agent-skill-manager (asm)](https://github.com/luongnv89/asm) by luongnv89: `doctor`, `--json` everywhere, context
  cost reporting, harness definitions as data.
- [vercel-labs/skills](https://github.com/vercel-labs/skills) by Vercel: the map of the agent-harness landscape, the
  universal `.agents/skills` directory, and a deterministic content hash for a skill folder.
- [skillpin](https://github.com/nbsp1221/skillpin) by nbsp1221: a manifest and lockfile pinned to exact commits.

See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). skigard is [MIT licensed](LICENSE).
