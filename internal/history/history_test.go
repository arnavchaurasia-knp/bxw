package history

import (
	"testing"
)

func TestHistoryLogAndQuery(t *testing.T) {
	testBrowser := "test-browser"
	err := Log(testBrowser, "Default", "tab-1", "https://example.com/login", "Login Page", "agent")
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	entries, err := Query(testBrowser, Filter{Grep: "login"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(entries) == 0 {
		t.Errorf("expected matching entry for 'login'")
	}

	if entries[0].URL != "https://example.com/login" {
		t.Errorf("unexpected URL: %s", entries[0].URL)
	}
}
