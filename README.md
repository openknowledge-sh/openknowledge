
  <img src="docs/assets/openknowledge-readme-logo.png" alt="Open Knowledge CLI" width="200">


Open Knowledge CLI helps you create local LLM wikis that are readable for both
humans and agents, then keep them up to date using a maintenance loop.

Implements the [Open Knowledge Format v0.1][okf-spec] specification.

## Start with an agent

The fastest way to start is to paste this prompt into Codex, Cowork, Cursor, Claude, or another coding agent in the workspace where the wiki should live:

```text
Set up an Open Knowledge agentic wiki for this workspace.

First check whether the openknowledge CLI is available with command -v openknowledge and openknowledge --help. If it is missing, install it with curl -fsSL https://openknowledge.sh/install | bash. Then run openknowledge setup, ask me the setup questions, create and customize the wiki for this workspace, run openknowledge validate, and show me how to inspect it with openknowledge open.
```

The agent will install the CLI if needed, run setup, ask where the wiki should live, create the scaffold, tailor it to your use case, and validate the result.

### CLI shortcut

If you use agent CLIs such as Claude Code or Codex, you can pass the generated
setup prompt directly as the initial prompt:

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

## What Open Knowledge CLI gives you

An agentic wiki that lives inside a project repo or stand alone as your private
knowledge base. With skills and workflows, to help your agents maintain it.

- Turn a project, research folder, or private knowledge dump into a wiki that
  agents can use effectively.
- Guided setup through an agent interview, so the wiki starts with the right
  purpose, structure, and maintenance habits and rules.
- An agentic maintenance loop so wiki stays up to date.
- Local markdown viewer to inspect the wiki.
- Consistency against the [Open Knowledge Format v0.1][okf-spec]
  specification.

## How it works

`openknowledge setup` prints an agent prompt for setting up a useful local
knowledge base with the user. Paste it into a coding agent, or pass it as an
initial CLI prompt when your agent CLI supports that pattern. The agent asks
where the knowledge base should live, creates it with `openknowledge new`,
creates maintenance workflows and local skill guidance, and customizes the
scaffold for the chosen use case.

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
| `openknowledge <command> --help` | Print command-specific usage, flags, and examples. |
| `openknowledge setup` | Print an agent prompt for creating and customizing a knowledge base. |
| `openknowledge new [folder]` | Scaffold a local Open Knowledge bundle. |
| `openknowledge open [path]` | Start a local Markdown viewer for a knowledge base. |
| `openknowledge spec latest` | Print the latest embedded OKF spec. |
| `openknowledge spec 0.1` | Print a specific embedded spec version. |
| `openknowledge validate [path]` | Validate a bundle against the latest spec. |
| `openknowledge validate --spec 0.1 [path]` | Validate against a specific spec version. |
| `openknowledge list [path]` | Print a bundle tree with inline validation issues. |
| `openknowledge list --spec 0.1 [path]` | List while validating against a specific spec version. |
| `openknowledge list --json [path]` | Print machine-readable inventory output. |
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
