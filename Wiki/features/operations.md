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
pnpm test:install
pnpm test:web
pnpm check:versions
pnpm check:workflow-pins
pnpm check:workflow-secret-scope
pnpm check:workflow-permissions
pnpm check:container-runtime
pnpm build:cli
pnpm build:web
pnpm dev:web
```

The root `package.json` is the release-version source of truth and maps those
commands to the Go CLI package and web workspace. `pnpm check:versions` verifies
that the root, npm wrapper, web package, and Go CLI fallback versions match.
`pnpm check:workflow-pins` rejects remote GitHub Actions that do not use a full
immutable commit SHA. `pnpm check:workflow-secret-scope` rejects repository
secrets outside an explicit consuming step and forbids blanket
`secrets: inherit` forwarding. `pnpm check:workflow-permissions` permits write
capabilities only on reviewed publish jobs and locks the minimal GitHub release
step set. `pnpm test` runs all workflow and version checks before the CLI test
suite. `pnpm check:container-runtime` requires the final Node image to select
its built-in unprivileged user. The root test gate also runs transactional
shell-installer fixtures, and `pnpm build` builds both the CLI and web package.
`pnpm test:web` exercises the production static handler without binding a
network socket, so the same checks run in sandboxed agents.

## Continuous Integration

`.github/workflows/ci.yml` is the required repository quality gate for pull
requests and pushes to `main`; it can also be dispatched manually. The workflow
uses read-only repository permissions, cancels superseded runs for the same PR
or ref, and has a finite job timeout. Every remote action is pinned to a full
commit SHA with its release version retained as an update hint, and the test
gate prevents a mutable action tag from being reintroduced. It performs a
frozen dependency install, checks that Go modules are tidy, runs version
alignment and the full CLI test suite, runs `go vet`, builds the CLI and
website, validates `Wiki/` with the newly built binary, and fails if those
operations leave generated files or module metadata out of date.

Configure the GitHub `main` branch protection rule to require the workflow's
`CI / verify` check before merge. The workflow provides the check; repository
branch-protection settings remain an administrator-controlled GitHub setting.

## Project Website

`pnpm build:web` writes the landing page into `packages/web/dist` and then runs
the Open Knowledge HTML exporter for this repository wiki:

```sh
openknowledge to html --head-html '<landing analytics head HTML>' --out packages/web/dist/wiki Wiki
```

That makes the public website's `wiki/` path a static viewer export of the
colocated `Wiki/` bundle. The web export extracts the Google Analytics
`gtag.js` block from `packages/web/index.html` and injects that same trusted
head HTML into every generated wiki page, keeping the landing page as the
single source for the measurement ID.

`pnpm build:web` can also inject trusted HTML into the generated landing page
`<head>`. Use this for analytics, verification meta tags, or small loader
scripts without hard-coding a provider into the repository:

```sh
OPENKNOWLEDGE_HEAD_FILE=./head.html pnpm build:web
OPENKNOWLEDGE_HEAD_HTML='<meta name="..." content="...">' pnpm build:web
OPENKNOWLEDGE_SCRIPT_SRC=/analytics.js pnpm build:web
```

`OPENKNOWLEDGE_SCRIPT_SRC` accepts comma- or newline-separated values. Script
URLs may be relative, `http:`, or `https:`.

`openknowledge to html` supports the same trusted head injection flags and
environment variables for default viewer exports. Use `--head-file`,
`--head-html`, or repeatable `--script-src` when another deployed wiki needs its
own analytics or verification tags.

The web server redirects `/install` and `/install/` to
`https://github.com/openknowledge-sh/openknowledge/releases/latest/download/install`.
Keep this redirect in `packages/web/scripts/serve.mjs` because Railway serves
the site through the Node server.

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
The deployed wiki brand is controlled by `Wiki/index.md` root frontmatter
`okf_bundle_title`, currently `Open Knowledge CLI Documentation`.

The web server keeps canonical generated wiki pages under their exported paths
and redirects short top-level command aliases to those canonical pages after
checking for real static files.

The production static handler accepts only `GET` and `HEAD`, returns explicit
`400`, `403`, `404`, `405`, and `500` responses, drains rejected request bodies,
implements bodyless HEAD responses with the real content length, and treats
redirects and errors as `no-store`. HTML revalidates on each visit while other
assets use a short five-minute cache with stale revalidation.

Every application response carries `nosniff`, deny-framing, strict-origin
referrer, opener isolation, HSTS, a restrictive permissions policy, and a CSP
that denies objects and framing while allowing the existing inline viewer code
and HTTPS analytics/font integrations. HSTS takes effect only when the site is
reached over HTTPS. The CSP deliberately retains `unsafe-inline` because the
generated static viewer and trusted head injection currently emit inline
scripts and styles; removing that exception requires a nonce/hash build step.

File serving resolves both the configured root and final target through the
real filesystem before opening a stream, so a symlink cannot expose files
outside the public tree. Stream errors and client disconnects are contained,
common binary/font MIME types are explicit, request headers are capped at 16
KiB, request/header/keep-alive timeouts are finite, and each socket is limited
to 100 requests. `pnpm test:web` covers files, directories, fallback Wiki
assets, methods, malformed URLs, redirects, headers, caches, symlink escape,
and server resource bounds.

