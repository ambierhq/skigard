# Security policy

skigard is a security tool, so reports are welcome and taken seriously.

Please report vulnerabilities privately through GitHub's "Report a vulnerability" (Security tab) on this repository,
not in a public issue. Include the command you ran, your OS, and a minimal skill or archive that reproduces the problem.

In scope: ways to get unreviewed bytes installed, to approve without a human, to write outside the skigard home or a
harness directory, to make skigard execute skill content, SSRF in the fetcher, and terminal-escape injection.
Out of scope: a malicious skill that passes the heuristic scanner (please add a rule or a fixture instead), and attackers
who already control your account.

See [docs/threat-model.md](docs/threat-model.md) for what skigard does and does not promise.
