# WORKORDER — Full Verification Audit (bxw M0–M7)

**You (the executing agent) are being audited, not trusted.** Another agent claims M0–M7 are
"complete and verified." This work order independently re-checks that claim on the real
Windows 11 machine. Your job is to **run each step and paste its raw, verbatim output** — the
reviewer decides PASS/FAIL, not you.

## Rules (read first, non-negotiable)
1. Run in `C:\Users\arnav\Downloads\bxw`. Use PowerShell. Windows 11.
2. For **every** step: paste the **exact command** and its **complete output**. Do not
   summarize, truncate, or say "PASS/looks good." Paste the evidence.
3. **Do not edit any source file** to make a step pass. If something fails to build or run,
   paste the failure and move on. Fixing is a separate task.
4. **Never damage the user's real browser profile.** The real-profile steps (§6) back up
   first and revert; if a step would touch the live profile unsafely, skip it and say so.
5. If a command/flag named here does not exist, paste the actual `--help` output instead of
   inventing one. Discrepancies between claims and reality are exactly what we're hunting.
6. Label each section exactly as below so the reviewer can map output to checks.

---

## §0 — Environment & integrity
```
go version
git -C . log --oneline
git -C . status --porcelain        # expect empty (clean tree)
git -C . clean -nxd                # what's untracked/ignored
go env GOOS GOARCH GOMODCACHE
type go.mod                         # (PowerShell: Get-Content go.mod)
```
Also list the real source surface (so claims about files are checkable):
```
Get-ChildItem -Recurse -Filter *.go | Where-Object { $_.FullName -notmatch '\\testdata\\' } | ForEach-Object { "{0,6}  {1}" -f (Get-Content $_.FullName | Measure-Object -Line).Lines, $_.FullName }
```

## §1 — Build, vet, tests (NO shortcuts, no cache)
```
go build ./...
go vet ./...
$env:GOOS="linux";  go build ./...; $env:GOOS=""        # cross-compile stub
$env:GOOS="windows"; go build -o bxw.exe .; $env:GOOS=""
go test ./... -count=1 -v
```
Adversarial — prove tests aren't hidden/skipped:
```
go test ./... -list ".*"                                # every test that EXISTS
go test ./... -count=1 -v 2>&1 | Select-String -Pattern "SKIP|FAIL|ok |--- "
Select-String -Path (Get-ChildItem -Recurse -Filter *_test.go).FullName -Pattern "t.Skip|//go:build|testing.Short" 
```
> Reviewer checks: tree clean; full test run (not cached) passes; the live E2E test actually
> ran (not skipped); no `t.Skip`/`Short` gating the important tests.

## §2 — Discovery (M0/M1)
```
.\bxw.exe list browsers
.\bxw.exe list profiles edge
.\bxw.exe list profiles brave
.\bxw.exe doctor
```

## §3 — Daemon, attach, tabs (M2/M3)
```
.\bxw.exe launch edge about:blank --headless
.\bxw.exe list tabs edge
Get-ChildItem "$env:LOCALAPPDATA\bxw" -Recurse | Select-Object FullName,Length   # socket/lock/log
.\bxw.exe stop edge
Test-Path "$env:LOCALAPPDATA\bxw\edge.sock"                                        # expect False after stop
```
Singleton proof (run quickly back-to-back, then count daemons):
```
.\bxw.exe launch edge about:blank --headless
Start-Sleep 1
Get-CimInstance Win32_Process -Filter "Name='bxw.exe'" | Select-Object ProcessId,CommandLine
.\bxw.exe stop edge
```
Cleanup any agent browser left behind (must match ONLY the agent dir, never real Edge):
```
Get-CimInstance Win32_Process -Filter "Name='msedge.exe'" | Where-Object { $_.CommandLine -like '*\bxw\*agent*' } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }
```

## §4 — Snapshot + actions (M4), live headless against a local fixture
Create a throwaway form page, then drive it end to end:
```
$f = Join-Path $env:TEMP "bxw-audit-form.html"
@'
<!doctype html><title>audit</title>
<input id="n" aria-label="Name"><button id="go" onclick="document.getElementById('r').textContent='Submitted: '+document.getElementById('n').value">Submit</button>
<div id="r"></div>
'@ | Set-Content -Encoding utf8 $f
.\bxw.exe launch edge ("file:///" + ($f -replace '\\','/')) --headless
$tab = (.\bxw.exe list tabs edge)   # note the tab id from this output
```
Then (substitute the real `<tab>` and the ref numbers the snapshot prints):
```
.\bxw.exe snapshot <tab>
.\bxw.exe fill <tab> <ref-of-Name-textbox> "Alice"
.\bxw.exe click <tab> <ref-of-Submit-button>
.\bxw.exe eval <tab> "document.getElementById('r').textContent"   # expect: Submitted: Alice
.\bxw.exe key <tab> "Tab"
.\bxw.exe stop edge
```
> Reviewer checks: snapshot shows the textbox + button with refs and states; fill+click
> actually mutate the DOM; `eval` returns `Submitted: Alice` (proves the action chain is real).

