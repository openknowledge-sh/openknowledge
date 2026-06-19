---
type: Candidate Command Documentation
title: openknowledge disconnect
description: Candidate specification for removing connected knowledge bundles from the local registry.
tags: [openknowledge, cli, command, registry, disconnect, candidate]
timestamp: 2026-06-19T00:00:00Z
status: candidate
---

# `openknowledge disconnect`

`openknowledge disconnect` is the candidate user-facing command for removing a
bundle from the local Open Knowledge connections registry.

It should replace normal use of low-level registry removal commands. The
command unregisters the connection first and only deletes files when explicitly
asked.

## Candidate Usage

```sh
openknowledge disconnect <key-or-path>
openknowledge disconnect <key-or-path> --delete-files
openknowledge disconnect <key-or-path> --keep-files
openknowledge disconnect --help
```

## Resolution

`key-or-path` resolves through the same connection registry as `where`, `list`,
`open`, and candidate `use`:

* if it matches a key, disconnect that connection;
* if it looks like a path, normalize it to an absolute path and disconnect that
  exact path when present;
* if no connection matches, fail clearly and print available keys.

Path resolution must use absolute paths because agents may run from different
working directories.

## File Deletion

Default behavior should be conservative:

* Local user-owned paths are never deleted by default.
* Managed remote caches are not deleted unless the user passes
  `--delete-files`.
* `--keep-files` is explicit no-delete behavior and useful for scripts.
* `--delete-files` is valid only for managed paths under Open Knowledge's cache
  root, unless a future interactive confirmation deliberately supports broader
  deletion.

## Output

```text
Disconnected knowledge bundle
key    accessibility
path   /Users/me/.openknowledge/bundles/accessibility
files  kept
```

With deletion:

```text
Disconnected knowledge bundle
key    accessibility
path   /Users/me/.openknowledge/bundles/accessibility
files  deleted
```

## Failure Cases

`disconnect` should fail when:

* the key is unknown;
* the path is not connected;
* `--delete-files` targets a non-managed path;
* files cannot be deleted after the registry update. In that case, print a
  warning with the path so the user can clean up manually.

## Relationship To Registry

The internal registry store remains the persistence layer. `disconnect` is the
user-facing command that removes one connection from it. If a compatibility
`registry remove` command exists later, it should be an alias for unregistering
only and should never delete files.

## Update Notes

When implemented, update root help, `README.md`, [connect](connect.md),
[list](list.md), [where](where.md), tests, and
[CLI changelog](/changelog/cli.md).
