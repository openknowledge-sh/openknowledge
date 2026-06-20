
  <img src="docs/assets/openknowledge-readme-logo.png" alt="Open Knowledge CLI" width="140">


Open Knowledge CLI helps you create local LLM wikis that are readable for both
humans and agents, then keep them up to date using a maintenance loop.

Implements the [Open Knowledge Format v0.1][okf-spec] specification.

## Start with an agent

The fastest way to start is to paste this prompt into Codex, Cowork, Cursor, Claude, or another coding agent in the workspace where the wiki should live:

```text
Set up an Open Knowledge agentic wiki for this workspace.

First check whether the openknowledge CLI is available with command -v openknowledge and openknowledge --help. If it is missing, install it with curl -fsSL https://openknowledge.sh/install | bash. Then run openknowledge setup, inspect this workspace and any relevant memories, ask only the setup questions still needed, create and customize the wiki for this workspace, run openknowledge validate, and show me how to inspect it with openknowledge open.
```

The agent will install the CLI if needed, run setup, inspect local context and relevant memories, ask only for missing decisions, create the scaffold, tailor it to your use case, and validate the result.

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

Create and inspect a minimal scaffold directly:

```sh
openknowledge new ./project-memory
openknowledge new --name "Accessibility Review" --bundle-name accessibility --bundle-tag accessibility ./accessibility
openknowledge connect ./project-memory --as personal
openknowledge connect ./accessibility
openknowledge use personal --info
openknowledge use personal
openknowledge where personal
openknowledge open
openknowledge open ./project-memory
openknowledge list ./project-memory
openknowledge list personal
openknowledge validate ./project-memory
openknowledge to html --out ./project-site ./project-memory
openknowledge to html --plain --out ./project-plain-site ./project-memory
openknowledge to json ./project-memory
openknowledge disconnect personal
```

## What Open Knowledge CLI gives you

An agentic wiki that lives inside a project repo or stand alone as your private
knowledge base. With workflows and agent instructions to help your agents maintain it.

- Turn a project, research folder, or private knowledge dump into a wiki that
  agents can use effectively.
- Guided setup through an agent interview, so the wiki starts with the right
  purpose, structure, and maintenance habits and rules.
- An agentic maintenance loop so wiki stays up to date.
- Local markdown viewer with full-text search to inspect the wiki.
- Consistency against the [Open Knowledge Format v0.1][okf-spec]
  specification.

## How it works

`openknowledge setup` prints an agent prompt for setting up a useful local
knowledge base with the user. Paste it into a coding agent, or pass it as an
initial CLI prompt when your agent CLI supports that pattern. The agent first
inspects the workspace and any relevant user or project memories available in
its runtime, asks only the missing setup questions, creates the bundle with
`openknowledge new`, then creates the folders, workflows, agent instructions,
native automations when supported, and seed pages that fit the chosen use case.
When setup creates repo-scoped or user-scoped skills, the prompt tells the
agent to include guidance for focused lower-reasoning subagents on bounded wiki
maintenance tasks when the runtime supports that.

`openknowledge new` creates a minimal local bundle with the base OKF files: a
setup handoff, starter agent guidance, an update log, and a pinned copy of the
current spec. Optional `--bundle-*` flags can seed `okf_bundle_*` metadata in
the root index for discovery and future agent entrypoints. The use-case
structure is intentionally left to setup.

After that, humans and agents edit normal Markdown files. `openknowledge open`
starts a registry-backed local viewer with a workspace selector, and
`openknowledge open <path-or-name>` opens one knowledge base directly.
`openknowledge validate` checks the bundle for portable OKF structure, and
`openknowledge list` prints the bundle tree with inline validation issues.
`openknowledge to html` writes the same static viewer app bundle by default,
`openknowledge to html --plain` writes unstyled semantic HTML, and
`openknowledge to json` writes a normalized bundle model for tools and agents.
The default HTML viewer export can inherit your site styling from an optional
`openknowledge.toml` in the bundle root:

```toml
[html.theme]
name = "landing"
stylesheet = "assets/wiki-theme.css"
```

