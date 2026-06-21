// SPDX-License-Identifier: GPL-3.0-only

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/Radixen-Dev/AgentRoute/internal/fsutil"
	"github.com/Radixen-Dev/AgentRoute/internal/gateway"
)

// Sentinel errors ParseManifest and Validate return, so callers (notably
// the registry, which must skip rather than fail hard on an
// as-yet-unsupported manifest) can distinguish "malformed manifest" from
// "well-formed manifest this version of AgentRoute can't wire up yet".
var (
	ErrUnknownWire             = errors.New("manifest: unknown wire protocol")
	ErrUnsupportedConfigTarget = errors.New("manifest: unsupported config_target.type in this version of AgentRoute")
	ErrMissingWiring           = errors.New("manifest: config_target.type has no matching [wiring] block")
	ErrUnknownTemplateVar      = errors.New("manifest: unknown template variable")
)

// manifestWireByName maps a manifest's "wire" string to the gateway.Wire
// the registry must have a Translator running for. Kept separate from
// gateway's own constants since the manifest format is a stable on-disk
// contract independent of gateway's Go types.
var manifestWireByName = map[string]gateway.Wire{
	"anthropic": gateway.WireAnthropic,
	"openai":    gateway.WireOpenAI,
	"gemini":    gateway.WireGemini,
}

// ManifestDetect is a manifest's [detect] table: how to tell whether the
// described tool is installed.
type ManifestDetect struct {
	Binary      string   `toml:"binary"`
	ConfigPaths []string `toml:"config_paths"`
}

// ManifestConfigTarget is a manifest's [config_target] table: which file
// (if any) Link/Unlink edit, and how.
//
// v1 implements two of the three documented types — "toml" (a TOML file,
// edited via dotted-key wiring.toml entries) and "shell-env" (no file;
// Link reports the variables the tool needs without writing anything, see
// ManifestAdapter.Link). "json-env" (Claude Code's settings.json) is
// recognized but deliberately unsupported here: it needs backup/restore
// semantics the in-tree claudecode adapter already provides, and the
// generic interpreter doesn't yet replicate them (see
// manifests/claude-code.toml's comment). ParseManifest/Validate report
// ErrUnsupportedConfigTarget for it so the registry can skip it cleanly
// instead of failing to load every manifest in the directory.
type ManifestConfigTarget struct {
	Type string `toml:"type"`
	Path string `toml:"path"`
}

// ManifestWiring is a manifest's [wiring.*] tables: the literal (template)
// values to write into the target config once rendered against a
// platform.LinkInput. Exactly one of these is populated, matching
// ConfigTarget.Type.
type ManifestWiring struct {
	ShellEnv map[string]string `toml:"shell_env"`
	TOML     map[string]string `toml:"toml"`
}

// Manifest is AgentRoute's declarative platform-adapter format (plan §6.3):
// most tools need no Go code, just a TOML file describing how to detect the
// tool and how to point it at the gateway.
type Manifest struct {
	ID           string               `toml:"id"`
	DisplayName  string               `toml:"display_name"`
	Wire         string               `toml:"wire"`
	Detect       ManifestDetect       `toml:"detect"`
	ConfigTarget ManifestConfigTarget `toml:"config_target"`
	Roles        map[string]string    `toml:"roles"`
	Wiring       ManifestWiring       `toml:"wiring"`
}

// ParseManifest decodes raw TOML into a Manifest and validates it. It does
// not touch the filesystem beyond what the caller already read.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if _, err := toml.Decode(string(data), &m); err != nil {
		return Manifest{}, fmt.Errorf("manifest: parse: %w", err)
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Validate checks that m is well-formed and that its config_target/wiring
// combination is one AgentRoute can actually interpret.
func (m Manifest) Validate() error {
	if m.ID == "" {
		return errors.New("manifest: missing id")
	}
	if m.DisplayName == "" {
		return fmt.Errorf("manifest %s: missing display_name", m.ID)
	}
	if _, ok := manifestWireByName[m.Wire]; !ok {
		return fmt.Errorf("%w: %q (manifest %s)", ErrUnknownWire, m.Wire, m.ID)
	}
	switch m.ConfigTarget.Type {
	case "toml":
		if m.ConfigTarget.Path == "" {
			return fmt.Errorf("manifest %s: config_target.type=toml requires config_target.path", m.ID)
		}
		if len(m.Wiring.TOML) == 0 {
			return fmt.Errorf("%w: manifest %s declares config_target.type=toml but has no [wiring.toml]", ErrMissingWiring, m.ID)
		}
	case "shell-env":
		if len(m.Wiring.ShellEnv) == 0 {
			return fmt.Errorf("%w: manifest %s declares config_target.type=shell-env but has no [wiring.shell_env]", ErrMissingWiring, m.ID)
		}
	case "json-env":
		return fmt.Errorf("%w: manifest %s uses config_target.type=json-env", ErrUnsupportedConfigTarget, m.ID)
	default:
		return fmt.Errorf("manifest %s: unknown config_target.type %q", m.ID, m.ConfigTarget.Type)
	}
	return nil
}

