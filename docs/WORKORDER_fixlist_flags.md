# WORKORDER — Fix-list (6 flags from the A–D audit)

All found live on Windows 11 against commit `19ad77c` (+ uncommitted changes). None is
architectural; these are polish/correctness. Fix each, keep all existing tests green, then
re-run `docs/WORKORDER_VERIFICATION_AUDIT.md`.

---

### F1 — Working tree not committed (recurring)
**Problem:** `19ad77c` has uncommitted edits on top (`internal/{cache,config,contextmgr,history,perms}/*.go`, `internal/daemon/diff*.go`, `.gitignore`). Audits keep validating uncommitted state; "clean tree / all committed" claims have been false 3×.
**Fix:** commit the working tree with an accurate message; from now, **commit each milestone fully before reporting it done** (the report must match `git status --porcelain` = empty).
**Acceptance:** `git status --porcelain` is empty; `git log -1` matches what's on disk.

### F2 — Flaky daemon startup: `CDP did not come up on <port>`
**Problem:** recurred across launches; the daemon/attach reports the port isn't up, then recovers — a readiness race between browser launch, port bind, and `discoverPort`. (`internal/daemon/cdp.go` `waitCDP`/`launch`, `internal/daemon/client.go` spawn handshake.)
**Fix:** don't signal ready until `GET /json/version` actually answers on the chosen port; give `waitCDP` a longer, backoff-based poll (cold Edge start can exceed the current window); make `list_tabs`/`snapshot` retry `discoverPort` with backoff instead of failing on the first miss. Log the timeline to `logs/` for diagnosis.
**Acceptance:** 10 consecutive `launch → list tabs → stop` cycles on Edge and Brave with **zero** "CDP did not come up" warnings.

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

### F6 — Stale artifacts linger in the data root
**Problem:** data root held old root-level `*.lock/.sock`/`.log`, orphaned agent dirs, and a `test-agent-9333` dir — from pre-`run/` builds and crashed daemons.
**Fix:** `bxw clean` sweeps stale `run/` files whose owning daemon is not alive (lock not held / socket dead), orphaned `*-agent*` profile dirs, and any `test-*` dirs; daemon removes its own `run/<browser>.{sock,lock,port}` on exit (crash-safe: on next start, reclaim if lock is free). Document `clean --all` vs default retention.
**Acceptance:** after a crash-kill of the daemon, `bxw clean` leaves only live artifacts; no `test-*` or orphaned agent dirs remain; `run/` holds only files for running daemons.

---

## Verify after fixes
- `go vet ./...` clean; `go test ./... -count=1` green; Linux cross-compile green.
- Re-run `docs/WORKORDER_VERIFICATION_AUDIT.md` §1–§7; all six flags resolved; `git status` clean.
