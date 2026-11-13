package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"bxw/internal/browser"
	"bxw/internal/exit"
	"bxw/internal/osport"
)

// Daemon manages the browser connection and handles client requests.
type Daemon struct {
	br       browser.Browser
	mu       sync.Mutex
	cdpPort  int
	agentPID int
	stop     chan struct{}
	since    time.Time
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
		br:    b,
		stop:  make(chan struct{}),
		since: time.Now(),
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
	switch req.Op {
	case "tabs", "list_tabs":
		tabs, err := d.listTabs()
		if err != nil {
			return errorResponse(err)
		}
		data, _ := json.Marshal(tabs)
		return Response{Data: data}
	case "launch":
		headless := false
		targetURL := "about:blank"
		if req.Args != nil {
			if h, ok := req.Args["headless"].(bool); ok {
				headless = h
			}
			if u, ok := req.Args["url"].(string); ok && u != "" {
				targetURL = u
			}
		}
		if err := d.launch(headless, targetURL); err != nil {
			return errorResponse(err)
		}
		d.mu.Lock()
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
