# WORKORDER — M5: Profile ↔ CDP Context Correlation

**Goal.** Target a tab in a **named** profile (`bxw open <url> --profile Work`). CDP cannot
open a tab in a chosen profile and `Target.targetCreated` reports only an opaque
`BrowserContextID`; correlate that id to a human profile folder via a nonce probe.

**Depends on:** M3 (daemon + CDP). For the *real* multi-profile browser this also depends on
**M6** (real-profile attach) — against a bxw agent instance there is only one profile, so M5
is most useful once M6 lands. Build the mechanism now; wire the real path when M6 is ready.

---

## Contracts this builds on
- `browser.Browser.WSURL()`, `browser.Profiles(b)` → `[]Profile{Dir, Name}` (folder→name),
  `osport.Platform.SpawnBrowser(exe, args)` / `SpawnDetached`.
- Profile launch flag: `--profile-directory=<folder>` (folder = `Profile.Dir`, e.g. `Default`,
  `Profile 1` — verified A6; "Work" is Brave's `Default` on this box).
- CDP: `Target.getTargets` / `Target.createTarget` / events `Target.targetCreated`
  (`cdproto/target`), `Target.getBrowserContexts`.

## The problem (verified reality)
- `Target.createTarget` opens tabs only in the currently focused window/context.
- `Target.targetCreated` carries `BrowserContextID` (opaque UUID, regenerated when windows
  close/reopen) — **not** a profile path.
- `Local State.profile.info_cache` maps folder→display name (M0), but nothing maps
  `BrowserContextID`→folder at runtime. The nonce probe bridges that gap.

## Files
- `internal/daemon/profile.go` — NEW: `probeProfile`, `profileOf map[cdp.BrowserContextID]string`
  on `Daemon`, `openInProfile`.
- `internal/daemon/server.go` — add `open` op (+ `--profile` arg) and the `profileOf` field.
- `internal/app/app.go` — `bxw open <url> [--profile <name>] [browser]`.

## Design / steps
1. **Resolve profile name→folder.** `--profile Work` → look up `browser.Profiles(b)` for the
   `Dir` whose `Name == "Work"` (case-insensitive); fall back to treating the arg as a folder.
2. **Launch the profile window** (real browser path, M6): `SpawnBrowser(exe,
   ["--profile-directory="+folder, url])`. This brings the named profile's window to the front.
3. **Nonce probe correlation.** When the profile↔context map is unknown for `folder`:
   - subscribe to `Target.targetCreated`;
   - open `data:text/html,<title>bxw</title><!--bxwprobe-<nonce>-->` into that profile window;
   - catch the new target whose URL contains `bxwprobe-<nonce>`, record
     `profileOf[ev.TargetInfo.BrowserContextID] = folder`;
   - close the probe target (`Target.closeTarget`).
   (The `/json/list` filter already drops `bxprobe-` URLs — keep it consistent; use
   `bxwprobe-` and update the filter in `cdp.go:listTabs` to match the new prefix.)
4. **open(url, profile).** snapshot current targets, launch the URL into the correlated
   `BrowserContextID` (or via `--profile-directory` + wait for the new target in that context),
   return the new tab id.
5. **list tabs --profile <name>** (optional): filter `/json/list` targets by
   `profileOf[ctxID] == folder`.

## Acceptance gate
- `bxw open https://example.com --profile Work` opens example.com in the **Work** profile
  window (verify the window's profile, not a different one).
- `profileOf` correctly maps the probe's context to `Default`/"Work" on this machine.
- Nonce probe target is closed afterward (no leftover `bxwprobe-` tabs; not shown in
  `list tabs`).

## Gotchas / anchors
- **Prefix consistency:** the repo currently filters `bxprobe-`; standardize on **`bxwprobe-`**
  in both the probe URL and the `listTabs` filter.
- **Context volatility:** `BrowserContextID` changes when the profile's windows all close —
  re-probe lazily when a cached id yields no live target.
- **Agent instances have one profile** — M5's value is realized against the real browser (M6).

## Out of scope
Opening *new* OS-level profiles; profile creation; cross-profile data access.
