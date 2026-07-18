---
type: Feature Documentation
title: Installation
description: Install and verify the Open Knowledge CLI.
tags: [openknowledge, cli, installation]
timestamp: 2026-07-18T00:00:00Z
---

# Installation

Installed releases expose both `openknowledge` and the shorter `okn` alias.
They run the same CLI; the examples below keep the descriptive command name.

## Shell installer

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

The installer supports macOS and Linux on `amd64` and `arm64`. It downloads the
matching release archive, verifies its SHA-256, probes the staged binary with
`openknowledge version`, atomically replaces the destination, and creates
`okn` as a relative symlink. It refuses to replace an unrelated existing
`okn` command. Existing binaries survive failed downloads, checks, or probes.

| Variable | Default | Purpose |
| --- | --- | --- |
| `OPENKNOWLEDGE_REPO` | `openknowledge-sh/openknowledge` | Release repository. |
| `OPENKNOWLEDGE_VERSION` | `latest` | Release version; an optional leading `v` is normalized. |
| `OPENKNOWLEDGE_BASE_URL` | GitHub Releases | Asset base URL; `file://` is accepted for controlled local tests. |
| `OPENKNOWLEDGE_INSTALL_DIR` | `$HOME/.local/bin` | Destination directory. |

For a stronger origin check, download an archive and verify its GitHub/Sigstore
attestation:

```sh
gh attestation verify openknowledge_linux_amd64.tar.gz \
  -R openknowledge-sh/openknowledge
```

If piping a remote script is outside your trust policy, download and inspect
`install` first, then run it locally. The archive is still checksum-verified.

## npm

```sh
npm install -g @openknowledge-sh/openknowledge
```

The npm package registers both command names and downloads the binary matching
the package version. It supports the release platforms, including Windows
assets when available, and applies bounded HTTPS redirects, download and
expansion limits, exact checksum lookup, strict tar member validation, and
atomic publication.

| Variable | Default | Purpose |
| --- | --- | --- |
| `OPENKNOWLEDGE_REPO` | `openknowledge-sh/openknowledge` | Release repository. |
| `OPENKNOWLEDGE_VERSION` | npm package version | Binary version. |
| `OPENKNOWLEDGE_SKIP_DOWNLOAD` | unset | Set to `1` for packaging or source-workspace checks. |

## From source

```sh
pnpm install --frozen-lockfile
pnpm build:cli
./bin/openknowledge version
```

The release workflow publishes npm only after matching GitHub Release assets
exist and versions align. Shell and npm installation behavior is covered by
offline transactional tests in the root `pnpm test` gate.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `install`
> - `packages/npm/install.js`
> - `packages/npm/install.test.js`
> - `.github/workflows/release.yml`
> - `scripts/test-install.sh`
>
> **Update notes**
>
> Update this page when platforms, variables, verification, or release asset
> behavior changes.
