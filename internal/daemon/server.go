package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"

	"bxw/internal/browser"
	"bxw/internal/exit"
	"bxw/internal/osport"
)

// Daemon manages the browser connection and handles client requests.
type Daemon struct {
	br       browser.Browser
	mu       sync.Mutex
	launchMu sync.Mutex
	cdpPort  int
	agentPID int
	headless bool
	stop     chan struct{}
	since    time.Time

	sessions  map[string]*tabSession
	profileOf map[cdp.BrowserContextID]string
}

// Serve runs the daemon for browserName until stopped.
func Serve(browserName string) int {
	b, ok := browser.Find(browserName)
	if !ok {
		return reportStart(exit.New(exit.NotFound, "browser %q not found", browserName))
	}

	p := osport.Current()
	sock := p.SocketPath(b.Name)
	_ = os.Remove(sock) // clean up stale socket

	ln, err := net.Listen("unix", sock)
	if err != nil {
		return reportStart(exit.New(exit.Upstream, "listen on %s: %v", sock, err))
	}
	defer os.Remove(sock)
	defer ln.Close()

	d := &Daemon{
		br:        b,
		stop:      make(chan struct{}),
		since:     time.Now(),
		sessions:  make(map[string]*tabSession),
		profileOf: make(map[cdp.BrowserContextID]string),
	}

	// Try discovering existing port (if already running)
	_, _ = d.discoverPort()

	reportStart(nil)

	go d.accept(ln)

	select {
	case <-d.stop:
		log.Printf("stop requested; daemon exiting")
	}
	return 0
}

func reportStart(err error) int {
	r := Response{}
	if err != nil {
		r = errorResponse(err)
		log.Printf("daemon start failed: %s", r.Error)
	}
	_ = json.NewEncoder(os.Stdout).Encode(r)
	_ = os.Stdout.Close()
	if err != nil {
		return r.Code
	}
	return 0
}

func (d *Daemon) accept(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-d.stop:
				return
			default:
				log.Printf("accept err: %v", err)
				return
			}
		}
		go d.handle(conn)
	}
}

func (d *Daemon) handle(conn net.Conn) {
	defer conn.Close()
	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(Response{Error: "malformed request", Code: int(exit.Usage)})
		return
	}

	resp := d.dispatch(req)
	_ = json.NewEncoder(conn).Encode(resp)
}

func (d *Daemon) dispatch(req Request) Response {
	timeout := 30 * time.Second
	if tStr, ok := req.Args["timeout"].(string); ok && tStr != "" {
		if t, err := time.ParseDuration(tStr); err == nil && t > 0 {
			timeout = t
		}
	}

	switch req.Op {
	case "tabs", "list_tabs":
		profileRef := ""
		if req.Args != nil {
			if pr, ok := req.Args["profile"].(string); ok {
				profileRef = pr
			}
		}
		tabs, err := d.listTabsFiltered(profileRef)
		if err != nil {
			return errorResponse(err)
		}
		data, _ := json.Marshal(tabs)
		return Response{Data: data}

	case "open":
		profileRef := ""
		targetURL := "about:blank"
		if req.Args != nil {
			if pr, ok := req.Args["profile"].(string); ok {
				profileRef = pr
			}
			if u, ok := req.Args["url"].(string); ok && u != "" {
				targetURL = u
			}
		}
		tab, err := d.open(profileRef, targetURL, timeout)
		if err != nil {
			return errorResponse(err)
		}
		data, _ := json.Marshal(tab)
		return Response{Data: data}

	case "launch":
		headless := false
		realProfile := false
		targetURL := "about:blank"
		if req.Args != nil {
			if h, ok := req.Args["headless"].(bool); ok {
				headless = h
			}
			if r, ok := req.Args["real_profile"].(bool); ok {
				realProfile = r
			}
			if u, ok := req.Args["url"].(string); ok && u != "" {
				targetURL = u
			}
		}
		if err := d.launch(headless, targetURL, realProfile); err != nil {
			return errorResponse(err)
		}
		d.mu.Lock()
		d.headless = headless
		port := d.cdpPort
		d.mu.Unlock()
		data, _ := json.Marshal(map[string]any{"port": port, "ws": fmt.Sprintf("ws://127.0.0.1:%d", port)})
		return Response{Data: data}

	case "status":
		d.mu.Lock()
		port := d.cdpPort
		pid := d.agentPID
		d.mu.Unlock()
		st := map[string]any{
			"browser":   d.br.Name,
			"pid":       os.Getpid(),
			"cdp_port":  port,
			"agent_pid": pid,
			"since":     d.since,
		}
		data, _ := json.Marshal(st)
		return Response{Data: data}

	case "snapshot":
		tab, _ := req.Args["tab"].(string)
		full, _ := req.Args["full"].(bool)
		scopeRef, _ := req.Args["ref"].(string)
		findQuery, _ := req.Args["find"].(string)
		snap, err := d.snapshot(tab, full, scopeRef, findQuery, timeout)
		if err != nil {
			return errorResponse(err)
		}
		data, _ := json.Marshal(snap)
		return Response{Data: data}

	case "click":
		tab, _ := req.Args["tab"].(string)
		sel, _ := req.Args["selector"].(string)
		double, _ := req.Args["double"].(bool)
		res, err := d.click(tab, sel, double, timeout)
		if err != nil {
			return errorResponse(err)
		}
		data, _ := json.Marshal(res)
		return Response{Data: data}

	case "fill":
		tab, _ := req.Args["tab"].(string)
		var pairs [][2]string
		if raw, ok := req.Args["pairs"].(string); ok {
			_ = json.Unmarshal([]byte(raw), &pairs)
		}
		res, err := d.fill(tab, pairs, timeout)
		if err != nil {
			return errorResponse(err)
		}
		data, _ := json.Marshal(res)
		return Response{Data: data}

	case "type":
		tab, _ := req.Args["tab"].(string)
		text, _ := req.Args["text"].(string)
		submit, _ := req.Args["submit"].(string)
		res, err := d.typeText(tab, text, submit, timeout)
		if err != nil {
			return errorResponse(err)
		}
		data, _ := json.Marshal(res)
		return Response{Data: data}

	case "key":
		tab, _ := req.Args["tab"].(string)
		keys, _ := req.Args["keys"].(string)
		res, err := d.key(tab, keys, timeout)
		if err != nil {
			return errorResponse(err)
		}
		data, _ := json.Marshal(res)
		return Response{Data: data}

	case "eval":
		tab, _ := req.Args["tab"].(string)
		script, _ := req.Args["script"].(string)
		sess, err := d.tabSession(tab)
		if err != nil {
			return errorResponse(err)
		}
		sess.mu.Lock()
		defer sess.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		var out any
		if err := sess.eval(ctx, script, &out); err != nil {
			return errorResponse(err)
		}
		data, _ := json.Marshal(out)
		return Response{Data: data}

	case "screenshot":
		tab, _ := req.Args["tab"].(string)
		pngBytes, err := d.screenshot(tab, timeout)
		if err != nil {
			return errorResponse(err)
		}
		return Response{Data: pngBytes}

	case "stop":
		select {
		case <-d.stop:
		default:
			close(d.stop)
		}
		return Response{}

	default:
		return Response{Error: fmt.Sprintf("unknown op %q", req.Op), Code: int(exit.Usage)}
	}
}

