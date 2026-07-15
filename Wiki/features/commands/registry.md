---
type: Command Documentation
title: openknowledge registry
description: Manages knowledge bundle connections.
tags: [openknowledge, cli, command, registry]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge registry`

`openknowledge registry` manages knowledge bundle connections. Registry
keys are shortcuts only; commands still work with normal filesystem paths. The
local viewer also uses the registry as its default workspace list when
`openknowledge view` is run without a path.

The top-level [openknowledge connect](connect.md) and
[openknowledge disconnect](disconnect.md) commands are aliases for
`openknowledge registry connect` and `openknowledge registry disconnect`.
Lookup lives under the same namespace as `openknowledge registry where`.

## Usage

```sh
openknowledge registry connect <source>
openknowledge registry connect <source> --as <key>
openknowledge registry connect <source> --access read|write
openknowledge registry connect <source> --no-validate
openknowledge registry disconnect <key|path>
openknowledge registry disconnect <key|path> --keep-files
openknowledge registry disconnect <key|path> --delete-files
openknowledge registry refresh <key|path>
openknowledge registry refresh <key|path> --force
openknowledge registry list
openknowledge registry status [key|path]
openknowledge registry status [key|path] --json
openknowledge registry where <name|path>
openknowledge registry --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `connect` | subcommand | Add or update a local or remote bundle connection. |
| `disconnect` | subcommand | Remove a local bundle connection. |
| `refresh` | subcommand | Fetch and atomically publish a new generation for a managed remote connection. |
| `list` | subcommand | Print registered knowledge bases. |
| `status` | subcommand | Check local bundle and managed-cache integrity offline. |
| `where` | subcommand | Print the absolute path for a registry key or path. |
| `key` | argument | Connection key using letters, numbers, dots, underscores, or dashes. |
| `source` | argument | Knowledge bundle folder path, existing registry key, Open Knowledge manifest URL, tar archive URL, or Git URL. |
| `--as` | flag | Explicit connection key for `connect`. |
| `--access` | flag | Access label stored with a connection, `read` or `write`. |
| `--no-validate` | flag | Skip validation status output for `connect`. |
| `--keep-files` | flag | Keep bundle files after `disconnect`; this is the default. |
| `--delete-files` | flag | Delete files only when the entry is marked `managed`. |
| `--force` | flag | Allow `refresh` to discard detected local cache changes. |
| `--json` | flag | Print the versioned registry status contract. |

## Behavior

Registry entries are stored as JSON under the user config directory at
`openknowledge/registry.json`. Set `OPENKNOWLEDGE_REGISTRY_FILE` to use a
specific registry file, which is useful in tests and isolated agent runs.
Mutations take an inter-process lock and replace the registry atomically, so
parallel CLI or agent processes cannot discard one another's connections and
readers never observe a partially written JSON document. Registry and lock
files use owner-only permissions.

`registry connect` expands `~`, resolves the target to an absolute path,
requires it to be an existing directory, derives or accepts a key, reads
optional root bundle metadata, writes the registry entry, and prints validation
status unless `--no-validate` is set. For remote sources, it first materializes
the source into the Open Knowledge cache and stores source metadata on the
connection. HTTP(S) sources try Open Knowledge manifests, direct tar archives,
then Git fallback.

`registry disconnect` removes entries by key or absolute path and keeps files
by default. `--delete-files` is guarded and only applies to CLI-managed remote
caches.

`registry refresh` applies only to managed remote connections. It inspects the
current cache first and refuses to discard detected content, Git, or provenance
changes unless `--force` is supplied. The command materializes and validates a
fresh cache generation at a new path, records the exact new source identity in
an atomic registry replacement, and only then deletes the previous generation.
A download, clone, validation, provenance-write, or registry-replacement failure
leaves the previously registered generation available. If the registry switch
succeeds but old-cache cleanup fails, the new generation remains active and the
command reports a warning with exit status `1`.

Connection, disconnection, refresh, and status flags may appear before or after
their required positional argument. The registry connection subcommands and
their top-level aliases share the applicable parsing contract.

`registry list` prints the registry file path and sorted entries.

`registry status` checks all entries, or one optional key/path, without network
access. It validates each bundle against its recorded concrete spec, checks
registered and managed paths, compares the deterministic managed-tree SHA-256,
compares registry provenance with the cache sidecar, and for Git also checks
the exact commit and dirty working-tree state. It reports `ok`, `warnings`,
`unverified`, `modified`, `invalid`, or `missing`. Legacy managed caches without
a recorded content hash are `unverified`. Any `unverified`, `modified`,
`invalid`, or `missing` entry produces exit status `1`.

`registry status --json` returns a `schemaVersion: "1"` envelope with the
registry path, summary counts, entry validation, identity checks, source
provenance, and problems. Its Draft 2020-12 schema is
`packages/cli/schemas/v1/registry-status.schema.json`. Status is deliberately
offline: it does not claim whether a newer remote version exists.

`registry where` prints only an absolute path. If the value looks like a path,
it is expanded and normalized directly. Otherwise it must match a registry key.

## Example Output

After connecting a bundle as `personal`, `openknowledge registry list` prints
the registry file and sorted entries:

```text
Open Knowledge Registry
known knowledge bases

config /home/user/.config/openknowledge/registry.json

  personal           /work/project-memory
```

`openknowledge registry where personal` prints only the resolved path, which is
useful for scripts:

```text
/work/project-memory
```

`openknowledge registry status personal` prints the local state:

