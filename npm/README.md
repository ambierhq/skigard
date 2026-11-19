# skigard

Lockfile and review gate for agent skills: fetch to quarantine, scan, advisory agent review, **human** approval,
hash-pinned lockfile, install into your agent harnesses.

```sh
npx skigard version          # also: bunx skigard, pnpm dlx skigard
npm install -g skigard
skigard --help
```

This package is a thin launcher. The native binary comes from the matching `@ambierhq/skigard-<os>-<cpu>`
optional dependency (linux, darwin, win32; x64, arm64). There is **no postinstall script** and nothing is downloaded
at install time. `skigard approve` reads the controlling terminal itself, so it works through `npx` in a real terminal
and refuses everywhere else.

Source, docs and release signatures: https://github.com/ambierhq/skigard
