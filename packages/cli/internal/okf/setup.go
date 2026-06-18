package okf

const setupPrompt = `This setup guide is meant to be executed by an AI coding agent.

If you are a human reading this in your terminal, pass it to an agent instead:
  codex "$(openknowledge setup)"

Do not pipe this into interactive Codex with openknowledge setup | codex; interactive Codex needs stdin to remain a terminal. Pipes are only appropriate for agent CLIs that explicitly accept prompts from stdin.

If you are an agent, follow the instructions below. Ask the user the setup questions, create the knowledge base with the Open Knowledge CLI, customize it for their use case, validate it, and explain what you created.

You are helping the user create an agentic LLM wiki with Open Knowledge.

Goal:
Create a useful local knowledge base, configure how agents should maintain it, and leave the user with a working wiki loop. Do not stop at a generic scaffold.

First ask only the questions needed to choose the initial shape:
1. What should this knowledge base help with?
2. Should it live inside an existing project repo, next to a project, or as a standalone wiki?
3. Which use case is closest: code/project memory, personal knowledge, team/work memory, research/data dump, recurring automation output, or something else?
4. Which ongoing behaviors should agents help with, if any: docs updates, changelog updates, decision logging, feature memory, bug triage, research import, weekly summaries, or another workflow?
5. Are there privacy, safety, source-boundary, or "do not edit" rules?

After the user answers:
- Choose a clear knowledge base name and folder path.
- If the user chose a colocated project wiki, prefer a folder such as .openknowledge or knowledge inside that project unless they ask otherwise.
- If the user chose a standalone wiki, prefer a clear standalone folder name.
- Run:
  openknowledge new --name "<knowledge base name>" "<folder path>"

After creation:
- Read SETUP.MD, AGENTS.md, SPEC.md, index.md, and log.md from the new bundle.
- Interview the user with at most five additional concise questions if needed to finish setting up the wiki.
- Customize AGENTS.md so future agents know when to read the wiki, when to update it, when not to update it, and that they must validate after meaningful wiki edits.
- Update index.md so a human or agent can quickly find the purpose, selected workflows, important pages, source material, decisions, and maintenance rules.
- Create seed pages for the selected use case. Keep the structure small and create only folders that fit the interview.
- If the user selected repeatable maintenance behaviors, create workflow docs, usually under workflows/. Each workflow should state its trigger, what to inspect, what to update, what not to update, and how to verify the result.
- If agent-tool guidance or skills would help, configure them where the agent will actually read them. For a wiki colocated with a repository, prefer repo-scoped instructions such as AGENTS.md updates or a repo-scoped skill/instruction file. For a standalone or external wiki, prefer user-scoped skill guidance when the user wants that behavior. Create wiki pages for skills only when they are useful as documentation or references, not as the default skill location.
- If the user wants recurring or external jobs, treat automations as orchestrator-native. Check whether the current agent runtime can create native automations, such as Codex app automations, Cowork automations, or another explicitly available scheduler. If it can and the user approves, configure the native automation with a prompt that references the wiki path, relevant workflows, validation command, outputs, and safety boundaries. If it cannot, or if the user does not approve installing it, do not claim an automation exists; optionally document an automation candidate or manual workflow in the wiki.
- Keep raw imported material separate from synthesized wiki pages.
- Record setup decisions in log.md.
- Run openknowledge validate "<folder path>" and fix any issues.
- Delete SETUP.MD only after setup is complete.

After setup, offer to start the local viewer with:
  openknowledge open "<folder path>"

Finish by telling the user:
- the exact path of the knowledge base
- what folders, workflows, agent instructions or skills, and native automations or automation candidates you created
- how future agents should use it
- how to inspect it with openknowledge list "<folder path>"
- how to view it with openknowledge open "<folder path>"
`

func SetupPrompt() string {
	return setupPrompt
}
