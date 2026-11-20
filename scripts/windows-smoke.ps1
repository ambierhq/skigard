# Smoke tests of the PUBLISHED skigard release on Windows, the way a user gets it.
# Driven by .github/workflows/windows-smoke.yml; also runnable by hand:
#   pwsh scripts/windows-smoke.ps1 -Task installer -Version 0.1.2 -Latest 0.1.2
#
# -Task     installer | scoop | zip | quickstart | npm
# -Version  release to test, without a leading "v"
# -Latest   the newest release (what Scoop and an unpinned installer deliver)
# -Pinned   the run asked for a specific version: the installer is pinned with SKIGARD_VERSION
#
# Every check is recorded; the script exits 1 if any failed and writes a table to $GITHUB_STEP_SUMMARY.
# Works in Windows PowerShell 5.1 and PowerShell 7.
param(
    [Parameter(Mandatory)][ValidateSet('installer', 'scoop', 'zip', 'quickstart', 'npm')][string]$Task,
    [Parameter(Mandatory)][string]$Version,
    [string]$Latest = '',
    [switch]$Pinned
)
Set-StrictMode -Version 3.0
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch { }

$Version = $Version.TrimStart('v')
if (-not $Latest) { $Latest = $Version }
$Latest = $Latest.TrimStart('v')
if ($Version -notmatch '^[0-9A-Za-z.+_-]+$') { throw "invalid version '$Version'" }
$Repo = 'ambierhq/skigard'
# cosign is pinned by version and by the SHA-256 of the release binary (windows/amd64 is the only Windows build).
$CosignVersion = 'v3.1.4'
$CosignSha256 = 'cbe953787074611f1334cbf37dc1cf953995bb8c45398eb4fc7bb781d29c1f2a'

$script:results = New-Object System.Collections.Generic.List[object]

function Write-Group([string]$Title) { if ($env:GITHUB_ACTIONS) { Write-Host "::group::$Title" } else { Write-Host "== $Title" } }
function Write-EndGroup { if ($env:GITHUB_ACTIONS) { Write-Host '::endgroup::' } }

# Check records pass/fail/skip and never throws, so one failure does not hide the next.
function Check([string]$Name, [scriptblock]$Body) {
    Write-Group $Name
    $status = 'pass'; $detail = ''
    try { & $Body }
    catch {
        if ($_.Exception.Message -like 'SKIP:*') { $status = 'skip'; $detail = $_.Exception.Message.Substring(5).Trim() }
        else { $status = 'FAIL'; $detail = ($_.Exception.Message -replace '\s+', ' ') }
    }
    Write-EndGroup
    $line = "[$status] $Name"
    if ($detail) { $line += ": $detail" }
    Write-Host $line
    if ($status -eq 'FAIL' -and $env:GITHUB_ACTIONS) { Write-Host "::error title=$Task::$Name failed: $detail" }
    $script:results.Add([pscustomobject]@{ Name = $Name; Status = $status; Detail = $detail })
}
function Skip([string]$Why) { throw "SKIP: $Why" }
function Assert([bool]$Cond, [string]$Message) { if (-not $Cond) { throw $Message } }

# Run a native command, capture stdout+stderr as text, and return the exit code without tripping $ErrorActionPreference.
function Invoke-Native([string]$Exe, [string[]]$Arguments) {
    $old = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
    try {
        $out = & $Exe @Arguments 2>&1 | ForEach-Object { "$_" }
        $code = $LASTEXITCODE
    }
    finally { $ErrorActionPreference = $old }
    $text = ($out -join "`n")
    if ($text) { Write-Host $text }
    [pscustomobject]@{ Code = $code; Out = $text }
}

function Assert-VersionOutput([string]$Text, [string]$Want) {
    # `skigard version` prints "skigard 0.1.2" (some builds "skigard v0.1.2").
    Assert ($Text -match ('(?m)^skigard v?' + [regex]::Escape($Want) + '(\s|$)')) "version output '$Text' is not skigard $Want"
}

function Get-Arch {
    $a = $env:PROCESSOR_ARCHITEW6432; if (-not $a) { $a = $env:PROCESSOR_ARCHITECTURE }
    switch ($a) { 'AMD64' { 'amd64' } 'ARM64' { 'arm64' } default { throw "unsupported CPU $a" } }
}

function Get-Download([string]$Url, [string]$Dest) {
    Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $Dest
}

