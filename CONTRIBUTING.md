# Contributing to bxw

Thank you for your interest in contributing to `bxw`!

`bxw` is an open-source, Windows-first browser automation engine and Model Context Protocol (MCP) server for AI coding agents. It is designed to be clean-room, robust, dependency-light, and cross-compilation friendly.

---

## Architecture Overview

`bxw` follows a hexagonal / port-and-adapter architecture:
```
                      ┌──────────────────────────────────────┐
                      │              bxw CLI / MCP           │
                      └───────────────────┬──────────────────┘
                                          │ (AF_UNIX RPC)
                      ┌───────────────────▼──────────────────┐
                      │             Daemon Core              │
                      │  - CDP WebSocket (cdproto)           │
                      │  - AX Snapshot Walker (refs e1, e2)  │
                      │  - Actions (click, fill, type, key)  │
                      │  - Profile Correlator (nonce probe)  │
                      └───────────────────┬──────────────────┘
                                          │
                      ┌───────────────────▼──────────────────┐
                      │            osport.Platform           │
                      │  (Hexagonal OS Port Interface)       │
                      ├───────────────────┬──────────────────┤
                      │                   │                  │
            platform_windows.go    platform_other.go (stub)
          (Registry, Paths, Procs)  (Build & CI compatibility)
```

- **Hexagonal OS Port (`internal/osport`)**: Core logic never branches on `runtime.GOOS`. Host-specific behaviors (registry lookups, Local State paths, process snapshots, user directories) are defined in `osport.Platform` and implemented by `platform_windows.go`.
- **Cross-Compilation Stub (`platform_other.go`)**: A stub implementation ensures that `bxw` cleanly cross-compiles on Linux/macOS, enabling standard CI pipelines to vet and test without requiring Windows runners for pure logic validation.

---

## Prerequisites

- **Go**: 1.22 or higher (1.24 recommended)
- **Node.js & npm** (optional, for packaging and npm runner): Node 18+
- **Windows 10 or 11** for native browser live integration testing.

---

## Development Workflow

### 1. Building Locally
```powershell
# Build Windows executable
go build -o bxw.exe .

# Verify version and doctor checks
.\bxw.exe doctor
```

### 2. Running Tests
```powershell
# Run all unit tests
go test -v ./...

# Run static analysis
go vet ./...
```

### 3. Verifying Cross-Compilation (Linux Stub)
Ensure that changes do not break non-Windows compilation targets:
```powershell
$env:GOOS = "linux"
go build ./...
$env:GOOS = "windows"
```

### 4. Testing NPM Packaging Locally
```powershell
# Build npm packages into npm/
.\scripts\build.ps1 -Version "0.1.0-dev"

# Test the node launcher
node npm/bxw/bin/bxw.js doctor
```

---

## Code Guidelines & Standards

1. **Clean-Room Engineering**: Write original code based strictly on public specifications, RFCs, and documentation (such as the Chrome DevTools Protocol specifications). Do not copy code from decompiled or proprietary sources.
2. **Standard Library First**: Minimize external dependencies. `bxw` relies solely on `golang.org/x/sys`, `github.com/coder/websocket`, and typed protocol bindings (`github.com/chromedp/cdproto`).
3. **No Unsafe Concurrency**: All daemon sessions and WebSocket connections must be gracefully cancellable via `context.Context`.
4. **Number Preservation**: When deserializing browser configuration files (like `Local State`), always use `json.Decoder.UseNumber()` to prevent silent precision loss on 64-bit timestamps and IDs.

---

## Submitting Pull Requests

1. Fork the repository and create your branch from `main`.
2. Ensure `go test ./...` and `go vet ./...` pass with zero warnings.
3. Verify that `GOOS=linux go build ./...` succeeds.
4. Open a pull request with a clear description of the problem solved and test steps performed.
