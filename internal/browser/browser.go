// Package browser discovers installed Chromium browsers and reads their
// profiles. It is OS-agnostic: every host-specific detail comes from osport.
package browser

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bxw/internal/exit"
	"bxw/internal/osport"
)

// Browser is an installed, resolved browser.
type Browser struct {
	Name    string // canonical key: edge, brave, chrome, ...
	Label   string // human name: Microsoft Edge, Brave, ...
	Exe     string // absolute executable path
	DataDir string // absolute user-data directory
	Inspect string // inspect URL: edge://inspect/#remote-debugging
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
			Label:   def.Label,
			Exe:     exe,
			DataDir: p.ExpandEnv(def.DataDir),
			Inspect: def.Inspect,
		})
	}
	return out
}

// Detect returns the first installed browser that has remote debugging on
// (its DevToolsActivePort exists), or the first installed browser if none.
func Detect() (Browser, bool) {
	bs := Installed()
	if len(bs) == 0 {
		return Browser{}, false
	}
	for _, b := range bs {
		portFile := filepath.Join(b.DataDir, "DevToolsActivePort")
		if _, err := os.Stat(portFile); err == nil {
			return b, true
		}
	}
	return bs[0], true
}

// AgentDataDir returns the isolated user data directory for an agent instance.
func (b Browser) AgentDataDir() string {
	p := osport.Current()
	d := filepath.Join(p.Dir(), b.Name+"-agent")
	_ = os.MkdirAll(d, 0o755)
	return d
}

// WSURL reads DevToolsActivePort from either everyday DataDir or AgentDataDir,
// returning the browser's WebSocket URL.
func (b Browser) WSURL() (string, error) {
	// 1. Try everyday browser's DevToolsActivePort
	portFile := filepath.Join(b.DataDir, "DevToolsActivePort")
	if data, err := os.ReadFile(portFile); err == nil {
		if ws, err := parsePort(data); err == nil {
			return ws, nil
		}
	}

	// 2. Try agent browser instance's DevToolsActivePort
	agentPortFile := filepath.Join(b.AgentDataDir(), "DevToolsActivePort")
	if data, err := os.ReadFile(agentPortFile); err == nil {
		if ws, err := parsePort(data); err == nil {
			return ws, nil
		}
	}

	return "", exit.New(exit.Upstream,
		"%s is not running with remote debugging on: run 'bxw launch %s' or turn it on at %s",
		b.Label, b.Name, b.Inspect)
}

// LaunchAgent launches an isolated agent browser instance and waits for DevToolsActivePort.
func (b Browser) LaunchAgent(headless bool) (string, error) {
	agentDir := b.AgentDataDir()
	portFile := filepath.Join(agentDir, "DevToolsActivePort")
	_ = os.Remove(portFile)

	args := []string{
		"--remote-debugging-port=0",
		"--user-data-dir=" + agentDir,
		"--no-first-run",
		"--no-default-browser-check",
	}
	if headless {
		args = append(args, "--headless=new")
	}

	p := osport.Current()
	if _, err := p.SpawnDetached(b.Exe, args); err != nil {
		return "", exit.New(exit.Upstream, "launch %s: %v", b.Label, err)
	}

	for i := 0; i < 50; i++ {
		time.Sleep(200 * time.Millisecond)
		if data, err := os.ReadFile(portFile); err == nil {
			if ws, err := parsePort(data); err == nil {
				return ws, nil
			}
		}
	}
	return "", exit.New(exit.Upstream, "%s started but did not write DevToolsActivePort within 10s", b.Label)
}

func parsePort(data []byte) (string, error) {
	f := strings.Fields(string(data))
	if len(f) < 2 || !strings.HasPrefix(f[1], "/devtools/browser/") {
		return "", exit.New(exit.Upstream, "DevToolsActivePort has unexpected contents")
	}
	return "ws://127.0.0.1:" + f[0] + f[1], nil
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

// FindProfile returns the profile whose display name or folder name matches ref.
func FindProfile(ps []Profile, ref string) (Profile, error) {
	for _, p := range ps {
		if strings.EqualFold(p.Name, ref) || strings.EqualFold(p.Dir, ref) {
			return p, nil
		}
	}
	var names []string
	for _, p := range ps {
		names = append(names, fmt.Sprintf("%q (%s)", p.Name, p.Dir))
	}
	return Profile{}, exit.New(exit.NotFound, "no profile matches %q (have: %s)", ref, strings.Join(names, ", "))
}

// CheckURL ensures the given URL uses an allowed scheme and cannot be parsed as a CLI flag.
func CheckURL(raw string) error {
	if strings.HasPrefix(raw, "-") {
		return exit.New(exit.Validation, "invalid URL %q", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return exit.New(exit.Validation, "invalid URL %q: %v", raw, err)
	}
	switch u.Scheme {
	case "http", "https", "file", "about", "data":
		return nil
	}
	return exit.New(exit.Validation, "URL %q must start with http://, https://, file://, about: or data:", raw)
}

// ProbeMarker starts the nonce in every probe URL, allowing bxw to hide its own probe tabs.
const ProbeMarker = "bxwprobe-"

// ProbeURL generates a lightweight data page carrying the probe marker and nonce.
func ProbeURL(nonce string) string {
	return "data:text/html,<title>bxw</title><!--" + ProbeMarker + nonce + "-->"
}
