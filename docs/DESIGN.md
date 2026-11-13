# bxw — Windows-native agentic browser control (HLD + LLD)

> **What this is.** A standalone, open-sourceable Windows-first CLI + MCP server that lets
> coding agents (Claude Code, Codex, Cursor, AGY) drive the Chromium browser the user
> *already has open*, in the *profile they name*, over the Chrome DevTools Protocol (CDP).
> It is a clean-room **Windows** sibling of `@facets-cloud/bx` (macOS-only today),
> architected so its OS-agnostic core stays **contract-compatible with bx** and the whole
> thing can later be folded upstream as `*_windows.go` files behind Go build tags.
>
> **Status of this document.** Every architectural decision below is anchored to a fact
> *verified by live probing of a real Windows 11 machine* (see §2 and Appendix A). This is a
> feasibility-proven design, not a sketch. It doubles as the implementation work order for
> downstream agents (§15).

---

## 1. Goals, non-goals, principles

### 1.1 Goals
- **Attach, don't sandbox.** Drive the user's real, already-authenticated browser profile
  (Google, GitHub, AWS Console, Jira) — not a blank Puppeteer sandbox.
- **Deterministic element refs.** Token-efficient accessibility snapshots with stable
  ephemeral refs (`e1`, `e2`…) that actions target directly.
- **One "Allow", then silent.** A persistent daemon holds a single CDP connection; the user
  authorizes once per browser session, never per tool call.
- **Single binary, zero runtime deps.** One Go `.exe`, no Node/Python required at runtime.
- **Self-sustained & scalable.** CI tests on `windows-latest`, releases cross-compiled and
  auto-published to npm + winget/scoop; an extensibility model so new tools/features drop in
  without touching the core.
- **Open-source ready.** MIT, SECURITY.md, docs, reproducible builds, no telemetry.
- **Mergeable upstream.** OS-agnostic packages mirror bx's layout and JSON contracts so a
  future merge into bx is a diff, not a rewrite.

### 1.2 Non-goals
- Not a cross-browser WebDriver replacement; CDP/Chromium only (Edge, Brave, Chrome, Vivaldi,
  Chromium). Firefox/Safari out of scope.
- Not a headless scraper framework; the point is the *human's live browser*.
- No cloud, no account, no data egress.

### 1.3 Principles (industry practice)
- **Hexagonal/ports-and-adapters**: a pure core (snapshot, refs, tools, CDP orchestration)
  behind an **OS-abstraction port** with a Windows adapter. Keeps the core testable and the
  merge-to-bx clean.
