---
type: Command Documentation
title: openknowledge agent integrate
description: Install discovery-only global skills or project-scoped Open Knowledge skills and observation hooks.
tags: [openknowledge, cli, command, integration, hooks, skills]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge agent integrate`

Use `okn agent integrate` to connect existing agent sessions to Open
Knowledge. The command supports Codex, Claude Code, and OpenCode. It does not
replace their native interfaces.

## Usage

```sh
okn agent integrate --global
okn agent integrate Wiki
```

## Global discovery

`--global` installs a discovery skill in each user-level skill directory. The
skill supports Codex, Claude Code, and OpenCode.

The skill finds `.openknowledge/integration.toml` and inspects connected
resources. It also uses read-only Open Knowledge commands. This mode does not
install hooks, observe sessions, or write to a knowledge base.

## Project integration

The project form requires a knowledge base inside a Git repository. For
`okn agent integrate Wiki`, the command atomically writes:

```text
.openknowledge/integration.toml
.agents/skills/openknowledge/SKILL.md
.codex/hooks.json
.claude/skills/openknowledge/SKILL.md
.claude/settings.json
.opencode/plugins/openknowledge-observer.js
```

Codex and OpenCode use the shared `.agents/skills` copy. Claude uses its native
`.claude/skills` copy. The command merges existing Codex and Claude hook
arrays. It does not replace them. Repeated integration produces the same
result.

The configuration stores repository-relative `knowledge_base` and `insights`
paths. Project skills explain the knowledge boundary and insight protocol.

Codex runs the project `Stop` command hook after a turn. Review and trust a
changed project hook through `/hooks`. Claude Code runs the equivalent command
asynchronously.

OpenCode starts the observer from its project plugin on `session.idle`. It
reads the session messages through the local OpenCode client. Codex and Claude
hook payloads can point to a user-owned JSON or JSONL transcript.

All three harnesses use the same bounded internal observer. Direct sessions and
`okn agent` sessions produce the same insight format.

The hook is advisory and does not block the parent agent session. Malformed
input, a missing integration, or an observer failure does not block the
session.

Insights are uncommitted files in the active checkout. A hook does not create a
branch, commit, push, or pull request.

## Security boundary

Project integration is explicit and applies only to the repository. The
observer limits its input. It accepts transcript references only below the
current user home directory.

The observer keeps only a sanitized assistant result and event counts. It
removes the raw session and common credential forms from its output. It ignores
changes below the insights directory. It writes only a Markdown insight with
`okf_publish: false`.

Agents and jobs must treat the insight as untrusted repository input.

The observer records changed repository paths as evidence. It does not copy
file contents, a unified diff, or a base commit into the insight.


---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/integrate_command.go`
> * `packages/cli/internal/integration/integration.go`
> * `packages/cli/internal/integration/integration_test.go`
