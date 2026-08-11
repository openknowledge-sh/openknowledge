---
type: Command Documentation
title: openknowledge setup
description: Set up, complete, inspect, and repair an Open Knowledge bundle.
tags: [openknowledge, cli, command, setup]
timestamp: 2026-08-02T00:00:00Z
---

# `openknowledge setup`

Use `okn setup` to create or complete a knowledge base with an agent.

## Usage

```sh
okn setup
okn setup Wiki --prompt
okn setup Wiki --interactive
okn setup Wiki --agent codex
okn setup Wiki --from ./repository
okn setup Wiki --from ./repository --about "Explain release workflows"
okn setup Wiki --from ./repository --rules project,writing,iso-plain-language
okn setup Wiki --from https://example.com/docs --depth 2
okn setup skill
okn setup skill --scope global --harness codex
okn setup skill --scope project --project ./repository --harness codex
okn setup complete Wiki --skill project --harness codex --observe off
okn setup status
okn setup repair
okn setup observe on
```

## Setup modes

On a terminal, `okn setup` starts the setup wizard. The wizard detects the
project context and asks only for missing setup decisions.

Without a terminal, `okn setup` prints the agent task. Use `--prompt` to print
the task in any environment. Use `--interactive` to start the wizard.

Use `--agent <codex|claude|opencode>` to start one installed agent harness.
The agent works from the project directory that contains the target bundle.
Setup does not require a Git repository.

The optional `wiki` argument selects the bundle path. The default is `Wiki`.

Use `--from <source>` to build a bundle from a repository, local folder, or
website. Use `--about <goal>` to give the intended result. Without `--about`,
the agent inspects the source and asks for the missing intent. Use `--depth <n>`
to limit source traversal. A value of `0` lets the agent choose the minimum
depth.

Setup has no knowledge-base type option. Maintenance rules are independent
choices in the setup flow.

The setup wizard selects `project` and `writing` by default. It offers
`iso-plain-language` and the other maintenance rules as optional selections.
An explicit `--rules` value replaces the default selection.

The same selection applies to ordinary setup and `--from` setup. The generated
task includes each selected rule's instructions and exact configuration.

The generated scaffold command persists the selection before content creation.

New scaffolds record the default in `.openknowledge.toml`:

```toml
[rules]
enabled = ["project", "writing"]
```

## Skill installation

Use `okn setup skill` to install or update Open Knowledge instructions without
the complete knowledge-base setup flow. The command does not require a Wiki
argument.

On a terminal, the command first asks for the `global`, `project`, or `both`
scope. For a project scope, select a compatible local registry entry or enter
a local knowledge base path. Then, select one or more detected agent harnesses.

For noninteractive use, set `--scope <global|project|both>`. Repeat
`--harness <codex|claude|opencode>` to select more than one harness. Set
`--project <registry-key|path>` when the selected scope includes a project.

A global installation does not require a Wiki, project, or registry entry.

## Completion

The generated agent task creates or updates the bundle. It then removes
`SETUP.MD`, runs `okn validate`, and fixes errors and avoidable warnings.

The task finishes technical installation with:

```sh
okn setup complete Wiki \
  --skill <global|project|both|none> \
  --harness <codex|claude|opencode> \
  --observe <on|off>
```

`--skill` selects instruction scope. `global` installs instructions for the
current user. `project` installs project-local instructions. `both` installs
both scopes. `none` installs no instructions.

Repeat `--harness` to select more than one supported harness. With the `none`
skill scope, use `--harness` only when observation is on. Observation is disabled
unless `--observe on` is explicit.

`complete` validates the finished bundle, creates a missing connection, updates
selected managed instructions, applies the observation choice, and reports
health. It does not create wiki content, commit changes, or publish content.
The command is idempotent. If it fails, fix the reported issue and run it again.

The agent then runs a representative `okn search` query and reports the result.

## Status and repair

Use `okn setup status` to inspect bundle validation, connections, installed
skills, and observation state.

Use `okn setup repair` to repair managed skill blocks, harness adapters, and
local observation configuration. The command does not change wiki content or
user-managed project guidance.

Use `okn setup observe on` only after you approve local capture of possible
knowledge gaps. Use `okn setup observe off` to disable capture.

Product telemetry is separate from observation. The `--observe` choice does not
enable, disable, or change product telemetry.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/setup_command.go`
> * `packages/cli/cmd/openknowledge/setup_lifecycle_command.go`
> * `packages/cli/cmd/openknowledge/setup_skill_command.go`
> * `packages/cli/internal/integration/manage.go`
> * `packages/cli/internal/okf/setup.go`
> * `packages/cli/internal/okf/from.go`
> * `packages/cli/internal/okf/rules.go`
>
> **Update notes**
>
> Update this page when the setup workflow, task, skill installation, or
> observation behavior changes.
