// SPDX-License-Identifier: GPL-3.0-only

package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ansiReset is the SGR sequence every lipgloss Style.Render call appends
// to its output. It is the source of the bug Opaque works around: see
// Opaque's doc comment.
const ansiReset = "\x1b[0m"

// Opaque re-asserts bg immediately after every reset found in rendered,
// and at its very start.
//
// lipgloss has no way to make an outer Background() "stick" across content
// that was already run through other styles' Render calls: each of those
// calls appends a full SGR reset, which clears any background the
// enclosing style set. Concretely, building a card body like
//
//	styles.Card.Render(styles.OK.Render("UP") + " " + styles.Muted.Render("2m"))
//
// only keeps the card's background for the literal text "UP" — the space
// and "2m" fall through to the terminal's own default background the
// moment OK's reset fires, producing a visible seam. Every screen that
// composes more than one styled fragment inside a Background()-styled
// container (Card, Header, StatusBar, Toast, the splash/help backdrop)
// must pass its final string through Opaque before handing it to the
// terminal.
//
// A no-op when color is disabled (NO_COLOR, non-tty, --plain): lipgloss
// then emits no escape codes at all, so there is nothing to patch.
func Opaque(bg lipgloss.TerminalColor, rendered string) string {
	prefix := strings.TrimSuffix(lipgloss.NewStyle().Background(bg).Render(""), ansiReset)
	if prefix == "" || rendered == "" {
		return rendered
	}
	out := prefix + strings.ReplaceAll(rendered, ansiReset, ansiReset+prefix)
	// The replacement above also re-asserts bg after rendered's own final
	// reset; trim that back off so the background doesn't leak into
	// whatever the caller concatenates or prints after this string.
	return strings.TrimSuffix(out, prefix)
}
