// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"fmt"

	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/platform/claudecode"
)

// newClaudeCodeAdapter is a test seam: tests reassign it to return an
// adapter pointed at a temp settings.json instead of the real
// ~/.claude/settings.json.
var newClaudeCodeAdapter = func() platform.Platform { return claudecode.New() }

// resolvePlatform looks up an in-tree Platform adapter by ID. v1 ships
// only Claude Code; manifest-driven adapters (Codex, Gemini CLI, ...) are
// added in Phase 9 without changing this function's shape.
func resolvePlatform(name string) (platform.Platform, error) {
	switch name {
	case claudecode.ID:
		return newClaudeCodeAdapter(), nil
	default:
		return nil, fmt.Errorf("unknown platform %q (supported: %s)", name, claudecode.ID)
	}
}
