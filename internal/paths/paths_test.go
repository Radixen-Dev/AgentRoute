// SPDX-License-Identifier: GPL-3.0-only

package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	root, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("expected Root() to create the directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("Root() path is not a directory: %s", root)
	}
}

func TestSubdirsNestUnderRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	root, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}

	profiles, err := ProfilesDir()
	if err != nil {
		t.Fatalf("ProfilesDir: %v", err)
	}
	if filepath.Dir(profiles) != root {
		t.Fatalf("ProfilesDir %q is not a direct child of Root %q", profiles, root)
	}

	sidecar, err := SidecarDir()
	if err != nil {
		t.Fatalf("SidecarDir: %v", err)
	}
	if filepath.Dir(sidecar) != root {
		t.Fatalf("SidecarDir %q is not a direct child of Root %q", sidecar, root)
	}
}

func TestLinkStateFileIsPerPlatform(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	a, err := LinkStateFile("claude-code")
	if err != nil {
		t.Fatalf("LinkStateFile: %v", err)
	}
	b, err := LinkStateFile("codex")
	if err != nil {
		t.Fatalf("LinkStateFile: %v", err)
	}
	if a == b {
		t.Fatalf("expected distinct paths per platform ID, got %q for both", a)
	}
}
