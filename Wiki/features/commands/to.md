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
openknowledge to graph --type source [path]
openknowledge to graph --type search [path]
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
| `--type` | flag | `graph` only | no | `source` | Graph type, `source` or `search`. |
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
openknowledge to graph --type search ./project-memory
```

## Example Output

File-writing targets print short summaries:

```text
OK Exported HTML
root /work/project-memory
out /work/site
wrote 10 files
```

```text
OK Exported TAR
root /work/project-memory
out /work/bundle.tar.gz
sha256 9f7f4c4832d5e833aff7574d957172cfbaf9bbece0cbb13ed69c97e5b9c11897
```

Stdout targets print JSON when `--out` is omitted:

```json
{
  "root": "/work/project-memory",
  "specVersion": "0.1",
  "type": "source",
  "nodes": [],
  "edges": []
}
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

Portable `to html` and `to tar` outputs require zero validation errors for the
selected spec. Warnings remain publishable. Validation failure occurs before
the exporter creates or replaces output, so it cannot advertise a manifest
whose archive the remote `connect` consumer will reject.

Both HTML modes build a complete sibling generation and switch it into place
only after every page and asset succeeds. A failed build preserves the previous
site, while a successful rebuild removes stale pages from the previous
generation. The output may be nested inside the source bundle, in which case it
is excluded from the portable source archive, but the output must not equal or
contain the source bundle.

`to json` serializes the normalized bundle model. It prints to stdout by
default and writes to `--out <file>` when provided. `--plain` is not valid for
JSON. File output is replaced atomically after the complete JSON document is
ready. See [JSON exporter](/features/exporters/json.md).

`to tar` requires `--out <file>`. It writes a gzip-compressed tar archive of the
source bundle and prints the archive SHA-256. `--plain` is not valid for TAR.
See [Tar exporter](/features/exporters/tar.md).

`to graph` serializes AST-backed graph JSON. The default `--type source` graph
contains bundle file nodes and deduplicated existing local Markdown links.
`--type search` writes a derivative search graph with source file nodes,
heading chunk nodes, file-to-chunk containment edges, chunk reading-order
edges, and chunk-level local-link edges. It prints to stdout by default and
writes to `--out <file>` when provided. `--plain` is not valid for graph
output. See [Graph exporter](/features/exporters/graph.md).

When `--out` is used for JSON or graph output, a write failure does not expose
a partially written machine-readable document at the destination path.

Unknown targets and unknown flags exit with status `2`.

## Use Cases

* Publish a static viewer copy of a wiki.
* Produce a normalized JSON model for downstream tools or agents.
* Produce a portable tarball that can be served from static hosting and
  connected later.
* Produce a link graph for visualization, orphan detection, or relationship
  analysis.
* Produce a search graph for retrieval tooling and graph-expanded search.

## Command Change History

### 2026-07-06

`openknowledge to graph` added `--type source|search`. `source` keeps the
existing file/link graph behavior as the default. `search` exports derivative
heading chunk nodes and typed retrieval edges.

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
