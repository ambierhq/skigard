---
title: Concepts
description: Quarantine, scan, review, approve, lockfile and harnesses, the six parts of the skigard gate.
---

```
add ──► quarantine ──► scan ──► review (agent, advisory) ──► approve (human) ──► skigard.lock ──► install
```

**Quarantine.** `add` fetches a skill (a folder with a `SKILL.md`) from GitHub or disk into a mode 0700 directory.
Nothing from a skill is executed and git is never invoked. Archives are extracted through `os.Root`.

**Scan.** A deterministic scanner with a [rule set](/skigard/docs/rules/) plus capability chains. `high` or
`critical` findings block; `medium` or partial coverage needs review. The scanner matches text, not intent, so a skill
that merely talks about `~/.ssh` can be flagged.

**Review.** Optional. An agent harness you already use reads the files with tools disabled, in an empty directory with
a scrubbed environment and, on macOS and Linux, an OS sandbox. It is advisory and can only raise concern. See
[Agent review](/skigard/docs/review/).

**Approve.** A human types the skill name in a real terminal. A block, partial coverage or a failing review needs
`--override --reason "..."`, which is recorded and shown by `status` and `verify` forever.

**Lockfile.** `skigard.lock` records source, commit, content hash, scan result, review and approver. Installs refuse
anything whose hash no longer matches. See [Lockfile format](/skigard/docs/lockfile/).

**Harnesses.** The registry knows where each agent harness reads skills from. `detect` finds the ones on your machine;
`install` links approved skills into each. See [Supported harnesses](/skigard/docs/harnesses/).
