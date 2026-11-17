# Agent review

`skigard review NAME` asks an agent harness you already have to read a quarantined skill and say what it thinks.
It is a second opinion for the human who approves. It is never the decision.

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
| `claude-code` | flags confirmed against `claude --help`; not run end to end (the development machine was not logged in) | `claude -p --output-format text --tools "" --allowed-tools "" --permission-mode dontAsk --strict-mcp-config --setting-sources= --disable-slash-commands --no-session-persistence --system-prompt ...` |
| `gemini-cli` | unverified, needs `--allow-unverified` | `gemini -p "" --approval-mode plan` with the system prompt on stdin |
| `codex` | not supported | its read-only sandbox still lets commands read host files, so a tool-less run cannot be guaranteed |

Other harnesses: use `--manual`, or open a PR adding a template to `internal/review/review.go` with evidence for
every flag. A template must disable tools, MCP and extensions, must not read the working directory, and must take the
prompt on stdin.

## What the reviewer sees and cannot do

- It receives the skill's files as text on **stdin** inside random delimiters, plus the scanner's findings.
  Skill content never appears in the argument list.
- It runs with **no tools**, in an **empty temporary working directory**, with a **scrubbed environment**:
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
4. Shows it in the approval screen. A reviewer verdict of `fail` requires `approve --override --reason`, the same as
   a scanner `block`. Set `require_review = true` in `skigard.toml` to refuse approvals with no review at all.

## Limits

- A model can be fooled. That is why the scanner and the human come first and the review can only add doubt.
- The provider may refuse to read malicious content (this happened in testing: "Cyber safeguards flagged this message").
  skigard reports the failure; the scan result still stands.
- Each review is one call to your own plan or API account.
