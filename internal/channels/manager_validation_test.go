package channels

import (
	"testing"

	"github.com/fractalmind-ai/fractalbot/internal/config"
)

func TestValidateOhMyCodeAgentConfig(t *testing.T) {
	cases := []struct {
		name         string
		defaultAgent string
		allowed      []string
		wantErr      bool
	}{
		{
			name:         "default-empty-allowed-empty",
			defaultAgent: "",
			allowed:      nil,
		},
		{
			name:         "default-empty-allowed-set",
			defaultAgent: "",
			allowed:      []string{"qa-1"},
			wantErr:      true,
		},
		{
			name:         "default-invalid",
			defaultAgent: "-bad",
			allowed:      nil,
			wantErr:      true,
		},
		{
			name:         "allowlist-invalid",
			defaultAgent: "qa-1",
			allowed:      []string{"-bad"},
			wantErr:      true,
		},
		{
			name:         "default-not-in-allowlist",
			defaultAgent: "qa-1",
			allowed:      []string{"coder-a"},
			wantErr:      true,
		},
		{
			name:         "default-in-allowlist",
			defaultAgent: "qa-1",
			allowed:      []string{"qa-1", "coder-a"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOhMyCodeAgentConfig(tc.defaultAgent, tc.allowed)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestActiveChannelAgentConfigUnionsAgentRouters(t *testing.T) {
	defaultAgent, allowed, source := activeChannelAgentConfig(&config.AgentsConfig{
		Router:       "ohMyCode",
		AgentRouters: map[string]string{"trader": "grokBotApp"},
		OhMyCode: &config.OhMyCodeConfig{
			Enabled:       true,
			DefaultAgent:  "main",
			AllowedAgents: []string{"main", "admin"},
		},
		GrokBotApp: &config.GrokBotAppConfig{
			Enabled:       true,
			DefaultAgent:  "trader",
			AllowedAgents: []string{"trader"},
		},
	})
	if defaultAgent != "main" {
		t.Fatalf("defaultAgent=%q, want main", defaultAgent)
	}
	if source != "agents.ohMyCode" {
		t.Fatalf("source=%q, want agents.ohMyCode", source)
	}
	if err := validateOhMyCodeAgentConfig(defaultAgent, allowed); err != nil {
		t.Fatalf("union allowlist should remain valid: %v", err)
	}
	got := map[string]bool{}
	for _, name := range allowed {
		got[name] = true
	}
	for _, name := range []string{"main", "admin", "trader"} {
		if !got[name] {
			t.Fatalf("allowed=%v missing %q", allowed, name)
		}
	}
}

func TestActiveChannelAgentConfigWithoutAgentRoutersKeepsBaseAllowlist(t *testing.T) {
	_, allowed, _ := activeChannelAgentConfig(&config.AgentsConfig{
		Router: "ohMyCode",
		OhMyCode: &config.OhMyCodeConfig{
			DefaultAgent:  "main",
			AllowedAgents: []string{"main"},
		},
	})
	if len(allowed) != 1 || allowed[0] != "main" {
		t.Fatalf("allowed=%v", allowed)
	}
}
