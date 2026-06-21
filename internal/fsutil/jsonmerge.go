// SPDX-License-Identifier: GPL-3.0-only
package fsutil

import (
	"encoding/json"
	"fmt"
)

// MergeEnvBlock decodes a JSON settings document, ensures it has an "env"
// object, sets each key in env into it, and re-encodes the document with
// stable (sorted) key order and 2-space indentation. It is used by
// platform adapters that wire a tool via a JSON settings file's env block
// (e.g. Claude Code's ~/.claude/settings.json).
//
// It returns the new document bytes. The caller is responsible for
// persisting it (typically via AtomicWrite after BackupIfMissing).
func MergeEnvBlock(original []byte, env map[string]string) ([]byte, error) {
	doc := map[string]any{}
	if len(original) > 0 {
		if err := json.Unmarshal(original, &doc); err != nil {
			return nil, fmt.Errorf("fsutil: parse settings json: %w", err)
		}
	}

	envBlock, ok := doc["env"].(map[string]any)
	if !ok {
		envBlock = map[string]any{}
	}
	for k, v := range env {
		envBlock[k] = v
	}
	doc["env"] = envBlock

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("fsutil: encode settings json: %w", err)
	}
	return append(out, '\n'), nil
}

// RemoveEnvKeys decodes a JSON settings document and removes exactly the
// given keys from its "env" object (no-op for keys not present), then
// re-encodes it. Used by Unlink to surgically remove only the keys
// AgentRoute itself added, leaving any keys the user added independently
// untouched.
func RemoveEnvKeys(original []byte, keys []string) ([]byte, error) {
	doc := map[string]any{}
	if len(original) > 0 {
		if err := json.Unmarshal(original, &doc); err != nil {
			return nil, fmt.Errorf("fsutil: parse settings json: %w", err)
		}
	}

	envBlock, ok := doc["env"].(map[string]any)
	if !ok {
		// Nothing to remove.
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("fsutil: encode settings json: %w", err)
		}
		return append(out, '\n'), nil
	}
	for _, k := range keys {
		delete(envBlock, k)
	}
	doc["env"] = envBlock

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("fsutil: encode settings json: %w", err)
	}
	return append(out, '\n'), nil
}
