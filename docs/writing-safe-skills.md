# Writing skills that pass the scanner honestly

The scanner matches text, not intent. It cannot tell a skill that *describes* `~/.ssh` from one that reads it. These habits keep your
skill easy to approve and make a reviewer's job short. Nothing here is about evading the scanner: a skill that needs a risky capability should
say so and let a human decide.

## Keep it small and readable

- One job per skill. A reviewer has to read all of it.
- Put instructions in `SKILL.md`; keep scripts few, short and in `scripts/`. Prefer instructions that call tools the agent already has.
- No binaries, minified code, large blobs or encoded payloads. They make scan coverage *partial*, which needs an explicit override.
  Ship source and say how to build it.
- No zero-width, bidirectional or tag characters, NUL bytes or HTML comments with instructions in them. Hidden text is exactly what a
  malicious skill would use, so the scanner treats it as a finding.

## Be explicit about what you touch

- State the files, hosts and commands the skill needs in the description and again at the top of any script.
- Do not read credentials, SSH keys, browser data or `.env` files. If the task really needs a credential, ask the user to supply it for that run.
- No `curl | sh`, no downloading and executing code at run time, no `eval` of fetched text. Pin and vendor what you need.
- No instructions that tell the agent to ignore earlier rules, hide what it is doing, skip confirmation, or change its own permissions.
- Write network calls with fixed, documented hosts. Never send file contents anywhere unless that is the declared purpose.

## When your skill is *about* these things

Security checklists and review skills will quote the very patterns the scanner looks for. Keep such text inside clearly fenced examples,
and expect a **block** on first scan. The reader then decides with `skigard approve NAME --override --reason "..."`; the reason is recorded and shown
by `status` and `verify`. [`skills/review-skill`](../skills/review-skill) in this repository is an example of such a skill.

Findings of `low` and `medium` can be accepted with `--accept REASON` (optionally `--accept-expires YYYY-MM-DD`). `high` and `critical` never can.

## What moves a safety score

`skigard add` turns the scan into a score from 1 to 10 (10 = no concern); with the default `auto` mode a skill installs without a prompt only at 7 or above. The score starts at `10 - ceil(risk / 12)` from the scanner's risk number, so every finding costs points, and then:

- any `high` or `critical` finding (verdict `block`) caps it at 3 and is a hard stop: it is never auto-installed;
- incomplete coverage (binaries, oversized or non-UTF-8 files) caps it at 6, so it never clears a threshold of 7 or more;
- a reviewer `fail` caps it at 3 (hard stop), and `concerns` takes 2 points off. A review can only lower the score.

So a small, readable skill with no scary text, no binaries and no hidden characters scores 10. A skill that quotes risky patterns will score low, and users will see it held in quarantine with the reasons; that is the system working. The score is a heuristic: do not tune a skill to hit a number.

## Check before you publish

```sh
skigard scan ./my-skill                           # text report
skigard scan ./my-skill --fail-on medium --fail-on-incomplete
skigard rules                                     # what is being checked, or see rules.md
skigard add ./my-skill --quarantine-only && skigard show my-skill   # what a user will see (no install)
```

- Tag releases and tell users to pin a commit. skigard resolves every ref to a SHA and re-reviews on change.
- Keep file modes honest: only scripts that must be executable should be. skigard records the execute bits and flags drift.
- Keep line endings as LF so the content hash is the same everywhere ([Windows notes](windows.md)).

See the full [rule list](rules.md) and the [threat model](threat-model.md).
