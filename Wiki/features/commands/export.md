---
type: Command Documentation
title: openknowledge export
description: Export a knowledge base to HTML, JSON, graph, or portable tar formats.
tags: [openknowledge, cli, command, export]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge export`

Create a static site, normalized model, graph, or portable source archive.
The source can be a local or connected knowledge base.

## Targets

| Target | Output | Reference |
| --- | --- | --- |
| `html` | Static viewer or plain semantic HTML. | [HTML](/features/exporters/html.md) |
| `json` | Normalized parsed bundle model. | [JSON](/features/exporters/json.md) |
| `graph` | Authored link graph or derivative search graph. | [Graph](/features/exporters/graph.md) |
| `tar` | Reproducible portable source archive. | [Tar](/features/exporters/tar.md) |

## Usage

```sh
okn export html --out ./site Wiki
okn export html --no-source-archive --out ./site Wiki
okn export html --plain --out ./plain-site Wiki
okn export json Wiki
okn export json --out ./bundle.json Wiki
okn export graph Wiki
okn export graph --type search --out ./search-graph.json Wiki
okn export tar --out ./wiki.tar.gz Wiki
```

| Option | Applies to | Default | Description |
| --- | --- | --- | --- |
| `key-or-path` | all | `.` | Registry key or bundle root. |
| `--spec <version>` | all | `latest` | OKF version. |
| `--out <path>` | all | stdout for JSON/graph | Required directory for HTML. Required file for tar. |
| `--type source|search` | graph | `source` | Graph projection. |
| `--plain` | HTML | off | Semantic HTML without viewer assets. |
| `--no-source-archive` | HTML viewer | off | Omit the source archive and connect manifest. |
| `--head-file`, `--head-html` | HTML viewer | environment | Trusted head injection. |
| `--script-src <src>` | HTML viewer | environment | Trusted script URL. Repeatable. |

HTML and tar require zero validation errors. HTML also requires publication
permission in `.openknowledge.toml`.

JSON and graph print `schemaVersion: "1"` documents to stdout by default. Use
`--out` to write a file. The command replaces a machine output file only after
complete serialization.

Unknown targets or unsupported flags are usage errors with exit status `2`.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/main.go`
> - `packages/cli/internal/okf/html.go`
> - `packages/cli/internal/okf/bundle.go`
> - `packages/cli/internal/okf/graph.go`
> - `packages/cli/internal/okf/archive.go`
>
> **Update notes**
>
> Add new targets here and under `/features/exporters/`.