function Get-Sha256([string]$Path) { (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant() }

# Download the Windows zip for $Version, check it against checksums.txt, extract it. Returns the folder.
function Get-ReleaseZip([string]$Dir) {
    $arch = Get-Arch
    $base = "https://github.com/$Repo/releases/download/v$Version"
    $zipName = "skigard_${Version}_windows_$arch.zip"
    New-Item -ItemType Directory -Force -Path $Dir | Out-Null
    Get-Download "$base/checksums.txt" (Join-Path $Dir 'checksums.txt')
    Get-Download "$base/$zipName" (Join-Path $Dir $zipName)
    $want = $null
    foreach ($l in Get-Content -LiteralPath (Join-Path $Dir 'checksums.txt')) {
        if ($l -match '^\s*([0-9a-fA-F]{64})\s+\*?(\S+)\s*$' -and $Matches[2] -eq $zipName) { $want = $Matches[1].ToLowerInvariant() }
    }
    Assert ($null -ne $want) "$zipName is not listed in checksums.txt"
    $got = Get-Sha256 (Join-Path $Dir $zipName)
    Assert ($got -eq $want) "sha256 mismatch for ${zipName}: want $want, got $got"
    Write-Host "sha256 ok: $got  $zipName"
    Expand-Archive -LiteralPath (Join-Path $Dir $zipName) -DestinationPath (Join-Path $Dir 'x') -Force
    return $Dir
}

# What a freshly opened terminal would see: the registry's Machine and User PATH, not this process's.
function Invoke-InFreshEnvironment([scriptblock]$Body) {
    $old = $env:Path
    $machine = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    $user = [Environment]::GetEnvironmentVariable('Path', 'User')
    $env:Path = "$machine;$user"
    try { & $Body } finally { $env:Path = $old }
}

function Get-UserPathCount([string]$Dir) {
    $user = [Environment]::GetEnvironmentVariable('Path', 'User'); if (-not $user) { $user = '' }
    @($user.Split(';') | Where-Object { $_.TrimEnd('\') -ieq $Dir.TrimEnd('\') }).Count
}

# ---------------------------------------------------------------------------------------------------------------------

function Test-Installer {
    $dir = Join-Path $env:LOCALAPPDATA 'Programs\skigard'
    $shell = if ($PSVersionTable.PSEdition -eq 'Core') { 'pwsh' } else { 'powershell' }
    Write-Host "shell: $shell $($PSVersionTable.PSVersion); expect skigard $Version in $dir (pinned: $($Pinned.IsPresent))"

    Check 'skigard is not installed beforehand' {
        Assert (-not (Get-Command skigard -ErrorAction SilentlyContinue)) 'skigard already on PATH'
        Assert (-not (Test-Path (Join-Path $dir 'skigard.exe'))) "$dir already has skigard.exe"
    }

    function Invoke-Installer {
        if ($Pinned) { $env:SKIGARD_VERSION = $Version } else { Remove-Item Env:SKIGARD_VERSION -ErrorAction SilentlyContinue }
        # Exactly the documented command.
        Invoke-RestMethod https://ambierhq.com/skigard/install.ps1 | Invoke-Expression
    }

    Check 'irm https://ambierhq.com/skigard/install.ps1 | iex' {
        Invoke-Installer
        Assert (Test-Path (Join-Path $dir 'skigard.exe')) "no skigard.exe in $dir"
    }
    Check 'installed binary reports the release version' {
        $r = Invoke-Native (Join-Path $dir 'skigard.exe') @('version')
        Assert ($r.Code -eq 0) "exit $($r.Code)"
        Assert-VersionOutput $r.Out ($(if ($Pinned) { $Version } else { $Latest }))
    }
    Check 'user PATH has the install dir exactly once' {
        Assert ((Get-UserPathCount $dir) -eq 1) "user PATH entries for ${dir}: $(Get-UserPathCount $dir)"
    }
    Check 'a new process finds skigard on PATH' {
        Invoke-InFreshEnvironment {
            $r = Invoke-Native $shell @('-NoProfile', '-Command', 'skigard version; exit $LASTEXITCODE')
            Assert ($r.Code -eq 0) "exit $($r.Code) in a fresh $shell"
            Assert-VersionOutput $r.Out ($(if ($Pinned) { $Version } else { $Latest }))
            $c = Invoke-Native 'cmd.exe' @('/c', 'skigard version')
            Assert ($c.Code -eq 0) "exit $($c.Code) in a fresh cmd.exe"
            $w = Invoke-Native 'where.exe' @('skigard')
            Assert ($w.Out -like "*$dir*") "where.exe skigard: $($w.Out)"
        }
    }
    Check 're-running the installer is idempotent' {
        $before = (Get-FileHash -Algorithm SHA256 (Join-Path $dir 'skigard.exe')).Hash
        Invoke-Installer
        Assert ((Get-UserPathCount $dir) -eq 1) "PATH entries after re-run: $(Get-UserPathCount $dir)"
        Assert ((Get-FileHash -Algorithm SHA256 (Join-Path $dir 'skigard.exe')).Hash -eq $before) 'binary changed on re-run'
        $r = Invoke-Native (Join-Path $dir 'skigard.exe') @('version')
        Assert ($r.Code -eq 0) "exit $($r.Code)"
    }
    Check 'info: Authenticode status (the binary is not signed yet)' {
        $s = Get-AuthenticodeSignature (Join-Path $dir 'skigard.exe')
        Write-Host "Authenticode: $($s.Status)"
    }
}

function Test-Scoop {
    Check 'install Scoop (no admin switch; the runner account is an administrator, so -RunAsAdmin is passed)' {
        Invoke-Expression "& { $(Invoke-RestMethod https://get.scoop.sh) } -RunAsAdmin"
        $env:Path = "$env:USERPROFILE\scoop\shims;$env:Path"
        $r = Invoke-Native 'scoop' @('--version')
        Assert ($r.Code -eq 0) "scoop exit $($r.Code)"
    }
    Check 'scoop bucket add ambierhq' {
        $r = Invoke-Native 'scoop' @('bucket', 'add', 'ambierhq', 'https://github.com/ambierhq/scoop-bucket')
        Assert ($r.Code -eq 0) "exit $($r.Code)"
    }
    Check "scoop install skigard reports $Latest" {
        $r = Invoke-Native 'scoop' @('install', 'skigard')
        Assert ($r.Code -eq 0) "exit $($r.Code)"
        $v = Invoke-Native 'skigard' @('version')
        Assert ($v.Code -eq 0) "skigard exit $($v.Code)"
        Assert-VersionOutput $v.Out $Latest
        $w = Invoke-Native 'where.exe' @('skigard')
        Assert ($w.Out -like '*scoop*shims*') "skigard does not resolve to a Scoop shim: $($w.Out)"
    }
    Check 'scoop uninstall skigard' {
        $r = Invoke-Native 'scoop' @('uninstall', 'skigard')
        Assert ($r.Code -eq 0) "exit $($r.Code)"
        Assert (-not (Get-Command skigard -ErrorAction SilentlyContinue)) 'skigard still resolves after uninstall'
    }
}

function Install-Cosign([string]$Dir) {
    New-Item -ItemType Directory -Force -Path $Dir | Out-Null
    $exe = Join-Path $Dir 'cosign.exe'
    Get-Download "https://github.com/sigstore/cosign/releases/download/$CosignVersion/cosign-windows-amd64.exe" $exe
    $got = Get-Sha256 $exe
    Assert ($got -eq $CosignSha256) "cosign sha256 mismatch: want $CosignSha256, got $got"
    $exe
}

function Test-Zip {
    $arch = Get-Arch
    $dir = Join-Path $env:RUNNER_TEMP "zip-$Version"
    if (-not $env:RUNNER_TEMP) { $dir = Join-Path ([IO.Path]::GetTempPath()) "zip-$Version" }
    $zipName = "skigard_${Version}_windows_$arch.zip"
    $base = "https://github.com/$Repo/releases/download/v$Version"

    Check 'download the zip and verify its sha256 against checksums.txt' { Get-ReleaseZip $dir | Out-Null }
    Check 'the zip contains skigard.exe that reports the version' {
        $r = Invoke-Native (Join-Path $dir 'x\skigard.exe') @('version')
        Assert ($r.Code -eq 0) "exit $($r.Code)"
        Assert-VersionOutput $r.Out $Version
    }
    $script:cosign = $null
    Check "install cosign $CosignVersion (pinned sha256)" { $script:cosign = Install-Cosign (Join-Path $dir 'bin') }
    $identity = "^https://github.com/$Repo/\.github/workflows/release\.yml@refs/tags/v"
    $issuer = 'https://token.actions.githubusercontent.com'
    Check 'cosign verify-blob checksums.txt' {
        Assert ($null -ne $script:cosign) 'cosign not installed'
        Get-Download "$base/checksums.txt.sigstore.json" (Join-Path $dir 'checksums.txt.sigstore.json')
        $r = Invoke-Native $script:cosign @('verify-blob', '--bundle', (Join-Path $dir 'checksums.txt.sigstore.json'), '--certificate-identity-regexp', $identity, '--certificate-oidc-issuer', $issuer, (Join-Path $dir 'checksums.txt'))
        Assert ($r.Code -eq 0) "exit $($r.Code)"
    }
    Check 'cosign verify-blob the zip itself' {
        Assert ($null -ne $script:cosign) 'cosign not installed'
        Get-Download "$base/$zipName.sigstore.json" (Join-Path $dir "$zipName.sigstore.json")
        $r = Invoke-Native $script:cosign @('verify-blob', '--bundle', (Join-Path $dir "$zipName.sigstore.json"), '--certificate-identity-regexp', $identity, '--certificate-oidc-issuer', $issuer, (Join-Path $dir $zipName))
        Assert ($r.Code -eq 0) "exit $($r.Code)"
    }
    Check 'gh attestation verify the zip' {
        if (-not $env:GH_TOKEN -and -not $env:GITHUB_TOKEN) { Skip 'no GH_TOKEN' }
        $r = Invoke-Native 'gh' @('attestation', 'verify', (Join-Path $dir $zipName), '--repo', $Repo)
        Assert ($r.Code -eq 0) "exit $($r.Code)"
    }
}

function Test-Quickstart {
    $dir = Join-Path ([IO.Path]::GetTempPath()) "qs-$Version"
    Check 'fetch and verify the release zip' { Get-ReleaseZip $dir | Out-Null }
    $exe = Join-Path $dir 'x\skigard.exe'
    $env:SKIGARD_SMOKE_BIN = $exe
    $env:SKIGARD_SMOKE_VERSION = $Version
    $env:SKIGARD_SMOKE_SKILL = (Resolve-Path 'examples\hello-skill').Path
    if ($env:GITHUB_TOKEN) { $env:SKIGARD_SMOKE_REMOTE = '1' }
    Push-Location (Resolve-Path .)
    try {
        Check 'quick start in a real console (go test -tags e2e -run TestPublishedSmoke)' {
            Assert (Test-Path $exe) "no $exe"
            $r = Invoke-Native 'go' @('test', '-tags', 'e2e', '-count=1', '-v', '-run', 'TestPublishedSmoke', './e2e/windows')
            Assert ($r.Code -eq 0) "go test exit $($r.Code)"
            # Skips are not passes: the console sub-tests must have run.
            $must = @('conpty:_wrong_answer_cancels', 'approve_is_refused_under_an_agent-named_ancestor', 'approve_is_refused_when_CLAUDECODE=1', 'verify_--strict', 'install_into_a_temp_skills_dir')
            if ($env:SKIGARD_SMOKE_REMOTE -eq '1') { $must += 'remote_add_has_the_same_content_hash' }
            foreach ($t in $must) {
                Assert ($r.Out.Contains("--- PASS: TestPublishedSmoke/$t")) "sub-test '$t' did not pass or did not run"
            }
        }
    }
    finally { Pop-Location }
}

function Test-Npm {
    Check "npx -y skigard@$Version version" {
        $found = Invoke-Native 'npm' @('view', "skigard@$Version", 'version')
        if ($found.Code -ne 0 -or -not $found.Out.Trim()) { Skip "not on npm (npm view skigard@$Version found nothing)" }
        $r = Invoke-Native 'npx' @('-y', "skigard@$Version", 'version')
        Assert ($r.Code -eq 0) "exit $($r.Code)"
        Assert-VersionOutput $r.Out $Version
    }
}

switch ($Task) {
    'installer' { Test-Installer }
    'scoop' { Test-Scoop }
    'zip' { Test-Zip }
    'quickstart' { Test-Quickstart }
    'npm' { Test-Npm }
}

$failed = @($script:results | Where-Object { $_.Status -eq 'FAIL' })
if ($env:GITHUB_STEP_SUMMARY) {
    $arch = Get-Arch
    $rows = @("### Windows smoke: $Task, skigard $Version ($arch, PowerShell $($PSVersionTable.PSVersion))", '', '| check | result | detail |', '| --- | --- | --- |')
    foreach ($r in $script:results) { $rows += "| $($r.Name -replace '\|', '/') | $($r.Status) | $(($r.Detail -replace '\|', '/')) |" }
    Add-Content -LiteralPath $env:GITHUB_STEP_SUMMARY -Value ($rows -join "`n")
}
Write-Host ''
Write-Host "$($script:results.Count) checks, $($failed.Count) failed"
if ($failed.Count -gt 0) { exit 1 }