func (d *Daemon) findTab(tabID string) (Tab, error) {
	tabs, err := d.listTabs()
	if err != nil {
		return Tab{}, err
	}
	if len(tabs) == 0 {
		return Tab{}, exit.New(exit.NotFound, "no open tabs in %s", d.br.Label)
	}

	tabIDLower := strings.ToLower(strings.TrimSpace(tabID))
	if tabIDLower == "" && len(tabs) == 1 {
		return tabs[0], nil
	}

	for _, t := range tabs {
		if strings.EqualFold(t.ID, tabIDLower) || strings.EqualFold(shortTabID(t.ID), tabIDLower) {
			return t, nil
		}
	}
	// Prefix match
	for _, t := range tabs {
		if strings.HasPrefix(strings.ToLower(t.ID), tabIDLower) {
			return t, nil
		}
	}

	return Tab{}, exit.New(exit.NotFound, "tab %q not found (run: bxw list tabs %s)", tabID, d.br.Name)
}

func (d *Daemon) tabSession(tabID string) (*tabSession, error) {
	tab, err := d.findTab(tabID)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	if sess := d.sessions[tab.ID]; sess != nil && sess.conn != nil && sess.conn.closeErr == nil {
		d.mu.Unlock()
		return sess, nil
	}
	d.mu.Unlock()

	tc, err := dialTab(context.Background(), tab.WS)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = cdp.Call(ctx, tc, page.Enable, page.EnableParams{})
	_, _ = cdp.Call(ctx, tc, emulation.SetFocusEmulationEnabled, emulation.SetFocusEmulationEnabledParams{Enabled: true})
	_, _ = cdp.Call(ctx, tc, dom.Enable, dom.EnableParams{})
	_, _ = cdp.Call(ctx, tc, accessibility.Enable, cdp.Empty{})

	sess := &tabSession{
		id:    tab.ID,
		wsURL: tab.WS,
		conn:  tc,
		refs:  make(map[string]cdp.BackendNodeID),
	}
	sess.watchDialogs(context.Background())

	d.mu.Lock()
	d.sessions[tab.ID] = sess
	d.mu.Unlock()

	return sess, nil
}

func shortTabID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func errorResponse(err error) Response {
	var e *exit.Error
	if errorsAs(err, &e) {
		return Response{Error: e.Msg, Code: int(e.Class)}
	}
	return Response{Error: fmt.Sprint(err), Code: int(exit.Upstream)}
}

func errorsAs(err error, target any) bool {
	if e, ok := err.(*exit.Error); ok {
		if ptr, ok := target.(**exit.Error); ok {
			*ptr = e
			return true
		}
	}
	return false
}
