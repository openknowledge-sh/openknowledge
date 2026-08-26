package okf

import "strings"

const setupPrompt = `This setup guide is meant to be executed by an AI coding agent.

If you are a human reading this in your terminal, pass it to an agent instead:
  copy this entire prompt and paste it into Codex, Claude Code, Cursor,
  Cowork, or another coding agent that can edit this workspace.

Avoid shell command substitution or piping for interactive agent CLIs; some security tools flag those patterns, and interactive agents usually need stdin to remain a terminal.

If you are an agent, follow the instructions below. First build context, then ask tailored setup questions, create the knowledge base with the Open Knowledge CLI, customize it for their use case, validate it, and explain what you created.

You are helping the user create a flexible knowledge base in Markdown with Open Knowledge.

Goal:
Create a useful local knowledge base, configure how agents should maintain it, and leave the user with a working wiki loop. Do not stop at a generic scaffold.

Keep onboarding focused on three outcomes: create the smallest useful wiki,
validate it, and demonstrate one source-grounded search. Do not introduce the
viewer, publishing, registry, runtime, jobs, automations, deterministic
scaffolding, or portable prompt workflows unless the user explicitly asks for
them or their stated use case requires them.

Before asking the user:
- Inspect the current workspace or folder you were spawned into. Prefer cheap, focused reads such as README files, AGENTS or instruction files, package manifests, docs, existing Open Knowledge bundles, and obvious source or data folders.
- If your runtime exposes relevant user or project memories, read only the small subset that applies to this setup. Respect privacy, safety, source-boundary, and "do not edit" rules.
- Infer the likely domain, audience, source systems, candidate wiki location, maintenance workflows, and constraints.
- Do not ask a fixed questionnaire when the context already answers it. Ask only the missing or ambiguous questions needed to make the setup correct.

Use these seed questions only when context cannot answer them:
1. What should this knowledge base help with?
2. Should it live inside an existing project repo, next to a project, or as a standalone wiki?
3. Which existing sources or context should the knowledge base use?
4. Which maintenance rules should future agents follow, if any?
   Default rules: project and writing. Optional rules: iso-plain-language, docs, decisions, changelog, research, bugs, schemas, summary, and agents. Run okn prompt rules --list for descriptions.
5. Are there privacy, safety, source-boundary, or "do not edit" rules?

After the user answers:
- Choose a clear knowledge base name and folder path.
- If the user chose a colocated project wiki, prefer a folder such as .openknowledge or knowledge inside that project unless they ask otherwise.
- If the user chose a standalone wiki, prefer a clear standalone folder name.
- Run:
  okn scaffold --name "<knowledge base name>" "<folder path>"

After creation:
- Read SETUP.MD, AGENTS.md, SPEC.md, index.md, and log.md from the new bundle.
- Re-check the final bundle location and surrounding project context. If the bundle is colocated with or next to a project, inspect that project before asking follow-up questions.
- Reuse relevant user or project memories only when the current agent runtime exposes them and they apply to this setup.
- Interview the user with at most five additional concise, context-specific questions if needed to finish setting up the wiki.
- Customize AGENTS.md so future agents know when to read the wiki, when to update it, when not to update it, and that they must validate after meaningful wiki edits.
- Update index.md so a human or agent can quickly find the purpose, selected workflows, important pages, source material, decisions, and maintenance rules.
- Create seed pages for the selected use case. Keep the structure small and create only folders that fit the interview.
- If the user selected repeatable maintenance behaviors, create workflow docs, usually under workflows/. Each workflow should state its trigger, what to inspect, what to update, what not to update, and how to verify the result.
- If agent-tool guidance or skills would help, configure them where the agent will actually read them. For a wiki colocated with a repository, prefer repo-scoped instructions such as AGENTS.md updates or a repo-scoped skill/instruction file. For a standalone or external wiki, prefer user-scoped skill guidance when the user wants that behavior. When creating repo-scoped or user-scoped skills, include guidance to spawn focused subagents with lower reasoning effort for bounded wiki maintenance tasks when the runtime supports that. Create wiki pages for skills only when they are useful as documentation or references, not as the default skill location.
- If the user wants recurring or external jobs, treat automations as orchestrator-native. Check whether the current agent runtime can create native automations, such as Codex app automations, Cowork automations, or another explicitly available scheduler. If it can and the user approves, configure the native automation with a prompt that references the wiki path, relevant workflows, validation command, outputs, and safety boundaries. If it cannot, or if the user does not approve installing it, do not claim an automation exists; optionally document an automation candidate or manual workflow in the wiki.
- Keep raw imported material separate from synthesized wiki pages.
- Record setup decisions in log.md.
- Remove SETUP.MD after all setup decisions are reflected in the bundle.
- Run okn validate "<folder path>" and fix all errors and avoidable warnings.
- Run one representative query with okn search "<folder path>" "<query>" and confirm the returned evidence is relevant.
- If the setup task does not include a preselected activation plan, ask the user which installed agent harnesses need Open Knowledge instructions. Also ask for the skill scope: global, project, both, or none. Explain that the global skill is reusable across knowledge bases. Explain that the project skill can contain repository-specific guidance. Ask separately whether to enable knowledge-gap observation. Observation is opt-in.
- Run:
  okn setup complete "<folder path>" --skill <global|project|both|none> [--harness <codex|claude|opencode>] --observe <on|off>
- Use the user's selected skill scope, harnesses, and observation choice.
- Repeat --harness for each selected harness. Omit --harness only when the skill scope is none and observation is off.
- If okn setup complete fails, fix the reported problem and run it again.

Finish by telling the user:
- the exact path of the knowledge base
- what folders, workflows, agent instructions or skills, and native automations or automation candidates you created
- how future agents should use it
- that validation passed
- which connections and skills were installed
- how to search it with okn search "<folder path>" "<query>"
- mention okn get, list, or view only when the user asks for exact
  reading, structural inspection, or human browsing
`

