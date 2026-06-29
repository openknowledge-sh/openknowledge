---
type: Concept
title: OKF, Skills, and Plugins
description: A simple user-facing comparison of raw OKF v0.1, agent skills, and plugins.
tags: [openknowledge, okf, agents, skills, plugins]
timestamp: 2026-06-20T00:00:00Z
---

# OKF, Skills, and Plugins

Many agent systems use the same basic materials: folders, Markdown files,
config files, and optional scripts. The difference is not the file format. The
difference is what those files are for.

## OKF: Knowledge

**OKF v0.1** is a simple format for storing knowledge.

An OKF bundle is a folder of Markdown files with YAML frontmatter. Each concept
file can describe an API, dataset, metric, workflow, decision, system, process,
or other useful piece of knowledge.

Use OKF when you want knowledge to be easy for both humans and agents to read,
update, search, and move between tools. It is best for durable context:
documentation, project memory, domain knowledge, decisions, and relationships
between concepts.

## Skills: Workflows

A **skill** teaches an agent how to do a specific kind of task.

A skill usually has a `SKILL.md` file with instructions, plus optional
references, templates, or scripts. The agent loads the skill when the task
matches, or when the user invokes it directly.

Use a skill when you want the agent to follow a repeatable procedure. It is best
for checklists, task recipes, review flows, generation patterns, or any workflow
you do not want to explain from scratch every time.

In this repository, the wiki content is the OKF bundle and
`.codex/skills/openknowledge-wiki/SKILL.md` is the maintenance workflow for
agents editing it.

## Plugins: Packaged Capabilities

A **plugin** is a package that installs capabilities into an agent runtime.

A plugin may include one or more skills, but it can also include tools, MCP
servers, app integrations, hooks, agents, settings, binaries, or marketplace
metadata.

Use a plugin when a skill or workflow needs to be shared, installed, versioned,
or bundled with extra capabilities. It is best for distributing reusable agent
functionality across projects, teams, or marketplaces.

## Simple Comparison

| Concept | Main Purpose | Best For |
| --- | --- | --- |
| **OKF** | Store knowledge | Documentation, memory, domain context |
| **Skill** | Guide behavior | Repeatable agent workflows |
| **Plugin** | Package capabilities | Sharing tools, skills, integrations, and runtime extensions |

## How They Fit Together

These concepts can work together: an OKF bundle holds the knowledge, a skill
tells the agent how to use it, and a plugin packages the skill or tools for
distribution.
