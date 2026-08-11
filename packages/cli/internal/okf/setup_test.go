package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupPromptAsksAgentToBuildContextBeforeQuestions(t *testing.T) {
	prompt := SetupPrompt()
	required := []string{
		"Inspect the current workspace or folder you were spawned into",
		"relevant user or project memories",
		"Do not ask a fixed questionnaire",
		"Use these seed questions only when context cannot answer them",
		"Default rules: project and writing. Optional rules: iso-plain-language, docs, decisions, changelog, research, bugs, schemas, summary, and agents.",
		"okn prompt rules --list",
		"okn scaffold --name \"<knowledge base name>\" --rules \"project,writing\" \"<folder path>\"",
		"copy this entire prompt and paste it into Codex",
		"Avoid shell command substitution or piping",
		"context-specific questions",
		"spawn focused subagents with lower reasoning effort",
		"Keep onboarding focused on three outcomes",
		"Run okn validate \"<folder path>\"",
		"Remove SETUP.MD after all setup decisions are reflected",
		"global skill is reusable across knowledge bases",
		"project skill can contain repository-specific guidance",
		"Observation is opt-in",
		"okn setup complete \"<folder path>\" --skill <global|project|both|none> [--harness <codex|claude|opencode>] --observe <on|off>",
		"Omit --harness only when the skill scope is none and observation is off",
		"If okn setup complete fails, fix the reported problem and run it again.",
		"okn search \"<folder path>\" \"<query>\"",
		"mention okn get, list, or view only when the user asks",
	}

	for _, expected := range required {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected setup prompt to include %q:\n%s", expected, prompt)
		}
	}
	forbidden := []string{
		"codex \"$(" + "openknowledge setup --prompt)\"",
		"openknowledge setup --prompt " + "| codex",
		"After setup, offer to start the local viewer",
		"how to view it with openknowledge view",
	}
	for _, unexpected := range forbidden {
		if strings.Contains(prompt, unexpected) {
			t.Fatalf("expected setup prompt not to include %q:\n%s", unexpected, prompt)
		}
	}
}

func TestGeneratedSetupHandoffRequiresContextFirstInterview(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "contextual-memory")

	if _, err := NewProject(NewProjectOptions{Name: "Contextual Memory", Path: target}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(target, "SETUP.MD"))
	if err != nil {
		t.Fatal(err)
	}
	setup := string(content)

	required := []string{
		"the current folder, and any surrounding project context",
		"relevant user or project memories",
		"Do not ask a\nfixed generic questionnaire",
		"context-specific questions only for missing or ambiguous details",
		"which optional maintenance rules apply in addition to project and writing",
		"okn prompt rules --list",
		"spawn focused subagents with lower reasoning effort",
		"confirm okn validate passed",
		"demonstrate one budget-bounded source query with okn search",
		"mention okn get, list, or view only when the user asks",
	}

	for _, expected := range required {
		if !strings.Contains(setup, expected) {
			t.Fatalf("expected generated SETUP.MD to include %q:\n%s", expected, setup)
		}
	}
}
