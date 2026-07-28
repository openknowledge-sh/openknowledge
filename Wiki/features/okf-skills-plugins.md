---
type: Concept
title: OKF, Skills, and Plugins
description: A simple user-facing comparison of raw OKF v0.1, agent skills, and plugins.
tags: [openknowledge, okf, agents, skills, plugins]
timestamp: 2026-06-20T00:00:00Z
---

# OKF, Skills, and Plugins

Many agent systems use folders, Markdown files, configuration files, and optional scripts.
Each system uses these files for a different purpose.

## OKF: knowledge

**OKF v0.1** is a simple format that stores knowledge.

An OKF bundle is a folder of Markdown files with YAML frontmatter.
Each concept file describes one useful item of knowledge.
For example, a concept can describe an API, dataset, metric, workflow, decision, system, or process.

Use OKF to make knowledge easy for people and agents to read.
You can also update, search, and move the knowledge between tools.
Use OKF for durable context, such as documentation, project memory, domain
knowledge, decisions, and relationships.

## Skills: workflows

A **skill** teaches an agent how to do a specific kind of task.

A skill usually has a `SKILL.md` file with instructions.
It can also contain references, templates, or scripts.
The agent loads the skill when the task matches.
The agent also loads the skill when a user directly invokes it.

Use a skill when an agent must follow a repeatable procedure.
For example, use a skill for a checklist, task recipe, review flow, or generation pattern.

In this repository, the wiki content is the OKF bundle and
`.codex/skills/openknowledge-wiki/SKILL.md` is the maintenance workflow for
agents that edit it.

## Plugins: packaged capabilities

A **plugin** is a package that installs capabilities into an agent runtime.

A plugin can include one or more skills.
It can also include tools, MCP servers, app integrations, hooks, agents,
settings, binaries, or marketplace metadata.

Use a plugin to share, install, or version a skill or workflow.
You can also use a plugin to package additional capabilities.
Plugins distribute reusable agent functions across projects, teams, or marketplaces.

## Simple comparison

| Concept | Main Purpose | Best For |
| --- | --- | --- |
| **OKF** | Store knowledge | Documentation, memory, domain context |
| **Skill** | Guide behavior | Repeatable agent workflows |
| **Plugin** | Package capabilities | Sharing tools, skills, integrations, and runtime extensions |

## How they fit together

These concepts can work together.
An OKF bundle contains the knowledge.
A skill tells the agent how to use the knowledge.
A plugin packages the skill or tools for distribution.
