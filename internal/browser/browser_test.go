package browser

import (
	"os"
	"path/filepath"
	"testing"

	"bxw/internal/osport"
)

// TestProfiles verifies the Local State parser against a fixture, independent
// of any installed browser. Uses osport so the file path is resolved the same
// way production does.
func TestProfiles(t *testing.T) {
	dir := t.TempDir()
	ls := `{"profile":{"info_cache":{
		"Default":{"name":"Work"},
		"Profile 1":{"name":"Personal"}
	}}}`
	path := osport.Current().LocalStatePath(dir)
	if err := os.WriteFile(path, []byte(ls), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := Profiles(Browser{Name: "test", DataDir: dir})
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	want := []Profile{{Dir: "Default", Name: "Work"}, {Dir: "Profile 1", Name: "Personal"}}
	if len(got) != len(want) {
		t.Fatalf("got %d profiles, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("profile[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestProfilesMissing confirms a readable, classified error when Local State
// is absent (no panic, no silent empty).
func TestProfilesMissing(t *testing.T) {
	if _, err := Profiles(Browser{Name: "test", DataDir: filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Fatal("expected error for missing Local State, got nil")
	}
}