- **Fail loud, structured.** Every error carries an exit class + actionable message
  (mirrors bx's `internal/exit`).
- **Deterministic over clever.** Static mappings and probes over heuristics.
- **Everything verifiable.** `bxw doctor` reproduces the §2 checks on any machine.

---

## 2. Verified feasibility anchors (probed on Windows 11 26300, 2026-11-13)

These are the load-bearing facts the design rests on. Each was executed live; raw commands
and output are in Appendix A. **Nothing below is assumed.**

| # | Anchor | Verified result | Design consequence |
|---|--------|-----------------|--------------------|
| A1 | Go toolchain | `go1.26.6 windows/amd64`; `golang.org/x/sys@v0.42.0` already in module cache | Native Windows build; `x/sys/windows` (OpenProcess/LockFileEx) available, no new dep |
| A2 | Node/npm/git | `node v24.14.1`, `npm 11.11.0`, `git 2.49.0` | npm platform-dispatch packaging viable; JS launcher shim works |
| A3 | Edge install | `157.0.4341.42` at `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe` | **Edge lives under `Program Files (x86)` even though 64-bit** — discovery must scan both PF and PF(x86) |
| A4 | Brave install | `157.1.99.57` at `C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe` | Per-browser path table (see §7) |
| A5 | Registry discovery | `HKLM\SOFTWARE\Clients\StartMenuInternet` lists Brave/Edge; `App Paths\{msedge,brave}.exe` give exact exe paths | Registry is the authoritative discovery source; FS paths are the fallback |
| A6 | Profile cache | `Local State` → `profile.info_cache`: Edge `Default`→"Profile 1"; Brave `Default`→"Work" | Folder→human-name mapping identical to macOS; profile reader is portable |
| A7 | Debug pref key | `devtools.remote_debugging.user-enabled` exists in `Local State` (currently unset) | The "enable debugging without CLI flags" pref-flip path is valid on Windows |
| A8 | **CDP live** | Launched Edge w/ `--remote-debugging-port`; `GET /json/version` returned `ws://127.0.0.1:9333/devtools/browser/<uuid>`, `/json/list` enumerated a page target; `File-Path-Style: windows` | **The entire mechanism works on Windows.** chromedp can attach, enumerate targets, open WS |
| A9 | **DevToolsActivePort gotcha** | In `--headless=new`, the `DevToolsActivePort` file was **NOT** written to the user-data-dir | Port discovery must **probe `/json/version`**, not rely on the port file alone (§8) |
| A10 | AF_UNIX IPC | Go `net.Listen("unix", …)` + `Dial` round-trip **OK** at `…\AppData\Local\bxw.sock` (len 44 < 108) | Daemon IPC reuses bx's unix-socket model verbatim — **no named-pipe rewrite** |
| A11 | **SingletonLock gotcha** | `SingletonLock`/`SingletonSocket` symlinks **absent** on Windows; Edge uses a plain `lockfile`; live `Win32_Process` cmdline captured: `msedge.exe --profile-directory=Default …` | bx's symlink-based "is it running + PID" is **non-portable**; detect via process enumeration + CommandLine (§8) |

**Two interactive validations deferred to implementation** (they require closing the user's
live browser, which is disruptive to automate here):
- **V1 — Allow-dialog on the attach path:** close browser → set `user-enabled=true` →
  relaunch → connect → confirm Chromium shows the `chrome://inspect` per-connection Allow
  dialog once, and the daemon's held connection suppresses further prompts.
- **V2 — fixed debug port for a pref-enabled real profile:** confirm which port a
  pref-enabled (non-flag) browser exposes and how to discover it (expected: process cmdline
  / `/json/version` sweep, per A9/A11).

---

## 3. Technology decisions (with rationale)

| Decision | Choice | Why (anchored) |
|----------|--------|----------------|
| Language | **Go** | bx is Go (A1 confirms it builds here); single static binary, trivial cross-compile, and **merge-to-bx becomes `_windows.go` drop-ins** rather than a rewrite |
| CDP library | **chromedp + cdproto** (same versions as bx: `chromedp v0.23.0`, `cdproto v0.159.0`) | A8 proves it attaches on Windows; reusing bx's exact stack keeps contracts identical and avoids reinventing WS/AX plumbing. (The earlier "raw-WebSocket, zero-dep" idea is rejected — bx already solved this.) |
| Windows syscalls | **`golang.org/x/sys/windows`** | A1: already cached; idiomatic for OpenProcess/LockFileEx/process-group |
| IPC | **AF_UNIX socket** under `%LOCALAPPDATA%` | A10: works natively; identical to bx so the daemon package stays shared |
| MCP | **`modelcontextprotocol` Go SDK over stdio** | Matches bx (`bx mcp` speaks MCP on stdio); one transport for CLI + MCP |
| Packaging | **Go binary + npm platform-dispatch + winget/scoop** | A2 enables npm; Windows users expect winget/scoop natively |
| Distribution subpkgs | `bxw-win32-x64`, `bxw-win32-arm64` | Mirrors bx's `bx-darwin-*` optionalDependencies pattern |

---

## 4. HLD — system architecture

```
                 ┌──────────────────────────────────────────────────────────┐
   Agent         │                        bxw (one .exe)                      │
 (Claude/AGY)    │                                                            │
      │ stdio    │  ┌────────────┐     ┌──────────────┐     ┌──────────────┐  │
      ├─ MCP ───►│  │  CLI /     │     │  Client       │     │  Daemon       │ │
      │          │  │  MCP server├────►│  (dials sock, │ UDS │  (singleton,  │ │
   Terminal      │  │ (tools)    │ RPC │  spawns daemon├────►│  holds 1 CDP  │ │
   (bxw click)   │  └────────────┘     │  if absent)   │     │  conn)        │ │
      └─ CLI ───►│         ▲           └──────────────┘     └──────┬───────┘  │
                 │         │                                        │ CDP/WS   │
                 │   ┌─────┴───────────────── core (OS-agnostic) ───┼───────┐  │
                 │   │ snapshot/refs │ tools │ session │ keys │ devlog│      │  │
                 │   └───────────────────────────────────────────────┘      │  │
                 │   ┌──────────────────── OS port (interface) ───────────┐  │  │
                 │   │ discover │ profiles │ proc │ filelock │ paths │ pref │  │  │
                 │   └──────────────── windows adapter (_windows.go) ─────┘  │  │
                 └──────────────────────────────────────────────────────────┘  │
                                                                     ▼          ▼
                                                        ┌────────────────────────────┐
                                                        │  User's real Edge / Brave   │
                                                        │  (profile "Work", authed)   │
                                                        └────────────────────────────┘
```

**Two runtime roles in one binary** (exactly like bx):
1. **Foreground command** (`bxw <verb>` or `bxw mcp`): parses args, dials the daemon socket,
   auto-starts the daemon if absent, issues a JSON-RPC op, prints/relays the result.
2. **Daemon** (`bxw __daemon --browser <name>`): detached background process; opens **one**
   CDP connection; keeps focus-emulation on, listens for JS dialogs, owns the ref table and
   profile↔context map; serves ops over the unix socket.

### 4.1 Core data flow — a single `click` tool call
```
agent --(MCP: click {ref:e12})--> CLI --(UDS JSON-RPC {Op:"click",Args})--> daemon
 daemon: refs["e12"] -> backendNodeId -> dom.ResolveNode
         -> runtime.CallFunctionOn(setAttribute data-bxw-ref)
         -> CDP Input dispatch on [data-bxw-ref="e12"]
 daemon --(Response{Data|Error})--> CLI --(MCP result / stdout)--> agent
```

### 4.2 Attach flow (the differentiated part)
```
bxw use brave --profile Work
  1. discover(): registry App Paths + FS scan  -> exe, userDataDir   (A3/A4/A5)
  2. profiles(): read Local State.info_cache    -> {Default:"Work"}   (A6)
  3. ensureDebuggable():
        if browser running without debugging -> instruct user, OR
        setPref(user-enabled=true) while closed -> relaunch           (A7, V1)
  4. ensureDaemon(): spawn detached daemon, one CDP connect (one Allow) (V1)
  5. bindProfile(): nonce-probe correlation of BrowserContextID->folder (§9)
```

---

## 5. LLD — package layout

```
bxw/
├── main.go                      # dispatch: __daemon? / mcp? / verb
├── go.mod                       # module github.com/<org>/bxw
├── internal/
│   ├── app/                     # [OS-agnostic] CLI + MCP surface
│   │   ├── app.go  args.go  cmds.go  act.go  choose.go  output.go
│   │   ├── mcp.go  mcpinstall.go          # stdio MCP + client auto-config
│   │   ├── setup.go  devtools.go          # bxw setup / doctor
│   │   └── release.go version.go
│   ├── daemon/                  # [OS-agnostic] singleton + CDP + refs
│   │   ├── server.go client.go            # UDS JSON-RPC (client.go has filelock -> OS port)
│   │   ├── ops.go actions.go session.go
│   │   ├── snapshot.go          # AX tree -> refs (data-bxw-ref)
│   │   ├── keys.go inspect.go devlog.go
│   ├── browser/                 # [SPLIT] discovery/launch/running-detection
│   │   ├── browser.go                      # table + selection (interface-driven)
│   │   ├── browser_windows.go              # NEW: PF/PF(x86) paths, registry
│   │   ├── agent.go                        # agent-mode launch (isolated)
│   │   ├── agent_windows.go                # NEW: CREATE_NEW_PROCESS_GROUP, no Setsid
│   │   └── proc_windows.go                 # NEW: OpenProcess/Win32_Process detection
│   ├── osport/                  # NEW: the port interface + windows adapter
│   │   ├── osport.go                       # interface Platform{...}
│   │   ├── platform_windows.go             # LockFileEx, paths, spawn, pref file loc
│   │   └── platform_stub.go  // //go:build !windows  (keeps repo cross-compilable)
│   └── exit/exit.go             # [OS-agnostic] exit classes
├── npm/
│   ├── bxw/package.json         # launcher, os:["win32"], optionalDeps -> subpkgs
│   ├── bxw-win32-x64/package.json
│   └── bxw-win32-arm64/package.json
├── scripts/npm-build.sh         # GOOS=windows build -> bin/bx.exe into subpkgs
├── .github/workflows/
│   ├── ci.yml                   # windows-latest + ubuntu (vet/test/build)
│   └── release.yml              # tag -> cross-build -> gh release -> npm publish
├── docs/ (README, ARCHITECTURE, SECURITY, CONTRIBUTING, demo.gif)
└── LICENSE (MIT), THIRD_PARTY_NOTICES
```

### 5.1 The OS-abstraction port (the key to merge-ability)
```go
// internal/osport/osport.go  — OS-agnostic core depends ONLY on this.
type Platform interface {
    // discovery
    BrowserTable() []BrowserDef                 // exe/dataDir per browser, OS-specific
    FromRegistry(name string) (exe string, ok bool)
    // process
    ProcessAlive(pid int) bool
    Terminate(pid int) error                    // graceful: WM_CLOSE / taskkill, then kill
    RunningBrowser(def BrowserDef) (pid int, userDataDir, profileDir string, ok bool)
    SpawnDetached(exe string, args []string) (pid int, err error)
    // file lock (daemon singleton)
    LockExclusive(f *os.File) (unlock func() error, err error)
    // paths
    LocalAppData() string
    SocketPath(browser string) string           // %LOCALAPPDATA%\bxw\<browser>.sock
    LocalStatePath(dataDir string) string        // <dataDir>\Local State
}
```
Windows adapter implements each using the **verified** mechanisms:
- `BrowserTable()` → §7 table (A3/A4).
- `FromRegistry()` → `App Paths` + `StartMenuInternet` (A5).
- `RunningBrowser()` → `Win32_Process` / `CreateToolhelp32Snapshot` match on exe + parse
  `--user-data-dir`/`--profile-directory` from CommandLine (A11).
- `Terminate()` → `WM_CLOSE` to the main window else `TerminateProcess` (replaces
  `syscall.Kill`).
- `SpawnDetached()` → `SysProcAttr{CreationFlags: CREATE_NEW_PROCESS_GROUP|DETACHED_PROCESS}`
  (replaces `Setsid`).
- `LockExclusive()` → `LockFileEx` (replaces `syscall.Flock`).

### 5.2 Exact divergences vs bx (verified source audit of `Facets-cloud/bx`)
bx has **no build tags today**; these are the only Unix-bound spots. The Windows port
isolates each behind the port interface above.

| bx file | Unix-bound call | Windows replacement |
|---------|-----------------|---------------------|
| `internal/browser/agent.go` | `syscall.Kill(pid,0)`, `SysProcAttr{Setsid:true}`, `syscall.Kill(pid,SIGTERM)` | `ProcessAlive`, `CREATE_NEW_PROCESS_GROUP`, `Terminate` |
| `internal/daemon/client.go` | `syscall.Flock` ×2, `SysProcAttr{Setsid:true}` | `LockFileEx`, process-group flag |
| `internal/daemon/server.go` | `syscall.*` | route through port |
| `internal/browser/browser.go` | macOS `/Applications` + `Library/Application Support` table | §7 Windows table |
| `internal/app/setup.go` | `/Applications` check; fetches `bx_darwin_%s.tar.gz` self-update | `%ProgramFiles%` check; `bxw_windows_%s.zip` asset |
| `internal/app/mcpinstall.go` | `Library/Application Support/Claude/...` | `%APPDATA%\Claude\claude_desktop_config.json`, `%APPDATA%\Cursor\...` |
| `bin/bx.js` (npm) | resolves `bin/bx` | resolve `bin/bx.exe` on `win32` |

---

## 6. Data contracts (stable; shared with bx)

### 6.1 Daemon JSON-RPC (over UDS)
```jsonc
// request
{ "op": "click", "args": { "tab": "AAC357…", "ref": "e12" } }
// response
{ "data": { "ok": true }, "error": null }
```
Ops: `open, goto, click, hover, drag, fill, type, key, upload, dialog, close_tab,
list_tabs, snapshot, read, screenshot, eval, wait, console, network, storage, performance,
emulate, cdp`. Unknown op → `{"error":"unknown daemon op"}` (as in bx).

### 6.2 Snapshot / ref contract
- Source: `Accessibility.getFullAXTree`.
- Filter roles: drop `generic/none/InlineTextBox`.
- Emit: `[e12] button "Submit"` with `refs["e12"] = backendNodeId`.
- Resolve on action: `DOM.resolveNode` → `Runtime.callFunctionOn` tag
  `setAttribute("data-bxw-ref","e12")` → operate on `[data-bxw-ref="e12"]`.
- Lifetime: refs valid until next snapshot; daemon holds them in session state.

### 6.3 MCP tool schema (per tool)
`{ name, description, inputSchema(JSON Schema) }` grouped: **setup** (doctor, list_browsers,
use, launch), **read** (list_tabs, snapshot, read, screenshot, eval, wait), **act**
(open/goto/click/fill/type/key/dialog/close_tab), **debug**
(console/network/storage/performance/cdp).

---

## 7. Browser registry (verified Windows paths)

```go
// internal/browser/browser_windows.go  (cite A3/A4/A5; %VAR% expanded at runtime)
var winBrowsers = []BrowserDef{
  {Name:"edge",     Exe:[]string{`%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`,
                                 `%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`},
                    DataDir:`%LOCALAPPDATA%\Microsoft\Edge\User Data`},                 // A3 ✓
  {Name:"brave",    Exe:[]string{`%ProgramFiles%\BraveSoftware\Brave-Browser\Application\brave.exe`,
                                 `%ProgramFiles(x86)%\BraveSoftware\Brave-Browser\Application\brave.exe`},
                    DataDir:`%LOCALAPPDATA%\BraveSoftware\Brave-Browser\User Data`},     // A4 ✓
  {Name:"chrome",   Exe:[]string{`%ProgramFiles%\Google\Chrome\Application\chrome.exe`,
                                 `%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe`,
                                 `%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe`},
                    DataDir:`%LOCALAPPDATA%\Google\Chrome\User Data`},
  {Name:"vivaldi",  Exe:[]string{`%LOCALAPPDATA%\Vivaldi\Application\vivaldi.exe`},
                    DataDir:`%LOCALAPPDATA%\Vivaldi\User Data`},
  {Name:"chromium", Exe:[]string{`%LOCALAPPDATA%\Chromium\Application\chrome.exe`},
                    DataDir:`%LOCALAPPDATA%\Chromium\User Data`},
}
// Resolution order: registry App Paths (A5) -> first existing Exe -> error with install hint.
// NOTE: Arc has no stable Windows build -> omitted.
```

---

## 8. Port & running-browser discovery (robust, per A9/A11)

bx's two macOS assumptions **both break on Windows**; replace with:

1. **Is the browser running, and with what profile/data-dir?** → enumerate processes
   (`Win32_Process` / toolhelp), match exe basename, parse `--user-data-dir` and
   `--profile-directory` from CommandLine. (A11 — captured live.)
2. **What debug port?** → precedence:
   a. If bxw launched it (agent mode): we chose the port.
   b. Else read `<userDataDir>\DevToolsActivePort` **if present** (not guaranteed — A9).
   c. Else **sweep candidate ports** probing `GET http://127.0.0.1:<p>/json/version`
      (A8 proves this is the reliable signal) and match `Browser` string to the target.
3. **Enable debugging on a real profile without CLI flags** → set
   `devtools.remote_debugging.user-enabled=true` in `Local State` **while the browser is
   closed** (A7), then relaunch; first connection triggers one Allow dialog (V1).

---

## 9. Profile ↔ CDP context correlation (ported from bx, verified-compatible)

CDP can't open a tab in a named profile, and `Target.targetCreated` reports only an opaque
`BrowserContextID`. Correlate it to a human folder via a **nonce probe** (bx's technique;
contracts unchanged, so this code is shared with bx):
1. Launch URL into a profile via `--profile-directory=<folder>` (A6 confirms folder names).
2. Open an invisible `data:text/html,<title>bxw</title><!--bxwprobe-<nonce>-->`.
3. Catch the new target, record `profileOf[BrowserContextID] = folder`, close the probe.
4. Subsequent `open … --profile Work` targets that `BrowserContextID`.

---

## 10. Packaging & distribution (self-sustained)

- **Primary:** single `bxw.exe` (`GOOS=windows GOARCH=amd64|arm64`, `-trimpath -ldflags "-s -w"`).
- **npm:** `bin/bxw.js` launcher resolves `@org/bxw-win32-${arch}/bin/bx.exe`
  (the `.exe` fix); `optionalDependencies` dispatch (bx pattern, A2 confirms npm present).
- **Native Windows:** **winget** manifest + **scoop** bucket + optional **Chocolatey** —
  what Windows users actually expect; `bxw setup`/self-update fetches
  `bxw_windows_<arch>.zip` from GitHub Releases.
- **No telemetry.** Reproducible, checksummed releases (`shasum -a 256`).

---

## 11. CI/CD (GitHub Actions)

- **ci.yml** (PRs): matrix `{windows-latest, ubuntu-latest}` → `go vet`, `go test ./...`,
  `go build`. **windows-latest is mandatory** so build-tag code is actually compiled/tested
  (this is what makes it self-sustained). Integration job on windows-latest launches Edge
  headless and asserts A8 (`/json/version` reachable) as a smoke test.
- **release.yml** (tag `v*`): cross-build win amd64+arm64 → `gh release create` → run
  `npm-build.sh` → publish `bxw-win32-*` subpkgs, wait until served, then publish launcher
  (bx's publish-order guard) → push winget/scoop manifests.

---

## 12. Testing strategy
- **Unit:** discovery (registry+FS mocks), Local State parse (A6 fixtures), ref assignment,
  JSON-RPC codec, path/env expansion, CommandLine parsing (A11 fixtures).
- **Integration (windows-latest + local):** real Edge/Brave headless — attach, snapshot,
  click, type, read; assert the A8/A9 behaviors and the port-sweep fallback.
- **E2E:** `bxw use → open → snapshot → fill → click → read` against a static test page
  (TodoMVC fixture bundled), the same demo bx ships.
- **Doctor-as-test:** `bxw doctor` runs the §2 checks and exits non-zero on any failure.

---

## 13. Security model (open-source grade)
- Opening a debug port / flipping the pref **exposes the profile to any local program** —
  documented prominently (bx's own binary says as much). `SECURITY.md` states blast radius.
- Bind CDP strictly to `127.0.0.1`; never a public interface.
- `bxw launch` (isolated agent profile, `--user-data-dir=%LOCALAPPDATA%\bxw\<browser>-agent`)
  for automation that must **not** touch personal logins — the safe default for untrusted agents.
- Socket under `%LOCALAPPDATA%` (per-user ACL). No secrets logged; `devlog` redacts.
- Clear teardown: `bxw stop` closes the daemon + connection; `bxw setup --revert` unsets the pref.

---

## 14. Extensibility (so "many other features" stay easy)
- **Tool registry:** each tool is `{name, schema, handler}` registered in one place; adding a
  feature = add a handler + an op, no core edits. CLI verb + MCP tool generated from the same
  registration.
- **Capability plugins (roadmap):** out-of-process plugins invoked over the same JSON-RPC
  (e.g. `bxw-record` GIF capture, `bxw-har` network export, DOM-diff watchers).
- **Config:** `%APPDATA%\bxw\config.toml` (default browser, default profile, port range,
  agent-profile policy). `bxw config` reads/writes; env overrides for CI.
- **Structured events:** `devlog` JSONL channel for observability / future reviewer loops.

---

## 15. Implementation work order (hand to AGY / Claude)

> Build in this order; each milestone has a hard acceptance gate. Keep OS-agnostic packages
> byte-compatible with bx where they overlap.

- **M0 — Skeleton & port.** Repo scaffold, `osport.Platform` interface + `platform_windows.go`
  + `platform_stub.go`. **Gate:** `GOOS=windows go build ./...` green; `go vet` clean.
- **M1 — Discovery.** `browser_windows.go` (registry A5 + FS A3/A4), `proc_windows.go`
  (A11). **Gate:** `bxw list browsers` prints Edge+Brave with correct exe/dataDir on this box;
  `bxw list profiles brave` prints "Work" (A6).
- **M2 — Daemon + IPC.** UDS server/client, `LockFileEx` singleton, detached spawn.
  **Gate:** two concurrent `bxw` commands share one daemon; only one daemon spawns.
- **M3 — CDP attach + port discovery.** chromedp connect; port precedence (§8) incl.
  `/json/version` sweep (A8/A9). **Gate:** `bxw list tabs` against a running debuggable Edge.
- **M4 — Snapshot/refs + actions.** AX tree, `data-bxw-ref`, click/fill/type/key.
  **Gate:** E2E TodoMVC script passes headed.
- **M5 — Profile correlation.** nonce probe (§9). **Gate:** `open … --profile Work` lands in
  the right profile window.
- **M6 — setup/doctor/mcp.** pref-flip (A7) + V1 Allow-dialog validation; `bxw doctor`
  (runs §2); `bxw mcp` stdio + `bxw mcp install` (Claude/Cursor Windows paths).
  **Gate:** Claude Code drives a real Brave "Work" tab end-to-end with one Allow click.
- **M7 — Packaging/CI/release.** npm dispatch (+`.exe` fix), winget/scoop, ci.yml
  windows matrix, release.yml. **Gate:** `npx @org/bxw doctor` works from a clean machine;
  tagged release auto-publishes.

---

## 16. Open decisions (need your call)
1. **Name & scope of module path** (`bxw`? `winbx`? org/user for `github.com/<x>/bxw`).
2. **Fork vs clean-room start:** fork bx internally (fastest merge, but company repo) vs
   clean-room reimplement the OS-agnostic core from the published contracts (cleanest IP story
   for a public repo you own "for now"). Recommendation: **clean-room core + identical
   contracts**, so it's yours to open-source and still merges as adapters later.
3. **Distribution channels** for v1 (npm only, or npm + winget + scoop day one).
4. **Default agent safety:** should untrusted MCP clients default to **`bxw launch`
   (isolated profile)** rather than the real profile? (Recommended yes.)

---

## Appendix A — reproducible verification (raw probes)
All run on Windows 11 (build 26300), 2026-11-13. Re-run via `bxw doctor` once M6 lands.

- **A1:** `go version` → `go1.26.6 windows/amd64`; `go env GOMODCACHE` lists `sys@v0.42.0`.
- **A2:** `node -v`→`v24.14.1`; `npm -v`→`11.11.0`; `git --version`→`2.49.0.windows.1`.
- **A3/A4:** FS `Test-Path` on exe/dataDir → Edge/Brave present (versions above).
- **A5:** `HKLM:\SOFTWARE\Clients\StartMenuInternet` → Brave, Microsoft Edge, IEXPLORE;
  `App Paths\msedge.exe`,`brave.exe` → exact exe paths.
- **A6:** parse `Local State` → `profile.info_cache`: Edge `Default`→"Profile 1",
  Brave `Default`→"Work".
- **A7:** `devtools.remote_debugging.user-enabled` key present (unset).
- **A8:** `Start-Process msedge --remote-debugging-port=9333 --user-data-dir=<tmp>
  --headless=new`; `GET /json/version` → `ws://127.0.0.1:9333/devtools/browser/<uuid>`,
  `File-Path-Style: windows`; `/json/list` → page target + ws URL.
- **A9:** `DevToolsActivePort` absent in `<tmp>` under `--headless=new`.
- **A10:** Go program: `net.Listen("unix", "%LOCALAPPDATA%\bxw-test.sock")` + dial →
  `"echo:ping"`.
- **A11:** `SingletonLock`/`SingletonSocket` absent; Edge `lockfile` present; `Win32_Process`
  CommandLine → `msedge.exe --profile-directory=Default …`.
