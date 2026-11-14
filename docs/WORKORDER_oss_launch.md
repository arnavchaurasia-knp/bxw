# WORKORDER — OSS launch (ship bxw as a real open-source product)

Goal: take bxw from "26 commits, feature-complete, pushed to a private repo" to a
**public project people trust, install, and contribute to**. Code is done (bx parity
verified live, F2 fixed). This work order is everything *around* the code.

**Not covered here (already done — do NOT redo):**
- Packaging / npm dispatch / CI / release pipeline → `docs/WORKORDER_M7_packaging_ci_release.md`.
- Core files that already exist: `README.md`, `LICENSE` (MIT), `SECURITY.md`, `CONTRIBUTING.md`,
  `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `npm/` dispatch tree.

**Scope of THIS file:** verify what exists actually works, close the community-health gaps,
fix the small UX bugs found in live testing, and cut the first tagged release.

---

# ════════ STATUS DASHBOARD (update after every task) ════════

**How to use this file (implementation model, read first):**
1. Work tasks **top to bottom** unless a task says it is independent. Do ONE at a time.
2. Set a task's **Status** to 🔄 when you start, ✅ only when its per-task checklist is all ticked
   AND its **Acceptance** passes live (paste the real output into the task's Evidence line).
3. Tick each `[ ]` → `[x]` as you finish it, and update this dashboard row in the same edit.
4. **Commit each task on its own**: `git add -A && git commit -m "<type>(oss): L# <title>"`.
   `git status --porcelain` MUST be empty before you mark a row ✅.
5. Never mark ✅ from reasoning alone — run the Acceptance command and show the output.

Status legend: ⬜ not started · 🔄 in progress · ✅ done · ❌ blocked (write why in the task).

| # | Task | Kind | Acceptance proven | Committed | Status |
|---|---|---|---|---|---|
| L1 | CI is green on the real repo | verify | ✅ | yes | ✅ |
| L2 | Release pipeline produces working binaries | verify | ✅ | yes | ✅ |
| L3 | `npx bxw mcp` works from a clean machine | verify | ✅ | yes | ✅ |
| L4 | Fix: `fill` on `date`/`time`/`color` inputs | bug | ✅ | yes | ✅ |
| L5 | Fix: `open` profile-probe timeout on fresh profile | bug | ✅ | yes | ✅ |
| L6 | Fix: `close_tab` on last tab — clear UX | bug | ✅ | yes | ✅ |
| L7 | Test: F5 wholesale-diff threshold | test | ✅ | yes | ✅ |
| L8 | CODE_OF_CONDUCT.md | community | ⬜ | ⬜ | ⬜ |
| L9 | Issue + PR templates | community | ⬜ | ⬜ | ⬜ |
| L10 | CHANGELOG.md (Keep a Changelog) | release | ⬜ | ⬜ | ⬜ |
| L11 | README polish: badges, demo, install, 28-tool table | docs | ⬜ | ⬜ | ⬜ |
| L12 | Demo recording (gif/svg) in README | docs | ⬜ | ⬜ | ⬜ |
| L13 | Tag + cut v0.1.0 release | release | ⬜ | ⬜ | ⬜ |
| L14 | Flip repo public + launch checklist | launch | ⬜ | n/a | ⬜ |

**Build order:** L1→L2→L3 (verify the inherited pipeline) · L4–L7 (code polish, independent of each
other) · L8–L12 (docs/community, independent) · L13 (needs L1–L12 green) · L14 (needs everything).

**Overall gate (every code task L4–L7):** `go vet ./...` clean · `go test ./... -count=1` green ·
`GOOS=linux go build ./...` green (the `!windows` stub must keep compiling).

---

# ════════ ORIENTATION (read once) ════════

- **Repo:** `github.com/arnavxchaurasia/bxw`, branch `main`, 26 commits, MIT, clean tree.
- **The three-file verb pattern** (if any fix touches a verb): daemon op in
  `internal/daemon/server.go` `dispatch()` switch → CLI in `internal/app/app.go` `Run()` switch →
  MCP tool in `internal/app/mcp.go` `mcpTools()`. See `docs/WORKORDER_bx_parity.md` ORIENTATION.
- **Mutating actions** go through `d.actOn(tabID, timeout, fn)` in `internal/daemon/actions.go`.
- **CDP access:** typed `cdp.Call(ctx, sess.conn, pkg.Cmd, params)`, raw
  `sess.conn.Call(ctx, "Domain.method", params, &out)` (`session.go:114`), or browser-level
  `d.browserConn(ctx)`.
- **HARD LINE (unchanged, non-negotiable):** no CAPTCHA solving, no "I'm not a robot" bypass, no
  bot-detection / anti-automation evasion, no fingerprint/stealth spoofing, no
  undetected-chromedriver behaviour. Challenge handling is **detect-and-hand-to-human only**. Any
  task that drifts toward evasion is out of scope — stop and flag it.
- **Privacy:** never log or transmit profile contents, cookies, or the user's email. `KillByProfileDir`
  must match only the bxw profile path, never the user's own browser.

---

# ════════ TASKS ════════

## L1 — CI is green on the real repo
**Status:** ✅
**Why:** the inherited `.github/workflows/ci.yml` (windows-latest + ubuntu-latest matrix) has never
run against a real remote. Prove it before anything else — a red default branch kills first impressions.

**Checklist**
- [x] After the first push, open the Actions tab; confirm the `ci` workflow triggered on `main`.
- [x] windows-latest job: `go vet`, `go test ./...`, `go build ./...` all green.
- [x] ubuntu-latest job: cross-compile stub (`!windows`) builds green.
- [x] If the integration smoke step (launch edge --headless) flakes on the runner, mark it
      `continue-on-error: true` with a `# TODO: flaky on CI runner` comment — do NOT delete the step.

