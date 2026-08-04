package okf

import (
	"strings"
	"testing"
)

func TestFromPromptBuildsPortableAgentTask(t *testing.T) {
	prompt, err := FromPrompt(FromPromptOptions{
		Source: "https://github.com/openknowledge-sh/openknowledge",
		Out:    "Wiki",
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
		"Depth: 2",
		"copy this entire prompt and paste it into Codex",
		"Avoid shell command substitution or piping",
		"Build the smallest source-grounded structure",
		"okn scaffold --name \"<clear wiki name>\" --no-agents \"Wiki\"",
		"okf_generated_from",
		"search and validate work without a generation runtime",
		"okn validate \"Wiki\"",
		"Remove SETUP.MD after all setup decisions are reflected",
		"global skill is reusable across knowledge bases",
		"project skill can contain repository-specific guidance",
		"Observation is opt-in",
		"okn setup complete \"Wiki\" --skill <global|project|both|none> [--harness <codex|claude|opencode>] --observe <on|off>",
		"Omit it only when the skill scope is `none` and observation is off",
		"If `okn setup complete` fails, fix the reported problem and run it again.",
		"okn search \"Wiki\" \"<query>\"",
		"confirm the returned evidence is relevant",
		"what the demonstrated search returned",
		"Mention `okn get`, `list`, or `view` only when the user asks",
	}
	for _, expected := range required {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected from prompt to include %q:\n%s", expected, prompt)
		}
	}
	forbidden := []string{
		"codex \"$(" + "openknowledge setup --prompt --from",
		"openknowledge setup --prompt --from ... " + "| codex",
	}
	for _, unexpected := range forbidden {
		if strings.Contains(prompt, unexpected) {
			t.Fatalf("expected from prompt not to include %q:\n%s", unexpected, prompt)
		}
	}
	for _, unexpected := range []string{
		"`openknowledge list \"Wiki\"`",
		"`openknowledge get \"Wiki\" <file>`",
		"`openknowledge view \"Wiki\"`",
	} {
		if strings.Contains(prompt, unexpected) {
			t.Fatalf("expected onboarding prompt not to prescribe %q:\n%s", unexpected, prompt)
		}
	}
}

func TestFromPromptWithoutAboutAsksForIntent(t *testing.T) {
	prompt, err := FromPrompt(FromPromptOptions{
		Source: "https://openknowledge.sh/wiki/",
		Out:    "Wiki",
	})
	if err != nil {
		t.Fatal(err)
	}
	required := []string{
		"Source kind: website",
		"When --about is absent",
		"what this wiki should help with",
		"Build the smallest source-grounded structure",
	}
	for _, expected := range required {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected custom prompt to include %q:\n%s", expected, prompt)
		}
	}
}
