---
type: Command Documentation
title: openknowledge to
description: Converts an Open Knowledge bundle to supported output formats.
tags: [openknowledge, cli, command, export]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge to`

`openknowledge to` is the conversion command group. Current shipped targets are
`html`, `json`, and `tar`.

## Usage

```sh
openknowledge to html --out <folder> [path]
openknowledge to html --plain --out <folder> [path]
openknowledge to html --spec <version> --out <folder> [path]
openknowledge to json [path]
openknowledge to json --out <file> [path]
openknowledge to json --spec <version> [path]
openknowledge to tar --out <file> [path]
openknowledge to tar --spec <version> --out <file> [path]
openknowledge to --help
```

## Targets

| Target | Status | Details |
| --- | --- | --- |
| `html` | shipped | See [HTML exporter](/features/exporters/html.md). |
| `json` | shipped | See [JSON exporter](/features/exporters/json.md). |
| `tar` | shipped | See [Tar exporter](/features/exporters/tar.md). |
| `graph` | candidate | See [Graph exporter candidate](/features/exporters/graph.md). |

## Common Flags

| Name | Kind | Applies To | Required | Default | Description |
| --- | --- | --- | --- | --- | --- |
| `path` | argument | `html`, `json`, `tar` | no | current directory | Knowledge base root. |
| `--spec` | flag | `html`, `json`, `tar` | no | `latest` | OKF spec version. |
| `--out` | flag | `html`, `json`, `tar` | HTML and TAR yes, JSON no | stdout for JSON | Output folder for HTML, optional output file for JSON, and archive file for TAR. |
| `--plain` | flag | `html` only | no | off | Write plain semantic HTML without viewer chrome, CSS, or JavaScript. |

## Quick Examples

```sh
openknowledge to html --out ./site ./project-memory
openknowledge to html --plain --out ./plain-site ./project-memory
openknowledge to json ./project-memory
openknowledge to json --out ./bundle.json ./project-memory
openknowledge to tar --out ./bundle.tar.gz ./project-memory
```

## Behavior

`to html` requires `--out <folder>`. It has two modes:

* Default viewer export: static viewer pages with file browsing, search,
  stacked-panel navigation, embedded note data, theme/source configuration, and
  remote-connect assets.
* Plain export: unstyled semantic HTML without viewer CSS, JavaScript, theme
  links, source buttons, or rich table controls.

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

Unknown targets and unknown flags exit with status `2`.

## Use Cases

* Publish a static viewer copy of a wiki.
* Produce a normalized JSON model for downstream tools or agents.
* Produce a portable tarball that can be served from static hosting and
  connected later.
* Keep future exporter targets grouped under one command family.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/html.go`
> * `packages/cli/internal/okf/bundle.go`
> * `packages/cli/cmd/openknowledge/viewer.go`
> * `packages/cli/cmd/openknowledge/viewer_theme.go`
> * `packages/cli/cmd/openknowledge/viewer_theme.css`
> * `packages/cli/internal/okf/export_test.go`
> * `packages/cli/cmd/openknowledge/viewer_test.go`
>
> **Update notes**
>
> When a new target is added, update this page, the exporter section, root help,
> README command tables, and [CLI changelog](/changelog/cli.md).
