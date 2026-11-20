# skigard on Windows

skigard builds for `windows/amd64` and `windows/arm64`. Everything works except automated review (see below).
This page lists what is different from Linux and macOS.

## Where files live

| What | Windows default | Other systems |
| --- | --- | --- |
| `skigard.toml`, `skigard.lock`, approved `skills/`, `reviews/`, `marketplace/` | `%APPDATA%\skigard` | `~/.config/skigard` |
| `quarantine/` (untrusted downloads) and `state/` (including the lock file `.lock`) | `%LOCALAPPDATA%\skigard` | same directory as above |

`%APPDATA%` is the roaming profile. On work or school accounts it can be copied to other machines, so the
untrusted, large, machine-local parts stay in `%LOCALAPPDATA%`.

Overrides, in order: `--home`, `SKIGARD_HOME`, `XDG_CONFIG_HOME` (`%XDG_CONFIG_HOME%\skigard`). All of them put
everything in one directory. If `APPDATA` or `LOCALAPPDATA` is unset, skigard uses `%USERPROFILE%\AppData\Roaming`
and `\Local`.

Harness paths in the registry: `~` is `%USERPROFILE%` and `{config}` is `%APPDATA%` (or `%XDG_CONFIG_HOME%` when
set). Harnesses that keep their skills under `~/.config` on Windows too (some do, because they use XDG
conventions) may need a `skigard harness add` entry with the real path. Entries stay `verified = false` until
someone has checked them on a Windows machine. Installed binaries are found through `PATH` and `PATHEXT`
(`.exe`, `.cmd`, `.bat`, plus `.ps1`), and in the usual npm, bun, Go and WinGet link directories.

## Approving

`skigard approve` reads the console (`CONIN$`) and writes to `CONOUT$`, the Windows counterpart of `/dev/tty`.
It works when stdin and stdout are redirected, and it refuses when the process has no console. There is no flag,
environment variable or stdin path. The residual risk is the same as on Unix: a process that owns a ConPTY can
type into the console. See [threat-model.md](threat-model.md).

## Installing into harnesses

Windows installs by copy, never by symlink, because creating symlinks needs a privilege most accounts lack.
Run `skigard sync` after approving a new version. `skigard verify` still compares every installed copy with the
approved hash.

## Automated review is disabled

`skigard review NAME` fails closed on Windows with a clear message. On Unix the reviewer is isolated by policy,
not by an OS sandbox (the macOS and Linux sandboxes do not exist on Windows; WSL gets the Linux one): the harness CLI runs with tools, MCP, extensions and sessions disabled, in an empty temporary
directory, with a scrubbed environment, the skill on stdin only, and bounded output. Those flags have not been
verified against the Windows builds of the CLIs (npm installs `.cmd` shims that go through `cmd.exe` argument
parsing), and the standard library offers no OS-level substitute. So skigard does not run an unverified reviewer.
Use `skigard review NAME --manual` and `skigard review record` instead. Review stays advisory: the scanner and your
approval do not depend on it, unless you set `require_review`, in which case record a manual review.

## Line endings and the content hash

The content hash covers raw bytes. skigard never normalises or rewrites line endings: skills are fetched as
archives (never `git checkout`, which `core.autocrlf` rewrites) and copied byte for byte. Consequences:

- A skill hashes the same on Linux, macOS and Windows as long as the bytes are the same (tested with a pinned
  fixture in `internal/hash/testdata`, which `.gitattributes` marks `-text`).
- **A skill you edit locally on Windows and save with CRLF line endings has a different hash from the LF version**,
  so it counts as changed and needs a new review and approval. Configure your editor to keep LF for skills.
- If you keep skills in a git repository on Windows, set `* -text` or `*.sh text eol=lf` in its `.gitattributes`,
  otherwise `autocrlf` will change the bytes skigard sees.

## Execute bit

Windows files have no execute bit. Since hash version 2 the **content hash does not include the file mode**, so one
skill has the same `content_hash` on Windows, Linux and macOS, and a lock written on one verifies on the others.

The execute bits are recorded separately as the lock entry's `exec_set` (taken from the source archive's modes at
fetch time, so it is right even when you fetch on Windows) with its own `exec_hash`. Windows cannot represent or check
it: there, `verify` reports the exec set as "not applicable" and checks only the content hash. The check is enforced
on Linux and macOS.

Locks from before hash v2 (`hash_version = 1`) still verify, but a skill with executable scripts has two different v1
hashes, so on Windows it verifies only if it has none. Run `skigard lock upgrade` on Linux or macOS to move such entries
to version 2. See [lockfile.md](lockfile.md).

## File names skigard refuses

Because names matter on Windows, skigard rejects archive entries and skill names that Windows treats specially,
on every platform (so a skill that is accepted anywhere can be installed everywhere):

- reserved device names, with or without an extension (`CON`, `NUL`, `AUX`, `PRN`, `COM1`-`COM9`, `LPT1`-`LPT9`,
  `CONIN$`, `CONOUT$`), including as a skill name
- `:` (alternate data streams like `file.txt:stream`, drive-relative `C:foo`), `\`, `<>"|?*` and control characters
- names ending in a dot or a space (Windows strips them)
- 8.3 short-name aliases such as `PROGRA~1`
- names that differ only by case, including parent directories
- absolute and UNC paths, `..` traversal

Symlinks and junctions in a local source are rejected, and symlinks in archives are skipped.

## Locking

Two skigard processes use the same home safely: mutating commands take an exclusive `LockFileEx` lock on
`state\.lock`, retry for up to ten seconds, then fail with "another skigard process is using ...".

## How this is tested

There is no Windows machine behind skigard, so Windows is tested in CI, in two ways.

- **Every pull request** (`ci.yml`): the unit tests and cross-compile on `windows-latest`, and `e2e/windows`, which builds
  skigard from source and drives `approve` through a real pseudo console (ConPTY).
- **The published release** ([`windows-smoke.yml`](../.github/workflows/windows-smoke.yml)): weekly, after every release
  and on demand (`gh workflow run windows-smoke.yml -f version=0.1.2`), it uses the artifacts a user gets, on
  `windows-latest` (amd64) and `windows-11-arm` (arm64):
  `irm https://ambierhq.com/skigard/install.ps1 | iex` in Windows PowerShell 5.1 and PowerShell 7 (PATH in a new
  process, version, idempotent re-run), Scoop (`bucket add`, `install`, `uninstall`), the release zip (SHA-256 against
  `checksums.txt`, `cosign verify-blob`, `gh attestation verify`), the quick start with a real console
  (`e2e/windows`, `TestPublishedSmoke`: wrong answer cancels with exit 3, right answer approves, `verify --strict`,
  `install`, the split `%APPDATA%` / `%LOCALAPPDATA%` layout, and `approve` refused under `CLAUDECODE=1` and under a
  process named `claude.exe`; releases before 0.1.2 have no ancestor check, so that step is skipped for them), `npx skigard@VERSION` (skipped when that version is not on npm yet), and the Linux build
  inside WSL.

This workflow is advisory: it is not a required check, and a green run does not mean a person has used skigard
on Windows. What it cannot cover: the Scoop install runs as the runner's administrator account (Scoop needs
`-RunAsAdmin` there), SmartScreen and Defender prompts do not appear on runners, and the Windows reviewer sandbox does
not exist (review is disabled, above).

In WSL, keep skills in the Linux filesystem (`~`), not under `/mnt/c`: the Windows drives report every file as mode 777, so
the scanner flags `SKILL.md` as an executable file (SKG-FS-004). The WSL job copies the example off `/mnt` for this reason.
