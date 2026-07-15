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
openknowledge registry connect <git-url> --git-ref <branch|tag|commit>
openknowledge registry connect <git-url> --git-subdir <path>
openknowledge registry connect <source> --no-validate
openknowledge registry disconnect <key|path>
openknowledge registry disconnect <key|path> --keep-files
openknowledge registry disconnect <key|path> --delete-files
openknowledge registry refresh <key|path>
openknowledge registry refresh <key|path> --force
openknowledge registry list
openknowledge registry list --json
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
| `key` | argument | Connection key beginning with an ASCII letter or digit, followed by letters, numbers, dots, underscores, or dashes; it must not look like a path. |
| `source` | argument | Knowledge bundle folder path, existing registry key, Open Knowledge manifest URL, tar archive URL, or Git URL. |
| `--as` | flag | Explicit connection key for `connect`. |
| `--access` | flag | Local authoring capability, `read` or `write`. Defaults to `read`; managed remote sources are always read-only. |
| `--git-ref` | flag | Git branch, tag, or commit to fetch instead of the remote default. Git sources only. |
| `--git-subdir` | flag | Canonical slash-separated OKF bundle root below the repository root. Git sources only. |
| `--no-validate` | flag | Skip validation status output for `connect`. |
| `--keep-files` | flag | Keep bundle files after `disconnect`; this is the default. |
| `--delete-files` | flag | Delete files only when the entry is marked `managed`. |
| `--force` | flag | Allow `refresh` to discard detected local cache changes. |
| `--json` | flag | Print the versioned registry inventory for `list` or integrity report for `status`. |

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

Git clone/fetch/checkout materialization is non-interactive and shares one
two-minute process budget. `GIT_TERMINAL_PROMPT=0` and
`GCM_INTERACTIVE=never` prevent credential helpers from waiting for input, and
captured stdout/stderr is capped at 256 KiB while the process continues to
drain output. Timeout or command failure leaves no published cache generation.
After each clone/init/fetch/checkout subprocess, the complete staging tree,
including Git object storage, is scanned with the same extracted-tree limits as
archive sources: at most 100,000 entries, 256 MiB per non-directory entry, and
2 GiB total. An over-limit generation is deleted before bundle validation,
content hashing, provenance writing, registry mutation, or cache publication.

Remote source URLs become durable registry and cache-sidecar provenance, so
validation runs before any I/O and rejects HTTP userinfo, URL passwords,
fragments, non-local `file://` hosts, and known credential-bearing query keys
such as access tokens and cloud signatures. Errors name only the rejected
field, never its value. Git authentication must use SSH keys or a credential
helper; HTTP manifests and archives must be directly accessible without
URL-embedded authentication. HTTP redirects retain the ten-hop limit, must
remain credential-free, and cannot downgrade an HTTPS request to HTTP.

Access is an enforced CLI capability rather than a display label. A `read`
connection can be browsed and inspected, but the viewer omits local editor
deeplinks and `rules apply` refuses to change files inside its registered root.
A local `write` connection enables those authoring paths. The most-specific
connection wins for nested roots, and canonical path checks prevent symlink
aliases from bypassing a read-only parent. Paths outside the registry are not
restricted. This is a product-level guard, not an operating-system ACL: other
programs can still modify files when filesystem permissions allow it.

Remote manifest, archive, and Git sources are immutable managed cache
generations and therefore accept only `read`. `--access write` is rejected
before remote materialization. Legacy or manually forged managed entries with
write access fail closed to `read` when loaded. Reconnecting the local path of
an existing managed cache preserves its source provenance and cannot promote it
to `write`.

`registry disconnect` removes entries by key or absolute path and keeps files
by default. `--delete-files` is guarded and only applies to CLI-managed remote
caches.

`registry refresh` applies only to managed remote connections. It inspects the
current cache first and refuses to discard detected content, Git, or provenance
changes unless `--force` is supplied. The command materializes and validates a
fresh cache generation at a new path, records the exact new source identity in
an atomic registry replacement, and only then deletes the previous generation.
A Git refresh retains the connection's recorded branch/tag/commit selector and
monorepo subdirectory; provenance and cache identity distinguish those
selectors from other views of the same repository URL.
A download, clone, validation, provenance-write, or registry-replacement failure
leaves the previously registered generation available. If the registry switch
succeeds but old-cache cleanup fails, the new generation remains active and the
command reports a warning with exit status `1`.

Connection, disconnection, refresh, and status flags may appear before or after
their required positional argument. The registry connection subcommands and
their top-level aliases share the applicable parsing contract.

