package okf

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

const DefaultFromType = "understanding"

type FromPromptOptions struct {
	Source string
	Out    string
	Type   string
	About  string
	Depth  int
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
	wikiType, err := normalizeFromType(options.Type)
	if err != nil {
		return "", err
	}
	options.Type = wikiType
	if options.Depth < 0 {
		return "", fmt.Errorf("--depth must be zero or a positive integer")
	}

	var builder strings.Builder
	builder.WriteString("This source-to-wiki guide is meant to be executed by an AI coding or research agent.\n\n")
	builder.WriteString("If you are a human reading this in your terminal, pass it to an agent instead:\n")
	builder.WriteString("  codex \"$(")
	builder.WriteString(fromCommand(options))
	builder.WriteString(")\"\n\n")
	builder.WriteString("Do not pipe this into interactive Codex with openknowledge from ... | codex; interactive Codex needs stdin to remain a terminal. Pipes are only appropriate for agent CLIs that explicitly accept prompts from stdin.\n\n")
	builder.WriteString("If you are an agent, follow the instructions below. Turn the source into an Open Knowledge wiki.\n\n")
	builder.WriteString("Simple model:\n")
	builder.WriteString("source URL or path -> local agent task -> OKF Markdown bundle\n\n")
	builder.WriteString("Inputs:\n")
	builder.WriteString(fmt.Sprintf("- Source: %s\n", markdownCode(options.Source)))
	builder.WriteString(fmt.Sprintf("- Source kind: %s\n", inferFromSourceKind(options.Source)))
	builder.WriteString(fmt.Sprintf("- Output wiki path: %s\n", markdownCode(options.Out)))
	builder.WriteString(fmt.Sprintf("- Wiki type: %s\n", markdownCode(options.Type)))
	if strings.TrimSpace(options.About) != "" {
		builder.WriteString(fmt.Sprintf("- Custom goal: %s\n", markdownCode(strings.TrimSpace(options.About))))
	}
	if options.Depth > 0 {
		builder.WriteString(fmt.Sprintf("- Depth: %d\n", options.Depth))
	}
	builder.WriteByte('\n')

	builder.WriteString("Before writing:\n")
	builder.WriteString("- Inspect the source first. For repositories, read README files, docs, manifests, build/test files, important directories, and existing agent instructions. For websites, crawl from the source URL only as deep as requested and preserve canonical page URLs.\n")
	builder.WriteString("- If the output wiki already exists, read its index.md, log.md, AGENTS.md, and any okf_generated_from metadata before editing.\n")
	builder.WriteString("- Ask the user only for missing intent, audience, scope, or source-boundary details. Do not ask a fixed questionnaire when the source already answers the question.\n")
	if options.Type == "custom" && strings.TrimSpace(options.About) == "" {
		builder.WriteString("- Because --type custom has no --about goal, ask what this wiki should help with, who it is for, what to focus on, and how deep to go.\n")
	}
	builder.WriteByte('\n')

	builder.WriteString("Generation recipe:\n")
	switch options.Type {
	case "custom":
		if strings.TrimSpace(options.About) == "" {
			builder.WriteString("- Build a custom generation recipe from the user's answers. Choose focused rules such as overview, architecture, workflows, API/reference, research synthesis, glossary, or citations.\n")
		} else {
			builder.WriteString("- Build a custom generation recipe around the custom goal. Choose focused rules such as overview, architecture, workflows, API/reference, research synthesis, glossary, or citations.\n")
		}
	default:
		builder.WriteString("- Create a DeepWiki-style understanding wiki: overview, architecture, structure, workflows, key entrypoints, diagrams when useful, glossary, and source-backed citations.\n")
	}
	builder.WriteByte('\n')

	builder.WriteString("Write the wiki:\n")
	builder.WriteString(fmt.Sprintf("- Create or update the OKF bundle at %s. If it does not exist or is empty, initialize it with `openknowledge new --name \"<clear wiki name>\" --no-agents --no-setup %q` before customizing it.\n", markdownCode(options.Out), options.Out))
	builder.WriteString("- Use `--no-agents --no-setup` for generated source wikis unless the user explicitly wants starter agent rules or an interactive setup handoff document.\n")
	builder.WriteString("- Keep raw copied material separate from synthesized wiki pages.\n")
	builder.WriteString("- Write ordinary OKF Markdown so list, search, get, view, validate, and to work without a generation runtime.\n")
	builder.WriteString("- Use normal concept page `type` values such as `Repository Overview`, `Architecture Overview`, `Module`, `Development Workflow`, `API Reference`, `Research Synthesis`, or `Glossary`.\n")
	builder.WriteString("- Add or update root metadata such as `okf_wiki_type`, `okf_generation_goal`, `okf_generation_rules`, and `okf_generated_from` when useful.\n")
	builder.WriteString("- Preserve source links, source files, line ranges, commit IDs, canonical URLs, crawl depth, and fetch timestamps where available.\n")
	builder.WriteString("- For refreshes, compare existing provenance with the current source and update only affected pages where practical. Preserve human edits when possible.\n\n")

	builder.WriteString("Verify and finish:\n")
	builder.WriteString(fmt.Sprintf("- Run `openknowledge validate %q` and fix validation errors or avoidable warnings.\n", options.Out))
	builder.WriteString("- Record meaningful generation or refresh notes in log.md.\n")
	builder.WriteString("- Finish by telling the user what changed and how to inspect the wiki:\n")
	builder.WriteString(fmt.Sprintf("  - `openknowledge list %q`\n", options.Out))
	builder.WriteString(fmt.Sprintf("  - `openknowledge search %q \"<query>\"`\n", options.Out))
	builder.WriteString(fmt.Sprintf("  - `openknowledge get %q <file>`\n", options.Out))
	builder.WriteString(fmt.Sprintf("  - `openknowledge view %q`\n", options.Out))
	return builder.String(), nil
}

func normalizeFromType(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return DefaultFromType, nil
	}
	switch value {
	case "understanding", "custom":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported from type %q; use understanding or custom", value)
	}
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

func fromCommand(options FromPromptOptions) string {
	parts := []string{"openknowledge", "from", shellQuote(options.Source), "--out", shellQuote(options.Out)}
	if options.Type != "" && options.Type != DefaultFromType {
		parts = append(parts, "--type", shellQuote(options.Type))
	} else if options.Type == DefaultFromType {
		parts = append(parts, "--type", shellQuote(options.Type))
	}
	if strings.TrimSpace(options.About) != "" {
		parts = append(parts, "--about", shellQuote(strings.TrimSpace(options.About)))
	}
	if options.Depth > 0 {
		parts = append(parts, "--depth", fmt.Sprintf("%d", options.Depth))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z') &&
			!(r >= 'a' && r <= 'z') &&
			!(r >= '0' && r <= '9') &&
			!strings.ContainsRune("@%_+=:,./-", r)
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
