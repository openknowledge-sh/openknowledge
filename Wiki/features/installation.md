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

The npm downloader accepts only credential-free HTTPS and follows at most five
HTTPS redirects with a 30-second inactivity timeout. Release archives are
limited to 64 MiB and `checksums.txt` to 1 MiB before buffering. Checksum lookup
requires one exact asset-name entry with a 64-character SHA-256 rather than a
filename suffix match.

The compressed archive may expand to at most 256 MiB. Tar headers, member
checksums, octal sizes, and bounds are validated before use; the archive must
contain exactly one root-level regular member with the platform binary name,
and that binary may be at most 128 MiB. Nested paths, basename-only matches,
special members, duplicates, truncation, and decompression bombs fail closed.
The npm installer writes to a unique same-directory staging file and atomically
renames it into `vendor/`, cleaning the staging file after either success or
failure.

The shell path validates explicit versions before constructing a release URL,
requires an exact 64-character archive checksum entry, and uses bounded curl
connection/runtime retries. Default GitHub downloads require HTTPS and TLS 1.2
or newer; an explicit `OPENKNOWLEDGE_BASE_URL` may use `file://` for local
release tests and is therefore treated as a caller-controlled trust override.

After checksum verification, the installer streams only the named
`openknowledge` member out of the tar archive instead of extracting arbitrary
archive paths. It copies that candidate to a unique same-filesystem staging
file, executes `openknowledge version` there, requires a semantic version, and
matches an explicitly requested release exactly. Only then does an atomic
rename replace the destination. Checksum, archive, execution, version, or
destination-type failures preserve an existing binary and clean up staging;
directories and other non-file destinations are refused.

`pnpm test:install` exercises successful replacement, checksum mismatch and
syntax failure, requested/downloaded version rejection, missing archive
members, invalid destination types, preservation of the old binary, and
staging cleanup. It runs as part of the root `pnpm test` gate used by CI, deploy
verification, and release preflight.

`pnpm test:npm-install` uses offline response and tar fixtures to cover declared
and streaming byte limits, redirect ceilings and HTTPS downgrades, exact and
unambiguous checksums, decompression bounds, member type/path/size/duplicate
rejection, and atomic publication. It runs in the same root test gate.

The `curl | bash` entry point authenticates transport to the configured host
but cannot checksum the installer script before executing it. Users requiring
an offline trust ceremony should download and inspect `install` first, then run
it locally; release archives are still checksum-verified by the script.

`https://openknowledge.sh/install` should serve this repository's `install`
script.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `install`
> * `packages/npm/install.js`
> * `packages/npm/install.test.js`
> * `packages/npm/package.json`
> * `.github/workflows/release.yml`
> * `scripts/check-versions.mjs`
> * `scripts/test-install.sh`
> * `README.md`
>
> **Update notes**
>
> When installer behavior, release asset names, npm package behavior, or
> environment variables change, update this page and [CLI changelog](/changelog/cli.md).