The Railway deployment workflow runs on pushes to `main`. It first verifies the
repository with `pnpm test` and `pnpm build`, then deploys through the Railway
CLI container with `railway up --ci --service="$RAILWAY_SERVICE"`. The container
uses an explicit CLI version plus its immutable linux/amd64 manifest digest;
`pnpm check:workflow-pins` rejects mutable job-container images as well as
mutable actions. Configure
`RAILWAY_TOKEN` as a repository secret and `RAILWAY_SERVICE` as the Railway
service name or service ID. `RAILWAY_PROJECT_ID` is optional, but should be set
when the token is not already scoped tightly enough to the target project. When
`RAILWAY_PROJECT_ID` is set, Railway also requires an environment; the workflow
uses `RAILWAY_ENVIRONMENT` with a default of `production`. Override it with the
exact Railway environment name or ID if the project uses a different
environment. The workflow still accepts the older `RAILWAY_SERVICE_ID` name as a
fallback, but it must contain a service name or service ID, not a project ID.
Railway secret expressions exist only on the configuration check and deploy
steps; checkout and any future preparation steps do not receive them. The
repository test gate rejects job-, workflow-, and container-wide secret
expressions so this boundary cannot silently regress.

`railway.json` keeps Railway build and runtime settings in code and tells
Railway to use the repository `Dockerfile`. The Docker build installs both Go
and Node/pnpm because `pnpm build:web` exports the wiki by running the current
Go CLI source. The runtime image copies only `packages/web/dist` and the web
server script, then starts `node packages/web/scripts/serve.mjs`. Runtime env in
the Dockerfile serves `packages/web/dist` without re-exporting the wiki and
binds to `0.0.0.0` so the Railway router can reach the container. The final
stage keeps static assets root-owned and read-only to the process, then selects
the official image's unprivileged `node` user before Railway's start command.
The container policy check prevents a missing or root final `USER` from passing
the standard test gate.

## Release

GitHub Releases are the source of truth for downloadable binaries. Run the
release manually from GitHub Actions:

```text
Actions -> Release -> Run workflow -> version: 0.6.0
```

Use the version already declared in the root `package.json`; the next prepared
release is `0.6.0`. Before dispatch, keep `packages/npm/package.json`,
`packages/web/package.json`, and the Go fallback version aligned and run
`pnpm check:versions`.

The release workflow normalizes the input to a `v*` tag and requires it to
match the repository version. Its first verification step requires
`workflow_dispatch` to target the repository's default branch and fetches that
branch again to prove the checked-out commit is still its current tip. A stale
dispatch, feature branch, or tag ref fails before version resolution, setup,
secrets, or publication. Before it creates a tag, the workflow then verifies
tidy Go modules, runs tests and `go vet`, builds the CLI and website, checks the
injected binary version, validates the Wiki, inspects the npm tarball, and
requires npm publishing credentials. If the tag already exists, it must point
at the workflow commit; otherwise the workflow fails without moving it.

Release verification runs with repository-wide `contents: read`. Only after it
succeeds does the three-step `publish_release` job receive `contents: write` to
check out the exact verified commit, prepare its tag, and run GoReleaser. Setup
actions, dependency installation, tests, builds, package inspection, and npm
credential preflight never receive the write-capable GitHub token. The
workflow permission checker rejects any new write capability or extra step in
that privileged job until the policy is explicitly reviewed and updated.
It also locks the default-branch guard as the verifier's first post-checkout
step.

An in-repository workflow cannot defend against a writer who changes or removes
that workflow on another ref. Administrators should pair the guard with GitHub
default-branch protection and a tag ruleset that limits `v*` creation to the
approved release path. Those repository settings are external state and are
not claimed as configured merely because this workflow exists.

GoReleaser uploads the installer, checksums, license files, third-party
notices, and platform archives to GitHub Releases. After that job succeeds, the
workflow checks out the exact release tag and publishes
`@openknowledge-sh/openknowledge` with npm provenance. Stable versions use the
`latest` dist-tag; prereleases use `next`.

Both the GoReleaser action and the GoReleaser binary it downloads are pinned:
the action uses a full commit SHA and its `version` input is an exact stable
release rather than `latest`. `pnpm check:workflow-pins` rejects dynamic tool
aliases so a privileged release cannot silently switch toolchains between
runs.

Configure the repository `NPM_TOKEN` secret with permission to create and
publish the public scoped package. The preflight checks this secret before a
new Git tag is pushed, preventing a known npm-credential failure from leaving a
GitHub-only release.

Local installer test against a directory of release assets:

```sh
OPENKNOWLEDGE_BASE_URL=file:///tmp/openknowledge-release \
OPENKNOWLEDGE_INSTALL_DIR=/tmp/openknowledge-bin \
bash install
```

Manual npm publish fallback after the matching GitHub Release exists:

```sh
cd packages/npm
npm publish --provenance --access public
```

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `package.json`
> * `scripts/check-versions.mjs`
> * `scripts/check-workflow-pins.mjs`
> * `scripts/check-workflow-secret-scope.mjs`
> * `scripts/check-workflow-permissions.mjs`
> * `scripts/test-install.sh`
> * `scripts/check-container-runtime.mjs`
> * `pnpm-workspace.yaml`
> * `.github/workflows/deploy-railway.yml`
> * `.github/workflows/release.yml`
> * `.github/workflows/ci.yml`
> * `.dockerignore`
> * `Dockerfile`
> * `.goreleaser.yaml`
> * `railway.json`
> * `install`
> * `packages/npm/package.json`
> * `packages/web/package.json`
> * `packages/web/scripts/build.mjs`
> * `packages/web/scripts/wiki-export.mjs`
> * `packages/web/scripts/serve.mjs`
> * `packages/web/scripts/server.mjs`
> * `packages/web/scripts/server.test.mjs`
> * `packages/web/index.html`
> * `packages/web/main.js`
> * `Wiki/openknowledge.toml`
> * `Wiki/assets/openknowledge-site.css`
>
> **Update notes**
>
> When workspace scripts, CI or deployment workflow behavior, release workflow
> behavior, GoReleaser outputs, npm publish behavior, or local release testing changes,
> update this page and [CLI changelog](/changelog/cli.md).
