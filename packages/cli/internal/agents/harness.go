package agents

import (
	"fmt"
	"sort"
	"strings"
)

const (
	RuntimeCodex    = "codex"
	RuntimeClaude   = "claude"
	RuntimeGrok     = "grok"
	RuntimeOpenCode = "opencode"

	PromptArgument = "argument"
	PromptStdin    = "stdin"

	AgentProtocolVersion = "openknowledge-agent/v1"
)

type HarnessDefinition struct {
	Runtime       string
	Executable    string
	ExecutableEnv string
	Credentials   []string
}

var harnessDefinitions = map[string]HarnessDefinition{
	RuntimeCodex: {
		Runtime:       RuntimeCodex,
		Executable:    "codex",
		ExecutableEnv: "OPENKNOWLEDGE_CODEX",
		Credentials:   []string{"CODEX_API_KEY"},
	},
	RuntimeClaude: {
		Runtime:       RuntimeClaude,
		Executable:    "claude",
		ExecutableEnv: "OPENKNOWLEDGE_CLAUDE",
		Credentials:   []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN"},
	},
	RuntimeGrok: {
		Runtime:       RuntimeGrok,
		Executable:    "grok",
		ExecutableEnv: "OPENKNOWLEDGE_GROK",
		Credentials:   []string{"XAI_API_KEY"},
	},
	RuntimeOpenCode: {
		Runtime:       RuntimeOpenCode,
		Executable:    "opencode",
		ExecutableEnv: "OPENKNOWLEDGE_OPENCODE",
		Credentials:   []string{"OPENCODE_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "XAI_API_KEY"},
	},
}

func SupportedAgentRuntimes() []string {
	runtimes := make([]string, 0, len(harnessDefinitions))
	for runtime := range harnessDefinitions {
		runtimes = append(runtimes, runtime)
	}
	sort.Strings(runtimes)
	return runtimes
}

func HarnessForRuntime(runtime string) (HarnessDefinition, error) {
	runtime = strings.ToLower(strings.TrimSpace(runtime))
	definition, ok := harnessDefinitions[runtime]
	if !ok {
		return HarnessDefinition{}, fmt.Errorf("unsupported agent runtime %q; expected %s", runtime, strings.Join(SupportedAgentRuntimes(), ", "))
	}
	definition.Credentials = append([]string(nil), definition.Credentials...)
	return definition, nil
}

func BuildHarnessCommand(spec AgentSpec, interactive bool) (Command, error) {
	definition, err := HarnessForRuntime(spec.Runtime)
	if err != nil {
		return Command{}, err
	}
	command := Command{
		Runtime:    definition.Runtime,
		Command:    definition.Executable,
		PromptMode: PromptArgument,
	}
	switch definition.Runtime {
	case RuntimeCodex:
		if !interactive {
			command.Args = append(command.Args, "exec")
			command.PromptMode = PromptStdin
		}
		command.Args = append(command.Args, "--sandbox", "workspace-write")
		if spec.Model != "" {
			command.Args = append(command.Args, "--model", spec.Model)
		}
		if !interactive {
			command.Args = append(command.Args, "-")
		}
	case RuntimeClaude:
		if !interactive {
			command.Args = append(command.Args,
				"--print",
				"--no-session-persistence",
				"--permission-mode", "acceptEdits",
				"--allowedTools", "Read,Glob,Grep,Edit,Write,Bash(openknowledge validate *),Bash(openknowledge list *),Bash(openknowledge get *),Bash(openknowledge search *),Bash(git status*),Bash(git diff*),Bash(git log*),Bash(git show*)",
			)
		}
		if spec.Model != "" {
			command.Args = append(command.Args, "--model", spec.Model)
		}
	case RuntimeOpenCode:
		if !interactive {
			command.Args = append(command.Args, "run", "--auto")
		}
		if spec.Model != "" {
			command.Args = append(command.Args, "--model", spec.Model)
		}
	case RuntimeGrok:
		if !interactive {
			command.Args = append(command.Args, "--no-auto-update", "--always-approve")
		}
		if spec.Model != "" {
			command.Args = append(command.Args, "--model", spec.Model)
		}
		if !interactive {
			command.Args = append(command.Args, "--single")
		}
	}
	return command, nil
}

func SteeredAgentPrompt(task string, mode string) string {
	task = strings.TrimSpace(task)
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "local"
	}
	var builder strings.Builder
	builder.WriteString("Open Knowledge agent contract (" + AgentProtocolVersion + ")\n\n")
	builder.WriteString("You are operating as an Open Knowledge Agent.\n")
	builder.WriteString("Treat files in the selected workspace as the source of truth.\n")
	builder.WriteString("Use the Open Knowledge CLI to inspect, search, validate, and export the wiki when useful.\n")
	builder.WriteString("Preserve provenance and existing repository conventions.\n")
	builder.WriteString("Respect publication boundaries. Never expose content that is not explicitly published.\n")
	builder.WriteString("Make only changes required by the task and validate the affected knowledge base before finishing.\n")
	switch mode {
	case "local":
		builder.WriteString("Edit the selected filesystem directly. Do not create a branch, commit, push, or pull request.\n")
	case "isolated":
		builder.WriteString("Work only inside the assigned isolated worktree. Do not push or create a pull request.\n")
	case "job":
		builder.WriteString("Work only inside the assigned job worktree. The Open Knowledge runtime owns commits, pushes, and pull requests.\n")
	case "init":
		builder.WriteString("Create and validate the requested Open Knowledge knowledge base in this workspace.\n")
	case "from":
		builder.WriteString("Generate the requested Open Knowledge knowledge base from the supplied source and validate it.\n")
	default:
		builder.WriteString("Execution mode: " + mode + ".\n")
	}
	if task == "" {
		builder.WriteString("\nAsk the user what knowledge-base task they want to perform, then wait for their answer.\n")
	} else {
		builder.WriteString("\nTask:\n")
		builder.WriteString(task)
		builder.WriteByte('\n')
	}
	return builder.String()
}
