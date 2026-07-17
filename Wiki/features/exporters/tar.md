---
type: Exporter Documentation
title: Tar Exporter
description: Portable tar.gz bundle export behavior.
tags: [openknowledge, cli, exporter, tar, archive]
timestamp: 2026-06-20T00:00:00Z
---

# Tar Exporter

`openknowledge export tar` writes a portable `tar.gz` archive of an Open Knowledge
bundle. The archive is the transport format used by published HTML exports and
remote `connect` materialization.

## Command

```sh
openknowledge export tar --out <file> [key-or-path]
openknowledge export tar --spec <version> --out <file> [key-or-path]
openknowledge export tar --help
```

## Behavior

The exporter resolves a registry key or bundle path, validates that root for
the selected spec version, and requires zero errors; warnings remain allowed.
It applies this gate before creating or replacing the output. It then walks the
source bundle, skips
`.git`, and writes source files with relative paths into a gzip-compressed tar
archive. The command prints the archive SHA-256 so callers can publish or
verify it. Symbolic links and other non-regular filesystem entries are rejected
before publication; the writer never follows them or copies content from
outside the real bundle root.

Archive identity is reproducible from content and executable intent. Entries
are sorted, gzip filename and host timestamps are omitted, tar timestamps and
owner fields are canonical, and regular-file modes normalize to `0644` or
`0755` when any executable bit is present. Changing the destination filename,
host UID/GID, modification time, or non-executable permission bits therefore
does not change the archive bytes or reported SHA-256.

Default viewer HTML exports call the same archive writer and place the archive
at `assets/openknowledge-bundle.tar.gz`. The companion `openknowledge.json`
manifest is contract version `1` with type `openknowledge.bundle`, a concrete
supported OKF `spec`, `archiveFormat: "tar.gz"`, the archive path, and its
required SHA-256. Unlike the standalone source-preserving `export tar` command, the
public HTML export first requires `[publish] enabled = true`, then uses an
explicit publication set: Markdown marked `okf_publish: false` is omitted, and
non-Markdown files are omitted unless they
match `[publish].assets`. Asset patterns cannot re-include Markdown;
`.git`, `.openknowledge`, and `openknowledge.toml` are always absent. This keeps
Markdown denied by `okf_publish`, `.openknowledge` runtime state, and incidental repository files
out of the remote-connect artifact. The standalone `export tar` command intentionally
remains a complete source export. The Draft 2020-12 manifest contract is published at
`https://openknowledge.sh/schemas/cli/manifest/v1/bundle.schema.json`.

Remote `openknowledge connect` downloads archives from manifests or direct
`.tar`, `.tar.gz`, and `.tgz` URLs, rejects unsafe archive entries such as path
traversal or symlinks, validates manifest archives against their declared spec,
rejects a conflicting root `okf_version`, then stores the materialized bundle
in the Open Knowledge cache. A portable manifest cannot use the moving
`latest` spec alias.

Manifest decoding fails closed on unknown fields, duplicate object keys,
trailing JSON, and invalid canonical identities. The runtime contract and its
published schema are compiled and exercised together in the CLI test suite.

Archive consumers cap compressed downloads at 512 MiB and extraction at
100,000 entries, 256 MiB per regular file, and 2 GiB total. Extraction uses a
sibling staging directory; the requested target appears only after the full
archive succeeds, and an existing target is never overlaid.


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
> or `export tar` command output changes. CLI behavior changes also require
> [CLI changelog](/changelog/cli.md) updates.
