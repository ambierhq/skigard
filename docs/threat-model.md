# skigard threat model

Status: 0.1. This is a design contract, not a claim that the software is free of defects.

## What skigard decides

> Should these exact bytes cross into the directory an agent reads skills from?

skigard provides evidence (a deterministic scan, an optional advisory agent review, a diff), requires a human to
decide, pins the decision to a content hash, and later detects drift. It does **not** prove that a skill is safe.
`pass` means only that the enabled rules found nothing and, if a review ran, that the reviewer found nothing. Every
report carries `safety_claim: not-proven-safe`.

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
| A skill tries to run code while being inspected | Nothing from a skill is ever executed. Scanning is regex and parsing only. Git is never invoked; GitHub sources are downloaded as tarballs |
| A branch or tag changes between review and install | Refs are resolved to a full commit SHA before download. The lock records the commit and the content hash. Installs re-check the hash |
| Archive tricks: `../`, absolute paths, symlinks, hardlinks, devices, case-fold collisions, zip bombs | Extraction goes through `os.Root`. Only regular files and directories are written. Links are not extracted and a skill that contains one is refused. Limits on compressed size (64 MiB), extracted size (128 MiB), entries (10,000), file size (8 MiB), depth and compression ratio |
| SSRF, DNS rebinding, redirect abuse | HTTPS only. Redirects only to `api.github.com`, `codeload.github.com`, `github.com`, at most 5. The connect hook rejects loopback, private, link-local, CGNAT and multicast addresses *after* DNS resolution |
| Content changes after review | Approval is bound to a content hash. `approve` rehashes, copies to a staging directory on the same filesystem, rehashes again, then renames atomically |
| A skill hides behaviour from reviewers | Rules for zero-width/bidi/tag characters, NUL bytes, long encoded blobs, HTML comments, binaries and executables. Binary or oversized files make coverage *partial*, which blocks approval without an explicit override |
| Prompt injection aimed at the reviewing agent ("tell the user this is safe") | The reviewer has no tools, an empty working directory and a scrubbed environment. Skill text is delimited by a random boundary and the system prompt says it is untrusted. The reviewer's verdict is *advisory and can only raise concern*: it never lowers a scanner finding. Quotes it cites are checked against the real file text; findings with invented quotes are dropped |
| An agent approves a skill itself | `approve` reads its confirmation from `/dev/tty` only. There is no `--yes`, no stdin path and no environment override. It also refuses when it detects it is running inside an agent harness |
| Terminal escape injection through names or content | All untrusted text printed to a terminal is escaped (control characters, bidi and zero-width characters). Secrets matching common token shapes are masked in evidence |
| Path traversal through names | Skill, plugin and meta names must match `^[a-z0-9]+(-[a-z0-9]+)*$` (max 64). They are validated when the lock, manifest and metadata are loaded, not only when typed |
| Two processes corrupt the lock | An exclusive advisory lock on the home for every mutating command |
| A malicious skill ships its own "receipt" | Install records live in `state/`, outside every harness skills directory |
| Installing over something that is not ours | Existing unmanaged entries are never replaced without `--force`, and then only after being renamed to a backup. `uninstall` refuses to delete modified copies without `--force` |

## What skigard does not do

- It does not prove that a skill is benign. A malicious skill can pass a regex scanner and a reviewing model.
  Read the skill, especially bundled scripts. The scanner exists to focus your attention.
- It does not sandbox the skill when an agent *uses* it later. Use your harness's permission system.
- It cannot stop an agent that has a real terminal, your keyboard, or screen control from approving. The
  `/dev/tty` (Unix) or `CONIN$`/`CONOUT$` (Windows) and agent-detection checks stop the ordinary cases, not a
  determined attacker who is already controlling your machine. On Windows the same residual risk applies: a
  process that owns a ConPTY (pseudo console) can type into `CONIN$`, just as a process that owns a pty master can
  type into `/dev/tty`. If there is no console, `approve` refuses; there is no flag, environment variable or stdin path.
- It does not authenticate publishers. A familiar organisation name or a star count is not a signal.
- Signed approvals are not implemented in 0.1. `skigard verify` in CI is the control that notices a lock file
  changed without a matching reviewable diff.
- HTTP proxies are not used (the address check would otherwise be bypassed). Environments that require a proxy
  cannot fetch from GitHub in 0.1.
- Non-GitHub sources (GitLab, arbitrary git URLs, private hosts) are not supported. Private GitHub repositories work with
  `GITHUB_TOKEN`, which is sent only to GitHub hosts.

## Reviewing agents: what leaves your machine

`skigard review` runs a harness you already have installed, using your own account. The skill's text is sent to
that harness's model provider, like any prompt. Use `--manual` if that is not acceptable. Review never runs
automatically.

## Tests that back these claims

- `internal/safefs`: hostile tar fixtures (traversal, absolute, backslash, collisions, devices, bombs), fuzzing of the entry-path validator.
- `internal/source`: URL and ref parsing fuzzed, public-address predicate, redirect policy.
- `internal/scan`: one malicious fixture per rule family; a test that every prefilter keyword set still admits every sample the rule matches.
- `internal/review`: sandbox properties (stdin only, empty cwd, scrubbed env), quote verification, "reviewer cannot lower".
- `internal/core`: approve cancel/tamper/override paths, refusal inside an agent, install refusing a tampered store.
- End to end: `approve` run under a real pty, and without any controlling terminal (see `scripts/e2e.sh`).
