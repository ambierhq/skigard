## What and why

<!-- One or two sentences. Link the issue if there is one. -->

## Checklist

- [ ] `mise run check` passes (lint, formatting, editorconfig, `go test ./...`)
- [ ] `mise run e2e` passes
- [ ] If I touched the registry or scanner rules, I ran `mise run docs` (generated docs are never edited by hand)
- [ ] New harness entries start with `verified = false` and the evidence is in this PR
- [ ] I did not add a way for an agent to approve (no `--yes`, stdin path, env bypass or test hook)
- [ ] I did not add a runtime dependency
