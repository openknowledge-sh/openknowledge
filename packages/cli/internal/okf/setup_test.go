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
		"Available rules: project, docs, decisions, changelog, research, bugs, schemas, summary, agents.",
		"openknowledge rules --list",
		"context-specific questions",
		"spawn focused subagents with lower reasoning effort",
		"openknowledge search \"<folder path>\" \"<query>\"",
		"openknowledge get \"<folder path>\" \"<file>\"",
		"openknowledge view \"<folder path>\"",
	}

	for _, expected := range required {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected setup prompt to include %q:\n%s", expected, prompt)
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
		"which maintenance rules apply",
		"openknowledge rules --list",
		"spawn focused subagents with lower reasoning effort",
		"read exact Markdown with openknowledge get",
		"browse it with openknowledge view",
	}

	for _, expected := range required {
		if !strings.Contains(setup, expected) {
			t.Fatalf("expected generated SETUP.MD to include %q:\n%s", expected, setup)
		}
	}
}
