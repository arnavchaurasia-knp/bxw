# WORKORDER — Fix-list (6 flags from the A–D audit)

All found live on Windows 11 against commit `19ad77c` (+ uncommitted changes). None is
architectural; these are polish/correctness. Fix each, keep all existing tests green, then
re-run `docs/WORKORDER_VERIFICATION_AUDIT.md`.

---

### F1 — Working tree not committed (recurring)
**Problem:** `19ad77c` has uncommitted edits on top (`internal/{cache,config,contextmgr,history,perms}/*.go`, `internal/daemon/diff*.go`, `.gitignore`). Audits keep validating uncommitted state; "clean tree / all committed" claims have been false 3×.
**Fix:** commit the working tree with an accurate message; from now, **commit each milestone fully before reporting it done** (the report must match `git status --porcelain` = empty).
**Acceptance:** `git status --porcelain` is empty; `git log -1` matches what's on disk.

### F2 — `CDP did not come up on <port>` — RESOLVED (root cause: browser singleton, not a readiness race)
**Status (FIXED + live-verified 2026-11-14):** reuse-on-launch + kill-the-tree-on-stop implemented and verified. Edge **10/10** and Brave **5/5** `launch → list tabs → stop` cycles: every `list tabs` exits 0, **zero** "CDP did not come up", **zero** orphaned `*\bxw\profiles\*` processes after the final stop (teardown log shows "terminated 14 browser process(es)" per cycle); the user's own browser (different profile) was untouched. Changes: `cdpFromProfile` reuse guard in `internal/daemon/cdp.go`; `teardownBrowser` in `internal/daemon/server.go`; `KillByProfileDir` port in `internal/osport/{osport,platform_windows,platform_other}.go` (matches the `--user-data-dir` profile path via the PEB, never the image name). AGY's earlier backoff is kept only as a cold-start guard.

---
**Original diagnosis (for the record) — AGY's backoff/timeout tuning was in and the acceptance test still FAILED.** Clean baseline (zero bxw-launched Edge), the result was deterministic:
- cycle 1: `launch edge` OK, CDP ready, `list tabs` OK;
- cycle 2: `launch edge` → `CDP did not come up on 127.0.0.1:<port>`; `list tabs` → "no Microsoft Edge is running with remote debugging";
- cycle 3+: same.
A handful of cycles also left **18 orphaned `msedge.exe`** processes (profile `…\bxw\profiles\edge`) still running after `stop`.

**Real root cause (not a race):**
1. `launch` (`internal/daemon/cdp.go:55`) spawns Edge with `--user-data-dir=<bxw profile>` and stores `d.agentPID`, **but nothing ever terminates the browser.** `stop` (`internal/daemon/server.go:292`) only closes `d.stop`; the `Serve` defer removes the sock/lock/port files — never the Edge process.
2. So every `launch` leaves an Edge instance alive. The next `launch` reuses the **same** `--user-data-dir`, so Chromium's single-instance lock routes the new invocation to the existing process, which exits immediately — **the new `--remote-debugging-port` flag is silently ignored.** `waitCDP` then polls a port nothing will ever bind → "CDP did not come up." No timeout length fixes this.
3. `discoverPort` reads `DevToolsActivePort` only from the **everyday** profile dir (`cdp.go:148`), never from the bxw persistent profile — so it cannot even fall back to attaching to the already-running instance.

