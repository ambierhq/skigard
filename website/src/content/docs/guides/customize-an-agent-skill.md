---
title: "Customize an agent skill and keep your changes"
description: "Fork a skill you approved, edit it safely, install it into one project, and merge upstream updates without losing your changes."
---

Sometimes a skill is almost right. Say you approved Microsoft's `playwright-cli` skill and want it to run Playwright
through your project's `mise` tasks. Editing the installed files is the obvious move and the unsafe one: installs are
symlinks into skigard's store, so an unreviewed edit is live in every agent at once, and the approved copy is gone, so
there is nothing left to compare against. skigard gives you a path that keeps both.

## 1. Edit a working copy

```sh
skigard add github:microsoft/playwright-cli --skill playwright-cli   # approve as usual
skigard edit playwright-cli                # copies it to quarantine and prints the path
$EDITOR <that path>/SKILL.md               # skigard never launches an editor or runs anything from the skill
```

The approved version stays live and untouched (approved skills are read-only, a deterrent rather than a security
boundary; the lock hash is the control). Nothing you type reaches an agent yet.

## 2. Adopt the edit

```sh
skigard adopt playwright-cli               # hash, scan and score the edit
skigard diff playwright-cli                # your edit against the approved version
skigard approve playwright-cli             # you, in a real terminal
```

`adopt` decides like `add`. In `auto` mode an edit that clears your threshold with no hard stop is approved at once and
recorded as `mode = "auto"`; in `strict` mode, below the threshold, or with a hard stop it waits in quarantine for you.
Either way the lock keeps an `upstream_hash`, the baseline of the original, so a local fork is never mistaken for an
upstream change. `approve` records a human decision as `mode = "local"`.

## 3. Put it in a project

Group the skill in a plugin and install it into one repository instead of your whole machine:

```sh
skigard plugin create web-testing
skigard plugin add web-testing playwright-cli
skigard install --plugin web-testing --scope project --project .
```

## 4. Take upstream updates

```sh
skigard update playwright-cli              # upstream changed: the new version waits in quarantine
skigard diff playwright-cli                # your copy against the new upstream
skigard merge playwright-cli --with claude-code --note "use mise run playwright"
skigard diff playwright-cli                # your copy against the merged result: read it
skigard approve playwright-cli
```

`update` never replaces your edits, not even in `auto` mode. `merge` stages a result in quarantine in one of two ways:

- **By hunks:** `merge --pick` asks you, hunk by hunk, in your terminal. The default is to keep yours. Agents cannot use it.
- **With an agent:** `merge --with claude-code` (or `pi`, `omp`) lets a coding agent do it, confined like a reviewer: tools
  off, empty directory, scrubbed environment, OS sandbox. This sends the skill to that agent's model provider. Its output
  is untrusted and parsed strictly; anything invalid stages nothing.

Either way the merged tree is scanned, scored and approved like any other skill. A merge result is never approved
automatically, in any mode, and only a human can approve it. The merge is two-way (no stored base), so the `diff` is the
review that matters.

## Limits

Your edits are only as safe as you read them to be: the scanner is heuristic and a score is not a safety proof. An agent
with write access to the working copy can change it before `adopt`, and in `auto` mode whatever clears your threshold is
approved and shown in the lock. Use `strict` mode, or `verify --require-human`, if that is not acceptable. See the
[threat model](/skigard/docs/threat-model/).

[Concepts: editing a skill](/skigard/docs/concepts/#editing-a-skill) · [Getting started](/skigard/docs/getting-started/) · [Command reference](/skigard/docs/commands/)
