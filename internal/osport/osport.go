// Package osport is the single seam between the OS-agnostic core and the host
// operating system. The core depends only on the Platform interface; a new OS
// is a new file implementing current(). The core never branches on GOOS.
//
// This is what keeps bxw mergeable upstream into bx: the agnostic packages stay
// identical across platforms, and OS differences live behind this port.
package osport

import (
	"io"
	"os"
)

// BrowserDef describes where a browser's executable and user-data directory
// live, using %ENV% placeholders that each platform expands.
type BrowserDef struct {
	Name    string
	Label   string
	Exes    []string // candidate executable paths, most-specific first
	DataDir string   // user-data directory ("User Data" on Chromium)
	Inspect string   // URL where user enables remote debugging
}

// Platform abstracts every host-specific operation bxw relies on.
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
	// Dir is the directory where bxw stores sockets, locks, and logs.
	Dir() string
	// SocketPath is the daemon's per-browser IPC socket path.
	SocketPath(browser string) string
	// LockPath is the daemon's per-browser lock file path.
	LockPath(browser string) string
	// LogPath is the daemon's per-browser log file path.
	LogPath(browser string) string

	// Lock acquires a blocking exclusive lock on path. The returned function
	// releases the lock when called.
	Lock(path string) (unlock func(), err error)
	// SpawnDetached starts a process in its own group, detached from any console.
	SpawnDetached(name string, args []string) (pid int, err error)
	// SpawnDaemon starts the daemon process in the background, detached,
	// returning the process and its stdout pipe for reading the startup handshake.
	SpawnDaemon(self, browser, logPath string) (*os.Process, io.ReadCloser, error)
}

// Current returns the platform adapter for this build's GOOS.
func Current() Platform { return current() }
