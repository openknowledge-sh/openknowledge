---
type: Exporter Documentation
title: Tar Exporter
description: Portable tar.gz bundle export behavior.
tags: [openknowledge, cli, exporter, tar, archive]
timestamp: 2026-06-20T00:00:00Z
---

# Tar Exporter

`openknowledge to tar` writes a portable `tar.gz` archive of an Open Knowledge
bundle. The archive is the transport format used by published HTML exports and
remote `connect` materialization.

## Command

```sh
openknowledge to tar --out <file> [path]
openknowledge to tar --spec <version> --out <file> [path]
openknowledge to tar --help
```

## Behavior

The exporter validates the bundle root for the selected spec version, walks the
source bundle, skips `.git`, and writes source files with relative paths into a
gzip-compressed tar archive. The command prints the archive SHA-256 so callers
can publish or verify it.

Default viewer HTML exports call the same archive writer and place the archive
at `assets/openknowledge-bundle.tar.gz`. The companion `openknowledge.json`
manifest is contract version `1` with type `openknowledge.bundle`, a concrete
supported OKF `spec`, `archiveFormat: "tar.gz"`, the archive path, and its
required SHA-256. Unlike the standalone source-preserving `to tar` command, the
public HTML export filters Markdown files marked `okf_publish: false` from this
downloadable archive so hidden drafts cannot be recovered through the remote
connect asset.

Remote `openknowledge connect` downloads archives from manifests or direct
`.tar`, `.tar.gz`, and `.tgz` URLs, rejects unsafe archive entries such as path
traversal or symlinks, validates manifest archives against their declared spec,
rejects a conflicting root `okf_version`, then stores the materialized bundle
in the Open Knowledge cache. A portable manifest cannot use the moving
`latest` spec alias.

Archive consumers cap compressed downloads at 512 MiB and extraction at
100,000 entries, 256 MiB per regular file, and 2 GiB total. Extraction uses a
sibling staging directory; the requested target appears only after the full
archive succeeds, and an existing target is never overlaid.

## Change History

### 2026-07-15 - Publish-scoped HTML archives

Default HTML exports now omit every Markdown file marked
`okf_publish: false` from `assets/openknowledge-bundle.tar.gz`, matching the
existing HTML, static payload, graph, and discovery-file filter. Standalone
`openknowledge to tar` remains a complete source-bundle export. Source anchors:
`packages/cli/internal/okf/archive.go`,
`packages/cli/cmd/openknowledge/viewer.go`, and
`packages/cli/cmd/openknowledge/viewer_test.go`.

### 2026-07-15 - Bounded atomic extraction

Archive download and extraction now have explicit compressed, entry, file, and
expanded-size ceilings. Failed extraction removes its staging directory and
does not publish or overwrite a target. Source anchors:
`packages/cli/internal/okf/archive.go`,
`packages/cli/internal/okf/archive_test.go`,
`packages/cli/cmd/openknowledge/main.go`, and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Strict manifest integrity

Manifest consumers now validate every required identity and format field,
require a SHA-256, and bind archive validation to the concrete declared spec.
Source anchors: `packages/cli/internal/okf/archive.go`,
`packages/cli/internal/okf/archive_test.go`,
`packages/cli/cmd/openknowledge/main.go`, and
`packages/cli/cmd/openknowledge/main_test.go`.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/archive.go`
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/cmd/openknowledge/viewer.go`
> * `packages/cli/internal/okf/archive_test.go`
> * `packages/cli/cmd/openknowledge/main_test.go`
> * `packages/cli/cmd/openknowledge/viewer_test.go`
>
> **Update notes**
>
> Update this page when archive layout, manifest fields, remote extraction safety,
> or `to tar` command output changes. CLI behavior changes also require
> [CLI changelog](/changelog/cli.md) updates.
