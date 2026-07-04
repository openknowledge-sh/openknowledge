---
type: Command Documentation
title: openknowledge to
description: Converts an Open Knowledge bundle to supported output formats.
tags: [openknowledge, cli, command, export]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge to`

`openknowledge to` is the conversion command group. Current shipped targets are
`html`, `json`, `tar`, and `graph`.

## Usage

```sh
openknowledge to html --out <folder> [path]
openknowledge to html --plain --out <folder> [path]
openknowledge to html --head-file <file> --out <folder> [path]
openknowledge to html --script-src <src> --out <folder> [path]
openknowledge to html --spec <version> --out <folder> [path]
openknowledge to json [path]
openknowledge to json --out <file> [path]
openknowledge to json --spec <version> [path]
openknowledge to tar --out <file> [path]
openknowledge to tar --spec <version> --out <file> [path]
openknowledge to graph [path]
openknowledge to graph --out <file> [path]
openknowledge to graph --spec <version> [path]
openknowledge to --help
```

## Targets

| Target | Status | Details |
| --- | --- | --- |
| `html` | shipped | See [HTML exporter](/features/exporters/html.md). |
| `json` | shipped | See [JSON exporter](/features/exporters/json.md). |
| `tar` | shipped | See [Tar exporter](/features/exporters/tar.md). |
| `graph` | shipped | See [Graph exporter](/features/exporters/graph.md). |

## Common Flags

| Name | Kind | Applies To | Required | Default | Description |
| --- | --- | --- | --- | --- | --- |
| `path` | argument | `html`, `json`, `tar`, `graph` | no | current directory | Knowledge base root. |
| `--spec` | flag | `html`, `json`, `tar`, `graph` | no | `latest` | OKF spec version. |
| `--out` | flag | `html`, `json`, `tar`, `graph` | HTML and TAR yes, JSON/graph no | stdout for JSON and graph | Output folder for HTML, optional output file for JSON and graph, and archive file for TAR. |
| `--head-file` | flag | `html` default viewer export only | no | `OPENKNOWLEDGE_HEAD_FILE` | Trusted HTML fragment file to inject into every generated viewer page `<head>`. |
| `--head-html` | flag | `html` default viewer export only | no | `OPENKNOWLEDGE_HEAD_HTML` | Trusted HTML fragment to inject into every generated viewer page `<head>`. |
| `--plain` | flag | `html` only | no | off | Write plain semantic HTML without viewer chrome, CSS, or JavaScript. |
| `--script-src` | repeatable flag | `html` default viewer export only | no | `OPENKNOWLEDGE_SCRIPT_SRC` | Script `src` to inject into every generated viewer page `<head>`. Environment values may be comma- or newline-separated. |

## Quick Examples

```sh
openknowledge to html --out ./site ./project-memory
openknowledge to html --plain --out ./plain-site ./project-memory
openknowledge to html --head-file ./head.html --out ./site ./project-memory
openknowledge to html --script-src /analytics.js --out ./site ./project-memory
openknowledge to json ./project-memory
openknowledge to json --out ./bundle.json ./project-memory
openknowledge to tar --out ./bundle.tar.gz ./project-memory
openknowledge to graph ./project-memory
openknowledge to graph --out ./graph.json ./project-memory
```

## Behavior

`to html` requires `--out <folder>`. It has two modes:

* Default viewer export: static viewer pages with file browsing, search,
  stacked-panel navigation, embedded note data, theme/source configuration, and
  discovery and remote-connect assets. Trusted custom head HTML can be injected
  into every generated viewer page through `--head-file`, `--head-html`,
  repeatable `--script-src`, or their matching environment variables.
* Plain export: unstyled semantic HTML without viewer CSS, JavaScript, theme
  links, source buttons, or rich table controls.

Default viewer exports write `llms.txt` for agents and LLM-oriented consumers.
When `[html.site].base_url` is configured in `openknowledge.toml`, they also
write `sitemap.xml` with absolute URLs for published pages. Files with
`okf_publish: false` are omitted from generated pages, static note payloads,
`llms.txt`, and `sitemap.xml`.

Default viewer exports also write `openknowledge.json` and
`assets/openknowledge-bundle.tar.gz`. The manifest points to the archive and
includes its SHA-256 so `openknowledge connect <deployed-wiki-url>` can
materialize the source bundle from the static site. See
[HTML exporter](/features/exporters/html.md) for rendering, theme, source-link,
and manifest details.

`to json` serializes the normalized bundle model. It prints to stdout by
default and writes to `--out <file>` when provided. `--plain` is not valid for
JSON. See [JSON exporter](/features/exporters/json.md).

`to tar` requires `--out <file>`. It writes a gzip-compressed tar archive of the
source bundle and prints the archive SHA-256. `--plain` is not valid for TAR.
See [Tar exporter](/features/exporters/tar.md).

`to graph` serializes an AST-backed node and edge graph. Nodes come from bundle
files, and edges are deduplicated existing local Markdown links. It prints to
stdout by default and writes to `--out <file>` when provided. `--plain` is not
valid for graph output. See [Graph exporter](/features/exporters/graph.md).

Unknown targets and unknown flags exit with status `2`.

## Use Cases

* Publish a static viewer copy of a wiki.
* Produce a normalized JSON model for downstream tools or agents.
* Produce a portable tarball that can be served from static hosting and
  connected later.
* Produce a link graph for visualization, orphan detection, or relationship
  analysis.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/html.go`
> * `packages/cli/internal/okf/bundle.go`
> * `packages/cli/internal/okf/graph.go`
> * `packages/cli/internal/okf/graph_types.go`
> * `packages/cli/cmd/openknowledge/viewer.go`
> * `packages/cli/cmd/openknowledge/viewer_discovery.go`
> * `packages/cli/cmd/openknowledge/viewer_theme.go`
> * `packages/cli/cmd/openknowledge/viewer_theme.css`
> * `packages/cli/internal/okf/export_test.go`
> * `packages/cli/cmd/openknowledge/viewer_test.go`
>
> **Update notes**
>
> When a new target is added, update this page, the exporter section, root help,
> README command tables, and [CLI changelog](/changelog/cli.md).
