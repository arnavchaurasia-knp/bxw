package contextmgr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bxw/internal/osport"
)

// TabSummary is a lightweight overview of a browser tab.
type TabSummary struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Profile string `json:"profile,omitempty"`
}

// ActionSummary records a terse action taken by an agent.
type ActionSummary struct {
	Action string    `json:"action"` // e.g. "click e12", "fill e4"
	Result string    `json:"result"` // e.g. "navigated to https://..."
	Time   time.Time `json:"time"`
}

// SessionContext stores persistent session state for an agent.
type SessionContext struct {
	SessionID   string          `json:"session_id"`
	Browser     string          `json:"browser"`
	ActiveTab   string          `json:"active_tab"`
	OpenTabs    []TabSummary    `json:"open_tabs"`
	LastActions []ActionSummary `json:"last_actions"`
	Notes       []string        `json:"notes,omitempty"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// SaveSession persists a session context to disk.
func SaveSession(sc SessionContext) error {
	p := osport.Current()
	ctxDir := p.ContextDir()
	if err := os.MkdirAll(ctxDir, 0o755); err != nil {
		return err
	}

	sc.UpdatedAt = time.Now().UTC()
	if sc.SessionID == "" {
		sc.SessionID = "default"
	}

	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return err
	}

	target := filepath.Join(ctxDir, sc.SessionID+".json")
	return os.WriteFile(target, data, 0o644)
}

// LoadSession loads a session context by ID.
func LoadSession(sessionID string) (SessionContext, error) {
	if sessionID == "" {
		sessionID = "default"
	}
	p := osport.Current()
	target := filepath.Join(p.ContextDir(), sessionID+".json")
	data, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return SessionContext{SessionID: sessionID, UpdatedAt: time.Now().UTC()}, nil
		}
		return SessionContext{}, err
	}

	var sc SessionContext
	if err := json.Unmarshal(data, &sc); err != nil {
		return SessionContext{}, err
	}
	return sc, nil
}

// ListSessions returns all stored sessions sorted by most recent first.
func ListSessions() ([]SessionContext, error) {
	p := osport.Current()
	ctxDir := p.ContextDir()
	entries, err := os.ReadDir(ctxDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []SessionContext
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			name := strings.TrimSuffix(e.Name(), ".json")
			sc, err := LoadSession(name)
			if err == nil {
				sessions = append(sessions, sc)
			}
		}
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Summary formats a compact 5-10 line summary of the session context for the model.
func (sc SessionContext) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Session: %s (Browser: %s, ActiveTab: %s)\n", sc.SessionID, sc.Browser, sc.ActiveTab))
	sb.WriteString(fmt.Sprintf("Open Tabs (%d):\n", len(sc.OpenTabs)))
	for _, t := range sc.OpenTabs {
		sb.WriteString(fmt.Sprintf("  [%s] %s (%s)\n", t.ID, t.Title, t.URL))
	}
	if len(sc.LastActions) > 0 {
		sb.WriteString("Recent Actions:\n")
		limit := len(sc.LastActions)
		if limit > 5 {
			limit = 5
		}
		for i := len(sc.LastActions) - limit; i < len(sc.LastActions); i++ {
			a := sc.LastActions[i]
			sb.WriteString(fmt.Sprintf("  • %s -> %s\n", a.Action, a.Result))
		}
	}
	if len(sc.Notes) > 0 {
		sb.WriteString("Notes:\n")
		for _, n := range sc.Notes {
			sb.WriteString(fmt.Sprintf("  - %s\n", n))
		}
	}
	return strings.TrimSpace(sb.String())
}
