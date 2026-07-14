---
type: Feature Documentation
title: Installation
description: Developer notes for installing the Open Knowledge CLI.
tags: [openknowledge, cli, installation]
timestamp: 2026-06-18T00:00:00Z
---

# Installation

The CLI can be installed through the shell installer or the npm package wrapper.
The installer path is part of the initial agent setup prompt and should stay
accurate because users copy it directly into coding agents.

## User-Facing Entry Points

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

The npm package lives under `packages/npm` and downloads the matching release
binary during installation. The shell installer lives at `install`.

The release workflow publishes the npm wrapper only after the matching GitHub
Release artifacts exist. The wrapper version and release tag are checked before
tag creation, and npm provenance is attached during publish.

## Installer Options

| Name | Applies To | Default | Description |
| --- | --- | --- | --- |
| `OPENKNOWLEDGE_REPO` | shell, npm | `openknowledge-sh/openknowledge` | GitHub repository used for release assets. |
| `OPENKNOWLEDGE_VERSION` | shell, npm | `latest` for shell, npm package version for npm | Release version to download. Values without `v` are normalized for tagged releases. |
| `OPENKNOWLEDGE_BASE_URL` | shell | GitHub release download URL | Override release asset base URL, mainly for local release testing. |
| `OPENKNOWLEDGE_INSTALL_DIR` | shell | `$HOME/.local/bin` | Destination directory for the installed binary. |
| `OPENKNOWLEDGE_SKIP_DOWNLOAD` | npm | unset | Set to `1` to skip npm binary download. Source workspaces also skip automatically. |

The shell installer supports macOS and Linux on `amd64` and `arm64`, downloads
the matching tarball plus `checksums.txt`, verifies the archive checksum, and
installs the `openknowledge` binary. The npm installer also supports Windows
binary names when release assets are available.

`https://openknowledge.sh/install` should serve this repository's `install`
script.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `install`
> * `packages/npm/install.js`
> * `packages/npm/package.json`
> * `.github/workflows/release.yml`
> * `scripts/check-versions.mjs`
> * `README.md`
>
> **Update notes**
>
> When installer behavior, release asset names, npm package behavior, or
> environment variables change, update this page and [CLI changelog](/changelog/cli.md).
