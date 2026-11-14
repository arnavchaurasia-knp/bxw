package contextmgr

import (
	"strings"
	"testing"
	"time"
)

func TestContextSaveAndLoad(t *testing.T) {
	sessionID := "test-session-123"
	ctx := SessionContext{
		SessionID: sessionID,
		Browser:   "edge",
		ActiveTab: "tab-42",
		OpenTabs: []TabSummary{
			{ID: "tab-42", Title: "Example", URL: "https://example.com"},
		},
		LastActions: []ActionSummary{
			{Action: "click e1", Result: "success", Time: time.Now()},
		},
		Notes: []string{"User requested sign in check"},
	}

	if err := SaveSession(ctx); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	loaded, err := LoadSession(sessionID)
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	if loaded.Browser != "edge" || loaded.ActiveTab != "tab-42" {
		t.Errorf("loaded context mismatch: %+v", loaded)
	}

	sum := loaded.Summary()
	if !strings.Contains(sum, "tab-42") || !strings.Contains(sum, "Example") {
		t.Errorf("summary missing expected fields: %s", sum)
	}
}