The stylesheet is copied into the static export and linked from every generated
viewer page. Override the documented `--ok-*` variables to match your landing
page. The canonical default theme lives at
`packages/cli/cmd/openknowledge/viewer_theme.css`; the local viewer and default
HTML export derive their colors, fonts, and viewer dimensions from that theme
layer.

`openknowledge connect` stores named local paths for shared or standalone
knowledge bases. A key is only an alias: path-based commands still work, and
agents can use `openknowledge where <key>` to get the real folder before using
normal filesystem tools such as `rg`. Agents can use `openknowledge use <key>`
to print a bundle-declared entrypoint, falling back to root `index.md` when no
default entrypoint is declared. `openknowledge disconnect` removes a connection
without deleting local files by default. `openknowledge registry` remains the
low-level compatibility command for listing and adding named paths directly.

The local viewer opens the printed `127.0.0.1` view URL in your default
browser. It serves registered knowledge bases under stable paths such as
`/personal/`; those path aliases do not require local DNS or `/etc/hosts`
changes.

## Commands

| Command | Purpose |
| --- | --- |
| `openknowledge --help` | Print command usage, summaries, and examples. |
| `openknowledge <command> --help` | Print command-specific usage, flags, and examples. |
| `openknowledge setup` | Print an agent prompt for creating and customizing a knowledge base. |
| `openknowledge new [folder]` | Scaffold a local Open Knowledge bundle. |
| `openknowledge new --bundle-name <id> [folder]` | Scaffold with optional bundle metadata. |
| `openknowledge connect <path>` | Connect a local knowledge bundle to the user registry. |
| `openknowledge connect <path> --as <key>` | Connect a local bundle with an explicit key. |
| `openknowledge connect <path> --access read\|write` | Store an access label with a connection. |
| `openknowledge disconnect <key-or-path>` | Remove a connection while keeping files. |
| `openknowledge disconnect <key-or-path> --delete-files` | Delete files only for managed cached entries. |
| `openknowledge use <name-or-path>` | Print a default agent entrypoint or root `index.md`. |
| `openknowledge use <name-or-path> <entry>` | Print a named bundle entrypoint. |
| `openknowledge use <name-or-path> --info` | Print bundle and entrypoint metadata. |
| `openknowledge registry list` | List named local knowledge base paths. |
| `openknowledge registry add <name> <path>` | Register a name for a knowledge base folder. |
| `openknowledge where <name-or-path>` | Print the absolute path for a registry name or path. |
| `openknowledge open [path]` | Start the registry or knowledge base Markdown viewer. |
| `openknowledge open --name <alias-name> [path]` | Start a direct viewer with a stable local alias path. |
| `openknowledge to html --out <folder> [path]` | Write a static viewer app bundle. |
| `openknowledge to html --plain --out <folder> [path]` | Write unstyled semantic HTML files. |
| `openknowledge to json [path]` | Print normalized bundle JSON. |
| `openknowledge to json --out <file> [path]` | Write normalized bundle JSON to a file. |
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
- YAML frontmatter parses cleanly; non-blocking formatting issues are warnings
- Markdown bodies avoid malformed links, code spans, tables, and fences
- `index.md` and `log.md` are reserved files, not concept documents
- root `index.md` may declare `okf_version: "0.1"` and optional Open Knowledge
  CLI `okf_bundle_*` metadata
- any `index.md` may declare `okf_publish: false` for public-view exclusion
- `log.md` `##` headings use `YYYY-MM-DD`
- local Markdown links resolve inside the bundle, reported as warnings

It does not fail on optional fields, unknown concept types, unknown frontmatter
keys, broken local links, non-blocking Markdown syntax warnings, or missing
index files.


## License and attribution

Open Knowledge is licensed under Apache-2.0.

The embedded OKF spec copy is Apache-2.0 material from
`GoogleCloudPlatform/knowledge-catalog`. See `THIRD_PARTY_NOTICES.md` and
`packages/cli/internal/okf/assets/specs/README.md` for attribution and license
handling.

[knowledge-catalog]: https://github.com/GoogleCloudPlatform/knowledge-catalog
[okf-spec]: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
