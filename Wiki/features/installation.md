---
type: Feature Documentation
title: Installation
description: Install and verify the Open Knowledge CLI.
tags: [openknowledge, cli, installation]
timestamp: 2026-07-28T00:00:00Z
---

# Installation

Installed releases expose both `openknowledge` and the shorter `okn` alias.
Both names run the same CLI. The documentation uses `okn` for user commands.

## Shell installer

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

The installer supports macOS and Linux on `amd64` and `arm64`.
It downloads the applicable release archive and verifies its SHA-256.
It tests the staged binary with `openknowledge version`.
The preflight test suppresses telemetry and does not create a telemetry ID.
It then replaces the destination and creates `okn` as a relative symbolic link.
It does not replace an unrelated `okn` command.
A failed download, check, or test does not remove an existing binary.

| Variable | Default | Purpose |
| --- | --- | --- |
| `OPENKNOWLEDGE_REPO` | `openknowledge-sh/openknowledge` | Release repository. |
| `OPENKNOWLEDGE_VERSION` | `latest` | Release version. The installer accepts an optional leading `v`. |
| `OPENKNOWLEDGE_BASE_URL` | GitHub Releases | HTTPS asset base URL. The installer accepts `file://` only for controlled local tests. It rejects plain HTTP. |
| `OPENKNOWLEDGE_INSTALL_DIR` | `$HOME/.local/bin` | Destination directory. |

For an additional origin check, download an archive.
Then, verify its GitHub or Sigstore attestation:

```sh
gh attestation verify openknowledge_linux_amd64.tar.gz \
  -R openknowledge-sh/openknowledge
```

If your trust policy prohibits a remote script pipe, download the `install` file.
Inspect the file.
Then, run it locally.
The installer still verifies the archive checksum.

## npm

```sh
npm install -g @openknowledge-sh/openknowledge
```

The npm package registers both command names.
It downloads the binary that matches the package version.
It supports all release platforms, including Windows assets when available.
It limits HTTPS redirects, download size, and expanded size.
It also requires an exact checksum and validates each tar member.
It publishes the binary atomically.

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

The release workflow publishes npm only when the applicable GitHub Release
assets exist. The package versions must also match.
Offline transactional tests cover shell and npm installation.
The root `pnpm test` gate runs these tests.

CLI telemetry is disabled by default. The CLI sends no event and creates no
installation ID before you run `okn telemetry enable`. See
[Product Telemetry and Privacy](telemetry.md).

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `install`
> - `packages/npm/install.js`
> - `packages/npm/install.test.js`
> - `packages/cli/internal/telemetry/`
> - `.github/workflows/release.yml`
> - `scripts/test-install.sh`
>
> **Update notes**
>
> Update this page after a change to a platform, variable, verification method,
> or release asset.
