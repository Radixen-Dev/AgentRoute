// SPDX-License-Identifier: GPL-3.0-only
package fsutil

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMergeEnvBlockCreatesEnv(t *testing.T) {
	out, err := MergeEnvBlock([]byte(`{}`), map[string]string{
		"ANTHROPIC_BASE_URL": "http://127.0.0.1:4505",
	})
	if err != nil {
		t.Fatalf("MergeEnvBlock: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	env, ok := doc["env"].(map[string]any)
	if !ok {
		t.Fatalf("expected env object, got %T", doc["env"])
	}
	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:4505" {
		t.Fatalf("got %v", env["ANTHROPIC_BASE_URL"])
	}

	// Settings files must use plain LF line endings on every OS, including
	// Windows — Claude Code and any editor opening the file expect that,
	// and nothing in this package should be doing OS-specific newline
	// translation.
	if strings.Contains(string(out), "\r\n") {
		t.Fatalf("output contains CRLF line endings, want LF only: %q", out)
	}
}

func TestMergeEnvBlockPreservesUnrelatedKeys(t *testing.T) {
	original := []byte(`{"someOtherSetting":true,"env":{"USER_KEY":"keep-me"}}`)
	out, err := MergeEnvBlock(original, map[string]string{
		"ANTHROPIC_BASE_URL": "http://127.0.0.1:4505",
	})
	if err != nil {
		t.Fatalf("MergeEnvBlock: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if doc["someOtherSetting"] != true {
		t.Fatalf("unrelated top-level key was lost")
	}
	env := doc["env"].(map[string]any)
	if env["USER_KEY"] != "keep-me" {
		t.Fatalf("unrelated env key was lost")
	}
	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:4505" {
		t.Fatalf("new key missing")
	}
}

func TestRemoveEnvKeysRemovesOnlyOwnKeys(t *testing.T) {
	original := []byte(`{"env":{"USER_KEY":"keep-me","ANTHROPIC_BASE_URL":"http://127.0.0.1:4505"}}`)
	out, err := RemoveEnvKeys(original, []string{"ANTHROPIC_BASE_URL"})
	if err != nil {
		t.Fatalf("RemoveEnvKeys: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	env := doc["env"].(map[string]any)
	if _, present := env["ANTHROPIC_BASE_URL"]; present {
		t.Fatalf("expected ANTHROPIC_BASE_URL to be removed")
	}
	if env["USER_KEY"] != "keep-me" {
		t.Fatalf("expected unrelated key to survive removal")
	}
}

// TestMergeEnvBlockHandlesAbsentFile covers the first-time-user case: the
// settings file does not exist yet, so the caller passes nil/empty bytes
// (there is nothing to read). MergeEnvBlock must build a fresh document
// rather than failing.
func TestMergeEnvBlockHandlesAbsentFile(t *testing.T) {
	out, err := MergeEnvBlock(nil, map[string]string{"ANTHROPIC_BASE_URL": "http://127.0.0.1:4505"})
	if err != nil {
		t.Fatalf("MergeEnvBlock(nil, ...): %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	env, ok := doc["env"].(map[string]any)
	if !ok || env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:4505" {
		t.Fatalf("got doc %v, want a fresh document with the env key set", doc)
	}
}

// TestMergeEnvBlockRejectsInvalidJSON guards against silently corrupting a
// settings file Claude Code (or the user) left in a broken state: Link must
// fail loudly rather than overwrite something it cannot safely parse.
func TestMergeEnvBlockRejectsInvalidJSON(t *testing.T) {
	if _, err := MergeEnvBlock([]byte(`{not valid json`), map[string]string{"K": "v"}); err == nil {
		t.Fatalf("expected an error for invalid JSON input, got nil")
	}
}

func TestRemoveEnvKeysHandlesAbsentFile(t *testing.T) {
	out, err := RemoveEnvKeys(nil, []string{"ANTHROPIC_BASE_URL"})
	if err != nil {
		t.Fatalf("RemoveEnvKeys(nil, ...): %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
}

func TestRemoveEnvKeysRejectsInvalidJSON(t *testing.T) {
	if _, err := RemoveEnvKeys([]byte(`{not valid json`), []string{"K"}); err == nil {
		t.Fatalf("expected an error for invalid JSON input, got nil")
	}
}

func TestRemoveEnvKeysNoEnvBlock(t *testing.T) {
	out, err := RemoveEnvKeys([]byte(`{"foo":1}`), []string{"ANTHROPIC_BASE_URL"})
	if err != nil {
		t.Fatalf("RemoveEnvKeys: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc["foo"] != float64(1) {
		t.Fatalf("expected foo preserved")
	}
}
