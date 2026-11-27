# `skigard.lock` and the content hash

`skigard.lock` lives in your skigard home (default `~/.config/skigard`) or, for a project, in `./.skigard/`. It is
TOML, written deterministically (sorted by name), and meant to be committed and reviewed like code.

```toml
version = 1
hash_version = 2                # default scheme for entries that carry no hash_version of their own

[[skill]]
  name = "archify"
  source = "github:tt-a1i/archify/archify"
  location = "https://github.com/tt-a1i/archify"
  ref = "v1.2"                  # branch/tag you asked for (omitted = default branch)
  subpath = "archify"           # where the skill lives inside the repository
  commit = "44781293f38728baf04dbad33ce1a0cef8c39cca"
  content_hash = "sha256:ee3a63eb39d75ba173b4f1f5a0d4cf2b3ee1fbb10daf78705d55a043fab2914a"
  hash_version = 2              # 1 = legacy (mode inside the hash); see "Content hash" below
  exec_set = ["scripts/run.sh"] # files that had an execute bit at fetch time (hash_version 2; omitted when empty)
  exec_hash = "sha256:..."      # digest of exec_set (always present for hash_version 2, also when the set is empty)
  # upgraded_from = "sha256:..." # only after `skigard lock upgrade`: the version 1 hash this entry replaced
  # upstream_hash = "sha256:..."  # only for a local fork: the upstream content it was edited from (see below)

  [skill.scan]
    tool = "skigard 0.3.0"
    rules = "2026.4"
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
    by = "dzung"                 # the person for human and forced; "skigard (auto)" for mode = auto
    at = "2026-11-17T20:35:53Z"
    mode = "human"               # human | local | auto | forced; absent in older locks and read as human
    score = 9                   # safety score 1..10 at approval time (omitted when not recorded)
    threshold = 7               # threshold in force when `add` decided (omitted for a plain `approve`)
    score_version = "2"         # formula version that produced score
    override = true             # approved despite block / partial coverage / failing review
    reason = "read it; the credential regex is a deny-list"
    [[skill.approval.accepted]] # findings accepted with a reason (low/medium only), optionally expiring
      key = "..."
      reason = "..."
      expires = "2027-01-01"
```

### Approval fields

- `mode`: `human` (a person confirmed on the terminal with `approve`, or in strict mode), `local` (a person approved their own edits of a skill that was staged with `skigard adopt`; see `upstream_hash`), `auto` (`add`, `update` or `adopt` in auto mode, no prompt; an `adopt` entry also carries `upstream_hash`) or `forced`
  (`add --force-install`). Locks without these fields read as `human`.
- `by`: who decided. For `human` and `forced` it is the OS user (or `--by`); for `auto` it is always `skigard (auto)`, because a machine decided, not a person. That value is reserved: a lock that uses it with another mode fails to load. Locks written before this change may carry the OS user with `mode = "auto"`; rely on `mode`, not `by`.
- `score`, `threshold`, `score_version`: the safety score (1..10, 10 = no concern), the threshold in force, and the version of the formula. `approve` records the score and `score_version` but no threshold; `add` (auto, forced, or confirmed in strict mode) records all three. A formula change bumps `score_version`, like the rules version.
- `upstream_hash` (on the skill entry, not under `approval`): the upstream content hash, in the entry's own `hash_version`, when the approved content is a **local fork**. Absent means `content_hash` is the upstream content. `adopt` pins the baseline (the previous `upstream_hash`, else the previous `content_hash`); `update` compares the freshly fetched upstream with it, so local edits are never mistaken for an upstream change and are never replaced silently. The lock stores no copy of the old upstream, so comparisons are two-way (your copy against the new upstream). `edit` keeps the approved version in `skills/` to compare with; for an edit made in place, `adopt` tries to rebuild the upstream in quarantine (see [concepts](concepts.md#editing-a-skill)). A malformed value fails to load. `adopt` needs a `hash_version = 2` entry. The walkthrough is in [customizing a skill](customizing-skills.md).
- All four are additive and `omitempty`; the lock `version` stays 1. `skigard verify --require-human` fails if any approval is `auto` or `forced`; `local` passes, because a person approved those exact bytes, but `list`, `status` (a `local` count) and `show` keep it distinguishable from `human`. `list`, `show` and `status` display mode and score.

`reviews/<name>/<hash12>.json` holds the evidence for each approval: the full scan report, the review record and the
approval.

## What each command trusts

