package okf

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const DefaultRulesWiki = ".openknowledge"
const RulesBlockStart = "<!-- openknowledge:rules:start -->"
const RulesBlockEnd = "<!-- openknowledge:rules:end -->"

type RuleSet struct {
	ID             string
	Label          string
	Summary        string
	Rules          []string
	ReviewPrompt   string
	ReviewEvidence []string
}

type AgentRulesOptions struct {
	Wiki    string
	Rules   []string
	Target  string
	Managed bool
}

type SetupPromptOptions struct {
	Rules []string
}

func RuleSets() []RuleSet {
	return []RuleSet{
		{
			ID:      "project",
			Label:   "Project",
			Summary: "General project knowledge.",
			Rules: []string{
				"Before non-trivial work, read the wiki index and follow only links relevant to the task.",
				"After work creates durable project knowledge, update or add the matching concept pages.",
				"Keep the wiki structure small and shaped around the project's real workflows.",
			},
		},
		{
			ID:      "docs",
			Label:   "Docs",
			Summary: "Keep docs in sync with implementation.",
			Rules: []string{
				"When behavior, APIs, commands, configs, or examples change, update the matching docs in the same task.",
				"Preserve source anchors or citations when docs depend on implementation details.",
				"Keep docs focused on shipped behavior; label planned work clearly.",
			},
		},
		{
			ID:      "decisions",
			Label:   "Decisions",
			Summary: "Record important decisions.",
			Rules: []string{
				"When a meaningful technical or product decision is made, record the context, options, chosen path, and tradeoffs.",
				"Link decisions to affected concepts, workflows, commands, systems, or source files.",
				"Do not rewrite decision history to hide old context; append clarifications or superseding decisions.",
			},
		},
		{
			ID:      "changelog",
			Label:   "Changelog",
			Summary: "Track user-facing changes.",
			Rules: []string{
				"When user-facing behavior, command flags, output, validation, publishing, packaging, or setup changes, update changelog memory.",
				"Include what changed, why it matters, source anchors, and docs updated.",
				"Skip changelog entries for formatting-only edits or internal cleanup with no user-visible effect.",
			},
		},
		{
			ID:      "research",
			Label:   "Research",
			Summary: "Import research with citations.",
			Rules: []string{
				"Keep raw sources separate from synthesized wiki pages.",
				"Preserve source links, file paths, quotes, or citations for claims that depend on external material.",
				"Do not turn uncertain or unsupported research into asserted project knowledge.",
			},
		},
		{
			ID:      "bugs",
			Label:   "Bugs",
			Summary: "Capture reusable debugging knowledge.",
			Rules: []string{
				"When a bug investigation produces reusable knowledge, capture symptoms, reproduction, root cause, fix, tests, and follow-up risks.",
				"Link bug notes to affected commands, modules, workflows, decisions, or runbooks.",
				"Separate confirmed facts from hypotheses and stale investigation notes.",
			},
		},
		{
			ID:      "schemas",
			Label:   "Schemas",
			Summary: "Document APIs, data models, configs, and contracts.",
			Rules: []string{
				"Create or update concepts for APIs, schemas, tables, config keys, data models, and contracts when their source changes.",
				"Prefer source pointers over copying generated or code-derived truth into prose.",
				"Keep schema docs linked to the authoritative source files, specs, or systems.",
			},
		},
		{
			ID:      "summary",
			Label:   "Summary",
			Summary: "Write recurring summaries.",
			Rules: []string{
				"When asked for a summary cycle, collect recent changes from reliable sources such as git history, issues, logs, or updated wiki pages.",
				"Write dated summaries that link back to the concepts or source material they summarize.",
				"Do not claim a recurring automation exists unless the current agent runtime actually created it.",
			},
		},
		{
			ID:      "agents",
			Label:   "Agents",
			Summary: "Create agent entrypoint docs.",
			Rules: []string{
				"Create focused agent entrypoint docs only when a repeated agent workflow needs a stable handoff.",
				"Keep entrypoints short and link to deeper wiki concepts instead of duplicating the wiki.",
				"When useful, wire entrypoints through bundle metadata such as okf_bundle_entry_*.",
			},
		},
	}
}

