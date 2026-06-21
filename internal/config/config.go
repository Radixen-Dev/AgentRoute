// SPDX-License-Identifier: GPL-3.0-only

// Package config loads and saves AgentRoute's own application configuration
// (internal/config/config.go), distinct from any third-party tool's config
// that a platform adapter may edit.
package config

import (
	"bytes"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/Radixen-Dev/AgentRoute/internal/fsutil"
	"github.com/Radixen-Dev/AgentRoute/internal/paths"
)

// DefaultPort is the gateway's default listen port. If already in use,
// the gateway picks the next free port automatically (see gateway.PickPort).
const DefaultPort = 4505

// ActiveProfile is the name of the profile to use when none is specified
// via --profile. Stored as "" until the user activates one.
type Config struct {
	ActiveProfile string `toml:"active_profile"`
	Port          int    `toml:"port"`
	ReduceMotion  bool   `toml:"reduce_motion"`
}

// defaults returns a Config with AgentRoute's built-in defaults.
func defaults() Config {
	return Config{Port: DefaultPort}
}

// Load reads AgentRoute's app config file, returning defaults if it does
// not yet exist.
func Load() (Config, error) {
	path, err := paths.ConfigFile()
	if err != nil {
		return Config{}, fmt.Errorf("config: resolve path: %w", err)
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaults(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}

	cfg := defaults()
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return cfg, nil
}

// Save persists cfg to AgentRoute's app config file atomically.
func Save(cfg Config) error {
	path, err := paths.ConfigFile()
	if err != nil {
		return fmt.Errorf("config: resolve path: %w", err)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}

	return fsutil.AtomicWrite(path, buf.Bytes(), 0o600)
}
