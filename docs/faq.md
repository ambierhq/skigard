# FAQ

## Is skigard safe? Does "pass" or a high score mean the skill is safe?

No. skigard is a recommender: it reduces risk and tells you what it found, it does not prove safety. The scanner is heuristic, an agent review can be fooled, and "pass" or a score of 10 only means the enabled rules found
nothing. Every report carries `safety_claim: not-proven-safe`. The control that matters is a human reading what they install; `--mode strict` makes that the rule. See the [threat model](threat-model.md).

## Does an agent review replace reading the skill?

No. Review is a second opinion for the person who approves. It can only add doubt (it never lowers a scanner finding), and it is optional. Read the skill,
especially bundled scripts.

## Why can't my agent approve skills for me?

Because the threat is an agent that has been prompt-injected into installing more skills. `approve` reads the terminal device (`/dev/tty`, or `CONIN$` on Windows),
has no `--yes`, no stdin path and no environment override, and refuses when it detects it is inside an agent harness (environment or parent processes, best effort). `add --force-install` and `config set mode|threshold` are refused in a detected agent too,
and inside one `--mode` and `--threshold` can only be stricter than your stored setting. A process that creates its own pseudo-terminal can still type the answer, so the gate stops accidents and ordinary prompt injection, not a determined agent. Signed approvals (a key in your OS keychain or a passkey) are planned. See [the pseudo-terminal case](threat-model.md#the-pseudo-terminal-case).

What a detected agent *can* do is run `skigard add` in `auto` mode, which installs skills that score at or above your threshold and have no hard stop. That is by design and is recorded as `mode = "auto"` in the lock.
To stop it, set the mode to `strict` (an agent's `add` then stays in quarantine); to detect it, run `skigard verify --require-human`. See the [threat model](threat-model.md).

## Does it phone home?

No. There is no telemetry and no update check. skigard talks to GitHub (`api.github.com`, `codeload.github.com`, `github.com`) only when you `add` or `update` a GitHub
source. If you run `review`, the harness you choose sends the skill to its own model provider; use `--manual` to avoid that.

## Which models see my skill?

Only if you run `skigard review`, pass `--review` to `add`, or turn on `auto_review`: the harness you pick (Pi, Oh My Pi, Claude Code and so on) sends it to whatever provider that harness uses. The scan, approve, lock and verify steps need no
model and no network.

## What happens when a skill updates?

`skigard update` re-fetches approved skills. Unchanged content is reported as up to date. In `strict` mode a changed skill goes back to quarantine with a diff and has to be approved again. In `auto` mode it is installed if it scores at or above the threshold, has no hard stop and does not score lower than the approved version; otherwise it goes back to quarantine with a warning and the diff.
The installed copy stays at the approved version until then.

## How is it different from vercel-labs/skills?

[`vercel-labs/skills`](https://github.com/vercel-labs/skills) (`npx skills`) is the widely used installer and discovery CLI for agent skills: `add`, `find`, `use`, `check`, `update`, `remove`, `init`, `list`,
a `skills-lock.json` with a content hash, and the [skills.sh](https://skills.sh) directory.

skigard is now close to a drop-in for `add`: `skigard add SOURCE` fetches, scans, scores and installs in one step when the skill clears your threshold. It also covers `update`, `uninstall` and `list`, and adds quarantine, a scan, a safety score, an optional review,
optional **human approval** (`--mode strict`) pinned to a hash, a hash-pinned lock, and `verify` to detect drift in CI.

What you give up today: discovery and search (`skills find`, the skills.sh directory), the `skills init` template, and one-off `skills use`. Many people will find skills on skills.sh and add them through skigard.

The lock file is different (`skigard.lock`); an importer for `skills-lock.json` is planned but not built (`skigard migrate` only reads `skm` repositories).
skigard fetches from GitHub (public repositories, or private ones with `GITHUB_TOKEN`) and from local directories; other hosts such as GitLab are not supported yet.
skigard credits vercel-labs/skills for the harness map and the shared `.agents/skills` directory.

## How is it different from asm (luongnv89/asm)?

[`asm`](https://github.com/luongnv89/asm) is a scriptable skill manager with a big catalog, a TUI, `--json`/`--yes` for automation, inventory and context-cost reporting, and a pre-install security scan.
It is built to be driven by agents and scripts. skigard differs on one point: `approve`, forcing an install and lowering your threshold need a human at a terminal (a process that builds its own pseudo-terminal can still type the answer: see the [threat model](threat-model.md#the-pseudo-terminal-case)), and
every approval is pinned to a hash and labelled `human`, `auto` or `forced` in a lockfile you commit. If you want a catalog and convenience, asm fits; if you want a score and an auditable approval trail, skigard does. We borrowed ideas from asm (`doctor`,
`--json` everywhere, token cost reporting); see [Acknowledgements](../README.md#acknowledgements).

## Does it work on Windows?

Yes, on `windows/amd64` and `windows/arm64`, including `approve` (it reads the console). Differences: installs are copies, files live in `%APPDATA%` and `%LOCALAPPDATA%`, and automated review is disabled (use `--manual`
or WSL). See [Windows notes](windows.md) and [install on Windows](install/windows.md).

## Which agents does it support?

[78 harnesses](harnesses.md) are in the registry, including Claude Code, Pi, Oh My Pi, Codex, Cursor, Gemini CLI and OpenCode. Three are verified (Claude Code, Pi, Oh My Pi); the rest are detected and listed but
installed to only with `--include-unverified` until someone confirms the path. Add your own with `skigard harness add`, or send a PR.

## Does it need an agent installed?

No. Scan, score, approve, lock and verify work without any agent. Review needs one.

## How do I uninstall?

`skigard uninstall` removes what skigard installed into your agents (it leaves your approvals). Then delete the binary. To drop everything, also delete the skigard home (`~/.config/skigard`, or
`%APPDATA%\skigard` and `%LOCALAPPDATA%\skigard` on Windows).

## How do I report a malicious skill?

Report the repository to GitHub (Report content). If the skill passes a skigard rule you would expect to catch it, that is not a vulnerability in skigard
([SECURITY.md](../SECURITY.md) lists what is in scope): open an issue or a PR with a defanged fixture and, ideally, a new rule (see [CONTRIBUTING.md](../CONTRIBUTING.md)). To report a flaw in skigard
itself (an agent approving or forcing an install, bypassing the threshold, unreviewed bytes reaching an agent, and so on), use GitHub's "Report a vulnerability" privately.

## Why is my security-review skill (or `archify`) blocked?

The scanner matches text, not intent. A skill that talks about `~/.ssh` or "ignore previous instructions" (a checklist, a deny-list regex) looks like one that does it. High and critical findings cannot be
accepted away: `add` leaves it in quarantine. Read the skill, then approve with `--override --reason "..."`. The override is recorded and `status`/`verify` keep showing it. See [writing safe skills](writing-safe-skills.md).

## What does the lockfile pin, and can I move it between machines?

Source, commit, a SHA-256 content hash identical on every OS, the exec set, scan and review results, the score, and the approval (mode `human`, `auto` or `forced`). Commit it (or the whole home as a private git repo) and run `skigard sync` elsewhere. Locks from before
hash v2 keep verifying; `skigard lock upgrade` moves them. See [lockfile](lockfile.md).

## Is it free? Who makes it?

MIT licensed, no accounts. Built by [Ambier Oy](https://ambierhq.com).
