---
type: Candidate Command Documentation
title: openknowledge connect
description: Candidate specification for connecting local and remote OKF bundles into the user's local knowledge registry.
tags: [openknowledge, cli, command, registry, connect, agent, candidate]
timestamp: 2026-06-19T00:00:00Z
status: candidate
---

# `openknowledge connect`

`openknowledge connect` is the candidate user-facing command for adding any
local or remote Open Knowledge Format bundle to the user's local knowledge
registry. It should replace `openknowledge registry add` in normal workflows.

The command is intentionally optimistic. OKF v0.1 defines a bundle as a
directory tree of Markdown files, with `index.md` and `okf_version` both
optional. `connect` should accept any readable directory that can be consumed as
an OKF bundle, then use validation results and optional Open Knowledge CLI
metadata as quality signals instead of hard gates.

## Candidate Usage

```sh
openknowledge connect <path-or-url>
openknowledge connect <path-or-url> --as <key>
openknowledge connect <path-or-url> --access read|write
openknowledge connect <path-or-url> --no-validate
openknowledge connect --help
```

## Purpose

`connect` gives agents a stable local knowledge registry:

* every connected bundle has an absolute filesystem path that can be used from
  any working directory or agent window;
* every connected bundle has a short key for commands such as
  `openknowledge where`, `openknowledge list`, and candidate
  `openknowledge use`;
* optional `okf_bundle_*` metadata adds agent entrypoints and discovery hints
  without changing OKF conformance;
* generic agent skills can discover knowledge bases with `openknowledge list`,
  inspect entrypoints with `openknowledge use <key> --info`, and load an
  entrypoint with `openknowledge use <key> [entry]`.

This makes Open Knowledge a lightweight context manager for specialized local
knowledge packages such as accessibility review, React guidance, Angular
guidance, or code review checklists.

## Inputs

`path-or-url` may be:

* a local directory;
* a local path that expands from `~`;
* a public GitHub repository URL;
* a future archive URL or other remote bundle source.

For local directories, `connect` stores the canonical absolute path and does not
copy files. For remote sources, `connect` should materialize the bundle into a
managed local cache such as `~/.openknowledge/bundles/<key>` and store that
absolute path.

## Metadata Parsing

`connect` reads root `index.md` when present. If the file has root frontmatter,
the parser should read these optional flat keys:

| Key | Meaning |
| --- | --- |
| `okf_version` | Declared OKF version. Optional and not required for connection. |
| `okf_bundle_name` | Preferred stable key when `--as` is not provided. |
| `okf_bundle_title` | Human-facing display name. |
| `okf_bundle_purpose` | Short reason the bundle exists. |
| `okf_bundle_tags` | Routing and discovery tags. |
| `okf_bundle_entry_<name>` | Agent entrypoint path used by candidate `openknowledge use`. |

The parser should stay lightweight: flat scalar and flow-list values are enough
for this metadata layer. Unknown root frontmatter keys should be ignored for
connection.

Entrypoint documents are normal OKF concept documents. Their own frontmatter
may include `title`, `description`, `tags`, and producer-defined `use_when`.
`connect` can summarize these in its success output, but entrypoint bodies are
loaded by `openknowledge use`, not by `connect`.

## Fallbacks

Missing metadata must not prevent connection:

* Without `okf_bundle_name`, derive the key from `--as`, then the repository or
  folder basename.
* Without `okf_bundle_title`, derive the display name from the root `index.md`
  H1, then the folder basename.
* Without `okf_bundle_purpose`, leave purpose empty.
* Without `okf_bundle_tags`, use no tags.
* Without `okf_bundle_entry_default`, candidate `openknowledge use <key>`
  falls back to printing root `index.md`.
* Without root `index.md`, connection can still succeed and later commands can
  fall back to bundle path, key, and validation/list output.

## Key And Path Rules

The registry's canonical identity should be the normalized absolute path. The
short key is a command shortcut, not the source of truth.

Rules:

* Connecting the same absolute path again updates or no-ops the existing
  connection; it does not create a duplicate.
* If an implicit key collides with an existing different path, choose the next
  available suffix such as `accessibility-2`, then warn the user.
* If explicit `--as <key>` collides with an existing different path, fail and
  print the path already using that key.
* Key validation should match current registry-name validation: letters,
  numbers, dots, underscores, and dashes; it must not look like a path.
* `openknowledge where <key>` should always print the absolute path for the
  selected connection.

## Storage Model

The stored registry should be path-keyed. Bundle metadata remains in the bundle
content and should be read fresh when needed.

```json
{
  "connections": {
    "/Users/me/.openknowledge/bundles/accessibility": {
      "key": "accessibility",
      "name": "Accessibility Review",
      "access": "read",
      "source": {
        "type": "github",
        "url": "https://github.com/openknowledge-sh/accessibility",
        "ref": "main"
      },
      "managed": true
    }
  }
}
```

Fields:

| Field | Meaning |
| --- | --- |
| `key` | User-facing shortcut for commands. |
| `name` | Display name derived from metadata, root H1, or folder basename. |
| `access` | Local permission decision, `read` or `write`; remote bundles do not grant themselves write access. |
| `source` | Optional source metadata for sync and provenance. |
| `managed` | Whether files live in Open Knowledge's managed cache and may be deleted by `disconnect --delete-files`. |

## Validation

`connect` should run validation by default and print the result as status:

* `valid`: no validation errors;
* `warnings`: validation warnings only;
* `invalid`: validation errors, but connection can still be allowed when the
  bundle is readable and the user confirms or passes a future force flag;
* `unknown`: validation skipped or unavailable.

Validation is not the same as connection eligibility. The goal is to preserve
OKF's best-effort consumption model while making quality visible.

## Success Output

For a bundle with metadata:

```text
Connected knowledge bundle
key      accessibility
name     Accessibility Review
path     /Users/me/.openknowledge/bundles/accessibility
access   read
status   valid
purpose  Accessibility review guidance for UI, HTML, ARIA...
entries  default, review
```

For a plain OKF bundle without metadata:

```text
Connected knowledge bundle
key      project-memory
name     Project Memory
path     /Users/me/project-memory
access   read
status   valid
metadata none
```

## Agent Skill Distribution

A future bundle MAY include a `.skills/` folder for agent-skill packages that
help specific runtimes call `openknowledge use`, `openknowledge where`, or
other registry commands. This is a candidate extension and is not part of the
initial `connect` contract.

Open questions before shipping `.skills/` support:

* whether `.skills/` is excluded from OKF validation/export or must wrap skill
  content in OKF-valid documents;
* how Codex, OpenAI, Claude Code, and other runtimes define skill manifests;
* whether there is a portable intersection format;
* whether `connect` should only list available skills or also offer explicit
  installation.

`connect` must not auto-install runtime skills without explicit user approval.

## Update Notes

When implemented, update root help, `README.md`, `docs/cli.md`,
[disconnect](disconnect.md), [list](list.md), [where](where.md),
[registry](registry.md), [use](use.md), tests, and
[CLI changelog](/changelog/cli.md).
