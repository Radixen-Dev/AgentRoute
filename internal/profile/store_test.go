// SPDX-License-Identifier: GPL-3.0-only

package profile

import (
	"testing"
)

func withIsolatedStateDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
}

func TestSaveLoadRoundTrip(t *testing.T) {
	withIsolatedStateDir(t)

	want := Profile{
		Name: "work",
		Port: 4505,
		Models: map[string]string{
			TierHeavy:    "openrouter/anthropic/claude-opus-4.5",
			TierBalanced: "openrouter/anthropic/claude-sonnet-4.5",
			TierFast:     "openrouter/deepseek/deepseek-v4-flash",
		},
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load("work")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Name != want.Name || got.Port != want.Port {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got.Created.IsZero() {
		t.Fatalf("expected Created to be auto-populated")
	}
	for tier, model := range want.Models {
		if got.Models[tier] != model {
			t.Fatalf("tier %q: got %q, want %q", tier, got.Models[tier], model)
		}
	}
}

func TestLoadNotFound(t *testing.T) {
	withIsolatedStateDir(t)
	if _, err := Load("does-not-exist"); err != ErrNotFound {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestInvalidNameRejected(t *testing.T) {
	withIsolatedStateDir(t)
	cases := []string{"", "../escape", "a/b", `a\b`}
	for _, name := range cases {
		if err := Save(Profile{Name: name}); err != ErrInvalidName {
			t.Errorf("Save(%q): got %v, want ErrInvalidName", name, err)
		}
		if _, err := Load(name); err != ErrInvalidName {
			t.Errorf("Load(%q): got %v, want ErrInvalidName", name, err)
		}
	}
}

func TestListSortedByName(t *testing.T) {
	withIsolatedStateDir(t)

	for _, name := range []string{"zeta", "alpha", "mu"} {
		if err := Save(Profile{Name: name, Port: 4505}); err != nil {
			t.Fatalf("Save(%q): %v", name, err)
		}
	}

	profiles, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(profiles) != 3 {
		t.Fatalf("got %d profiles, want 3", len(profiles))
	}
	got := []string{profiles[0].Name, profiles[1].Name, profiles[2].Name}
	want := []string{"alpha", "mu", "zeta"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got order %v, want %v", got, want)
		}
	}
}

func TestExistsAndDelete(t *testing.T) {
	withIsolatedStateDir(t)

	if exists, err := Exists("work"); err != nil || exists {
		t.Fatalf("Exists before Save: got (%v, %v), want (false, nil)", exists, err)
	}

	if err := Save(Profile{Name: "work", Port: 4505}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if exists, err := Exists("work"); err != nil || !exists {
		t.Fatalf("Exists after Save: got (%v, %v), want (true, nil)", exists, err)
	}

	if err := Delete("work"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if exists, err := Exists("work"); err != nil || exists {
		t.Fatalf("Exists after Delete: got (%v, %v), want (false, nil)", exists, err)
	}

	// Deleting again is a no-op, not an error.
	if err := Delete("work"); err != nil {
		t.Fatalf("Delete (idempotent): %v", err)
	}
}

func TestAlias(t *testing.T) {
	if got := Alias(TierBalanced); got != "agentroute-balanced" {
		t.Fatalf("Alias(%q) = %q, want %q", TierBalanced, got, "agentroute-balanced")
	}
}

func TestProfileAliasesKeysAllTiers(t *testing.T) {
	p := Profile{Models: map[string]string{
		TierHeavy:    "openrouter/a",
		TierBalanced: "openrouter/b",
		TierFast:     "openrouter/c",
	}}
	got := p.Aliases()

	// Aliases must keep returning a plain map[string]string so callers can
	// assign it directly to gateway.MapRouter (defined as the same
	// underlying type) without profile importing gateway. If this line
	// fails to compile after a refactor, the conversion site in the "up"
	// orchestration (gateway.MapRouter(p.Aliases())) needs an explicit fix
	// too.
	//nolint:staticcheck // explicit type is intentional: a type conversion
	// (which the suggested fix would require) is assignable even across
	// distinct named types, silently defeating the guard described above.
	var _ map[string]string = got

	want := map[string]string{
		"agentroute-heavy":    "openrouter/a",
		"agentroute-balanced": "openrouter/b",
		"agentroute-fast":     "openrouter/c",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for alias, model := range want {
		if got[alias] != model {
			t.Errorf("got[%q] = %q, want %q", alias, got[alias], model)
		}
	}
}

func TestSaveOverwritesExisting(t *testing.T) {
	withIsolatedStateDir(t)

	if err := Save(Profile{Name: "work", Port: 4505, Models: map[string]string{TierHeavy: "old"}}); err != nil {
		t.Fatalf("Save #1: %v", err)
	}
	if err := Save(Profile{Name: "work", Port: 4505, Models: map[string]string{TierHeavy: "new"}}); err != nil {
		t.Fatalf("Save #2: %v", err)
	}

	got, err := Load("work")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Models[TierHeavy] != "new" {
		t.Fatalf("got %q, want %q", got.Models[TierHeavy], "new")
	}
}
