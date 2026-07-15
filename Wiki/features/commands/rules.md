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
a managed rules block inside an agent instruction file. Before writing, it
resolves the target canonically and refuses files inside a registered `read`
knowledge base. A local connection must use `--access write` to permit the
change. `--dry-run` remains available for read-only connections because it does
not modify the target.

## Usage

```sh
openknowledge rules
openknowledge rules docs,changelog --path Wiki
openknowledge rules security --path Wiki
openknowledge rules changelog --path Wiki --target codex
openknowledge rules apply docs,changelog --path Wiki --file AGENTS.md
openknowledge rules apply changelog --path Wiki --yes
openknowledge rules apply docs --path Wiki --dry-run
openknowledge rules --list --path Wiki
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
| `--list` | Print an explainer plus the available built-in rules and any valid custom rules under the selected wiki's `rules/` directory. |
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
multiple rules with a comma-separated list. Built-in IDs are always available;
wiki-local custom IDs are loaded from OKF Markdown files under `rules/` when a
wiki path is provided.

| Rule name | Description |
| --- | --- |
| `project` | General project memory. Tells agents to read the wiki before non-trivial work, update durable project knowledge after meaningful changes, and keep the structure small and workflow-shaped. |
| `docs` | Documentation maintenance. Tells agents to update docs when behavior, APIs, commands, configs, or examples change, preserve source anchors, and separate shipped behavior from planned work. |
| `decisions` | Decision logging. Tells agents to record meaningful technical or product decisions with context, options, chosen path, tradeoffs, and links to affected concepts or source files. |
| `changelog` | Release and changelog memory. Tells agents to update changelog memory for user-facing behavior, flags, output, validation, publishing, packaging, or setup changes, while skipping formatting-only edits. |
| `research` | Research import. Tells agents to keep raw sources separate from synthesized wiki pages, preserve citations or source links, and avoid turning uncertain research into asserted project knowledge. |
| `bugs` | Debugging memory. Tells agents to capture reusable bug knowledge such as symptoms, reproduction, root cause, fix, tests, follow-up risks, and links to affected workflows or modules. |
| `schemas` | Contract documentation. Tells agents to document APIs, schemas, tables, config keys, data models, and contracts when authoritative source files or specs change. |
| `summary` | Recurring summaries. Tells agents to create dated summaries from reliable sources such as git history, issues, logs, or updated wiki pages, without claiming automations exist unless they were actually created. |
| `agents` | Agent entrypoints. Tells agents to create focused agent handoff docs only for repeated workflows, keep them short, link to deeper wiki concepts, and wire useful entrypoints through bundle metadata. |

## Custom Rule Documents

Custom rules live inside the selected wiki under `rules/` by default, or under
the directories configured with `[rules].paths` in `openknowledge.toml`. Each
rule is a normal OKF concept Markdown file with `type: Rule`, a canonical
`rule_id`, a summary from `rule_summary` or `description`, and at least one
instruction bullet, preferably under `## Instructions`:

```md
---
type: Rule
title: Security
description: Keep security-sensitive changes documented.
rule_id: security
rule_review_prompt: Check recent changes for auth, secrets, permissions, or data exposure changes.
rule_review_evidence: [git diff, Wiki/security/]
---

# Security

## Instructions

- When auth, permissions, secrets, or data exposure behavior changes, update security notes.
```

Custom IDs must use lowercase letters, numbers, and dashes, and start with a
letter. They cannot collide with built-in IDs or another custom rule. The
optional `rule_review_prompt` and `rule_review_evidence` fields are used by
`openknowledge review rules`, not by deterministic validation.

## Rule Configuration

`openknowledge.toml` can configure where custom rule Markdown lives and which
rules are selected by default:

```toml
[rules]
paths = ["rules", "policy-rules"]
enabled = ["docs", "changelog", "security"]
```

`rules.paths` defaults to `["rules"]`. Paths must be relative directories
inside the bundle. When explicitly configured, missing paths or non-directory
paths are deterministic `rule-catalog` validation errors.

