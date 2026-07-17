---
type: Command Documentation
title: openknowledge export
description: Export a knowledge base to HTML, JSON, graph, or portable tar formats.
tags: [openknowledge, cli, command, export]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge export`

Create a static site, normalized model, graph, or portable source archive from
a local or connected knowledge base.

## Targets

| Target | Output | Reference |
| --- | --- | --- |
| `html` | Static viewer or plain semantic HTML. | [HTML](/features/exporters/html.md) |
| `json` | Normalized parsed bundle model. | [JSON](/features/exporters/json.md) |
| `graph` | Authored link graph or derivative search graph. | [Graph](/features/exporters/graph.md) |
| `tar` | Reproducible portable source archive. | [Tar](/features/exporters/tar.md) |

## Usage

```sh
openknowledge export html --out ./site Wiki
openknowledge export html --plain --out ./plain-site Wiki
openknowledge export json Wiki
openknowledge export json --out ./bundle.json Wiki
openknowledge export graph Wiki
openknowledge export graph --type search --out ./search-graph.json Wiki
openknowledge export tar --out ./wiki.tar.gz Wiki
```

| Option | Applies to | Default | Description |
| --- | --- | --- | --- |
| `key-or-path` | all | `.` | Registry key or bundle root. |
| `--spec <version>` | all | `latest` | OKF version. |
| `--out <path>` | all | stdout for JSON/graph | Required directory for HTML; required file for tar. |
| `--type source|search` | graph | `source` | Graph projection. |
| `--plain` | HTML | off | Semantic HTML without viewer assets. |
| `--head-file`, `--head-html` | HTML viewer | environment | Trusted head injection. |
| `--script-src <src>` | HTML viewer | environment | Trusted script URL; repeatable. |

HTML and tar require zero validation errors. HTML also requires explicit
publication permission in `openknowledge.toml`. JSON and graph print
`schemaVersion: "1"` documents to stdout unless `--out` is set. Machine output
files are atomically replaced after complete serialization.

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
