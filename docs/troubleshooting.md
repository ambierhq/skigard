# Troubleshooting

Start with `skigard doctor` (checks the environment) and `skigard status` (summarises this home). Add `--json` to any command for details.

## `approve` says it needs a terminal, or refuses

`approve` reads the terminal device only: `/dev/tty` on Linux and macOS, the console (`CONIN$`) on Windows. It fails in CI, cron, containers without
`-it`, and when stdin is a pipe with no controlling terminal. Run it from an interactive terminal. It also refuses when it detects that it is
running inside an agent harness (for example Claude Code); open your own terminal window. This cannot be disabled.

## A skill I trust is blocked

The scanner matches text, not intent: a skill that mentions `~/.ssh`, `curl | sh` or "ignore previous instructions" looks like one that does it.
Read the skill and the finding (`skigard scan NAME`), then `skigard approve NAME --override --reason "..."`. Details in
[writing safe skills](writing-safe-skills.md).

## `install` refuses with a hash mismatch

The approved copy in `skills/<name>` no longer matches the lock. Something edited it, or a sync tool rewrote line endings. Run `skigard verify` to see what
changed. Restore the files from your home repository (`git checkout -- skills/NAME`), or re-add and re-approve the skill.

## `verify` reports an exec-bit change

A file gained or lost its execute bit since approval (same bytes, different mode). On Linux and macOS this is reported as drift on purpose. On Windows the
exec set is "not applicable". See [lockfile](lockfile.md#content-hash-and-exec-set).

## `lock upgrade` says the hash does not match

It only upgrades an entry when the old version 1 hash of `skills/<name>` still equals the locked one. If it does not, the files changed since approval (nothing is
modified). A skill with executable scripts can only be upgraded on Linux or macOS. Re-fetch and re-approve if you cannot restore the original.

## A skill is approved but my agent does not see it

- `skigard detect` must list the agent as `installed` or `configured`.
- Only **verified** harnesses are installed to by default. For others use `skigard install NAME --to ID --include-unverified`, and send a PR
  confirming the path ([harnesses](harnesses.md)).
- Windows installs are copies; run `skigard sync` after approving a new version.
- Restart the agent. Most read skills at start-up.

## Review failed or is unavailable

- `skigard review --list` shows harnesses that can review headlessly and are installed.
- Without an OS sandbox, a verified template only runs with `--allow-policy-only` ([review](review.md#what-the-reviewer-sees-and-cannot-do)).
- On Windows automated review is disabled; use `--manual` and `skigard review record`.
- Review is optional. Scan, approve, lock and verify work without any agent.

## `add` fails to fetch

- GitHub only, over HTTPS, with redirects limited to GitHub hosts. Check the ref (`github:owner/repo/path@ref`). For a private repository or a rate limit, set `GITHUB_TOKEN` (or `GH_TOKEN`), or clone it yourself and `skigard add ./path`.
- Behind a proxy, set `HTTPS_PROXY`. Private or loopback addresses are refused after DNS resolution.
- A local source that contains symlinks or junctions is refused.

## Another skigard process is using the home

Mutating commands take a lock on `state/.lock` and wait up to ten seconds. If a previous run crashed, the OS releases the lock; just rerun.
If it persists, make sure no sync tool is holding the home open.

## Exit codes

`0` ok, `1` policy, drift or findings, `2` usage or error, `3` cancelled, `4` incomplete scan coverage ([commands](commands.md#exit-codes)).

## Still stuck

Ask in [GitHub Discussions](https://github.com/ambierhq/skigard/discussions) or open an [issue](https://github.com/ambierhq/skigard/issues/new/choose).
For security problems follow [SECURITY.md](../SECURITY.md); do not open a public issue.
