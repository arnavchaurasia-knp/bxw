# bxw

**Drive the Windows Chromium browser you already have open — by profile, over the DevTools Protocol — from a coding agent.**

`bxw` lets agents (Claude Code, Codex, Cursor) attach to your *real, authenticated* browser
profile instead of a blank sandbox: snapshot the page into token-efficient element refs,
then click / type / read by ref. One Go binary, no runtime dependencies, no telemetry.

It is a clean-room, Windows-first sibling of [`bx`](https://github.com/Facets-cloud/bx)
(macOS-only), built behind an OS-abstraction port so the OS-agnostic core stays
contract-compatible and can later merge upstream as Windows adapters.

> **Status:** early. M0 (discovery) works. Attach / snapshot / act land next. See
> `docs/` for the full HLD+LLD and milestone plan.

## Supported (today)
- **OS:** Windows 10/11 (built and verified on Windows 11).
- **Browsers:** auto-discovers Edge, Brave, Chrome, Vivaldi, Chromium (Edge + Brave verified live).

## Install / build
```sh
go build -o bxw.exe .
```

## Usage (M0)
```sh
bxw list browsers            # installed Chromium browsers + exe + user-data dir
bxw list profiles <browser>  # a browser's profiles (folder -> display name)
bxw doctor                   # self-check this machine
```

Example:
```
> bxw list profiles brave
DIR      NAME
Default  Work
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