**Fix (address the cause, keep the backoff as a secondary guard):**
- **Reuse-or-replace on launch.** Before spawning, read `<bxw profile>/DevToolsActivePort`; if an instance is already up for that profile, **attach to it** (set `d.cdpPort` from that file) instead of spawning a colliding process. This makes repeat `launch` idempotent and is the cleanest path.
- **Tear the browser down on stop/exit.** Record the real browser PID (the `SpawnDetached` PID is unreliable — the Chromium launcher process exits after handing off to the singleton), or identify the instance by its `--user-data-dir=<bxw profile>` on the process command line, and kill that process tree on `stop` and on daemon exit. On Windows, matching the profile path (`Get-CimInstance Win32_Process` / `taskkill /T`) is the robust route; never kill by image name alone (would kill the user's real Edge).
- Keep `waitCDP` backoff only as a cold-start guard for the *first* launch.
**Acceptance (unchanged, must actually be run):** 10 consecutive `launch → list tabs → stop` cycles on Edge **and** Brave, each cycle: `list tabs` exits 0, **zero** "CDP did not come up", and **zero** orphaned `*\bxw\profiles\*` browser processes left after the final `stop` (verify with a command-line-filtered process query, not an image-name count).

### F3 — `screenshot` is MCP-only; `bxw://cache` resource link unverified
**Problem:** no CLI `screenshot` verb (`bxw: unknown command "screenshot"`), so the content-addressed resource-link path (`bxw://cache/<hash>.png` via MCP `resources/read`) was not verifiable from the CLI audit.
**Fix:** (a) add CLI `bxw screenshot <tab> [--out <file>] [--full]` that captures, writes to `cache/`, and prints the `bxw://cache/<hash>.png` link; (b) add an integration test that drives the MCP server: call the `screenshot` tool, assert the result is a `resource_link`, then `resources/read` that URI and assert bytes = the cached PNG.
**Acceptance:** `bxw screenshot <tab>` prints a `bxw://cache/...png` link; MCP test reads it back; the PNG exists in `cache/` by its sha256.

### F4 — `always` permission grant carries an expiry
**Problem:** `perms grant <origin> <cap> --always` listed an `EXPIRES` ~1 day out; `always` must never expire. (`internal/perms/perms.go`.)
**Fix:** for `scope == always`, store no expiry (zero `expiresAt`); render `EXPIRES` as `never`. Keep TTL only for `session`/`once`.
**Acceptance:** `perms grant … --always` then `perms` shows `EXPIRES=never`; the grant survives a simulated clock advance in a unit test.

### F5 — Diff degenerates to ~full on wholesale page change (optimization, not a bug)
**Problem:** a full-page replacement produced a diff (2093 chars) ≈ the full tree (2137) — correct, but no token win in that case.
**Fix:** when the computed diff is not materially smaller than the full tree (e.g. ≥ ~85% of full lines, or `removed+changed ≥ 0.7×baseline`), return the **full** snapshot instead with a one-line note (`[page replaced — full snapshot]`), and reset the baseline. Cheaper and clearer for the model than a near-full diff.
**Acceptance:** unit test: wholesale replacement returns full + note; incremental change still returns a small diff; unchanged still returns `unchanged`.
**Status (2026-11-14):** heuristic is implemented in `actions.go` (changed ≥ 0.7×baseline OR diff ≥ 0.85×full) and the `[page replaced — full snapshot]` prefix renders. **Partial:** the only test (`TestSnapshotPageReplacedFormatting`) asserts the note *formatting* when `PageReplaced` is preset — it does **not** exercise the threshold decision. Add a test that feeds a baseline + a wholesale-replacement node set through the snapshot/diff path and asserts `PageReplaced` is chosen (and that an incremental change is not), so the 0.7/0.85 logic itself is covered.

### F6 — Stale artifacts linger in the data root
**Problem:** data root held old root-level `*.lock/.sock`/`.log`, orphaned agent dirs, and a `test-agent-9333` dir — from pre-`run/` builds and crashed daemons.
**Fix:** `bxw clean` sweeps stale `run/` files whose owning daemon is not alive (lock not held / socket dead), orphaned `*-agent*` profile dirs, and any `test-*` dirs; daemon removes its own `run/<browser>.{sock,lock,port}` on exit (crash-safe: on next start, reclaim if lock is free). Document `clean --all` vs default retention.
**Acceptance:** after a crash-kill of the daemon, `bxw clean` leaves only live artifacts; no `test-*` or orphaned agent dirs remain; `run/` holds only files for running daemons.

---

## Verify after fixes
- `go vet ./...` clean; `go test ./... -count=1` green; Linux cross-compile green.
- Re-run `docs/WORKORDER_VERIFICATION_AUDIT.md` §1–§7; all six flags resolved; `git status` clean.
