// SPDX-License-Identifier: GPL-3.0-only
package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "file.txt")

	if err := AtomicWrite(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}

	// No leftover temp files.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 file in dir, got %d: %v", len(entries), entries)
	}
}

func TestBackupAndRestoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := []byte(`{"env":{}}`)

	if err := AtomicWrite(path, original, 0o600); err != nil {
		t.Fatalf("seed AtomicWrite: %v", err)
	}

	if err := BackupIfMissing(path); err != nil {
		t.Fatalf("BackupIfMissing: %v", err)
	}
	if _, err := os.Stat(BackupPath(path)); err != nil {
		t.Fatalf("expected backup to exist: %v", err)
	}

	// Simulate Link() mutating the file.
	modified := []byte(`{"env":{"ANTHROPIC_BASE_URL":"http://127.0.0.1:4505"}}`)
	if err := AtomicWrite(path, modified, 0o600); err != nil {
		t.Fatalf("mutate AtomicWrite: %v", err)
	}

	// A second BackupIfMissing must NOT clobber the original backup with
	// the now-modified file.
	if err := BackupIfMissing(path); err != nil {
		t.Fatalf("second BackupIfMissing: %v", err)
	}
	backupData, err := os.ReadFile(BackupPath(path))
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backupData) != string(original) {
		t.Fatalf("backup was clobbered: got %q, want %q", backupData, original)
	}

	if err := RestoreFromBackup(path); err != nil {
		t.Fatalf("RestoreFromBackup: %v", err)
	}

	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(restored) != string(original) {
		t.Fatalf("restored = %q, want byte-identical %q", restored, original)
	}

	if _, err := os.Stat(BackupPath(path)); !os.IsNotExist(err) {
		t.Fatalf("expected backup to be removed after restore, stat err = %v", err)
	}
}

func TestRestoreFromBackupNoopWhenNoBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")
	if err := RestoreFromBackup(path); err != nil {
		t.Fatalf("expected no-op, got error: %v", err)
	}
}

// TestRestoreFromBackupDeletesFileLinkCreatedFromNothing covers the case
// BackupAndRestoreRoundTrip doesn't: settings.json doesn't exist before
// Link (first-time user). BackupIfMissing must record that absence, and
// RestoreFromBackup must delete the file Link created rather than leaving
// it (and AgentRoute's env keys) in place.
func TestRestoreFromBackupDeletesFileLinkCreatedFromNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := BackupIfMissing(path); err != nil {
		t.Fatalf("BackupIfMissing on absent file: %v", err)
	}
	if _, err := os.Stat(BackupPath(path)); !os.IsNotExist(err) {
		t.Fatalf("expected no real backup file for an absent original, stat err = %v", err)
	}

	// Simulate Link() creating the file fresh.
	if err := AtomicWrite(path, []byte(`{"env":{"ANTHROPIC_BASE_URL":"http://127.0.0.1:4505"}}`), 0o600); err != nil {
		t.Fatalf("seed AtomicWrite: %v", err)
	}

	// A second BackupIfMissing (e.g. a re-run of Link) must not overwrite
	// the recorded "was absent" fact with the now-created file.
	if err := BackupIfMissing(path); err != nil {
		t.Fatalf("second BackupIfMissing: %v", err)
	}

	if err := RestoreFromBackup(path); err != nil {
		t.Fatalf("RestoreFromBackup: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted on restore, stat err = %v", err)
	}
	if _, err := os.Stat(BackupPath(path) + ".absent"); !os.IsNotExist(err) {
		t.Fatalf("expected absent-marker to be cleaned up, stat err = %v", err)
	}
}
