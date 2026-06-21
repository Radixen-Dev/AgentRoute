// SPDX-License-Identifier: GPL-3.0-only

// Package secret stores AgentRoute's sensitive values (currently just the
// user's OPENROUTER_API_KEY).
//
// Storage order of preference:
//  1. The OPENROUTER_API_KEY environment variable, if set — always wins,
//     never persisted, never overwritten by this package.
//  2. The OS keyring (Windows Credential Manager / macOS Keychain /
//     Linux Secret Service), via go-keyring.
//  3. A fallback file under the AgentRoute state directory with 0600
//     permissions, used only when no OS keyring backend is available
//     (e.g. a minimal Linux box with no Secret Service running). Doctor
//     surfaces a warning whenever this fallback is in use.
package secret

import (
	"fmt"
	"os"

	"github.com/zalando/go-keyring"

	"github.com/Radixen-Dev/AgentRoute/internal/fsutil"
	"github.com/Radixen-Dev/AgentRoute/internal/paths"
)

const (
	keyringService = "AgentRoute"
	keyringUser    = "openrouter-api-key"
	envVar         = "OPENROUTER_API_KEY"
)

// keyringBackend abstracts the OS keyring so tests can substitute a fake
// and never touch the real Windows Credential Manager / macOS Keychain /
// Linux Secret Service on the machine running the test.
type keyringBackend interface {
	Get(service, user string) (string, error)
	Set(service, user, value string) error
	Delete(service, user string) error
}

type realKeyring struct{}

func (realKeyring) Get(service, user string) (string, error) { return keyring.Get(service, user) }
func (realKeyring) Set(service, user, value string) error    { return keyring.Set(service, user, value) }
func (realKeyring) Delete(service, user string) error        { return keyring.Delete(service, user) }

// backend is swapped out in tests; production code always uses realKeyring.
var backend keyringBackend = realKeyring{}

// Source identifies where an API key value came from, so callers (e.g.
// `agentroute doctor`) can report it accurately.
type Source string

// The possible Source values OpenRouterAPIKey and SetOpenRouterAPIKey
// report, in precedence order (env wins over keyring wins over file).
const (
	SourceEnv     Source = "env"
	SourceKeyring Source = "keyring"
	SourceFile    Source = "file-fallback"
	SourceNone    Source = "none"
)

// OpenRouterAPIKey resolves the active OpenRouter API key and where it
// came from. An empty key with SourceNone means nothing is configured.
func OpenRouterAPIKey() (key string, source Source, err error) {
	if v := os.Getenv(envVar); v != "" {
		return v, SourceEnv, nil
	}

	v, kerr := backend.Get(keyringService, keyringUser)
	if kerr == nil && v != "" {
		return v, SourceKeyring, nil
	}

	v, ferr := readFallbackFile()
	if ferr == nil && v != "" {
		return v, SourceFile, nil
	}

	return "", SourceNone, nil
}

// SetOpenRouterAPIKey persists key for future sessions. It prefers the OS
// keyring; if that backend is unavailable, it falls back to a 0600 file
// under the AgentRoute state directory.
func SetOpenRouterAPIKey(key string) (Source, error) {
	if err := backend.Set(keyringService, keyringUser, key); err == nil {
		// Clear any stale fallback file so future reads don't prefer it.
		_ = removeFallbackFile()
		return SourceKeyring, nil
	}

	if err := writeFallbackFile(key); err != nil {
		return SourceNone, fmt.Errorf("secret: store key (keyring and file fallback both failed): %w", err)
	}
	return SourceFile, nil
}

// ClearOpenRouterAPIKey removes any persisted key from both the keyring
// and the file fallback. It does not affect the OPENROUTER_API_KEY
// environment variable, which the caller's shell owns.
func ClearOpenRouterAPIKey() error {
	_ = backend.Delete(keyringService, keyringUser) // ignore "not found"
	return removeFallbackFile()
}

func fallbackPath() (string, error) {
	root, err := paths.Root()
	if err != nil {
		return "", err
	}
	return root + string(os.PathSeparator) + "openrouter.key", nil
}

func readFallbackFile() (string, error) {
	path, err := fallbackPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func writeFallbackFile(key string) error {
	path, err := fallbackPath()
	if err != nil {
		return err
	}
	return fsutil.AtomicWrite(path, []byte(key), 0o600)
}

func removeFallbackFile() error {
	path, err := fallbackPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
