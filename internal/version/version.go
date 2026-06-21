// SPDX-License-Identifier: GPL-3.0-only

// Package version holds build-time metadata injected via -ldflags by goreleaser.
package version

// These are overwritten at build time via:
//
//	-X github.com/Radixen-Dev/AgentRoute/internal/version.Version=...
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a human-readable "version (commit, date)" summary.
func String() string {
	return Version + " (" + Commit + ", " + Date + ")"
}
