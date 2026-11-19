# Getting started

This takes about a minute and ends with a skill pinned in your lockfile and installed into your agents.

1. **Install skigard.** See [install](install/README.md). The quickest route today:

   ```sh
   go install github.com/ambierhq/skigard/cmd/skigard@latest      # needs Go 1.27+
   ```

   Prebuilt binaries and package managers arrive with v0.1.0 (see the [status table](install/README.md#channels-and-status)).

2. **See which agents skigard found.**

   ```sh
   skigard detect
   ```

   `installed` means the binary or app exists, `configured` means only its config directory exists. Add `--all` to list every one of the
   [78 known harnesses](harnesses.md).

3. **Fetch a skill into quarantine.** Nothing is installed yet; the files sit in a private directory and are scanned.

   ```sh
   skigard add github:tt-a1i/archify --skill archify
   ```

4. **Optional: get a second opinion.** An agent you already use reads the skill with its tools off. It is advisory and can only add doubt.
   See [review](review.md). On Windows use `skigard review archify --manual`.

   ```sh
   skigard review archify
   ```

5. **Approve it, as a human, in a real terminal.** You see the scan, the review and the diff against anything approved before, then type
   the skill's name. No flag, pipe or environment variable can answer for you. An agent running this command is refused.

   ```sh
   skigard approve archify
   ```

   A scanner block, incomplete coverage or a failing review needs `--override --reason "..."`; the override is recorded forever.
   (`archify` is a legitimate skill that the scanner blocks, because it talks about the things the scanner looks for. That makes it a good first
   example of reading a finding and deciding.)

6. **Install into every detected agent.**

   ```sh
   skigard install archify
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
