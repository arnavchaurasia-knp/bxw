package daemon

import (
	"encoding/json"
	"testing"
)

func TestRequestResponseJSON(t *testing.T) {
	req := Request{
		Op:   "list_tabs",
		Args: map[string]any{"headless": true},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var req2 Request
	if err := json.Unmarshal(data, &req2); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req2.Op != "list_tabs" {
		t.Errorf("got op %q, want %q", req2.Op, "list_tabs")
	}

	tabs := []Tab{
		{ID: "tab-1", Title: "Example", URL: "https://example.com", Type: "page"},
	}
	tabsRaw, err := json.Marshal(tabs)
	if err != nil {
		t.Fatalf("marshal tabs: %v", err)
	}

	resp := Response{
		Data: tabsRaw,
		Code: 0,
	}
	respData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var resp2 Response
	if err := json.Unmarshal(respData, &resp2); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp2.Error != "" {
		t.Errorf("unexpected error in response: %s", resp2.Error)
	}

	var decodedTabs []Tab
	if err := json.Unmarshal(resp2.Data, &decodedTabs); err != nil {
		t.Fatalf("unmarshal decoded tabs: %v", err)
	}
	if len(decodedTabs) != 1 || decodedTabs[0].Title != "Example" {
		t.Errorf("got %+v, want 1 tab with title Example", decodedTabs)
	}
}
