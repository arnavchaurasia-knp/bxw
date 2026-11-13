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
func (stubPlatform) Dir() string {
	d := filepath.Join(os.Getenv("HOME"), ".config", "bxw")
	_ = os.MkdirAll(d, 0o755)
	return d
}
func (stubPlatform) SocketPath(browser string) string { return filepath.Join(stubPlatform{}.Dir(), browser+".sock") }
func (stubPlatform) LockPath(browser string) string   { return filepath.Join(stubPlatform{}.Dir(), browser+".lock") }
func (stubPlatform) LogPath(browser string) string    { return filepath.Join(stubPlatform{}.Dir(), browser+".log") }

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


