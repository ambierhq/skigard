# Contributing

```sh
mise install
mise run check     # golangci-lint (gofmt, goimports, gosec, staticcheck, errcheck...), editorconfig, go test ./...
mise run fmt       # format Go code
mise run e2e
mise run snapshot  # optional: GoReleaser snapshot build into ./dist (archives, deb/rpm/apk/arch packages)
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

## CI and releases

- Every workflow action is pinned to a full commit SHA with a `# vX.Y.Z` comment, and workflows start with
  `permissions: {}` and grant per job. Renovate proposes bumps (see `docs/maintenance/deps-runbook.md`). Run
  `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` before changing a workflow.
- Required checks on `main`: `test (ubuntu-latest|macos-latest|windows-latest)`, `cross-compile (windows/amd64|windows/arm64)`,
  `editorconfig`, `fuzz`. Do not rename these jobs. Other workflows (`govulncheck`, `codeql`, `scorecard`,
  `dependency-review`, `workflow-lint`, `installers`, `links`, `fuzz-nightly`) are advisory.
- Releases are tag-driven: a maintainer pushes `vX.Y.Z` and `.github/workflows/release.yml` runs GoReleaser (archives,
  packages, SBOMs, cosign signatures, provenance attestation, container image). PRs that touch packaging get a snapshot
  dry run. `scripts/install.sh` and `scripts/install.ps1` are tested against a snapshot in the `installers` workflow.
- Do not change the archive name pattern `skigard_<version>_<os>_<arch>`; mise (`ubi:`, `github:`) and eget depend on it.
