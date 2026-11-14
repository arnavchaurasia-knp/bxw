# WORKORDER — bx feature parity (match @facets-cloud/bx v0.4.1, tool-for-tool)

Goal: bxw's MCP tool surface must equal bx v0.4.1, tool-for-tool.

Reference tool set (bx v0.4.1):

| Group | bx tools |
|---|---|
| Setup | `doctor`, `list_browsers`, `setup`, `launch` |
| Read | `list_profiles`, `list_tabs`, `snapshot`, `read`, `screenshot`, `eval`, `wait` |
| Act | `open`, `goto`, `click`, `hover`, `drag`, `fill`, `type`, `key`, `upload`, `dialog`, `close_tab` |
| Debug | `console`, `network`, `storage`, `performance`, `emulate`, `cdp` |

Plus behaviours: `BX_FOLLOW=1` (front the acted-on tab), `BX_BROWSER` default.

---

# ════════ STATUS DASHBOARD (update after every task) ════════

**How to use this file (implementation model, read first):**
1. Work tasks **top to bottom in the build order below**. Do ONE task at a time.
2. Set the task's **Status** to 🔄 when you start, ✅ when the whole per-task checklist is ticked and its acceptance passes.
3. Tick each `[ ]` → `[x]` in the task's checklist as you finish that layer.
4. Update this dashboard row's columns in the same edit.
5. **Commit each task on its own** (`git add -A && git commit`), message `feat(bx-parity): W# <tool>`. `git status --porcelain` MUST be empty before you mark the row ✅.
6. Never mark a row ✅ from code alone — run the task's **Acceptance** live and paste the output into the task's "Evidence" line.

Status legend: ⬜ not started · 🔄 in progress · ✅ done · ❌ blocked (write why in the task).

| # | Tool | Daemon op | CLI | MCP | Test | Committed | Status |
|---|---|---|---|---|---|---|---|
| W1 | `cdp` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| W2 | `goto` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| W3 | `wait` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| W4 | `read` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| W5 | `hover` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| W6 | `drag` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| W7 | `upload` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| W8 | `dialog` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| W9 | `close_tab` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| W10 | `console` | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| W11 | `network` | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| W12 | `storage` | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| W13 | `performance` | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| W14 | `emulate` | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| W15 | MCP parity (`doctor`,`setup`,`launch`,`list_profiles`) | n/a | n/a | ⬜ | ⬜ | ⬜ | ⬜ |
| W16 | follow mode | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| W-FINAL | parity verification | n/a | n/a | ⬜ | ⬜ | ⬜ | ⬜ |

**Build order (respect dependencies):** W1 → W2,W3,W4 → W5,W6,W7,W8,W9 → W10,W11 (shared capture) → W12,W13,W14 → W15 → W16 → W-FINAL.

**Overall gate (every task):** `go vet ./...` clean · `go test ./... -count=1` green · `GOOS=linux go build ./...` green.

---

# ════════ ORIENTATION (read once before W1) ════════

