---
type: Command Documentation
title: openknowledge connect
description: Connects a local or remote OKF bundle to the user's local knowledge registry.
tags: [openknowledge, cli, command, registry, connect, agent]
timestamp: 2026-06-20T00:00:00Z
---

# `openknowledge connect`

`openknowledge connect` adds an Open Knowledge bundle to the user's
registry so later commands can refer to it by a stable key. It is syntactic
sugar for `openknowledge registry connect`, with the same parsing, output, and
exit-code behavior as the registry subcommand.

The command connects existing local directories directly. Remote sources are
materialized into the Open Knowledge cache before registration. Resolution order
for HTTP(S) sources is Open Knowledge manifest, direct tar archive, then Git
fallback.

## Usage

```sh
openknowledge connect <source>
openknowledge connect <source> --as <key>
openknowledge connect <source> --access read|write
openknowledge connect <source> --no-validate
openknowledge connect --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `source` | argument | Local bundle root, existing registry key, Open Knowledge manifest URL, tar archive URL, or Git URL. Registry keys resolve to their stored path. |
| `--as` | flag | Explicit connection key. Defaults to root `okf_bundle_name`, then the folder name. |
| `--access` | flag | Access label stored with the connection, `read` or `write`. Defaults to `read`. |
| `--no-validate` | flag | Skip the validation status check in success output. |

Connection keys use the same validation as registry names: letters, numbers,
dots, underscores, and dashes, and they must not look like paths. Implicit keys
are normalized when needed.

## Behavior

`connect` resolves the target to an absolute local directory, reads optional
root `index.md` metadata, writes or updates the registry entry, then prints a
validation status unless `--no-validate` is set.

Connection flags may appear before or after the required `<source>` argument.
The positional-first forms in this page and the equivalent flag-first forms
share the same behavior.

Remote source handling:

* Open Knowledge manifest URLs are version `1` JSON documents with type
  `openknowledge.bundle`, a concrete supported `spec`, an archive path,
  `archiveFormat: "tar.gz"`, and a required 64-character `archiveSha256`.
* Website URLs try `openknowledge.json` under the URL path, then
  `/.well-known/openknowledge.json`.
* Direct `.tar`, `.tar.gz`, and `.tgz` URLs are downloaded and extracted.
* HTTP(S) URLs that are neither manifests nor archives fall back to shallow
  `git clone`.

Archives referenced by manifests are checksum-verified. Downloads are extracted
into the Open Knowledge cache using safe path checks, then manifest archives are
validated against the concrete declared spec before registration. When their
root declares `okf_version`, it must match the manifest spec. Relative archive
URLs are resolved from the manifest's final URL after HTTP redirects.

Remote manifest and archive processing has finite resource ceilings: manifests
are limited to 1 MiB, compressed archives to 512 MiB, and extraction to 100,000
entries, 256 MiB per regular file, and 2 GiB total. Extraction occurs in a
sibling staging directory and publishes the target only after every entry and
bundle check succeeds; rejected archives leave no partial target. These byte
limits do not apply to the separate shallow Git clone fallback.

Remote cache identity is derived from the normalized source, not from `--as`,
so reconnecting the same source under another key reuses one materialization.
Each new cache stores a versioned owner-only provenance sidecar. Registry source
metadata preserves the requested URL, final manifest and archive URLs after
redirects, concrete spec, archive SHA-256 or exact Git commit, a deterministic
managed-tree SHA-256, fetch time, and the complete managed cache root. Cache
hits restore this record instead of guessing the source type from the input URL.

Materialization for one source is serialized by in-process and cross-process
cache locks. Archives and Git repositories are downloaded or cloned into
sibling staging directories; Git content must pass OKF validation, and only a
complete validated staging tree is published. Bundle validation rejects every
symbolic link below the root, so a cloned repository cannot redirect later
reads or exports into the host filesystem. If replacement fails, the
previous cache is restored. Different sources keep independent locks and can be
materialized concurrently.

`connect` uses root metadata when present: `okf_bundle_name` can provide the
default key, `okf_bundle_title` and `okf_bundle_purpose` appear in success
output, and `okf_bundle_entry_<name>` values are listed as entrypoints. Missing
metadata does not block connection; display names fall back to the root
`index.md` H1, then the folder name.

Connecting the same absolute path updates the existing connection. If an
implicit key collides with another path, `connect` chooses the next available
suffix such as `project-2` and prints a warning. If an explicit `--as <key>`
collides with another path, the command fails.

Validation is a status signal, not a connection gate. Success output reports
`valid`, `warnings`, `invalid`, or `unknown` depending on whether validation
ran and what it found.

## Quick Examples

```sh
openknowledge connect ./project-memory
openknowledge connect ./accessibility --as accessibility
openknowledge connect ./team-wiki --access write
openknowledge connect https://openknowledge.sh/wiki/
openknowledge connect https://openknowledge.sh/openknowledge-bundle.tar.gz
openknowledge connect https://github.com/openknowledge-sh/accessibility.git --as accessibility
```

## Example Output

`openknowledge connect --as personal ./project-memory` prints the registry key,
display name, resolved path, access label, validation status, and any bundle
metadata:

```text
OK Connected knowledge bundle
key      personal
name     Project Memory
path     /work/project-memory
access   read
status   valid
purpose  Durable project context.
entries  default
```

## Caveats

Remote archive and manifest sources require network access for non-local URLs.
Git fallback requires `git` on `PATH`. Existing cached materializations are
reused when they still validate; `connect` does not check remote freshness for
an existing cache entry. Use `openknowledge registry refresh <key>` to fetch and
atomically switch a managed connection to a newly validated generation. See
[registry](registry.md) for lifecycle and storage details.

## Command Change History

### 2026-07-15 - Atomic locked remote cache publication

Remote sources now take a source-specific process and filesystem lock. Git
clones use staging and must validate before publication, archive staging no
longer deletes the prior target first, and replacement rolls back to the prior
cache on failure. The cache directory and lock files are owner-only. Source
anchors: `packages/cli/cmd/openknowledge/main.go` and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Source-addressed cache provenance

Remote cache names no longer depend on registry aliases. New materializations
persist a versioned owner-only provenance record with requested and resolved
URLs, immutable archive or Git identity, spec, fetch time, and managed root;
cache hits reuse that exact record. Legacy caches without a sidecar are retained
and marked `unknown` when their source type cannot be established honestly.
Source anchors: `packages/cli/internal/okf/registry.go`,
`packages/cli/internal/okf/registry_test.go`,
`packages/cli/cmd/openknowledge/main.go`, and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Bounded atomic archive materialization

Manifest and archive downloads now enforce byte ceilings, tar extraction
enforces entry, per-file, and total expanded-size ceilings, and successful
extraction is published atomically from staging. Oversized or malformed inputs
leave no partial download or extraction target. Source anchors:
`packages/cli/internal/okf/archive.go`,
`packages/cli/internal/okf/archive_test.go`,
`packages/cli/cmd/openknowledge/main.go`, and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Enforced remote manifest contract

Remote manifests now require the supported versioned type, a concrete OKF spec,
`tar.gz`, and a valid SHA-256. Downloads are validated against that declared
spec, conflicting root `okf_version` values are rejected, relative archive URLs
use the post-redirect manifest location, and non-404 manifest download errors
are preserved. Source anchors: `packages/cli/internal/okf/archive.go`,
`packages/cli/internal/okf/archive_test.go`,
`packages/cli/cmd/openknowledge/main.go`, and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Positional-first flags

`openknowledge connect` and `openknowledge registry connect` now parse
`--as`, `--access`, and `--no-validate` after `<source>` as documented while
continuing to accept flag-first forms. Source anchors:
`packages/cli/cmd/openknowledge/main.go` and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-06-20

`openknowledge connect` became the top-level alias for the registry `connect`
subcommand after connection management moved under the registry namespace.

`openknowledge connect` shipped for local directories with `--as`,
`--access`, and `--no-validate`, metadata-derived keys, validation status
output, implicit key suffixing, and explicit collision failures.

`openknowledge connect` now materializes Open Knowledge manifests, tar archives,
and Git remote sources into the Open Knowledge cache, records source metadata,
and stores registry state as path-keyed `connections`.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/internal/okf/registry.go`
> * `packages/cli/internal/okf/metadata.go`
> * `packages/cli/internal/okf/registry_test.go`
> * `packages/cli/internal/okf/metadata_test.go`
>
> **Update notes**
>
> Update this page when connection storage, remote-source materialization, key
> derivation, validation status semantics, or success output changes. CLI
> behavior changes also require [CLI changelog](/changelog/cli.md) updates.
