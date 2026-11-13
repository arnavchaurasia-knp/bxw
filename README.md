# bxw

**Drive the Windows Chromium browser you already have open — by profile, over the Chrome DevTools Protocol (CDP) — from AI coding agents.**

`bxw` lets AI coding agents (Claude Desktop, Cursor, Antigravity, Codex, Cline) interact with real, authenticated browser sessions or isolated sandboxes. It extracts token-efficient accessibility snapshots with sequential element references (`e1`, `e2`, `e3`), then drives actions (`click`, `fill`, `type`, `key`, `eval`) with zero brittle coordinate guessing.

Built in clean-room Go, `bxw` runs as a single static binary with no external runtimes, zero telemetry, and an OS-agnostic hexagonal core.

---

## Features

- **Windows-First Discovery**: Automatically detects Microsoft Edge, Brave, Google Chrome, Vivaldi, and Chromium via registry `App Paths` and discovers all user profiles (`Default`, `Profile 1`, etc.) from `Local State`.
- **Two Execution Modes**:
  - **Isolated Agent Sandbox (`bxw launch`)**: Creates an ephemeral, isolated profile on a dynamic CDP port. Perfect for unauthenticated scraping, testing, or sandboxed browsing.
  - **Real Authenticated Profile (`bxw setup`)**: Opts into Chromium's native remote debugging flag (`devtools.remote_debugging.user-enabled`) so your agent can navigate internal dashboards, Google Workspace, GitHub, or Jira using your real logged-in sessions.
- **Accessibility Snapshots**: Walks the DOM AXTree, pruning noise (`generic`, `none`, inline text nodes) and assigns concise references:
  ```text
  [e1] heading "Sign In"
  [e2] textbox "Email address"
  [e3] textbox "Password" [focused]
  [e4] checkbox "Remember me" [checked]
  [e5] button "Submit"
  ```
- **Ref-Based Precision Actions**: Tag elements in DOM, compute center coordinates, and dispatch real mouse and keyboard events:
  - `bxw click e5`
  - `bxw fill e2 user@example.com`
  - `bxw type e3 s3cr3t --submit`
  - `bxw key Enter`
- **Profile Correlation via Nonce Probe**: Dynamically correlates opaque Chromium `BrowserContextID`s with profile directories, allowing `bxw open <browser> <url> --profile "Work"` to open tabs directly in the requested profile.
- **Model Context Protocol (MCP) Server**: Built-in JSON-RPC 2.0 stdio server exposing 9 core tools to any MCP-compliant AI assistant.
- **One-Command MCP Installer**: `bxw mcp install` automatically configures Claude Desktop, Cursor, and Antigravity.
- **AF_UNIX Singleton Daemon**: Manages persistent WebSocket multiplexing to CDP behind a lightweight background daemon over Windows AF_UNIX domain sockets.

---

## Installation

### Option 1: via npm / npx (Recommended for AI agents)
```bash
# Run immediately with npx
npx bxw doctor

# Or install globally
npm install -g bxw
```

