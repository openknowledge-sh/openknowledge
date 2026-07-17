---
type: Command Documentation
title: openknowledge connect
description: Register a local or remote knowledge base under a stable key.
tags: [openknowledge, cli, command, registry, connect]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge connect`

Add an existing local or remote bundle to the registry. Later commands can use
the assigned key instead of a filesystem path or URL.

## Usage

```sh
openknowledge connect <source>
openknowledge connect <source> --as <key>
openknowledge connect ./Wiki --access write
openknowledge connect <git-url> --git-ref <ref> --git-subdir <path>
```

| Option | Default | Description |
| --- | --- | --- |
| `source` | required | Local directory, registry key, manifest URL, tar URL, or Git URL. |
| `--as <key>` | bundle/folder name | Explicit registry key. |
| `--access read|write` | `read` | Local authoring capability; remotes are always read-only. |
| `--git-ref <ref>` | remote default | Branch, tag, or commit for Git sources. |
| `--git-subdir <path>` | repository root | Bundle root inside a Git repository. |
| `--no-validate` | off | Omit validation status from success output. |

Keys begin with an ASCII letter or digit and may contain letters, numbers,
dots, underscores, and dashes. Implicit collisions receive a numeric suffix;
explicit collisions fail.

## Source resolution

| Source | Behavior |
| --- | --- |
| Local directory | Register the resolved directory directly. |
| Website URL | Try `openknowledge.json` at the path, then `/.well-known/`. |
| Manifest | Verify its strict v1 contract, archive digest, and declared OKF spec. |
| `.tar`, `.tar.gz`, `.tgz` URL | Download and safely extract the archive. |
| Other HTTP(S) or Git URL | Perform a shallow Git materialization. |

Remote materializations are staged under the Open Knowledge cache and
published atomically after validation. Archive paths, symlinks, extraction
sizes, Git selectors, and bundle boundaries are checked before registration.
Managed cache provenance records final URLs, archive digest or Git commit,
selectors, tree digest, and fetch time.

Cache identity comes from the normalized source plus Git selectors, not the
registry key. Reconnecting the same selected source can therefore reuse its
validated materialization. `connect` does not check a reused cache for remote
freshness; run [`openknowledge registry refresh`](registry.md) to fetch a new
generation.

## Access and validation

Local `read` connections hide editor links and block maintenance-rule writes;
`write` enables those CLI authoring surfaces. This capability does not change
operating-system permissions or constrain other tools. Managed remotes cannot
be writable.

Validation is reported as `valid`, `warnings`, `invalid`, or `unknown`, but is
not a connection gate. Root `okf_bundle_*` metadata supplies the default key,
display title, purpose, and named entrypoints when present.

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
