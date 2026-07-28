---
type: Command Documentation
title: openknowledge prompt from
description: Prints a portable source-to-wiki prompt. It does not start an agent.
tags: [openknowledge, cli, command, prompt, source]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge prompt from`

This advanced command prints instructions that convert a repository, folder,
or website into an OKF bundle. It does not call a model or write the wiki.

For a managed workflow, use
`openknowledge setup Wiki --from <source>`. That command also validates and
integrates the result.

## Usage

```sh
openknowledge prompt from <source> --out Wiki
openknowledge prompt from <source> --out Wiki --type understanding
openknowledge prompt from <source> --out Wiki --type custom --about "Release operations"
openknowledge prompt from https://example.com/docs --out Wiki --depth 2
```

`--type understanding` is the default architecture and workflow recipe.
`--type custom` uses `--about` when you supply it. Otherwise, the prompt asks
the agent to clarify the goal.

`--depth` is a nonnegative crawl or traversal hint. A value of `0` lets the
agent select the minimum depth. The prompt requires source provenance and
validation.

Open Knowledge removed the old top-level `openknowledge from` form before 1.0.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/from.go`
> * `packages/cli/cmd/openknowledge/prompt_command.go`
