param(
    [string]$Version = "0.1.0"
)

$ErrorActionPreference = "Stop"
Write-Host "Building bxw release $Version..."

$targets = @(
    @{ GoArch = "amd64"; NpmArch = "x64" },
    @{ GoArch = "arm64"; NpmArch = "arm64" }
)

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
    Copy-Item "LICENSE" "$pkgDir/" -Force
}

$env:GOOS = ""
$env:GOARCH = ""
$env:CGO_ENABLED = ""

Write-Host "Build completed successfully for $Version."