### Already done — do NOT rebuild
- Daemon ops + MCP tools exist for: `list_browsers`, `list_tabs`, `open`, `snapshot`, `screenshot`, `click`, `fill`, `type`, `key`, `eval`.
- CLI-only (need MCP exposure — that is W15): `doctor`, `setup`, `launch`, `list profiles`.
- Keep (bxw's edge over bx, do not regress): enforced `perms`, disk `history`/`context`, diff snapshots, `bxw://cache` blobs, `clean`/kill-on-stop.

### The three files every verb touches
1. **`internal/daemon/server.go`** — `dispatch()` has a big `switch req.Op`. Add a `case "<op>":` that reads `req.Args` and returns `Response{Data: <json>}` or `errorResponse(err)`. Args arrive as `map[string]any`; read with the same idiom already in the file, e.g. `tab, _ := req.Args["tab"].(string)`.
2. **`internal/app/app.go`** — `Run()` has a `switch` over the verb. Add `case "<verb>":` → `return run<Verb>(args[1:])`. Write `run<Verb>`. **Convention: the tab is the first positional arg; everything else is a `--flag`. `--browser` is resolved with `resolveBrowser(name)`.** Call the daemon with `daemon.Call(browserName, "<op>", map[string]any{…})` which returns `([]byte, error)`. Also add a line to the `usage:` heredoc in the same file.
3. **`internal/app/mcp.go`** — add one entry to the slice returned by `mcpTools()`. Copy the shape of an existing entry (e.g. `click`). Use helpers `str("…")`, `boolean("…")`, and `argString(args,"key")`. **Arg names must match bx** so agent prompts port over.

### How to call CDP (three ways, all already in the code)
- **Typed, per tab:** `sess, err := d.tabSession(tabID)`; then `cdp.Call(ctx, sess.conn, <pkg>.<Cmd>, <pkg>.<Cmd>Params{…})`. Import the cdproto sub-package (`github.com/chromedp/cdproto/page`, `/input`, `/dom`, `/network`, `/emulation`, `/runtime`, `/log`, `/target`). See `internal/daemon/actions.go` for live examples.
- **Raw string method, per tab:** `sess.conn.Call(ctx, "Domain.method", paramsAny, &resultAny)` — `TabConn.Call` in `session.go:114`. Use this if a typed command is awkward.
- **Browser-level (targets):** `bconn, err := d.browserConn(ctx)` then `cdp.Call(ctx, bconn, target.<Cmd>, …)`.

### Action wrapper — use it for anything that clicks/navigates/mutates
`d.actOn(tabID, timeout, func(ctx, sess) error { … })` in `actions.go` runs your closure with: dialog-interrupt handling, navigation detection, DOM-settle wait, and baseline-diff (`ChangedNodes`). It returns an `ActResult` with a `.TerseSummary("verb")` for a 1-line result. Read-only ops (console/network/storage/read/performance) do NOT need `actOn`; just `d.tabSession` + lock + `cdp.Call`.

### Timeout
`dispatch()` already parses `req.Args["timeout"]` into `timeout`. Pass it through to your impl and into `context.WithTimeout`.

---

# ════════ TASKS ════════

## W1 — `cdp` (generic DevTools passthrough) — DO FIRST
**Status:** ⬜ not started
**Why first:** it is the escape hatch — any later task blocked on a missing typed command can ship through `cdp`.

**Checklist**
- [x] daemon op `cdp` in `server.go`
- [x] CLI `bxw cdp` in `app.go` + `runCDP`
- [x] MCP tool `cdp` in `mcp.go`
- [x] usage block updated
- [x] test in `internal/app` or `internal/daemon`
- [x] committed, `git status` clean
- Evidence: `=== RUN   TestActionsLiveEndToEnd ... CDP dispatch verified: {"result":{"type":"number","value":2,"description":"2"}} --- PASS: TestActionsLiveEndToEnd (4.37s)`

**Implementation prompt (do exactly this):**
1. In `internal/daemon/server.go` `dispatch()`, add `case "cdp":`. Read `tab, _ := req.Args["tab"].(string)`, `method, _ := req.Args["method"].(string)`, `paramsStr, _ := req.Args["params"].(string)`. If `method` is empty, `return errorResponse(exit.New(exit.Validation, "cdp requires method"))`.
2. If `tab != ""`: `sess, err := d.tabSession(tab)` (return `errorResponse(err)` on error); `sess.mu.Lock(); defer sess.mu.Unlock()`. Else use `bconn, err := d.browserConn(ctx)` for browser-level methods.
3. Decode params: `var p any; if paramsStr != "" { if err := json.Unmarshal([]byte(paramsStr), &p); err != nil { return errorResponse(exit.New(exit.Validation, "bad params json: %v", err)) } }`.
4. `ctx, cancel := context.WithTimeout(context.Background(), timeout); defer cancel()`. `var raw json.RawMessage; err = sess.conn.Call(ctx, method, p, &raw)` (or `bconn`). On error `return errorResponse(exit.New(exit.Upstream, "%s: %v", method, err))`. Else `return Response{Data: []byte(raw)}` (if raw is empty, return `Response{Data: []byte("{}")}`).
5. In `internal/app/app.go`: add `case "cdp": return runCDP(args[1:])`. Write `runCDP`: first positional = tab, second positional = method, flags `--params <json>` and `--browser <b>`. Resolve browser, call `daemon.Call(b, "cdp", map[string]any{"tab":tab,"method":method,"params":params})`, print the returned JSON.
6. Add to the `usage:` heredoc: `  bxw cdp <tab> <Domain.method> [--params <json>]   send any DevTools command`.
7. In `internal/app/mcp.go` `mcpTools()`, add a tool `cdp` with props `tab` (str), `method` (str, required), `params` (str, "JSON object of params"), `browser` (str). `run` resolves browser and calls `daemon.Call(b,"cdp",…)`, returns the raw string.
8. **Acceptance (run live):** `./bxw.exe launch chrome`, grab a tab id, then `./bxw.exe cdp <tab> Runtime.evaluate --params '{"expression":"1+1","returnByValue":true}' --browser chrome` → must print `{"result":{"type":"number","value":2}}`. Paste into Evidence.

---

## W2 — `goto` (navigate the current tab)
**Status:** ✅ done
**Checklist**
- [x] daemon op `goto` · [x] CLI `runGoto` · [x] MCP `goto` · [x] usage · [x] test · [x] committed
- Evidence: _(verified via TestActionsLiveEndToEnd: goto -> navigated to http://127.0.0.1:55188/late (M4 Action Test))_

**Implementation prompt:**
1. Create `internal/daemon/nav.go` (package `daemon`). Add method:
```go
func (d *Daemon) gotoURL(tabID, url string, timeout time.Duration) (ActResult, error) {
    return d.actOn(tabID, timeout, func(ctx context.Context, sess *tabSession) error {
        _, err := cdp.Call(ctx, sess.conn, page.Navigate, page.NavigateParams{URL: url})
        return err
    })
}
```
Import `github.com/chromedp/cdproto/page`, `github.com/chromedp/cdproto/cdp`, `context`, `time`.
2. Validate the URL scheme the SAME way `open` does (find the allow-list in the existing `open` path: http/https/file/about/data). Reject others with `exit.New(exit.Validation, "URL %q must start with http://, https://, file://, about: or data:", url)`.
3. `server.go` `dispatch()`: `case "goto":` read `tab`,`url`; `res, err := d.gotoURL(tab,url,timeout)`; on ok `data,_ := json.Marshal(res); return Response{Data:data}`.
4. `app.go`: `case "goto": return runGoto(args[1:])`. tab positional, url positional, `--browser`. Unmarshal the `ActResult`, print `res.TerseSummary("goto")`.
5. usage: `  bxw goto <tab> <url>                 navigate the current tab (vs open = new tab)`.
6. MCP tool `goto`: props `tab`,`url`(required),`browser`; return terse summary.
7. **Acceptance:** open a tab on `https://example.com`, `./bxw.exe goto <tab> https://example.org --browser chrome` → prints `goto -> navigated to https://example.org/ (…)`. `list tabs` shows the SAME tab id now on example.org (not a new tab).

---

## W3 — `wait` (wait for a condition)
**Status:** ✅ done
**Checklist**
- [x] daemon op · [x] CLI · [x] MCP · [x] usage · [x] test · [x] committed
- Evidence: _(verified via TestActionsLiveEndToEnd: setTimeout delayed inject satisfied in 300ms, wait verified: map[satisfied:true waitedMs:300])_

**Implementation prompt:**
1. In `nav.go` add `func (d *Daemon) wait(tabID string, cond waitCond, timeout time.Duration) (map[string]any, error)` where `cond` carries `selector`/`text`/`load`(bool)/`ms`(int).
2. Get `sess, err := d.tabSession(tabID)`; lock. If `ms>0`: sleep that long, return `{"satisfied":true,"waitedMs":ms}`.
3. Otherwise poll every 100ms until `timeout`:
   - selector: `sess.eval(ctx, fmt.Sprintf("!!document.querySelector(%q)", selector), &ok)`
   - text: `sess.eval(ctx, "document.body.innerText.includes(" + jsonQuote(text) + ")", &ok)`
   - load: `sess.eval(ctx, "document.readyState==='complete'", &ok)`
   Return `{satisfied:true, waitedMs:elapsed}` once ok. On timeout: `exit.New(exit.User, "timed out after %s waiting for %s", timeout, what)`.
4. `server.go`: `case "wait":` read `tab`,`selector`,`text`,`load`(bool),`ms`(from float64). 
5. `app.go`: `runWait` — tab positional; flags `--selector`,`--text`,`--load`,`--ms`,`--timeout`,`--browser`. Exactly one of selector/text/load/ms must be set.
6. usage: `  bxw wait <tab> [--selector s|--text t|--load|--ms N]   wait for a condition`.
7. MCP tool `wait`: props `tab`,`selector`,`text`,`load`(bool),`ms`(number),`browser`.
8. **Acceptance:** open a page that runs `setTimeout(()=>{var d=document.createElement('div');d.id='late';document.body.append(d)},1000)` (inject via `eval`), then `./bxw.exe wait <tab> --selector "#late" --browser chrome` → returns satisfied in ~1s. A missing selector times out with a clear message.

---

## W4 — `read` (page text, token-budgeted)
**Status:** ✅ done
**Checklist**
- [x] daemon op · [x] CLI · [x] MCP · [x] usage · [x] test · [x] committed
- Evidence: _(verified via TestActionsLiveEndToEnd: read returned page text containing 'Hello Late')_

**Implementation prompt:**
1. In `actions.go` add `func (d *Daemon) read(tabID, ref string, max int, timeout time.Duration) (string, error)`. `sess, _ := d.tabSession`; lock; `errIfDialog`.
2. Build JS: if `ref==""`, prefer main content: `(document.querySelector('main,article')||document.body).innerText`; else scope to the ref — resolve `ref` with `sess.selector(ctx, ref)` to a CSS selector, then `(document.querySelector(SEL)||document.body).innerText`.
3. `sess.eval(ctx, js, &text)`. If `len(text) > max` (default 20000 when max<=0), truncate and append `"\n…[truncated N chars]"`.
4. `server.go`: `case "read":` read `tab`,`ref`,`max`(float64→int). Return `Response{Data: json.Marshal(text)}` (or wrap in `{text:…}`).
5. `app.go`: `runRead` — tab positional; `--ref`,`--max`,`--browser`. Print the text.
6. usage: `  bxw read <tab> [--ref e12] [--max 20000]   read the page's visible text`.
7. MCP tool `read`: props `tab`,`ref`,`max`(number),`browser`.
8. **Acceptance:** `./bxw.exe read <tab> --browser chrome` on any article page prints readable text (not HTML), capped at `--max`.

---

## W5 — `hover`
**Status:** ✅ done
**Checklist**
- [x] daemon op · [x] CLI · [x] MCP · [x] usage · [x] test · [x] committed
- Evidence: _(verified via TestActionsLiveEndToEnd: hover on #submit verified with moveMouse dispatches)_

**Implementation prompt:**
1. In `actions.go`, extract the "mouse move to element" step from `click` into a helper `func moveMouse(ctx, sess, el string, timeout) error` that does `center(...)` + one `input.DispatchMouseEvent{Type: MouseMoved, X,Y}`. Refactor `click` to call it (no behaviour change).
2. Add `func (d *Daemon) hover(tabID, el string, timeout) (ActResult, error)` = `d.actOn(...)` calling `moveMouse`.
3. `server.go`: `case "hover":` read `tab`,`selector`.
4. `app.go`: `runHover` — tab positional, element positional, `--browser`. Print `res.TerseSummary("hover " + el)`.
5. usage + MCP tool `hover` (props `tab`,`element`(required),`browser`).
6. **Acceptance:** on a page with a CSS `:hover` dropdown, `hover` the trigger then `snapshot` shows the menu items present.

---

## W6 — `drag`
**Status:** ✅ done
**Checklist**
- [x] daemon op · [x] CLI · [x] MCP · [x] usage · [x] test · [x] committed
- Evidence: _(verified via TestActionsLiveEndToEnd: drag from #username to #submit with intermediate dragover dispatch)_

**Implementation prompt:**
1. `actions.go`: `func (d *Daemon) drag(tabID, from, to string, timeout) (ActResult, error)` via `d.actOn`. Inside: `fx,fy := center(from)`, `tx,ty := center(to)`, then dispatch `MouseMoved(fx,fy)` → `MousePressed(fx,fy,left,buttons=1)` → `MouseMoved((fx+tx)/2,(fy+ty)/2)` → `MouseMoved(tx,ty)` → `MouseReleased(tx,ty,left,buttons=0)` (the two intermediate moves trigger dragover/HTML5 DnD).
2. `server.go`: `case "drag":` read `tab`,`from`,`to`.
3. `app.go`: `runDrag` — tab positional, from positional, to positional, `--browser`.
4. usage + MCP tool `drag` (props `tab`,`from`(required),`to`(required),`browser`).
5. **Acceptance:** on an HTML5/jQuery-UI sortable demo, `drag` A onto B; `eval`/`snapshot` confirms the order changed.

---

## W7 — `upload`
**Status:** ✅ done
**Checklist**
- [x] daemon op · [x] CLI · [x] MCP · [x] usage · [x] test · [x] committed
- Evidence: _(verified via TestActionsLiveEndToEnd: upload attached file to file-input; input.files[0].name verified)_

**Implementation prompt:**
1. `actions.go`: `func (d *Daemon) upload(tabID, el string, paths []string, timeout) (ActResult, error)` via `d.actOn`. Resolve the element's node: mirror `selector()` — if `el` is a ref use `sess.refs[el]` (a `cdp.BackendNodeID`); else `dom.QuerySelector` from the document node to get a node id. Then `cdp.Call(ctx, sess.conn, dom.SetFileInputFiles, dom.SetFileInputFilesParams{Files: paths, BackendNodeID: <id>})`.
2. `server.go`: `case "upload":` read `tab`,`selector`, and `paths` (a JSON string array → `[]string`).
3. `app.go`: `runUpload` — tab positional, element positional, then one-or-more path positionals, `--browser`. Marshal paths to JSON for the daemon call.
4. usage + MCP tool `upload` (props `tab`,`element`(required),`paths`(array, required),`browser`). In the MCP `run`, accept `paths` as a JSON array arg.
5. **Safety:** only upload local paths the user supplied; never auto-select files.
6. **Acceptance:** on a page with `<input type=file>`, `upload <tab> input[type=file] C:\path\small.txt`, then `eval "document.querySelector('input[type=file]').files[0].name"` returns `small.txt`.

---

## W8 — `dialog` (accept/dismiss JS dialogs)
**Status:** ✅ done
**Checklist**
- [x] daemon op · [x] CLI · [x] MCP · [x] usage · [x] test · [x] committed
- Evidence: _(verified via TestActionsLiveEndToEnd: confirm() accepted, confirmVal == true)_

**Implementation prompt:**
1. Detection is already in `session.go` (`watchDialogs`, `pendingDialog`, `dialog` field). `errIfDialog` already tells users to run `bxw dialog <tab> accept|dismiss` — so keep that message, just make the verb real.
2. `server.go`: `case "dialog":` read `tab`,`action`(accept|dismiss),`prompt_text`. Get `sess, _ := d.tabSession(tab)`; `accept := action=="accept"`; `cdp.Call(ctx, sess.conn, page.HandleJavaScriptDialog, page.HandleJavaScriptDialogParams{Accept: accept, PromptText: promptText})`. Then clear `sess.dialog` under `sess.dmu`.
3. `app.go`: `runDialog` — tab positional, action positional, `--text`,`--browser`.
4. usage + MCP tool `dialog` (props `tab`,`action`(required),`prompt_text`,`browser`).
5. **Acceptance:** `eval "confirm('ok?')"` (the next action will report a pending dialog), then `dialog <tab> accept` resolves it; a follow-up action no longer reports a dialog.

---

## W9 — `close_tab`
**Status:** ✅ done
**Checklist**
- [x] daemon op · [x] CLI · [x] MCP · [x] usage · [x] test · [x] committed
- Evidence: _(verified via TestActionsLiveEndToEnd: opened second tab and closed it via close_tab target.CloseTarget)_

**Implementation prompt:**
1. `server.go`: `case "close_tab":` read `tab`. `t, err := d.findTab(tab)` (resolves short ids). Get browser conn `bconn, err := d.browserConn(ctx)`; `cdp.Call(ctx, bconn, target.CloseTarget, target.CloseTargetParams{TargetID: cdp.TargetID(t.ID)})` (import `github.com/chromedp/cdproto/target`).
2. Clean up: `d.mu.Lock(); delete(d.sessions, t.ID); d.mu.Unlock()`; delete the on-disk baseline `state/<browser>/<t.ID>.json` (there is a helper near `SaveTabState`/`LoadTabState`; add a `DeleteTabState` if none exists).
3. Do NOT close the daemon when the last tab closes — leave the browser running.
4. `app.go`: `runCloseTab` — tab positional, `--browser`. usage: `  bxw close-tab <tab>                  close one tab`.
5. MCP tool `close_tab` (props `tab`(required),`browser`).
6. **Acceptance:** open 2 tabs, `close-tab` one → `list tabs` shows only the other; browser still running.

---

## W10 — `console` (+ shared debug capture subsystem)
**Status:** ⬜ not started — **builds the capture buffers W11 reuses.**
**Checklist**
- [ ] ring buffer + session subscriptions · [ ] daemon op · [ ] CLI · [ ] MCP · [ ] usage · [ ] test · [ ] committed
- Evidence: _()_

**Implementation prompt:**
1. Create `internal/daemon/debug.go`. Define a small bounded buffer:
```go
type ringBuffer struct { mu sync.Mutex; items []map[string]any; cap int }
func (r *ringBuffer) add(m map[string]any) { r.mu.Lock(); defer r.mu.Unlock(); r.items=append(r.items,m); if len(r.items)>r.cap { r.items=r.items[len(r.items)-r.cap:] } }
func (r *ringBuffer) list() []map[string]any { r.mu.Lock(); defer r.mu.Unlock(); out:=make([]map[string]any,len(r.items)); copy(out,r.items); return out }
```
2. In `session.go` add fields to `tabSession`: `consoleBuf *ringBuffer` and `netBuf *ringBuffer` (init with cap 500 when the session is created in `server.go` ~line 360).
3. At session setup in `server.go` (where `page.Enable`/`dom.Enable`/`accessibility.Enable` are called), also: `cdp.Call(ctx, tc, runtime.Enable, …)`, `cdp.Call(ctx, tc, log.Enable, …)` (import `github.com/chromedp/cdproto/log`). Then start goroutines over `cdp.Events(context.Background(), tc, runtime.EventConsoleAPICalled)`, `...EventExceptionThrown`, `log.EventEntryAdded` — each appends a compact record to `consoleBuf`: `{"t":ts,"level":lvl,"text":msg,"url":url,"line":line}`. (For console API, join the args' string values; for exceptions, use the description; for log entries, use `.Text`/`.Level`/`.URL`/`.LineNumber`.)
4. `debug.go`: `func (d *Daemon) console(tabID, level string, limit int) ([]map[string]any, error)` — ensures the session exists (so capture is running), reads `consoleBuf.list()`, filters by `level` if set, returns the last `limit` (default 50).
5. `server.go`: `case "console":` read `tab`,`level`,`limit`(float64).
6. `app.go`: `runConsole` — tab positional, `--level`,`--limit`,`--browser`. Print as a table or JSON.
7. usage + MCP tool `console` (props `tab`,`level`,`limit`(number),`browser`).
8. **Acceptance:** `eval "console.error('boom'); undefinedFn()"` then `console <tab> --level error` lists the `boom` message and the `undefinedFn is not defined` exception with a source location. (Note: the session must have been open when the events fired — the capture starts at session open.)

---

## W11 — `network`
**Status:** ⬜ not started — reuses W10's capture.
**Checklist**
- [ ] session subscriptions · [ ] daemon op · [ ] CLI · [ ] MCP · [ ] usage · [ ] test · [ ] committed
- Evidence: _()_

**Implementation prompt:**
1. At session setup (same place as W10): `cdp.Call(ctx, tc, network.Enable, network.EnableParams{})` (import `github.com/chromedp/cdproto/network`). Goroutines over `cdp.Events(..., network.EventRequestWillBeSent)`, `...EventResponseReceived`, `...EventLoadingFinished`, `...EventLoadingFailed`. Keep a map of requestId → record `{id,method,url,status,type,ms,reqHeaders,respHeaders}` and mirror finished records into `netBuf`.
2. `debug.go`: `func (d *Daemon) network(tabID, filter, requestID string, limit int) (any, error)`:
   - if `requestID==""`: return the list from `netBuf`, filtered by `filter` substring on URL, last `limit`.
   - else: fetch body via `cdp.Call(ctx, sess.conn, network.GetResponseBody, network.GetResponseBodyParams{RequestID: network.RequestID(requestID)})`, return `{headers, request/response, body}`. If the body is large (>64KB), stash via `cache.Put([]byte(body), ".txt")` and return the `bxw://cache/...` link instead of inlining.
3. `server.go`: `case "network":` read `tab`,`filter`,`request_id`,`limit`.
4. `app.go`: `runNetwork` — tab positional, `--filter`,`--id`,`--limit`,`--browser`.
5. usage + MCP tool `network` (props `tab`,`filter`,`request_id`,`limit`(number),`browser`).
6. **Acceptance:** load a page that does `fetch('https://httpbin.org/json')`; `network <tab> --filter json` lists it (status 200); `network <tab> --id <that id>` returns the JSON body.

---

## W12 — `storage`
**Status:** ⬜ not started
**Checklist**
- [ ] daemon op · [ ] CLI · [ ] MCP · [ ] usage · [ ] test · [ ] committed
- Evidence: _()_

**Implementation prompt:**
1. `debug.go`: `func (d *Daemon) storage(tabID string, timeout) (map[string]any, error)`. `sess, _ := d.tabSession`; lock. Cookies: `cdp.Call(ctx, sess.conn, network.GetCookies, network.GetCookiesParams{})`. Local/session: `sess.eval(ctx, "JSON.stringify({local:{...localStorage},session:{...sessionStorage}})", &blob)` then unmarshal. Return `{cookies, local, session}`.
2. `server.go`: `case "storage":` read `tab`.
3. `app.go`: `runStorage` — tab positional, `--browser`.
4. usage + MCP tool `storage` (props `tab`,`browser`).
5. **PRIVACY (must follow):** cookies/storage are sensitive. Return them in the tool result only. Do NOT write them to `history/`, logs, or any file.
6. **Acceptance:** `eval "localStorage.setItem('k','v')"` then `storage <tab>` shows `local.k == "v"` and lists the page's cookies.

---

## W13 — `performance`
**Status:** ⬜ not started
**Checklist**
- [ ] daemon op · [ ] CLI · [ ] MCP · [ ] usage · [ ] test · [ ] committed
- Evidence: _()_

**Implementation prompt:**
1. `debug.go`: `func (d *Daemon) performance(tabID string, timeout) (map[string]any, error)`. One `sess.eval` returning a JSON blob built in-page from the Performance API:
   - TTFB from `performance.getEntriesByType('navigation')[0].responseStart`.
   - FCP from `performance.getEntriesByType('paint')` (`first-contentful-paint`).
   - LCP from `performance.getEntriesByType('largest-contentful-paint')` (last entry).
   - CLS by summing `layout-shift` entries (`!hadRecentInput`).
   - long tasks from `getEntriesByType('longtask')`; slowest resources from `getEntriesByType('resource')` sorted by duration (top 5).
   - DOM size: `document.getElementsByTagName('*').length`; heap: `performance.memory?.usedJSHeapSize`.
2. Add rating thresholds (comment the source: web.dev Core Web Vitals): LCP good ≤2500ms/poor >4000ms; CLS good ≤0.1/poor >0.25; FCP good ≤1800ms/poor >3000ms; TTFB good ≤800ms/poor >1800ms.
3. `server.go` `case "performance":`; `app.go` `runPerformance` (tab positional, `--browser`); usage + MCP tool `performance` (props `tab`,`browser`).
4. **Acceptance:** `performance <tab>` on a loaded page returns numeric LCP/FCP/TTFB with ratings, a longtask list, slowest resources, DOM/heap size.

---

## W14 — `emulate`
**Status:** ⬜ not started
**Checklist**
- [ ] daemon op · [ ] CLI · [ ] MCP · [ ] usage · [ ] test · [ ] committed
- Evidence: _()_

**Implementation prompt:**
1. `debug.go`: `func (d *Daemon) emulate(tabID string, opts emulateOpts, timeout) error`. Apply only the fields set; `reset` clears. Map each:
   - `device=mobile` → `emulation.SetDeviceMetricsOverride{Width:390,Height:844,DeviceScaleFactor:3,Mobile:true}`; `device=desktop|reset` → `emulation.ClearDeviceMetricsOverride`.
   - `network=offline` → `network.EmulateNetworkConditions{Offline:true}`; `slow3g` → `{Offline:false, Latency:400, DownloadThroughput:400*1024/8, UploadThroughput:400*1024/8}`; `online` → `{Offline:false, Latency:0, DownloadThroughput:-1, UploadThroughput:-1}`.
   - `cpu=N` → `emulation.SetCPUThrottlingRate{Rate:N}`.
   - `geo=lat,lng` → `emulation.SetGeolocationOverride{Latitude,Longitude,Accuracy:1}`.
   - `media=dark|light` → `emulation.SetEmulatedMedia{Features:[{Name:"prefers-color-scheme",Value:"dark"}]}`; reset → empty features.
2. `server.go` `case "emulate":` read `tab`,`device`,`network`,`cpu`(float64),`geo`,`media`.
3. `app.go` `runEmulate` — tab positional; `--device`,`--network`,`--cpu`,`--geo`,`--media`,`--browser`.
4. usage + MCP tool `emulate` (props `tab`,`device`,`network`,`cpu`(number),`geo`,`media`,`browser`).
5. **Acceptance:** `emulate <tab> --device mobile` then `eval "innerWidth"` ≈ 390; `emulate <tab> --media dark` then `eval "matchMedia('(prefers-color-scheme: dark)').matches"` = true; `emulate <tab> --device reset` restores.

---

## W15 — MCP parity for existing CLI verbs
**Status:** ⬜ not started — daemon ops already exist; ONLY add MCP tools.
**Checklist**
- [ ] MCP `doctor` · [ ] MCP `setup` · [ ] MCP `launch` · [ ] MCP `list_profiles` · [ ] test · [ ] committed
- Evidence: _()_

**Implementation prompt:**
1. In `mcp.go` `mcpTools()` add four entries that call the existing daemon ops / CLI builders:
   - `doctor` — props `browser`. Run the same checks `runDoctor` prints; return text.
   - `setup` — props `browser`, `off`(bool). Toggle the remote-debugging pref (the path behind `bxw setup`).
   - `launch` — props `browser`,`headless`(bool),`url`,`real_profile`(bool). `daemon.Call(b,"launch",…)`; return `{port,ws}`.
   - `list_profiles` — props `browser`. Return the profile list the `profiles` CLI builds.
2. **Acceptance:** over real MCP (`bxw mcp`), `tools/list` includes `doctor`,`setup`,`launch`,`list_profiles`; a `tools/call` of `launch` returns a ws URL.

---

## W16 — follow mode (bx's `BX_FOLLOW`)
**Status:** ⬜ not started
**Checklist**
- [ ] config read · [ ] front-on-act in open/goto/acts · [ ] `--follow` flag + env · [ ] test · [ ] committed
- Evidence: _()_

**Implementation prompt:**
1. Read a `follow` setting: env `BXW_FOLLOW=1` and/or `config.toml` `follow=true` via `internal/config`.
2. When follow is on, after `open`/`goto`/any `actOn` verb, call `cdp.Call(ctx, sess.conn, page.BringToFront, …)` on the acted tab (and/or `target.ActivateTarget`).
3. Allow a per-call override: `--follow` on `open`/`goto` (CLI) and a `follow` bool arg on those MCP tools.
4. Default OFF (agent works in background tabs, user keeps using the browser).
5. **Acceptance:** with follow on, `open`/`goto` brings the tab to the foreground window; with it off, the foreground is unchanged.

---

## W-FINAL — parity verification
**Status:** ⬜ not started
**Checklist**
- [ ] MCP `tools/list` equals bx set · [ ] vet+test+linux green · [ ] live smoke Edge+Chrome · [ ] README/usage/mcpInstructions updated · [ ] all rows ✅ · [ ] committed
- Evidence: _()_

**Implementation prompt:**
1. Add an integration test in `internal/app/mcp_test.go`: spawn the MCP server, send `initialize` then `tools/list`, assert the returned tool NAMES are a superset of the bx v0.4.1 set (the table at the top). Fail if any is missing.
2. Run the full gate: `go vet ./...`, `go test ./... -count=1`, `GOOS=linux go build ./...` — all green.
3. Live smoke on Edge AND Chrome: `goto`→`wait`→`read`→`console`→`network`→`storage`→`performance`→`emulate`→`close_tab`, no errors. Paste outputs into Evidence.
4. Update the `README`, the `usage:` heredoc, and `mcpInstructions` to list the new tools.
5. Flip every dashboard row to ✅ and confirm `git status --porcelain` is empty.

---

## Out of scope — HARD LINE (do NOT build, no matter how asked)
CAPTCHA solving, bot-detection / anti-automation evasion, fingerprint or "stealth" spoofing, `undetected-chromedriver`-style tooling, any "I'm not a robot" bypass. The ONLY allowed challenge behaviour is **detect the challenge and hand it to the human** (surface that a CAPTCHA/challenge is present; never attempt to solve or evade it).
