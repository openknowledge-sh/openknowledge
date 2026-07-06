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
| Authoring and OKF hygiene | `setup`, `rules`, `new`, `spec` | Create a bundle, seed maintenance instructions, and keep Markdown shaped around OKF v0.1. |
| Connection and bundle lifecycle | `connect`, `disconnect`, `registry connect`, `registry disconnect`, `registry list`, `registry where`, `to tar` | Give local, published, archive, or Git bundles stable names, materialize remote sources, resolve names back to filesystem paths, and package portable source archives. |
| Validation and inspection | `validate`, `list` | Check OKF structure, link health, and bundle inventory before humans or agents rely on the knowledge. |
| Use and navigation | `use`, `search`, `open` | Load known entrypoints, search source-grounded chunks, follow graph-expanded context, and browse connected or direct bundles. |
| OKF views and publishing | `ast`, `to json`, `to graph`, `to graph --type search`, `to html`, `to html --plain` | View the same OKF bundle as parsed AST, normalized JSON, source graph, search graph, static viewer, or plain semantic HTML. |

## Current Boundaries

`connect` and the registry store aliases for existing bundle folders. They also
materialize Open Knowledge manifests, tar archives, and Git remote sources into
the Open Knowledge cache before registration. After registration,
`registry where`, `use`, `search`, `open`, `validate`, `list`, and `to` work
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

## Agent Flow

The intended agent loop is path-light:

```sh
openknowledge connect ./accessibility --as accessibility
openknowledge list accessibility
openknowledge use accessibility --info
openknowledge use accessibility
openknowledge search accessibility "validation workflow"
openknowledge search accessibility "validation workflow" --expand graph
openknowledge open accessibility
```

The agent can read `use` output as its task-specific entrypoint. When it needs
focused source snippets, it can call `search` for ranked heading chunks and
graph-expanded neighbors. When it needs raw files, it can resolve the bundle
with:

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
* [Use command](commands/use.md) - agent entrypoint selection.
* [Search command](commands/search.md) - section-level search and graph-expanded retrieval.
* [Open command](commands/open.md) - local Markdown viewer behavior.
* [Graph exporter](exporters/graph.md) - source and search graph views.

---

<!-- okf-footer: agent-maintenance -->

> **Update notes**
>
> Update this page when a command moves between candidate and shipped status, a
> new layer is added, remote source behavior changes, or README and index
> navigation need a new product-level explanation.
