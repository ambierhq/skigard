# Getting started

This takes about a minute and ends with a skill pinned in your lockfile and installed into your agents. One command does the work; the steps after it are for when a skill is held back, or when you want to go step by step.

1. **Install skigard.** See [install](install/README.md). The quickest route:

   ```sh
   curl -fsSL https://ambierhq.com/skigard/install.sh | sh        # macOS, Linux
   brew install ambierhq/tap/skigard                              # or Homebrew
   ```

   Windows, Go and the other channels are in the [status table](install/README.md#channels-and-status).

2. **See which agents skigard found.**

   ```sh
   skigard detect
   ```

   `installed` means the binary or app exists, `configured` means only its config directory exists. Add `--all` to list every one of the
   [78 known harnesses](harnesses.md).

3. **Add a skill.** skigard fetches it into a private quarantine, scans it, gives it a safety score from 1 to 10 with reasons, and then decides by mode.
   This sample is one small file that passes the scanner (`PASS`, risk 0/100, score 10). In the default `auto` mode a skill scoring at or above the threshold (7) with no hard stop is approved and installed with no prompt. Anything else stays in
   quarantine with a warning, the reasons and the commands to continue. Nothing from the skill is executed.

   ```sh
   skigard add github:ambierhq/skigard/examples/hello-skill
   ```

   Want a human to confirm every time? `skigard config set mode strict` (or `add --mode strict`). Want the old fetch-and-scan-only behaviour? `add --quarantine-only`.
   See [concepts](concepts.md#score-mode-and-threshold). The steps below are what to do when a skill stays in quarantine.

4. **Get a second opinion.** `add` already asks an agent you have (the first detected, verified one) to read the skill with its tools off, unless you ran `skigard config set auto_review false`. That sends the skill to that harness's model provider. It is advisory and can only add doubt. Run it by hand, or choose the harness, here.
   See [review](review.md). On Windows use `skigard review hello-skill --manual`.

   ```sh
   skigard review hello-skill
   skigard config set review_harness pi   # pin a reviewer; `auto` goes back to the first detected one
   ```

   While a reviewer runs, `add`, `update` and `review` show a spinner with the elapsed time on stderr (terminals only, not with `--json`). A default review that fails to run keeps the skill in quarantine: it is never auto-installed unreviewed.

5. **Approve it, as a human, in a real terminal.** (Needed for a held-back skill, and always in `strict` mode.) You see the scan, the review and the diff against anything approved before, then type
   the skill's name. There is no `--yes`, no stdin path and no environment variable, and skigard refuses inside the agent harnesses it can detect. A process that creates its own pseudo-terminal can still type the answer, so the gate stops accidents and ordinary prompt injection, not a determined agent. Signed approvals (a key in your OS keychain or a passkey) are planned.

   ```sh
   skigard approve hello-skill
   ```

   A scanner block, incomplete coverage or a failing review needs `--override --reason "..."`; the override is recorded forever. To see a block,
   add this repository's own `skills/review-skill` (it talks about credentials and `curl | sh`, so the scanner stops it):

   ```sh
   skigard add github:ambierhq/skigard/skills/review-skill
   skigard approve review-skill --override --reason "reviewed by hand: it only describes risks"
   ```

   Big skills can have hundreds of findings. Read the verdict and the counts first, then open the files they point at.

   **Noisy but trusted source?** If a source you trust keeps tripping the same `medium` or `low` rule, a human can stop it being reported with `skigard allow add SOURCE RULE... --reason "..."` (see `skigard allow list`, `skigard allow remove`). You confirm by typing `allow` at a terminal; an agent cannot do it, and `high` and `critical` rules can never be allowed. Suppressed findings are still counted in the scan output.

6. **Install into every detected agent.** (Already done by `add` when the skill cleared the threshold; run it for skills you approved by hand.)

   ```sh
   skigard install hello-skill
   skigard verify
   ```

   `verify` re-checks the lockfile, the approved store, the installs and, if you built one, the marketplace. It exits 1 on any drift. Run it in [CI](ci.md).

## Change a skill you approved

Do not edit `skills/NAME` in place. Copy it to a working copy, change it, adopt it, and let `update` and `merge` carry your changes forward:

```sh
skigard edit playwright-cli      # working copy in quarantine; skills/ stays untouched
skigard adopt playwright-cli     # scan and score your edit; approved at once in auto mode if it clears the threshold, else `approve`
skigard update playwright-cli    # later: new upstream waits in quarantine, never approved over your edits
skigard merge playwright-cli --with claude-code --note "use mise exec -- playwright"    # or --pick
```

The full walkthrough, with a plugin installed into one project, is in [Customizing a skill](customizing-skills.md).

## Where things are

Everything lives under `~/.config/skigard` (`%APPDATA%\skigard` on Windows), or wherever `SKIGARD_HOME` points. Keep it in a private git
repository with `skigard init --git`. On another machine, clone it there and run `skigard sync`. Layout and file formats:
[concepts](concepts.md) and [lockfile](lockfile.md).

## Next

- [Concepts](concepts.md): quarantine, scan, review, approve, lockfile, harnesses.
- [Customizing a skill](customizing-skills.md): edit, adopt, update and merge.
- [Command reference](commands.md).
- [Using skigard in CI](ci.md).
- [Writing skills that pass the scanner honestly](writing-safe-skills.md).
- [Troubleshooting](troubleshooting.md) and the [FAQ](faq.md).
