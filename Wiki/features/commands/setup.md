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
it, configure repo-scoped or user-scoped skills with focused lower-reasoning
subagent guidance when useful, validate it, and show how to inspect it.

## Usage

```sh
openknowledge setup
openknowledge setup --help
```

## Arguments And Flags

No arguments are accepted. `--help` prints setup-specific help.

## Use Cases

* Start a project wiki from inside an agent session.
* Generate a reusable bootstrap prompt for agent CLIs.
* Let setup interviews adapt to the existing workspace, project docs, and
  available agent memory instead of repeating the same generic questionnaire.
* Seed repo-scoped or user-scoped skills with guidance for spawning focused
  lower-reasoning subagents for bounded wiki maintenance tasks.
* Keep interactive agent stdin available by passing the prompt as an argument,
  for example `codex "$(openknowledge setup)"`.

## Source Anchors

* `packages/cli/internal/okf/setup.go`
* `packages/cli/cmd/openknowledge/main.go`
* `README.md`

## Update Notes

The setup prompt is a product workflow, not only help text. Update
[Feature docs workflow](/workflows/feature-docs.md) and [CLI changelog](/changelog/cli.md)
when the interview, expected outputs, or validation loop changes.
