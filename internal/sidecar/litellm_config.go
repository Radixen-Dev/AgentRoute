// SPDX-License-Identifier: GPL-3.0-only

// Package sidecar manages the v1 LiteLLM proxy subprocess that serves the
// Anthropic-wire translator until v2's native Go translator replaces it
// (see the architecture plan, §5.4).
package sidecar

import (
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Radixen-Dev/AgentRoute/internal/profile"
)

type litellmModelEntry struct {
	ModelName     string        `yaml:"model_name"`
	LitellmParams litellmParams `yaml:"litellm_params"`
}

type litellmParams struct {
	Model               string   `yaml:"model"`
	APIKey              string   `yaml:"api_key"`
	AllowedOpenAIParams []string `yaml:"allowed_openai_params,omitempty"`
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
// profile.Alias). The gateway forwards requests with the alias intact so
// LiteLLM can look it up here, along with the api_key, and route to OpenRouter.
func RenderConfig(p profile.Profile, apiKey, masterKey string) ([]byte, error) {
	cfg := litellmConfig{
		GeneralSettings: litellmGeneralSettings{MasterKey: masterKey},
	}
	for tier, model := range p.Models {
		cfg.ModelList = append(cfg.ModelList, litellmModelEntry{
			ModelName: profile.Alias(tier),
			LitellmParams: litellmParams{
				Model:               withOpenRouterPrefix(model),
				APIKey:              apiKey,
				AllowedOpenAIParams: []string{"thinking", "betas"},
			},
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

// withOpenRouterPrefix ensures the model string has the "openrouter/" prefix
// that LiteLLM requires to route through OpenRouter. OpenRouter's own catalog
// returns bare provider IDs (e.g. "anthropic/claude-opus-4.8"), so profiles
// created from the model picker won't have the prefix unless we add it here.
func withOpenRouterPrefix(model string) string {
	if model == "" || strings.HasPrefix(model, "openrouter/") {
		return model
	}
	return "openrouter/" + model
}