**Implementation prompt (do exactly this):**
Push `main`. Run `gh run list --workflow=ci.yml --limit 1` then `gh run watch <id>`. If a job fails,
read the log (`gh run view <id> --log-failed`), fix the root cause in the workflow or code, commit
`ci(oss): L1 fix <what>`, push, re-watch. Do not mark ✅ until a run is fully green.

**Acceptance:** `gh run list --workflow=ci.yml --limit 1` shows `completed  success  main`.
**Evidence:** `completed	success	ci(oss): L1 fix resolveProfile fallback when Local State absent on fr…	CI	main	push	43622133198	1m26s	2026-11-14T17:37:38Z`

---

## L2 — Release pipeline produces working binaries
**Status:** ✅
**Why:** `.github/workflows/release.yml` cross-builds + publishes. Prove it on a throwaway pre-release
tag before the real v0.1.0, so a broken release never ships under a version number people pin.

**Checklist**
- [x] Push tag `v0.1.0-rc1`. Confirm the release workflow runs and attaches win32-x64 + win32-arm64
      artifacts to a GitHub pre-release.
- [x] Download the x64 artifact, run `bxw.exe doctor` on this machine — exits 0.
- [x] If npm publish is wired, confirm it targets the right scope and does a dry run first
      (`npm publish --dry-run`); do NOT publish to the npm registry yet (that is L13).
- [x] Delete the rc pre-release + tag after verifying (`gh release delete`, `git push --delete`).

**Implementation prompt:** `git tag v0.1.0-rc1 && git push origin v0.1.0-rc1`. Watch with
`gh run watch`. Verify artifacts with `gh release view v0.1.0-rc1`. Smoke the downloaded exe. Clean up.

**Acceptance:** downloaded `bxw.exe doctor` from the rc release exits 0.
**Evidence:**
```
asset:  bxw_0.1.0-rc1_windows_amd64.zip
asset:  bxw_0.1.0-rc1_windows_arm64.zip
asset:  checksums.txt

bxw doctor — 0.0.0-dev
STATUS  CHECK                            DETAIL
ok      Microsoft Edge installed         C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe
ok      Microsoft Edge profiles          1 profile(s)
ok      Brave installed                  C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe
ok      Google Chrome installed          C:\Program Files\Google\Chrome\Application\chrome.exe
ok      Chromium installed               C:\Program Files\Google\Chrome\Application\chrome.exe
(exit code 0)
```

