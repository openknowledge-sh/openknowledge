---
type: Command Documentation
title: openknowledge rules
description: Prints ready-to-paste maintenance instructions for agents working with an Open Knowledge wiki.
tags: [openknowledge, cli, command, agents, rules]
timestamp: 2026-07-05T00:00:00Z
---

# `openknowledge rules`

`openknowledge rules` prints a Markdown instruction block for AI agents that
maintain an Open Knowledge wiki. It is print-only: it does not create a wiki,
modify files, install skills, or register automations.

Use it when a wiki already exists, or when the user wants agent rules before
running a full setup. The printed block is designed for repository instructions
such as `AGENTS.md`, `CLAUDE.md`, Cursor project rules, or a generic agent
instruction file.

`openknowledge rules apply` is the explicit write path. It inserts or replaces
a managed rules block inside an agent instruction file.

## Usage

```sh
openknowledge rules
openknowledge rules docs,changelog --path Wiki
openknowledge rules changelog --path Wiki --target codex
openknowledge rules apply docs,changelog --path Wiki --file AGENTS.md
openknowledge rules apply changelog --path Wiki --yes
openknowledge rules apply docs --path Wiki --dry-run
openknowledge rules --list
openknowledge rules --help
```

## Arguments And Flags

`openknowledge rules` accepts at most one positional rules argument. It is a
comma-separated list of canonical rule IDs, for example `docs,changelog`. When
omitted, the selected rules default to `project`.

| Flag | Description |
| --- | --- |
| `--path <path>` | Open Knowledge wiki path used in generated rules. Defaults to `.openknowledge`. |
| `--target <target>` | Adjusts the sentence that tells the user where to paste the block. Valid values are `generic`, `codex`, `claude`, and `cursor`. Defaults to `generic`. |
| `--list` | Print an explainer plus the available canonical rules. |
| `--help` | Print command-specific help. |

`openknowledge rules apply` accepts the same optional rules argument and
`--path` flag. It also supports:

| Flag | Description |
| --- | --- |
| `--file <file>` | Agent instruction file to update. |
| `--yes` | Use the nearest detected instruction file without prompting, create `AGENTS.md` when none exists, and skip confirmation. |
| `--dry-run` | Print the managed block that would be written without editing files. |
| `--target <target>` | Override the target sentence. Defaults to the target inferred from `--file` when possible. |

## Rules

Rules are canonical IDs. The CLI intentionally does not accept extra aliases,
so users and generated instructions share one stable vocabulary. Select
multiple rules with a comma-separated list.

| Rule | Purpose |
| --- | --- |
| `project` | General project knowledge. |
| `docs` | Keep docs in sync with implementation. |
| `decisions` | Record important decisions. |
| `changelog` | Track user-facing changes. |
| `research` | Import research with citations. |
| `bugs` | Capture reusable debugging knowledge. |
| `schemas` | Document APIs, data models, configs, and contracts. |
| `summary` | Write recurring summaries. |
| `agents` | Create focused agent entrypoint docs. |

## Behavior

Without a rules argument, the command prints the `project` rule. With a
comma-separated rules list, only those selected rules are included. Duplicate
rules are deduplicated in the rendered output.

The wiki path comes from `--path` and defaults to `.openknowledge`. It is
checked before rendering. In an interactive terminal, wiki
path issues print after the generated rules as `⚠ Warning:` messages and are
spaced apart from nearby output. Each warning includes an agent action, such as
creating the wiki, selecting a folder, adding Markdown files, or running
validation. With pipes or redirection, warnings go to stderr. They do not block
stdout output. The command reports a warning when the path does not exist, is
not a directory, contains no Markdown files, or does not currently validate as
OKF. This keeps shell redirection predictable:

```sh
openknowledge rules docs --path Wiki > rules.md
```

The output always includes a small baseline loop:

* Read the wiki index before relevant work.
* Treat the wiki as durable project memory.
* Avoid inventing facts when the wiki is missing, stale, or wrong.
* Keep OKF-valid Markdown and run `openknowledge validate "<wiki>"` after
  meaningful wiki updates.

`--target` only changes the paste-location sentence. It does not create
agent-specific files.

`rules apply` writes a managed block:

```md
<!-- openknowledge:rules:start -->
...
<!-- openknowledge:rules:end -->
```

If the markers already exist, the generated block is replaced in place. Without
`--file`, `rules apply` searches from the current directory upward for the
nearest `AGENTS.md`, `CLAUDE.md`, `.cursor/rules/openknowledge.md`, or
`.cursor/rules/openknowledge.mdc`. In an interactive terminal it asks which
file to use. If the selected file already exists, interactive mode warns before
it appends a managed block or replaces an existing managed block. It shows the
generated block first, then prints the warning with the same highlighted
`⚠ Warning:` marker. The default answer is no; pass `--yes` to skip
confirmation. In text-only contexts, pass `--file` or `--yes`.

## Use Cases

* Add Open Knowledge maintenance guidance to an existing project `AGENTS.md`.
* Print Claude Code or Cursor project instructions without running setup.
* Give an agent a narrow wiki maintenance contract, such as docs plus changelog.
* Update a repo instruction file idempotently with `openknowledge rules apply`.
* Inspect the canonical rule list with `openknowledge rules --list` before
  choosing setup rules.

## Command Change History

### 2026-07-05 - Agent maintenance rules

Added `openknowledge rules` as the print-only command for generating
agent-maintenance instructions. The command accepts a comma-separated
canonical rules argument, uses `--path` for the wiki path, and shares its
canonical rule catalog with `openknowledge setup --rules`.

Added non-blocking wiki path warnings with agent actions. Added
`openknowledge rules apply` for explicit file mutation through an idempotent
managed block. Interactive `rules apply` shows the generated block, then warns
and asks for confirmation before changing an existing instruction file unless
`--yes` is passed.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/rules.go`
> * `packages/cli/internal/okf/rules_test.go`
> * `packages/cli/cmd/openknowledge/main_test.go`
> * `README.md`
>
> **Update notes**
>
> Update this page when the rule catalog, rules output contract, `--target`
> values, or setup interaction changes.
