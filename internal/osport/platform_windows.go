//go:build windows

package osport

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

type winPlatform struct{}

func current() Platform { return winPlatform{} }

func (winPlatform) LocalAppData() string { return os.Getenv("LOCALAPPDATA") }

// ExpandEnv expands Windows %VAR% references. (os.ExpandEnv only handles $VAR,
// which does not match Windows path conventions.)
func (winPlatform) ExpandEnv(s string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '%')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		rest := s[i+1:]
		j := strings.IndexByte(rest, '%')
		if j < 0 { // dangling '%': keep literal
			b.WriteByte('%')
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(os.Getenv(rest[:j]))
		s = rest[j+1:]
	}
}

// Browsers is the Windows browser table. Edge/Brave paths were verified on
// Windows 11 build 26300 (2026-11-13) — note Edge installs under the 32-bit
// Program Files directory even though it is a 64-bit app, so both roots are
// probed. Chrome/Vivaldi/Chromium follow the same install conventions.
func (winPlatform) Browsers() []BrowserDef {
	return []BrowserDef{
		{
			Name:    "edge",
			Exes:    []string{`%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`, `%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`},
			DataDir: `%LOCALAPPDATA%\Microsoft\Edge\User Data`,
		},
		{
			Name:    "brave",
			Exes:    []string{`%ProgramFiles%\BraveSoftware\Brave-Browser\Application\brave.exe`, `%ProgramFiles(x86)%\BraveSoftware\Brave-Browser\Application\brave.exe`},
			DataDir: `%LOCALAPPDATA%\BraveSoftware\Brave-Browser\User Data`,
		},
		{
			Name:    "chrome",
			Exes:    []string{`%ProgramFiles%\Google\Chrome\Application\chrome.exe`, `%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe`, `%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe`},
			DataDir: `%LOCALAPPDATA%\Google\Chrome\User Data`,
		},
		{
			Name:    "vivaldi",
			Exes:    []string{`%LOCALAPPDATA%\Vivaldi\Application\vivaldi.exe`, `%ProgramFiles%\Vivaldi\Application\vivaldi.exe`},
			DataDir: `%LOCALAPPDATA%\Vivaldi\User Data`,
		},
		{
			Name:    "chromium",
			Exes:    []string{`%LOCALAPPDATA%\Chromium\Application\chrome.exe`},
			DataDir: `%LOCALAPPDATA%\Chromium\User Data`,
		},
	}
}

// ResolveExe prefers the registry's App Paths entry (authoritative, survives
// non-default install locations) and falls back to the filesystem table.
func (p winPlatform) ResolveExe(def BrowserDef) (string, bool) {
	var exeName string
	for _, e := range def.Exes {
		if b := filepath.Base(p.ExpandEnv(e)); b != "" {
			exeName = b
			break
		}
	}
	if exeName != "" {
		if exe, ok := appPathsLookup(exeName); ok {
			if _, err := os.Stat(exe); err == nil {
				return exe, true
			}
		}
	}
	for _, e := range def.Exes {
		exe := p.ExpandEnv(e)
		if fi, err := os.Stat(exe); err == nil && !fi.IsDir() {
			return exe, true
		}
	}
	return "", false
}

// appPathsLookup reads HKLM App Paths for an executable's canonical path.
func appPathsLookup(exeName string) (string, bool) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\`+exeName,
		registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("")
	if err != nil || v == "" {
		return "", false
	}
	return v, true
}

func (p winPlatform) LocalStatePath(dataDir string) string {
	return filepath.Join(p.ExpandEnv(dataDir), "Local State")
}

func (p winPlatform) SocketPath(browser string) string {
	return filepath.Join(p.LocalAppData(), "bxw", browser+".sock")
}
