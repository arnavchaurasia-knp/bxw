package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"bxw/internal/cache"
)

func TestMCPServerInitializeAndTools(t *testing.T) {
	// Request 1: initialize
	// Request 2: tools/list
	// Request 3: ping
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
		"",
	}, "\n")

	in := bytes.NewBufferString(input)
	var out bytes.Buffer

	err := serveMCP(in, &out)
	if err != nil {
		t.Fatalf("serveMCP error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 response lines, got %d:\n%s", len(lines), out.String())
	}

	// Parse responses
	resps := make(map[int]rpcMsg)
	for _, l := range lines {
		var m rpcMsg
		if err := json.Unmarshal([]byte(l), &m); err == nil {
			var id int
			_ = json.Unmarshal(m.ID, &id)
			resps[id] = m
		}
	}

	// 1. Check initialize
	initResp, ok := resps[1]
	if !ok || initResp.Error != nil {
		t.Fatalf("invalid initialize response: %+v", initResp)
	}
	initResult, ok := initResp.Result.(map[string]any)
	if !ok || initResult["protocolVersion"] != mcpProtocol {
		t.Errorf("expected protocolVersion %s, got %v", mcpProtocol, initResult["protocolVersion"])
	}

	// 2. Check tools/list
	toolsResp, ok := resps[2]
	if !ok || toolsResp.Error != nil {
		t.Fatalf("invalid tools/list response: %+v", toolsResp)
	}
	toolsResult, ok := toolsResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result for tools/list, got %T", toolsResp.Result)
	}
	toolList, ok := toolsResult["tools"].([]any)
	if !ok || len(toolList) < 8 {
		t.Errorf("expected at least 8 tools in tools/list, got %d", len(toolList))
	}

	toolNames := make(map[string]bool)
	for _, tool := range toolList {
		if tm, ok := tool.(map[string]any); ok {
			if name, ok := tm["name"].(string); ok {
				toolNames[name] = true
			}
		}
	}

	requiredTools := []string{"list_browsers", "list_tabs", "open", "close_tab", "goto", "wait", "read", "console", "network", "storage", "performance", "dialog", "snapshot", "screenshot", "click", "hover", "drag", "upload", "fill", "type", "key", "eval", "cdp"}
	for _, req := range requiredTools {
		if !toolNames[req] {
			t.Errorf("missing expected MCP tool: %s", req)
		}
	}

	// 3. Check ping
	pingResp, ok := resps[3]
	if !ok || pingResp.Error != nil {
		t.Fatalf("invalid ping response: %+v", pingResp)
	}
}

func TestMCPServerResourcesRead(t *testing.T) {
	// Put a test blob into cache
	testData := []byte("fake-png-binary-content-1234")
	uri, _, err := cache.Put(testData, ".png")
	if err != nil {
		t.Fatalf("cache.Put failed: %v", err)
	}

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":10,"method":"resources/read","params":{"uri":"` + uri + `"}}`,
		"",
	}, "\n")

	in := bytes.NewBufferString(input)
	var out bytes.Buffer

	err = serveMCP(in, &out)
	if err != nil {
		t.Fatalf("serveMCP error: %v", err)
	}

	var resp rpcMsg
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json unmarshal failed: %v\nOutput: %s", err, out.String())
	}
	if resp.Error != nil {
		t.Fatalf("resources/read returned error: %+v", resp.Error)
	}

	resMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	contents, ok := resMap["contents"].([]any)
	if !ok || len(contents) == 0 {
		t.Fatalf("expected non-empty contents array")
	}
	first := contents[0].(map[string]any)
	if first["uri"] != uri {
		t.Errorf("expected uri %s, got %v", uri, first["uri"])
	}
	if first["mimeType"] != "image/png" {
		t.Errorf("expected image/png mimeType, got %v", first["mimeType"])
	}
	if first["blob"] != string(testData) {
		t.Errorf("blob content mismatch")
	}
}
