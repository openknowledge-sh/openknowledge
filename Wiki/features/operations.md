---
type: Feature Documentation
title: CLI Operations
description: Developer operations notes for working on and releasing the Open Knowledge CLI.
tags: [openknowledge, cli, operations, release]
timestamp: 2026-06-20T00:00:00Z
---

# CLI Operations

This page holds developer-facing operational details that do not belong in the
product-oriented root README or command-specific reference pages.

## Workspace

```text
packages/cli  - Go CLI
packages/npm  - npm wrapper for the release binary
packages/web  - static HTML/CSS site
```

## Development Commands

```sh
pnpm test:cli
pnpm build:cli
pnpm build:web
pnpm dev:web
```

The root `package.json` maps those commands to the Go CLI package and web
workspace. `pnpm test` currently runs the CLI test suite, and `pnpm build`
builds both the CLI and web package.

## Project Website

`pnpm build:web` writes the landing page into `packages/web/dist` and then runs
the Open Knowledge HTML exporter for this repository wiki:

```sh
openknowledge to html --out packages/web/dist/wiki Wiki
```

That makes the public website's `wiki/` path a static viewer export of the
colocated `Wiki/` bundle. The landing page links to that output from the top
navigation before the GitHub icon.

`pnpm dev:web` serves source files from `packages/web` by default, but falls
back to `packages/web/dist/wiki` for `/wiki/` URLs when a generated wiki export
exists. Run `pnpm build:web` after wiki or theme changes before checking
`http://127.0.0.1:4173/wiki/` through the dev server.

The wiki export reads `Wiki/openknowledge.toml` and copies
`Wiki/assets/openknowledge-site.css` into the generated output. Keep that theme
CSS aligned with `packages/web/styles.css` when changing the landing page
palette, fonts, or core spacing.

## Release

GitHub Releases are the source of truth for downloadable binaries. Run the
release manually from GitHub Actions:

```text
Actions -> Release -> Run workflow -> version: 0.1.0
```

The release workflow normalizes the input to a `v*` tag, validates the version
shape, creates and pushes the tag when needed, then runs GoReleaser. If the tag
already exists, it must point at the workflow commit; otherwise the workflow
fails before creating or replacing a release.

GoReleaser uploads the installer, checksums, license files, third-party
notices, and platform archives to GitHub Releases.

The npm publishing job is present in the workflow as commented YAML, but it is
disabled while the GitHub Release artifact flow is validated first. When
re-enabling npm publishing:

* set `packages/npm/package.json` `version` to match the tag without the
  leading `v`;
* configure the repository `NPM_TOKEN` secret with permission to publish
  `@openknowledge-sh/openknowledge`.

The commented npm publish job is designed to fail fast when the package version
does not match the tag or `NPM_TOKEN` is missing.

Local installer test against a directory of release assets:

```sh
OPENKNOWLEDGE_BASE_URL=file:///tmp/openknowledge-release \
OPENKNOWLEDGE_INSTALL_DIR=/tmp/openknowledge-bin \
bash install
```

Manual npm publish fallback after the matching GitHub Release exists:

```sh
cd packages/npm
npm publish --access public
```

## Source Anchors

* `package.json`
* `pnpm-workspace.yaml`
* `.github/workflows/release.yml`
* `.goreleaser.yaml`
* `install`
* `packages/npm/package.json`
* `packages/web/scripts/build.mjs`
* `packages/web/index.html`
* `Wiki/openknowledge.toml`
* `Wiki/assets/openknowledge-site.css`

## Update Notes

When workspace scripts, release workflow behavior, GoReleaser outputs, npm
publish behavior, or local release testing changes, update this page and
[CLI changelog](/changelog/cli.md).
