# WORKORDER — M7: Packaging, CI, and Release (self-sustained)

**Goal.** Make bxw installable and self-maintaining: one command to install, a `windows-latest`
CI that actually compiles/tests the Windows build tags, and a tagged-release pipeline that
cross-builds and publishes to npm (+ winget/scoop). This is what makes the project
"self-sustained" rather than a local binary.

**Depends on:** M0–M3 (a working binary). Can proceed in parallel with M4–M6 since it touches
packaging/CI files, not core logic — low collision risk with the code agent.

---

## Distribution model (mirror bx's verified pattern)
A thin JS launcher + per-platform binary subpackages via `optionalDependencies` (confirmed by
inspecting the published `@facets-cloud/bx`: `bin/bx.js` resolves
`@facets-cloud/bx-${process.platform}-${process.arch}/bin/bx`, subpkgs carry `os`/`cpu`).

### Files
- `npm/bxw/package.json` — launcher: `bin: { "bxw": "bin/bxw.js" }`, `os: ["win32"]`,
  `optionalDependencies: { "@scope/bxw-win32-x64": "<v>", "@scope/bxw-win32-arm64": "<v>" }`.
- `npm/bxw/bin/bxw.js` — resolve `@scope/bxw-win32-${arch}/bin/bx.exe` and spawn with inherited
  stdio. **Critical fix bx would not need:** on Windows the binary is `bx.exe` — resolve the
  `.exe` name (`const binName = process.platform === "win32" ? "bx.exe" : "bx"`), else
  `require.resolve(".../bin/bx")` fails and nothing launches.
- `npm/bxw-win32-x64/package.json`, `npm/bxw-win32-arm64/package.json` — `os:["win32"]`,
  `cpu:["x64"]`/`["arm64"]`, `files:["bin/bx.exe","LICENSE","THIRD_PARTY_NOTICES"]`.
- `scripts/npm-build.sh` — for `win32:amd64` and `win32:arm64`:
  `GOOS=windows GOARCH=<goarch> CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X bxw/internal/app.Version=<v>" -o npm/bxw-win32-<npmarch>/bin/bx.exe .`
  then copy `LICENSE`/`THIRD_PARTY_NOTICES`, and stamp every `npm/*/package.json` version.

## CI — `.github/workflows/ci.yml`
- Matrix: **`windows-latest`** (mandatory — compiles the `//go:build windows` code and runs
  `osport`/daemon tests on the real OS) **+ `ubuntu-latest`** (verifies the `!windows` stub
  keeps the repo cross-compilable).
- Steps: `actions/setup-go` (from `go.mod`), `go vet ./...`, `go test ./...`, `go build ./...`.
- Integration smoke (windows-latest): `go run . launch edge --headless` then assert
  `go run . list tabs edge` returns a page target, then `go run . stop edge`. (Edge ships on
  GitHub windows runners.) Clean up the agent instance after.

## Release — `.github/workflows/release.yml`
Trigger on tag `v*`:
1. **Build job** (ubuntu or windows runner; Go cross-compiles fine): for `amd64`,`arm64`
   `GOOS=windows go build … -o bxw.exe` → zip `bxw_windows_<arch>.zip` (with
   `THIRD_PARTY_NOTICES`) → `shasum -a 256 *.zip > checksums.txt` → `gh release create
   "$GITHUB_REF_NAME" dist/* --generate-notes`.
2. **npm job** (needs build): `scripts/npm-build.sh "$GITHUB_REF_NAME"`; `.npmrc` from
   `secrets.NPM_TOKEN`; **publish subpackages first**, then poll `npm view @scope/bxw-win32-*`
   until served (bx's published workflow waits up to ~15 min — reuse that guard), then publish
   the launcher `@scope/bxw`.
3. **winget + scoop** (what Windows users expect): push a winget manifest (via
   `winget-create`/PR to `microsoft/winget-pkgs`) and a scoop manifest (JSON in a scoop bucket
   repo) pointing at the GitHub release zips + checksums. Chocolatey optional.

## Self-update (ties to M6 `setup.go`)
`bxw setup`/self-update must fetch `bxw_windows_<arch>.zip` from GitHub Releases (the macOS bx
fetches `bx_darwin_%s.tar.gz` — the Windows analogue is the `.zip` above), verify the
checksum, and replace the binary.

## Repo hygiene to add
- `SECURITY.md` (blast radius of an open debug port / pref flip — see M6), `CONTRIBUTING.md`,
  issue/PR templates, a `docs/demo.gif`, README badges (CI, npm version).

## Acceptance gate
- `npx -y @scope/bxw doctor` works on a clean Windows machine (launcher resolves the win32
  binary; `.exe` fix verified).
- Pushing a tag produces a GitHub release with both arch zips + checksums, and publishes all
  three npm packages in the correct order.
- CI is red if the Windows build tags fail to compile (prove it once by breaking a
  `platform_windows.go` symbol in a scratch branch).

## Gotchas / anchors
- **`.exe` resolution** in the launcher is the #1 Windows-specific packaging bug — test it.
- **Build host:** Go cross-compiles Windows binaries from any runner (`CGO_ENABLED=0`), so the
  release build need not run on Windows — but **testing** the Windows tags must (hence the CI
  matrix).
- **Scope/name:** decide the npm scope and module path before first publish (open decision in
  `docs/DESIGN.md` §16); keep `bxw-win32-<arch>` naming parallel to bx's `bx-darwin-<arch>`.
