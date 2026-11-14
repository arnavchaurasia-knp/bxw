package daemon

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"

	"bxw/internal/cache"
	"bxw/internal/exit"
)

type ringBuffer struct {
	mu    sync.Mutex
	items []map[string]any
	cap   int
}

func newRingBuffer(capacity int) *ringBuffer {
	if capacity <= 0 {
		capacity = 500
	}
	return &ringBuffer{cap: capacity}
}

func (r *ringBuffer) add(m map[string]any) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, m)
	if len(r.items) > r.cap {
		r.items = r.items[len(r.items)-r.cap:]
	}
}

func (r *ringBuffer) list() []map[string]any {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]map[string]any, len(r.items))
	copy(out, r.items)
	return out
}

func (sess *tabSession) startDebugCapture(ctx context.Context) {
	if sess.consoleBuf == nil {
		sess.consoleBuf = newRingBuffer(500)
	}
	if sess.netBuf == nil {
		sess.netBuf = newRingBuffer(500)
	}

	// 1. Console API calls
	consoleCh := cdp.Events(ctx, sess.conn, runtime.ConsoleAPICalled)
	go func() {
		for ev, err := range consoleCh {
			if err != nil {
				return
			}
			var parts []string
			for _, arg := range ev.Args {
				if arg != nil {
					if len(arg.Value) > 0 {
						parts = append(parts, string(arg.Value))
					} else if arg.Description != "" {
						parts = append(parts, arg.Description)
					}
				}
			}
			url := ""
			line := int64(0)
			if ev.StackTrace != nil && len(ev.StackTrace.CallFrames) > 0 {
				url = ev.StackTrace.CallFrames[0].URL
				line = ev.StackTrace.CallFrames[0].LineNumber
			}
			sess.consoleBuf.add(map[string]any{
				"t":     time.Now().UTC().Format(time.RFC3339Nano),
				"level": string(ev.Type),
				"text":  strings.Join(parts, " "),
				"url":   url,
				"line":  line,
			})
		}
	}()

	// 2. Runtime exceptions
	exCh := cdp.Events(ctx, sess.conn, runtime.ExceptionThrown)
	go func() {
		for ev, err := range exCh {
			if err != nil {
				return
			}
			msg := ev.ExceptionDetails.Text
			if ev.ExceptionDetails.Exception != nil && ev.ExceptionDetails.Exception.Description != "" {
				msg = ev.ExceptionDetails.Exception.Description
			}
			sess.consoleBuf.add(map[string]any{
				"t":     time.Now().UTC().Format(time.RFC3339Nano),
				"level": "error",
				"text":  msg,
				"url":   ev.ExceptionDetails.URL,
				"line":  ev.ExceptionDetails.LineNumber,
			})
		}
	}()

	// 3. Log entries
	logCh := cdp.Events(ctx, sess.conn, log.EntryAdded)
	go func() {
		for ev, err := range logCh {
			if err != nil {
				return
			}
			if ev.Entry != nil {
				sess.consoleBuf.add(map[string]any{
					"t":     time.Now().UTC().Format(time.RFC3339Nano),
					"level": string(ev.Entry.Level),
					"text":  ev.Entry.Text,
					"url":   ev.Entry.URL,
					"line":  ev.Entry.LineNumber,
				})
			}
		}
	}()

	// 4. Network tracking
	pendingNetMu := sync.Mutex{}
	pendingNet := make(map[network.RequestID]map[string]any)

	reqCh := cdp.Events(ctx, sess.conn, network.RequestWillBeSent)
	respCh := cdp.Events(ctx, sess.conn, network.ResponseReceived)
	finishCh := cdp.Events(ctx, sess.conn, network.LoadingFinished)
	failCh := cdp.Events(ctx, sess.conn, network.LoadingFailed)

	go func() {
		for ev, err := range reqCh {
			if err != nil {
				return
			}
			pendingNetMu.Lock()
			rec := map[string]any{
				"id":         string(ev.RequestID),
				"method":     ev.Request.Method,
				"url":        ev.Request.URL,
				"type":       string(ev.Type),
				"start":      time.Now(),
				"reqHeaders": ev.Request.Headers,
			}
			pendingNet[ev.RequestID] = rec
			pendingNetMu.Unlock()
		}
	}()

	go func() {
		for ev, err := range respCh {
			if err != nil {
				return
			}
			pendingNetMu.Lock()
			rec, ok := pendingNet[ev.RequestID]
			if !ok {
				rec = map[string]any{
					"id":    string(ev.RequestID),
					"type":  string(ev.Type),
					"start": time.Now(),
				}
				pendingNet[ev.RequestID] = rec
			}
			if ev.Response != nil {
				rec["url"] = ev.Response.URL
				rec["status"] = ev.Response.Status
				rec["respHeaders"] = ev.Response.Headers
				rec["mimeType"] = ev.Response.MimeType
			}
			pendingNetMu.Unlock()
		}
	}()

	go func() {
		for ev, err := range finishCh {
			if err != nil {
				return
			}
			pendingNetMu.Lock()
			rec, ok := pendingNet[ev.RequestID]
			if ok {
				delete(pendingNet, ev.RequestID)
				if startTime, ok := rec["start"].(time.Time); ok {
					rec["ms"] = time.Since(startTime).Milliseconds()
					delete(rec, "start")
				}
				sess.netBuf.add(rec)
			}
			pendingNetMu.Unlock()
		}
	}()

	go func() {
		for ev, err := range failCh {
			if err != nil {
				return
			}
			pendingNetMu.Lock()
			rec, ok := pendingNet[ev.RequestID]
			if ok {
				delete(pendingNet, ev.RequestID)
				rec["error"] = ev.ErrorText
				if startTime, ok := rec["start"].(time.Time); ok {
					rec["ms"] = time.Since(startTime).Milliseconds()
					delete(rec, "start")
				}
				sess.netBuf.add(rec)
			}
			pendingNetMu.Unlock()
		}
	}()
}

