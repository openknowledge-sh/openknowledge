---
type: Command Documentation
title: openknowledge agent integrate
description: Install discovery-only global skills or project-scoped Open Knowledge skills and observation hooks.
tags: [openknowledge, cli, command, integration, hooks, skills]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge agent integrate`

`openknowledge agent integrate` connects existing Codex, Claude Code, and OpenCode
sessions to Open Knowledge without replacing their native agent interfaces.

## Usage

```sh
openknowledge agent integrate --global
openknowledge agent integrate Wiki
```

## Global Discovery

`--global` installs the same small discovery skill into the user-level skill
directories for Codex, Claude Code, and OpenCode. It teaches an agent to look
for `.openknowledge/integration.toml`, inspect connected resources, and use the
read-only Open Knowledge commands. It never installs hooks, observes sessions,
or writes to a knowledge base.

## Project Integration

The project form requires a knowledge-base directory inside a Git repository.
For `openknowledge agent integrate Wiki`, it atomically writes:

```text
.openknowledge/integration.toml
.agents/skills/openknowledge/SKILL.md
.codex/hooks.json
.claude/skills/openknowledge/SKILL.md
.claude/settings.json
.opencode/plugins/openknowledge-observer.js
```

The shared `.agents/skills` copy is discovered by Codex and OpenCode; Claude
uses its native `.claude/skills` copy. Existing Codex and Claude hook arrays
are merged, not replaced, and repeated
integration is idempotent. The config stores repository-relative
`knowledge_base` and `suggestions` paths. Project skills explain the knowledge
boundary and suggestion protocol.

Codex runs the project `Stop` command hook after a turn and requires the user
to review and trust a changed project hook through `/hooks`. Claude Code runs
the equivalent command asynchronously. OpenCode invokes the observer from its
project plugin on `session.idle` and reads that session's messages through the
local OpenCode client. Codex and Claude hook payloads may instead point at their
user-owned JSON/JSONL transcript. All three feed the same bounded internal
observer, so direct harness sessions and `openknowledge agent` produce the same
suggestion format.

The hook is advisory and non-blocking: malformed input, a missing integration,
or an observer failure never blocks the parent agent session. Suggestions are
ordinary uncommitted files in the active checkout; no hook creates a branch,
commit, push, or pull request.

## Security Boundary

Project integration is explicit and repository-scoped. The observer bounds its
input, accepts transcript references only below the current user's home,
reduces the available trace to a sanitized final assistant outcome and event
counts, strips the raw session from its output, redacts common credential forms,
ignores changes below the suggestions directory, and writes only an
`okf_publish: false` Markdown suggestion. Agents and Jobs must continue to treat
that file as untrusted repository-controlled input.

The observer records changed repository paths as evidence, but copies unified
diff content only for files inside the connected knowledge base. It omits the
whole diff when credential detection triggers.

## Command Change History

### 2026-07-17 - Project integrations

Added discovery-only global skills and project skills/hooks for Codex, Claude
Code, and OpenCode, backed by `.openknowledge/integration.toml`.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/integrate_command.go`
> * `packages/cli/internal/integration/integration.go`
> * `packages/cli/internal/integration/integration_test.go`
