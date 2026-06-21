// SPDX-License-Identifier: GPL-3.0-only

package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/Radixen-Dev/AgentRoute/internal/fsutil"
	"github.com/Radixen-Dev/AgentRoute/internal/paths"
)

// readTOMLFile decodes path into a generic document, treating a missing
// file as an empty table (the same convention claudecode.Adapter uses for
// settings.json: Link on a tool that has no config file yet creates one).
func readTOMLFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	doc := map[string]any{}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return doc, nil
}

// writeTOMLFile encodes doc back to path atomically, preserving the
// original file's permissions (or 0600 for a newly created one).
func writeTOMLFile(path string, doc map[string]any) error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(doc); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	perm := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	return fsutil.AtomicWrite(path, buf.Bytes(), perm)
}

// setTOMLDotted sets doc's value at a dotted key path (e.g.
// "model_providers.agentroute.base_url"), creating intermediate tables as
// needed. It errors rather than overwriting if an intermediate segment
// already holds a non-table value — silently replacing a user's existing
// scalar with a table would be a worse failure mode than refusing to link.
func setTOMLDotted(doc map[string]any, dottedKey, value string) error {
	parts := strings.Split(dottedKey, ".")
	cur := doc
	for i, part := range parts[:len(parts)-1] {
		next, ok := cur[part]
		if !ok {
			nm := map[string]any{}
			cur[part] = nm
			cur = nm
			continue
		}
		nm, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("%q is not a table in existing config", strings.Join(parts[:i+1], "."))
		}
		cur = nm
	}
	cur[parts[len(parts)-1]] = value
	return nil
}

// getTOMLDotted reads doc's value at a dotted key path.
func getTOMLDotted(doc map[string]any, dottedKey string) (any, bool) {
	parts := strings.Split(dottedKey, ".")
	cur := any(doc)
	for _, part := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[part]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

// deleteTOMLDotted removes doc's value at a dotted key path, if present.
// Missing intermediate tables are simply not present to begin with — not
// an error.
func deleteTOMLDotted(doc map[string]any, dottedKey string) {
	parts := strings.Split(dottedKey, ".")
	cur := doc
	for _, part := range parts[:len(parts)-1] {
		next, ok := cur[part]
		if !ok {
			return
		}
		nm, ok := next.(map[string]any)
		if !ok {
			return
		}
		cur = nm
	}
	delete(cur, parts[len(parts)-1])
}

// mergeTOMLFile reads path (or starts from an empty document), sets every
// dotted key in rendered, and writes the result back atomically.
func mergeTOMLFile(path string, rendered map[string]string) error {
	doc, err := readTOMLFile(path)
	if err != nil {
		return err
	}
	for dottedKey, value := range rendered {
		if err := setTOMLDotted(doc, dottedKey, value); err != nil {
			return err
		}
	}
	return writeTOMLFile(path, doc)
}

// removeTOMLKeysFallback deletes exactly the listed dotted keys from path,
// used by ManifestAdapter.Unlink when the pre-Link backup is missing.
func removeTOMLKeysFallback(path string, keys []string) error {
	doc, err := readTOMLFile(path)
	if err != nil {
		return err
	}
	for _, k := range keys {
		deleteTOMLDotted(doc, k)
	}
	return writeTOMLFile(path, doc)
}

// manifestLinkState is the bookkeeping ManifestAdapter persists about its
// most recent Link call, analogous to claudecode's linkState. Status does
// not consult it (it inspects the live file directly, same rationale as
// claudecode.Adapter.Status); it exists solely so Unlink's
// backup-missing fallback knows which keys to remove.
type manifestLinkState struct {
	ConfigPath string   `json:"configPath"`
	KeysSet    []string `json:"keysSet"`
}

func loadManifestLinkState(platformID string) (manifestLinkState, bool, error) {
	path, err := paths.LinkStateFile(platformID)
	if err != nil {
		return manifestLinkState{}, false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return manifestLinkState{}, false, nil
	}
	if err != nil {
		return manifestLinkState{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	var st manifestLinkState
	if err := json.Unmarshal(data, &st); err != nil {
		return manifestLinkState{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	return st, true, nil
}

func saveManifestLinkState(platformID string, st manifestLinkState) error {
	path, err := paths.LinkStateFile(platformID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("encode link state: %w", err)
	}
	return fsutil.AtomicWrite(path, append(data, '\n'), 0o600)
}

func removeManifestLinkState(platformID string) error {
	path, err := paths.LinkStateFile(platformID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