`rules.enabled` is a comma-free TOML string array or single string of canonical
rule IDs. When present, it becomes the default selection for
`openknowledge rules`, `openknowledge rules apply`, and
`openknowledge review rules` unless the command passes an explicit rule list
or `--all`. Unknown IDs are reported through `rule-catalog` validation and make
rule rendering fail when used as defaults.

All sections share the strict typed
[`openknowledge.toml` configuration contract](/features/configuration.md).
Unknown fields or invalid types anywhere in the file are errors rather than
being ignored by the rules-specific reader.

## Behavior

Without a rules argument, the command prints `[rules].enabled` when configured,
otherwise the `project` rule. With a comma-separated rules list, only those
selected rules are included. Duplicate rules are deduplicated in the rendered
output. `--list --path <wiki>` includes valid custom rules from the configured
rule paths alongside the built-in catalog.

When a custom rule document or `[rules]` configuration is malformed, rule
rendering exits with usage status `2` and reports the catalog issue.
`openknowledge validate <wiki>` checks the same catalog structure through the
deterministic `rule-catalog` validation rule.

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
`--file`, `rules apply` searches the current directory and every ancestor for
all supported `AGENTS.md`, `CLAUDE.md`, `.cursor/rules/openknowledge.md`, and
`.cursor/rules/openknowledge.mdc` files. Results are ordered from the nearest
directory upward, with that filename order inside each directory. One result is
selected automatically; multiple results are shown before interactive
selection, while non-interactive use requires `--file` or `--yes`. With
`--yes`, the first result is selected, or `AGENTS.md` is created when none
exists. If the selected file already exists, interactive mode warns before it
appends a managed block or replaces an existing managed block. It shows the
generated block first, then prints the warning with the same highlighted
`⚠ Warning:` marker. The default answer is no; pass `--yes` to skip
confirmation.

## Example Output

`openknowledge rules docs,changelog --path Wiki --target codex` prints a
Markdown instruction block:

```text
## Open Knowledge Maintenance

This project has an Open Knowledge wiki at `Wiki`.

Add this block to the repository `AGENTS.md` file for Codex.

Before relevant work:
- Read `Wiki/index.md` and follow only links relevant to the task.
- Treat the wiki as durable project memory, not as a scratchpad.
- If the wiki is missing, stale, or wrong, say so instead of inventing facts.

Enabled rules:
- docs: Keep docs in sync with implementation.
- changelog: Track user-facing changes.
```

`openknowledge rules --list` prints an explainer plus the built-in rule IDs:

```text
Available rules:

  project        General project knowledge.
  docs           Keep docs in sync with implementation.
  decisions      Record important decisions.
  changelog      Track user-facing changes.
```

## Use Cases

* Add Open Knowledge maintenance guidance to an existing project `AGENTS.md`.
* Print Claude Code or Cursor project instructions without running setup.
* Give an agent a narrow wiki maintenance contract, such as docs plus changelog.
* Add repository-specific maintenance rules, such as security, release, or data
  boundary rules, without adding aliases to the built-in catalog.
* Update a repo instruction file idempotently with `openknowledge rules apply`.
* Inspect the built-in rule list with `openknowledge rules --list`, or include
  local custom rules with `openknowledge rules --list --path Wiki`.

## Command Change History

### 2026-07-07 - Custom rule catalog

Added wiki-local custom rule support through OKF Markdown files under
`rules/`. `openknowledge rules --list --path <wiki>` lists built-in rules plus
valid custom rules, and `openknowledge rules <id> --path <wiki>` can render
custom IDs. `openknowledge validate` now checks custom rule catalog structure
with `rule-catalog`.

Added `[rules]` configuration in `openknowledge.toml`. `rules.paths` selects
custom rule directories, and `rules.enabled` defines the default selected rule
IDs for `rules`, `rules apply`, and `review rules`.

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
> * `packages/cli/internal/okf/rule_catalog.go`
> * `packages/cli/internal/okf/rules_test.go`
> * `packages/cli/internal/okf/validation_checks.go`
> * `packages/cli/internal/okf/validation_policy.go`
> * `packages/cli/cmd/openknowledge/main_test.go`
> * `README.md`
>
> **Update notes**
>
> Update this page when the rule catalog, rules output contract, `--target`
> values, or setup interaction changes.
