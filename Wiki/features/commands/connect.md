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

Remote source handling:

* Open Knowledge manifest URLs are JSON documents with type
  `openknowledge.bundle`, an archive path, and optional `archiveSha256`.
* Website URLs try `openknowledge.json` under the URL path, then
  `/.well-known/openknowledge.json`.
* Direct `.tar`, `.tar.gz`, and `.tgz` URLs are downloaded and extracted.
* HTTP(S) URLs that are neither manifests nor archives fall back to shallow
  `git clone`.

Downloaded archives are extracted into the Open Knowledge cache using safe path
checks, then validated as Open Knowledge bundles before registration.

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
openknowledge connect https://example.com/openknowledge-bundle.tar.gz
openknowledge connect https://github.com/openknowledge-sh/accessibility.git --as accessibility
```

## Caveats

Remote archive and manifest sources require network access for non-local URLs.
Git fallback requires `git` on `PATH`. Existing cached materializations are
reused when they still validate. See [registry](registry.md) for storage
compatibility details.

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

`openknowledge connect` now materializes Open Knowledge manifests, tar archives,
and Git remote sources into the Open Knowledge cache, records source metadata,
and stores registry state as path-keyed `connections` while preserving reads of
legacy `entries` registries.

## Update Notes

Update this page when connection storage, remote-source materialization, key
derivation, validation status semantics, or success output changes. CLI
behavior changes also require [CLI changelog](/changelog/cli.md) updates.
