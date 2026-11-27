---
title: "How to review agent skills before installing"
description: "A practical checklist for reading an agent skill before it reaches Claude Code, Pi, Codex or Cursor, and how skigard makes the review stick."
---

An agent skill is a folder with a `SKILL.md` and sometimes scripts. Your agent reads it as instructions and may run what
it bundles, with your files and your shell in reach. Reviewing it before install takes a few minutes. This is the
checklist, and how [skigard](/skigard/) turns it into a habit.

## The checklist

1. **Read `SKILL.md` end to end.** Look for instructions that tell the agent to ignore earlier rules, hide what it does,
   skip confirmations or change its own permissions.
2. **List every file.** Scripts, binaries, minified code and large encoded blobs deserve the most suspicion. A reviewer
   cannot read what is not readable, so treat unreadable files as a reason to stop.
3. **Search for hidden text.** Zero-width and bidirectional characters, NUL bytes and HTML comments can hide instructions
   that render as nothing.
4. **Check what it touches.** Credentials, SSH keys, browser data and `.env` files are not needed by most skills. Fixed,
   documented network hosts are fine; "download and run" is not.
5. **Pin what you read.** A branch can change after you looked. Record the exact commit and a hash of the content, and
   install only those bytes.

## Doing it with skigard

```sh
skigard add github:owner/repo --skill name --mode strict   # fetch, scan, score, then you confirm in a terminal
skigard add github:owner/repo --skill name --quarantine-only   # or: fetch and scan only, nothing installed
skigard scan name                              # deterministic rules, no model involved
skigard review name                            # second opinion, advisory only (add already runs one by default)
skigard approve name                           # you read the evidence and type the skill name
```

By default (`auto` mode) `add` installs a skill that scores 7 or more out of 10 with no hard stop, and keeps the rest in quarantine with the reasons; `--mode strict` always asks you. `add` resolves the reference to a full commit and downloads it as an archive, without running git. `approve` shows the
scan, the review and the diff against anything you approved before, and pins the result to a content hash in
`skigard.lock`. Later, `skigard verify` fails if anything drifts.

## What a pass does not mean

The scanner is heuristic and a model review can be fooled. A pass or a high score means the enabled rules found nothing, not that the
skill is safe. Read the skill yourself, especially bundled scripts. See the [threat model](/skigard/docs/threat-model/).

[Install skigard](/skigard/docs/install/) · [Getting started](/skigard/docs/getting-started/)
