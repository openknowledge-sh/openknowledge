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
validate it, and show how to inspect it.

The optional `--rules` flag preselects comma-separated maintenance rules in the
prompt. It uses the same canonical rule IDs listed by
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

Canonical rules are `project`, `docs`, `decisions`, `changelog`, `research`,
`bugs`, `schemas`, `summary`, and `agents`.

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

## Command Change History

### 2026-07-05 - Maintenance rules

`openknowledge setup` gained `--rules <rules>` support. Selected
comma-separated rules are inserted into the generated setup prompt as the
starting point for
`AGENTS.md`, workflow docs, and agent instruction files. The default setup
prompt also lists the available canonical rules and points the user toward the
same maintenance-loop vocabulary exposed by `openknowledge rules --list`.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/setup.go`
> * `packages/cli/internal/okf/rules.go`
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/rules_test.go`
> * `README.md`
>
> **Update notes**
>
> The setup prompt is a product workflow, not only help text. Update
> [Feature docs workflow](/workflows/feature-docs.md) and [CLI changelog](/changelog/cli.md)
> when the interview, expected outputs, or validation loop changes.
