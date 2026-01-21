package provider

import (
	"testing"
)

func TestFromConfig_ClaudeExplicit(t *testing.T) {
	p, err := FromConfig(Config{Type: ProviderClaude})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if p.Name() != ProviderClaude {
		t.Errorf("expected claude, got %q", p.Name())
	}
}

func TestFromConfig_ClaudeDefault(t *testing.T) {
	p, err := FromConfig(Config{Type: ""})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if p.Name() != ProviderClaude {
		t.Errorf("expected claude as default, got %q", p.Name())
	}
}

func TestFromConfig_CodexExplicit(t *testing.T) {
	p, err := FromConfig(Config{Type: ProviderCodex})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if p.Name() != ProviderCodex {
		t.Errorf("expected codex, got %q", p.Name())
	}
}

func TestFromConfig_CustomCommand(t *testing.T) {
	p, err := FromConfig(Config{Type: ProviderClaude, Command: "/usr/local/bin/claude"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if p.Name() != ProviderClaude {
		t.Errorf("expected claude, got %q", p.Name())
	}
}

func TestFromConfig_UnknownProvider(t *testing.T) {
	_, err := FromConfig(Config{Type: "gpt4"})
	if err == nil {
		t.Error("expected error for unknown provider type")
	}
}

func TestFromConfig_EmptyConfig(t *testing.T) {
	p, err := FromConfig(Config{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if p.Name() != ProviderClaude {
		t.Errorf("expected claude as default for empty config, got %q", p.Name())
	}
}
