// SPDX-License-Identifier: GPL-3.0-only

// Package claudecode is AgentRoute's v1 in-tree Platform adapter for
// Claude Code. It wires Claude Code to the gateway purely via the "env"
// block of ~/.claude/settings.json (ANTHROPIC_BASE_URL,
// ANTHROPIC_AUTH_TOKEN, and the three ANTHROPIC_DEFAULT_*_MODEL
// selectors) — no rewriting of CLAUDE.md or any other file. See the
// architecture plan §6.2 and §1.3 for why this mechanism (and not an
// LLM-driven CLAUDE.md rewrite) is the production-correct one.
package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/Radixen-Dev/AgentRoute/internal/fsutil"
	"github.com/Radixen-Dev/AgentRoute/internal/gateway"
	"github.com/Radixen-Dev/AgentRoute/internal/paths"
	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/profile"
)

// ID is this platform's stable identifier, used as the key for its link
// state file (paths.LinkStateFile) and in CLI commands ("agentroute link
// claude-code").
const ID = "claude-code"

// Adapter implements platform.Platform for Claude Code.
type Adapter struct {
	// SettingsPath overrides the resolved settings.json path. Tests set
	// this to a temp file so adapter tests never touch a real
	// ~/.claude/settings.json. Production callers leave it empty, which
	// resolves to os.UserHomeDir()+"/.claude/settings.json".
	SettingsPath string
}

// New returns a Claude Code adapter using the real, OS-resolved settings
// path.
func New() *Adapter { return &Adapter{} }