func SetupPrompt() string {
	prompt, _ := SetupPromptWithOptions(SetupPromptOptions{})
	return prompt
}

func SetupPromptWithOptions(options SetupPromptOptions) (string, error) {
	rules, err := resolveSetupRuleSets(options.Rules)
	if err != nil {
		return "", err
	}
	ruleIDs := make([]string, 0, len(rules))
	for _, rule := range rules {
		ruleIDs = append(ruleIDs, rule.ID)
	}
	prompt := strings.Replace(
		setupPrompt,
		`okn scaffold --name "<knowledge base name>" "<folder path>"`,
		`okn scaffold --name "<knowledge base name>" --rules "`+strings.Join(ruleIDs, ",")+`" "<folder path>"`,
		1,
	)
	prompt = strings.Replace(
		prompt,
		"Before asking the user:\n",
		setupUseCaseInstructions(options.UseCase)+"\nBefore asking the user:\n",
		1,
	)
	selected := renderSelectedSetupRules(rules)
	return strings.Replace(prompt, "\nAfter the user answers:", selected+"\nAfter the user answers:", 1), nil
}

func setupUseCaseOutcome(useCase string) string {
	switch strings.ToLower(strings.TrimSpace(useCase)) {
	case "trusted-knowledge":
		return "trusted knowledge across multiple sources"
	case "custom":
		return "a custom knowledge base"
	default:
		return "searchable documentation for the codebase"
	}
}

func setupUseCaseInstructions(useCase string) string {
	switch strings.ToLower(strings.TrimSpace(useCase)) {
	case "trusted-knowledge":
		return `First working result: trusted knowledge across multiple sources
- Create source-grounded knowledge across the selected sources.
- Preserve provenance, disagreement, lifecycle, and access boundaries when the sources support them.
- Start with ordinary Markdown and one useful search.
- Ask which trust capabilities the user needs after the first result.
- Add typed claims, corpus schema, semantic query, Knowledge CI, or runtime only when a stated requirement needs each capability.
- Do not enable the complete trust stack by default.`
	case "custom":
		return `First working result: a custom knowledge base
- Ask for the intended outcome only when workspace context does not make it clear.
- Create the smallest useful Markdown knowledge base for that outcome.
- Validate it and demonstrate one useful search before you offer optional capabilities.`
	default:
		return `First working result: searchable documentation for the codebase
- Create searchable, maintainable documentation for this codebase.
- Prefer focused pages for the product, architecture, services or modules, development workflows, decisions, and changelog when the repository supports them.
- Use ordinary Markdown with the minimum required frontmatter.
- Validate the documentation and demonstrate one useful search.
- Do not introduce typed claims, evidence receipts, corpus schema, semantic query, Knowledge CI, or runtime unless the user asks for them later.`
	}
}

func resolveSetupRuleSets(ids []string) ([]RuleSet, error) {
	if len(ids) == 0 {
		ids = []string{"project", "writing"}
	}
	return ResolveRuleSets(ids)
}
