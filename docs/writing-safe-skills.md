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

## Check before you publish

```sh
skigard scan ./my-skill                           # text report
skigard scan ./my-skill --fail-on medium --fail-on-incomplete
skigard rules                                     # what is being checked, or see rules.md
skigard add ./my-skill && skigard show my-skill   # what a user will see
```

- Tag releases and tell users to pin a commit. skigard resolves every ref to a SHA and re-reviews on change.
- Keep file modes honest: only scripts that must be executable should be. skigard records the execute bits and flags drift.
- Keep line endings as LF so the content hash is the same everywhere ([Windows notes](windows.md)).

See the full [rule list](rules.md) and the [threat model](threat-model.md).
