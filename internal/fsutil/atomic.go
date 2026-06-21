// SPDX-License-Identifier: GPL-3.0-only

// Package fsutil provides small filesystem helpers used wherever AgentRoute
// must write state or edit a third-party config file safely.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWrite writes data to path by writing to a temporary file in the
// same directory and renaming it into place, so readers never observe a
// partially-written file. perm is applied to the temp file before rename.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("fsutil: create dir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("fsutil: create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// Best-effort cleanup if anything below fails before the rename.
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fsutil: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fsutil: close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("fsutil: chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("fsutil: rename into place: %w", err)
	}

	success = true
	return nil
}

// BackupPath returns the conventional backup path AgentRoute uses when it
// is about to surgically edit a third-party config file (e.g. a coding
// tool's settings.json).
func BackupPath(path string) string {
	return path + ".agentroute.bak"
}

// absentMarkerPath marks that, at the moment AgentRoute first touched path,
// the file did not exist yet. Without this, RestoreFromBackup cannot tell
// "Link was never called" (no backup, file absent — true no-op) apart from
// "Link created this file from nothing" (no backup, file now present —
// Unlink must delete it to restore the pre-Link state).
func absentMarkerPath(path string) string {
	return path + ".agentroute.bak.absent"
}

// BackupIfMissing copies path to its BackupPath, but only if that backup
// does not already exist. This ensures repeated Link() calls never
// overwrite the user's true original file with an already-AgentRoute-
// modified one. If path does not exist yet, it records that fact (via
// absentMarkerPath) instead, so RestoreFromBackup knows to delete the file
// Link is about to create rather than leaving it in place.
func BackupIfMissing(path string) error {
	backup := BackupPath(path)
	marker := absentMarkerPath(path)

	if _, err := os.Stat(backup); err == nil {
		return nil // backup already exists; do not overwrite it
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("fsutil: stat backup %s: %w", backup, err)
	}
	if _, err := os.Stat(marker); err == nil {
		return nil // already recorded "no original file" on a prior call
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("fsutil: stat absent-marker %s: %w", marker, err)
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return AtomicWrite(marker, []byte{}, 0o600)
	}
	if err != nil {
		return fmt.Errorf("fsutil: read %s for backup: %w", path, err)
	}

	info, err := os.Stat(path)
	perm := os.FileMode(0o600)
	if err == nil {
		perm = info.Mode().Perm()
	}
	return AtomicWrite(backup, data, perm)
}

// HasRecordedOriginal reports whether BackupIfMissing has recorded path's
// pre-Link state, as either a real backup or an absent-marker. Callers use
// this to decide whether RestoreFromBackup will actually do something, or
// whether they need a fallback restore strategy (e.g. the backup file was
// deleted out-of-band).
func HasRecordedOriginal(path string) bool {
	if _, err := os.Stat(BackupPath(path)); err == nil {
		return true
	}
	if _, err := os.Stat(absentMarkerPath(path)); err == nil {
		return true
	}
	return false
}

// RestoreFromBackup restores path to exactly the state it was in before
// the most recent BackupIfMissing call: if path did not exist then, it is
// removed; otherwise its prior bytes and permissions are restored. If
// BackupIfMissing was never called (no backup and no absent-marker), this
// is a no-op.
func RestoreFromBackup(path string) error {
	marker := absentMarkerPath(path)
	if _, err := os.Stat(marker); err == nil {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("fsutil: remove %s to restore absent state: %w", path, err)
		}
		return os.Remove(marker)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("fsutil: stat absent-marker %s: %w", marker, err)
	}

	backup := BackupPath(path)
	data, err := os.ReadFile(backup)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("fsutil: read backup %s: %w", backup, err)
	}

	info, statErr := os.Stat(backup)
	perm := os.FileMode(0o600)
	if statErr == nil {
		perm = info.Mode().Perm()
	}

	if err := AtomicWrite(path, data, perm); err != nil {
		return fmt.Errorf("fsutil: restore %s from backup: %w", path, err)
	}
	return os.Remove(backup)
}
