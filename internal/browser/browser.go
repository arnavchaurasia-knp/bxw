// Package browser discovers installed Chromium browsers and reads their
// profiles. It is OS-agnostic: every host-specific detail comes from osport.
package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"bxw/internal/osport"
)

// Browser is an installed, resolved browser.
type Browser struct {
	Name    string // canonical key: edge, brave, chrome, ...
	Exe     string // absolute executable path
	DataDir string // absolute user-data directory
}

// Profile is one browser profile: its on-disk folder and human name.
type Profile struct {
	Dir  string // folder name under User Data: "Default", "Profile 1"
	Name string // display name from Local State
}

// Installed returns the browsers actually present on this machine.
func Installed() []Browser {
	p := osport.Current()
	var out []Browser
	for _, def := range p.Browsers() {
		exe, ok := p.ResolveExe(def)
		if !ok {
			continue
		}
		out = append(out, Browser{
			Name:    def.Name,
			Exe:     exe,
			DataDir: p.ExpandEnv(def.DataDir),
		})
	}
	return out
}

// Find returns the installed browser with the given canonical name.
func Find(name string) (Browser, bool) {
	for _, b := range Installed() {
		if b.Name == name {
			return b, true
		}
	}
	return Browser{}, false
}

// localState is the minimal shape of a Chromium "Local State" file we read.
type localState struct {
	Profile struct {
		InfoCache map[string]struct {
			Name string `json:"name"`
		} `json:"info_cache"`
	} `json:"profile"`
}

// Profiles reads the browser's "Local State" and returns its profiles,
// sorted by folder name.
func Profiles(b Browser) ([]Profile, error) {
	p := osport.Current()
	path := p.LocalStatePath(b.DataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Local State: %w", err)
	}
	var ls localState
	if err := json.Unmarshal(data, &ls); err != nil {
		return nil, fmt.Errorf("parse Local State: %w", err)
	}
	out := make([]Profile, 0, len(ls.Profile.InfoCache))
	for dir, info := range ls.Profile.InfoCache {
		out = append(out, Profile{Dir: dir, Name: info.Name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out, nil
}