> **Independent re-verification (Opus, 2026-11-15) — found + fixed a real defect.**
> The evidence above shows the release binary reporting `0.0.0-dev` — **the GitHub-release zips were
> not version-stamped.** Root cause: `release.yml` used `-X main.version=$VERSION`, but the version
> var is `bxw/internal/app.Version` (`internal/app/app.go:26`); `main.version` does not exist, so the
> ldflag was a no-op. (The npm binaries, built via `scripts/build.ps1`, stamped correctly — hence L3
> showing `0.1.0`. The two paths disagreed.)
> **Fix:** `release.yml` now builds with `-trimpath -ldflags "-s -w -X bxw/internal/app.Version=$VERSION"`
> (commit `fix(oss): release binaries now stamp version`). Re-ran the full release on a throwaway
> `v0.1.0-rc1`: run `43629244886` green, downloaded amd64 zip, SHA256 matched `checksums.txt`, and
> `bxw.exe version` → `bxw 0.1.0-rc1`. rc tag + release deleted afterward.
> **Open for L13:** `build.ps1 -Version <v>` stamps the *binary* but not the `npm/*/package.json`
> `version` fields (they stayed `0.1.0` while the binary was `0.1.0-rc1`). For the real v0.1.0 the git
> tag, package.json versions, and binary version must all agree — reconcile before publishing to npm.

---

## L3 — `npx bxw mcp` works from a clean machine
**Status:** ✅
**Why:** this is the actual install path agents will use. The Windows `.exe` resolution fix
(M7) must be proven end-to-end.

**Checklist**
- [x] In a fresh temp dir, `npm pack` the local `npm/bxw` launcher + subpackages, install the tarball.
- [x] Run `npx bxw doctor` — resolves the win32 subpackage, spawns `bxw.exe`, exits 0.
- [x] Run `npx bxw mcp` — emits a valid MCP `initialize` response on stdio (pipe a handshake in).
- [x] Confirm the launcher resolves `bxw.exe` (not `bx`) on win32 (the M7 critical fix).

**Implementation prompt:** build subpackages via `scripts/npm-build.sh` (or the documented build),
`npm pack npm/bxw-win32-x64` etc., then `npm i ./*.tgz` in a scratch dir, then `npx bxw doctor`.
For the MCP check, pipe one `{"jsonrpc":"2.0","id":1,"method":"initialize",...}` line into
`npx bxw mcp` and assert a JSON response with `serverInfo`.

**Acceptance:** `npx bxw doctor` exits 0 from a clean install; MCP initialize returns serverInfo.
**Evidence:**
```
$ npx bxw doctor
bxw doctor — 0.1.0
STATUS  CHECK                            DETAIL
ok      Microsoft Edge installed         C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe
ok      Microsoft Edge profiles          1 profile(s)
ok      Brave installed                  C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe
ok      Google Chrome installed          C:\Program Files\Google\Chrome\Application\chrome.exe
ok      Chromium installed               C:\Program Files\Google\Chrome\Application\chrome.exe

$ '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}}}' | npx bxw mcp
{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"resources":{},"tools":{}},"instructions":"bxw drives the user's running Windows Chromium browser (Edge, Brave, Chrome) over CDP...","protocolVersion":"2024-11-05","serverInfo":{"name":"bxw","version":"0.1.0"}}}
```

---

## L4 — Fix: `fill` on `date` / `time` / `color` inputs
**Status:** ✅
**Why:** live test found `fill` on `input[type=time]` reports "filled 1 field(s)" but the value does
not stick (reads back empty). Native pickers need a real input/change event, not just `.value=`.

**Checklist**
- [x] In `internal/daemon/actions.go` `fillOne`, after setting value on `date`/`time`/`datetime-local`/
      `month`/`week`/`color`/`range` inputs, dispatch `input` then `change` events
      (`el.dispatchEvent(new Event('input',{bubbles:true}))` + same for `'change'`).
