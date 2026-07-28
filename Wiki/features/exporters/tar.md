---
type: Exporter Documentation
title: Tar Exporter
description: Portable tar.gz bundle export behavior.
tags: [openknowledge, cli, exporter, tar, archive]
timestamp: 2026-06-20T00:00:00Z
---

# Tar Exporter

`openknowledge export tar` writes a portable `tar.gz` archive of an Open Knowledge bundle.
Published HTML exports and remote `connect` operations use this transport format.

## Command

```sh
openknowledge export tar --out <file> [key-or-path]
openknowledge export tar --spec <version> --out <file> [key-or-path]
openknowledge export tar --help
```

## Behavior

The exporter resolves a registry key or bundle path.
It validates that root against the selected spec version.
The root must have no errors, but it can have warnings.
The exporter applies this gate before it creates or replaces the output.
It then reads the source bundle and skips `.git`.
It writes source files with relative paths to a gzip-compressed tar archive.
The command prints the archive SHA-256 for publication or verification.
The exporter rejects symbolic links and other non-regular file system entries.
It does not follow these entries or copy content from outside the real bundle root.

Archive identity is reproducible from content and executable intent.
The exporter sorts the entries.
It omits the gzip file name and host timestamps.
It uses canonical tar timestamps and owner fields.
It normalizes regular file modes to `0644`.
It uses `0755` when an executable bit is present.
The destination file name does not change the archive bytes or SHA-256.
The host UID, GID, modification time, and non-executable permission bits do not change these values.

Default viewer HTML exports use the same archive writer.
They write the archive to `assets/openknowledge-bundle.tar.gz`.
The related `openknowledge.json` manifest uses contract version `1`.
Its type is `openknowledge.bundle`.
It contains a supported OKF `spec` value and `archiveFormat: "tar.gz"`.
It also contains the archive path and required SHA-256.

The public HTML export requires `[publish] enabled = true`.
It then uses an explicit publication set.
It omits Markdown with `okf_publish: false`.
It omits non-Markdown files that do not match `[publish].assets`.
Asset patterns cannot include Markdown again.
The artifact never includes `.git`, `.openknowledge`, or `openknowledge.toml`.
These rules exclude Markdown denied by `okf_publish`.
They also exclude runtime state and unrelated repository files.
The standalone `export tar` command preserves the complete source.
The Draft 2020-12 manifest contract is at
`https://openknowledge.sh/schemas/cli/manifest/v1/bundle.schema.json`.

Remote `openknowledge connect` downloads archives from manifests.
It also accepts direct `.tar`, `.tar.gz`, and `.tgz` URLs.
It rejects unsafe archive entries, such as path traversal and symbolic links.
It validates manifest archives against their declared spec.
It rejects a conflicting root `okf_version`.
Then, it stores the materialized bundle in the Open Knowledge cache.
A portable manifest cannot use the moving `latest` spec alias.

Manifest decoding rejects unknown fields and duplicate object keys.
It also rejects trailing JSON and invalid canonical identities.
The CLI test suite compiles and tests the runtime contract with its published schema.

Archive consumers limit compressed downloads to 512 MiB.
They limit extraction to 100,000 entries.
They also limit each regular file to 256 MiB and the total to 2 GiB.
Extraction uses a sibling staging directory.
The requested target appears only after extraction completes.
Extraction does not overlay an existing target.


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
> Update this page after a change to archive layout, manifest fields, remote
> extraction safety, or `export tar` output.
> Also update the [CLI changelog](/changelog/cli.md) after a CLI behavior change.
