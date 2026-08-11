package okf

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

type FromPromptOptions struct {
	Source string
	Out    string
	About  string
	Depth  int
	Rules  []string
}

func FromPrompt(options FromPromptOptions) (string, error) {
	options.Source = strings.TrimSpace(options.Source)
	if options.Source == "" {
		return "", fmt.Errorf("from requires a source URL or path")
	}
	options.Out = strings.TrimSpace(options.Out)
	if options.Out == "" {
		return "", fmt.Errorf("from requires --out <path>")
	}
	if options.Depth < 0 {
		return "", fmt.Errorf("--depth must be zero or a positive integer")
	}
	rules, err := resolveSetupRuleSets(options.Rules)
	if err != nil {
		return "", err
	}
	selectedRuleIDs := make([]string, 0, len(rules))
	for _, rule := range rules {
		selectedRuleIDs = append(selectedRuleIDs, rule.ID)
	}

	var builder strings.Builder
	builder.WriteString("This source-to-wiki guide is meant to be executed by an AI coding or research agent.\n\n")
	builder.WriteString("If you are a human reading this in your terminal, pass it to an agent instead:\n")
	builder.WriteString("  copy this entire prompt and paste it into Codex, Claude Code, Cursor,\n")
	builder.WriteString("  Cowork, or another agent that can access the source and edit the output folder.\n\n")
	builder.WriteString("Avoid shell command substitution or piping for interactive agent CLIs; some security tools flag those patterns, and interactive agents usually need stdin to remain a terminal.\n\n")
	builder.WriteString("If you are an agent, follow the instructions below. Turn the source into an Open Knowledge wiki.\n\n")
	builder.WriteString("Simple model:\n")
	builder.WriteString("source URL or path -> local agent task -> OKF Markdown bundle\n\n")
	builder.WriteString("Inputs:\n")
	builder.WriteString(fmt.Sprintf("- Source: %s\n", markdownCode(options.Source)))
	builder.WriteString(fmt.Sprintf("- Source kind: %s\n", inferFromSourceKind(options.Source)))
	builder.WriteString(fmt.Sprintf("- Output wiki path: %s\n", markdownCode(options.Out)))
	if strings.TrimSpace(options.About) != "" {
		builder.WriteString(fmt.Sprintf("- Requested outcome: %s\n", markdownCode(strings.TrimSpace(options.About))))
	}
	if options.Depth > 0 {
		builder.WriteString(fmt.Sprintf("- Depth: %d\n", options.Depth))
	}
	builder.WriteByte('\n')

	builder.WriteString("Before writing:\n")
	builder.WriteString("- Inspect the source first. For repositories, read README files, docs, manifests, build/test files, important directories, and existing agent instructions. For websites, crawl from the source URL only as deep as requested and preserve canonical page URLs.\n")
	builder.WriteString("- If the output wiki already exists, read its index.md, log.md, AGENTS.md, and any okf_generated_from metadata before editing.\n")
	builder.WriteString("- Ask the user only for missing intent, audience, scope, or source-boundary details. Do not ask a fixed questionnaire when the source already answers the question.\n")
	builder.WriteString("- When --about is absent, ask what this wiki should help with, who it is for, what to focus on, and how deep to go.\n")
	builder.WriteByte('\n')
	builder.WriteString(renderSelectedSetupRules(rules))
	builder.WriteByte('\n')

	builder.WriteString("Generation recipe:\n")
	builder.WriteString("- Build the smallest source-grounded structure that serves the user's goal. Choose focused pages for overview, architecture, workflows, API/reference, research synthesis, glossary, diagrams, or citations when useful.\n")
	builder.WriteByte('\n')

	builder.WriteString("Write the wiki:\n")
	builder.WriteString(fmt.Sprintf("- Create or update the OKF bundle at %s. If it does not exist or is empty, initialize it with `okn scaffold --name \"<clear wiki name>\" --rules %q --no-agents %q` before customizing it.\n", markdownCode(options.Out), strings.Join(selectedRuleIDs, ","), options.Out))
	builder.WriteString("- Keep raw copied material separate from synthesized wiki pages.\n")
	builder.WriteString("- Write ordinary OKF Markdown so search and validate work without a generation runtime. Keep exact reads, browsing, and exports as optional follow-up workflows.\n")
	builder.WriteString("- Use normal concept page `type` values such as `Repository Overview`, `Architecture Overview`, `Module`, `Development Workflow`, `API Reference`, `Research Synthesis`, or `Glossary`.\n")
	builder.WriteString("- Add or update root metadata such as `okf_generation_goal`, `okf_generation_rules`, and `okf_generated_from` when useful.\n")
	builder.WriteString("- Preserve source links, source files, line ranges, commit IDs, canonical URLs, crawl depth, and fetch timestamps where available.\n")
	builder.WriteString("- For refreshes, compare existing provenance with the current source and update only affected pages where practical. Preserve human edits when possible.\n\n")

	builder.WriteString("Verify and finish:\n")
	builder.WriteString("- Remove SETUP.MD after all setup decisions are reflected in the bundle.\n")
	builder.WriteString(fmt.Sprintf("- Run `okn validate %q` and fix validation errors or avoidable warnings.\n", options.Out))
	builder.WriteString("- Ask which installed agent harnesses need Open Knowledge instructions. Also ask for the skill scope: global, project, both, or none. Explain that the global skill is reusable across knowledge bases and the project skill can contain repository-specific guidance. Ask separately whether to enable knowledge-gap observation. Observation is opt-in.\n")
	builder.WriteString(fmt.Sprintf("- Run `okn setup complete %q --skill <global|project|both|none> [--harness <codex|claude|opencode>] --observe <on|off>` with the user's selected skill scope, harnesses, and observation choice. Repeat `--harness` for each selected harness. Omit it only when the skill scope is `none` and observation is off.\n", options.Out))
	builder.WriteString("- If `okn setup complete` fails, fix the reported problem and run it again.\n")
	builder.WriteString(fmt.Sprintf("- Run one representative source-grounded query with `okn search %q \"<query>\"`; choose a query that demonstrates the wiki's intended use and confirm the returned evidence is relevant.\n", options.Out))
	builder.WriteString("- Record meaningful generation or refresh notes in log.md.\n")
	builder.WriteString("- Finish by telling the user what changed, that validation passed, which connections and skills were installed, and what the demonstrated search returned.\n")
	builder.WriteString("- Mention `okn get`, `list`, or `view` only when the user asks for exact reading, structural inspection, or human browsing.\n")
	return builder.String(), nil
}

func inferFromSourceKind(source string) string {
	parsed, err := url.Parse(source)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		host := strings.ToLower(parsed.Host)
		if host == "github.com" || strings.HasSuffix(host, ".github.com") {
			return "GitHub repository"
		}
		if host == "gitlab.com" || host == "bitbucket.org" || strings.HasSuffix(parsed.Path, ".git") {
			return "Git repository"
		}
		if strings.HasPrefix(parsed.Scheme, "http") {
			return "website"
		}
		return parsed.Scheme + " URL"
	}
	if strings.HasSuffix(source, ".git") {
		return "Git repository"
	}
	if filepath.IsAbs(source) || strings.HasPrefix(source, ".") {
		return "local path"
	}
	return "source path"
}
