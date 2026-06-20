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
navigation before the GitHub icon. A latest-release badge sits below that
topbar, links to GitHub Releases, and hydrates at runtime from GitHub's latest
release API so the displayed tag and relative publish age stay current.

The web server redirects `/install` and `/install/` to
`https://github.com/openknowledge-sh/openknowledge/releases/latest/download/install`.
Keep this redirect in `packages/web/scripts/serve.mjs` because Railway serves
the site through the Node server.

The landing page includes Google Analytics through `gtag.js` with measurement
ID `G-62SWM7FC2J`. Keep the tag in `packages/web/index.html` so `pnpm build:web`
copies it into `packages/web/dist/index.html`.

`pnpm dev:web` serves source files from `packages/web` by default, refreshes the
wiki export on startup, and then falls back to `packages/web/dist/wiki` for
`/wiki/` URLs. Set `OPENKNOWLEDGE_WEB_EXPORT_WIKI=0` only when you intentionally
want to skip that startup export.

Both `pnpm build:web` and `pnpm dev:web` run the exporter through the current Go
source by default with `go run ./packages/cli/cmd/openknowledge`, so local
viewer changes are reflected in the exported wiki without requiring a rebuilt
`bin/openknowledge`. Set `OPENKNOWLEDGE_BIN=/path/to/openknowledge` to test a
specific binary intentionally.

The wiki export reads `Wiki/openknowledge.toml` and copies
`Wiki/assets/openknowledge-site.css` into the generated output. Keep that theme
CSS aligned with `packages/web/styles.css` when changing the landing page
palette, fonts, or core spacing. The same TOML also sets `[html.source]` with
`github_base = "https://github.com/openknowledge-sh/openknowledge/blob/main"`
and `entry = "Wiki"`, so deployed wiki panels link back to their Markdown
source files on GitHub instead of showing local editor deeplinks.

The web server keeps canonical generated wiki pages under their exported paths,
such as `/wiki/features/commands/disconnect.html`, and redirects short top-level
command aliases such as `/wiki/disconnect.html` and `/wiki/disconnect` to those
canonical pages after checking for real static files.

The Railway deployment workflow runs on pushes to `main`. It first verifies the
repository with `pnpm test` and `pnpm build`, then deploys through the Railway
CLI container with `railway up --ci --service="$RAILWAY_SERVICE"`. Configure
`RAILWAY_TOKEN` as a repository secret and `RAILWAY_SERVICE` as the Railway
service name or service ID. `RAILWAY_PROJECT_ID` is optional, but should be set
when the token is not already scoped tightly enough to the target project. When
`RAILWAY_PROJECT_ID` is set, Railway also requires an environment; the workflow
uses `RAILWAY_ENVIRONMENT` with a default of `production`. Override it with the
exact Railway environment name or ID if the project uses a different
environment. The workflow still accepts the older `RAILWAY_SERVICE_ID` name as a
fallback, but it must contain a service name or service ID, not a project ID.

`railway.json` keeps Railway build and runtime settings in code and tells
Railway to use the repository `Dockerfile`. The Docker build installs both Go
and Node/pnpm because `pnpm build:web` exports the wiki by running the current
Go CLI source. The runtime image copies only `packages/web/dist` and the web
server script, then starts `node packages/web/scripts/serve.mjs`. Runtime env in
the Dockerfile serves `packages/web/dist` without re-exporting the wiki and
binds to `0.0.0.0` so the Railway router can reach the container.

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
* `.github/workflows/deploy-railway.yml`
* `.github/workflows/release.yml`
* `.dockerignore`
* `Dockerfile`
* `.goreleaser.yaml`
* `railway.json`
* `install`
* `packages/npm/package.json`
* `packages/web/package.json`
* `packages/web/scripts/build.mjs`
* `packages/web/scripts/wiki-export.mjs`
* `packages/web/scripts/serve.mjs`
* `packages/web/index.html`
* `packages/web/main.js`
* `Wiki/openknowledge.toml`
* `Wiki/assets/openknowledge-site.css`

## Update Notes

When workspace scripts, deployment workflow behavior, release workflow behavior,
GoReleaser outputs, npm publish behavior, or local release testing changes,
update this page and [CLI changelog](/changelog/cli.md).
