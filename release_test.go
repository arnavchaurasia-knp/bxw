package main

import (
	"encoding/json"
	"os"
	"testing"
)

// TestNpmVersionsConsistent guards the release packaging: the launcher and both
// per-arch subpackages must all carry the same version, and the launcher's
// optionalDependencies must pin that exact version. build.ps1 stamps these; this
// test fails loudly if a release build (or a hand edit) lets them drift apart —
// the mismatch that L13 verification surfaced (binary 0.1.0-rc1 vs package.json 0.1.0).
func TestNpmVersionsConsistent(t *testing.T) {
	type pkg struct {
		Version              string            `json:"version"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	read := func(path string) pkg {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var p pkg
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if p.Version == "" {
			t.Fatalf("%s has empty version", path)
		}
		return p
	}

	launcher := read("npm/bxw/package.json")
	x64 := read("npm/bxw-win32-x64/package.json")
	arm64 := read("npm/bxw-win32-arm64/package.json")

	if x64.Version != launcher.Version {
		t.Errorf("x64 subpackage version %q != launcher version %q", x64.Version, launcher.Version)
	}
	if arm64.Version != launcher.Version {
		t.Errorf("arm64 subpackage version %q != launcher version %q", arm64.Version, launcher.Version)
	}

	for _, dep := range []string{"bxw-win32-x64", "bxw-win32-arm64"} {
		got, ok := launcher.OptionalDependencies[dep]
		if !ok {
			t.Errorf("launcher optionalDependencies missing %q", dep)
			continue
		}
		if got != launcher.Version {
			t.Errorf("launcher pins %s@%q, want %q (must match launcher version)", dep, got, launcher.Version)
		}
	}
}
