// SPDX-License-Identifier: GPL-3.0-only

package sidecar

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Radixen-Dev/AgentRoute/internal/profile"
)

func TestRenderConfigProducesOneEntryPerTier(t *testing.T) {
	p := profile.Profile{
		Name: "work",
		Models: map[string]string{
			profile.TierHeavy:    "openrouter/anthropic/claude-opus-4.5",
			profile.TierBalanced: "openrouter/anthropic/claude-sonnet-4.5",
			profile.TierFast:     "openrouter/deepseek/deepseek-v4-flash",
		},
	}

	data, err := RenderConfig(p, "sk-or-key", "master-tok")
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}

	var cfg litellmConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal of rendered config: %v", err)
	}

	if cfg.GeneralSettings.MasterKey != "master-tok" {
		t.Fatalf("got master_key %q, want %q", cfg.GeneralSettings.MasterKey, "master-tok")
	}
	if len(cfg.ModelList) != 3 {
		t.Fatalf("got %d model_list entries, want 3", len(cfg.ModelList))
	}

	want := map[string]string{
		"agentroute-heavy":    "openrouter/anthropic/claude-opus-4.5",
		"agentroute-balanced": "openrouter/anthropic/claude-sonnet-4.5",
		"agentroute-fast":     "openrouter/deepseek/deepseek-v4-flash",
	}
	for _, entry := range cfg.ModelList {
		wantModel, ok := want[entry.ModelName]
		if !ok {
			t.Errorf("unexpected model_name %q in rendered config", entry.ModelName)
			continue
		}
		if entry.LitellmParams.Model != wantModel {
			t.Errorf("model_name %q: got model %q, want %q", entry.ModelName, entry.LitellmParams.Model, wantModel)
		}
		if entry.LitellmParams.APIKey != "sk-or-key" {
			t.Errorf("model_name %q: got api_key %q, want %q", entry.ModelName, entry.LitellmParams.APIKey, "sk-or-key")
		}
	}
}

func TestWithOpenRouterPrefix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"openrouter/anthropic/claude-opus-4.5", "openrouter/anthropic/claude-opus-4.5"},
		{"anthropic/claude-opus-4.8", "openrouter/anthropic/claude-opus-4.8"},
		{"deepseek/deepseek-chat-v3-0324", "openrouter/deepseek/deepseek-chat-v3-0324"},
		{"openrouter/deepseek/deepseek-chat-v3-0324", "openrouter/deepseek/deepseek-chat-v3-0324"},
	}
	for _, tc := range cases {
		got := withOpenRouterPrefix(tc.input)
		if got != tc.want {
			t.Errorf("withOpenRouterPrefix(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestRenderConfigNormalizesBarModelIDs(t *testing.T) {
	p := profile.Profile{
		Name: "bare",
		Models: map[string]string{
			profile.TierHeavy:    "anthropic/claude-opus-4.8",
			profile.TierBalanced: "deepseek/deepseek-chat-v3-0324",
			profile.TierFast:     "openrouter/meta-llama/llama-3.3-70b-instruct",
		},
	}

	data, err := RenderConfig(p, "key", "tok")
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}

	var cfg litellmConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	want := map[string]string{
		"agentroute-heavy":    "openrouter/anthropic/claude-opus-4.8",
		"agentroute-balanced": "openrouter/deepseek/deepseek-chat-v3-0324",
		"agentroute-fast":     "openrouter/meta-llama/llama-3.3-70b-instruct",
	}
	for _, entry := range cfg.ModelList {
		wantModel, ok := want[entry.ModelName]
		if !ok {
			t.Errorf("unexpected model_name %q", entry.ModelName)
			continue
		}
		if entry.LitellmParams.Model != wantModel {
			t.Errorf("%q: got model %q, want %q", entry.ModelName, entry.LitellmParams.Model, wantModel)
		}
	}
}

func TestRenderConfigIsDeterministic(t *testing.T) {
	p := profile.Profile{Models: map[string]string{
		profile.TierHeavy:    "openrouter/a",
		profile.TierBalanced: "openrouter/b",
		profile.TierFast:     "openrouter/c",
	}}

	first, err := RenderConfig(p, "key", "tok")
	if err != nil {
		t.Fatalf("RenderConfig #1: %v", err)
	}
	for i := 0; i < 5; i++ {
		next, err := RenderConfig(p, "key", "tok")
		if err != nil {
			t.Fatalf("RenderConfig #%d: %v", i+2, err)
		}
		if string(next) != string(first) {
			t.Fatalf("render %d differs from render 1:\n--- first ---\n%s\n--- next ---\n%s", i+2, first, next)
		}
	}
}

func TestRenderConfigNeverLeaksAPIKeyAsModelName(t *testing.T) {
	p := profile.Profile{Models: map[string]string{profile.TierBalanced: "openrouter/x"}}
	data, err := RenderConfig(p, "sk-super-secret", "tok")
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}
	if !strings.Contains(string(data), "sk-super-secret") {
		t.Fatalf("expected api_key to appear in rendered config (it's meant to be there, under api_key)")
	}
	if strings.Count(string(data), "sk-super-secret") != 1 {
		t.Fatalf("expected api_key to appear exactly once, got config: %s", data)
	}
}