func (d *Daemon) console(tabID, level string, limit int) ([]map[string]any, error) {
	sess, err := d.tabSession(tabID)
	if err != nil {
		return nil, err
	}

	items := sess.consoleBuf.list()
	var filtered []map[string]any
	levelLower := strings.ToLower(strings.TrimSpace(level))

	for _, item := range items {
		if levelLower != "" {
			itemLvl, _ := item["level"].(string)
			if !strings.EqualFold(itemLvl, levelLower) {
				continue
			}
		}
		filtered = append(filtered, item)
	}

	if limit <= 0 {
		limit = 50
	}
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered, nil
}

func (d *Daemon) network(tabID, filter, requestID string, limit int) (any, error) {
	sess, err := d.tabSession(tabID)
	if err != nil {
		return nil, err
	}

	if requestID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		sess.mu.Lock()
		bodyBytes, err := cdp.Call(ctx, sess.conn, network.GetResponseBody, network.GetResponseBodyParams{
			RequestID: network.RequestID(requestID),
		})
		sess.mu.Unlock()

		if err != nil {
			return nil, exit.New(exit.Upstream, "get response body: %v", err)
		}

		bodyStr := string(bodyBytes.Body)
		if len(bodyBytes.Body) > 64*1024 {
			uri, _, err := cache.Put(bodyBytes.Body, ".txt")
			if err == nil {
				bodyStr = uri
			}
		}

		// Find matching record from buffer
		items := sess.netBuf.list()
		var matched map[string]any
		for _, it := range items {
			if it["id"] == requestID {
				matched = it
				break
			}
		}
		return map[string]any{
			"id":       requestID,
			"metadata": matched,
			"body":     bodyStr,
		}, nil
	}

	items := sess.netBuf.list()
	var filtered []map[string]any
	filterLower := strings.ToLower(strings.TrimSpace(filter))

	for _, item := range items {
		if filterLower != "" {
			url, _ := item["url"].(string)
			if !strings.Contains(strings.ToLower(url), filterLower) {
				continue
			}
		}
		filtered = append(filtered, item)
	}

	if limit <= 0 {
		limit = 50
	}
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered, nil
}
