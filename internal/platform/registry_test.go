// SPDX-License-Identifier: GPL-3.0-only

package platform

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Radixen-Dev/AgentRoute/internal/gateway"
)

// stubPlatform is a minimal in-tree-style Platform for registry tests.
type stubPlatform struct{ id string }

func (s stubPlatform) ID() string                                { return s.id }
func (s stubPlatform) DisplayName() string                       { return s.id }
func (s stubPlatform) Wire() gateway.Wire                        { return gateway.WireAnthropic }
func (s stubPlatform) Roles() []Role                             { return nil }
func (s stubPlatform) Detect(context.Context) (Detection, error) { return Detection{}, nil }
func (s stubPlatform) Link(context.Context, LinkInput) (LinkResult, error) {
	return LinkResult{}, nil
}
func (s stubPlatform) Unlink(context.Context) error               { return nil }
func (s stubPlatform) Status(context.Context) (LinkStatus, error) { return LinkStatus{}, nil }

func writeManifestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

const validShellEnvManifest = `
id = "stub-tool"
display_name = "Stub Tool"
wire = "gemini"
[config_target]
type = "shell-env"
[wiring.shell_env]
STUB_URL = "{{gateway_url}}"
`

func TestLoadManifestAdaptersLoadsValidTOMLAndSkipsExamplesDir(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, "stub-tool.toml", validShellEnvManifest)

	examplesDir := filepath.Join(dir, "examples")
	if err := os.Mkdir(examplesDir, 0o755); err != nil {
		t.Fatalf("mkdir examples: %v", err)
	}
	writeManifestFile(t, examplesDir, "other.toml.example", validShellEnvManifest)
	// Also drop a literal ".toml.example" at the top level to prove the
	// suffix filter (not just the directory skip) excludes it.
	writeManifestFile(t, dir, "also-example.toml.example", validShellEnvManifest)

	adapters, err := LoadManifestAdapters(dir, nil)
	if err != nil {
		t.Fatalf("LoadManifestAdapters: %v", err)
	}
	if len(adapters) != 1 {
		t.Fatalf("len(adapters) = %d, want 1 (got %v)", len(adapters), adapters)
	}
	if adapters[0].ID() != "stub-tool" {
		t.Fatalf("adapters[0].ID() = %q, want stub-tool", adapters[0].ID())
	}
}

func TestLoadManifestAdaptersSkipsUnsupportedConfigTargetWithoutFailing(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, "stub-tool.toml", validShellEnvManifest)
	writeManifestFile(t, dir, "claude-code.toml", `
id = "claude-code"
display_name = "Claude Code"
wire = "anthropic"
[config_target]
type = "json-env"
path = "~/.claude/settings.json"
`)

	var logged []string
	adapters, err := LoadManifestAdapters(dir, func(format string, _ ...any) {
		logged = append(logged, format)
	})
	if err != nil {
		t.Fatalf("LoadManifestAdapters: %v", err)
	}
	if len(adapters) != 1 {
		t.Fatalf("len(adapters) = %d, want 1 (json-env manifest should be skipped, not loaded)", len(adapters))
	}
	if len(logged) != 1 {
		t.Fatalf("expected exactly one skip log line, got %d: %v", len(logged), logged)
	}
}

func TestLoadManifestAdaptersFailsLoudlyOnMalformedManifest(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, "broken.toml", `this is not valid toml = = =`)

	if _, err := LoadManifestAdapters(dir, nil); err == nil {
		t.Fatal("expected an error for a malformed manifest, got nil")
	}
}

func TestLoadManifestAdaptersOnMissingDirReturnsEmptyNotError(t *testing.T) {
	adapters, err := LoadManifestAdapters(filepath.Join(t.TempDir(), "does-not-exist"), nil)
	if err != nil {
		t.Fatalf("LoadManifestAdapters: %v", err)
	}
	if len(adapters) != 0 {
		t.Fatalf("adapters = %v, want empty", adapters)
	}
}

func TestNewRegistryAgainstRealManifestsDirReturnsOnlyInTreeClaudeCode(t *testing.T) {
	// Exercises the actual shipped manifests/ directory: claude-code.toml
	// (json-env, skipped) plus manifests/examples/* (excluded entirely),
	// so the net effect in v1 is exactly the in-tree adapter passed in —
	// this is the behavior the architecture plan's Phase 9 description
	// commits to ("not shipped/enabled" for Codex/Gemini).
	reg, err := NewRegistry(filepath.Join("..", "..", "manifests"), nil, stubPlatform{id: "claude-code"})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	all := reg.All()
	if len(all) != 1 || all[0].ID() != "claude-code" {
		t.Fatalf("All() = %v, want exactly the in-tree claude-code adapter", all)
	}
}

func TestRegistryInTreeAdapterTakesPrecedenceOverManifestWithSameID(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, "stub-tool.toml", validShellEnvManifest)

	reg, err := NewRegistry(dir, nil, stubPlatform{id: "stub-tool"})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	got, ok := reg.Get("stub-tool")
	if !ok {
		t.Fatal("expected stub-tool to be registered")
	}
	if _, isStub := got.(stubPlatform); !isStub {
		t.Fatalf("got %T, want the in-tree stubPlatform to win over the manifest adapter", got)
	}
	if len(reg.All()) != 1 {
		t.Fatalf("All() = %v, want exactly one entry (no duplicate for the shadowed manifest)", reg.All())
	}
}
