---
type: Command Documentation
title: openknowledge connect
description: Register a local or remote knowledge base under a stable key.
tags: [openknowledge, cli, command, registry, connect]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge connect`

Add a local or remote bundle to the registry. Other commands can then use the
assigned key instead of a path or URL.

## Usage

```sh
okn connect <source>
okn connect <source> --as <key>
okn connect ./Wiki --access write
okn connect <git-url> --git-ref <ref> --git-subdir <path>
```

| Option | Default | Description |
| --- | --- | --- |
| `source` | required | Local directory, registry key, manifest URL, tar URL, or Git URL. |
| `--as <key>` | bundle/folder name | Explicit registry key. |
| `--access read|write` | `read` | Local authoring capability. Remotes are always read-only. |
| `--git-ref <ref>` | remote default | Branch, tag, or commit for Git sources. |
| `--git-subdir <path>` | repository root | Bundle root inside a Git repository. |
| `--no-validate` | off | Omit validation status from success output. |

A key starts with an ASCII letter or digit. It can contain letters, numbers,
dots, underscores, and dashes. An implicit collision adds a numeric suffix. An
explicit collision stops the command.

## Source resolution

| Source | Behavior |
| --- | --- |
| Local directory | Register the resolved directory directly. |
| `file://` URL | Read a local manifest or archive, or clone a local Git source. Use `file:///C:/path` for a Windows drive. |
| Website URL | Try `openknowledge.json` at the path, then `/.well-known/`. |
| Manifest | Verify its strict v1 contract, archive digest, and declared OKF spec. |
| `.tar`, `.tar.gz`, `.tgz` URL | Download and safely extract the archive. |
| Other HTTP(S) or Git URL | Perform a shallow Git materialization. |

The command stages remote materializations in the Open Knowledge cache. It
publishes them atomically after validation.

Before registration, the command validates archive paths, symlinks, extraction
sizes, Git selectors, and bundle boundaries. Cache provenance records final
URLs, selectors, tree digest, and fetch time. It also records an archive digest
or Git commit.

The normalized source and Git selectors define the cache identity. The
registry key does not define it. A repeated connection can reuse its validated
materialization.

`connect` does not inspect a reused cache for remote updates. Run
[`okn registry refresh`](registry.md) to get a new generation.

## Access and validation

Local `read` connections hide editor links and block maintenance rule writes.
`write` enables these CLI authoring functions. This setting does not change
operating system permissions. It does not constrain other tools. A managed
remote is always read-only.

Validation reports `valid`, `warnings`, `invalid`, or `unknown`. Validation
does not prevent a connection. Root `okf_bundle_*` metadata can supply the
default key, display title, purpose, and named entrypoints.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/main.go`
> - `packages/cli/internal/okf/registry.go`
> - `packages/cli/internal/okf/registry_test.go`
>
> **Update notes**
>
> Update this page when source resolution, cache provenance, access, or key
> behavior changes.