```text
Open Knowledge Registry Status
offline cache and bundle integrity

config /home/user/.config/openknowledge/registry.json

  OK         personal           /work/project-memory
```

For a managed remote connection, `openknowledge registry refresh personal`
prints the generation switch:

```text
OK Refreshed knowledge bundle
key        personal
old path   /home/user/.config/openknowledge/bundles/wiki-a1b2c3d4e5f6
path       /home/user/.config/openknowledge/bundles/wiki-a1b2c3d4e5f6-refresh-123456
source     git
identity   0123456789abcdef0123456789abcdef01234567
```

## Top-Level Aliases

`openknowledge connect` and `openknowledge disconnect` are retained as
convenience aliases for the corresponding `openknowledge registry` subcommands.
They share parsing, output, and exit-code behavior.

`openknowledge where` is not part of the command surface. Use
`openknowledge registry where`.

## Storage

Registry storage is path-keyed under `connections`. Entries store the stable
key, optional access label, optional source metadata, and whether the files are
CLI-managed. Local connections normally leave `managed` unset; remote manifest,
tar, and Git connections are marked managed because their files live in the
Open Knowledge cache.

New remote source records preserve requested and resolved URLs, final manifest
and archive URLs, the archive SHA-256 or Git commit, concrete OKF spec, fetch
timestamp, deterministic managed-tree SHA-256, and the complete managed cache
root. `ref` remains populated for compatibility with older archive-URL readers.
Cache provenance is also stored in a versioned owner-only sidecar beside the
source-addressed cache directory, so reconnecting does not infer or lose the
source identity.

Source-specific in-process and filesystem locks serialize cache publication.
The cache root is owner-only. Archive extraction and Git clone staging are
published only after validation, and a failed replacement restores the previous
cache rather than deleting it first.

Managed deletion uses the persisted complete cache root rather than assuming
the registered bundle path is the cache root. The CLI accepts deletion only for
a direct child of its cache whose registered bundle lies inside it, then
tombstones that container and removes the exact unchanged registry snapshot.

Bundle metadata such as purpose, tags, and entrypoints remains in bundle
content as `okf_bundle_*` root metadata.

Use the registry to give shared or standalone wikis stable names while keeping
aliases outside the bundle content.

## Command Change History

### 2026-07-15 - Atomic remote refresh

Added `registry refresh <key|path> [--force]` for managed remote connections.
Refresh protects locally modified caches by default, materializes and validates
a distinct generation, atomically replaces the exact registry snapshot, rolls
back the new generation on failure, and removes the previous cache only after
the switch succeeds. Source anchors:
`packages/cli/internal/okf/registry.go`,
`packages/cli/internal/okf/registry_test.go`,
`packages/cli/cmd/openknowledge/main.go`, and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Offline registry integrity status

Added `registry status [key|path] [--json]` with bundle validation, deterministic
content identity, provenance-sidecar comparison, Git commit and dirty-tree
checks, stable states and exit codes, a v1 JSON envelope, and a checked Draft
2020-12 schema. The command does not contact remotes. Source anchors:
`packages/cli/internal/okf/content_hash.go`,
`packages/cli/internal/okf/content_hash_test.go`,
`packages/cli/cmd/openknowledge/main.go`,
`packages/cli/cmd/openknowledge/main_test.go`, and
`packages/cli/schemas/v1/registry-status.schema.json`.

### 2026-07-15 - Safe managed cache deletion

Registry removal can now require an exact expected entry snapshot. Managed-file
deletion uses that guard together with the source cache lock, cache-boundary
validation, and reversible tombstoning of the full managed root. Source anchors:
`packages/cli/internal/okf/registry.go`,
`packages/cli/internal/okf/registry_test.go`,
`packages/cli/cmd/openknowledge/main.go`, and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Locked cache transactions

Remote cache reads and publication now share source-specific process and file
locks. Staged Git and archive replacements publish only after validation and
roll back on failure. Source anchors:
`packages/cli/cmd/openknowledge/main.go` and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Durable remote provenance

Registry source records now retain immutable content identity and the full
remote resolution chain. Remote cache paths are source-addressed rather than
alias-addressed, and a versioned cache sidecar preserves provenance across
cache hits. Source anchors: `packages/cli/internal/okf/registry.go`,
`packages/cli/internal/okf/registry_test.go`,
`packages/cli/cmd/openknowledge/main.go`, and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Atomic concurrent registry updates

Registry mutations now use both in-process and cross-process locking, load one
consistent snapshot, and atomically replace the owner-only registry file.
Managed-file deletion eligibility is checked inside the same removal
transaction. Source anchors: `packages/cli/internal/okf/registry.go`,
`packages/cli/internal/okf/registry_test.go`, and
`packages/cli/cmd/openknowledge/main.go`.

### 2026-07-15 - Positional-first connection flags

`registry connect` and `registry disconnect` now accept their documented
positional-first flag forms while preserving flag-first usage. The top-level
aliases use the same parser. Source anchors:
`packages/cli/cmd/openknowledge/main.go` and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-06-20

`openknowledge registry connect`, `openknowledge registry disconnect`, and
`openknowledge registry where` replaced the previous `registry add` and
top-level `where` surface. The top-level `connect` and `disconnect` commands
remain as aliases.

Registry storage now writes path-keyed `connections` and records access,
managed-file, and source metadata for local or remote connections.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/registry.go`
> * `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> Update this page when registry storage, key validation, output format,
> resolution behavior, or connection command aliases change.
