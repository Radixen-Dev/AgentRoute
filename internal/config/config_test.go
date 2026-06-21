// SPDX-License-Identifier: GPL-3.0-only

package config

import (
	"os"
	"testing"

	"github.com/Radixen-Dev/AgentRoute/internal/paths"
)

func TestLoadDefaultsWhenMissing(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != DefaultPort {
		t.Fatalf("got port %d, want default %d", cfg.Port, DefaultPort)
	}
	if cfg.ActiveProfile != "" {
		t.Fatalf("expected empty ActiveProfile by default, got %q", cfg.ActiveProfile)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)

	want := Config{ActiveProfile: "work", Port: 5000, ReduceMotion: true}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestConfigFileHasRestrictivePerms(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("file mode bits are not meaningful on Windows")
	}
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)

	if err := Save(Config{Port: DefaultPort}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path, err := paths.ConfigFile()
	if err != nil {
		t.Fatalf("ConfigFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("got perm %o, want 0600", perm)
	}
}