| Command | Reads the lock | Verifies |
|---|---|---|
| `install`, `sync` | which skills are approved | recomputes `content_hash` (and, on Unix, the exec set) of `skills/<name>` and refuses on mismatch |
| `verify` | everything | lock vs `skills/` vs install records vs marketplace (when built, or required when `settings.marketplace` is on); unapproved directories in `skills/` are errors; a still-writable approved skill (approved before read-only existed) is a warning, fixed by `skigard lock protect` |
| `update` | source, ref, subpath, approval score | re-fetches; unchanged content and exec set (for a local fork, unchanged against `upstream_hash`) is "up to date". A change goes to quarantine (always, when the skill is a local fork or has unadopted local edits), except in `auto` mode when it scores at or above the threshold with no hard stop and not lower than the approved score: then it is approved and installed |
| `lock upgrade` | hash versions | upgrades version 1 entries to version 2, only where the version 1 hash still matches |

## Content hash and exec set

Every entry says which scheme produced its `content_hash` (`hash_version`). New approvals use version 2. Entries from
earlier releases are version 1 and keep verifying with the version 1 code until you run `skigard lock upgrade`.

### Version 2 (current): content identity, the same on every OS

```
files   = every regular file under the skill directory
order   = sorted by the bytes of the slash-separated relative path
for each file:
    "F" NUL path NUL length NUL content
hash    = "sha256:" + hex(sha256(concatenation))
```

- The file mode is **not** an input, so the hash is identical on Linux, macOS and Windows and for the same archive.
- Paths are the UTF-8 bytes as stored: no Unicode normalisation. Names that collide case-insensitively are rejected
  at fetch time. There is no line-ending normalisation: bytes are never rewritten.
- Empty directories are ignored. Symlinks and special files are an error: a skill containing one cannot be hashed
  or approved.

Execute bits are bound separately, by the **exec set**:

```
exec_set  = sorted slash-separated paths that had any execute bit (mode & 0111) in the SOURCE
exec_hash = "sha256:" + hex(sha256( for each path: "E" NUL decimal-length NUL path NUL ))
```

- Where the mode is taken from, at fetch time: for GitHub sources, the **tar header** (git's 100755 vs 100644),
  recorded before anything is written to disk, so it is also correct when you fetch on Windows. For local sources
  (a directory, an `skm` import) it is the file mode on disk; on Windows that is always "not executable".
- It is kept in the quarantine record, checked again before and after staging at `approve`, and written to the lock
  entry next to `content_hash`. `exec_hash` is present even for an empty set, so removing the field is a change.
  A lock whose `exec_set` and `exec_hash` disagree is refused when loaded.
- `verify`, `install`, `sync` and `list` compare the set found on disk with the lock on Linux and macOS. A file that
  gained or lost an execute bit is reported as drift and `install` refuses it, even though `content_hash` is unchanged.
- On Windows, which has no execute bit, the exec set is **not applicable**: it is not checked there, `verify` prints an
  `info` line saying so, and nothing is reported as matching that was not compared. The content hash is still fully
  checked. The exec set in the lock is whatever the source said, so the same lock stays enforceable on Unix.

What each digest binds: `content_hash` binds the names and bytes (and so the set of files). `exec_hash` binds which of
those files are executable. Neither binds timestamps, owners, or other permission bits (installs are written 0644/0755).

### Version 1 (legacy)

```
"F" NUL path NUL mode NUL length NUL content        mode = "x" if any execute bit is set, else "-"
```

Same ordering and rules as above. The mode was inside the hash, so Windows (which sees every file as `-`) computed a
different hash for any skill with executable scripts. Version 1 entries have no `exec_set`: the mode is part of the
hash. They verify exactly as before on Unix; on Windows they verify only if the skill has no executable files.

### Upgrading: `skigard lock upgrade [NAME...]`

For each version 1 entry it recomputes the **version 1** hash of `skills/<name>`. Only if that equals the locked hash
(so the bytes and execute bits are exactly what you approved) does it derive the version 2 hash and exec set from
the same files, and rewrite the entry with `hash_version = 2` and `upgraded_from = <old hash>`. If it does not match
nothing is changed and it says so. It never fetches, never approves, and leaves scan, review and approval records
untouched. A skill with executable scripts can only be upgraded on Linux or macOS, because Windows cannot reproduce
its version 1 hash. A version 2 lock read by an older skigard fails closed (every hash mismatches), it is never
trusted wrongly.

The scheme is versioned; a new scheme gets a new number and old locks keep verifying. The test vectors in
`internal/hash/hash_test.go` pin both outputs (version 2 was cross-checked against an independent Python
implementation), and `internal/hash/testdata/golden-skill` is hashed on Linux, macOS and Windows CI to the same value.

Example (version 2): files `SKILL.md` = `hello\n` and `scripts/run.sh` = `echo hi\n` hash to
`sha256:1dbfa1cf29367d433069f3615ffd1b94d245220fcb439a97ee65dee17f879e2d`, whatever their modes. With the script
executable, `exec_set = ["scripts/run.sh"]`. (Version 1 gave `sha256:b1a18ee21c6d...` with mode `x`, `sha256:8b96a0fa8643...` without.)
