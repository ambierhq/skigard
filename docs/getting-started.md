# Getting started

This takes about a minute and ends with a skill pinned in your lockfile and installed into your agents.

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

3. **Fetch a skill into quarantine.** Nothing is installed yet; the files sit in a private directory and are scanned. This sample is one
   small file that passes the scanner (`PASS`, risk 0/100):

   ```sh
   skigard add github:ambierhq/skigard/examples/hello-skill
   ```

4. **Optional: get a second opinion.** An agent you already use reads the skill with its tools off. It is advisory and can only add doubt.
   See [review](review.md). On Windows use `skigard review hello-skill --manual`.

   ```sh
   skigard review hello-skill
   ```

5. **Approve it, as a human, in a real terminal.** You see the scan, the review and the diff against anything approved before, then type
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

6. **Install into every detected agent.**

   ```sh
   skigard install hello-skill
   skigard verify
   ```

   `verify` re-checks the lockfile, the approved store, the installs and the marketplace. It exits 1 on any drift. Run it in [CI](ci.md).

## Where things are

Everything lives under `~/.config/skigard` (`%APPDATA%\skigard` on Windows), or wherever `SKIGARD_HOME` points. Keep it in a private git
repository with `skigard init --git`. On another machine, clone it there and run `skigard sync`. Layout and file formats:
[concepts](concepts.md) and [lockfile](lockfile.md).

## Next

- [Concepts](concepts.md): quarantine, scan, review, approve, lockfile, harnesses.
- [Command reference](commands.md).
- [Using skigard in CI](ci.md).
- [Writing skills that pass the scanner honestly](writing-safe-skills.md).
- [Troubleshooting](troubleshooting.md) and the [FAQ](faq.md).
