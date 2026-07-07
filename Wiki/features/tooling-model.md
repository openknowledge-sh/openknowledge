---
type: Feature Documentation
title: Tooling Model
description: Product-level map of the Open Knowledge CLI tooling layers.
tags: [openknowledge, cli, tooling, registry, viewer, export]
timestamp: 2026-06-20T00:00:00Z
---

# Tooling Model

Open Knowledge CLI is best understood as a set of layers around local Markdown
knowledge bases. The layers share the same bundle format, but each one answers
a different user or agent need.

## Layers

| Layer | Shipped commands | Purpose |
| --- | --- | --- |
| Source-to-wiki generation | `from` | Print an agent task prompt that turns a source URL or path into an OKF Markdown bundle. |
| Authoring and OKF hygiene | `setup`, `new`, `spec` | Create a bundle and keep Markdown shaped around OKF v0.1. |
| Connection and bundle lifecycle | `connect`, `disconnect`, `registry connect`, `registry disconnect`, `registry list`, `registry where`, `to tar` | Give local, published, archive, or Git bundles stable names, materialize remote sources, resolve names back to filesystem paths, and package portable source archives. |
| Validation and inspection | `validate`, `list`, `rules`, `review` | Check OKF structure, link health, bundle inventory, maintenance rules, and depth-limited tree views before humans or agents rely on the knowledge. |
| Use and navigation | `get`, `search`, `list`, `view` | Read exact Markdown or known entrypoints, inspect structure, search source-grounded chunks, follow graph-expanded context, and browse connected or direct bundles. |
| OKF views and publishing | `ast`, `to json`, `to graph`, `to graph --type search`, `to html`, `to html --plain` | View the same OKF bundle as parsed AST, normalized JSON, source graph, search graph, static viewer, or plain semantic HTML. |

## Current Boundaries

`connect` and the registry store aliases for existing bundle folders. They also
materialize Open Knowledge manifests, tar archives, and Git remote sources into
the Open Knowledge cache before registration. After registration,
`registry where`, `get`, `search`, `view`, `validate`, `list`, and `to` work
through the same key-or-path resolution model for local folders and remote
sources.

Default HTML viewer exports publish a portable bundle archive at
`assets/openknowledge-bundle.tar.gz` and an `openknowledge.json` manifest that
points to it. A deployed static wiki can therefore be connected by URL without
requiring Git access.

`ast`, `to json`, and `to graph` expose different views of the Open Knowledge
Format without changing the authored Markdown. `ast` shows parsed syntax,
frontmatter, sections, and links. `to json` shows the normalized bundle model.
`to graph` exports AST-backed source graph JSON, and `to graph --type search`
emits a derivative chunk graph with file containment, chunk reading order, and
chunk-level local links for retrieval tooling.

`validate` remains deterministic, while `review rules` prints advisory AI
review prompts for checking whether selected maintenance rules appear to have
been followed.

## Source-To-Wiki Generation

`openknowledge from` is the source-to-wiki layer. Its simple model is
source URL or path, local agent task, then OKF Markdown bundle. The command can
print a prompt for Codex, Claude Code, Cursor, Cowork, or another
filesystem-capable agent to inspect the source, generate the wiki, validate it,
and hand back navigation commands.

Usage:

```sh
openknowledge from https://github.com/openknowledge-sh/openknowledge --out Wiki --type understanding
openknowledge from https://github.com/openknowledge-sh/openknowledge --out Wiki --type custom
openknowledge from https://openknowledge.sh/wiki/ --out Wiki --type understanding --depth 2
```

The `--type` flag selects a generation recipe such as `understanding` or
`custom`. See [openknowledge from](commands/from.md) for command behavior.

## Agent Flow

The intended agent loop is path-light:

```sh
openknowledge connect ./accessibility --as accessibility
openknowledge list accessibility
openknowledge list --depth 2 accessibility
openknowledge get accessibility --info
openknowledge get accessibility
openknowledge search accessibility "validation workflow"
openknowledge search accessibility "validation workflow" --expand graph
openknowledge view accessibility
```

The agent can read `get` output as its task-specific entrypoint. When it needs
focused source snippets, it can call `search` for ranked heading chunks and
graph-expanded neighbors. When it needs structure before choosing files, it can
call `list --depth`. When it needs raw filesystem access, it can resolve the
bundle with:

```sh
openknowledge registry where accessibility
```

Then it can use normal filesystem tools such as `rg`.

## Publishing Flow

The published documentation site at
[https://openknowledge.sh/wiki/](https://openknowledge.sh/wiki/) is this
repository's `Wiki/` bundle exported with:

```sh
openknowledge to html --out packages/web/dist/wiki Wiki
```

The same publishing layer can export another bundle as the default static
viewer, plain semantic HTML, normalized JSON, or a tar archive.

## Related Docs

* [Commands](commands/) - command-by-command behavior and flags.
* [Exporters](exporters/) - shipped and candidate export targets.
* [Registry command](commands/registry.md) - connection storage and lookup.
* [List command](commands/list.md) - bundle inventory and validation context.
* [From command](commands/from.md) - agent-driven source-to-wiki generation.
* [Get command](commands/get.md) - exact Markdown and entrypoint retrieval.
* [Search command](commands/search.md) - section-level search and graph-expanded retrieval.
* [View command](commands/view.md) - local Markdown viewer behavior.
* [Graph exporter](exporters/graph.md) - source and search graph views.

---

<!-- okf-footer: agent-maintenance -->

> **Update notes**
>
> Update this page when a command moves between candidate and shipped status, a
> new layer is added, remote source behavior changes, or README and index
> navigation need a new product-level explanation.
