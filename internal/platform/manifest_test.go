// SPDX-License-Identifier: GPL-3.0-only

package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// withIsolatedState redirects AgentRoute's own state dir (where manifest
// link-state bookkeeping lives) into a temp dir, exactly like the pattern
// used in internal/platform/claudecode and internal/profile tests.
func withIsolatedState(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
}

// repoManifest reads a file relative to this package's source tree, so
// tests exercise the actual shipped manifests rather than copies.
func repoManifest(t *testing.T, relPath string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	return data
}

func TestParseManifestCodexExampleParsesAsOpenAIWireWithTOMLWiring(t *testing.T) {
	m, err := ParseManifest(repoManifest(t, "manifests/examples/codex.toml.example"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.ID != "codex" {
		t.Errorf("ID = %q, want codex", m.ID)
	}
	if m.Wire != "openai" {
		t.Errorf("Wire = %q, want openai", m.Wire)
	}
	if m.ConfigTarget.Type != "toml" {
		t.Errorf("ConfigTarget.Type = %q, want toml", m.ConfigTarget.Type)
	}
	if got := m.Wiring.TOML["model_provider"]; got != "agentroute" {
		t.Errorf(`Wiring.TOML["model_provider"] = %q, want "agentroute"`, got)
	}
	if got := m.Wiring.TOML["model"]; got != "{{roles.balanced}}" {
		t.Errorf(`Wiring.TOML["model"] = %q, want "{{roles.balanced}}"`, got)
	}
}

func TestParseManifestGeminiExampleParsesAsGeminiWireWithShellEnvWiring(t *testing.T) {
	m, err := ParseManifest(repoManifest(t, "manifests/examples/gemini-cli.toml.example"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.ID != "gemini-cli" {
		t.Errorf("ID = %q, want gemini-cli", m.ID)
	}
	if m.Wire != "gemini" {
		t.Errorf("Wire = %q, want gemini", m.Wire)
	}
	if m.ConfigTarget.Type != "shell-env" {
		t.Errorf("ConfigTarget.Type = %q, want shell-env", m.ConfigTarget.Type)
	}
	if got := m.Wiring.ShellEnv["GEMINI_MODEL"]; got != "{{roles.balanced}}" {
		t.Errorf(`Wiring.ShellEnv["GEMINI_MODEL"] = %q, want "{{roles.balanced}}"`, got)
	}
}

func TestParseManifestClaudeCodeReferenceManifestIsUnsupportedJSONEnv(t *testing.T) {
	_, err := ParseManifest(repoManifest(t, "manifests/claude-code.toml"))
	if !errors.Is(err, ErrUnsupportedConfigTarget) {
		t.Fatalf("err = %v, want ErrUnsupportedConfigTarget", err)
	}
}

func TestParseManifestRejectsUnknownWire(t *testing.T) {
	_, err := ParseManifest([]byte(`
id = "x"
display_name = "X"
wire = "carrier-pigeon"
[config_target]
type = "shell-env"
[wiring.shell_env]
FOO = "bar"
`))
	if !errors.Is(err, ErrUnknownWire) {
		t.Fatalf("err = %v, want ErrUnknownWire", err)
	}
}

func TestParseManifestRejectsMissingWiringBlock(t *testing.T) {
	_, err := ParseManifest([]byte(`
id = "x"
display_name = "X"
wire = "openai"
[config_target]
type = "toml"
path = "~/.x/config.toml"
`))
	if !errors.Is(err, ErrMissingWiring) {
		t.Fatalf("err = %v, want ErrMissingWiring", err)
	}
}

func TestParseManifestRejectsMissingID(t *testing.T) {
	_, err := ParseManifest([]byte(`
display_name = "X"
wire = "openai"
[config_target]
type = "shell-env"
[wiring.shell_env]
FOO = "bar"
`))
	if err == nil {
		t.Fatal("expected an error for a manifest with no id")
	}
}

func TestManifestAdapterLinkRendersTOMLWiringIntoTargetFile(t *testing.T) {
	withIsolatedState(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	m, err := ParseManifest(repoManifest(t, "manifests/examples/codex.toml.example"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	a := &ManifestAdapter{manifest: m, configPathOverride: path}

	res, err := a.Link(context.Background(), LinkInput{
		GatewayURL:  "http://127.0.0.1:4505",
		AuthToken:   "session-tok",
		RoleAliases: map[string]string{"balanced": "agentroute-balanced"},
	})
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if res.ConfigPath != path {
		t.Fatalf("ConfigPath = %q, want %q", res.ConfigPath, path)
	}

	doc, err := readTOMLFile(path)
	if err != nil {
		t.Fatalf("readTOMLFile: %v", err)
	}
	got, ok := getTOMLDotted(doc, "model_providers.agentroute.base_url")
	if !ok || got != "http://127.0.0.1:4505/v1" {
		t.Fatalf("model_providers.agentroute.base_url = %v, ok=%v, want http://127.0.0.1:4505/v1", got, ok)
	}
	if got, _ := getTOMLDotted(doc, "model"); got != "agentroute-balanced" {
		t.Fatalf("model = %v, want agentroute-balanced", got)
	}

	status, err := a.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.Linked {
		t.Fatal("expected Status.Linked = true after Link")
	}
}

func TestManifestAdapterUnlinkRestoresOriginalFile(t *testing.T) {
	withIsolatedState(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	original := "model_provider = \"openai\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	m, err := ParseManifest(repoManifest(t, "manifests/examples/codex.toml.example"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	a := &ManifestAdapter{manifest: m, configPathOverride: path}

	if _, err := a.Link(context.Background(), LinkInput{
		GatewayURL:  "http://127.0.0.1:4505",
		AuthToken:   "session-tok",
		RoleAliases: map[string]string{"balanced": "agentroute-balanced"},
	}); err != nil {
		t.Fatalf("Link: %v", err)
	}

	if err := a.Unlink(context.Background()); err != nil {
		t.Fatalf("Unlink: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read restored config: %v", err)
	}
	if string(got) != original {
		t.Fatalf("restored config = %q, want %q", got, original)
	}
}

func TestManifestAdapterLinkOnShellEnvWiringWritesNoFile(t *testing.T) {
	withIsolatedState(t)
	m, err := ParseManifest(repoManifest(t, "manifests/examples/gemini-cli.toml.example"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	a := NewManifestAdapter(m)

	res, err := a.Link(context.Background(), LinkInput{
		GatewayURL:  "http://127.0.0.1:4505",
		AuthToken:   "session-tok",
		RoleAliases: map[string]string{"balanced": "agentroute-balanced"},
	})
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if res.ConfigPath != "" {
		t.Fatalf("ConfigPath = %q, want empty (shell-env writes nothing)", res.ConfigPath)
	}
	if len(res.KeysSet) != 3 {
		t.Fatalf("KeysSet = %v, want 3 entries", res.KeysSet)
	}

	if err := a.Unlink(context.Background()); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
}

func TestManifestAdapterLinkRejectsUnknownTemplateVar(t *testing.T) {
	withIsolatedState(t)
	m, err := ParseManifest([]byte(`
id = "x"
display_name = "X"
wire = "openai"
[config_target]
type = "shell-env"
[wiring.shell_env]
FOO = "{{not_a_real_var}}"
`))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	a := NewManifestAdapter(m)

	_, err = a.Link(context.Background(), LinkInput{GatewayURL: "http://x", AuthToken: "tok"})
	if !errors.Is(err, ErrUnknownTemplateVar) {
		t.Fatalf("err = %v, want ErrUnknownTemplateVar", err)
	}
}

func TestManifestAdapterRolesAndDetect(t *testing.T) {
	m, err := ParseManifest(repoManifest(t, "manifests/examples/codex.toml.example"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	a := NewManifestAdapter(m)

	if a.ID() != "codex" {
		t.Errorf("ID() = %q, want codex", a.ID())
	}
	roles := a.Roles()
	if len(roles) != 1 || roles[0].ID != "balanced" {
		t.Fatalf("Roles() = %+v, want one role with ID balanced", roles)
	}

	det, err := a.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	// codex is very unlikely to be on PATH in CI/dev test environments;
	// this just exercises the lookup path without asserting either value.
	_ = det.Installed
}
