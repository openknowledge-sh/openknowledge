package agents

import (
	"reflect"
	"strings"
	"testing"
)

func TestHarnessCommandsUseCanonicalHeadlessContracts(t *testing.T) {
	tests := []struct {
		runtime    string
		model      string
		command    string
		args       []string
		promptMode string
	}{
		{runtime: RuntimeCodex, model: "gpt-test", command: "codex", args: []string{"exec", "--sandbox", "workspace-write", "--model", "gpt-test", "-"}, promptMode: PromptStdin},
		{runtime: RuntimeClaude, model: "sonnet", command: "claude", args: []string{"--print", "--no-session-persistence", "--permission-mode", "acceptEdits", "--allowedTools", "Read,Glob,Grep,Edit,Write,Bash(openknowledge validate *),Bash(openknowledge list *),Bash(openknowledge get *),Bash(openknowledge search *),Bash(git status*),Bash(git diff*),Bash(git log*),Bash(git show*)", "--model", "sonnet"}, promptMode: PromptArgument},
		{runtime: RuntimeOpenCode, model: "provider/model", command: "opencode", args: []string{"run", "--auto", "--model", "provider/model"}, promptMode: PromptArgument},
	}
	for _, test := range tests {
		t.Run(test.runtime, func(t *testing.T) {
			command, err := BuildHarnessCommand(AgentSpec{Runtime: test.runtime, Model: test.model}, false)
			if err != nil {
				t.Fatal(err)
			}
			if command.Command != test.command || command.PromptMode != test.promptMode || !reflect.DeepEqual(command.Args, test.args) {
				t.Fatalf("unexpected command: %#v", command)
			}
		})
	}
}

func TestSteeredAgentPromptSeparatesContractModeAndTask(t *testing.T) {
	prompt := SteeredAgentPrompt("Refresh the whitepaper.", "job")
	for _, expected := range []string{AgentProtocolVersion, "Open Knowledge Agent", "publication boundaries", "runtime owns commits", "Task:\nRefresh the whitepaper."} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("missing %q in prompt:\n%s", expected, prompt)
		}
	}
}

func TestHarnessRegistryRejectsUnknownRuntime(t *testing.T) {
	if _, err := HarnessForRuntime("unsupported"); err == nil || !strings.Contains(err.Error(), "claude, codex, opencode") {
		t.Fatalf("unexpected runtime validation: %v", err)
	}
}
