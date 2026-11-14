package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bxw/internal/exit"
	"bxw/internal/osport"
)

// launch starts an isolated or persistent profile browser instance with remote
// debugging on a free fixed port. This is the browser-independent path (Pillar A):
// it uses a dedicated persistent profile under %LOCALAPPDATA%\bxw\profiles\<browser>
// so the user signs in once and credentials persist across sessions.
func (d *Daemon) launch(headless bool, targetURL string, realProfile bool) error {
	p := osport.Current()
	port, err := freePort()
	if err != nil {
		return err
	}
	if targetURL == "" {
		targetURL = "about:blank"
	}

	var profileDir string
	if realProfile {
		if d.br.Name != "brave" {
			return exit.New(exit.User, "real-profile remote debugging is blocked on %s by Chromium 136; use persistent profile instead (default)", d.br.Label)
		}
		profileDir = d.br.DataDir
	} else {
		profileDir = p.ProfileDir(d.br.Name)
	}
	_ = os.MkdirAll(profileDir, 0o755)

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--user-data-dir=" + profileDir,
		"--no-first-run",
		"--no-default-browser-check",
	}
	if headless {
		args = append(args, "--headless=new")
	}
	args = append(args, "--new-window", targetURL)

	pid, err := p.SpawnDetached(d.br.Exe, args)
	if err != nil {
		return err
	}
	if err := waitCDP(port, 20*time.Second); err != nil {
		return err
	}
	d.mu.Lock()
	d.cdpPort = port
	d.agentPID = pid
	d.mu.Unlock()
	_ = os.WriteFile(portFile(p, d.br.Name), []byte(fmt.Sprint(port)), 0o644)
	return nil
}

// listTabs returns the browser's page targets via the DevTools /json endpoint.
func (d *Daemon) listTabs() ([]Tab, error) {
	return d.listTabsFiltered("")
}

// listTabsFiltered returns page targets, optionally filtered by profile.
func (d *Daemon) listTabsFiltered(profileRef string) ([]Tab, error) {
	d.mu.Lock()
	port := d.cdpPort
	d.mu.Unlock()
	if port == 0 {
		discovered, err := d.discoverPort()
		if err != nil {
			return nil, err
		}
		port = discovered
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).
		Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return nil, fmt.Errorf("CDP not reachable on 127.0.0.1:%d: %w (try: bxw launch %s)", port, err, d.br.Name)
	}
	defer resp.Body.Close()
	var all []Tab
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		return nil, err
	}

	pNames := d.profileNames()

	// Try getting browser target info for browserContextId mapping
	ctxMap := make(map[string]string) // targetID -> profileDir
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if bconn, err := d.browserConn(ctx); err == nil {
		if btgt, err := d.browserTargets(ctx, bconn); err == nil {
			d.mu.Lock()
			for _, t := range btgt {
				if dir, ok := d.profileOf[t.BrowserContextID]; ok {
					ctxMap[string(t.TargetID)] = dir
				}
			}
			d.mu.Unlock()
		}
		_ = bconn.Close()
	}

	var targetDir string
	if profileRef != "" {
		if p, err := d.resolveProfile(profileRef); err == nil {
			targetDir = p.Dir
		}
	}

	pages := make([]Tab, 0, len(all))
	for _, t := range all {
		if t.Type != "page" || strings.Contains(t.URL, "bxprobe-") || strings.Contains(t.URL, "bxwprobe-") {
			continue
		}
		if dir, ok := ctxMap[t.ID]; ok {
			t.ProfileDir = dir
			t.Profile = pNames[dir]
		}
		if targetDir != "" && t.ProfileDir != "" && t.ProfileDir != targetDir {
			continue
		}
		pages = append(pages, t)
	}
	return pages, nil
}

// discoverPort finds an active CDP port for this browser:
// 1. Checks everyday browser's DevToolsActivePort
// 2. Checks previously launched agent instance's .port file
func (d *Daemon) discoverPort() (int, error) {
	p := osport.Current()

	// 1. Everyday browser DevToolsActivePort
	portFileEveryday := filepath.Join(d.br.DataDir, "DevToolsActivePort")
	if data, err := os.ReadFile(portFileEveryday); err == nil {
		f := strings.Fields(string(data))
		if len(f) >= 1 {
			if port, err := strconv.Atoi(f[0]); err == nil && port > 0 {
				if err := waitCDP(port, time.Second); err == nil {
					d.mu.Lock()
					d.cdpPort = port
					d.mu.Unlock()
					return port, nil
				}
			}
		}
	}

	// 2. Agent instance .port file
	notRunning := fmt.Errorf("no %s is running with remote debugging (run: bxw launch %s or turn on debugging at %s)",
		d.br.Label, d.br.Name, d.br.Inspect)

	data, err := os.ReadFile(portFile(p, d.br.Name))
	if err != nil {
		return 0, notRunning
	}
	var port int
	if _, err := fmt.Sscan(string(data), &port); err != nil || port == 0 {
		return 0, notRunning
	}
	if err := waitCDP(port, time.Second); err != nil {
		return 0, notRunning
	}
	d.mu.Lock()
	d.cdpPort = port
	d.mu.Unlock()
	return port, nil
}

func portFile(p osport.Platform, browser string) string {
	newPath := filepath.Join(p.RunDir(), browser+".port")
	if _, err := os.Stat(newPath); err == nil {
		return newPath
	}
	legacyPath := filepath.Join(p.LocalAppData(), "bxw", browser+"-agent.port")
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath
	}
	return newPath
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitCDP(port int, d time.Duration) error {
	deadline := time.Now().Add(d)
	cli := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		if r, err := cli.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port)); err == nil {
			r.Body.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("CDP did not come up on 127.0.0.1:%d", port)
}
