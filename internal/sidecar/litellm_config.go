// SPDX-License-Identifier: GPL-3.0-only

// Package sidecar manages the v1 LiteLLM proxy subprocess that serves the
// Anthropic-wire translator until v2's native Go translator replaces it
// (see the architecture plan, §5.4).
package sidecar

import (
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/Radixen-Dev/AgentRoute/internal/profile"
)

type litellmModelEntry struct {
	ModelName     string        `yaml:"model_name"`
	LitellmParams litellmParams `yaml:"litellm_params"`
}

type litellmParams struct {
	Model  string `yaml:"model"`
	APIKey string `yaml:"api_key"`
}

type litellmGeneralSettings struct {
	MasterKey string `yaml:"master_key"`
}

type litellmConfig struct {
	ModelList       []litellmModelEntry    `yaml:"model_list"`
	GeneralSettings litellmGeneralSettings `yaml:"general_settings"`
}

// RenderConfig builds the LiteLLM proxy YAML config for p's tier->model
// mappings. Every model entry uses apiKey (the user's OPENROUTER_API_KEY) as
// its credential. masterKey becomes LiteLLM's own required bearer token, so
// only the AgentRoute gateway (which is the only holder of masterKey) can
// reach the sidecar directly.
//
// Model names in the rendered config are AgentRoute aliases (see
// profile.Alias), matching exactly what the gateway's ModelRouter resolves
// requests to before proxying here — so the sidecar receives a model name
// it already has a model_list entry for.
func RenderConfig(p profile.Profile, apiKey, masterKey string) ([]byte, error) {
	cfg := litellmConfig{
		GeneralSettings: litellmGeneralSettings{MasterKey: masterKey},
	}
	for tier, model := range p.Models {
		cfg.ModelList = append(cfg.ModelList, litellmModelEntry{
			ModelName:     profile.Alias(tier),
			LitellmParams: litellmParams{Model: model, APIKey: apiKey},
		})
	}
	// Deterministic output: map iteration order is randomized in Go, but
	// config diffs (and tests) should be stable across renders of the same
	// profile.
	sort.Slice(cfg.ModelList, func(i, j int) bool {
		return cfg.ModelList[i].ModelName < cfg.ModelList[j].ModelName
	})

	return yaml.Marshal(cfg)
}
