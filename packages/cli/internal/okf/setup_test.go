package okf

import (
	"strings"
	"testing"
)

func TestSetupPromptGuidesAgentSetupFlow(t *testing.T) {
	prompt := SetupPrompt()
	if strings.TrimSpace(prompt) == "" {
		t.Fatal("expected setup prompt to be non-empty")
	}

	for _, expected := range []string{
		"meant to be executed by an AI coding agent",
		`codex "$(openknowledge setup)"`,
		"interactive Codex needs stdin",
		"If you are an agent",
		"openknowledge new --name",
		"SETUP.MD",
		"AGENTS.md",
		"SPEC.md",
		"workflows/index.md",
		"skills/index.md",
		"automations/index.md",
		"docs updates",
		"changelog updates",
		"openknowledge validate",
		"openknowledge list",
		"openknowledge open",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected setup prompt to contain %q", expected)
		}
	}
}
