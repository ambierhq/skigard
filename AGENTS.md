# AGENTS.md

skigard is a Go CLI that gates agent skills: fetch to quarantine, scan, advisory agent review, **human** approval,
hash-pinned lockfile, install into detected agent harnesses. Read `README.md` for usage and `docs/threat-model.md`
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

- `cmd/skigard` entrypoint. `internal/cli` flags and output only; workflows live in `internal/core`.
- `internal/source` (parse + fetch), `safefs` (hostile-input copy/extract), `hash` (content hash v1),
  `scan` (rules), `review` (sandboxed reviewer runner), `harness` (registry, detection, install),
  `store` (home layout, lock, manifest, quarantine meta), `marketplace`, `diff`, `skill`, `util`.
- `internal/harness/registry.toml` is the harness table. `docs/harnesses.md` and `docs/rules.md` are generated; never edit by hand.

## Rules that must not be broken

- **An agent must never be able to approve.** `approve` reads `/dev/tty` only. Do not add `--yes`, a stdin path,
  an env-var bypass or a test hook to `ttyConfirm`. Test approval through `core.Approve` with an injected `Confirmer`.
  Never call `approve` through the CLI in a Go test (it would block on a developer's terminal).
- **Never execute anything from a skill**, and never invoke `git`. Skill content goes to a reviewer on stdin only,
  never in argv.
- **The reviewer can only raise concern**, never lower a scanner finding.
- All untrusted text printed to a terminal goes through `util.Clean`; evidence goes through `util.MaskSecrets`.
- Names from disk (lock, manifest, meta) are validated with `util.CheckName` on load. Paths from archives go through `os.Root`.
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
- Tests must behave the same inside and outside an agent (clear agent env vars, see `internal/cli/cli_test.go`).
- Code style is enforced by `.golangci.yml` and `.editorconfig`. Fix lint findings instead of adding `nolint`;
  if one is justified, state why on the same line.
