<#
.SYNOPSIS
  Fetch the RENDERED text of a JavaScript page via bxw (headless Chromium).
  Plain HTTP fetches (curl / WebFetch) return only the shell of SPA pages like
  cloud.google.com/*/pricing or aws.amazon.com/*/pricing — the real numbers are
  injected by JS. bxw loads the page in a real browser, so the rendered text is
  available. This is the systemic fix for the "pricing page JS-blocked, had to
  copy-paste manually" problem.

.EXAMPLE
  pwsh scripts/fetch.ps1 -Url https://cloud.google.com/bigquery/pricing
  pwsh scripts/fetch.ps1 -Url https://aws.amazon.com/ebs/pricing/ -Browser edge -SettleMs 5000
#>
param(
  [Parameter(Mandatory)][string]$Url,
  [string]$Browser = 'edge',
  [int]$SettleMs = 4000   # extra wait after load for lazy JS content
)
$ErrorActionPreference = 'Stop'
$bxw = Join-Path $PSScriptRoot '..\bxw.exe'

try {
  & $bxw launch $Browser $Url --headless | Out-Null
  Start-Sleep -Milliseconds $SettleMs

  # Pick the tab whose URL matches (fall back to the first page tab).
  $rows = & $bxw list tabs $Browser
  $idLine = $rows | Select-String -Pattern '^[0-9A-Fa-f]{8}\s'
  if (-not $idLine) { throw "no tab found for $Url" }
  $match = $idLine | Where-Object { $_.Line -like "*$([Uri]$Url).Host*" } | Select-Object -First 1
  if (-not $match) { $match = $idLine | Select-Object -First 1 }
  $tab = ($match.Line -split '\s+')[0]

  # Rendered, readable text of the page.
  & $bxw eval $tab "document.body.innerText" --browser $Browser
}
finally {
  & $bxw stop $Browser 2>&1 | Out-Null
  # Terminate the isolated agent browser this fetch launched (never the real one).
  $dir = "*\bxw\$Browser-agent*"
  $procName = if ($Browser -eq 'brave') { 'brave' } else { 'msedge' }
  Get-CimInstance Win32_Process -Filter "Name='$procName.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.CommandLine -like $dir } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
}