func RenderAgentRules(options AgentRulesOptions) (string, error) {
	wiki := strings.TrimSpace(options.Wiki)
	if wiki == "" {
		wiki = DefaultRulesWiki
	}
	target := strings.TrimSpace(options.Target)
	if target == "" {
		target = "generic"
	}
	targetLine, err := ruleTargetLine(target)
	if options.Managed {
		targetLine, err = managedRuleTargetLine(target)
	}
	if err != nil {
		return "", err
	}
	ruleSets, err := ResolveRuleSetsForWiki(wiki, options.Rules)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("## Open Knowledge Maintenance\n\n")
	builder.WriteString(fmt.Sprintf("This project has an Open Knowledge wiki at %s.\n\n", markdownCode(wiki)))
	builder.WriteString(targetLine)
	builder.WriteString("\n\n")
	builder.WriteString("Before relevant work:\n")
	builder.WriteString(fmt.Sprintf("- Read %s and follow only links relevant to the task.\n", markdownCode(wikiIndexPath(wiki))))
	builder.WriteString("- Treat the wiki as durable project memory, not as a scratchpad.\n")
	builder.WriteString("- If the wiki is missing, stale, or wrong, say so instead of inventing facts.\n\n")
	builder.WriteString("Enabled rules:\n")
	for _, ruleSet := range ruleSets {
		builder.WriteString(fmt.Sprintf("- %s: %s\n", ruleSet.ID, ruleSet.Summary))
	}
	builder.WriteByte('\n')
	for _, ruleSet := range ruleSets {
		builder.WriteString(ruleSet.Label)
		builder.WriteString(" rules:\n")
		for _, rule := range ruleSet.Rules {
			builder.WriteString("- ")
			builder.WriteString(rule)
			builder.WriteByte('\n')
		}
		builder.WriteByte('\n')
	}
	builder.WriteString("After wiki updates:\n")
	builder.WriteString("- Keep non-reserved Markdown files OKF-valid with YAML frontmatter and a non-empty `type`.\n")
	builder.WriteString("- Update `index.md` links when pages are added, moved, or removed.\n")
	builder.WriteString("- Update `log.md` when durable wiki knowledge changes.\n")
	builder.WriteString(fmt.Sprintf("- Run `openknowledge validate %q` before finishing.\n", wiki))
	return builder.String(), nil
}

func RulesWikiWarnings(wiki string) []string {
	wiki = strings.TrimSpace(wiki)
	if wiki == "" {
		wiki = DefaultRulesWiki
	}
	info, err := os.Stat(wiki)
	if os.IsNotExist(err) {
		return []string{fmt.Sprintf("Open Knowledge wiki path does not exist: %s. Agent action: create the wiki first, for example with `openknowledge scaffold %q`, or rerun with an existing wiki path.", wiki, wiki)}
	}
	if err != nil {
		return []string{fmt.Sprintf("Open Knowledge wiki path could not be inspected: %s (%v). Agent action: check filesystem permissions or choose another wiki path before relying on these rules.", wiki, err)}
	}
	if !info.IsDir() {
		return []string{fmt.Sprintf("Open Knowledge wiki path is not a directory: %s. Agent action: use an existing Open Knowledge folder or create one with `openknowledge scaffold <folder>`.", wiki)}
	}

	markdownCount := 0
	walkErr := filepath.WalkDir(wiki, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			markdownCount++
		}
		return nil
	})
	if walkErr != nil {
		return []string{fmt.Sprintf("Open Knowledge wiki path could not be scanned for Markdown: %s (%v). Agent action: fix filesystem access or invalid directory entries, then rerun this command.", wiki, walkErr)}
	}
	if markdownCount == 0 {
		return []string{fmt.Sprintf("Open Knowledge wiki path contains no Markdown files: %s. Agent action: initialize it with `openknowledge scaffold %q` if it is empty, or add OKF Markdown files before relying on these rules.", wiki, wiki)}
	}

	result, err := Validate(wiki)
	if err != nil {
		return []string{fmt.Sprintf("Open Knowledge wiki path could not be validated as OKF: %s (%v). Agent action: run `openknowledge validate %q`, fix the setup issue, then rerun this command.", wiki, err, wiki)}
	}
	if len(result.Errors) > 0 {
		return []string{fmt.Sprintf("Open Knowledge wiki path does not currently validate as OKF (%d errors). Agent action: run `openknowledge validate %q` and fix validation errors before relying on these rules.", len(result.Errors), wiki)}
	}
	if len(result.Warnings) > 0 {
		return []string{fmt.Sprintf("Open Knowledge wiki path validates with warnings (%d warnings). Agent action: run `openknowledge validate %q` and review warnings before depending on this maintenance loop.", len(result.Warnings), wiki)}
	}
	return nil
}

