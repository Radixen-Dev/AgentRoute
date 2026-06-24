// SPDX-License-Identifier: GPL-3.0-only

package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// withTrueColor forces lipgloss to emit truecolor SGR codes for the
// duration of the test, regardless of how `go test` itself is invoked
// (which is normally not a tty, so lipgloss would otherwise strip all
// color and the bug Opaque fixes would never reproduce).
func withTrueColor(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

func TestOpaqueKeepsBackgroundAcrossInnerResets(t *testing.T) {
	withTrueColor(t)

	ok := lipgloss.NewStyle().Foreground(OK)
	muted := lipgloss.NewStyle().Foreground(Muted)
	// Same composition pattern every card body uses: independently
	// rendered fragments concatenated with plain strings, then wrapped by
	// an outer Background()-styled container.
	body := ok.Render("UP") + " " + muted.Render("2m ago")
	card := lipgloss.NewStyle().Background(SurfaceAlt).Render(body)

	bgPrefix := strings.TrimSuffix(lipgloss.NewStyle().Background(SurfaceAlt).Render(""), ansiReset)
	if bgPrefix == "" {
		t.Fatal("expected a non-empty background SGR prefix under TrueColor")
	}

	// Reproduce the bug first: the unstyled space between "UP" and "2m
	// ago" must NOT carry the card's background — proving the seam this
	// type exists to fix is real.
	if strings.Contains(card, bgPrefix+" ") {
		t.Fatalf("test fixture didn't reproduce the bug; got:\n%q", card)
	}

	fixed := Opaque(SurfaceAlt, card)

	if !strings.Contains(fixed, bgPrefix+" ") {
		t.Errorf("expected the gap between fragments to carry the background after Opaque:\n%q", fixed)
	}
	if strings.HasSuffix(fixed, bgPrefix) {
		t.Errorf("Opaque leaked a trailing background past the final reset:\n%q", fixed)
	}
	if got := stripANSI(fixed); got != "UP 2m ago" {
		t.Errorf("Opaque altered visible text: got %q, want %q", got, "UP 2m ago")
	}
}

func TestOpaqueNoopWithoutColor(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	in := "plain text, no color profile active"
	if got := Opaque(SurfaceAlt, in); got != in {
		t.Errorf("expected Opaque to be a no-op under Ascii profile: got %q, want %q", got, in)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			b.WriteRune(r)
		}
	}
	return b.String()
}
