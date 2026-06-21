// SPDX-License-Identifier: GPL-3.0-only

// Package platform defines the extension boundary AgentRoute uses to wire
// up coding agent tools (Claude Code in v1; Codex, Gemini CLI, and others
// later via manifest-driven or in-tree adapters). See the architecture
// plan §6.
package platform

import (
	"context"

	"github.com/Radixen-Dev/AgentRoute/internal/gateway"
)

// Role is one generic tier ("heavy"/"balanced"/"fast", see
// internal/profile) that a platform exposes a native concept for. A
// platform may use a subset of AgentRoute's tiers.
type Role struct {
	ID          string
	DisplayName string
}

// Detection reports what Detect found about whether a tool is installed
// and how it's configured.
type Detection struct {
	Installed  bool
	ConfigPath string
	Version    string
}

// LinkInput carries everything Link needs to point a tool at AgentRoute's
// gateway.
type LinkInput struct {
	// GatewayURL is the local gateway's base URL, e.g. "http://127.0.0.1:4505".
	GatewayURL string
	// AuthToken is the bearer credential the tool must send; the gateway
	// validates it against the same value.
	AuthToken string
	// RoleAliases maps tier ID (profile.TierHeavy etc.) to the AgentRoute
	// alias (profile.Alias(tier)) the tool should be configured to request
	// for that role.
	RoleAliases map[string]string
}

// LinkResult reports what Link actually changed.
type LinkResult struct {
	ConfigPath string
	KeysSet    []string
}

// LinkStatus reports a platform's current wiring state.
type LinkStatus struct {
	Linked     bool
	GatewayURL string
	ConfigPath string
}

// Platform adapts one coding agent tool to AgentRoute's gateway. Link and
// Unlink must be exact inverses: Unlink after Link must restore the tool's
// config to byte-identical its pre-Link state.
type Platform interface {
	ID() string
	DisplayName() string
	// Wire identifies which gateway Translator must be running for this
	// platform's requests to be served.
	Wire() gateway.Wire
	Roles() []Role
	Detect(ctx context.Context) (Detection, error)
	Link(ctx context.Context, in LinkInput) (LinkResult, error)
	Unlink(ctx context.Context) error
	Status(ctx context.Context) (LinkStatus, error)
}
