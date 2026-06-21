// SPDX-License-Identifier: GPL-3.0-only
// Package paths resolves AgentRoute's own state directories.
//
// AgentRoute never writes its own state into a linked tool's config
// directory (e.g. ~/.claude/). All AgentRoute state lives under the OS
// standard config directory, in an "AgentRoute" subfolder:
//
//	Windows: %APPDATA%\AgentRoute\
//	macOS:   ~/Library/Application Support/AgentRoute/
//	Linux:   ~/.config/agentroute/ (respects $XDG_CONFIG_HOME via os.UserConfigDir)
package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

const appDirName = "AgentRoute"

// linuxDirName is used instead of appDirName on Linux/BSD to match XDG
// lowercase-hyphenated convention; os.UserConfigDir already returns
// $XDG_CONFIG_HOME (or ~/.config) on those platforms.
const linuxDirName = "agentroute"

// Root returns AgentRoute's top-level state directory, creating it if it
// does not already exist.
func Root() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	name := appDirName
	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" {
		name = linuxDirName
	}

	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// subdir joins Root() with the given path elements, creating the resulting
// directory if it does not already exist.
func subdir(elem ...string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(append([]string{root}, elem...)...)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// ConfigFile returns the path to AgentRoute's main app config file.
func ConfigFile() (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "config.toml"), nil
}

// ProfilesDir returns (creating if needed) the directory holding saved
// profile JSON files.
func ProfilesDir() (string, error) {
	return subdir("profiles")
}

// GatewayStateFile returns the path to the file recording the running
// gateway's port, session auth token, and pid.
func GatewayStateFile() (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "gateway.json"), nil
}

// SidecarDir returns (creating if needed) the directory holding the
// rendered LiteLLM config, its pid file, and its log file.
func SidecarDir() (string, error) {
	return subdir("sidecar")
}

// LogsDir returns (creating if needed) AgentRoute's own log directory.
func LogsDir() (string, error) {
	return subdir("logs")
}

// LinkStateFile returns the path to the file recording exactly which keys
// AgentRoute wrote into a given platform's config, so Unlink can remove
// precisely those keys and nothing else.
func LinkStateFile(platformID string) (string, error) {
	dir, err := subdir("links")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, platformID+".json"), nil
}
