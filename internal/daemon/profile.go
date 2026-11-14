package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"

	"bxw/internal/browser"
	"bxw/internal/config"
	"bxw/internal/exit"
	"bxw/internal/osport"
)

func newNonce() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// browserWS returns the DevTools browser-level WebSocket URL from /json/version.
func (d *Daemon) browserWS() (string, error) {
	d.mu.Lock()
	port := d.cdpPort
	d.mu.Unlock()
	if port == 0 {
		discovered, err := d.discoverPort()
		if err != nil {
			return "", err
		}
		port = discovered
	}

	resp, err := (&http.Client{Timeout: 5 * time.Second}).
		Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return "", exit.New(exit.Upstream, "reach DevTools /json/version on port %d: %v", port, err)
	}
	defer resp.Body.Close()

	var ver struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ver); err != nil {
		return "", exit.New(exit.Upstream, "decode /json/version: %v", err)
	}
	if ver.WebSocketDebuggerURL == "" {
		return "", exit.New(exit.Upstream, "no webSocketDebuggerUrl reported by /json/version")
	}
	return ver.WebSocketDebuggerURL, nil
}

func (d *Daemon) browserConn(ctx context.Context) (*TabConn, error) {
	wsURL, err := d.browserWS()
	if err != nil {
		return nil, err
	}
	return dialTab(ctx, wsURL)
}

func (d *Daemon) profileNames() map[string]string {
	m := map[string]string{}
	ps, err := browser.Profiles(d.br)
	if err != nil {
		return m
	}
	for _, p := range ps {
		m[p.Dir] = p.Name
	}
	return m
}

func (d *Daemon) resolveProfile(ref string) (browser.Profile, error) {
	ps, err := browser.Profiles(d.br)
	if err != nil {
		// When Local State does not exist yet (e.g. brand-new profile or CI runner),
		// default to the standard "Default" profile.
		if ref == "" || strings.EqualFold(ref, "default") {
			return browser.Profile{Dir: "Default", Name: "Default"}, nil
		}
		return browser.Profile{}, err
	}
	if len(ps) == 0 {
		return browser.Profile{Dir: "Default", Name: "Default"}, nil
	}
	if ref == "" {
		return ps[0], nil
	}
	return browser.FindProfile(ps, ref)
}

func (d *Daemon) learn(ctx cdp.BrowserContextID, dir string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.profileOf == nil {
		d.profileOf = make(map[cdp.BrowserContextID]string)
	}
	for c, ddir := range d.profileOf {
		if ddir == dir {
			delete(d.profileOf, c)
		}
	}
	d.profileOf[ctx] = dir
}

func (d *Daemon) launchInProfile(profileDir, rawURL string) error {
	p := osport.Current()
	args := []string{"--profile-directory=" + profileDir, rawURL}

	d.mu.Lock()
	port := d.cdpPort
	isHeadless := d.headless
	d.mu.Unlock()

	// If this daemon manages an agent instance, pass its user data dir
	if port != 0 {
		agentDir := filepath.Join(p.LocalAppData(), "bxw", d.br.Name+"-agent")
		args = append([]string{"--user-data-dir=" + agentDir}, args...)
	}
	if isHeadless {
		args = append(args, "--headless=new")
	}

	_, err := p.SpawnDetached(d.br.Exe, args)
	return err
}

func (d *Daemon) browserTargets(ctx context.Context, bconn *TabConn) ([]*target.Info, error) {
	res, err := cdp.Call(ctx, bconn, target.GetTargets, target.GetTargetsParams{})
	if err != nil {
		return nil, err
	}
	return res.TargetInfos, nil
}

