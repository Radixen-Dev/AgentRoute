// SPDX-License-Identifier: GPL-3.0-only

// Package cli implements AgentRoute's plain, scriptable command surface
// (cobra commands), per the architecture plan §7.5-7.6. The TUI (Phase 8)
// is a separate, additive frontend over the same internal packages this
// package orchestrates.
package cli

// Exit codes are part of AgentRoute's plain-mode contract: scripts and
// other agents depend on these being stable. Documented in docs/cli.md.
const (
	ExitOK            = 0
	ExitGeneric       = 1
	ExitUsage         = 2
	ExitMissingKey    = 3
	ExitGatewayFailed = 4
	ExitLinkFailed    = 5
)

// exitError pairs an error with the exit code main() should use, without
// forcing every internal package to know about CLI exit codes.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

// ExitCode implements the (unexported, structurally matched) ExitCoder
// interface main.go checks for via errors.As.
func (e *exitError) ExitCode() int { return e.code }

// withExitCode wraps err so main.go's error handler exits with code
// instead of the ExitGeneric default.
func withExitCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: code, err: err}
}
