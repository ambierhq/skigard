# Using skigard in CI

Two jobs fit CI, and neither needs an agent or a terminal:

- **`skigard verify`** fails the build when the lockfile, the approved store, the installs or the generated marketplace no longer agree.
  It catches a skill that changed without a re-approval, an unapproved directory that appeared, or a lock edited by hand.
- **`skigard scan`** checks a skill you are *writing* (or reviewing in a PR) and can emit SARIF.

`approve` cannot run in CI. It reads a terminal, has no `--yes`, and refuses inside agent harnesses. That is the point.

## Verify a repository's pinned skills

Commit a project-scoped home to the repository (`.skigard/`, containing `skigard.lock`, `skills/`, `reviews/`) and point skigard at it:

```yaml
# .github/workflows/skills.yml
name: skills
on: [pull_request]
permissions: {}
jobs:
  verify:
    runs-on: ubuntu-latest
    permissions: {contents: read}
    env:
      SKIGARD_HOME: ${{ github.workspace }}/.skigard
    steps:
      - uses: actions/checkout@v4          # pin to a full commit SHA in your own repository
        with: {persist-credentials: false}
      - run: curl -fsSL https://raw.githubusercontent.com/ambierhq/skigard/main/scripts/install.sh | sh
      - run: skigard verify --strict
```

A copy lives in [`examples/verify-workflow.yml`](../examples/verify-workflow.yml). Pin the installer to a release with `SKIGARD_VERSION=v0.1.1`, so
a CI run cannot change behaviour under you.

`--strict` also fails on warnings, such as an approval that used `--override` or a review that did not pass. Without it, only errors fail.
Exit codes: `0` ok, `1` drift or policy failure, `2` usage or error ([reference](commands.md#exit-codes)). Use `--json` for machine-readable results:

```sh
skigard verify --strict --json | jq '.problems[] | select(.level == "error")'
```

On Windows runners use the [PowerShell installer](install/windows.md). `verify` checks content hashes everywhere and the exec set on Linux
and macOS only.

## Scan a skill in a pull request

```sh
skigard scan path/to/skill --fail-on high --fail-on-incomplete
skigard scan path/to/skill --format sarif > skigard.sarif     # for GitHub code scanning
```

`--fail-on` takes a severity (`low`, `medium`, `high`, `critical`). `--fail-on-incomplete` also fails (exit 4) when coverage is partial (binary or oversized files). `--output FILE` writes the report to a file.
A skill that talks about secrets or `curl | sh` in its text will be flagged; see [writing safe skills](writing-safe-skills.md).

## The GitHub Action

[`ambierhq/skigard-action`](https://github.com/ambierhq/skigard-action) installs a pinned, SHA-256-verified skigard release and runs
`skigard verify --strict` (inputs: `version` required, optional `sha256`, `args`, `home` = `.skigard`, `working-directory`, `verify`). It has no
`latest`: you pin a version. It never approves anything. Pin the action itself by commit SHA like every action in a security-sensitive workflow (`v1.0.0` is `71259a27cb1217ca715054d849f16900d495a041`; the moving `v1` tag also exists):

```yaml
- uses: actions/checkout@<full-commit-sha>
  with: {persist-credentials: false}
- uses: ambierhq/skigard-action@<full-commit-sha>
  with:
    version: 0.1.1
    sha256: <archive checksum from the release's checksums.txt>   # optional: pins the exact bytes
    verify-signature: true                                        # optional: cosign-verify checksums.txt (needs cosign on the runner, see below)
```

`skigard init-ci` writes this workflow to `.github/workflows/skigard.yml` with placeholders for the action SHAs (it refuses to overwrite
without `--force`). With `verify-signature: true` the action runs `cosign verify-blob` on `checksums.txt` against the release workflow's identity
(add `sigstore/cosign-installer` to the job first); pinning `sha256` is the other strong option, and the two combine.

## Keep the lock honest in review

Treat `skigard.lock` like a dependency lockfile: review its diff in PRs. A change to `content_hash` means the bytes changed; `[skill.approval]` shows
who approved and when; `override` entries stay visible forever. Pair it with a `CODEOWNERS` rule so lock changes need a maintainer.
