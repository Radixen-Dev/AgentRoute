// SPDX-License-Identifier: GPL-3.0-only

package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Logf is the shape of a logging callback the registry uses to report
// manifests it skipped, mirroring orchestrator.Logf so callers can wire
// both into the same printer/TUI toast sink.
type Logf func(format string, args ...any)

// LoadManifestAdapters reads every "*.toml" file directly inside dir
// (manifests/ in production) and returns the ones whose config_target type
// AgentRoute currently knows how to wire up, as Platform adapters.
//
// Files under a nested "examples/" directory, or any file ending in
// ".example", are reference material rather than adapters to load — the
// manifest authoring docs (plan §6.3) point at them as schema examples for
// tools not enabled in v1, and manifests/examples/*.toml.example exist
// purely to be read by a person, never by this loader. A manifest with a
// currently-unsupported config_target (json-env, in v1) is skipped with a
// log line rather than failing the whole load, since Claude Code's own
// manifests/claude-code.toml is exactly such a file: it documents the
// schema but is deliberately served by the in-tree claudecode adapter
// instead (see manifest.go's ManifestConfigTarget doc).
//
// A manifest that fails to parse, or is well-formed but invalid in any
// other way, is reported as an error rather than silently skipped — those
// indicate either a typo in a manifest meant to be loaded, or a bug, and
// both deserve to be loud.
func LoadManifestAdapters(dir string, logf Logf) ([]Platform, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("platform: read manifest dir %s: %w", dir, err)
	}

	var adapters []Platform
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("platform: read manifest %s: %w", path, err)
		}

		m, err := ParseManifest(data)
		if errors.Is(err, ErrUnsupportedConfigTarget) {
			logf("platform: skipping manifest %s (%v)", path, err)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("platform: %s: %w", path, err)
		}
		adapters = append(adapters, NewManifestAdapter(m))
	}
	return adapters, nil
}

// Registry holds every Platform AgentRoute can wire a profile to: the
// in-tree adapters plus whatever manifests LoadManifestAdapters found.
type Registry struct {
	platforms map[string]Platform
	order     []string
}

// NewRegistry builds a Registry from inTree (e.g. claudecode.New()) plus
// any manifest adapters loaded from manifestsDir. inTree adapters take
// precedence on ID collision — a manifest can never shadow an in-tree
// adapter, since the in-tree one is the one AgentRoute actually ships and
// tests.
func NewRegistry(manifestsDir string, logf Logf, inTree ...Platform) (*Registry, error) {
	manifestAdapters, err := LoadManifestAdapters(manifestsDir, logf)
	if err != nil {
		return nil, err
	}

	r := &Registry{platforms: make(map[string]Platform)}
	for _, p := range manifestAdapters {
		r.add(p)
	}
	for _, p := range inTree {
		r.add(p)
	}
	return r, nil
}

func (r *Registry) add(p Platform) {
	if _, exists := r.platforms[p.ID()]; !exists {
		r.order = append(r.order, p.ID())
	}
	r.platforms[p.ID()] = p
}

// Get returns the platform registered under id, if any.
func (r *Registry) Get(id string) (Platform, bool) {
	p, ok := r.platforms[id]
	return p, ok
}

// All returns every registered platform, in registration order
// (deterministic: manifests in directory-listing order, then in-tree
// adapters in the order passed to NewRegistry).
func (r *Registry) All() []Platform {
	platforms := make([]Platform, 0, len(r.order))
	for _, id := range r.order {
		platforms = append(platforms, r.platforms[id])
	}
	return platforms
}