// contextFor returns the BrowserContextID of profileDir, running a nonce probe if unknown.
func (d *Daemon) contextFor(ctx context.Context, bconn *TabConn, dir string) (cdp.BrowserContextID, string, error) {
	targets, err := d.browserTargets(ctx, bconn)
	if err == nil {
		d.mu.Lock()
		for _, t := range targets {
			if t.Type == "page" && d.profileOf[t.BrowserContextID] == dir {
				d.mu.Unlock()
				return t.BrowserContextID, "", nil
			}
		}
		// Fresh profile / single profile Default: learn from first open page context.
		// On fresh profiles, Chromium may spawn first-run/welcome tabs (e.g. chrome://welcome)
		// whose BrowserContextID represents the default context.
		if len(d.profileOf) == 0 && (dir == "Default" || dir == "") {
			var fallbackTarget *target.Info
			for _, t := range targets {
				if t.Type == "page" {
					if !strings.HasPrefix(t.URL, "chrome://") && !strings.HasPrefix(t.URL, "edge://") {
						fallbackTarget = t
						break
					}
					if fallbackTarget == nil {
						fallbackTarget = t
					}
				}
			}
			if fallbackTarget != nil {
				d.profileOf[fallbackTarget.BrowserContextID] = "Default"
				d.mu.Unlock()
				return fallbackTarget.BrowserContextID, "", nil
			}
		}
		d.mu.Unlock()
	}

	// Not found; run nonce probe
	nonce := newNonce()
	probeURL := browser.ProbeURL(nonce)

	if err := d.launchInProfile(dir, probeURL); err != nil {
		return "", "", exit.New(exit.Upstream, "launch probe in %s: %v", dir, err)
	}

	deadline := time.Now().Add(10 * time.Second)
	marker := browser.ProbeMarker + nonce

	// On a fresh profile, Chrome spawns first-run/welcome tabs (chrome://welcome) that can
	// swallow or delay the probe URL navigation. We poll for the specific marker probe tab,
	// but if not seen after a short grace period, relax the predicate to accept the first available
	// page target (preferring non-internal pages, or accepting first-run pages) associated with the launch.
	var firstRunFallback *target.Info
	for time.Now().Before(deadline) && ctx.Err() == nil {
		all, err := d.browserTargets(ctx, bconn)
		if err == nil {
			for _, t := range all {
				if t.Type == "page" {
					if strings.Contains(t.URL, marker) {
						d.learn(t.BrowserContextID, dir)
						return t.BrowserContextID, string(t.TargetID), nil
					}
					// Track potential first-run / welcome tab fallback
					if firstRunFallback == nil {
						firstRunFallback = t
					} else if (strings.HasPrefix(firstRunFallback.URL, "chrome://") || strings.HasPrefix(firstRunFallback.URL, "edge://")) &&
						(!strings.HasPrefix(t.URL, "chrome://") && !strings.HasPrefix(t.URL, "edge://")) {
						firstRunFallback = t
					}
				}
			}
		}
		time.Sleep(150 * time.Millisecond)
	}

	// If probe tab was swallowed by a fresh profile first-run window, use the fallback page target
	if firstRunFallback != nil {
		d.learn(firstRunFallback.BrowserContextID, dir)
		return firstRunFallback.BrowserContextID, "", nil
	}

	return "", "", exit.New(exit.Upstream, "timed out waiting for probe tab in profile %s", dir)
}

func (d *Daemon) closeTarget(targetID string) error {
	if targetID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bconn, err := d.browserConn(ctx)
	if err != nil {
		return err
	}
	defer bconn.Close()

	_, err = cdp.Call(ctx, bconn, target.CloseTarget, target.CloseTargetParams{TargetID: target.ID(targetID)})
	return err
}

func (d *Daemon) open(profileRef, rawURL string, timeout time.Duration) (Tab, error) {
	if err := browser.CheckURL(rawURL); err != nil {
		return Tab{}, err
	}

	prof, err := d.resolveProfile(profileRef)
	if err != nil {
		return Tab{}, err
	}

	d.launchMu.Lock()
	defer d.launchMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	bconn, err := d.browserConn(ctx)
	if err != nil {
		return Tab{}, err
	}
	defer bconn.Close()

	ctxID, probeID, err := d.contextFor(ctx, bconn, prof.Dir)
	if err != nil {
		return Tab{}, err
	}
	if probeID != "" {
		defer func() {
			_ = d.closeTarget(probeID)
		}()
	}

	// Try Target.createTarget directly
	var newTargetID target.ID
	createParams := target.CreateTargetParams{URL: rawURL}
	if ctxID != "" {
		createParams.BrowserContextID = ctxID
	}
	res, err := cdp.Call(ctx, bconn, target.CreateTarget, createParams)
	if err == nil && res.TargetID != "" {
		newTargetID = res.TargetID
	}

	// Fallback to launchInProfile if createTarget did not return a target ID
	if newTargetID == "" {
		beforeTargets, err := d.browserTargets(ctx, bconn)
		if err != nil {
			return Tab{}, exit.New(exit.Upstream, "query targets before open: %v", err)
		}

		seen := make(map[target.ID]bool, len(beforeTargets))
		for _, t := range beforeTargets {
			seen[t.TargetID] = true
		}

		if err := d.launchInProfile(prof.Dir, rawURL); err != nil {
			return Tab{}, exit.New(exit.Upstream, "launch URL in profile %s: %v", prof.Dir, err)
		}

		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) && ctx.Err() == nil {
			current, err := d.browserTargets(ctx, bconn)
			if err == nil {
				for _, t := range current {
					if t.Type == "page" && !seen[t.TargetID] && string(t.TargetID) != probeID && t.BrowserContextID == ctxID {
						newTargetID = t.TargetID
						break
					}
				}
			}
			if newTargetID != "" {
				break
			}
			time.Sleep(150 * time.Millisecond)
		}
	}

	if newTargetID == "" {
		return Tab{}, exit.New(exit.Upstream, "timed out waiting for new tab in profile %s", prof.Name)
	}

	// Query target info to return title and URL
	title := ""
	allTargets, err := d.browserTargets(ctx, bconn)
	if err == nil {
		for _, t := range allTargets {
			if t.TargetID == newTargetID {
				title = t.Title
				break
			}
		}
	}

	tab := Tab{
		ID:         string(newTargetID),
		Type:       "page",
		Title:      title,
		URL:        rawURL,
		Profile:    prof.Name,
		ProfileDir: prof.Dir,
	}

	// Follow mode: bring newly opened tab to front
	if config.IsFollowEnabled() {
		_, _ = cdp.Call(ctx, bconn, target.ActivateTarget, target.ActivateTargetParams{TargetID: newTargetID})
	}

	return tab, nil
}
