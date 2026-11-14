# WORKORDER — Browser-Independent, Token-Minimal, Disk-Backed bxw

Supersedes the real-profile parts of M6. Three pillars: (A) one auth path that works on every
Chromium browser, (B) minimum tokens into the model, (C) all state on disk under one C: root.

---

## A. Browser independence — persistent isolated profile ("log in once")

**Why:** Chromium 136 blocks remote debugging on the real/default profile; the only remedy is a
non-default `--user-data-dir` (different encryption key → real logins unreadable). Verified:
Brave real-profile works (fragile exception), Chrome/Edge blocked, admin/policy does NOT lift it.
So the real profile is a dead end for browser independence.

**Design:** bxw owns a **persistent isolated profile per browser** under its data root. The user
signs into their accounts **once** inside it; it stays authenticated across sessions (it's a real,
persisted Chromium profile dir — cookies/tokens persist like any profile).
- Launch: `<exe> --remote-debugging-port=<free> --user-data-dir=%LOCALAPPDATA%\bxw\profiles\<browser>` (+ `--no-first-run --no-default-browser-check`).
- Works on Edge, Brave, Chrome, Vivaldi, Chromium — **identical code path, no per-browser pref/policy.**
- `bxw login <browser>` → opens the persistent profile headed so the user logs into their sites.
- Real/default profile stays a **Brave-only "advanced, at-your-own-risk"** mode behind an explicit
  Allow-consent + warning. Not Chrome/Edge.
- **Allow button:** bxw's own one-time consent before enabling debugging on any profile (Chromium's
  own dialog is absent/unreliable — V1 proved none fired on Brave). No UAC, no machine policy.

**Acceptance:** `bxw login brave` → user signs into a site → `bxw stop` → later `bxw snapshot` shows
still-logged-in; same on Edge and Chrome. No registry/policy writes anywhere.

---

## B. Token minimization (the model sees as little as possible)

Target: a typical action turn costs tens of tokens, not thousands. Techniques:

1. **Diff snapshots (biggest win).** Keep the last AX snapshot per tab on disk. After an action,
   return only the **delta** (added/removed/changed refs), not the whole tree. A full page is
   e1..e80+ (~2–4k tokens); a diff is usually a handful of lines (~20–80 tokens). ~10–100×.
   - `snapshot` full only on first call or explicit `--full`; otherwise diff vs stored baseline.
2. **Refs live on disk, never in tokens.** `ref → backendNodeId` map persisted per tab in
   `state/`. Model only ever sees short `e12` tokens; resolution happens server-side.
3. **Aggressive AX pruning.** Drop `generic/none/presentation/InlineTextBox/RootWebArea`, collapse
   wrapper chains, truncate names (~80 chars). Only interactive + landmark + text-bearing nodes.
4. **Big blobs → MCP resources, not inline.** Screenshots, full HTML, full snapshots, network
   bodies are written to `cache/` (content-addressed by sha256) and returned as an MCP
   **resource_link** (`bxw://cache/<hash>`), ~0 inline tokens; the client fetches only if needed.
5. **Terse tool results.** Actions return one line (`clicked e12 → 2 nodes changed`), not a dump.
   No echoing args, no JSON verbosity in model-facing text.
6. **Scoped queries.** `snapshot --ref e40` returns only that subtree; `find <text>` returns only
   matching refs. Avoid whole-page dumps when a region suffices.
7. **Dedupe / cache.** If the page is unchanged since last snapshot (mutation observer + hash),
   return `unchanged` (1 token) instead of re-sending.

**Acceptance:** fill-then-click-then-observe on an e80 page costs < 150 tokens of tool output
(measured), vs a full re-snapshot each step; screenshot returns a resource link, not base64.

---

## C. Disk-backed state — one C: data root

Like any native app, keep everything on disk, nothing large in memory/context.

