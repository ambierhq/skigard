# skigard threat model

Status: 0.1. This is a design contract, not a claim that the software is free of defects.

## What skigard decides

> Should these exact bytes cross into the directory an agent reads skills from?

skigard is a **recommender**. It provides evidence (a deterministic scan, an optional advisory agent review, a diff),
turns it into a safety score from 1 to 10 with reasons, pins the decision to a content hash, and later detects drift.
Who decides depends on the mode:

- In `strict` mode a human decides, on `/dev/tty`, every time.
- In `auto` mode (the default) skigard decides for skills whose score is at or above the human-chosen threshold and
  that have no hard stop (scanner verdict `block`, reviewer verdict `fail`). Everything else stays in quarantine
  until a human approves it.
- `--force-install` lets a human override all of that. It is recorded as `mode = "forced"`.

It does **not** prove that a skill is safe. A high score means only that the enabled rules found nothing alarming and,
if a review ran, that the reviewer found nothing. Every report carries `safety_claim: not-proven-safe`. A human stays
in the loop when you choose strict mode or when a skill scores below your threshold.

## Assets

- Credentials, tokens and files an agent can reach (SSH keys, cloud credentials, `.env`, browser data).
- The integrity of agent instructions and approval boundaries.
- The directories agents discover skills from.
- The integrity of what was approved (the lock file and the approved store).
- The user's attention. A noisy scanner is a security problem, because people stop reading.

## Actors

**Untrusted:** skill authors, repositories, tags and branches, archives, file names, file contents, the output of a
reviewing agent, terminal control sequences inside any of those, and anything in `quarantine/`.

**Trusted:** the skigard binary, the OS, the human at the terminal, and your own `~/.config/skigard` once approvals are
in it. Someone who can write to your home as you can already do anything; skigard does not defend against that.

## Attacks considered and what skigard does