## §5 — Profile correlation (M5)
```
.\bxw.exe launch edge about:blank --headless
.\bxw.exe open https://example.com/
.\bxw.exe list tabs edge
.\bxw.exe list tabs edge --profile "Profile 1"
```
Prove the probe tab was cleaned up (NO `bxwprobe-`/`data:text/html` tab remains):
```
(.\bxw.exe list tabs edge) -match "bxwprobe|data:text/html"    # expect no match
.\bxw.exe stop edge
```

## §6 — Real-profile attach / setup / doctor / MCP (M6) — the risky milestone
This is the one the other agent could NOT fully self-verify (needs a human for the Allow
dialog). Do the safe, automated parts; mark the interactive parts clearly.

### 6a — prefs.go pref-flip, SAFELY (back up first, verify, revert)
```
# Ensure Edge is fully closed first (manually), then:
$ls = "$env:LOCALAPPDATA\Microsoft\Edge\User Data\Local State"
Copy-Item $ls "$env:TEMP\bxw-audit-localstate.before.json" -Force
# capture a known large 64-bit-ish value BEFORE, to catch truncation:
(Get-Content $ls -Raw)  | Out-File "$env:TEMP\ls_before.txt"
.\bxw.exe setup edge
Get-ChildItem "$env:LOCALAPPDATA\Microsoft\Edge\User Data\*.bxw.bak"   # backup created?
# confirm ONLY the pref changed and nothing truncated:
.\bxw.exe setup edge --revert
Copy-Item $ls "$env:TEMP\bxw-audit-localstate.after.json" -Force
# semantic diff before vs after-revert (should be equivalent):
python -c "import json,sys; a=json.load(open(r'$env:TEMP\bxw-audit-localstate.before.json',encoding='utf-8')); b=json.load(open(r'$env:TEMP\bxw-audit-localstate.after.json',encoding='utf-8')); print('EQUAL' if a==b else 'DIFFERS'); import itertools"
```
> Reviewer checks: a `.bxw.bak` backup exists; `setup` set `devtools.remote_debugging.user-enabled`
> true; `--revert` restored it; before vs after-revert are semantically EQUAL (no dropped keys,
> no truncated 64-bit timestamps). Paste the pref value at each stage:
```
# show just the relevant key before/after (PowerShell):
(Get-Content "$env:TEMP\ls_before.txt" -Raw) -match '"remote_debugging"[^}]*'
```

### 6b — process detection (unelevated)
```
# with Edge running normally, confirm detection finds it WITHOUT admin:
Start-Process msedge ; Start-Sleep 2
.\bxw.exe doctor        # should report Edge running + its profile
```

### 6c — MCP server over stdio (prove the 9 tools actually respond)
Pipe a JSON-RPC initialize + tools/list into `bxw mcp` and paste the RAW JSON replies:
```
$init  = '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"audit","version":"0"}}}'
$inited= '{"jsonrpc":"2.0","method":"notifications/initialized"}'
$tools = '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
($init,$inited,$tools) -join "`n" | .\bxw.exe mcp
```
> Reviewer checks: a valid JSON-RPC `initialize` result, then a `tools/list` result listing
> the claimed tools (`list_browsers,list_tabs,open,snapshot,click,fill,type,key,eval`). Count them.

### 6d — MCP installer merges (does not clobber)
```
.\bxw.exe mcp install --help          # paste actual flags
# If a dry-run exists use it; else test against a TEMP config with a pre-existing server entry
# and confirm bxw is ADDED, the existing server PRESERVED. Paste the before/after config.
```

### 6e — V1 / V2 (INTERACTIVE — needs the human; do not fake)
State clearly that these require a person:
- **V1:** after `bxw setup edge` with Edge closed, launch + attach and report whether the
  Chromium "Allow remote debugging?" dialog appears, and whether a second bxw command prompts
  again (should not — daemon holds the connection).
- **V2:** with the pref enabled (no `--remote-debugging-port` flag), report whether a CDP port
  is discoverable (`/json/version` reachable) and on which port, or whether a fixed flag is
  still required. Paste whatever you can observe; mark the rest "REQUIRES HUMAN."

## §7 — Packaging (M7)
```
node npm/bxw/bin/bxw.js doctor                 # launcher resolves the .exe and runs the binary
Get-Content npm/bxw/package.json               # os/cpu/bin/optionalDependencies
Get-Content npm/bxw/bin/bxw.js                 # confirm it resolves "bx.exe" on win32
Get-ChildItem npm -Recurse -Filter package.json | Get-Content
Select-String -Path .github/workflows/*.yml -Pattern "windows-latest|ubuntu-latest|go test|npm publish"
Get-ChildItem SECURITY.md,README.md,CONTRIBUTING.md | Select-Object Name,Length
```
> Reviewer checks: launcher actually runs (not just exists); `bxw.js` handles the `.exe` name;
> subpackages declare `os:["win32"]` + correct `cpu`; CI has a `windows-latest` job; docs exist.

## §8 — Final integrity
```
git -C . status --porcelain          # STILL clean? (you must not have edited source)
go build -o bxw.exe . ; (Get-FileHash bxw.exe).Hash     # binary builds from current source
```

---

## How to return results
Paste back, in order, a labeled block per section (§0…§8) containing the **raw command output**.
For any step you could not run, write `SKIPPED: <reason>`. Do not add a verdict — the reviewer
reads the evidence and decides. Do not modify source anywhere in this audit.
