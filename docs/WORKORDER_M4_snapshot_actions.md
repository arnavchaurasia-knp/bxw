# WORKORDER — M4: DOM Snapshot + Ref-Based Actions

**Goal.** Turn bxw from "lists tabs" into "operates the page": a token-efficient
accessibility snapshot that labels elements with stable refs (`e1`, `e2`…), and actions
(`click`, `fill`, `type`, `key`) that target those refs over CDP.

**Depends on:** M3 (daemon can reach a CDP endpoint; `d.cdpPort` is set after `launch`).
**Status of prerequisites:** DONE (M0–M3 committed).

---

## Contracts this builds on (verified on disk — do not re-invent)
- `daemon.Daemon{ br browser.Browser; mu sync.Mutex; cdpPort int; agentPID int; stop chan struct{}; since time.Time }` (`internal/daemon/server.go`).
- IPC: `Request{Op string; Args map[string]any}`, `Response{Data json.RawMessage; Error string; Code int}` (`internal/daemon/proto.go`). Dispatch in `server.go:dispatch`.
- Client: `daemon.Call(browser, op string, args any) ([]byte, error)` (`internal/daemon/client.go`).
- Browser→WS URL: `browser.Browser.WSURL() (string, error)` returns `ws://127.0.0.1:<port>/devtools/browser/<uuid>` by parsing `DevToolsActivePort`; or derive the browser WS from `GET http://127.0.0.1:<d.cdpPort>/json/version` → `webSocketDebuggerUrl` (preferred for daemon-launched agent instances — see Gotcha G1).
- Per-tab WS: `GET /json/list` → `Tab.WS` (`ws://127.0.0.1:<port>/devtools/page/<id>`).
- Deps already in `go.mod`: `github.com/chromedp/cdproto v0.159.0`, `github.com/chromedp/chromedp v0.23.0`.

## CDP API shape (cdproto v0.159 — NEW generic style, verified)
Commands are **vars** of type `cdp.Command[P,R]`, invoked with `cdp.Call`:
```go
// generic invoker (cdp/types.go)
func Call[P, R any](ctx context.Context, s cdp.Session, c cdp.Command[P, R], params P) (R, error)
```
- `accessibility.GetFullAXTree` → `cdp.Command[GetFullAXTreeParams, GetFullAXTreeResult]`; `GetFullAXTreeResult.Nodes []*accessibility.Node`.
- `accessibility.Node{ NodeID; Ignored bool; Role *Value; Name *Value; Value *Value; BackendDOMNodeID cdp.BackendNodeID; ChildIDs []NodeID; ... }`.
- `accessibility.Value{ Type ValueType; Value jsonv2.Value /* = []byte JSON */ }` → extract a string with `jsonv2.Unmarshal(v.Value, &s)` (import `jsonv2 "github.com/go-json-experiment/json"`), falling back to `strings.Trim(string(v.Value), "\"")`.
- `dom.ResolveNode` → `cdp.Command[ResolveNodeParams, ResolveNodeResult]`; build `dom.ResolveNodeParams{BackendNodeID: id}`; result has `*runtime.RemoteObject` with `ObjectID`.
- `runtime.CallFunctionOn` → `cdp.Command[CallFunctionOnParams, CallFunctionOnResult]`; `CallFunctionOnParams{FunctionDeclaration, ObjectID, Arguments []*runtime.CallArgument}`; `runtime.CallArgument{Value jsonv2.Value}`.
- Input: `input.DispatchMouseEvent`, `input.DispatchKeyEvent`, `input.InsertText` (same generic-command pattern).

> **The one thing the implementer must confirm against chromedp v0.23:** how to obtain a
> `cdp.Session` for a specific target to pass to `cdp.Call`. chromedp v0.23 is the paired
> release for cdproto v0.159, so attach to the existing tab with
> `chromedp.NewContext(allocCtx, chromedp.WithTargetID(target.ID(tabID)))` (both symbols
> verified present: `chromedp.WithTargetID(id target.ID)`, `chromedp.Targets(ctx)`), then
> bridge to a `cdp.Session`/executor from that context. **If the bridge is awkward, use the
> raw-WS fallback below** — it is fully under our control and avoids version coupling.

### Raw-WS fallback (recommended if chromedp↔cdproto bridging is painful)
Open a WebSocket to the per-tab `Tab.WS` and speak CDP JSON directly:
`{ "id":N, "method":"Accessibility.getFullAXTree", "params":{} }` → match responses by `id`.
Methods needed (all stable, version-independent): `Accessibility.getFullAXTree`,
`DOM.resolveNode`, `Runtime.callFunctionOn`, `Input.dispatchMouseEvent`,
`Input.dispatchKeyEvent`, `Input.insertText`, `DOM.getBoxModel`. Use a small stdlib-friendly
WS lib (e.g. `github.com/coder/websocket`) — one new dep, clean-room, no cdproto churn.

