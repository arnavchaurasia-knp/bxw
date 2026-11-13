package daemon

import (
	"strings"
	"unicode"

	"bxw/internal/exit"
)

type keyDef struct {
	vk   int64
	key  string
	code string
}

var keyDefs = map[string]keyDef{
	"enter":      {vk: 13, key: "Enter", code: "Enter"},
	"tab":        {vk: 9, key: "Tab", code: "Tab"},
	"escape":     {vk: 27, key: "Escape", code: "Escape"},
	"esc":        {vk: 27, key: "Escape", code: "Escape"},
	"backspace":  {vk: 8, key: "Backspace", code: "Backspace"},
	"delete":     {vk: 46, key: "Delete", code: "Delete"},
	"del":        {vk: 46, key: "Delete", code: "Delete"},
	"space":      {vk: 32, key: " ", code: "Space"},
	"arrowup":    {vk: 38, key: "ArrowUp", code: "ArrowUp"},
	"up":         {vk: 38, key: "ArrowUp", code: "ArrowUp"},
	"arrowdown":  {vk: 40, key: "ArrowDown", code: "ArrowDown"},
	"down":       {vk: 40, key: "ArrowDown", code: "ArrowDown"},
	"arrowleft":  {vk: 37, key: "ArrowLeft", code: "ArrowLeft"},
	"left":       {vk: 37, key: "ArrowLeft", code: "ArrowLeft"},
	"arrowright": {vk: 39, key: "ArrowRight", code: "ArrowRight"},
	"right":      {vk: 39, key: "ArrowRight", code: "ArrowRight"},
	"pageup":     {vk: 33, key: "PageUp", code: "PageUp"},
	"pagedown":   {vk: 34, key: "PageDown", code: "PageDown"},
	"home":       {vk: 36, key: "Home", code: "Home"},
	"end":        {vk: 35, key: "End", code: "End"},
	"insert":     {vk: 45, key: "Insert", code: "Insert"},
	"f1":         {vk: 112, key: "F1", code: "F1"},
	"f2":         {vk: 113, key: "F2", code: "F2"},
	"f3":         {vk: 114, key: "F3", code: "F3"},
	"f4":         {vk: 115, key: "F4", code: "F4"},
	"f5":         {vk: 116, key: "F5", code: "F5"},
	"f6":         {vk: 117, key: "F6", code: "F6"},
	"f7":         {vk: 118, key: "F7", code: "F7"},
	"f8":         {vk: 119, key: "F8", code: "F8"},
	"f9":         {vk: 120, key: "F9", code: "F9"},
	"f10":        {vk: 121, key: "F10", code: "F10"},
	"f11":        {vk: 122, key: "F11", code: "F11"},
	"f12":        {vk: 123, key: "F12", code: "F12"},
}

// keyCombo represents a parsed shortcut like "Control+A" or "Enter".
type keyCombo struct {
	vk        int64
	key       string
	code      string
	text      string
	modifiers int64
	commands  []string
}

// parseCombo parses key combinations like "Enter", "Control+A", "Shift+Tab".
func parseCombo(s string) (keyCombo, error) {
	parts := strings.Split(s, "+")
	if strings.HasSuffix(s, "++") {
		parts = append(parts[:len(parts)-2], "+")
	}

	var mods int64
	var keyPart string

	for i, p := range parts {
		if i < len(parts)-1 {
			switch strings.ToLower(strings.TrimSpace(p)) {
			case "alt":
				mods |= 1
			case "ctrl", "control":
				mods |= 2
			case "cmd", "command", "meta", "win", "windows":
				mods |= 4
			case "shift":
				mods |= 8
			default:
				return keyCombo{}, exit.New(exit.Validation, "unknown modifier %q in %q (use Control, Shift, Alt or Meta)", p, s)
			}
			continue
		}
		keyPart = strings.TrimSpace(p)
	}

	if keyPart == "" {
		return keyCombo{}, exit.New(exit.Validation, "key combo %q is missing a key name", s)
	}

	// Check if known special key
	if kd, ok := keyDefs[strings.ToLower(keyPart)]; ok {
		return keyCombo{
			vk:        kd.vk,
			key:       kd.key,
			code:      kd.code,
			modifiers: mods,
		}, nil
	}

	// Single character key (e.g. 'a', '1', '/')
	runes := []rune(keyPart)
	if len(runes) == 1 {
		r := runes[0]
		vk := int64(unicode.ToUpper(r))
		code := "Key" + string(unicode.ToUpper(r))
		if unicode.IsDigit(r) {
			code = "Digit" + string(r)
		}

		var cmds []string
		// Common editing commands for Ctrl/Meta + key
		if (mods&2 != 0) || (mods&4 != 0) {
			switch strings.ToLower(string(r)) {
			case "a":
				cmds = []string{"selectAll"}
			case "c":
				cmds = []string{"copy"}
			case "v":
				cmds = []string{"paste"}
			case "x":
				cmds = []string{"cut"}
			case "z":
				if mods&8 != 0 {
					cmds = []string{"redo"}
				} else {
					cmds = []string{"undo"}
				}
			}
		}

		return keyCombo{
			vk:        vk,
			key:       string(r),
			code:      code,
			text:      string(r),
			modifiers: mods,
			commands:  cmds,
		}, nil
	}

	return keyCombo{}, exit.New(exit.Validation, "unknown key %q (use a DOM key name such as Enter, Tab, Escape, ArrowDown, F5, or a single character)", keyPart)
}