### Option 2: Pre-compiled Windows Binaries
Download the latest pre-compiled archive for your architecture from the [GitHub Releases](https://github.com/arnavchaurasia/bxw/releases) page:
- **x64 (Intel/AMD)**: `bxw_windows_amd64.zip`
- **ARM64 (Surface / Snapdragon)**: `bxw_windows_arm64.zip`

Extract `bxw.exe` and place it in your `PATH` (e.g. `%USERPROFILE%\bin` or `C:\Windows\System32`).

### Option 3: Build from Source
```powershell
git clone https://github.com/arnavchaurasia/bxw.git
cd bxw
go build -o bxw.exe .
```

---

## Quickstart

### 1. Check Your Machine
```powershell
bxw doctor
```
Inspects all detected Chromium browsers, profile directories, running instances, and daemon statuses.

### 2. Isolated Mode (Sandbox)
```powershell
# Launch an isolated Edge session
bxw launch edge https://github.com

# List open tabs
bxw list tabs edge

# Take an accessibility snapshot
bxw snapshot edge

# Click an element by ref
bxw click e3 edge

# Stop the daemon & close the instance
bxw stop edge
```

### 3. Real-Profile Mode (Your Logged-in Browser)
```powershell
# 1. Enable remote debugging flag in Edge's Local State
bxw setup edge

# 2. Restart Edge normally
# 3. Open a tab in your "Default" or "Profile 1" profile
bxw open edge https://github.com --profile "Default"

# 4. Interact with your logged-in session
bxw snapshot edge
```
*To revert the debugging flag at any time:*
```powershell
bxw setup edge --revert
```

---

## CLI Reference

| Command | Description |
|---|---|
| `bxw doctor` | Self-test machine environment, browsers, profiles, and daemons |
| `bxw list browsers` | List all installed Chromium browsers, binary paths, and user data directories |
| `bxw list profiles <browser>` | List all profile directories and human-readable names for a browser |
| `bxw launch <browser> [url]` | Launch an isolated temporary agent instance on a dynamic CDP port |
| `bxw list tabs [browser]` | List open tabs (tab ID, title, URL) across connected instances |
| `bxw open <browser> <url> [--profile <name>]` | Open a URL in a new tab, optionally targeting a specific profile |
| `bxw snapshot [browser] [--tab <id>]` | Print indented AXTree element snapshot with `e1`, `e2` refs |
| `bxw click <ref> [browser] [--tab <id>]` | Click an element by its snapshot reference |
| `bxw fill <ref> <text> [browser] [--tab <id>]` | Focus, clear, and populate text into an input or textarea |
| `bxw type <ref> <text> [--submit]` | Type characters sequentially into an element, optional Enter submit |
| `bxw key <key> [browser] [--tab <id>]` | Dispatch a keypress (e.g. `Enter`, `Tab`, `Escape`, `ArrowDown`) |
| `bxw eval <code> [browser] [--tab <id>]` | Evaluate a JavaScript expression in the tab's execution context |
| `bxw setup <browser> [--revert]` | Toggle `devtools.remote_debugging.user-enabled` in `Local State` |
| `bxw stop <browser>` | Terminate the background daemon and disconnect CDP sessions |
| `bxw mcp serve` | Run the Model Context Protocol (MCP) server over stdio |
| `bxw mcp install [--target all\|claude\|cursor\|antigravity]` | Automatically register the `bxw` MCP server with agent configurations |

---

## Model Context Protocol (MCP) Setup

`bxw` includes a zero-dependency, JSON-RPC 2.0 stdio MCP server exposing 9 browser automation tools:

1. `list_browsers`
2. `list_tabs`
3. `open`
4. `snapshot`
5. `click`
6. `fill`
7. `type`
8. `key`
9. `eval`

### Automatic Setup
Run:
```powershell
bxw mcp install
```
`bxw` automatically detects and updates the configuration files for **Claude Desktop**, **Cursor**, and **Antigravity**.

### Manual Setup

#### Claude Desktop
Add to `%APPDATA%\Claude\claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "bxw": {
      "command": "bxw",
      "args": ["mcp", "serve"]
    }
  }
}
```

#### Cursor
Add to `.cursor/mcp.json` (or Cursor Settings $\to$ Features $\to$ MCP):
```json
{
  "mcpServers": {
    "bxw": {
      "command": "bxw",
      "args": ["mcp", "serve"]
    }
  }
}
```

#### Antigravity
Add to `%USERPROFILE%\.gemini\antigravity\mcp_config.json`:
```json
{
  "mcpServers": {
    "bxw": {
      "command": "bxw",
      "args": ["mcp", "serve"]
    }
  }
}
```

---

## Architecture & Design

```
┌────────────────────────────────────────────────────────┐
│             AI Agent (Claude / Cursor / CLI)           │
└───────────────────────────┬────────────────────────────┘
                            │ (stdio MCP / CLI invocations)
┌───────────────────────────▼────────────────────────────┐
│                    bxw CLI & MCP Server                │
└───────────────────────────┬────────────────────────────┘
                            │ (AF_UNIX socket RPC)
┌───────────────────────────▼────────────────────────────┐
│                     bxw Daemon                         │
│  - WebSocket multiplexing over github.com/coder/websocket│
│  - Typed CDP protocol using github.com/chromedp/cdproto │
│  - AXTree walker with role/aria filtering              │
│  - Action dispatch (DOM node resolution & synthetic input)│
│  - Profile correlation via data: URL nonce probes      │
└───────────────────────────┬────────────────────────────┘
                            │ (CDP over WebSocket)
┌───────────────────────────▼────────────────────────────┐
│                 Chromium Browser                       │
│    (Microsoft Edge / Brave / Google Chrome / etc.)     │
└────────────────────────────────────────────────────────┘
```

### Hexagonal OS Abstraction (`osport.Platform`)
The core automation logic never branches on `runtime.GOOS`. All Windows-specific operations (Registry queries, App Paths lookups, `Local State` parsing, and Toolhelp32 process snapshots) are sequestered in `internal/osport/platform_windows.go`. A stub implementation (`platform_other.go`) ensures clean cross-compilation on Linux and macOS for testing and CI.

---

## Security

`bxw` has **zero external network telemetry**, makes no outbound internet requests, and requires **no administrator privileges**.

For detailed discussion on local loopback security, the difference between Isolated Agent Mode and Real-Profile Attach, and safety protocols, refer to [SECURITY.md](SECURITY.md).

---

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on code style, testing workflows, and cross-compilation requirements.

---

## License

[MIT License](LICENSE) © 2026 Arnav Chaurasia
