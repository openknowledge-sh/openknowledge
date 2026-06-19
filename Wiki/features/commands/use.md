---
type: Candidate Command Documentation
title: openknowledge use
description: Candidate design for printing agent entrypoints from connected knowledge bundles.
tags: [openknowledge, cli, command, registry, agent, candidate]
timestamp: 2026-06-19T00:00:00Z
status: candidate
---

# `openknowledge use`

`openknowledge use` is a candidate command for printing an agent-facing
entrypoint from a local or connected Open Knowledge bundle. It is not part of
the shipped command surface yet.

The command adds an Open Knowledge CLI metadata layer on top of OKF. Any
OKF-valid bundle must remain usable without this layer. Bundle-level metadata
helps discovery and entrypoint routing, but it does not define OKF conformance.

`use` is the agent-consumption half of the connected bundle model. A generic
agent skill can discover bundles with [openknowledge list](list.md), inspect a
bundle with `openknowledge use <key> --info`, and then load the right entrypoint
with `openknowledge use <key> [entry]`.

## Candidate Usage

```sh
openknowledge use <name-or-path>
openknowledge use <name-or-path> <entry>
openknowledge use <name-or-path> --info
openknowledge use <name-or-path> <entry> --info
```

## Bundle Metadata Layer

Bundle metadata lives in the bundle-root `index.md` frontmatter as flat
`okf_bundle_*` keys. Nested `index.md` files should keep the normal OKF index
shape and should not use frontmatter.

```md
---
okf_version: "0.1"
okf_bundle_name: accessibility
okf_bundle_title: Accessibility Review
okf_bundle_purpose: Accessibility review guidance for UI, HTML, ARIA, keyboard navigation, and design systems.
okf_bundle_tags: [accessibility, ui, review]
okf_bundle_entry_default: agents/accessibility-checker.md
okf_bundle_entry_review: agents/accessibility-review.md
okf_bundle_entry_authoring: agents/accessibility-authoring.md
---

# Accessibility Review
```

The root metadata intentionally maps entrypoint names to Markdown files only.
Entrypoint-specific routing metadata belongs in the target page's own
frontmatter.

## Entrypoint Documents

Entrypoints are normal OKF concept documents. They should include a non-empty
`type`, ordinary display metadata, and a producer-defined `use_when` field that
explains when an agent should use that entrypoint.

```md
---
type: Agent Entrypoint
title: Accessibility Checker
description: Run an accessibility checker over UI code.
tags: [accessibility, checker, ui]
use_when: [reviewing UI accessibility, building forms and dialogs, checking keyboard navigation]
---

# Accessibility Checker

Read the relevant accessibility guidance, then inspect the target UI...
```

## Candidate Behavior

`openknowledge use accessibility` resolves `accessibility` as a registry entry
or path, reads the root `index.md`, and prints the file referenced by
`okf_bundle_entry_default`.

`openknowledge use accessibility review` prints the file referenced by
`okf_bundle_entry_review`.

When `--info` is present, the command prints bundle metadata and entrypoint
metadata instead of printing the entrypoint body. For each entrypoint, it should
resolve the target file and read its frontmatter, including `title`,
`description`, `tags`, and `use_when`.

The command prints Markdown content exactly enough for an agent prompt. It
should not recursively inline every linked page. Entrypoint documents should
tell the agent which pages to read next.

## Compatibility And Fallbacks

The metadata layer is optional. A bundle whose root `index.md` only declares
`okf_version: "0.1"` is still a valid Open Knowledge bundle and must still be
loadable by `connect`, `registry`, `open`, and `use`.

Fallback rules:

* Without `okf_bundle_name`, use `--as` from `connect`, then the repository or
  folder name.
* Without `okf_bundle_title`, use the root `index.md` H1, then the bundle name.
* Without `okf_bundle_purpose`, leave the purpose blank or show the first root
  index paragraph as display summary.
* Without `okf_bundle_entry_default`, `openknowledge use <bundle>` prints the
  root `index.md`.
* If a named entry does not exist, print the available entrypoint names and
  exit with a usage error.
* If an entrypoint path is declared but the file is missing, validation should
  report it and `use` should fail with a clear missing-entrypoint error.

## Example Journey

```sh
openknowledge connect https://github.com/openknowledge-sh/accessibility
openknowledge use accessibility --info
openknowledge use accessibility
openknowledge use accessibility review
```

`connect` should read the same root metadata when present, but it should not
require it. The user's local registry remains responsible for local source,
path, and access decisions such as read-only versus writable local knowledge.

`openknowledge new` can seed this optional metadata with flags such as
`--bundle-name`, `--bundle-purpose`, `--bundle-tag`, and
`--bundle-entry default=agents/checker.md`. The metadata remains optional:
plain OKF bundles without `okf_bundle_*` keys still load and fall back to the
root `index.md`.

## Agent Flow

A runtime skill for accessibility review could be as small as:

```text
When asked to review accessibility, run:
  openknowledge use accessibility

Then follow the printed entrypoint instructions.
```

The skill does not need to know where the bundle lives. `use` resolves the
local connection key through the user's registry and prints the bundle's own
agent entrypoint. This keeps specialized knowledge packages local, inspectable,
and swappable without rewriting every agent workflow.

## Skill Distribution Candidate

Bundles may later include a `.skills/` folder with runtime-specific skill
packages that teach agents when to call `openknowledge use <key>`. This is not
part of the initial `use` contract. Before shipping it, the project should
research Codex, OpenAI, Claude Code, and other skill manifest formats and
decide whether `.skills/` is excluded from OKF validation/export or represented
through OKF-valid wrapper documents.

## Implementation Notes

This candidate likely requires:

* Root index validation that tolerates optional `okf_bundle_*` keys while still
  accepting root indexes with only `okf_version`.
* A frontmatter reader that can parse flat string and list values used by
  `okf_bundle_tags`, entrypoint maps, and `use_when`.
* Registry and connect behavior that stores local path, source, and access
  while treating bundle metadata as content read from the bundle.

## Update Notes

When this command is implemented, update root help, `README.md`,
[registry documentation](registry.md), tests, and [CLI changelog](/changelog/cli.md).
