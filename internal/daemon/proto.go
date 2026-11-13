// Package daemon is the singleton background process that owns the browser
// connection, plus the client used to reach it over a unix socket. The
// request/response types are the stable IPC contract (shared, OS-agnostic).
package daemon

import "encoding/json"

// Request is one JSON op sent to the daemon over the unix socket.
type Request struct {
	Op   string         `json:"op"`
	Args map[string]any `json:"args,omitempty"`
}

// Response is the daemon's reply.
type Response struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
	Code  int             `json:"code,omitempty"`
}

// Tab is one CDP target as reported by the DevTools /json/list endpoint.
type Tab struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	WS         string `json:"webSocketDebuggerUrl,omitempty"`
	Profile    string `json:"profile,omitempty"`
	ProfileDir string `json:"profile_dir,omitempty"`
}
