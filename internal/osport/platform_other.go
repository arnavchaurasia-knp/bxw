//go:build !windows

package osport

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Non-Windows builds compile but carry no browser table yet. This keeps the
// repo cross-compilable (so CI can vet/build on Linux) while Windows is the
// only supported target. macOS/Linux adapters land here later.
type stubPlatform struct{}

func current() Platform { return stubPlatform{} }

func (stubPlatform) LocalAppData() string                 { return os.Getenv("HOME") }
func (stubPlatform) ExpandEnv(s string) string            { return os.ExpandEnv(s) }
func (stubPlatform) Browsers() []BrowserDef               { return nil }
func (stubPlatform) ResolveExe(BrowserDef) (string, bool) { return "", false }
func (stubPlatform) LocalStatePath(dataDir string) string { return dataDir + "/Local State" }
func (stubPlatform) DataRoot() string {
	d := filepath.Join(os.Getenv("HOME"), ".local", "share", "bxw")
	_ = os.MkdirAll(d, 0o755)
	return d
}
func (stubPlatform) RunDir() string {
	d := filepath.Join(stubPlatform{}.DataRoot(), "run")
	_ = os.MkdirAll(d, 0o755)
	return d
}
func (stubPlatform) ProfilesDir() string {
	d := filepath.Join(stubPlatform{}.DataRoot(), "profiles")
	_ = os.MkdirAll(d, 0o755)
	return d
}
func (stubPlatform) ProfileDir(browser string) string {
	d := filepath.Join(stubPlatform{}.ProfilesDir(), browser)
	_ = os.MkdirAll(d, 0o755)
	return d
}
func (stubPlatform) StateDir(browser string) string {
	d := filepath.Join(stubPlatform{}.DataRoot(), "state", browser)
	_ = os.MkdirAll(d, 0o755)
	return d
}
func (stubPlatform) CacheDir() string {
	d := filepath.Join(stubPlatform{}.DataRoot(), "cache")
	_ = os.MkdirAll(d, 0o755)
	return d
}
func (stubPlatform) LogsDir() string {
	d := filepath.Join(stubPlatform{}.DataRoot(), "logs")
	_ = os.MkdirAll(d, 0o755)
	return d
}
func (stubPlatform) HistoryDir() string {
	d := filepath.Join(stubPlatform{}.DataRoot(), "history")
	_ = os.MkdirAll(d, 0o755)
	return d
}
func (stubPlatform) ContextDir() string {
	d := filepath.Join(stubPlatform{}.DataRoot(), "context")
	_ = os.MkdirAll(d, 0o755)
	return d
}
func (stubPlatform) ConfigFile() string {
	return filepath.Join(stubPlatform{}.DataRoot(), "config.toml")
}
func (stubPlatform) PermissionsFile() string {
	return filepath.Join(stubPlatform{}.DataRoot(), "permissions.json")
}
func (stubPlatform) Dir() string {
	return stubPlatform{}.RunDir()
}
func (stubPlatform) SocketPath(browser string) string {
	return filepath.Join(stubPlatform{}.RunDir(), browser+".sock")
}
func (stubPlatform) LockPath(browser string) string {
	return filepath.Join(stubPlatform{}.RunDir(), browser+".lock")
}
func (stubPlatform) LogPath(browser string) string {
	return filepath.Join(stubPlatform{}.LogsDir(), browser+".log")
}

func (stubPlatform) Lock(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	unlock := func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
	return unlock, nil
}

func (stubPlatform) SpawnDaemon(self, browser, logPath string) (*os.Process, io.ReadCloser, error) {
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.Command(self, "__daemon", "--browser", browser)
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logf.Close()
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		logf.Close()
		return nil, nil, err
	}
	logf.Close()
	return cmd.Process, stdout, nil
}

func (stubPlatform) SpawnBrowser(exe string, args []string) error {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	return nil
}

func (stubPlatform) SpawnDetached(name string, args []string) (int, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return pid, nil
}

func (stubPlatform) ProcessRunning(exeName string) bool { return false }

// KillByProfileDir is a best-effort no-op on non-Windows builds (which carry no
// browser table yet). Real macOS/Linux adapters will match on the profile path
// via the platform's process API, as the Windows adapter does.
func (stubPlatform) KillByProfileDir(profileDir string) (int, error) { return 0, nil }