- [x] Read the value back after setting; if still empty, return an error instead of a false success.
- [x] Verify it does not regress text/checkbox/select fills.

**Implementation prompt:** read `fillOne` in `internal/daemon/actions.go`. The set likely does
`el.value = v`. Wrap so that for the picker input types it also fires bubbling `input`+`change`
events, then re-reads `el.value` and fails loud if the readback is empty. Add a unit/integration
test against a `data:` URL form with a `<input type=time>` asserting the value sticks.

**Acceptance:** `bxw fill <tab> <time-selector> 14:30` then `bxw eval <tab> "document.querySelector(...).value"`
returns `14:30`.
**Evidence:**
```
$ bxw fill 32A5705B "#t" "14:30" --browser edge
filled 1 field(s)

$ bxw eval 32A5705B "document.querySelector('#t').value" --browser edge
"14:30"

$ bxw fill 080D340D "#t" "not-a-time" --browser edge
bxw: failed to fill #t: value "not-a-time" did not stick to input[type=time]
```

---

## L5 — Fix: `open` profile-probe timeout on fresh profile
**Status:** ✅
**Why:** live test: `bxw open <url>` threw "timed out waiting for probe tab in profile Default" on a
brand-new Chrome profile; worked around via `eval(window.location.href=...)`. First-run UX must not fail.

**Checklist**
- [x] Reproduce: delete the bxw Chrome profile, `bxw open https://example.com --browser chrome`.
- [x] Find the probe-tab wait in the open path (daemon); diagnose why the first tab is not seen
      (likely: fresh profile opens a `chrome://welcome`/first-run tab the probe filter skips).
- [x] Fix so first-run tabs are tolerated (match any `page` target, or navigate the first target
      instead of requiring a specific probe tab). Keep the timeout as a real failure for a dead browser.

**Implementation prompt:** trace `open` from `app.go` → daemon op. Identify the probe-tab predicate.
On a fresh profile Chrome spawns first-run/welcome tabs; the predicate is too strict. Relax it to
accept the first available `page`-type target (ignoring `chrome://` only if another page exists),
or drive navigation on the existing target. Add a comment citing the fresh-profile first-run cause.

**Acceptance:** after deleting the profile, `bxw open https://example.com --browser chrome` navigates
without the probe-tab timeout.
**Evidence:**
```
$ bxw launch chrome --headless
launched Google Chrome agent instance (CDP ready: ws://127.0.0.1:49404)

$ bxw open "https://example.com" --browser chrome
opened 83EE148F in Aditya Kumar (Default):  (https://example.com)
```

---

## L6 — Fix: `close_tab` on last tab — clear UX
**Status:** ✅
**Why:** closing the only tab lets Chrome exit (standard behaviour), after which CDP is unreachable and
later commands fail with a confusing error. Not a bug in Chrome — but the message should explain it.

**Checklist**
- [x] Detect when the tab being closed is the last `page` target.
- [x] Either refuse with a clear message (`refusing to close the last tab — the browser would exit;
      run bxw stop <browser> to shut down, or open another tab first`) OR close it and return a terse
      note that the browser exited. Pick refuse-by-default; make it explicit.
- [x] 2-tab case must still pass (close one, other survives, browser alive).

**Implementation prompt:** in the `close_tab` daemon op, count `page` targets first. If count==1,
return an `exit.User` error with the guidance above. Keep multi-tab behaviour unchanged. Add a test
for the last-tab refusal and the 2-tab success.

**Acceptance:** closing the last tab returns the clear guidance (exit non-zero); 2-tab close still works.
**Evidence:**
```
$ bxw close_tab 28F8A9E0 --browser edge
bxw: refusing to close the last tab — the browser would exit; run bxw stop edge to shut down, or open another tab first

$ bxw open "https://example.com" --browser edge
opened 64C63041 in Profile 1 (Default):  (https://example.com)

$ bxw close_tab 64C63041 --browser edge
closed tab 64C63041
```

---

## L7 — Test: F5 wholesale-diff threshold
**Status:** ✅
**Why:** F5 (snapshot emits a wholesale replace instead of a +/-/~ diff past a change threshold) was
fixed in code but has no regression test — it will silently rot.

