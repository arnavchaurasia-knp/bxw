param(
    [string]$Version = "0.1.0"
)

$ErrorActionPreference = "Stop"
Write-Host "Building bxw release $Version..."

$targets = @(
    @{ GoArch = "amd64"; NpmArch = "x64" },
    @{ GoArch = "arm64"; NpmArch = "arm64" }
)

# Stamp the "version" field of a package.json in place, preserving file formatting
# (regex on the first top-level "version" key only — no JSON round-trip reordering).
function Set-PkgVersion {
    param([string]$Path, [string]$Ver)
    if (!(Test-Path $Path)) { throw "package.json not found: $Path" }
    $text = Get-Content -Raw -Path $Path
    $new = [regex]::Replace($text, '("version"\s*:\s*")[^"]*(")', "`${1}$Ver`${2}", 1)
    # Pin optionalDependencies on bxw subpackages to the same version (launcher only).
    $new = [regex]::Replace($new, '("bxw-win32-(?:x64|arm64)"\s*:\s*")[^"]*(")', "`${1}$Ver`${2}")
    # Write UTF-8 WITHOUT BOM. Windows PowerShell 5.1's `Set-Content -Encoding utf8`
    # emits a BOM, which makes the file invalid JSON (0xEF BOM parsed as 'ï').
    [System.IO.File]::WriteAllText((Resolve-Path $Path), $new, (New-Object System.Text.UTF8Encoding($false)))
    Write-Host "Stamped $Path -> $Ver"
}

foreach ($t in $targets) {
    $pkgDir = "npm/bxw-win32-$($t.NpmArch)"
    $binDir = "$pkgDir/bin"
    if (!(Test-Path $binDir)) {
        New-Item -ItemType Directory -Path $binDir -Force | Out-Null
    }

    Write-Host "Building Windows $($t.GoArch) -> $binDir/bxw.exe..."
    $env:GOOS = "windows"
    $env:GOARCH = $t.GoArch
    $env:CGO_ENABLED = "0"

    go build -trimpath -ldflags "-s -w -X bxw/internal/app.Version=$Version" -o "$binDir/bxw.exe" main.go
    if ($LASTEXITCODE -ne 0) { throw "go build failed for $($t.GoArch)" }
    Copy-Item "LICENSE" "$pkgDir/" -Force

    Set-PkgVersion -Path "$pkgDir/package.json" -Ver $Version
}

# Stamp the launcher package (its own version + the optionalDependencies pins).
Set-PkgVersion -Path "npm/bxw/package.json" -Ver $Version

$env:GOOS = ""
$env:GOARCH = ""
$env:CGO_ENABLED = ""

Write-Host "Build completed successfully for $Version."
