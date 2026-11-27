# AGENTS.md

skigard is a Go CLI that recommends and gates agent skills: fetch to quarantine, scan, advisory agent review, a safety score
(1..10), then either human approval (`strict`) or auto-approval at or above a threshold (`auto`), hash-pinned lockfile, install into
detected agent harnesses. Read `README.md` for usage and `docs/threat-model.md`
for what it promises.

## Commands

```sh
mise install            # go, golangci-lint, editorconfig-checker
mise run check          # lint (golangci-lint, gofmt/goimports, editorconfig) + go test ./...
mise run fmt            # format Go
mise run e2e            # builds the binary, drives `approve` through a real pty
mise run fuzz           # short fuzz runs of the parsers
mise run docs           # regenerate docs/harnesses.md and docs/rules.md
go test -race ./...
```

Run `mise run check` and `mise run e2e` before finishing any change.

## Layout

- `cmd/skigard` entrypoint. `internal/cli` flags, output and help (every command and subcommand takes `-h`; group subcommands live in one table that feeds usage, completion and `docs/commands.md`) only; workflows live in `internal/core`.
- `internal/source` (parse + fetch), `safefs` (hostile-input copy/extract), `hash` (content hash v1 legacy, v2 current),
  `scan` (rules), `review` (reviewer runner: tools off, empty cwd, scrubbed env; OS sandbox: seatbelt/bwrap/landlock; `--allow-policy-only` affects only review, never approve), `harness` (registry, detection, install),
  `store` (home layout, lock, manifest, quarantine meta), `marketplace` (opt-in: `verify` checks it only when `marketplace/` exists or `settings.marketplace` is on), `diff`, `skill`, `util`.
- `internal/harness/registry.toml` is the harness table. `docs/harnesses.md` and `docs/rules.md` are generated; never edit by hand.

## Rules that must not be broken

- **An agent must never be able to use `approve`, to force an install, or to change `mode`/`threshold`. In `auto` mode an agent
  that runs `skigard add` can install skills that clear the human-chosen threshold; that is by design, recorded as `mode = "auto"`
  in the lock, and visible to `verify --require-human`.** `approve` reads `/dev/tty` only. Do not add `--yes`, a stdin path,
  an env-var bypass or a test hook to `ttyConfirm`. The auto path is a separate core function with no `Confirmer`, and it is never
  reachable with a weaker threshold or mode than the stored one from inside an agent. `--force-install` and `config set mode|threshold`
  need a human terminal and are refused in an agent (`a.RefuseInAgent`).
  Test approval through `core.Approve` with an injected `Confirmer`.
  Never call `approve` or a strict-mode prompt through the CLI in a Go test (it would block on a developer's terminal).
- **Local forks.** `edit` (copies to quarantine), `merge --with` and `lock protect` are allowed in an agent: they only change quarantine or permissions. `merge --pick` and `approve` are not (`RefuseInAgent`). `adopt` takes the same decision as `add` (shared assess/promote step, same mode/threshold floor in an agent, no force, no `Confirmer`) and may approve an edit as `mode = "auto"`; a merge result is never auto-approved. A human-approved fork is `mode = "local"`, both carry `upstream_hash`. Approved skills are read-only (`0555`/`0444`, not on Windows): a deterrent, the lock hash is the control. `update` never replaces a fork's edits, even in `auto`. The merge agent's output is untrusted: strict parser, scanner, human approval.
- **Score and threshold.** `scan.Score` is pure and has a table test. It returns the number, the tier and the reasons; the reviewer can
  only lower it. A scanner `block` and a reviewer `fail` are hard stops that no threshold can install. Changing the formula bumps
  `ScoreVersion`. The score is always printed with its reasons, never alone. `settings.mode`/`threshold`/`auto_review` are validated on load;
  `auto_review` defaults to false because review sends the skill to a model provider.
- **Never execute anything from a skill**, and never invoke `git`. Skill content goes to a reviewer on stdin only,
  never in argv.
- **The reviewer can only raise concern**, never lower a scanner finding or raise the score.
- All untrusted text printed to a terminal goes through `util.Clean`; evidence goes through `util.MaskSecrets`.
- Names from disk (lock, manifest, meta) are validated with `util.CheckName` on load. Paths from archives go through `os.Root`.
- Lock approval records (`mode` = human|local|auto|forced, `score`, `threshold`, `score_version`) and `upstream_hash` are additive and `omitempty`; the lock `version` stays 1 and old entries read as `human`.
- Mutating `core` entry points take `a.Home.Lock()`. Do not call a locking method from another locking one (it deadlocks);
  use the lower-case helper (see `marketplaceBuild`).
- The content hash is versioned. Changing it needs a new `hash_version`; the vector in `internal/hash/hash_test.go` pins it.
- One runtime dependency (the TOML parser). Ask before adding another.
- Do not copy code from other projects; reimplement and credit them in `THIRD_PARTY_NOTICES.md`.

## Conventions

- Errors for users: `util.Errf` (exit 2), `util.Policyf` (exit 1), `util.Cancelled` (exit 3).
- A new scanner rule needs a `Pre` prefilter, samples in `TestPrefilterCoversSamples`, a fixture in
  `TestMaliciousFixtures`, a bumped `RulesVersion`, and `mise run docs`.
- A new harness starts `verified = false`; add evidence in the PR. A new reviewer template must disable tools and MCP.
- `docs/commands.md`, `docs/harnesses.md` and `docs/rules.md` are generated (`mise run docs`); a new user-facing flow also gets a hand-written page under `docs/` and, for the website, an entry in `website/astro.config.mjs`.
- Tests must behave the same inside and outside an agent (clear agent env vars, see `internal/cli/cli_test.go`).
- Code style is enforced by `.golangci.yml` and `.editorconfig`. Fix lint findings instead of adding `nolint`;
  if one is justified, state why on the same line.
