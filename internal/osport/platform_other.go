//go:build !windows

package osport

// Non-Windows builds compile but carry no browser table yet. This keeps the
// repo cross-compilable (so CI can vet/build on Linux) while Windows is the
// only supported target. macOS/Linux adapters land here later.
type stubPlatform struct{}

func current() Platform { return stubPlatform{} }

func (stubPlatform) LocalAppData() string                 { return "" }
func (stubPlatform) ExpandEnv(s string) string            { return s }
func (stubPlatform) Browsers() []BrowserDef               { return nil }
func (stubPlatform) ResolveExe(BrowserDef) (string, bool) { return "", false }
func (stubPlatform) LocalStatePath(dataDir string) string { return dataDir + "/Local State" }
func (stubPlatform) SocketPath(browser string) string     { return "/tmp/bxw-" + browser + ".sock" }
