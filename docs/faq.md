# FAQ

## Is skigard safe? Does "pass" mean the skill is safe?

No. skigard reduces risk; it does not prove safety. The scanner is heuristic, an agent review can be fooled, and "pass" only means the enabled rules found
nothing. Every report carries `safety_claim: not-proven-safe`. The control that matters is a human reading what they approve. See the [threat model](threat-model.md).

## Does an agent review replace reading the skill?

No. Review is a second opinion for the person who approves. It can only add doubt (it never lowers a scanner finding), and it is optional. Read the skill,
especially bundled scripts.

## Why can't my agent approve skills for me?

Because the threat is an agent that has been prompt-injected into installing more skills. `approve` reads the terminal device (`/dev/tty`, or `CONIN$` on Windows),
has no `--yes`, no stdin path and no environment override, and refuses when it detects it is inside an agent harness. Agents can still `add`, `scan`, `review`
and `verify`.

## Does it phone home?

No. There is no telemetry and no update check. skigard talks to GitHub (`api.github.com`, `codeload.github.com`, `github.com`) only when you `add` or `update` a GitHub
source. If you run `review`, the harness you choose sends the skill to its own model provider; use `--manual` to avoid that.

## Which models see my skill?

Only if you run `skigard review`: the harness you pick (Pi, Oh My Pi, Claude Code and so on) sends it to whatever provider that harness uses. The scan, approve, lock and verify steps need no
model and no network.

## What happens when a skill updates?

`skigard update` re-fetches approved skills. Unchanged content is reported as up to date. A changed skill goes back to quarantine with a diff, and has to be scanned, reviewed and approved again.
The installed copy stays at the approved version until then.

## How is it different from vercel-labs/skills?

[`vercel-labs/skills`](https://github.com/vercel-labs/skills) (`npx skills`) is the widely used installer and discovery CLI for the agent skills ecosystem, with a large agent list, `skills.sh` search and a
`skills-lock.json` with a content hash. Its job is getting skills into your agents conveniently. skigard's job is the gate: quarantine, a scan, an optional review and a **human approval** pinned to a hash, with
`verify` for CI. They solve different problems and can sit side by side; skigard credits it for the harness map and the shared `.agents/skills` directory.

## How is it different from asm (luongnv89/asm)?

[`asm`](https://github.com/luongnv89/asm) is a scriptable skill manager with a big catalog, a TUI, `--json`/`--yes` for automation, inventory and context-cost reporting, and a pre-install security scan.
It is built to be driven by agents and scripts. skigard is deliberately the opposite on one point: installation can never be completed by an agent or a script, because `approve` needs a human at a terminal, and
approvals are pinned to hashes in a lockfile you commit. If you want a catalog and convenience, asm fits; if you want an auditable approval trail, skigard does. We borrowed ideas from asm (`doctor`,
`--json` everywhere, token cost reporting); see [Acknowledgements](../README.md#acknowledgements).

## Does it work on Windows?

Yes, on `windows/amd64` and `windows/arm64`, including `approve` (it reads the console). Differences: installs are copies, files live in `%APPDATA%` and `%LOCALAPPDATA%`, and automated review is disabled (use `--manual`
or WSL). See [Windows notes](windows.md) and [install on Windows](install/windows.md).

## Which agents does it support?

[78 harnesses](harnesses.md) are in the registry, including Claude Code, Pi, Oh My Pi, Codex, Cursor, Gemini CLI and OpenCode. Three are verified (Claude Code, Pi, Oh My Pi); the rest are detected and listed but
installed to only with `--include-unverified` until someone confirms the path. Add your own with `skigard harness add`, or send a PR.

## Does it need an agent installed?

No. Scan, approve, lock and verify work without any agent. Review needs one.

## How do I uninstall?

`skigard uninstall` removes what skigard installed into your agents (it leaves your approvals). Then delete the binary. To drop everything, also delete the skigard home (`~/.config/skigard`, or
`%APPDATA%\skigard` and `%LOCALAPPDATA%\skigard` on Windows).

## How do I report a malicious skill?

Report the repository to GitHub (Report content). If the skill passes a skigard rule you would expect to catch it, that is not a vulnerability in skigard
([SECURITY.md](../SECURITY.md) lists what is in scope): open an issue or a PR with a defanged fixture and, ideally, a new rule (see [CONTRIBUTING.md](../CONTRIBUTING.md)). To report a flaw in skigard
itself (approving without a human, unreviewed bytes reaching an agent, and so on), use GitHub's "Report a vulnerability" privately.

## Why is my security-review skill (or `archify`) blocked?

The scanner matches text, not intent. A skill that talks about `~/.ssh` or "ignore previous instructions" (a checklist, a deny-list regex) looks like one that does it. High and critical findings cannot be
accepted away: read the skill, then approve with `--override --reason "..."`. The override is recorded and `status`/`verify` keep showing it. See [writing safe skills](writing-safe-skills.md).

## What does the lockfile pin, and can I move it between machines?

Source, commit, a SHA-256 content hash identical on every OS, the exec set, scan and review results, and the approval. Commit it (or the whole home as a private git repo) and run `skigard sync` elsewhere. Locks from before
hash v2 keep verifying; `skigard lock upgrade` moves them. See [lockfile](lockfile.md).

## Is it free? Who makes it?

MIT licensed, no accounts. Built by [Ambier Oy](https://ambierhq.com).
