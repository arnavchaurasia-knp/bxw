// Package osport is the single seam between the OS-agnostic core and the host
// operating system. The core depends only on the Platform interface; a new OS
// is a new file implementing current(). The core never branches on GOOS.
//
// This is what keeps bxw mergeable upstream into bx: the agnostic packages stay
// identical across platforms, and OS differences live behind this port.
package osport

// BrowserDef describes where a browser's executable and user-data directory
// live, using %ENV% placeholders that each platform expands.
type BrowserDef struct {
	Name    string
	Exes    []string // candidate executable paths, most-specific first
	DataDir string   // user-data directory ("User Data" on Chromium)
}

// Platform abstracts every host-specific operation bxw relies on. Discovery is
// implemented now; process/lock/spawn methods arrive with the daemon (M2+).
type Platform interface {
	// LocalAppData is the per-user state root (%LOCALAPPDATA% on Windows).
	LocalAppData() string
	// ExpandEnv resolves this platform's env-var syntax in a path.
	ExpandEnv(string) string
	// Browsers is the platform's table of supported browsers.
	Browsers() []BrowserDef
	// ResolveExe returns the first usable executable for a browser, if installed.
	ResolveExe(BrowserDef) (string, bool)
	// LocalStatePath is the absolute path to a data dir's "Local State" file.
	LocalStatePath(dataDir string) string
	// SocketPath is the daemon's per-browser IPC socket path.
	SocketPath(browser string) string
}

// Current returns the platform adapter for this build's GOOS.
func Current() Platform { return current() }
