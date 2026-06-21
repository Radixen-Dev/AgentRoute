// SPDX-License-Identifier: GPL-3.0-only

package secret

import (
	"errors"
	"testing"
)

// fakeKeyring is an in-memory keyringBackend for tests. It never touches
// the real OS credential store.
type fakeKeyring struct {
	values map[string]string
	// failSet, when true, makes Set always fail, simulating a machine
	// with no keyring backend available (e.g. headless Linux CI).
	failSet bool
}

func newFakeKeyring() *fakeKeyring { return &fakeKeyring{values: map[string]string{}} }

func key(service, user string) string { return service + "/" + user }

func (f *fakeKeyring) Get(service, user string) (string, error) {
	v, ok := f.values[key(service, user)]
	if !ok {
		return "", errors.New("secret not found in fake keyring")
	}
	return v, nil
}

func (f *fakeKeyring) Set(service, user, value string) error {
	if f.failSet {
		return errors.New("simulated: no keyring backend available")
	}
	f.values[key(service, user)] = value
	return nil
}

func (f *fakeKeyring) Delete(service, user string) error {
	delete(f.values, key(service, user))
	return nil
}

func withFakeKeyring(t *testing.T) *fakeKeyring {
	t.Helper()
	original := backend
	fake := newFakeKeyring()
	backend = fake
	t.Cleanup(func() { backend = original })
	return fake
}

func withIsolatedStateDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestEnvVarAlwaysWins(t *testing.T) {
	withFakeKeyring(t)
	withIsolatedStateDir(t)
	t.Setenv(envVar, "env-key")

	got, src, err := OpenRouterAPIKey()
	if err != nil {
		t.Fatalf("OpenRouterAPIKey: %v", err)
	}
	if got != "env-key" || src != SourceEnv {
		t.Fatalf("got (%q, %q), want (\"env-key\", env)", got, src)
	}
}

func TestSetAndGetViaKeyring(t *testing.T) {
	withFakeKeyring(t)
	withIsolatedStateDir(t)
	t.Setenv(envVar, "")

	src, err := SetOpenRouterAPIKey("ring-key")
	if err != nil {
		t.Fatalf("SetOpenRouterAPIKey: %v", err)
	}
	if src != SourceKeyring {
		t.Fatalf("got source %q, want keyring", src)
	}

	got, gotSrc, err := OpenRouterAPIKey()
	if err != nil {
		t.Fatalf("OpenRouterAPIKey: %v", err)
	}
	if got != "ring-key" || gotSrc != SourceKeyring {
		t.Fatalf("got (%q, %q), want (\"ring-key\", keyring)", got, gotSrc)
	}
}

func TestFallsBackToFileWhenKeyringUnavailable(t *testing.T) {
	fake := withFakeKeyring(t)
	fake.failSet = true
	withIsolatedStateDir(t)
	t.Setenv(envVar, "")

	src, err := SetOpenRouterAPIKey("file-key")
	if err != nil {
		t.Fatalf("SetOpenRouterAPIKey: %v", err)
	}
	if src != SourceFile {
		t.Fatalf("got source %q, want file-fallback", src)
	}

	got, gotSrc, err := OpenRouterAPIKey()
	if err != nil {
		t.Fatalf("OpenRouterAPIKey: %v", err)
	}
	if got != "file-key" || gotSrc != SourceFile {
		t.Fatalf("got (%q, %q), want (\"file-key\", file-fallback)", got, gotSrc)
	}
}

func TestClearRemovesBothBackends(t *testing.T) {
	withFakeKeyring(t)
	withIsolatedStateDir(t)
	t.Setenv(envVar, "")

	if _, err := SetOpenRouterAPIKey("to-be-cleared"); err != nil {
		t.Fatalf("SetOpenRouterAPIKey: %v", err)
	}
	if err := ClearOpenRouterAPIKey(); err != nil {
		t.Fatalf("ClearOpenRouterAPIKey: %v", err)
	}

	got, src, err := OpenRouterAPIKey()
	if err != nil {
		t.Fatalf("OpenRouterAPIKey: %v", err)
	}
	if got != "" || src != SourceNone {
		t.Fatalf("got (%q, %q), want (\"\", none) after Clear", got, src)
	}
}

func TestNoneWhenNothingConfigured(t *testing.T) {
	withFakeKeyring(t)
	withIsolatedStateDir(t)
	t.Setenv(envVar, "")

	got, src, err := OpenRouterAPIKey()
	if err != nil {
		t.Fatalf("OpenRouterAPIKey: %v", err)
	}
	if got != "" || src != SourceNone {
		t.Fatalf("got (%q, %q), want (\"\", none)", got, src)
	}
}