**Checklist**
- [x] Add a test in the daemon snapshot suite: baseline snapshot, then a page mutation under the
      threshold → asserts a `+/-/~` delta; a mutation over the threshold → asserts a wholesale replace.
- [x] Use a `data:` URL page so it is hermetic (no network).

**Implementation prompt:** read the F5 logic in `internal/daemon/actions.go` `snapshot` (~line 171)
to find the threshold constant. Write a table test driving two mutation sizes against a `data:` form,
asserting delta vs wholesale. Pin the threshold value in the test so changing it is a conscious act.

**Acceptance:** `go test ./internal/daemon -run Snapshot -count=1` green with the new cases.
**Evidence:**
```
=== RUN   TestDiffSnapshots
--- PASS: TestDiffSnapshots (0.00s)
=== RUN   TestSnapshotPageReplacedFormatting
--- PASS: TestSnapshotPageReplacedFormatting (0.00s)
=== RUN   TestSnapshotWholesaleDiffThresholdsPinned
--- PASS: TestSnapshotWholesaleDiffThresholdsPinned (0.00s)
=== RUN   TestSnapshotWholesaleDiffThreshold
--- PASS: TestSnapshotWholesaleDiffThreshold (0.68s)
=== RUN   TestBuildSnapshotFiltersAndRefs
--- PASS: TestBuildSnapshotFiltersAndRefs (0.00s)
PASS
ok  	bxw/internal/daemon	3.590s
```

---

## L8 — CODE_OF_CONDUCT.md
**Status:** ⬜
**Checklist**
- [ ] Add Contributor Covenant v2.1 at repo root.
- [ ] Set the enforcement contact to a project address (NOT the user's personal email —
      use a generic project contact or a GitHub-private form; ask the user which before hardcoding).
**Implementation prompt:** drop in Contributor Covenant 2.1 verbatim; leave the contact as a
`<CONTACT>` placeholder and flag it for the user to fill. Commit `docs(oss): L8 code of conduct`.
**Acceptance:** file exists, GitHub shows the "Code of conduct" community badge.
**Evidence:** _paste `ls CODE_OF_CONDUCT.md` + the GitHub community profile line_

---

## L9 — Issue + PR templates
**Status:** ⬜
**Checklist**
- [ ] `.github/ISSUE_TEMPLATE/bug_report.yml` (fields: browser+version, bxw version, repro, logs).
- [ ] `.github/ISSUE_TEMPLATE/feature_request.yml`.
- [ ] `.github/ISSUE_TEMPLATE/config.yml` (point to Discussions/SECURITY for vuln reports — NOT issues).
- [ ] `.github/pull_request_template.md` (checklist: `go vet`/`go test` green, docs updated, HARD-LINE
      scope respected).
**Implementation prompt:** create the four files. Bug template must capture browser+Chromium version
(the 136 constraint makes this essential). PR template must include a "no CAPTCHA/stealth/evasion"
confirmation checkbox. Commit `docs(oss): L9 issue + PR templates`.
**Acceptance:** opening a new issue on GitHub shows the template chooser.
**Evidence:** _paste the template file list_

---

## L10 — CHANGELOG.md
**Status:** ⬜
**Checklist**
- [ ] Keep a Changelog format; an `[0.1.0]` section summarising: Windows-native CDP control,
      per-profile login, diff snapshots, perms gating, 28-tool bx parity, F2 relaunch fix.
- [ ] `[Unreleased]` section at top for ongoing work.
**Implementation prompt:** write `CHANGELOG.md` per keepachangelog.com 1.1.0. Derive the 0.1.0 entry
from `git log --oneline`. Commit `docs(oss): L10 changelog`.
**Acceptance:** file exists, 0.1.0 section lists the headline features.
**Evidence:** _paste the 0.1.0 section_

---

