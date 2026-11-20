# skigard

A safer way to manage agent skills: skigard scans and scores each agent skill from 1 to 10, pins the exact bytes in a lockfile,
and installs in one command (default mode `auto`, threshold 7); strict mode (`skigard config set mode strict`) asks a human first.

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