**Root:** `%LOCALAPPDATA%\bxw\`  (= `C:\Users\<user>\AppData\Local\bxw\`). Local, non-roaming
(profiles/caches are big and machine-specific). Config may mirror to `%APPDATA%\bxw\config.toml`
if roaming is wanted.

```
%LOCALAPPDATA%\bxw\
├── config.toml                 # default browser, port range, profile policy, retention
├── profiles\<browser>\         # persistent isolated Chromium profiles (A) — the logged-in ones
├── run\<browser>.sock|.lock|.port   # daemon IPC + singleton + discovered port (existing)
├── state\<browser>\<tab>.json  # ref map + last snapshot baseline (B) for diffing
├── cache\<sha256>              # content-addressed: screenshots, full snapshots, HTML, net bodies
├── history\<browser>.jsonl     # per-profile browsing history (D) — audit + recall
├── permissions.json           # per-origin capability grants (D) — granted/denied/prompt + scope/TTL
├── context\<session>.json      # durable agent task/session state (D) — cheap resume
└── logs\<browser>.log          # daemon logs (rotated, size-capped)
```
Rules: content-addressed cache (sha256 of bytes) → free dedupe; size-capped + LRU eviction
(`cache.max_mb` in config); `bxw clean` purges cache/logs/state (keeps profiles); all paths via
the `osport` layer so non-Windows roots (`~/.local/share/bxw`, `~/Library/Application Support/bxw`)
drop in later. Everything the model would otherwise carry (snapshots, refs, blobs) lives here.

**Acceptance:** after a session, `profiles/` holds the logged-in profile, `cache/` holds blobs by
hash under the cap, `state/` holds the diff baselines; `bxw clean` reclaims space; no large data
ever sat in the agent's context.

---

---

## D. Permissions, history & context (stateful, industry-standard)

### Permissions — per-origin capability grants (W3C Permissions-API shape)
- **Record:** `{origin, capability, state: granted|denied|prompt, scope: once|session|always, grantedAt, expiresAt}`. Default **prompt** (least privilege), stored in `permissions.json` (human-readable, auditable).
- **Origin** = scheme+host(+port), isolated per site; optional subdomain/wildcard rules.
- **Capabilities map to risk tiers** (from the safety spec): *free* = navigate/read/snapshot/list (never gated); *gated* = form submit, purchase/pay, delete, download, send/post, enter-credentials, cross-origin nav. **Only gated actions consult the store.**
- **Flow:** daemon reaches a gated action → look up `(origin, capability)` → if `granted` & unexpired, proceed; else return `needs-consent` → **Allow button** (bxw consent UX, or MCP *elicitation* for agents) → user picks allow/deny + scope → persist. `always` = durable grant; `session` = daemon lifetime; `once` = not stored.
- **Manage:** `bxw perms` (list), `bxw perms revoke <origin> [cap]`, `bxw perms grant <origin> <cap> [--always]`. TTL expiry auto-prunes.

### Browsing history — per profile, on disk
- Append-only `history\<browser>.jsonl`: `{ts, profile, tabId, url, title, source: agent|user}`.
- Powers audit ("what did the agent visit"), context recall, and nav dedupe. `bxw history [--since <t>] [--grep <s>] [--profile <p>]`. Retention cap + `bxw clean`. **Local only — never auto-sent to the model**, only on explicit query (and then compactly).

### Context management — durable + token-frugal
- `context\<session>.json`: open tabs, active tab, last N actions, task notes, snapshot-baseline pointers. A new agent turn **reloads** this instead of re-deriving → cheap resume, no re-sending big data.
- **Model side:** the agent sees a compact `state` summary (tabs + current refs + last diff); full data stays on disk (pillar B). `bxw context` prints it; MCP exposes it as a small resource.
- History + permissions + baselines together are the durable "what's going on" the agent reads back **without burning context**.

---

## Build order
1. Data-root layout + `osport` path methods (`ProfilesDir/StateDir/CacheDir/LogsDir/HistoryDir/ContextDir`, config load).
2. Persistent-profile launch + `bxw login <browser>` (pillar A) — replaces pref/policy real-profile.
3. On-disk ref map + snapshot baseline; diff engine; `snapshot` full/diff/scoped (pillar B).
4. Content-addressed `cache/` + MCP `resource_link` returns for blobs; terse action results.
5. Permissions store + gated-action check + Allow/elicitation wiring; `bxw perms`.
6. History logger (`history\`) + `bxw history`; context store (`context\`) + compact `state` resource + `bxw context`.
7. `bxw clean`, cache cap/LRU, config.toml (retention for history/cache/context).
8. Keep Brave-only real-profile as an advanced flag behind Allow-consent + warning.

## Out of scope / refused
No CAPTCHA solving, bot-detection evasion, stealth/fingerprint spoofing, or
undetected-chromedriver-style tooling — defeats the human/bot checks; not built.