`registry list` prints the registry file path and sorted entries. It does not
stat, parse, validate, hash, or otherwise inspect connected bundle contents.
`registry list --json` provides the same cheap discovery operation as a
`schemaVersion: "1"` envelope with the registry path and sorted connection
objects. Each object contains `name`, `path`, effective `access`, `managed`,
and optional remote `source` provenance. Use `registry status --json` when a
consumer also needs content, validation, Git, or cache-integrity state.
Git provenance includes the requested `gitRef` and `gitSubdir` when selected,
in addition to the resolved exact `gitCommit`.

The Draft 2020-12 discovery schema is
`packages/cli/schemas/v1/registry-list.schema.json`.

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

`openknowledge registry list --json` exposes the same registry without
terminal formatting or bundle-health work:

```json
{
  "schemaVersion": "1",
  "registry": "/home/user/.config/openknowledge/registry.json",
  "entries": [
    {
      "name": "personal",
      "path": "/work/project-memory",
      "access": "read",
      "managed": false
    }
  ]
}
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
key, access capability, optional source metadata, and whether the files are
CLI-managed. Local connections normally leave `managed` unset; remote manifest,
tar, and Git connections are marked managed because their files live in the
Open Knowledge cache.

Current registry files declare `schemaVersion: "1"`. The runtime accepts a
missing version only for legacy migration and the next successful atomic
mutation writes v1. It fails closed before mutation on unsupported versions,
unknown fields, duplicate object keys, trailing JSON, non-canonical or relative
stored paths, invalid keys/access values, and duplicate logical connection
names. The public Draft 2020-12 contract is
`https://openknowledge.sh/schemas/cli/storage/v1/registry.schema.json`.
Registry reads are capped at 8 MiB before decoding.

New remote source records preserve requested and resolved URLs, final manifest
and archive URLs, the archive SHA-256 or Git commit, concrete OKF spec, fetch
timestamp, deterministic managed-tree SHA-256, and the complete managed cache
root. `ref` remains populated for compatibility with older archive-URL readers.
Cache provenance is also stored in a versioned owner-only sidecar beside the
source-addressed cache directory, so reconnecting does not infer or lose the
source identity.
The sidecar uses
`https://openknowledge.sh/schemas/cli/storage/v1/cache-source.schema.json` and
the same strict JSON boundary. A recorded `managedRoot` that does not equal the
actual cache generation is corruption rather than a value the loader silently
replaces; offline status therefore reports the provenance mismatch. Sidecar
reads are capped at 1 MiB.

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

### 2026-07-15 - Bounded Git staging generations

Remote Git staging trees now receive entry-count, single-file, and aggregate
byte checks after every Git subprocess. Limits match archive extraction at
100,000 entries, 256 MiB per file, and 2 GiB total, include `.git` objects, and
fail before validation, hashing, provenance, or publication. Source anchors:
`packages/cli/cmd/openknowledge/main.go` and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Secret-safe remote source URLs

Connection, refresh, manifest archive resolution, and HTTP redirects now share
pre-I/O URL validation. Durable provenance cannot contain URL userinfo,
passwords, fragments, or recognized credential query parameters; rejection
does not echo secret values. HTTPS redirects cannot downgrade to HTTP, and the
default ten-hop redirect bound remains explicit. Source anchors:
`packages/cli/cmd/openknowledge/main.go` and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Bounded non-interactive Git transport

Remote Git clone/fetch/checkout now shares a two-minute deadline, disables
terminal and Git Credential Manager interaction, and drains subprocess output
through a 256 KiB diagnostic cap. Timed-out or failed materializations remain
transactional and do not publish a partial cache. Source anchors:
`packages/cli/cmd/openknowledge/main.go` and
`packages/cli/cmd/openknowledge/main_test.go`.

### 2026-07-15 - Versioned strict persistence

Registry writes now declare storage schema v1, legacy unversioned files migrate
on their next mutation, and registry/sidecar readers reject ambiguous,
extended, unsupported, or invariant-breaking state without rewriting it.
Public Draft 2020-12 storage schemas ship with the website. Source anchors:
`packages/cli/internal/okf/strict_json.go`,
`packages/cli/internal/okf/registry.go`,
`packages/cli/cmd/openknowledge/main.go`, and
`packages/cli/schemas/storage/v1/`.

### 2026-07-15 - Versioned registry discovery

Added `registry list --json` as a versioned, schema-backed discovery API for
sorted connections, effective access, managed state, and optional source
provenance. The list operation deliberately remains cheaper than
`registry status`: it does not inspect bundle contents. Added dedicated
`registry list --help` output and strict rejection of positional arguments.
Source anchors:
`packages/cli/cmd/openknowledge/main.go`,
`packages/cli/cmd/openknowledge/main_test.go`, and
`packages/cli/schemas/v1/registry-list.schema.json`.

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