---

## Files
- `internal/daemon/dom.go` — NEW: `snapshot`, `click`, `fill`, `typeText`, `key`, plus the
  per-tab CDP session helper.
- `internal/daemon/server.go` — add ref store to `Daemon` and new ops to `dispatch`.
- `internal/app/app.go` — add CLI verbs: `snapshot`, `click`, `fill`, `type`, `key`.
- `internal/daemon/dom_test.go` — NEW: ref assignment + AX-filter unit tests (table-driven,
  no browser); an integration test gated behind a build tag or env flag that drives a real
  headless agent instance.

## Design / steps
1. **Ref store.** Add to `Daemon`: `refs map[string]map[string]cdp.BackendNodeID` (tabID → ref
   → backendNodeId), guarded by `d.mu`. Refs live until the next `snapshot` of that tab.
2. **Session helper.** `func (d *Daemon) tabSession(tabID string) (session, cleanup, error)`:
   resolve browser WS (via `/json/version` on `d.cdpPort`), attach to `tabID`, return a
   handle usable by `cdp.Call` (or a raw-WS conn). Cache per tab if cheap; otherwise per-call.
3. **snapshot(tabID).** `Accessibility.getFullAXTree` → iterate `Nodes`:
   - skip `n.Ignored`; skip roles in `{"none","generic","InlineTextBox","LineBreak"}`;
   - assign sequential `e1,e2,…` to the rest; record `refs[tabID][ref]=n.BackendDOMNodeID`;
   - emit one line per ref: `[e12] <role> "<name>"` (truncate name ~80 chars).
   Return the text block (and optionally structured JSON).
4. **tag(ref).** `DOM.resolveNode{BackendNodeID}` → `ObjectID`; `Runtime.callFunctionOn` with
   `function(r){ (this.nodeType===3?this.parentElement:this).setAttribute("data-bxw-ref", r); }`
   and arg `ref`. (Matches bx's proven technique.)
5. **click(tabID, ref|selector).** Resolve selector `[data-bxw-ref="<ref>"]` (after tag) or the
   raw selector; `DOM.getBoxModel` for center; `Input.dispatchMouseEvent` (mousePressed +
   mouseReleased, button=left). Or `chromedp.Click(sel, chromedp.ByQuery)` if using chromedp.
6. **fill(tabID, ref, value).** focus element, select-all + delete (or set `.value=""` via
   `callFunctionOn`), then `Input.insertText{Text:value}`. Fire an `input` event if needed.
7. **type(tabID, ref, text).** per-rune `Input.dispatchKeyEvent` (keyDown/char/keyUp).
   **key(tabID, name).** named keys (Enter, Tab, Escape, ArrowDown, F5, …) → `Input.dispatchKeyEvent`.
8. **Ops.** Add to `dispatch`: `"snapshot"`, `"click"`, `"fill"`, `"type"`, `"key"`, each
   reading `Args["tab"]`, `Args["ref"]`, `Args["value"]/Args["text"]`.
9. **CLI.** `bxw snapshot <tab>`, `bxw click <tab> <ref>`, `bxw fill <tab> <ref> "<value>"`,
   `bxw type <tab> <ref> "<text>"`, `bxw key <tab> <name>` → `daemon.Call(...)`.

## Acceptance gate
- Unit: AX filtering drops ignored/generic nodes; refs are sequential and stable within a
  snapshot; name extraction from `accessibility.Value` works.
- Integration (headless agent, real browser): bundle `testdata/form.html` (an input + a
  button + a result div). `launch` it, `snapshot` shows the input and button with refs,
  `fill` the input, `click` the button, `snapshot`/`read` confirms the result text.
- `go vet ./...` clean; `GOOS=linux go build ./...` still green (stub unaffected).

## Gotchas / anchors
- **G1 (A9):** daemon-launched agent instances may use `--remote-debugging-port=<fixed>` with
  **no** `DevToolsActivePort` file in headless. Always resolve the browser WS via
  `GET /json/version` on `d.cdpPort`, not by reading `DevToolsActivePort`, for agent instances.
- **Ref lifetime:** navigation invalidates refs; a stale ref must return a clear "re-snapshot"
  error, not a silent no-op.
- **Single frame for M4:** cross-origin iframes are out of scope here (own milestone later).
- **data-bxw-ref** (not `data-bx-ref`) — keep the bxw namespace.

## Out of scope (later milestones)
Multi-frame, `drag`, `upload`, `screenshot`, `wait`, console/network/storage/performance
(debug tools), real-profile attach (M6).
