# Security policy

skigard is a security tool, so reports are welcome and taken seriously.

Please report vulnerabilities privately through GitHub's "Report a vulnerability" (Security tab) on this repository,
not in a public issue. If you cannot use GitHub advisories, email security@ambierhq.com. Include the command you ran, your
OS, and a minimal skill or archive that reproduces the problem.

**Response:** we acknowledge a report within 3 business days and give a fix or a mitigation plan within 30 days.

## In scope

Bypasses of the guarantees in the [threat model](docs/threat-model.md), for example:

- approving without going through the terminal device (a flag, stdin, an environment variable, a code path other than a pseudo-terminal), or getting unreviewed bytes installed;
- an agent forcing an install, lowering the `mode`/`threshold`, or auto-installing a skill below the threshold or with a hard stop (scanner `block`, reviewer `fail`) through `add`, `update` or `adopt`, or a merge result (`merge`) installing without a human `approve`;
- bugs in content-hash, exec-set or lockfile verification (`verify`, `install`, `sync` accepting bytes that were not approved);
- extraction or path escapes: writing outside the skigard home or a harness directory, symlink or archive tricks;
- escapes from the review runner (tools, network, files or environment reachable by the reviewer despite the sandbox);
- making skigard execute skill content, SSRF in the fetcher, and terminal-escape injection.

## Out of scope

- **The pseudo-terminal case is a documented limitation, not a vulnerability.** A process that creates its own pseudo-terminal
  (for example `script`, Python's `pty`, `expect`) can type the confirmation. See
  [the pseudo-terminal case](docs/threat-model.md#the-pseudo-terminal-case). Signed approvals are planned. Reports of
  agent-detection gaps (a harness or variable we do not know) are welcome as normal issues or PRs.
- A malicious skill that passes the heuristic scanner (please add a rule or a fixture instead).
- Attackers who already control your account, including forging `skigard.lock` or writing into harness skills directories.

See [docs/threat-model.md](docs/threat-model.md) for what skigard does and does not promise.
