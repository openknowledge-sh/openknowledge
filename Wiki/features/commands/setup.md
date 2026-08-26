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
okn setup Wiki --use-case base
okn setup Wiki --use-case trusted
okn setup Wiki --use-case custom
okn setup Wiki --prompt
okn setup Wiki --interactive
okn setup Wiki --agent codex
okn setup Wiki --from ./repository
okn setup Wiki --from ./repository --about "Explain release workflows"
okn setup Wiki --from ./repository --rules project,writing,iso-plain-language
okn setup Wiki --from https://example.com/docs --depth 2
okn setup ci Wiki --plan
okn setup ci Wiki
okn setup runtime Wiki
okn setup runtime Wiki --maintenance runtime --runtimes codex
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

Use `--use-case <base|trusted|custom>` to select the setup intent. The default
is `base`.

| Intent | Use |
| --- | --- |
| `base` | Create repository documentation for architecture, services, decisions, and changelog content. |
| `trusted` | Model knowledge with explicit sources, evidence, claim lifecycle, and conflict checks. |
| `custom` | Let the source and the stated goal determine the initial structure. |

All intents use the same Markdown and OKF format. The intent changes the agent
task and starter content. It does not enable a separate runtime mode.

Use `--from <source>` to build a bundle from a repository, local folder, or
website. Use `--about <goal>` to give the intended result. Without `--about`,
the agent inspects the source and asks for the missing intent. Use `--depth <n>`
to limit source traversal. A value of `0` lets the agent choose the minimum
depth.

The intent is not a knowledge-base type. Maintenance rules are independent
choices in the setup flow.

### Product profiles

Use two cumulative profiles after the local setup:

```sh
okn setup ci Wiki
okn setup runtime Wiki
```

`setup ci` requires a Git repository. It adds one recommended Knowledge CI
contract:

- `.openknowledge/evals/knowledge.yaml` with starter questions;
- `.openknowledge/audit-sources.json` with the initial source baseline;
- `.github/workflows/openknowledge-ci.yml` with validation, base-aware claim
  lifecycle checks, audit, answer regression checks, and durable reports.

The workflow runs for pull requests, pushes to `main`, and one daily scheduled
audit. The production push creates the exact `knowledge-ci` check that runtime
publication requires.

The command preserves existing files. Use `--plan` to inspect changes without
writing. Use `--force` to replace generated profile files. Repeated setup is
idempotent.

`setup runtime` requires a GitHub origin. It creates the production MCP and
viewer runtime scaffold. The generated runtime requires the `knowledge-ci`
check, unresolved claims cannot be published, and rollback remains available.

Maintenance has one executor:

- `--maintenance github-actions` uses the installed CI workflow;
- `--maintenance runtime` enables the runtime jobs worker and writes
  `.openknowledge/jobs/knowledge-maintenance.md`. It also installs a starter
  eval dataset and source baseline for the runtime Knowledge CI pass;
- `--maintenance auto` uses GitHub Actions when `setup ci` is present. It uses
  the runtime worker otherwise.

Use `--runtimes codex,claude,opencode` to select agent harnesses for runtime
maintenance. The default runtime executor uses Codex.

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

`complete` validates the finished bundle, updates selected managed
instructions, applies the observation choice, and reports health. A CLI-only
bundle (`--skill none --observe off`) remains local and does not create a
registry connection. Project instructions or observation create the connection
they need. The command does not create wiki content, commit changes, or publish
content. It is idempotent. If it fails, fix the reported issue and run it again.

Before optional completion choices, the agent runs a representative
`okn search` query and confirms that the first result is useful.

## Status and repair

Use `okn setup status` to inspect bundle validation, connections, installed
skills, and observation state. A valid local-only bundle is healthy even when
it is not connected.

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
> * `packages/cli/cmd/openknowledge/setup_product_command.go`
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
