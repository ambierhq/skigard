# Customizing a skill

You approved a skill and want it to behave differently: a different command, a house rule, a trimmed section. Do not edit `skills/NAME` in place. Installs are symlinks into it, so an unreviewed edit is live in every agent at once, and the pristine approved version is gone, so there is nothing left to review the edit against. `skills/` is read-only after approval for the same reason (a deterrent, not a security boundary; the lock hash is the control).

Instead: edit a working copy, adopt it, and keep it as a **local fork** that skigard tracks against its upstream. This page walks one example end to end. Terms are in [concepts](concepts.md#keeping-your-local-changes).

The example: Microsoft's `playwright-cli` skill tells the agent to run `npx playwright ...`. Your project runs Playwright through [mise](https://mise.jdx.dev), so you want `mise exec -- playwright ...` instead, in one project only.

## 1. Add the skill

```sh
skigard add github:microsoft/playwright-cli --skill playwright-cli
```

Approve it as usual (automatically if it clears your threshold, otherwise with `skigard approve playwright-cli` in a terminal). The lock records `mode = "auto"` or `"human"`.

## 2. Edit a working copy

```sh
skigard edit playwright-cli
```

This copies the approved skill into `quarantine/playwright-cli` and prints the path. `skills/`, the lock and your installs are untouched, so the approved version stays live. Change the files with your editor (skigard never starts one and never runs anything from a skill):

```sh
perl -pi -e 's/npx playwright/mise exec -- playwright/' ~/.config/skigard/quarantine/playwright-cli/SKILL.md
skigard diff playwright-cli        # your edit against the approved version
```

While the working copy is open, `list`, `status` and `show` call it `editing`; `approve`, `review` and `update` refuse to touch it, and `skigard reject playwright-cli` discards it.

## 3. Adopt it

```sh
skigard adopt playwright-cli
```

`adopt` hashes, scans and scores the edit, then decides exactly like `add` (same mode, threshold and `auto_review`; `--mode`, `--threshold`, `--review` override them, and an agent cannot weaken them):

- `auto` mode, score at or above the threshold, no hard stop, any required review done: approved at once, recorded as `mode = "auto"`.
- `strict` mode, a lower score, a hard stop, or a review that did not run: the edit stays in quarantine with the reasons. Run `skigard approve playwright-cli` in a real terminal; the approval screen shows the diff against the approved version and the lock records `mode = "local"`.

Either way the lock entry gains an `upstream_hash`, the content hash of the upstream you forked from. After `adopt` (or `approve`), run `skigard sync`: symlink installs already show the new version, and `sync` refreshes the install records (until then `verify` warns) and updates copies. `approve` itself never installs anything.

"no changes" means the working copy is identical to the approved version; it stays open.

## 4. Group it and install it into one project

A [plugin](concepts.md#plugins-and-the-marketplace) is a named group of approved skills. Installing a plugin into a project puts the forked skill only where you want it:

```sh
skigard plugin create web-development
skigard plugin add web-development playwright-cli
skigard install --plugin web-development --scope project --project ~/code/my-app
skigard verify
```

Project installs are copies in the project's harness directories (`.claude/skills`, `.agents/skills`), so they do not follow later changes. To refresh them, repeat the `install` with `--force` (the old copy is kept next to it as `NAME.skigard-backup-*`; delete it once you have checked the result). `marketplace/` is not needed for this; it only exists to publish plugins (`skigard marketplace build`, or `skigard config set marketplace true` to keep it in sync).

## 5. When upstream changes

```sh
skigard update playwright-cli
```

`update` compares upstream with the `upstream_hash` you forked from, not with your copy, so your edits are never mistaken for an upstream change. If nothing changed it says `up to date (local fork)`. If upstream changed, the new version goes to quarantine and is **never auto-approved over your edits**, even in `auto` mode; your approved copy stays installed.

```sh
skigard diff playwright-cli        # your copy against the new upstream
```

Bring the new upstream changes into your copy with `merge`:

```sh
skigard merge playwright-cli --with claude-code --note "use mise exec -- playwright"
# or hunk by hunk, yourself (nothing leaves your machine):
skigard merge playwright-cli --pick
```

- `--pick` shows each hunk in your terminal; the default for every hunk is to keep yours. Humans only: refused inside a detected agent.
- `--with HARNESS` has a verified reviewer-template harness (`claude-code`, `pi`, `omp`) do the merge, confined like a reviewer: tools and MCP off, empty working directory, scrubbed environment, an OS sandbox where one works (`--allow-policy-only` as for [review](review.md)). It sees your copy, the new upstream and your `--note`, which **sends them to that harness's model provider**. Its answer is untrusted: a strict parser accepts only a complete, well-formed tree, and anything else stages nothing.

Either way only quarantine changes. The merged result is scanned and scored like any skill, and then a human approves it:

```sh
skigard diff playwright-cli        # now: your current copy against the merged result. Read it.
skigard review playwright-cli      # optional second opinion
skigard approve playwright-cli     # human, real terminal; recorded as mode = "local", upstream_hash = the new upstream
```

Merge results are never auto-approved, in any mode, and `adopt` will not approve one either. Merging compares two versions (yours and the new upstream; no stored base), so read the diff. `skigard reject playwright-cli` drops a merge you do not like, and `skigard update playwright-cli` fetches upstream again.

## What an agent can and cannot do here

| Step | In a detected agent |
|---|---|
| `edit`, `diff`, `merge --with`, `lock protect` | allowed; they only change quarantine or file permissions |
| `adopt` | allowed, and it follows the same gates as `add`: it can approve an edit that clears your threshold (recorded `mode = "auto"`), never weaker than your stored mode and threshold |
| `merge --pick`, `approve` | refused: human terminal only |
| a merge result | never auto-approved; only `approve` installs it |

If that auto path is more than you want, set `mode` to `strict`: then `adopt` always leaves the edit for you. `verify --require-human` accepts `local` approvals (a person approved those bytes) and fails on `auto` and `forced` ones. See the [threat model](threat-model.md).

## If you already edited in place

If you changed `skills/NAME` after a `chmod`, `verify` fails and `skigard adopt NAME` stages that edit instead. `adopt` re-fetches the original upstream at the locked commit (a local source is re-read from its path) and keeps it as the comparison only if it hashes to the approved baseline; otherwise the approval screen says no comparison is available and you read the files yourself.
