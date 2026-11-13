# WORKORDER — M6: Real-Profile Attach + setup / doctor / MCP

**Goal.** Drive the user's **real, authenticated** browser profile (their Google/GitHub/AWS
logins), and expose bxw to agents as an MCP server. This is the milestone that makes bxw
useful beyond the isolated agent sandbox.

**Depends on:** M3 (daemon/CDP), M4 (snapshot + actions). M5 (profile targeting) pairs with
this. **This is the milestone with the two deferred interactive validations (V1, V2) — resolve
them here.**

---

## Contracts this builds on
- `osport.Platform`: `Lock`, `SpawnBrowser`, `SpawnDetached`, `LocalStatePath(dataDir)`,
  `Dir/SocketPath/LockPath/LogPath`, `LocalAppData`.
- `browser.Browser{Name,Label,Exe,DataDir,Inspect}`, `browser.Profiles(b)`, `browser.WSURL()`.
- Daemon ops/dispatch; `daemon.Call`.

## Part A — Enable remote debugging on the real profile (anchor A7)
Chrome 136+ refuses `--remote-debugging-port` on the **default** user-data dir, and a custom
dir can't see the real profile's cookies. The supported path is the `chrome://inspect` opt-in
**pref**, set while the browser is closed:

1. **Detect running browser + its profile** — add `osport` method
   `RunningBrowser(def BrowserDef) (pid int, userDataDir, profileDir string, ok bool)`.
   Windows impl uses process enumeration + CommandLine (anchor A11): match `msedge.exe`/
   `brave.exe`, parse `--user-data-dir` / `--profile-directory` from the command line via
   `Win32_Process` (WMI) or `CreateToolhelp32Snapshot` + `NtQueryInformationProcess`. (Live
   sample captured during probing: `msedge.exe --profile-directory=Default …`.)
2. **Require the browser closed.** If running, instruct the user to quit it (or offer a
   graceful close: `WM_CLOSE` to the main window, then verify exit; never `TerminateProcess`
   on the user's real browser without consent). **Do not** close it silently.
3. **Flip the pref.** Read `<DataDir>\Local State` (JSON), set
   `devtools.remote_debugging.user-enabled = true` (create the nested objects if absent),
   write it back **preserving all other keys** (decode into a generic map, not a narrow
   struct). Verified key path exists on this machine (A7, currently unset).
4. **Relaunch + attach.** Start the browser; connect via CDP. The **first** connection raises
   the Chromium "Allow remote debugging?" dialog (opt-in flow) — the daemon holds the one
   connection so the user clicks Allow once (V1).
5. **Revert.** `bxw setup <browser> --revert` sets the pref back to `false`.

### V1 / V2 — the interactive validations to perform here
- **V1 (Allow dialog):** confirm the per-connection Allow dialog appears once on Windows and
  that the daemon's persistent connection suppresses further prompts. Document the exact UX.
- **V2 (port discovery for a pref-enabled real browser):** determine **which port** a
  pref-enabled (no-flag) browser exposes and how to find it. Expected resolution order:
  `GET /json/version` sweep of candidate ports, and/or reading `DevToolsActivePort` from the
  real `DataDir` (anchor A9 says this file is unreliable under headless — the real browser is
  headed, so it is more likely present; **verify empirically**). If the pref alone does not
  open a port, fall back to launching the real profile with `--remote-debugging-port=<fixed>`
  **after** the pref is set and the browser is closed. Record the finding in `docs/DESIGN.md`.

## Part B — `bxw setup` and `bxw doctor`
- `setup.go`: `bxw setup <browser>` (flip pref, guide through Allow), `--revert`.
  On Windows, the "is it installed" check uses `%ProgramFiles%`/registry (not macOS
  `/Applications`), and self-update (M7) fetches `bxw_windows_<arch>.zip`.
- `doctor.go`: extend `bxw doctor` to run the §2 verification checks (browsers, profiles,
  CDP reachability, socket, pref state) and exit non-zero on failure — the reproducible
  machine self-check.

## Part C — MCP server (`bxw mcp`) + installer
- **Dep:** add an MCP Go SDK (e.g. `github.com/modelcontextprotocol/go-sdk`) — new `go.mod`
  entry; pin an exact version.
- `internal/app/mcp.go`: `bxw mcp` speaks MCP over **stdio** (same stdio the npm launcher
  passes through). Register tools grouped **setup** (doctor, list_browsers, use, launch),
  **read** (list_tabs, snapshot, read, screenshot, wait), **act** (open, goto, click, fill,
  type, key, close_tab) — each tool just calls the existing daemon op, so the tool layer is a
  thin schema wrapper over `daemon.Call`.
- `internal/app/mcpinstall.go`: `bxw mcp install [client]` writes/updates the client config:
  - Claude Desktop (Windows): `%APPDATA%\Claude\claude_desktop_config.json`
  - Cursor: `%APPDATA%\Cursor\...` and/or project `.cursor/mcp.json`
  Merge into existing JSON (don't clobber other servers); entry runs `bxw mcp` (or `npx -y
  @scope/bxw mcp`).

## Files
- `internal/osport/osport.go` + `platform_windows.go` + `platform_other.go` —
  `RunningBrowser`, graceful `Terminate`, and a `LocalState` read/write helper (or keep the
  JSON edit in `browser`/`setup`).
- `internal/browser/prefs.go` — NEW: read/modify/write `Local State` preserving unknown keys.
- `internal/app/setup.go`, `internal/app/mcp.go`, `internal/app/mcpinstall.go` — NEW.
- `internal/daemon/*` — attach path that targets the real browser's WS (reuse M4 session
  helper; port from Part A step 4/V2).

## Acceptance gate
- `bxw setup brave` → (browser closed) flips the pref; relaunch + `bxw use brave` connects with
  **one** Allow click; `bxw list tabs brave` shows the user's **real** tabs (logged-in sites).
- End-to-end: Claude Code (via `bxw mcp`) snapshots and drives a real Brave "Work" tab.
- `bxw setup brave --revert` restores the pref; `bxw doctor` passes.
- `go vet` clean; Linux stub still builds.

## Gotchas / anchors
- **Never** silently kill or restart the user's real browser; always confirm (outward-facing,
  surprising action).
- **Preserve Local State:** edit as a generic map; a narrow struct round-trip would drop the
  user's other settings. Back up the file before first write.
- **A11 CommandLine access** worked unelevated during probing, but may be restricted in some
  environments — have a fallback (toolhelp snapshot) and a clear error.
- **Security:** enabling the pref exposes the profile to any local program; state this in
  `SECURITY.md` and in the `setup` confirmation prompt. Offer `bxw launch` (isolated) as the
  safer default for untrusted agents.
