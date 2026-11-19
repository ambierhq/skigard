# Dependency update runbook (weekly)

For the maintenance agent (Hermes) and for humans. Policy: nothing is adopted until it is at least **7 days old**
and has been reviewed. Renovate (`renovate.json`) opens PRs labelled `deps` on Monday mornings, grouped, with
`minimumReleaseAge: 7 days`. It never automerges.

## Boundaries

- The agent's GitHub token has `contents` and `pull-requests` on this repository only: no admin, no secrets, and no
  `workflows` permission. Changes to `.github/workflows/**` (including the pinned-actions bump) are reviewed and
  merged by a human.
- Never push to `main`. Never merge when a check failed or was skipped.
- A PR that adds a **new module** to `go.mod` is rejected unless the owner agreed (`AGENTS.md`: one runtime dependency).
- Do not run anything from an updated dependency beyond `mise run check`, `mise run e2e` and the checks below.

## Each week

1. **List and age-check.** `gh pr list --label deps --state open`. For each update, confirm the release is at least
   7 days old (`gh api repos/<owner>/<repo>/releases/tags/<tag> --jq .published_at`, or `go list -m -json <mod>@<ver>`
   `.Time`) and that the upstream owner or maintainers have not changed (`gh api repos/<owner>/<repo> --jq .owner.login`,
   compare with the previous version's repository). Security advisories may bypass the age gate, but only with an
   explicit review note in the PR.
2. **Read the change.** Read the upstream changelog and the dependency diff. For Go modules:
   ```sh
   go mod verify
   go mod why -m <module>
   go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...
   # compare the module source of the old and new version
   old="$(go mod download -json <module>@<old> | jq -r .Dir)"
   new="$(go mod download -json <module>@<new> | jq -r .Dir)"
   diff -ru "$old" "$new"
   ```
   Look in the new code for: new network calls (`net/http`, `net.Dial`), process execution (`os/exec`), `unsafe`,
   `//go:linkname`, `init()` side effects, embedded binaries, and changes to `go.sum` that touch more than the
   updated module. Anything new of that kind: label `needs-human`.
3. **Run the project checks** on the PR branch: `mise run check` and `mise run e2e`. For tool pins in `mise.toml`,
   also run `mise install`. For GoReleaser changes run `mise run snapshot` and look at the produced archives and packages.
4. **Check CI.** All of these must be green on the PR (the first seven are required on `main`):
   `test (ubuntu-latest)`, `test (macos-latest)`, `test (windows-latest)`, `cross-compile (windows/amd64)`,
   `cross-compile (windows/arm64)`, `editorconfig`, `fuzz`, and also `govulncheck`, `dependency-review`,
   `codeql (go)`, `codeql (actions)`, `go-mod`, and (for packaging changes) `snapshot`, `install-sh`,
   `install-ps1`, `packages`. Windows CI must be green.
5. **Comment and decide.** Post a short summary (what changed, age, upstream review result, checks run) and then
   either merge (squash) or label `needs-human` and say why. Majors, GitHub Actions, the Go toolchain and anything
   in step 2 that looked new always go to `needs-human`.

## Not covered by Renovate

- Tools pinned in workflows by version (`golangci-lint` `v2.15.0`, GoReleaser `v2.20.2`, `editorconfig-checker`,
  `zizmor`) must be kept in step with `mise.toml` by hand. The weekly review should check that they match.
