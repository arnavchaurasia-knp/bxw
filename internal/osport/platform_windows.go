//go:build windows

package osport

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
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
			Label:   "Microsoft Edge",
			Exes:    []string{`%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`, `%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`},
			DataDir: `%LOCALAPPDATA%\Microsoft\Edge\User Data`,
			Inspect: "edge://inspect/#remote-debugging",
		},
		{
			Name:    "brave",
			Label:   "Brave",
			Exes:    []string{`%ProgramFiles%\BraveSoftware\Brave-Browser\Application\brave.exe`, `%ProgramFiles(x86)%\BraveSoftware\Brave-Browser\Application\brave.exe`},
			DataDir: `%LOCALAPPDATA%\BraveSoftware\Brave-Browser\User Data`,
			Inspect: "brave://inspect/#remote-debugging",
		},
		{
			Name:    "chrome",
			Label:   "Google Chrome",
			Exes:    []string{`%ProgramFiles%\Google\Chrome\Application\chrome.exe`, `%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe`, `%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe`},
			DataDir: `%LOCALAPPDATA%\Google\Chrome\User Data`,
			Inspect: "chrome://inspect/#remote-debugging",
		},
		{
			Name:    "vivaldi",
			Label:   "Vivaldi",
			Exes:    []string{`%LOCALAPPDATA%\Vivaldi\Application\vivaldi.exe`, `%ProgramFiles%\Vivaldi\Application\vivaldi.exe`},
			DataDir: `%LOCALAPPDATA%\Vivaldi\User Data`,
			Inspect: "vivaldi://inspect/#remote-debugging",
		},
		{
			Name:    "chromium",
			Label:   "Chromium",
			Exes:    []string{`%LOCALAPPDATA%\Chromium\Application\chrome.exe`},
			DataDir: `%LOCALAPPDATA%\Chromium\User Data`,
			Inspect: "chrome://inspect/#remote-debugging",
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

func (p winPlatform) DataRoot() string {
	d := filepath.Join(p.LocalAppData(), "bxw")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func (p winPlatform) RunDir() string {
	d := filepath.Join(p.DataRoot(), "run")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func (p winPlatform) ProfilesDir() string {
	d := filepath.Join(p.DataRoot(), "profiles")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func (p winPlatform) ProfileDir(browser string) string {
	d := filepath.Join(p.ProfilesDir(), browser)
	_ = os.MkdirAll(d, 0o755)
	return d
}

func (p winPlatform) StateDir(browser string) string {
	d := filepath.Join(p.DataRoot(), "state", browser)
	_ = os.MkdirAll(d, 0o755)
	return d
}

func (p winPlatform) CacheDir() string {
	d := filepath.Join(p.DataRoot(), "cache")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func (p winPlatform) LogsDir() string {
	d := filepath.Join(p.DataRoot(), "logs")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func (p winPlatform) HistoryDir() string {
	d := filepath.Join(p.DataRoot(), "history")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func (p winPlatform) ContextDir() string {
	d := filepath.Join(p.DataRoot(), "context")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func (p winPlatform) ConfigFile() string {
	return filepath.Join(p.DataRoot(), "config.toml")
}

func (p winPlatform) PermissionsFile() string {
	return filepath.Join(p.DataRoot(), "permissions.json")
}

func (p winPlatform) Dir() string {
	return p.RunDir()
}

func (p winPlatform) SocketPath(browser string) string {
	return filepath.Join(p.RunDir(), browser+".sock")
}

func (p winPlatform) LockPath(browser string) string {
	return filepath.Join(p.RunDir(), browser+".lock")
}

func (p winPlatform) LogPath(browser string) string {
	return filepath.Join(p.LogsDir(), browser+".log")
}

// Lock acquires a blocking exclusive lock on path using LockFileEx.
// The returned unlock function releases the lock and closes the file.
func (winPlatform) Lock(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	ol := new(windows.Overlapped)
	err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, ol)
	if err != nil {
		f.Close()
		return nil, err
	}
	unlock := func() {
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
		_ = f.Close()
	}
	return unlock, nil
}

// SpawnDaemon launches the child daemon process detached with CREATE_NEW_PROCESS_GROUP
// and DETACHED_PROCESS, logging stderr to logPath and returning stdout for the startup handshake.
func (winPlatform) SpawnDaemon(self, browser, logPath string) (*os.Process, io.ReadCloser, error) {
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.Command(self, "__daemon", "--browser", browser)
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logf.Close()
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		logf.Close()
		return nil, nil, err
	}
	logf.Close() // child process keeps its inherited handle
	return cmd.Process, stdout, nil
}

// SpawnBrowser starts a browser executable detached in the background.
func (winPlatform) SpawnBrowser(exe string, args []string) error {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	return nil
}

// SpawnDetached starts a process in its own group, detached from any console.
func (winPlatform) SpawnDetached(name string, args []string) (int, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return pid, nil
}

// ProcessRunning reports whether any running process has the given executable name.
func (winPlatform) ProcessRunning(exeName string) bool {
	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	for err := windows.Process32First(h, &pe); err == nil; err = windows.Process32Next(h, &pe) {
		name := windows.UTF16ToString(pe.ExeFile[:])
		if strings.EqualFold(name, exeName) {
			return true
		}
	}
	return false
}

// KillByProfileDir terminates every process whose command line contains the
// given profile directory (as passed via --user-data-dir). Chromium propagates
// --user-data-dir to all child processes (renderers, GPU, utility, crashpad),
// so matching the profile path kills the whole tree. The path is unique to a
// bxw-managed instance, so the user's own browser (a different profile) is never
// touched.
func (winPlatform) KillByProfileDir(profileDir string) (int, error) {
	if strings.TrimSpace(profileDir) == "" {
		return 0, nil
	}
	needle := strings.ToLower(profileDir)

	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(h)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	killed := 0
	for err := windows.Process32First(h, &pe); err == nil; err = windows.Process32Next(h, &pe) {
		pid := pe.ProcessID
		if pid == 0 {
			continue
		}
		cl := processCommandLine(pid)
		if cl == "" || !strings.Contains(strings.ToLower(cl), needle) {
			continue
		}
		if kh, e := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid); e == nil {
			if windows.TerminateProcess(kh, 1) == nil {
				killed++
			}
			windows.CloseHandle(kh)
		}
	}
	return killed, nil
}

// processCommandLine reads another process's command line out of its PEB.
// Offsets are the stable 64-bit Windows layout: PEB.ProcessParameters at 0x20,
// RTL_USER_PROCESS_PARAMETERS.CommandLine (a UNICODE_STRING: Length u16 at 0x70,
// Buffer pointer at 0x78). Returns "" on any failure (access denied, exited).
func processCommandLine(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)

	var pbi windows.PROCESS_BASIC_INFORMATION
	var retLen uint32
	if err := windows.NtQueryInformationProcess(h, windows.ProcessBasicInformation,
		unsafe.Pointer(&pbi), uint32(unsafe.Sizeof(pbi)), &retLen); err != nil {
		return ""
	}
	peb := uintptr(unsafe.Pointer(pbi.PebBaseAddress))
	if peb == 0 {
		return ""
	}

	var procParams uintptr
	if !readRemote(h, peb+0x20, unsafe.Pointer(&procParams), unsafe.Sizeof(procParams)) || procParams == 0 {
		return ""
	}
	var clLen uint16
	if !readRemote(h, procParams+0x70, unsafe.Pointer(&clLen), unsafe.Sizeof(clLen)) || clLen == 0 {
		return ""
	}
	var clBuf uintptr
	if !readRemote(h, procParams+0x78, unsafe.Pointer(&clBuf), unsafe.Sizeof(clBuf)) || clBuf == 0 {
		return ""
	}
	buf := make([]uint16, clLen/2)
	if !readRemote(h, clBuf, unsafe.Pointer(&buf[0]), uintptr(clLen)) {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// readRemote reads size bytes from addr in process h into out. Reports success
// only on a full read.
func readRemote(h windows.Handle, addr uintptr, out unsafe.Pointer, size uintptr) bool {
	var n uintptr
	err := windows.ReadProcessMemory(h, addr, (*byte)(out), size, &n)
	return err == nil && n == size
}
