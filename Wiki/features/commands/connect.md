---
type: Command Documentation
title: openknowledge connect
description: Connects a local OKF bundle to the user's local knowledge registry.
tags: [openknowledge, cli, command, registry, connect, agent]
timestamp: 2026-06-20T00:00:00Z
---

# `openknowledge connect`

`openknowledge connect` adds a local Open Knowledge bundle to the user's
registry so later commands can refer to it by a stable key. It is syntactic
sugar for `openknowledge registry connect`, with the same parsing, output, and
exit-code behavior as the registry subcommand.

The shipped implementation connects existing local directories. Remote URL
materialization is still planned work; clone a remote bundle locally before
connecting it.

## Usage

```sh
openknowledge connect <path>
openknowledge connect <path> --as <key>
openknowledge connect <path> --access read|write
openknowledge connect <path> --no-validate
openknowledge connect --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `path` | argument | Local bundle root. Existing registry names are accepted and resolve to their stored path. |
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

Root metadata keys used by `connect`:

| Key | Meaning |
| --- | --- |
| `okf_bundle_name` | Preferred default key when `--as` is omitted. |
| `okf_bundle_title` | Display name in success output. |
| `okf_bundle_purpose` | Purpose shown in success output. |
| `okf_bundle_tags` | Discovery tags parsed from a YAML flow list. |
| `okf_bundle_entry_<name>` | Entrypoint names listed in success output. |

Missing metadata does not block connection. Display names fall back to the root
`index.md` H1, then the folder name. Without metadata, `connect` still stores
the key, absolute path, and access label.

Connecting the same absolute path updates the existing connection. If an
implicit key collides with another path, `connect` chooses the next available
suffix such as `project-2` and prints a warning. If an explicit `--as <key>`
collides with another path, the command fails.

Validation is a status signal, not a connection gate:

| Status | Meaning |
| --- | --- |
| `valid` | No validation errors or warnings. |
| `warnings` | Validation warnings were found. |
| `invalid` | Validation errors were found. |
| `unknown` | Validation was skipped or could not run. |

## Quick Examples

```sh
openknowledge connect ./project-memory
openknowledge connect ./accessibility --as accessibility
openknowledge connect ./team-wiki --access write
openknowledge registry where accessibility
openknowledge use accessibility --info
```

## Caveats

Remote URL sources are not supported yet. `connect` rejects `http://`,
`https://`, and `git@` inputs with a message telling the user to connect a local
clone.

The registry storage remains the existing `entries` JSON array, with optional
`access` and `managed` fields on entries. A path-keyed storage model remains a
future migration candidate.

## Source Anchors

* `packages/cli/cmd/openknowledge/main.go`
* `packages/cli/internal/okf/registry.go`
* `packages/cli/internal/okf/metadata.go`
* `packages/cli/internal/okf/registry_test.go`
* `packages/cli/internal/okf/metadata_test.go`

## Command Change History

### 2026-06-20

`openknowledge connect` became the top-level alias for the registry `connect`
subcommand after connection management moved under the registry namespace.

`openknowledge connect` shipped for local directories with `--as`,
`--access`, and `--no-validate`, metadata-derived keys, validation status
output, implicit key suffixing, and explicit collision failures.

## Update Notes

Update this page when connection storage, remote-source materialization, key
derivation, validation status semantics, or success output changes. CLI
behavior changes also require [CLI changelog](/changelog/cli.md) updates.