// templateVarPattern matches "{{ name }}" placeholders in wiring values.
// Names are restricted to word characters and dots ("roles.balanced") —
// anything else is a template syntax error, not a silent pass-through.
var templateVarPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.]+)\s*\}\}`)

// renderTemplate substitutes every {{var}} placeholder in s using vars,
// returning ErrUnknownTemplateVar (wrapped with the offending name) on the
// first placeholder vars doesn't define.
func renderTemplate(s string, vars map[string]string) (string, error) {
	var firstErr error
	out := templateVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		if firstErr != nil {
			return match
		}
		name := templateVarPattern.FindStringSubmatch(match)[1]
		v, ok := vars[name]
		if !ok {
			firstErr = fmt.Errorf("%w: %q", ErrUnknownTemplateVar, name)
			return match
		}
		return v
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

// templateVars builds the substitution map for a manifest's {{...}}
// placeholders from a Link call's input. {{roles.<tier>}} prefers the
// caller-supplied alias (in.RoleAliases, the live AgentRoute alias for
// that tier) and only falls back to the manifest's own declared literal
// (e.g. "agentroute-balanced" in [roles]) if the caller didn't supply one
// — which in practice only happens in tests that construct LinkInput
// directly without going through profile.Profile.RoleAliases.
func (m Manifest) templateVars(in LinkInput) map[string]string {
	vars := map[string]string{
		"gateway_url": in.GatewayURL,
		"auth_token":  in.AuthToken,
	}
	for tier, fallback := range m.Roles {
		if alias, ok := in.RoleAliases[tier]; ok {
			vars["roles."+tier] = alias
		} else {
			vars["roles."+tier] = fallback
		}
	}
	return vars
}

// ManifestAdapter implements Platform by interpreting a Manifest. Most
// tools need nothing beyond this — see registry.go for how manifests are
// discovered and turned into these.
type ManifestAdapter struct {
	manifest Manifest

	// configPathOverride lets tests redirect Link/Unlink at a temp file
	// instead of the manifest's real config_target.path, mirroring
	// claudecode.Adapter.SettingsPath.
	configPathOverride string
}

// NewManifestAdapter returns a Platform backed by m, using m's own
// config_target.path. Manifest must have already passed Validate (as
// ParseManifest guarantees) — NewManifestAdapter does not re-validate.
func NewManifestAdapter(m Manifest) *ManifestAdapter {
	return &ManifestAdapter{manifest: m}
}

func (a *ManifestAdapter) configPath() string {
	if a.configPathOverride != "" {
		return a.configPathOverride
	}
	return expandHome(a.manifest.ConfigTarget.Path)
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") && path != "~" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}

// ID implements Platform.
func (a *ManifestAdapter) ID() string { return a.manifest.ID }

// DisplayName implements Platform.
func (a *ManifestAdapter) DisplayName() string { return a.manifest.DisplayName }

// Wire implements Platform.
func (a *ManifestAdapter) Wire() gateway.Wire { return manifestWireByName[a.manifest.Wire] }

// roleDisplayNames gives the three well-known AgentRoute tiers a friendlier
// label; any other tier ID (a manifest is free to declare a subset, or in
// principle a different name) is shown as-is.
var roleDisplayNames = map[string]string{
	"heavy":    "Heavy",
	"balanced": "Balanced",
	"fast":     "Fast",
}

// Roles implements Platform, deriving the role list from the manifest's
// [roles] table (sorted by ID for deterministic output).
func (a *ManifestAdapter) Roles() []Role {
	ids := make([]string, 0, len(a.manifest.Roles))
	for id := range a.manifest.Roles {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	roles := make([]Role, 0, len(ids))
	for _, id := range ids {
		name, ok := roleDisplayNames[id]
		if !ok {
			name = id
		}
		roles = append(roles, Role{ID: id, DisplayName: name})
	}
	return roles
}

// Detect implements Platform: Installed reports whether [detect].binary is
// on PATH (true if no binary is declared); ConfigPath is always returned.
func (a *ManifestAdapter) Detect(_ context.Context) (Detection, error) {
	installed := true
	if a.manifest.Detect.Binary != "" {
		_, err := exec.LookPath(a.manifest.Detect.Binary)
		installed = err == nil
	}
	return Detection{Installed: installed, ConfigPath: a.configPath()}, nil
}

// Link implements Platform.
//
// For config_target.type=="toml", it merges the manifest's [wiring.toml]
// dotted keys (rendered against in) into the target TOML file, taking a
// backup on first link exactly like the claudecode adapter.
//
// For config_target.type=="shell-env", it does NOT write any file: per
// manifests/examples/gemini-cli.toml.example's comment, the UX for
// shell_env wiring (env file vs. direct export vs. wrapper script) is an
// open design question deferred past v1. Link instead returns the rendered
// variables in LinkResult so a caller can show them to the user, with a
// nil error — guessing silently here would be worse than doing nothing.
func (a *ManifestAdapter) Link(_ context.Context, in LinkInput) (LinkResult, error) {
	vars := a.manifest.templateVars(in)

	switch a.manifest.ConfigTarget.Type {
	case "shell-env":
		keys := make([]string, 0, len(a.manifest.Wiring.ShellEnv))
		for k, raw := range a.manifest.Wiring.ShellEnv {
			if _, err := renderTemplate(raw, vars); err != nil {
				return LinkResult{}, fmt.Errorf("manifest %s: render %s: %w", a.manifest.ID, k, err)
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return LinkResult{KeysSet: keys}, nil

	case "toml":
		path := a.configPath()
		rendered := make(map[string]string, len(a.manifest.Wiring.TOML))
		keys := make([]string, 0, len(a.manifest.Wiring.TOML))
		for dottedKey, raw := range a.manifest.Wiring.TOML {
			v, err := renderTemplate(raw, vars)
			if err != nil {
				return LinkResult{}, fmt.Errorf("manifest %s: render %s: %w", a.manifest.ID, dottedKey, err)
			}
			rendered[dottedKey] = v
			keys = append(keys, dottedKey)
		}
		sort.Strings(keys)

		if err := fsutil.BackupIfMissing(path); err != nil {
			return LinkResult{}, fmt.Errorf("manifest %s: backup %s: %w", a.manifest.ID, path, err)
		}
		if err := mergeTOMLFile(path, rendered); err != nil {
			return LinkResult{}, fmt.Errorf("manifest %s: merge %s: %w", a.manifest.ID, path, err)
		}
		if err := saveManifestLinkState(a.manifest.ID, manifestLinkState{ConfigPath: path, KeysSet: keys}); err != nil {
			return LinkResult{}, err
		}
		return LinkResult{ConfigPath: path, KeysSet: keys}, nil

	default:
		// Validate guarantees this is unreachable for any manifest the
		// registry actually loaded.
		return LinkResult{}, fmt.Errorf("manifest %s: unsupported config_target.type %q", a.manifest.ID, a.manifest.ConfigTarget.Type)
	}
}

// Unlink implements Platform. For shell-env wiring (nothing was written by
// Link) it is a no-op. For toml wiring it restores the backup taken on
// Link, falling back to removing exactly the recorded dotted keys if the
// backup is missing — mirroring claudecode.Adapter.Unlink.
func (a *ManifestAdapter) Unlink(_ context.Context) error {
	if a.manifest.ConfigTarget.Type != "toml" {
		return nil
	}
	path := a.configPath()

	st, hadState, err := loadManifestLinkState(a.manifest.ID)
	if err != nil {
		return fmt.Errorf("manifest %s: load link state: %w", a.manifest.ID, err)
	}

	hadRecordedOriginal := fsutil.HasRecordedOriginal(path)
	if err := fsutil.RestoreFromBackup(path); err != nil {
		return fmt.Errorf("manifest %s: restore %s: %w", a.manifest.ID, path, err)
	}
	if !hadRecordedOriginal && hadState {
		if err := removeTOMLKeysFallback(path, st.KeysSet); err != nil {
			return fmt.Errorf("manifest %s: fallback key removal: %w", a.manifest.ID, err)
		}
	}
	return removeManifestLinkState(a.manifest.ID)
}

// Status implements Platform. For toml wiring it reports linked if the
// target file currently contains the dotted keys this adapter would set
// (regardless of value, since the value is profile-specific). For
// shell-env wiring, since Link never writes anything, Status always
// reports not-linked — there is nothing on disk to check.
func (a *ManifestAdapter) Status(_ context.Context) (LinkStatus, error) {
	path := a.configPath()
	if a.manifest.ConfigTarget.Type != "toml" {
		return LinkStatus{ConfigPath: path}, nil
	}

	doc, err := readTOMLFile(path)
	if err != nil {
		return LinkStatus{ConfigPath: path}, nil
	}
	linked := true
	for dottedKey := range a.manifest.Wiring.TOML {
		if _, ok := getTOMLDotted(doc, dottedKey); !ok {
			linked = false
			break
		}
	}
	if len(a.manifest.Wiring.TOML) == 0 {
		linked = false
	}
	return LinkStatus{Linked: linked, ConfigPath: path}, nil
}