func RenderManagedRulesBlock(rules string) string {
	rules = strings.TrimSpace(rules)
	if rules == "" {
		return RulesBlockStart + "\n" + RulesBlockEnd + "\n"
	}
	return RulesBlockStart + "\n" + rules + "\n" + RulesBlockEnd + "\n"
}

func UpsertManagedRulesBlock(existing string, block string) string {
	block = strings.TrimRight(block, "\n") + "\n"
	start := strings.Index(existing, RulesBlockStart)
	end := strings.Index(existing, RulesBlockEnd)
	if start >= 0 && end >= start {
		end += len(RulesBlockEnd)
		updated := existing[:start] + strings.TrimRight(block, "\n") + existing[end:]
		return strings.TrimRight(updated, "\n") + "\n"
	}
	existing = strings.TrimRight(existing, "\n")
	if existing == "" {
		return block
	}
	return existing + "\n\n" + block
}

func RenderSetupRulesList() string {
	var builder strings.Builder
	builder.WriteString("Available maintenance rules:\n")
	for _, ruleSet := range RuleSets() {
		builder.WriteString(fmt.Sprintf("- %s: %s\n", ruleSet.ID, ruleSet.Summary))
	}
	return builder.String()
}

func renderSelectedSetupRules(ruleSets []RuleSet) string {
	if len(ruleSets) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("\nSelected maintenance rules:\n")
	builder.WriteString("Use these as the starting point for AGENTS.md, workflow docs, and any agent instruction files. Ask the user before adding or removing rules if the workspace context is unclear.\n")
	for _, ruleSet := range ruleSets {
		builder.WriteString(fmt.Sprintf("- %s: %s\n", ruleSet.ID, ruleSet.Summary))
	}
	return builder.String()
}

func ruleTargetLine(target string) (string, error) {
	switch target {
	case "generic":
		return "Add this block to the project instruction file your agent reads.", nil
	case "codex":
		return "Add this block to the repository `AGENTS.md` file for Codex.", nil
	case "claude":
		return "Add this block to `CLAUDE.md` for Claude Code.", nil
	case "cursor":
		return "Add this block to Cursor project rules.", nil
	default:
		return "", fmt.Errorf("unsupported rules target %q; use generic, codex, claude, or cursor", target)
	}
}

func managedRuleTargetLine(target string) (string, error) {
	switch target {
	case "generic":
		return "This block is managed by `openknowledge prompt rules apply`.", nil
	case "codex":
		return "This Codex instruction block is managed by `openknowledge prompt rules apply`.", nil
	case "claude":
		return "This Claude Code instruction block is managed by `openknowledge prompt rules apply`.", nil
	case "cursor":
		return "This Cursor rules block is managed by `openknowledge prompt rules apply`.", nil
	default:
		return "", fmt.Errorf("unsupported rules target %q; use generic, codex, claude, or cursor", target)
	}
}

func markdownCode(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "\\`") + "`"
}

func wikiIndexPath(wiki string) string {
	return strings.TrimRight(wiki, "/") + "/index.md"
}
