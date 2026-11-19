# npm, npx, bunx, pnpm dlx

```sh
npx skigard version
npm install -g skigard
bunx skigard version
pnpm dlx skigard version
```

How it works: `skigard` is a ~60-line launcher. The binary is in `@ambierhq/skigard-<os>-<cpu>` (linux, darwin, win32;
x64, arm64), an `optionalDependency` that npm only installs for your platform. There is **no postinstall script** and nothing
is downloaded at install time. Do not install with `--omit=optional` / `--no-optional`: the launcher then has no binary and says so.

The launcher inherits stdio and propagates the exit code, and `skigard approve` opens your terminal itself, so approval
works through `npx` in a real terminal. Packages are published from GitHub Actions with npm provenance, so
`npm audit signatures` verifies where they were built.
