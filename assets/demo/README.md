# Demo media

Everything under `assets/media/` is generated. Rebuild it with one command:

```sh
brew install vhs ttyd ffmpeg   # once (macOS). On Linux install vhs, ttyd and ffmpeg from your package manager
mise run demo                  # same as scripts/demo.sh; takes about 3 minutes
```

It builds the binary from the working tree, records the scenes with [VHS](https://github.com/charmbracelet/vhs) into a
temp directory (`/tmp/skigard-demo`, so no real `SKIGARD_HOME` is touched), stitches and encodes them, and fails if the
GIF is over 5 MB or a site video over 3 MB. Set `CHROME=/path/to/chrome` if the poster step cannot find a browser.

| Scene | Tape | What it shows |
|---|---|---|
| 1 | `scene1-add-auto.tape` | `add` of the benign `commit-helper` in auto mode: scan PASS, score 10/10, installed automatically |
| 2 | `scene2-add-blocked.tape` | `add` of `free-tokens`: scan BLOCK, score 1/10, NOT installed, stays in quarantine with a warning and next commands |
| 3 | `scene3-agent-human.tape` | with `CLAUDECODE=1` set, `add --force-install` and `approve` are refused (exit 1); then a human runs `approve --override --reason` and types the skill name at the prompt |
| 4 | `scene4-verify-tamper.tape` | `verify` passes, a file is edited, `verify` fails |

- Fixtures: `fixtures/skills/commit-helper` (benign) and `fixtures/skills/free-tokens` (malicious-looking: only fake
  strings, and the only hosts are `*.example.invalid`, which cannot resolve). skigard never executes skill content.
- `lib/theme.tape` is the shared look (brand palette, window bar). `scripts/demo.sh` prepends it to each scene; run a
  tape standalone by replacing `@ACCENT@` in the theme with `#5AA9FF`, concatenating it with the tape, and exporting
  `DEMO_ROOT`, `DEMO_BIN`, `DEMO_TMP`.
- `lib/setup.sh` runs hidden at the start of each scene: it strips every agent environment variable (so the human steps are
  honest), points `SKIGARD_HOME` at a temp directory and prepares the state. Scene 3 sets `CLAUDECODE=1` on screen.
- Nothing is scripted around the rules. In scene 3 a human-like keystroke sequence is typed into VHS's pseudo-terminal
  (ttyd), which is a real tty. There is no flag, stdin path or env var that bypasses the prompt.
- skigard refuses `approve` when an agent harness is an ancestor of the process (name or environment marker). If you run
  `mise run demo` from inside an agent session, scene 3 shows that refusal instead of the prompt. Run it from a normal
  terminal, or re-parent it with the agent variables removed:
  `env -u PI_CODING_AGENT ... python3 scripts/detach.py mise run demo` (the documented evasion in `docs/threat-model.md`).
- `poster.html` renders `poster.png` with headless Chrome. The mark is `assets/brand/mark.svg` (provisional,
  option B); change that one file to change the mark.
- Fonts: the terminal uses Menlo (a system font, so it renders on macOS). Newsreader and IBM Plex Sans are not bundled yet;
  the poster falls back to Georgia and the system sans. Self-host them under `assets/brand/fonts/` when the site does.

Output files: `demo.gif` (README hero, 960 px), `demo.mp4` and `demo.webm` (site, 1120 px), `poster.png`,
`screenshot-1-auto-install.png`, `screenshot-1-block.png` (not installed), `screenshot-2-agent-refused.png`,
`screenshot-3-approve.png` (human override) and `screenshot-4-verify-fails.png`.
The site copies `demo.webm`, `demo.mp4` and `poster.png` from here via `website/scripts/sync-docs.mjs`.
The output shows no user name: `HOME` is a temp directory and the scenes do not print the lock's `by =` line.

## Windows clip (manual)

A Windows recording cannot be produced from macOS/Linux CI here, so record one by hand on Windows 10/11:

1. Install skigard (`scripts/install.ps1` or a release zip) and open **Windows Terminal** with a dark profile, 120x30.
2. `$env:SKIGARD_HOME = "$env:TEMP\skigard-demo"`, copy `assets/demo/fixtures/skills` to a work folder, `cd` into it.
3. Run the scene 1-4 commands from `video-script.md` (`approve --override` is a ConPTY prompt: type the skill name yourself).
   For the agent refusal use `$env:CLAUDECODE = "1"`.
4. Record with the Xbox Game Bar (`Win+Alt+R`) or ScreenToGif, then encode: `ffmpeg -i in.mp4 -vf scale=1120:-2 -crf 30 windows.mp4`.
5. Save as `assets/media/demo-windows.mp4` (< 3 MB) and open a PR.

A later option: a `windows-latest` job running VHS or a ConPTY recorder on release tags (see follow-ups).
