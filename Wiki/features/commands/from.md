---
type: Command Documentation
title: openknowledge prompt from
description: Prints a portable source-to-wiki prompt without starting an agent.
tags: [openknowledge, cli, command, prompt, source]
timestamp: 2026-07-17T00:00:00Z
---

# `openknowledge prompt from`

This advanced command prints the canonical instructions used to turn a
repository, folder, or website into an OKF bundle. It does not call a model or
write the wiki. For the managed workflow, use
`openknowledge setup Wiki --from <source>`; that command also validates and
integrates the result.

## Usage

```sh
openknowledge prompt from <source> --out Wiki
openknowledge prompt from <source> --out Wiki --type understanding
openknowledge prompt from <source> --out Wiki --type custom --about "Release operations"
openknowledge prompt from https://example.com/docs --out Wiki --depth 2
```

`--type understanding` is the default architecture-and-workflows recipe.
`--type custom` uses `--about` when supplied or asks the receiving agent to
clarify the goal. `--depth` is a positive crawl or traversal hint. The printed
prompt requires source provenance and validation.

The old top-level `openknowledge from` form was removed before 1.0.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/from.go`
> * `packages/cli/cmd/openknowledge/prompt_command.go`
