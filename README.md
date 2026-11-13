# bxw

**Drive the Windows Chromium browser you already have open — by profile, over the DevTools Protocol — from a coding agent.**

`bxw` lets agents (Claude Code, Codex, Cursor) attach to your *real, authenticated* browser
profile instead of a blank sandbox: snapshot the page into token-efficient element refs,
then click / type / read by ref. One Go binary, no runtime dependencies, no telemetry.

It is a clean-room, Windows-first sibling of [`bx`](https://github.com/Facets-cloud/bx)
(macOS-only), built behind an OS-abstraction port so the OS-agnostic core stays
contract-compatible and can later merge upstream as Windows adapters.

> **Status:** M0–M3 implemented & verified live on Windows 11. Discovery, singleton daemon over AF_UNIX IPC, and live CDP tab enumeration are working. Actions and snapshot refs land next. See `docs/` for the full HLD+LLD and milestone plan.

## Supported (today)
- **OS:** Windows 10/11 (built and verified on Windows 11).
- **Browsers:** auto-discovers Edge, Brave, Chrome, Vivaldi, Chromium (Edge + Brave verified live).

## Install / build
```sh
go build -o bxw.exe .
```

## Usage
```sh
bxw list browsers                   # installed Chromium browsers + exe + user-data dir
bxw list profiles <browser>         # a browser's profiles (folder -> display name)
bxw launch <browser> [url]          # launch isolated agent instance with dynamic CDP port
bxw list tabs [browser]             # query open tabs over CDP via the singleton daemon
bxw stop <browser>                  # stop the daemon
bxw doctor                          # self-check this machine
```

Example:
```
> bxw launch edge https://example.com
launched Microsoft Edge agent instance (CDP ready: ws://127.0.0.1:54801)

> bxw list tabs edge
ID        TITLE           URL
14FB1C59  Example Domain  https://example.com/

> bxw stop edge
stopped daemon for edge
```

## Architecture
```
CLI / MCP (stdio)  ──RPC──►  daemon (singleton, 1 CDP connection)  ──CDP──►  your browser
         └──────────── OS-agnostic core (snapshot/refs/tools) ───────────┘
                       └── osport.Platform ── windows adapter ──┘
```
The core never branches on `GOOS`; every host difference lives behind `internal/osport`.

## License
MIT. No telemetry. Opening a debug port exposes the profile to local programs — see
`SECURITY.md` (coming) before using on a sensitive profile.
