// SPDX-License-Identifier: GPL-3.0-only

package theme

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestPaletteMatchesLockedBrand pins the exact hex values from the
// approved plan's branding section (§8). If this test needs to change,
// BRANDING.md must change in the same commit.
func TestPaletteMatchesLockedBrand(t *testing.T) {
	gotToWant := map[string]string{
		string(Ink): "#0F1419", string(Surface): "#171E25", string(SurfaceAlt): "#202A33",
		string(Border): "#27343F", string(AccentCyan): "#41D6C3", string(AccentBlue): "#7AA7FF",
		string(OK): "#80DF96", string(Warn): "#FFC86B", string(Err): "#FF7676",
		string(Text): "#E6EDF3", string(Muted): "#7D8A99",
	}
	for got, want := range gotToWant {
		if got != want {
			t.Errorf("token = %q, want %q", got, want)
		}
	}
}

// TestNoHardCodedHexOutsideTokens enforces the plan's branding rule: every
// color in internal/tui must be read from theme tokens, not a literal hex
// string. tokens.go (this package) is the one allowed exception.
func TestNoHardCodedHexOutsideTokens(t *testing.T) {
	hexRe := regexp.MustCompile(`#[0-9A-Fa-f]{6}`)
	tuiRoot := findTUIRoot(t)

	err := filepath.WalkDir(tuiRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if filepath.Base(path) == "tokens.go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if m := hexRe.FindString(string(data)); m != "" {
			t.Errorf("%s: hard-coded hex color %q found; use a theme.* token instead", path, m)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", tuiRoot, err)
	}
}

// findTUIRoot walks up from this test file's own directory (theme/) to
// internal/tui, so the test works regardless of the working directory
// `go test` is invoked from.
func findTUIRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Dir(wd) // theme/ -> tui/
}
