# Security Policy

`bxw` drives local Windows Chromium browsers over the Chrome DevTools Protocol (CDP). Because browser automation touches live sessions and user data, understanding the security boundaries is essential.

## Security Architecture & Boundaries

### 1. Zero External Telemetry & Local Loopback Only
- `bxw` makes **no external network connections**. All network operations occur exclusively over loopback (`127.0.0.1` / `::1`) or local inter-process communication (AF_UNIX sockets / named pipes).
- No analytical events, diagnostic payloads, or user interactions leave your machine.

### 2. Privilege Model
- `bxw` runs strictly with your standard user privileges.
- It **never requires administrative elevation (UAC)**, does not install system drivers, and does not modify system-wide registry hives (`HKEY_LOCAL_MACHINE`). Browser detection queries read-only registry paths (`HKEY_CURRENT_USER` and `HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths`).

### 3. Profiles & CDP Access
`bxw` provides two distinct modes of operation with differing threat boundaries:

#### A. Isolated Agent Mode (Default for `bxw launch`)
- Launches a dedicated browser instance in a temporary, isolated user data directory (`%TEMP%\bxw-<browser>-<timestamp>`).
- Completely isolated from your real personal profiles, extensions, passwords, cookies, and browsing history.
- The dynamic CDP port binds to `127.0.0.1` and is torn down when the session exits.
- **Recommended for:** general web scraping, automated testing, unauthenticated workflows, and untrusted agent prompts.

#### B. Real-Profile Attach Mode (`bxw setup`)
- Enables remote debugging on your primary browser by toggling `devtools.remote_debugging.user-enabled: true` within `<UserDataDir>\Local State`.
- **Consideration:** Enabling CDP allows any local process running with user privileges on your machine to connect to the browser's local debugging port while the browser is running.
- **Opt-In & Reversibility:** `bxw` never silently enables this flag. It requires explicit user consent via `bxw setup <browser>`, and can be reverted at any time using:
  ```powershell
  bxw setup <browser> --revert
  ```
  A backup of the previous configuration is automatically saved at `<UserDataDir>\Local State.bxw.bak`.

### 4. IPC Protection
- Communications between the `bxw` CLI/MCP clients and the background daemon occur over an AF_UNIX domain socket stored inside the user's `%LOCALAPPDATA%\bxw\run\` directory.
- On Windows, NTFS file permissions restrict access to files created in this directory to the running user account and system administrators.

---

## Reporting a Vulnerability

If you discover a security vulnerability in `bxw`, please report it responsibly:
- Email the maintainers directly or open a confidential Security Advisory on GitHub.
- Do not open public issues for suspected security vulnerabilities.
- Please provide a reproducible proof-of-concept and detailed version information (output of `bxw doctor`).

