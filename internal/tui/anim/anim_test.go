// SPDX-License-Identifier: GPL-3.0-only

package anim

import "testing"

func TestReducedRespectsReduceMotionEnvVar(t *testing.T) {
	t.Setenv("AGENTROUTE_REDUCE_MOTION", "1")
	if !Reduced() {
		t.Fatalf("Reduced() = false, want true when AGENTROUTE_REDUCE_MOTION=1")
	}
}

func TestReducedRespectsNoColorEnvVar(t *testing.T) {
	t.Setenv("AGENTROUTE_REDUCE_MOTION", "")
	t.Setenv("NO_COLOR", "1")
	if !Reduced() {
		t.Fatalf("Reduced() = false, want true when NO_COLOR is set")
	}
}

func TestSpringSettlesAtTarget(t *testing.T) {
	s := NewSpring(60, 1.0)
	for i := 0; i < 1000 && !s.Settled(); i++ {
		s.Step()
	}
	if !s.Settled() {
		t.Fatalf("spring did not settle within 1000 steps")
	}
	if pos := s.Pos(); pos < 0.95 || pos > 1.05 {
		t.Fatalf("settled position = %f, want close to 1.0", pos)
	}
}

func TestSpringSetTargetRetargets(t *testing.T) {
	s := NewSpring(60, 1.0)
	for i := 0; i < 500; i++ {
		s.Step()
	}
	s.SetTarget(5.0)
	for i := 0; i < 1000 && !s.Settled(); i++ {
		s.Step()
	}
	if pos := s.Pos(); pos < 4.95 || pos > 5.05 {
		t.Fatalf("settled position after retarget = %f, want close to 5.0", pos)
	}
}
