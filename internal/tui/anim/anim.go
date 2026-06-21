// SPDX-License-Identifier: GPL-3.0-only

// Package anim provides Harmonica-backed spring animation helpers for the
// TUI, plus the single reduced-motion check every animation must consult
// (see plan §7.4: animations are disabled under --plain, non-TTY,
// NO_COLOR, or AGENTROUTE_REDUCE_MOTION=1).
package anim

import (
	"os"

	"github.com/charmbracelet/harmonica"
	"github.com/mattn/go-isatty"
)

// Reduced reports whether animations must be skipped. Every screen that
// animates checks this before starting a spring or scheduling a tick.
func Reduced() bool {
	if os.Getenv("AGENTROUTE_REDUCE_MOTION") == "1" {
		return true
	}
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return true
	}
	return false
}

// Spring wraps a harmonica.Spring with the position/velocity state it
// advances, so callers don't have to thread three float64s through Update.
type Spring struct {
	spring harmonica.Spring
	pos    float64
	vel    float64
	target float64
}

// NewSpring builds a critically-damped-by-default spring starting at 0,
// targeting target. fps should match the TUI's tick rate (60 is standard).
func NewSpring(fps int, target float64) *Spring {
	return &Spring{
		spring: harmonica.NewSpring(harmonica.FPS(fps), 6.0, 0.85),
		target: target,
	}
}

// Step advances the spring by one tick and returns the new position.
func (s *Spring) Step() float64 {
	s.pos, s.vel = s.spring.Update(s.pos, s.vel, s.target)
	return s.pos
}

// Pos returns the current position without advancing.
func (s *Spring) Pos() float64 { return s.pos }

// Settled reports whether the spring is close enough to its target and
// nearly stationary that further animation would be imperceptible.
func (s *Spring) Settled() bool {
	const eps = 0.001
	diff := s.target - s.pos
	return diff < eps && diff > -eps && s.vel < eps && s.vel > -eps
}

// SetTarget retargets the spring (e.g. user changed selection mid-flight).
func (s *Spring) SetTarget(target float64) { s.target = target }
