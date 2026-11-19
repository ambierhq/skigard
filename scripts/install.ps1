# Install skigard on Windows (amd64 or arm64). No administrator rights needed.
#   irm https://raw.githubusercontent.com/ambierhq/skigard/main/scripts/install.ps1 | iex
#
# Environment variables (all optional):
#   SKIGARD_VERSION          release to install, with or without a leading "v" (default: latest)
#   SKIGARD_INSTALL_DIR      install directory (default: %LOCALAPPDATA%\Programs\skigard)
#   SKIGARD_BASE_URL         override the download location (used by tests)
#   SKIGARD_NO_MODIFY_PATH   set to 1 to leave the user PATH alone
#   SKIGARD_SKIP_COSIGN      set to 1 to skip the cosign check even if cosign is installed
#
# The archive is checked against checksums.txt (SHA-256). If cosign is installed, checksums.txt is also
# verified against its keyless signature from this repository's release workflow.
# Works in Windows PowerShell 5.1 and PowerShell 7.

function Install-Skigard {
    Set-StrictMode -Version 3.0
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'

    $repo = 'ambierhq/skigard'
    function Fail([string]$Message) { throw "skigard-install: $Message" }
    function Get-EnvVar([string]$Name) { [Environment]::GetEnvironmentVariable($Name, 'Process') }

    # Windows PowerShell 5.1 may default to TLS 1.0/1.1.
    try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch { }

    $version = Get-EnvVar 'SKIGARD_VERSION'
    if (-not $version) { $version = 'latest' }
    if ($version -ne 'latest') {
        if ($version -notmatch '^v?[0-9A-Za-z.+_-]+$') { Fail "invalid SKIGARD_VERSION '$version'" }
        if (-not $version.StartsWith('v')) { $version = "v$version" }
    }

    $arch = Get-EnvVar 'PROCESSOR_ARCHITEW6432'
    if (-not $arch) { $arch = Get-EnvVar 'PROCESSOR_ARCHITECTURE' }
    switch ($arch) {
        'AMD64' { $arch = 'amd64' }
        'ARM64' { $arch = 'arm64' }
        default { Fail "unsupported CPU '$arch' (supported: amd64, arm64)" }
    }

    $base = Get-EnvVar 'SKIGARD_BASE_URL'
    if ($base) { $base = $base.TrimEnd('/') }
    elseif ($version -eq 'latest') { $base = "https://github.com/$repo/releases/latest/download" }
    else { $base = "https://github.com/$repo/releases/download/$version" }

    $dir = Get-EnvVar 'SKIGARD_INSTALL_DIR'
    if (-not $dir) {
        $local = Get-EnvVar 'LOCALAPPDATA'
        if (-not $local) { Fail 'LOCALAPPDATA is not set; set SKIGARD_INSTALL_DIR' }
        $dir = Join-Path $local 'Programs\skigard'
    }

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ('skigard-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        function Get-Asset([string]$Name) {
            $dest = Join-Path $tmp $Name
            try { Invoke-WebRequest -UseBasicParsing -Uri "$base/$Name" -OutFile $dest }
            catch { Fail "download failed: $base/$Name ($($_.Exception.Message)); is '$version' a published release with a windows/$arch build?" }
            $dest
        }

        $sums = Get-Asset 'checksums.txt'

        if ((Get-EnvVar 'SKIGARD_SKIP_COSIGN') -ne '1' -and (Get-Command cosign -ErrorAction SilentlyContinue)) {
            $bundle = Get-Asset 'checksums.txt.sigstore.json'
            & cosign verify-blob --bundle $bundle `
                --certificate-identity-regexp "^https://github.com/$repo/\.github/workflows/release\.yml@refs/tags/v" `
                --certificate-oidc-issuer https://token.actions.githubusercontent.com $sums | Out-Null
            if ($LASTEXITCODE -ne 0) { Fail 'cosign signature verification of checksums.txt failed' }
            Write-Host 'skigard-install: cosign signature verified'
        }

        $suffix = "_windows_$arch.zip"
        $line = Get-Content -LiteralPath $sums | Where-Object { $_ -match '^\s*([0-9a-fA-F]{64})\s+\*?(\S+)\s*$' -and $Matches[2].EndsWith($suffix) } | Select-Object -First 1
        if (-not $line) { Fail "no release asset for windows/$arch in checksums.txt" }
        $null = $line -match '^\s*([0-9a-fA-F]{64})\s+\*?(\S+)\s*$'
        $want = $Matches[1].ToLowerInvariant()
        $asset = $Matches[2]
        if ($asset -match '[\\/]' -or $asset.Contains('..')) { Fail "unexpected asset name '$asset'" }

        $zip = Get-Asset $asset
        $got = (Get-FileHash -Algorithm SHA256 -LiteralPath $zip).Hash.ToLowerInvariant()
        if ($got -ne $want) { Fail "checksum mismatch for $asset (want $want, got $got)" }

        $extract = Join-Path $tmp 'x'
        Expand-Archive -LiteralPath $zip -DestinationPath $extract -Force
        $exe = Join-Path $extract 'skigard.exe'
        if (-not (Test-Path -LiteralPath $exe)) { Fail "skigard.exe not found in $asset" }

        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        $target = Join-Path $dir 'skigard.exe'
        try { Copy-Item -LiteralPath $exe -Destination $target -Force }
        catch { Fail "could not write $target ($($_.Exception.Message)); close running skigard processes and try again" }
    }
    finally {
        Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Host "skigard-install: installed to $target"

    if ((Get-EnvVar 'SKIGARD_NO_MODIFY_PATH') -ne '1') {
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        if (-not $userPath) { $userPath = '' }
        $norm = $dir.TrimEnd('\')
        $present = $userPath.Split(';') | Where-Object { $_.TrimEnd('\') -ieq $norm }
        if (-not $present) {
            $newPath = if ($userPath) { $userPath.TrimEnd(';') + ';' + $dir } else { $dir }
            [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
            Write-Host "skigard-install: added $dir to your user PATH (open a new terminal to pick it up)"
        }
        if (-not (($env:Path -split ';') | Where-Object { $_.TrimEnd('\') -ieq $norm })) { $env:Path = "$env:Path;$dir" }
    }

    & $target version
    if ($LASTEXITCODE -ne 0) { Fail 'the installed skigard.exe did not run' }
}

Install-Skigard
