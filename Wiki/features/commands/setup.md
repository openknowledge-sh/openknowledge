---
type: Command Documentation
title: openknowledge setup
description: Prints an agent setup prompt for creating and customizing a knowledge base.
tags: [openknowledge, cli, command, setup]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge setup`

`openknowledge setup` prints a prompt intended for an AI coding agent. The
prompt asks the agent to inspect the current workspace or target folder, read
relevant user or project memories when the runtime exposes them, ask only the
missing setup questions, create a bundle with `openknowledge new`, customize
it, choose maintenance rules such as docs, changelog, decisions, research, bugs,
schemas, summaries, or general project memory, configure repo-scoped or
user-scoped skills with focused lower-reasoning subagent guidance when useful,
validate it, and finish with the concrete post-setup loop:
`openknowledge list`, `openknowledge search`, `openknowledge get`, and
`openknowledge view`.

The optional `--rules` flag preselects comma-separated built-in maintenance
rules in the prompt. It uses the same built-in rule IDs listed by
`openknowledge rules --list`. The generated setup prompt also tells agents to
run `openknowledge rules --list` when they need rule descriptions.

## Usage

```sh
openknowledge setup
openknowledge setup --rules docs,changelog
openknowledge setup --help
```

## Arguments And Flags

No positional arguments are accepted.

| Flag | Description |
| --- | --- |
| `--rules <rules>` | Suggest comma-separated maintenance rules for setup. |
| `--help` | Print setup-specific help. |

Built-in canonical rules are `project`, `docs`, `decisions`, `changelog`,
`research`, `bugs`, `schemas`, `summary`, and `agents`.

## Example Output

`openknowledge setup --rules docs,changelog` prints a full setup prompt for an
agent. The beginning and selected-rule block look like:

```text
This setup guide is meant to be executed by an AI coding agent.

If you are a human reading this in your terminal, pass it to an agent instead:
  codex "$(openknowledge setup)"

You are helping the user create an agentic LLM wiki with Open Knowledge.

Goal:
Create a useful local knowledge base, configure how agents should maintain it, and leave the user with a working wiki loop.

Selected maintenance rules:
Use these as the starting point for AGENTS.md, workflow docs, and any agent instruction files.
- docs: Keep docs in sync with implementation.
- changelog: Track user-facing changes.
```

## Use Cases

* Start a project wiki from inside an agent session.
* Generate a reusable bootstrap prompt for agent CLIs.
* Preselect known maintenance loops, for example docs plus changelog, while
  still letting the setup agent inspect context before creating files.
* Let setup interviews adapt to the existing workspace, project docs, and
  available agent memory instead of repeating the same generic questionnaire.
* Choose concrete maintenance rules for future agents, such as docs,
  changelog, decisions, research, bugs, schemas, summary, or project memory.
* Seed repo-scoped or user-scoped skills with guidance for spawning focused
  lower-reasoning subagents for bounded wiki maintenance tasks.
* Keep interactive agent stdin available by passing the prompt as an argument,
  for example `codex "$(openknowledge setup)"`.
* Leave the user with the use/navigation commands for the created bundle:
  `openknowledge list`, `openknowledge search`, `openknowledge get`, and
  `openknowledge view`.

## Command Change History

### 2026-07-06 - Use/navigation loop

The setup prompt, generated `SETUP.MD`, README setup prompt, and landing page
prompt now tell agents to show users how to inspect and navigate a finished
wiki with `openknowledge list`, `openknowledge search`, and
`openknowledge get`, and `openknowledge view`.

### 2026-07-05 - Maintenance rules

`openknowledge setup` gained `--rules <rules>` support. Selected
comma-separated rules are inserted into the generated setup prompt as the
starting point for
`AGENTS.md`, workflow docs, and agent instruction files. The default setup
prompt also lists the available built-in canonical rules and points the user
toward the same maintenance-loop vocabulary exposed by
`openknowledge rules --list`.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/setup.go`
> * `packages/cli/internal/okf/new.go`
> * `packages/cli/internal/okf/rules.go`
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/setup_test.go`
> * `packages/cli/internal/okf/rules_test.go`
> * `README.md`
> * `packages/web/index.html`
>
> **Update notes**
>
> The setup prompt is a product workflow, not only help text. Update
> [Feature docs workflow](/workflows/feature-docs.md) and [CLI changelog](/changelog/cli.md)
> when the interview, expected outputs, or validation loop changes.