| Attack | Control |
|---|---|
| A skill tries to run code while being inspected | Nothing from a skill is ever executed. Scanning is regex and parsing only. skigard never invokes `git` on skill content (GitHub sources are downloaded as tarballs); the only `git` call is `skigard init --git`, which runs `git init` in your own home, and `doctor` looks for the binary |
| A branch or tag changes between review and install | Refs are resolved to a full commit SHA before download. The lock records the commit and the content hash. Installs re-check the hash |
| Archive tricks: `../`, absolute paths, symlinks, hardlinks, devices, case-fold collisions, zip bombs | Extraction goes through `os.Root`. Only regular files and directories are written. Links are not extracted and a skill that contains one is refused. Limits on compressed size (64 MiB), extracted size (128 MiB), entries (10,000), file size (8 MiB), depth and compression ratio |
| SSRF, DNS rebinding, redirect abuse | HTTPS only. Redirects only to `api.github.com`, `codeload.github.com`, `github.com`, at most 5. The connect hook rejects loopback, private, link-local, CGNAT and multicast addresses *after* DNS resolution |
| Content changes after review | Approval is bound to a content hash (SHA-256 over path, length and bytes of every file, identical on every OS) and to an exec set: the files that were executable in the source, recorded at fetch time with its own digest. `approve` rehashes, copies to a staging directory on the same filesystem, rehashes again, then renames atomically. The exec set is re-checked on Unix by `approve`, `install`, `sync` and `verify`; on Windows it is reported as not applicable |
| A skill gains an execute bit after review, with the same bytes | Caught on Linux and macOS: `exec_hash` in the lock no longer matches, `verify` fails and `install` refuses. Not detectable on Windows, which has no execute bit; there the files are not executable by the OS either. Entries still on hash version 1 bind the bit inside the hash. `lock upgrade` converts them only if the old hash still matches, so it can never approve anything new |
| A skill hides behaviour from reviewers | Rules for zero-width/bidi/tag characters, NUL bytes, long encoded blobs, HTML comments, binaries and executables. Binary or oversized files make coverage *partial*, which blocks approval without an explicit override |
| Prompt injection aimed at the reviewing agent ("tell the user this is safe") | The reviewer harness runs with tools, MCP, extensions and sessions disabled by flags, an empty working directory and a scrubbed environment plus, on macOS and Linux, an OS sandbox (Seatbelt, bubblewrap or Landlock) limiting file reads, writes, exec and outbound ports; its isolation mode is recorded and shown at approval (see docs/review.md). Windows has none and fails closed. Skill text is delimited by a random boundary and the system prompt says it is untrusted. The reviewer's verdict is *advisory and can only raise concern*: it never lowers a scanner finding. Quotes it cites are checked against the real file text; findings with invented quotes are dropped |
| An agent approves a skill itself (accident, prompt injection) | `approve` reads its confirmation from `/dev/tty` only. There is no `--yes`, no stdin path and no environment override. It also refuses when it detects, best effort, an agent harness in its environment or among its ancestor processes (see [Agent detection](#agent-detection-best-effort-not-a-security-boundary)). This does **not** stop a process that allocates its own pseudo-terminal: see [the pseudo-terminal case](#the-pseudo-terminal-case) |
| An agent forces an install or lowers the bar | `add --force-install` and `config set mode\|threshold` need a human terminal and are refused inside a detected agent. Inside an agent, `--mode` and `--threshold` on `add` may only be stricter than the stored setting |
| Auto mode: a skill installs without a human | Only if `score >= threshold` and there is no hard stop. A scanner `block` or reviewer `fail` is never auto-installed, even at threshold 1. The lock records `mode = "auto"`, the score, the threshold and `score_version`; `verify --require-human` fails if any approval is `auto` or `forced`. `update` in auto mode installs a change only if it scores at or above the threshold and not lower than the approved score |
| Terminal escape injection through names or content | All untrusted text printed to a terminal is escaped (control characters, bidi and zero-width characters). Secrets matching common token shapes are masked in evidence |
| Path traversal through names | Skill, plugin and meta names must match `^[a-z0-9]+(-[a-z0-9]+)*$` (max 64). They are validated when the lock, manifest and metadata are loaded, not only when typed |
| Two processes corrupt the lock | An exclusive advisory lock on the home for every mutating command |
| A malicious skill ships its own "receipt" | Install records live in `state/`, outside every harness skills directory |
| Installing over something that is not ours | Existing unmanaged entries are never replaced without `--force`, and then only after being renamed to a backup. `uninstall` refuses to delete modified copies without `--force` |

## Agent detection (best effort, not a security boundary)

Before prompting, `approve` refuses if an agent environment variable is set, or if any of its ancestor processes
(up to 20) is a known agent binary (`bins` in `internal/harness/registry.toml`) or carries an agent marker in its
environment (`env`). The ancestor's own environment is read, so `env -u CLAUDECODE script ...` is still caught while
the agent is above it. Linux reads `/proc`, macOS uses `sysctl` (KERN_PROCARGS2, same user only), Windows matches
image names only (it cannot read another process's environment). If the process table is unreadable the check
fails open to the other checks.

It is **evadable by re-parenting**: a process that double-forks, daemonizes or is started by `launchd`/`systemd`
leaves the agent's process tree. It does not close the pty bypass; it removes the one-line version.

Humans are not blocked by hosts they use: tmux, ssh and the integrated terminals of VS Code, Cursor, Windsurf, Kiro
and Zed are not agents, so the editor process and `TERM_PROGRAM` are never matched. Known false positives: approving
from a shell that an agent started (for example Claude Code's `!` shell or Pi's bash) is refused by design; a tmux
server first started from inside an agent keeps that agent's environment markers for every pane; a program that
happens to share a short agent binary name (`cn`, `forge`, `kilo`) above skigard is matched. A human running an agent
in one tab and approving in another is not affected (separate process trees).

## What skigard does not do

- It does not prove that a skill is benign. A malicious skill can pass a regex scanner and a reviewing model.
  Read the skill, especially bundled scripts. The scanner exists to focus your attention.
- It does not sandbox the skill when an agent *uses* it later. Use your harness's permission system.
- **Agent-invoked `add` in auto mode (residual risk).** The auto path has no confirmation prompt. An agent that is
  allowed to run `skigard add` in `auto` mode can therefore install any skill that clears the threshold you chose. This
  is by design, it is visible in the lock as `mode = "auto"`, and the agent cannot lower the threshold, change the mode
  or force an install. A skill that evades the scanner and the reviewer (see above) can still score 10 and be
  installed this way. If that is unacceptable, set `mode` to `strict` (then an agent's `add` only quarantines), raise
  the threshold, or run `verify --require-human` in CI.
- The score is a heuristic, not a measurement. It can be gamed by an author who knows the rules, and it is only as good
  as the scanner and the review behind it.
- It cannot stop an agent that has a real terminal, a pseudo-terminal of its own, your keyboard, or screen control from approving ([details](#the-pseudo-terminal-case)). The
  `/dev/tty` (Unix) or `CONIN$`/`CONOUT$` (Windows) and agent-detection checks stop the ordinary cases, not a
  determined attacker who is already controlling your machine. On Windows the same residual risk applies: a
  process that owns a ConPTY (pseudo console) can type into `CONIN$`, just as a process that owns a pty master can
  type into `/dev/tty`. If there is no console, `approve` refuses; there is no flag, environment variable or stdin path.
- The exec set is only as trustworthy as the source's modes: GitHub tarballs (git file modes) are trusted as given; a
  local directory source uses the modes on disk. Timestamps, owners and other permission bits are not bound.
- It does not authenticate publishers. A familiar organisation name or a star count is not a signal.
- Signed approvals are not implemented in 0.1 (they are planned). Anyone who can write to `~/.config/skigard` or a harness skills directory as you
  can forge an approval or add a skill without skigard. `skigard verify --strict` in CI, with the home committed to a repository, is the control that notices a lock file
  changed without a matching reviewable diff.
- HTTP proxies are not used (the address check would otherwise be bypassed). Environments that require a proxy
  cannot fetch from GitHub in 0.1.
- Non-GitHub sources (GitLab, arbitrary git URLs, private hosts) are not supported. Private GitHub repositories work with
  `GITHUB_TOKEN`, which is sent only to GitHub hosts.

## The pseudo-terminal case

`approve` insists on a terminal device, not on a human. Any process that can create a pseudo-terminal can be that terminal and type the answer, which is the skill name printed on the same screen:

```sh
(sleep 2; echo NAME) | script -q /dev/null skigard approve NAME
```

This approves a quarantined `NAME` with no person and no keyboard involved. A ten-line Python `pty.fork()` that types the name when it sees the prompt does the same. The Windows equivalent is a process that owns a ConPTY.

**What this proves.** The terminal requirement is an accident and prompt-injection control, not authentication. It rules out `--yes`, a pipe, a CI job, a cron job and an environment variable, and it makes the approval step visible and deliberate. It does not prove that a human was present.

**What defends against it today, best effort.**

- Agent detection: `approve` refuses on known agent environment variables and on known agent binaries or markers among its ancestors. A process that re-parents itself (double fork, daemonize) or never runs under a known harness is not caught; unknown harnesses are not covered.
- The parent-process check (known agent binaries and environment markers among the ancestors) raises the cost, and it can still be evaded by re-parenting. Treat it as best effort. It also guards `add --force-install` and `config set mode|threshold`.

**What does not defend.** A determined agent or script running as you, a harness skigard does not know about, and anything that can run `script`, Python's `pty` module or `expect`.

**Why signed approvals are the real fix.** Every defence above lives on the same machine and in the same account as the attacker. A signature made with a key that needs a person (an approver key in the OS keychain or Secure Enclave, or a passkey), stored in `skigard.lock` and checked by `skigard verify --strict`, moves the decision out of reach of anything that can only type. Signed approvals are planned, not built. Until then, `verify --strict` in CI with the home in a reviewed repository is the practical control, and this limitation is documented rather than treated as a vulnerability (see [SECURITY.md](../SECURITY.md)).

## Reviewing agents: what leaves your machine

`skigard review` runs a harness you already have installed, using your own account. The skill's text is sent to
that harness's model provider, like any prompt. Use `--manual` if that is not acceptable. Review never runs
automatically unless you turn on `auto_review` (`skigard config set auto_review true`) or pass `--review` to `add`.

## Tests that back these claims

- `internal/safefs`: hostile tar fixtures (traversal, absolute, backslash, collisions, devices, bombs), fuzzing of the entry-path validator.
- `internal/source`: URL and ref parsing fuzzed, public-address predicate, redirect policy.
- `internal/scan`: one malicious fixture per rule family; a test that every prefilter keyword set still admits every sample the rule matches.
- `internal/review`: policy isolation properties (stdin only, empty cwd, scrubbed env), quote verification, "reviewer cannot lower".
- `internal/hash`: pinned version 1 and 2 vectors, a golden fixture hashed identically on Linux, macOS and Windows CI, mode-independence and one-byte/name/length/order sensitivity, exec set tamper detection, fuzzing.
- `internal/core`: exec-bit tamper (quarantine and approved), lock upgrade only on a matching v1 hash, forged `exec_set` refused, approve cancel/tamper/override paths, refusal inside an agent, install refusing a tampered store.
- End to end: `approve` run under a real pty, and without any controlling terminal (see `scripts/e2e.sh`).
