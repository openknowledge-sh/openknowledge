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
| Authoring and OKF hygiene | `setup`, `new`, `validate`, `list`, `spec` | Create a bundle, seed maintenance instructions, inspect the file tree, and keep Markdown portable against OKF v0.1. |
| Local registry management | `connect`, `disconnect`, `registry connect`, `registry disconnect`, `registry list`, `registry where` | Give local, published, archive, or Git bundles stable names and resolve those names back to filesystem paths. |
| Agent entrypoints | `use` | Print bundle-declared instructions or root `index.md` so an agent can load the right knowledge without hardcoding paths. |
| Local Markdown viewer | `open` | Browse connected or direct bundles with search, stacked Markdown panels, validation context, graph overview, and rich table rendering. |
| Export and publish | `to html`, `to html --plain`, `to json`, `to tar` | Publish a static viewer, emit plain semantic HTML, produce normalized JSON, or package a portable bundle archive. |

## Current Boundaries

`connect` and the registry store aliases for existing bundle folders. They also
materialize Open Knowledge manifests, tar archives, and Git remote sources into
the Open Knowledge cache before registration. After registration,
`registry where`, `use`, `open`, `validate`, and `to` work through the same
key-or-path resolution model for local folders and remote sources.

Default HTML viewer exports publish a portable bundle archive at
`assets/openknowledge-bundle.tar.gz` and an `openknowledge.json` manifest that
points to it. A deployed static wiki can therefore be connected by URL without
requiring Git access.

`to graph` is also planned work. Keep graph-export design notes on the
[graph exporter candidate](exporters/graph.md) page until the command is
implemented.

## Agent Flow

The intended agent loop is path-light:

```sh
openknowledge connect ./accessibility --as accessibility
openknowledge use accessibility --info
openknowledge use accessibility
openknowledge open accessibility
```

The agent can read `use` output as its task-specific entrypoint. When it needs
source files, it can resolve the bundle with:

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
* [Use command](commands/use.md) - agent entrypoint selection.
* [Open command](commands/open.md) - local Markdown viewer behavior.

## Update Notes

Update this page when a command moves between candidate and shipped status, a
new layer is added, remote source behavior changes, or README and index
navigation need a new product-level explanation.
