---
type: Command Documentation
title: openknowledge automation
description: Run unattended maintenance and hosted runtime workflows.
tags: [openknowledge, cli, command, automation, jobs, insights, runtime, deploy]
timestamp: 2026-07-28T00:00:00Z
---

# `openknowledge automation`

Use `okn automation` for workflows that run outside the direct local
knowledge loop.

## Usage

```sh
okn automation jobs <command>
okn automation insights <command>
okn automation runtime <command>
okn automation deploy <provider>
okn automation github plan --event pull_request --path Wiki
okn automation github run --event push --ref main --path Wiki
```

## Commands

| Command | Purpose |
| --- | --- |
| [`jobs`](jobs.md) | Run repeatable isolated maintenance jobs from Markdown specifications. |
| [`insights`](insights.md) | Capture and resolve maintenance insights. |
| [`runtime`](runtime.md) | Build and serve immutable knowledge generations. |
| [`deploy`](deploy.md) | Provision the runtime on a supported provider. |
| `github` | Plan or run the GitHub Action bridge from `.openknowledge.toml`. |

`automation github` accepts `pull_request`, `push`, `schedule`, and
`workflow_dispatch` events. The `plan` command prints the selected release and
maintenance actions. The `run` command executes structure and quality checks.

The command reads `[release]` and `[maintenance]` from the selected bundle.
Its plan reports `releaseOutputs`; runtime publication uses the same values.
It does not observe remote sources by default. Scheduled and manual events run
a generated maintenance job only when the configured mode is not `off`. A
successful job with changes is pushed to its isolated branch and delivered as
a pull request. `autonomous` enables GitHub auto-merge; `propose` leaves the PR
for review unless `maintenance.auto_merge` was explicitly enabled.

Pushes create a release action only when `release.outputs` is nonempty.
Structure failures always stop the run. Under the default `follow-main`
policy, claims, audit, or native eval failures on a push to the configured
production branch produce `degraded` health and a successful Action exit so
that branch remains releasable. Pull-request quality failures stay visible as
failing checks. `last-passing` returns a failing exit for the production push
too. Neither path runs model-backed answer evals.

Local commands remain at the root. For example, use `okn setup`,
`okn search`, `okn validate`, `okn view`, and `okn setup status` without the
automation prefix.

The old root forms, such as `okn jobs` and `okn runtime`, remain compatible.
They are hidden from root help. New scripts and documentation must use the
`automation` namespace.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/automation_command.go`
> * `packages/cli/cmd/openknowledge/automation_github.go`
> * `action.yml`
> * `packages/cli/cmd/openknowledge/command_catalog.go`
