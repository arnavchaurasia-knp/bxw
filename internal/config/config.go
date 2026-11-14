package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bxw/internal/osport"
)

// Config represents bxw configuration settings.
type Config struct {
	DefaultBrowser string `json:"default_browser"`
	PortMin        int    `json:"port_min"`
	PortMax        int    `json:"port_max"`
	CacheMaxMB     int    `json:"cache_max_mb"`
	HistoryDays    int    `json:"history_days"`
	CacheDays      int    `json:"cache_days"`
}

// Default returns a Config with production defaults.
func Default() Config {
	return Config{
		DefaultBrowser: "edge",
		PortMin:        50000,
		PortMax:        65000,
		CacheMaxMB:     500,
		HistoryDays:    30,
		CacheDays:      7,
	}
}

// Load loads the configuration from disk, falling back to defaults if not present.
func Load() Config {
	cfg := Default()
	p := osport.Current()
	f, err := os.Open(p.ConfigFile())
	if err != nil {
		return cfg
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	currentSection := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[] ")
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)

		switch {
		case currentSection == "" && key == "default_browser":
			cfg.DefaultBrowser = val
		case currentSection == "" && key == "port_min":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.PortMin = n
			}
		case currentSection == "" && key == "port_max":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.PortMax = n
			}
		case currentSection == "cache" && key == "max_mb":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.CacheMaxMB = n
			}
		case currentSection == "cache" && key == "cache_days":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.CacheDays = n
			}
		case currentSection == "retention" && key == "history_days":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.HistoryDays = n
			}
		case currentSection == "retention" && key == "cache_days":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.CacheDays = n
			}
		}
	}
	return cfg
}

// Save writes the configuration to disk in TOML format.
func (c Config) Save() error {
	p := osport.Current()
	target := p.ConfigFile()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	content := fmt.Sprintf(`# bxw configuration
default_browser = %q
port_min = %d
port_max = %d

[cache]
max_mb = %d
cache_days = %d

[retention]
history_days = %d
`, c.DefaultBrowser, c.PortMin, c.PortMax, c.CacheMaxMB, c.CacheDays, c.HistoryDays)

	return os.WriteFile(target, []byte(content), 0o644)
}
