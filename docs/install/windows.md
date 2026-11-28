# Install on Windows

`windows/amd64` and `windows/arm64`. Windows 10 or 11, PowerShell 5.1 or 7. No administrator rights needed.

## PowerShell installer

```powershell
irm https://ambierhq.com/skigard/install.ps1 | iex
```

Installs the latest release to `%LOCALAPPDATA%\Programs\skigard` (or the directory in `SKIGARD_INSTALL_DIR`) after checking `checksums.txt`, and adds it
to your user `PATH`. Open a new terminal afterwards. Pin a version with `$env:SKIGARD_VERSION = "v0.4.0"` first.

## winget (pending review)

The manifest is submitted ([winget-pkgs#484914](https://github.com/microsoft/winget-pkgs/pull/484914), validation passed) and waits for a Microsoft moderator, so this
command does **not** work yet; it will once that pull request is merged:

```powershell
winget install Ambier.Skigard
```

## Scoop

The bucket is live and updated by the release workflow, but it has not been tested by hand on Windows yet.

```powershell
scoop bucket add ambierhq https://github.com/ambierhq/scoop-bucket
scoop install skigard
```

## Go

```powershell
go install github.com/ambierhq/skigard/cmd/skigard@latest
```

## Notes

- The binary is not code-signed (there is no code-signing certificate and none is planned for now), so SmartScreen may warn on a direct download. winget, Scoop and
  the installer verify the SHA-256 of what they download.
- `approve` reads the console (`CONIN$`), so it works with redirected streams and refuses without a console.
- Automated `review` is disabled on Windows; use `skigard review NAME --manual` or run skigard in WSL.
- Installs are copies, not symlinks. Run `skigard sync` after approving a new version.
- Files live in `%APPDATA%\skigard` and `%LOCALAPPDATA%\skigard`.

Details and caveats: [Windows notes](../windows.md). Next: [Getting started](../getting-started.md).
