# WORKORDER — release readiness (single source of truth)

One place to **track every defect found during verification, its fix, and the test that
guards it** — plus the two remaining launch steps that need a human decision. If you only
read one doc before cutting a release, read this one.

- Detailed per-task prompts for the launch tasks (L1–L14) live in
  [`WORKORDER_oss_launch.md`](WORKORDER_oss_launch.md). This file is the consolidated ledger.
- **Rule:** nothing is ✅ here without a *test or a live command* proving it, named in the
  "Guard / proof" column. "It should work" is not a status.

Legend: ✅ done + guarded · ⏸ blocked on user decision · ⬜ open

---

# ════════ A. DEFECTS FOUND DURING VERIFICATION (all fixed + guarded) ════════

| # | Defect | Fix | Guard / proof | Status |
|---|---|---|---|---|
| V1 | Release zips shipped as `0.0.0-dev` — `release.yml` stamped `-X main.version` (var doesn't exist) | `release.yml` → `-X bxw/internal/app.Version` (+`-trimpath`) | CI job **Version Stamp Smoke** (`ci.yml`) builds with a sentinel and asserts `bxw version` echoes it | ✅ |
| V2 | `build.ps1` stamped the binary but **not** `npm/*/package.json` — launcher/subpackages drifted (binary `0.1.0-rc1` vs package.json `0.1.0`) | `build.ps1` now stamps launcher version, its `optionalDependencies` pins, and both subpackage versions | **`TestNpmVersionsConsistent`** (`release_test.go`, in `go test ./...`) | ✅ |
| V3 | My own V2 fix wrote UTF-8 **with BOM** (PS 5.1 `Set-Content -Encoding utf8`) → invalid JSON (`'ï'`) | write via `UTF8Encoding($false)` (BOM-less) | same test (parse would fail on BOM) + byte-check: `npm/` is byte-identical after a 0.1.0 re-stamp | ✅ |
| V4 | README clone/release URLs said `arnavchaurasia/bxw` (wrong account) | corrected to `arnavxchaurasia/bxw` | `grep -rn "arnavchaurasia/bxw" README.md` → empty | ✅ |
| V5 | README MCP configs used a phantom `["mcp","serve"]` arg | `["-y","bxw","mcp"]` (server runs via `bxw mcp`; `app.go:97`, `mcp.go:43`) | `grep -n '"serve"' README.md` → empty; live MCP handshake returned 28 tools | ✅ |
| V6 | README said "10 core tools" | "28 tools (bx v0.4.1 parity)" | live `tools/list` = 28 | ✅ |
| V7 | README quickstart `bxw snapshot edge` / `click e3 edge` — those verbs take a **tab ID** first, so the examples fail | rewrote quickstart to the real flow (`snapshot <tab>`, `click <tab> <ref>`) | live: `bxw snapshot F0DBC61D` returns the AXTree | ✅ |

**Earlier code fixes (prior turns, already guarded by tests) — listed for completeness:**

| # | Fix | Guard / proof | Status |
|---|---|---|---|
| F2 | CDP singleton collision on relaunch (reuse-on-launch + kill-on-stop, bxw-profile-scoped) | `actions_test.go` close/relaunch cases; live 10/10 Edge, 5/5 Brave | ✅ |
| L4 | `fill` on picker inputs dispatches `input`+`change`, fails loud on empty readback | `actions.go:437-460` + fill test | ✅ |
| L5 | `open` tolerates fresh-profile first-run tabs | `profile.go:156-162` | ✅ |
| L6 | `close_tab` refuses the last tab with guidance | `server.go:224-226` + `actions_test.go:592` | ✅ |
| L7 | snapshot wholesale-diff threshold | `TestSnapshotWholesaleDiffThreshold(+Pinned)` `diff_test.go` | ✅ |

---

# ════════ B. REMAINING LAUNCH TASKS (blocked on user) ════════

| # | Task | Why blocked | Guard / proof when done |
|---|---|---|---|
| L13 | Tag + cut **v0.1.0** release | `npm publish` is a permanent package-name claim — needs explicit go-ahead | `gh release view v0.1.0` shows 3 assets; `npx bxw@latest version` = `0.1.0`; version-consistency test green |
| L14 | Flip repo **public** | irreversible exposure — needs explicit yes + clean secret/PII scan | `gh repo view` shows PUBLIC; secret scan clean; CI green on public |

### L13 runbook (do NOT run npm publish without explicit ok)
1. Finalise `CHANGELOG.md` `[0.1.0]` date (already 2026-11-15 — confirm).
2. `pwsh scripts/build.ps1 -Version 0.1.0` → stamps binary + all package.json to `0.1.0`.
3. `go test ./... -count=1` → `TestNpmVersionsConsistent` green.
4. `git tag v0.1.0 && git push origin v0.1.0` → release workflow builds + attaches zips/checksums.
5. `gh release view v0.1.0` → confirm 3 assets; download amd64, `bxw.exe version` → `0.1.0`.
6. **STOP.** Get explicit ok, then `npm publish npm/bxw-win32-x64 && npm publish npm/bxw-win32-arm64 && npm publish npm/bxw` (each `--access public`). Then `npx bxw@latest version`.

### L14 runbook (do NOT flip without explicit yes)
1. `git log -p | grep -iE 'token|secret|password|arnav\.chaurasia@'` → must be empty.
2. Confirm no personal email / absolute user paths in tracked files.
3. Set repo description + topics (`cdp`, `mcp`, `windows`, `browser-automation`, `chromium`, `go`); enable Discussions.
4. **STOP.** On explicit yes: `gh repo edit arnavxchaurasia/bxw --visibility public`.
5. Confirm CI green + release downloadable while public.

---

# ════════ C. FULL VERIFICATION RUNBOOK (run before any release) ════════

```bash
# 1. Static + tests (includes version-consistency + all daemon/action/diff tests)
go vet ./...
go test ./... -count=1
GOOS=linux go build ./...          # the !windows stub must keep compiling

# 2. Version stamping actually works (the V1 bug class)
go build -ldflags "-X bxw/internal/app.Version=probe" -o bxw_probe.exe .
./bxw_probe.exe version            # must print: bxw probe
rm -f bxw_probe.exe

# 3. npm packaging stamps consistently (the V2/V3 bug class)
pwsh scripts/build.ps1 -Version 0.1.0
git diff --stat -- npm/            # must be empty (reproducible via -trimpath)

# 4. Docs sanity
grep -rn "arnavchaurasia/bxw" README.md   # must be empty (V4)
grep -n '"serve"' README.md               # must be empty (V5)
```

CI (`ci.yml`, windows + ubuntu) runs vet, `go test`, build, the **Version Stamp Smoke**,
and a doctor/list-browsers smoke on every push + PR. The release pipeline (`release.yml`)
re-runs tests, builds both arches, and attaches checksummed zips on any `v*` tag.

---

# ════════ OUT OF SCOPE (permanent) ════════
- CAPTCHA solving / bot-detection evasion / stealth / fingerprint spoofing. Detect-and-hand-to-human only.
- macOS / Linux `osport` adapters — separate future milestone, not part of v0.1.0.
