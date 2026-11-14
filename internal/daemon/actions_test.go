package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bxw/internal/browser"
)

func TestActionsLiveEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live browser action test in short mode")
	}

	bs := browser.Installed()
	if len(bs) == 0 {
		t.Skip("no supported Chromium browser found on this machine")
	}
	targetBrowser := bs[0]

	// Start local test server serving testdata/form.html
	testHTML, err := os.ReadFile(filepath.Join("..", "..", "testdata", "form.html"))
	if err != nil {
		t.Fatalf("read testdata/form.html: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(testHTML)
	}))
	defer ts.Close()

	d := &Daemon{
		br:       targetBrowser,
		stop:     make(chan struct{}),
		since:    time.Now(),
		sessions: make(map[string]*tabSession),
	}

	// 1. Launch headless agent browser pointed at test server
	if err := d.launch(true, ts.URL, false); err != nil {
		t.Fatalf("launch headless %s: %v", targetBrowser.Name, err)
	}
	defer func() {
		if d.agentPID != 0 {
			if proc, err := os.FindProcess(d.agentPID); err == nil {
				_ = proc.Kill()
			}
		}
	}()

	// 2. Find the tab
	tabs, err := d.listTabs()
	if err != nil {
		t.Fatalf("listTabs: %v", err)
	}
	if len(tabs) == 0 {
		t.Fatal("expected at least 1 tab open")
	}
	tabID := tabs[0].ID

	// Wait for test server page to load completely before taking initial snapshot
	_, _ = d.wait(tabID, waitCond{Load: true}, 5*time.Second)

	// 3. Take snapshot
	snap, err := d.snapshot(tabID, true, "", "", 15*time.Second)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	var inputRef, buttonRef string
	for _, n := range snap.Nodes {
		if n.Role == "textbox" {
			inputRef = n.Ref
		}
		if n.Role == "button" && strings.Contains(n.Name, "Submit") {
			buttonRef = n.Ref
		}
	}

	if inputRef == "" {
		t.Fatalf("failed to find textbox in snapshot:\n%s", snap.Text())
	}
	if buttonRef == "" {
		t.Fatalf("failed to find submit button in snapshot:\n%s", snap.Text())
	}

	// 4. Fill textbox with "Alice"
	_, err = d.fill(tabID, [][2]string{{inputRef, "Alice"}}, 10*time.Second)
	if err != nil {
		t.Fatalf("fill %s: %v", inputRef, err)
	}

	// 5. Click submit button
	_, err = d.click(tabID, buttonRef, false, 10*time.Second)
	if err != nil {
		t.Fatalf("click %s: %v", buttonRef, err)
	}

	// 6. Verify result div in DOM
	sess, err := d.tabSession(tabID)
	if err != nil {
		t.Fatalf("tabSession: %v", err)
	}

	var resultText string
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sess.eval(ctx, `document.getElementById('result').textContent`, &resultText)
	if err != nil {
		t.Fatalf("eval result: %v", err)
	}

	if resultText != "Submitted: Alice" {
		t.Fatalf("expected 'Submitted: Alice', got %q", resultText)
	}
	t.Logf("Success! Live browser action verified. Result: %s", resultText)

	// 7. Test generic CDP dispatch (W1)
	resp := d.dispatch(Request{
		Op: "cdp",
		Args: map[string]any{
			"tab":    tabID,
			"method": "Runtime.evaluate",
			"params": `{"expression":"1+1","returnByValue":true}`,
		},
	})
	if resp.Error != "" {
		t.Fatalf("dispatch cdp error: %s", resp.Error)
	}
	var evalOut struct {
		Result struct {
			Type  string `json:"type"`
			Value int    `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp.Data, &evalOut); err != nil {
		t.Fatalf("unmarshal cdp response %s: %v", string(resp.Data), err)
	}
	if evalOut.Result.Value != 2 {
		t.Fatalf("expected cdp eval result 2, got %v", evalOut.Result.Value)
	}
	t.Logf("CDP dispatch verified: %s", string(resp.Data))

	// Test W2 goto dispatch
	gotoResp := d.dispatch(Request{
		Op: "goto",
		Args: map[string]any{
			"tab": tabID,
			"url": ts.URL + "/late",
		},
	})
	if gotoResp.Error != "" {
		t.Fatalf("dispatch goto error: %s", gotoResp.Error)
	}
	var gotoRes ActResult
	if err := json.Unmarshal(gotoResp.Data, &gotoRes); err != nil {
		t.Fatalf("unmarshal goto result %s: %v", string(gotoResp.Data), err)
	}
	t.Logf("goto summary: %s", gotoRes.TerseSummary("goto"))

	// Test W3 wait dispatch (inject delayed element and wait for selector)
	_ = d.dispatch(Request{
		Op: "eval",
		Args: map[string]any{
			"tab":    tabID,
			"script": `setTimeout(()=>{var el=document.createElement('div');el.id='delayed-el';el.innerText='Hello Late';document.body.appendChild(el);}, 250)`,
		},
	})

	waitResp := d.dispatch(Request{
		Op: "wait",
		Args: map[string]any{
			"tab":      tabID,
			"selector": "#delayed-el",
			"timeout":  "5s",
		},
	})
	if waitResp.Error != "" {
		t.Fatalf("dispatch wait error: %s", waitResp.Error)
	}
	var waitRes map[string]any
	if err := json.Unmarshal(waitResp.Data, &waitRes); err != nil {
		t.Fatalf("unmarshal wait result %s: %v", string(waitResp.Data), err)
	}
	if sat, ok := waitRes["satisfied"].(bool); !ok || !sat {
		t.Fatalf("expected wait satisfied=true, got %v", waitRes)
	}
	t.Logf("wait verified: %+v", waitRes)
}
