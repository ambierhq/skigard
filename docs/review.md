# Agent review

`skigard review NAME` asks an agent harness you already have to read a quarantined skill and say what it thinks.
It is a second opinion for the human who approves, and an input to the safety score. It is never the decision on its own: it can only lower the score, never raise it.
It runs on `skigard add` and `update` by default (`auto_review`), using `settings.review_harness` or else the first detected, verified reviewer. Turn that off with `skigard config set auto_review false`. It sends the skill to a model provider; `--manual` does not.

## Choosing the reviewer

```
skigard review --list                 # harnesses that can review headlessly and are installed
skigard review NAME                   # first time: asks which one, can remember it (settings.review_harness)
skigard review NAME --harness pi
skigard review NAME --manual          # prints the prompt; paste it into any agent yourself
skigard review record NAME --verdict pass|concerns|fail --summary "..."   # store a manual result
```

| Harness | Status | How it is run |
|---|---|---|
| `pi` | verified, run end to end | `pi -p --no-tools --no-extensions --no-skills --no-mcp --no-session --no-context-files --no-prompt-templates --system-prompt ...` |
| `omp` | verified, run end to end | `omp -p --no-tools --no-extensions --no-skills --no-rules --no-context-files --no-session --no-lsp --system-prompt ...` |
| `claude-code` | verified, run end to end under Seatbelt on macOS | `claude -p --output-format text --tools "" --allowed-tools "" --permission-mode dontAsk --strict-mcp-config --setting-sources= --disable-slash-commands --no-session-persistence --system-prompt ...` |
| `gemini-cli` | unverified, needs `--allow-unverified` | `gemini -p "" --approval-mode plan` with the system prompt on stdin |
| `codex` | not supported | its read-only sandbox still lets commands read host files, so a tool-less run cannot be guaranteed |

**Windows:** automated review is disabled and fails closed; use `--manual`. See [windows.md](windows.md).

Other harnesses: use `--manual`, or open a PR adding a template to `internal/review/review.go` with evidence for
every flag. A template must disable tools, MCP and extensions, must not read the working directory, and must take the
prompt on stdin.

## What the reviewer sees and cannot do

The reviewer runs in two layers. Both are recorded with the review as `isolation`, and the approval screen shows it.

**1. Policy (always).** Tools, MCP, extensions, skills and sessions are disabled with the harness's own flags; an empty
temporary working directory; a scrubbed environment; the skill on stdin only, never in argv; a timeout and output caps.

**2. OS sandbox (when one works here).**

| `isolation` | Platform | Confinement |
|---|---|---|
| `seatbelt` | macOS | `/usr/bin/sandbox-exec` with a generated profile (`deny default`), written to a private temp file. Reads only from system libraries, the harness install tree and the harness's own config dir (read-only); writes only to the temp working dir; process exec only for the harness and its runtime (shell, interpreter); outbound TCP only to port 443 plus name resolution |
| `bwrap` | Linux | bubblewrap: new user/pid/ipc/uts/cgroup namespaces, read-only system dirs, tmpfs `$HOME` with only the harness's config bound read-only, writable temp dir. **Gap:** it shares the host network, so ports are not restricted, and system tools under `/usr` stay executable |
| `landlock` | Linux >= 5.13 | `skigard __sandbox-exec` re-executes under Landlock (raw syscalls, `PR_SET_NO_NEW_PRIVS`): filesystem allowlist, exec only for the harness and runtime, and with ABI >= 4 (kernel 6.7) TCP connect only to 443. On older kernels the network is not restricted; the note says so |
| `policy-only` | any | no OS sandbox worked. A **verified** template only runs if you pass `--allow-policy-only`; unverified templates never run this way |

Windows has no OS sandbox and review stays fail-closed (use `--manual`, or run skigard inside WSL).

`--allow-policy-only` only decides whether the advisory review may run without an OS sandbox. It never turns an
available sandbox off, and approval does not read it.

Per-harness needs (what the allowlist contains beyond the system):

- `pi`: its install tree, `~/.pi` read-only, and creating/removing `*.lock` entries in `~/.pi/agent`. Run end to end under Seatbelt.
- `omp`: its install tree, `~/.omp` and `~/.config/omp` read-only, plus **write** access to `~/.omp/logs`, `natives`, `run`, `cache` and its `agent/*.db*` sqlite files (it writes them even for a one-shot run). Run end to end under Seatbelt. Reads of other tools' config (for example `~/.claude`) are denied.
- `claude-code`: `~/.claude`, `~/.claude.json`, `~/.config/claude` and `~/.config/anthropic` (its login and profile) read-only, plus creating/removing `*.lock` entries in `~/.config/anthropic`. Its scratch directory is redirected into the work dir with `CLAUDE_CODE_TMPDIR` (it ignores `TMPDIR` and otherwise writes `/tmp/claude-<uid>`). Run end to end under Seatbelt on macOS. The login is not refreshed inside the sandbox: if your token has expired, run `claude` once outside it.
- `gemini-cli`: unverified.

Known gaps: macOS mach services are not restricted; the harness can read its own credentials (it must authenticate)
and talk to its provider on 443; HTTP proxies on other ports do not work under the sandbox. The sandbox is defence in depth: the
reviewer is advisory and can only raise concern.

- It receives the skill's files as text on **stdin** inside random delimiters, plus the scanner's findings.
  Skill content never appears in the argument list.
- It runs with **tools disabled by flag**, in an **empty temporary working directory**, with a **scrubbed environment**:
  only `PATH`, `HOME`, locale and proxy variables, plus the credential prefixes that harness needs
  (for example `ANTHROPIC_*` for Claude). `GITHUB_TOKEN`, `AWS_*` (unless the harness needs it), SSH agent sockets and
  everything else are removed.
- Files larger than the prompt budget (200 KB total) or not valid UTF-8 are listed as *not shown*. The result then has
  `coverage: partial` and a "pass" is raised to "concerns".
- 10 minute timeout, 1 MiB output cap.

## What happens to the answer

The reviewer must return one JSON object. skigard:

1. Verifies every quoted line exists in the named file it was shown. Findings with a missing file or an invented quote
   are dropped and counted (`unverified_findings`).
2. **Never lets the review lower anything.** A "pass" with a verified high/critical finding becomes "concerns". A
   claim that the description does not match the behaviour becomes "concerns". Partial coverage becomes "concerns".
3. Stores the record next to the quarantined skill, tagged with the content hash it saw. If the skill changes, the
   review is ignored.
4. Feeds the safety score: `concerns` takes 2 points off, `fail` caps the score at 3 and is a **hard stop** (never auto-installed). Shows it in the approval screen. A reviewer verdict of `fail` requires `approve --override --reason`, the same as
   a scanner `block`. Set `require_review = true` in `skigard.toml` to refuse approvals with no review at all.

## Limits

- A model can be fooled. That is why the scanner and the human come first and the review can only add doubt.
- The provider may refuse to read malicious content (this happened in testing: "Cyber safeguards flagged this message").
  skigard reports the failure; the scan result still stands.
- Each review is one call to your own plan or API account.
