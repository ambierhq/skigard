# Scanner rules

<!-- Generated from internal/scan. Run: go test ./internal/scan -run Docs -update -->

Rules version `2026.4`. The scanner is deterministic and heuristic: it flags, a human decides. It never executes anything.

Severity maps to a verdict: any `high` or `critical` finding gives **block**; `medium` (or incomplete coverage) gives **review**; otherwise **pass**.
In the prose of Markdown/text files (outside code fences) command-like mentions matching rules marked `(prose-low)` are downgraded to `low`.
Accepted findings (`approve --accept REASON`) and rules allowed for a source (`skigard allow`) can suppress `medium` and below, never `high` or `critical`.
`risk` is 0 to 100 and **higher is riskier**: each rule counts once at the weight of its worst finding (critical 40, high 20, medium 5, low 1), plus a small bump when it matched in many places. The safety score on the other hand is 1 to 10 and higher is safer.

| ID | Name | Family | Severity | What it flags |
|---|---|---|---|---|
| `SKG-CHAIN-001` | exfiltration-chain | CHAIN | critical | reads credential locations and has network access |
| `SKG-CHAIN-002` | persistent-exec | CHAIN | high | installs persistence and executes dynamic code |
| `SKG-EXEC-001` | pipe-to-shell | EXEC | high | downloads and pipes straight into an interpreter |
| `SKG-EXEC-002` | reverse-shell | EXEC | high | reverse-shell pattern |
| `SKG-EXEC-003` | destructive | EXEC | high | recursive delete of root or home |
| `SKG-EXEC-004` | dynamic-exec | EXEC | medium | dynamic code or process execution (prose-low) |
| `SKG-EXEC-005` | privilege | EXEC | medium | privilege or permission change (prose-low) |
| `SKG-EXEC-006` | persistence | EXEC | medium | shell startup or scheduler persistence (prose-low) |
| `SKG-FS-001` | binary-file | FS | high | binary or non-UTF-8 file that cannot be reviewed (makes coverage partial) |
| `SKG-FS-002` | executable-binary | FS | high | native executable (ELF, Mach-O, PE) |
| `SKG-FS-003` | large-file | FS | medium | file too large to inspect (makes coverage partial) |
| `SKG-FS-004` | executable-file | FS | medium | executable bit set |
| `SKG-FS-005` | script-present | FS | low | bundled script (reminder to read its code) |
| `SKG-FS-006` | asset-not-inspected | FS | info | known binary asset (image, font, pdf); contents not inspected |
| `SKG-FS-007` | symlink | FS | high | symlink |
| `SKG-FS-008` | too-many-files | FS | high | more files than the scanner inspects (makes coverage partial) |
| `SKG-MANIFEST-001` | invalid-skill | MANIFEST | high | SKILL.md missing, no frontmatter, or invalid name/description |
| `SKG-MANIFEST-002` | broad-tools | MANIFEST | medium | `allowed-tools` grants broad access (`Bash` without a pattern, or `*`) |
| `SKG-NET-001` | network-tool | NET | medium | network tool (prose-low) |
| `SKG-NET-002` | network-lib | NET | medium | network library (prose-low) |
| `SKG-NET-003` | external-url | NET | low | external URL |
| `SKG-OBF-001` | invisible-unicode | OBF | high | zero-width, bidirectional or tag characters |
| `SKG-OBF-002` | base64-decode | OBF | medium | decodes base64 (possible obfuscation) (prose-low) |
| `SKG-OBF-003` | encoded-blob | OBF | medium | long encoded-looking blob |
| `SKG-OBF-004` | html-comment | OBF | medium | HTML comment (can hide instructions) |
| `SKG-OBF-005` | nul-bytes | OBF | medium | NUL bytes inside a text file (can hide content from reviewers) |
| `SKG-OBF-006` | encoded-command | OBF | medium | runs an encoded PowerShell command (prose-low) |
| `SKG-PROMPT-001` | instruction-override | PROMPT | high | tries to override higher-priority instructions |
| `SKG-PROMPT-002` | concealment | PROMPT | high | asks the agent to hide actions from the user |
| `SKG-PROMPT-003` | safety-bypass | PROMPT | medium | asks to bypass safety controls |
| `SKG-SECRETS-001` | credential-access | SECRETS | high | touches credential or secret locations |
| `SKG-SECRETS-002` | env-access | SECRETS | medium | reads environment or secrets (prose-low) |
| `SKG-SECRETS-003` | exfiltration | SECRETS | high | uploads secrets or environment over the network |
| `SKG-SUPPLY-001` | package-install | SUPPLY | medium | installs third-party packages (prose-low) |
| `SKG-SUPPLY-002` | remote-instructions | SUPPLY | medium | loads instructions from a remote URL |
| `SKG-SUPPLY-003` | remote-instructions-indirect | SUPPLY | medium | loads the real instructions from somewhere else and follows them |
| `SKG-SUPPLY-004` | install-hook | SUPPLY | medium | package install hook (runs code when the skill's dependencies are installed) |

## Capabilities and chains

Findings imply capabilities (`sensitive_read`, `network_egress`, `dynamic_exec`, `persistence`, `remote_exec`, `destructive_write`, `env_read`, `supply_chain`, `external_instruction`).
Dangerous combinations raise their own finding: `sensitive_read` + `network_egress` is `SKG-CHAIN-001` (critical), `persistence` + `dynamic_exec` is `SKG-CHAIN-002` (high).
