# `skigard.lock` and the content hash

`skigard.lock` lives in your skigard home (default `~/.config/skigard`) or, for a project, in `./.skigard/`. It is
TOML, written deterministically (sorted by name), and meant to be committed and reviewed like code.

```toml
version = 1
hash_version = 1

[[skill]]
  name = "archify"
  source = "github:tt-a1i/archify/archify"
  location = "https://github.com/tt-a1i/archify"
  ref = "v1.2"                  # branch/tag you asked for (omitted = default branch)
  subpath = "archify"           # where the skill lives inside the repository
  commit = "44781293f38728baf04dbad33ce1a0cef8c39cca"
  content_hash = "sha256:ee3a63eb39d75ba173b4f1f5a0d4cf2b3ee1fbb10daf78705d55a043fab2914a"

  [skill.scan]
    tool = "skigard 0.1.0"
    rules = "2026.1"
    verdict = "block"           # pass | review | block
    coverage = "complete"       # complete | partial
    risk = 100
    findings = 90

  [skill.review]                # present only if a review was recorded for exactly this content
    reviewer = "pi"             # a harness id, or "manual"
    verdict = "pass"            # pass | concerns | fail (advisory)
    coverage = "complete"
    record = "reviews/archify/ee3a63eb39d7.json"

  [skill.approval]
    by = "dzung"
    at = "2026-11-17T20:35:53Z"
    override = true             # approved despite block / partial coverage / failing review
    reason = "read it; the credential regex is a deny-list"
    [[skill.approval.accepted]] # findings accepted with a reason (low/medium only), optionally expiring
      key = "..."
      reason = "..."
      expires = "2027-01-01"
```

`reviews/<name>/<hash12>.json` holds the evidence for each approval: the full scan report, the review record and the
approval.

## What each command trusts

| Command | Reads the lock | Verifies |
|---|---|---|
| `install`, `sync` | which skills are approved | recomputes `content_hash` of `skills/<name>` and refuses on mismatch |
| `verify` | everything | lock vs `skills/` vs install records vs marketplace; unapproved directories in `skills/` are errors |
| `update` | source, ref, subpath | re-fetches; an unchanged hash is "up to date", a changed one goes to quarantine |

## Content hash, version 1

```
files   = every regular file under the skill directory
order   = sorted by the bytes of the slash-separated relative path
for each file:
    "F" NUL path NUL mode NUL length NUL content
mode    = "x" if any execute bit is set, else "-"
hash    = "sha256:" + hex(sha256(concatenation))
```

- Empty directories are ignored. Symlinks and special files are an error: a skill containing one cannot be hashed
  or approved.
- There is no line-ending normalisation.
- The scheme is versioned (`hash_version`). A new scheme gets a new number; old locks keep verifying.
- The test vector in `internal/hash/hash_test.go` pins the output, and was cross-checked against an independent
  Python implementation.

Example: files `SKILL.md` = `hello\n` (mode `-`) and `scripts/run.sh` = `echo hi\n` (mode `x`) hash to
`sha256:b1a18ee21c6d18ec9d1472a89236ace9501b5c187a48bd24f643988b611e90c9`.
