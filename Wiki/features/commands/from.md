---
type: Command Documentation
title: openknowledge from
description: Prints an agent prompt for turning a source into an OKF bundle.
tags: [openknowledge, cli, command, generation, agents]
timestamp: 2026-07-07T00:00:00Z
status: shipped
---

# `openknowledge from`

`openknowledge from` prints an agent task prompt for turning a source into an
Open Knowledge wiki.

The simple model is:

```text
source URL or path -> local agent task -> OKF Markdown bundle
```

Like `openknowledge setup`, this command does not do the source reading or file
writing itself. It prints a prompt for a local agent. The agent reads the
source, asks only missing questions, writes the wiki, validates it, and gives
the user the next commands. This should work in Codex, Claude Code, Cursor,
Cowork, or any other agent app or CLI that can access the source and write
files.

## Usage

```sh
openknowledge from https://github.com/owner/repo --out Wiki
openknowledge from https://github.com/owner/repo --out Wiki --type understanding
openknowledge from https://github.com/owner/repo --out Wiki --type custom
openknowledge from https://github.com/owner/repo --out Wiki --type custom --about "Help new contributors understand the plugin system and release workflow."
openknowledge from https://example.com/docs --out Wiki --type understanding --depth 2
openknowledge from ./local-repo --out Wiki --type understanding
openknowledge from --help
```

For agent CLIs that accept an initial prompt:

```sh
codex "$(openknowledge from https://github.com/owner/repo --out Wiki --type custom)"
claude "$(openknowledge from https://github.com/owner/repo --out Wiki --type custom)"
```

Interactive agents need stdin to remain a terminal. Pipes are only appropriate
for agent CLIs that explicitly accept prompts from stdin.

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `source` | argument | Source URL or local path. Examples include GitHub repositories, generic Git repositories, local paths, and websites. |
| `--out <path>` | flag | Target Open Knowledge bundle folder. |
| `--type <type>` | flag | Generation profile. Values are `understanding` and `custom`; defaults to `understanding`. |
| `--about <text>` | flag | Non-interactive goal for `--type custom`. |
| `--depth <count>` | flag | Website crawl depth or repository traversal depth where a source adapter supports it. |
| `--help` | flag | Print command-specific help. |

The CLI-facing flag is `--type`, but generated bundle metadata should store the
value in a namespaced key such as `okf_wiki_type` so it does not conflict with
concept-document `type` frontmatter.

## What The Agent Does

The generated task should be concrete and short:

1. Inspect the source before writing.
2. Ask the user only for missing intent or scope.
3. Create or update the OKF bundle at `--out`.
4. Keep raw copied material separate from synthesized pages.
5. Preserve source links, source files, line ranges, commit IDs, or canonical
   page URLs where available.
6. Run `openknowledge validate "<out>"`.
7. Finish with the useful next commands: `openknowledge list`,
   `openknowledge search`, `openknowledge get`, and `openknowledge view`.

Repository sources should keep commit provenance and source-file anchors.
Website sources should keep canonical URLs, crawl depth, source titles, and
fetch timestamps. The result should always be ordinary OKF Markdown so the
rest of the CLI works without a generation runtime.

## Generation Types

Think of `--type` as a generation recipe.

| Type | Purpose |
| --- | --- |
| `understanding` | Default DeepWiki-style wiki for understanding a repo or site: overview, architecture, structure, workflows, entrypoints, diagrams when useful, glossary, and citations. |
| `custom` | Ask what the wiki should help with, who it is for, what to focus on, and how deep to go. `--about` supplies that goal without an interview. |

`custom` should compose the same underlying generation rules as other types,
then store the chosen goal and rules so future refreshes keep the same intent.

## Stored Metadata

The generated prompt asks the agent to write root metadata like this when it is
useful:

```yaml
---
okf_version: "0.1"
okf_bundle_name: "owner-repo"
okf_bundle_title: "owner/repo Understanding"
okf_wiki_type: understanding
okf_generation_goal: "Help new contributors understand the plugin system."
okf_generation_rules: [overview, architecture, workflows, glossary, citations]
okf_generated_from:
  kind: github
  url: https://github.com/owner/repo
  branch: main
  commit: abc123
  generated_at: 2026-07-07T12:00:00Z
  generator: openknowledge from
---
```

Generated concept pages still use normal OKF `type` values such as
`Repository Overview`, `Architecture Overview`, `Module`,
`Development Workflow`, `API Reference`, or `Glossary`.

## Refresh Behavior

If the output bundle already has `okf_generated_from` metadata, `from` should
prefer an update flow over a full rewrite.

For repositories, compare the recorded commit with the current commit, inspect
changed files, and update only affected pages where practical. For websites,
compare known URLs and page content where available.

Refreshes should preserve human edits when possible. Generated pages should
make their provenance explicit enough that an agent can distinguish generated
sections from later human-maintained notes.

## Command Change History

### 2026-07-07

`openknowledge from` shipped as a prompt-producing source-to-wiki command. It
accepts one source argument, requires `--out`, defaults `--type` to
`understanding`, supports `--type custom`, accepts `--about` for non-interactive
custom goals, and accepts `--depth` as a crawl or traversal hint.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/from.go`
> * `packages/cli/cmd/openknowledge/main_test.go`
> * `packages/cli/internal/okf/from_test.go`
>
> **Update notes**
>
> Update this page when `from` flags, supported generation types, prompt
> behavior, or provenance guidance change. CLI behavior changes also require
> [CLI changelog](/changelog/cli.md) updates.
