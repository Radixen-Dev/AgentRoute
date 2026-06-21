// SPDX-License-Identifier: GPL-3.0-only

// Package profile manages saved AgentRoute profiles: named sets of
// per-tier OpenRouter model choices (the generic "heavy"/"balanced"/"fast"
// roles described in the architecture plan), persisted as JSON files
// under the AgentRoute state directory.
package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Radixen-Dev/AgentRoute/internal/fsutil"
	"github.com/Radixen-Dev/AgentRoute/internal/paths"
)

// Tier names AgentRoute exposes to every platform adapter. A given
// platform may use a subset (see internal/platform.Role).
const (
	TierHeavy    = "heavy"
	TierBalanced = "balanced"
	TierFast     = "fast"
)

// Alias returns the stable AgentRoute model alias for a tier, e.g.
// "balanced" -> "agentroute-balanced". This is the exact string platform
// adapters configure their tool to send as the model, and the exact string
// ModelRouter.Resolve is called with.
func Alias(tier string) string {
	return "agentroute-" + tier
}

// Profile is a named, saved set of per-tier OpenRouter model choices.
type Profile struct {
	Name    string            `json:"name"`
	Port    int               `json:"port"`
	Models  map[string]string `json:"models"` // tier -> "openrouter/<id>"
	Created time.Time         `json:"created"`
}

// Aliases returns p's tier->model mappings keyed by AgentRoute alias (see
// Alias) instead of bare tier name. The result is directly assignable to a
// gateway.MapRouter (map[string]string with the same underlying type).
func (p Profile) Aliases() map[string]string {
	out := make(map[string]string, len(p.Models))
	for tier, model := range p.Models {
		out[Alias(tier)] = model
	}
	return out
}

// RoleAliases returns p's tier->alias mapping (e.g. "heavy" ->
// "agentroute-heavy"), independent of which upstream model each alias
// currently resolves to. This is what platform.LinkInput.RoleAliases
// expects: a platform adapter configures its tool to request an alias for
// each tier; resolving that alias to a concrete model is the gateway's
// ModelRouter's job (see Aliases for that mapping), not the adapter's.
func (p Profile) RoleAliases() map[string]string {
	out := make(map[string]string, len(p.Models))
	for tier := range p.Models {
		out[tier] = Alias(tier)
	}
	return out
}

// ErrNotFound is returned by Load when no profile with the given name
// exists.
var ErrNotFound = fmt.Errorf("profile: not found")

// ErrInvalidName is returned when a profile name would not round-trip
// safely as a filename (empty, or containing a path separator).
var ErrInvalidName = fmt.Errorf("profile: invalid name")

func validateName(name string) error {
	if name == "" {
		return ErrInvalidName
	}
	if strings.ContainsAny(name, `/\`) || name != filepath.Base(name) {
		return ErrInvalidName
	}
	return nil
}

func pathFor(name string) (string, error) {
	dir, err := paths.ProfilesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".json"), nil
}

// Save persists p atomically. If p.Created is zero, it is set to now.
func Save(p Profile) error {
	if err := validateName(p.Name); err != nil {
		return err
	}
	if p.Created.IsZero() {
		p.Created = time.Now().UTC()
	}

	path, err := pathFor(p.Name)
	if err != nil {
		return fmt.Errorf("profile: resolve path: %w", err)
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("profile: encode %q: %w", p.Name, err)
	}
	data = append(data, '\n')

	return fsutil.AtomicWrite(path, data, 0o600)
}

// Load reads a single profile by name.
func Load(name string) (Profile, error) {
	if err := validateName(name); err != nil {
		return Profile{}, err
	}
	path, err := pathFor(name)
	if err != nil {
		return Profile{}, fmt.Errorf("profile: resolve path: %w", err)
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("profile: read %q: %w", name, err)
	}

	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("profile: parse %q: %w", name, err)
	}
	return p, nil
}

// Exists reports whether a profile with the given name has been saved.
func Exists(name string) (bool, error) {
	if err := validateName(name); err != nil {
		return false, err
	}
	path, err := pathFor(name)
	if err != nil {
		return false, fmt.Errorf("profile: resolve path: %w", err)
	}
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("profile: stat %q: %w", name, err)
	}
	return true, nil
}

// Delete removes a saved profile. Deleting a profile that does not exist
// is a no-op (returns nil), matching idempotent CLI semantics.
func Delete(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	path, err := pathFor(name)
	if err != nil {
		return fmt.Errorf("profile: resolve path: %w", err)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("profile: delete %q: %w", name, err)
	}
	return nil
}

// List returns all saved profiles, sorted by name.
func List() ([]Profile, error) {
	dir, err := paths.ProfilesDir()
	if err != nil {
		return nil, fmt.Errorf("profile: resolve profiles dir: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("profile: read profiles dir: %w", err)
	}

	var profiles []Profile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		p, err := Load(name)
		if err != nil {
			return nil, fmt.Errorf("profile: load %q while listing: %w", name, err)
		}
		profiles = append(profiles, p)
	}

	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles, nil
}
