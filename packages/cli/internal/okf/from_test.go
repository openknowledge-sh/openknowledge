package okf

import (
	"strings"
	"testing"
)

func TestFromPromptBuildsPortableAgentTask(t *testing.T) {
	prompt, err := FromPrompt(FromPromptOptions{
		Source: "https://github.com/openknowledge-sh/openknowledge",
		Out:    "Wiki",
		Type:   "understanding",
		Depth:  2,
	})
	if err != nil {
		t.Fatal(err)
	}
	required := []string{
		"source URL or path -> local agent task -> OKF Markdown bundle",
		"Source: `https://github.com/openknowledge-sh/openknowledge`",
		"Source kind: GitHub repository",
		"Output wiki path: `Wiki`",
		"Wiki type: `understanding`",
		"Depth: 2",
		"copy this entire prompt and paste it into Codex",
		"Avoid shell command substitution or piping",
		"DeepWiki-style understanding wiki",
		"openknowledge scaffold --name \"<clear wiki name>\" --no-agents --no-setup \"Wiki\"",
		"unless the user explicitly wants starter agent rules or an interactive setup handoff document",
		"okf_generated_from",
		"list, search, get, view, validate, and export work",
		"openknowledge validate \"Wiki\"",
		"openknowledge list \"Wiki\"",
		"openknowledge search \"Wiki\" \"<query>\"",
		"openknowledge get \"Wiki\" <file>",
		"openknowledge view \"Wiki\"",
	}
	for _, expected := range required {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected from prompt to include %q:\n%s", expected, prompt)
		}
	}
	forbidden := []string{
		"codex \"$(" + "openknowledge prompt from",
		"openknowledge prompt from ... " + "| codex",
	}
	for _, unexpected := range forbidden {
		if strings.Contains(prompt, unexpected) {
			t.Fatalf("expected from prompt not to include %q:\n%s", unexpected, prompt)
		}
	}
}

func TestFromPromptCustomWithoutAboutAsksForIntent(t *testing.T) {
	prompt, err := FromPrompt(FromPromptOptions{
		Source: "https://openknowledge.sh/wiki/",
		Out:    "Wiki",
		Type:   "custom",
	})
	if err != nil {
		t.Fatal(err)
	}
	required := []string{
		"Source kind: website",
		"Wiki type: `custom`",
		"Because --type custom has no --about goal",
		"what this wiki should help with",
		"Build a custom generation recipe from the user's answers",
	}
	for _, expected := range required {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected custom prompt to include %q:\n%s", expected, prompt)
		}
	}
}

func TestFromPromptRejectsUnsupportedType(t *testing.T) {
	_, err := FromPrompt(FromPromptOptions{
		Source: "https://github.com/openknowledge-sh/openknowledge",
		Out:    "Wiki",
		Type:   "reference",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported from type") {
		t.Fatalf("expected unsupported type error, got %v", err)
	}
}
