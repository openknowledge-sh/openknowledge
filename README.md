# Open Knowledge CLI

Open Knowledge helps you create and maintain local LLM wikis that are human and agent readable. It keeps your knowledge base portable and consistent using the [Open Knowledge Format v0.1][okf-spec] specification.

It is built for people who want knowledge to stay readable in Git and easy for coding agents to navigate. A wiki can live inside a project repo or stand alone as a private knowledge base.

## Start with an agent

The fastest way to start is to paste this prompt into Codex, Cowork, Cursor, Claude, or another coding agent in the workspace where the wiki should live:

```text
Set up an Open Knowledge agentic wiki for this workspace.

First make sure the Open Knowledge CLI is installed. If it is not installed,
install it with:

curl -fsSL https://openknowledge.sh/install | bash

Then run:

openknowledge setup

Read the setup guide printed by that command and follow it. Ask me the setup
questions, create the knowledge base with openknowledge new, customize it for
this workspace, create useful workflows/skills/automation specs, run
openknowledge validate, and show me how to inspect it with openknowledge list
and openknowledge open.
```

The agent will install the CLI if needed, run the setup guide, ask where the wiki should live, create the scaffold, tailor it to your use case, and validate the result.

### CLI shortcut

If you are using your agents in CLI, (Claude Code, Codex) you can directly pass the generated setup prompt as the initial prompt:

```sh
codex "$(openknowledge setup)"
claude "$(openknowledge setup)"
```

## Manual setup

Manual setup is useful when you want to install the CLI yourself and keep control over the process.

Install with the shell installer:

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

Create and inspect a generic scaffold directly:

```sh
openknowledge new ./project-memory
openknowledge open ./project-memory
openknowledge list ./project-memory
openknowledge validate ./project-memory
```

## Why Open Knowledge

- **Portable by default**: knowledge lives in Markdown files with predictable
  names, frontmatter, indexes, and logs.
- **Agentic setup**: `codex "$(openknowledge setup)"` asks an agent to interview
  the user, create the scaffold, and configure the wiki for the chosen use case.
- **Workflow-ready**: new bundles include `AGENTS.md`, `SETUP.MD`, `workflows/`,
  `skills/`, `automations/`, and a pinned `SPEC.md` so agents know how to use
  and maintain the wiki.
- **Spec-backed**: validation targets an embedded Open Knowledge Format spec
  version, starting with OKF v0.1.

## How it works

`openknowledge setup` prints an agent prompt for setting up a useful local
knowledge base with the user. Pass it to Codex with command substitution so
Codex keeps an interactive terminal. The agent asks where the knowledge base
should live, creates it with `openknowledge new`, creates maintenance workflows
and local skill guidance, and customizes the scaffold for the chosen use case.

`openknowledge new` creates a local bundle with the base OKF structure, a setup
handoff, agent guidance, workflow and automation sections, an update log, and a
pinned copy of the current spec.

After that, humans and agents edit normal Markdown files. `openknowledge
open` starts a local viewer for reading the wiki, `openknowledge validate`
checks the bundle for portable OKF structure, and `openknowledge list` prints
the bundle tree with inline validation issues.


## Commands

| Command | Purpose |
| --- | --- |
| `openknowledge --help` | Print command usage, summaries, and examples. |
| `openknowledge setup` | Print an agent prompt for creating and customizing a knowledge base. |
| `openknowledge new [folder]` | Scaffold a local Open Knowledge bundle. |
| `openknowledge open [path]` | Start a local Markdown viewer for a knowledge base. |
| `openknowledge spec latest` | Print the latest embedded OKF spec. |
| `openknowledge spec 0.1` | Print a specific embedded spec version. |
| `openknowledge validate [path]` | Validate a bundle against the latest spec. |
| `openknowledge validate --spec 0.1 [path]` | Validate against a specific spec version. |
| `openknowledge list [path]` | Print a bundle tree with inline validation issues. |
| `openknowledge list --spec 0.1 [path]` | List while validating against a specific spec version. |
| `openknowledge list -json [path]` | Print machine-readable inventory output. |
| `openknowledge version` | Print the CLI version. |

## What validation checks

The validator enforces the OKF v0.1 rules that matter for a portable bundle:

- every non-reserved Markdown file has top-level YAML frontmatter
- every concept frontmatter has a non-empty `type`
- `index.md` and `log.md` are reserved files, not concept documents
- root `index.md` may declare `okf_version: "0.1"`
- `log.md` `##` headings use `YYYY-MM-DD`
- local Markdown links resolve inside the bundle, reported as warnings

It does not fail on optional fields, unknown concept types, unknown frontmatter
keys, broken local links, or missing index files.


## License and attribution

Open Knowledge is licensed under Apache-2.0.

The embedded OKF spec copy is Apache-2.0 material from
`GoogleCloudPlatform/knowledge-catalog`. See `THIRD_PARTY_NOTICES.md` and
`packages/cli/internal/okf/assets/specs/README.md` for attribution and license
handling.

[knowledge-catalog]: https://github.com/GoogleCloudPlatform/knowledge-catalog
[okf-spec]: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
