# Contributing

```sh
mise install
mise run check     # golangci-lint (gofmt, goimports, gosec, staticcheck, errcheck...), editorconfig, go test ./...
mise run fmt       # format Go code
mise run e2e
mise run demo      # optional: rebuild assets/media (GIF, MP4, WebM, poster, screenshots); needs vhs, ttyd, ffmpeg
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
- **Adding a reviewer template:** see `docs/review.md`. It must disable tools, MCP and extensions and take input on stdin. The same templates run `merge --with`, so a template also has to be safe to hand merge input to (verified ones only).
- Docs: `docs/commands.md`, `docs/harnesses.md` and `docs/rules.md` are generated (`mise run docs`); the rest is hand-written. Put a new page in the docs table of the README and the sidebar of the website.
- Do not copy code from other projects. Learn from them, reimplement, and credit them in `THIRD_PARTY_NOTICES.md`.

## Demo media

`mise run demo` (or `scripts/demo.sh`) rebuilds everything in `assets/media/` from the real binary: four VHS scenes
in `assets/demo/`, a GIF for the README, MP4/WebM for the site, a poster and five screenshots. Install the tools with
`brew install vhs ttyd ffmpeg` (or the Linux equivalents). Details, the Windows clip and the video script are in
[`assets/demo/README.md`](assets/demo/README.md). The mark used by the poster lives in `assets/brand/`.

## CI and releases

- Every workflow action is pinned to a full commit SHA with a `# vX.Y.Z` comment, and workflows start with
  `permissions: {}` and grant per job. Renovate proposes bumps (see `docs/maintenance/deps-runbook.md`). Run
  `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` before changing a workflow.
- Required checks on `main`: `test (ubuntu-latest|macos-latest|windows-latest)`, `cross-compile (windows/amd64|windows/arm64)`,
  `editorconfig`, `fuzz`. Do not rename these jobs. Other workflows (`govulncheck`, `codeql`, `scorecard`,
  `dependency-review`, `workflow-lint`, `installers`, `links`, `fuzz-nightly`, `windows-smoke`) are advisory. `windows-smoke` tests the
  *published* release on Windows runners (weekly, after a release, or `gh workflow run windows-smoke.yml -f version=X`); see `docs/windows.md`.
- Releases are tag-driven: a maintainer pushes `vX.Y.Z` and `.github/workflows/release.yml` runs GoReleaser (archives,
  packages, SBOMs, cosign signatures, provenance attestation). PRs that touch packaging get a snapshot
  dry run. `scripts/install.sh` and `scripts/install.ps1` are tested against a snapshot in the `installers` workflow.
- Do not change the archive name pattern `skigard_<version>_<os>_<arch>`; mise (`ubi:`, `github:`) and eget depend on it.
