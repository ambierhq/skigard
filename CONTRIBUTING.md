# Contributing

```sh
mise install
mise run check     # golangci-lint (gofmt, goimports, gosec, staticcheck, errcheck...), editorconfig, go test ./...
mise run fmt       # format Go code
mise run e2e
```

- Keep the runtime dependency list at one (the TOML parser). Ask before adding another.
- Anything that touches fetching, extraction, paths, hashing, approval or the reviewer runner needs a test that
  tries to break it, and a line in `docs/threat-model.md` if it changes what skigard promises.
- **Adding a harness:** edit `internal/harness/registry.toml`, run `go test ./internal/harness -run Docs -update`, and put your
  evidence (a link to the harness's docs, or "runs on my machine") in the PR. New entries start as `verified = false`.
- **Adding a scanner rule:** add it to `internal/scan/rules.go` with a `Pre` prefilter, add samples to
  `TestPrefilterCoversSamples`, a fixture to `TestMaliciousFixtures`, bump `RulesVersion`, and run
  `go test ./internal/scan -run Docs -update`.
- **Adding a reviewer template:** see `docs/review.md`. It must disable tools, MCP and extensions and take input on stdin.
- Do not copy code from other projects. Learn from them, reimplement, and credit them in `THIRD_PARTY_NOTICES.md`.
