---
type: Command Documentation
title: openknowledge setup
description: Create or adopt a managed knowledge base from repository Markdown.
tags: [openknowledge, cli, command, setup]
timestamp: 2026-08-02T00:00:00Z
---

# `openknowledge setup`

Use `okn setup` to create a managed copy or adopt one Markdown directory.

## Usage

```sh
okn setup
okn setup Wiki --interactive
okn setup Wiki --from ./repository --mode copy
okn setup Wiki --from ./repository --mode copy --include README.md --include docs
okn setup Wiki --from ./repository --mode copy --exclude docs/generated --plan
okn setup ./docs --from ./docs --mode in-place
okn setup github Wiki --plan
okn setup github Wiki
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

On a terminal, `okn setup` discovers Markdown below the current directory.
Use `--from <directory>` to select a different discovery root. The optional
`wiki` argument selects the target. The default target is `Wiki`.

Discovery accepts ordinary Markdown without YAML frontmatter or links. It
honors Git ignore rules and skips generated directories and symbolic links.
The wizard groups discovered files and lets you select paths. You can also add
a path in the selection prompt.

The wizard provides two management modes:

| Mode | Result |
| --- | --- |
| `copy` | Create a new bundle and copy selected files below `imported/`. Keep source files unchanged. |
| `in-place` | Adopt one complete directory. Add required scaffold files and minimal metadata where needed. |

Copy mode records source paths and content digests in
`.openknowledge/import.json`. In-place mode does not accept partial include or
exclude selections. It does not move or delete existing documents.

Setup prints the create, update, preserve, move, and delete counts before it
writes. Interactive setup asks for confirmation. Use `--plan` with an explicit
`--mode` to print the deterministic plan without writes. Apply refuses to
replace an existing copy target or overwrite a file that changed after plan
creation. Writes are atomic.

After apply, setup validates the bundle and runs one representative search.
It reports the document count and readiness of validation and search.

If discovery finds no Markdown, setup asks for the goal, location, and source
context. It then creates a tailored agent task. Installed agents appear first
in the continuation list. The remaining choices copy the task or save it. No
continuation choice has a recommended label.

If the target already contains an OKF bundle, setup does not change it. It
prints commands for `okn check`, `okn review`, and `okn upgrade`.

The older `--prompt`, `--agent`, `--use-case`, `--about`, `--depth`, and
`--rules` agent workflow remains available for compatibility. Use
`--interactive` to start source selection without terminal detection.

### Product profiles

Use the GitHub Action profile after local setup. Add the runtime only when
clients need a hosted service:

```sh
okn setup github Wiki
okn setup runtime Wiki
```

`setup github` requires a Git repository. It adds these files:

- `.openknowledge/evals/knowledge.yaml` with starter questions;
- `.openknowledge/audit-sources.json` with the initial source baseline;
- `.github/workflows/openknowledge.yml` with the reusable Open Knowledge Action.

The workflow runs for pull requests, pushes to `main`, scheduled runs, and
manual runs. The Action calls `okn automation github run`.

The bridge reads `[release]` and `[maintenance]` from the bundle's
`.openknowledge.toml`. It does not observe remote sources by default.
On a push to the configured production branch, `follow-main` reports quality
failures as degraded health while keeping the release available.
`last-passing` rejects that run. Pull-request quality failures remain failing
checks under either policy.

Scheduled and manual runs skip model setup when `maintenance.mode = "off"`.
Pull-request and push checks have read-only repository permission and never
receive `OPENKNOWLEDGE_MODEL_TOKEN`. They run the native deterministic eval;
no answer command, embedding endpoint, or model API call is configured.

The `propose` and `autonomous` modes run a generated maintenance job on
scheduled and manual events. That separate job has permission to push a branch
and open a pull request. `propose` stops at the PR. `autonomous` also enables
GitHub auto-merge after repository protections pass. Configure
`OPENKNOWLEDGE_MODEL_TOKEN` for the selected agent only when either mode is
active.

`setup ci` writes the standalone `.github/workflows/openknowledge-ci.yml`
workflow. `setup runtime` does not detect or reuse that workflow.

The command preserves existing files. Use `--plan` to inspect changes without
writing. Use `--force` to replace generated profile files. Repeated setup is
idempotent.

`setup runtime` requires a GitHub origin and at least one explicit
`release.outputs` entry. It creates a runtime scaffold for exactly those
outputs. The GitHub Actions executor requires the `Open Knowledge checks`
check. Unresolved claims cannot be published, and rollback remains available.

Maintenance has one executor:

- `--maintenance github-actions` uses the installed CI workflow;
- `--maintenance runtime` enables the runtime jobs worker and writes
  `.openknowledge/jobs/knowledge-maintenance.md`. It also installs a starter
  eval dataset and source baseline for the runtime Knowledge CI pass;
- `--maintenance auto` uses GitHub Actions when `setup github` or `setup ci` is
  present. It uses the runtime worker otherwise.

Use `--runtimes codex,claude,opencode` to select agent harnesses for runtime
maintenance. The default runtime executor uses Codex.

New managed bundles enable `project` and `writing` by default. An explicit
`--rules` value replaces this default in the compatibility agent workflow.

New scaffolds record the default in `.openknowledge.toml`:

```toml
[release]
outputs = []

[rules]
enabled = ["project", "writing"]
```

The empty output list keeps a new bundle local. Before `okn publish`, select
`viewer`, `mcp`, or both. Local `okn view` does not require a release output.

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
> * `packages/cli/cmd/openknowledge/automation_github.go`
> * `action.yml`
> * `packages/cli/cmd/openknowledge/setup_lifecycle_command.go`
> * `packages/cli/cmd/openknowledge/setup_skill_command.go`
> * `packages/cli/internal/integration/manage.go`
> * `packages/cli/internal/okf/setup.go`
> * `packages/cli/internal/okf/setup_import.go`
> * `packages/cli/internal/okf/from.go`
> * `packages/cli/internal/okf/rules.go`
>
> **Update notes**
>
> Update this page when the setup workflow, task, skill installation, or
> observation behavior changes.
