// SPDX-License-Identifier: GPL-3.0-only

package diagnostics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLitellmPythonFindsAdjacentInterpreter(t *testing.T) {
	dir := t.TempDir()

	// Simulate a litellm binary and an adjacent python3 interpreter.
	litellmBin := filepath.Join(dir, "litellm")
	if err := os.WriteFile(litellmBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	pythonBin := filepath.Join(dir, "python3")
	if err := os.WriteFile(pythonBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// EvalSymlinks normalises the path on macOS where t.TempDir() returns a
	// /var/... symlink to /private/var/...
	wantPython, _ := filepath.EvalSymlinks(pythonBin)
	got := litellmPython(litellmBin)
	if got != wantPython {
		t.Errorf("litellmPython(%q) = %q, want %q", litellmBin, got, wantPython)
	}
}

func TestLitellmPythonFollowsSymlink(t *testing.T) {
	dir := t.TempDir()

	// Simulate a pipx layout: real binary and Python live inside a venv bin/,
	// but the PATH-visible entry is a symlink in a separate directory.
	venvBin := filepath.Join(dir, "venv", "bin")
	if err := os.MkdirAll(venvBin, 0o755); err != nil {
		t.Fatal(err)
	}
	realBin := filepath.Join(venvBin, "litellm")
	if err := os.WriteFile(realBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	pythonBin := filepath.Join(venvBin, "python3")
	if err := os.WriteFile(pythonBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	pathBin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(pathBin, 0o755); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(pathBin, "litellm")
	if err := os.Symlink(realBin, symlink); err != nil {
		t.Fatal(err)
	}

	// litellmPython must follow the symlink and find python3 in the venv.
	// EvalSymlinks normalises the path on macOS where t.TempDir() returns a
	// /var/... symlink to /private/var/...
	wantPython, _ := filepath.EvalSymlinks(pythonBin)
	got := litellmPython(symlink)
	if got != wantPython {
		t.Errorf("litellmPython(symlink) = %q, want %q", got, wantPython)
	}
}

func TestLitellmPythonReturnsEmptyWhenNoInterpreter(t *testing.T) {
	dir := t.TempDir()
	litellmBin := filepath.Join(dir, "litellm")
	if err := os.WriteFile(litellmBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := litellmPython(litellmBin)
	if got != "" {
		t.Errorf("litellmPython with no adjacent interpreter: got %q, want %q", got, "")
	}
}

func TestPortFreeReturnsTrueForUnusedPort(t *testing.T) {
	// Port 0 lets the OS pick a free port; we immediately check a high port
	// that should be free on any CI machine.
	if !PortFree(0) {
		// Port 0 binding always succeeds; a false result is a bug.
		t.Error("PortFree(0) returned false, expected true")
	}
}