func (a *Adapter) settingsPath() (string, error) {
	if a.SettingsPath != "" {
		return a.SettingsPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("claudecode: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// ID implements platform.Platform.
func (a *Adapter) ID() string { return ID }

// DisplayName implements platform.Platform.
func (a *Adapter) DisplayName() string { return "Claude Code" }

// Wire implements platform.Platform.
func (a *Adapter) Wire() gateway.Wire { return gateway.WireAnthropic }

// Roles implements platform.Platform. Claude Code exposes all three
// AgentRoute tiers as its native Opus/Sonnet/Haiku model selectors.
func (a *Adapter) Roles() []platform.Role {
	return []platform.Role{
		{ID: profile.TierHeavy, DisplayName: "Heavy (Opus)"},
		{ID: profile.TierBalanced, DisplayName: "Balanced (Sonnet)"},
		{ID: profile.TierFast, DisplayName: "Fast (Haiku)"},
	}
}

// Detect implements platform.Platform. Installed reports whether the
// `claude` binary is on PATH; ConfigPath is always returned so callers can
// show it even before any link has happened.
func (a *Adapter) Detect(_ context.Context) (platform.Detection, error) {
	path, err := a.settingsPath()
	if err != nil {
		return platform.Detection{}, err
	}

	_, lookErr := exec.LookPath("claude")
	return platform.Detection{
		Installed:  lookErr == nil,
		ConfigPath: path,
	}, nil
}

// envKeyForTier maps a profile tier to the Claude Code settings.json env
// key that selects the model Claude Code requests for that tier.
var envKeyForTier = map[string]string{
	profile.TierHeavy:    "ANTHROPIC_DEFAULT_OPUS_MODEL",
	profile.TierBalanced: "ANTHROPIC_DEFAULT_SONNET_MODEL",
	profile.TierFast:     "ANTHROPIC_DEFAULT_HAIKU_MODEL",
}

// Link implements platform.Platform. It is idempotent: re-running Link
// (e.g. to switch profiles) re-merges the env block without disturbing the
// original pre-Link backup taken on the first call.
func (a *Adapter) Link(_ context.Context, in platform.LinkInput) (platform.LinkResult, error) {
	path, err := a.settingsPath()
	if err != nil {
		return platform.LinkResult{}, err
	}

	if err := fsutil.BackupIfMissing(path); err != nil {
		return platform.LinkResult{}, fmt.Errorf("claudecode: backup %s: %w", path, err)
	}

	env := map[string]string{
		"ANTHROPIC_BASE_URL":   in.GatewayURL,
		"ANTHROPIC_AUTH_TOKEN": in.AuthToken,
	}
	for tier, key := range envKeyForTier {
		if alias, ok := in.RoleAliases[tier]; ok {
			env[key] = alias
		}
	}

	original, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return platform.LinkResult{}, fmt.Errorf("claudecode: read %s: %w", path, err)
	}

	merged, err := fsutil.MergeEnvBlock(original, env)
	if err != nil {
		return platform.LinkResult{}, fmt.Errorf("claudecode: merge env block: %w", err)
	}

	perm := os.FileMode(0o600)
	if info, statErr := os.Stat(path); statErr == nil {
		perm = info.Mode().Perm()
	}
	if err := fsutil.AtomicWrite(path, merged, perm); err != nil {
		return platform.LinkResult{}, fmt.Errorf("claudecode: write %s: %w", path, err)
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if err := saveLinkState(linkState{ConfigPath: path, KeysSet: keys, GatewayURL: in.GatewayURL}); err != nil {
		return platform.LinkResult{}, err
	}

	return platform.LinkResult{ConfigPath: path, KeysSet: keys}, nil
}

// Unlink implements platform.Platform. The canonical path is a full,
// byte-identical restore from the backup BackupIfMissing took on Link (or
// deleting the file if Link created it from nothing). If that backup is
// missing — e.g. deleted out-of-band between Link and Unlink — Unlink
// falls back to surgically removing only the keys it recorded setting,
// rather than silently doing nothing and leaving the tool linked.
func (a *Adapter) Unlink(_ context.Context) error {
	path, err := a.settingsPath()
	if err != nil {
		return err
	}

	st, hadState, err := loadLinkState()
	if err != nil {
		return fmt.Errorf("claudecode: load link state: %w", err)
	}

	hadRecordedOriginal := fsutil.HasRecordedOriginal(path)
	if err := fsutil.RestoreFromBackup(path); err != nil {
		return fmt.Errorf("claudecode: restore %s: %w", path, err)
	}

	if !hadRecordedOriginal && hadState {
		if err := removeKeysFallback(path, st.KeysSet); err != nil {
			return fmt.Errorf("claudecode: fallback key removal: %w", err)
		}
	}

	return removeLinkState()
}

func removeKeysFallback(path string, keys []string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	out, err := fsutil.RemoveEnvKeys(data, keys)
	if err != nil {
		return fmt.Errorf("remove keys: %w", err)
	}

	perm := os.FileMode(0o600)
	if info, statErr := os.Stat(path); statErr == nil {
		perm = info.Mode().Perm()
	}
	return fsutil.AtomicWrite(path, out, perm)
}

// Status implements platform.Platform. It trusts the live settings.json
// over AgentRoute's own bookkeeping: if the user (or Claude Code itself)
// removed ANTHROPIC_BASE_URL independently, Status correctly reports
// "not linked" even though a stale link-state file still exists.
func (a *Adapter) Status(_ context.Context) (platform.LinkStatus, error) {
	path, err := a.settingsPath()
	if err != nil {
		return platform.LinkStatus{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return platform.LinkStatus{ConfigPath: path}, nil
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return platform.LinkStatus{ConfigPath: path}, nil
	}
	env, _ := doc["env"].(map[string]any)
	baseURL, _ := env["ANTHROPIC_BASE_URL"].(string)

	return platform.LinkStatus{
		Linked:     baseURL != "",
		GatewayURL: baseURL,
		ConfigPath: path,
	}, nil
}

// linkState is what AgentRoute persists about its own most recent Link
// call, under paths.LinkStateFile(ID). It is bookkeeping only — Status
// does not trust it over the live file (see Status above) — and exists so
// Unlink's fallback path knows exactly which keys it once set.
type linkState struct {
	ConfigPath string   `json:"configPath"`
	KeysSet    []string `json:"keysSet"`
	GatewayURL string   `json:"gatewayUrl"`
}

func loadLinkState() (linkState, bool, error) {
	path, err := paths.LinkStateFile(ID)
	if err != nil {
		return linkState{}, false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return linkState{}, false, nil
	}
	if err != nil {
		return linkState{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	var st linkState
	if err := json.Unmarshal(data, &st); err != nil {
		return linkState{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	return st, true, nil
}

func saveLinkState(st linkState) error {
	path, err := paths.LinkStateFile(ID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("encode link state: %w", err)
	}
	return fsutil.AtomicWrite(path, append(data, '\n'), 0o600)
}

func removeLinkState() error {
	path, err := paths.LinkStateFile(ID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