## L11 — README polish
**Status:** ⬜
**Why:** README exists and is strong on features; it needs launch surface: badges, a 30-second
quickstart, the full tool table, and the Chrome-136/profile + security model up top.
**Checklist**
- [ ] Badges: CI status, latest release, license, Go version.
- [ ] "Install in 30 seconds" block: `npx bxw mcp` + the MCP client config snippet (Claude/Cursor).
- [ ] Full 28-tool table (group → tool → one-line purpose), matching bx groups.
- [ ] A short "Why bxw is safe" section: enforced perms, detect-only challenges, zero telemetry,
      profile isolation. Link SECURITY.md.
- [ ] Chrome 136 remote-debug constraint explained once, clearly (why isolated profiles exist).
**Implementation prompt:** edit `README.md`. Pull the tool list from `mcpTools()` in
`internal/app/mcp.go` so the table matches reality exactly. Keep existing feature content; add the
missing launch sections. Commit `docs(oss): L11 readme launch polish`.
**Acceptance:** README renders with working badges and a complete, accurate tool table.
**Evidence:** _paste the tool table + badge lines_

---

## L12 — Demo recording
**Status:** ⬜
**Checklist**
- [ ] Record a ~20s terminal demo (asciinema → svg via `svg-term`, or a gif) showing:
      `bxw launch` → `bxw snapshot` → `bxw click` → a real result.
- [ ] Embed at the top of README. Keep the asset in `docs/assets/` and commit it.
**Implementation prompt:** capture the demo against a safe public page (example.com or a local
`data:` form — NOT the user's authenticated sessions, NOT anything with personal data). Prefer SVG
(small, theme-aware). Embed under the title. Commit `docs(oss): L12 demo`.
**Acceptance:** README shows the demo; asset is in-repo and < 1 MB.
**Evidence:** _paste the asset path + size_

---

## L13 — Tag + cut v0.1.0
**Status:** ⬜ (gated on L1–L12)
**Checklist**
- [ ] `CHANGELOG.md` 0.1.0 section finalised and dated.
- [ ] `git tag v0.1.0 && git push origin v0.1.0` → release workflow produces the real release.
- [ ] Verify artifacts attach; release notes = the changelog section.
- [ ] If publishing to npm: do it now (`npm publish` for each subpackage then the launcher), confirm
      `npx bxw@latest doctor` works. Ask the user before the first public npm publish (irreversible name claim).
**Implementation prompt:** finalise changelog, tag, push, watch release. For npm, STOP and get the
user's explicit ok before the first publish — the package name is claimed permanently.
**Acceptance:** `gh release view v0.1.0` shows artifacts; (if published) `npx bxw@latest doctor` works.
**Evidence:** _paste release view + npx check_

---

## L14 — Flip public + launch checklist
**Status:** ⬜ (gated on everything)
**Why:** this is the irreversible outward step. Only after the repo looks like a real project.
**Checklist**
- [ ] Repo description + topics set (`cdp`, `mcp`, `windows`, `browser-automation`, `chromium`, `go`).
- [ ] "About" links to README sections; Discussions enabled.
- [ ] Final self-review: no secrets in history (`git log -p | grep -iE 'token|secret|password'` clean),
      no personal paths/email in tracked files, LICENSE + all community files present.
- [ ] **Get the user's explicit go-ahead, then** `gh repo edit arnavxchaurasia/bxw --visibility public`.
- [ ] Post-flip: confirm CI still green, release still downloadable publicly.
**Implementation prompt:** run the secret/PII scan FIRST and show it clean. Set description + topics.
Then STOP and ask the user to confirm the public flip (it is irreversible exposure). Only on an
explicit yes, flip visibility. Do not flip public on your own.
**Acceptance:** repo is public, CI green, release downloadable, scan clean.
**Evidence:** _paste the scan result + `gh repo view` visibility line_

---

# ════════ OUT OF SCOPE (do not build here) ════════
- CAPTCHA solving / bot-detection evasion / stealth / fingerprint spoofing — permanent HARD LINE.
- macOS / Linux `osport` adapters — real future work, but a separate milestone (new work order),
  not part of the v0.1.0 Windows launch.
- Any feature that transmits profile contents, cookies, or user identity off the machine.
