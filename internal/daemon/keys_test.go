package daemon

import (
	"testing"
)

func TestParseCombo(t *testing.T) {
	tests := []struct {
		input     string
		wantKey   string
		wantVK    int64
		wantMods  int64
		wantCmd   string
		shouldErr bool
	}{
		{input: "Enter", wantKey: "Enter", wantVK: 13, wantMods: 0},
		{input: "Escape", wantKey: "Escape", wantVK: 27, wantMods: 0},
		{input: "Tab", wantKey: "Tab", wantVK: 9, wantMods: 0},
		{input: "Control+A", wantKey: "A", wantVK: 65, wantMods: 2, wantCmd: "selectAll"},
		{input: "Ctrl+c", wantKey: "c", wantVK: 67, wantMods: 2, wantCmd: "copy"},
		{input: "Shift+Tab", wantKey: "Tab", wantVK: 9, wantMods: 8},
		{input: "Alt+F4", wantKey: "F4", wantVK: 115, wantMods: 1},
		{input: "UnknownKey", shouldErr: true},
		{input: "BadMod+A", shouldErr: true},
	}

	for _, tc := range tests {
		c, err := parseCombo(tc.input)
		if tc.shouldErr {
			if err == nil {
				t.Errorf("parseCombo(%q) expected error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseCombo(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if c.key != tc.wantKey {
			t.Errorf("parseCombo(%q) key = %q, want %q", tc.input, c.key, tc.wantKey)
		}
		if c.vk != tc.wantVK {
			t.Errorf("parseCombo(%q) vk = %d, want %d", tc.input, c.vk, tc.wantVK)
		}
		if c.modifiers != tc.wantMods {
			t.Errorf("parseCombo(%q) mods = %d, want %d", tc.input, c.modifiers, tc.wantMods)
		}
		if tc.wantCmd != "" {
			if len(c.commands) == 0 || c.commands[0] != tc.wantCmd {
				t.Errorf("parseCombo(%q) commands = %v, want [%s]", tc.input, c.commands, tc.wantCmd)
			}
		}
	}
}
