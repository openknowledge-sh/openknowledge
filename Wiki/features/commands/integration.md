---
type: Command Documentation
title: openknowledge integration
description: Install and manage one agent-runtime integration.
tags: [openknowledge, cli, command, integration, hooks, skills]
timestamp: 2026-07-28T00:00:00Z
---

# `openknowledge integration`

Use `okn integration` to connect one agent runtime to a project knowledge
base. The command supports Codex, Claude Code, and OpenCode.

## Usage

```sh
okn integration install Wiki --runtime codex
okn integration install Wiki --runtime claude --observe
okn integration install --global --runtime opencode
okn integration status
okn integration remove
```

## Project installation

`install` requires a knowledge base inside a Git repository. It writes
`.openknowledge/integration.toml` and the skill for the selected runtime.
It does not write files for the other runtimes.

| Runtime | Project skill |
| --- | --- |
| `codex` | `.agents/skills/openknowledge/SKILL.md` |
| `claude` | `.claude/skills/openknowledge/SKILL.md` |
| `opencode` | `.opencode/skills/openknowledge/SKILL.md` |

Session observation is off by default. Add `--observe` to install the
selected runtime observer. Codex uses `.codex/hooks.json`. Claude Code uses
`.claude/settings.json`. OpenCode uses
`.opencode/plugins/openknowledge-observer.js`.

The configuration records the runtime, observation state, managed paths,
ownership, and content digests. An install does not overwrite an unknown file
or a modified managed file. Remove the current integration before you select
a different runtime.

## Status and removal

`status` reads the project configuration. It reports each managed file as
`managed`, `modified`, `missing`, or `unreadable`. It does not change files.

`remove` deletes unchanged files that the integration created. It preserves a
file if the user changed it or if the file existed before installation. For a
Codex or Claude Code settings file, it removes only the Open Knowledge hook.
It preserves all other settings.

The configuration accepts only the known paths for its selected runtime. A
modified configuration cannot make `remove` delete an arbitrary repository
path.

## Global discovery

Use `--global` with `install` and one `--runtime`. This form installs one
discovery skill in the selected runtime directory. It never installs a hook or
enables session observation.

## Compatibility

`okn agent integrate` remains as a deprecated alias for
`okn integration install`. The alias also requires `--runtime`.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/integrate_command.go`
> * `packages/cli/internal/integration/manage.go`
> * `packages/cli/internal/integration/integration.go`
> * `packages/cli/internal/integration/integration_test.go`
