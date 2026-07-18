---
type: Feature Documentation
title: CLI Operations
description: Develop, test, publish, and release the Open Knowledge CLI.
tags: [openknowledge, cli, operations, release]
timestamp: 2026-07-18T00:00:00Z
---

# CLI Operations

## Workspace

```text
packages/cli  Go CLI and public Go package
packages/npm  npm wrapper for release binaries
packages/web  website and static wiki host
Wiki          canonical CLI documentation
```

The root `package.json` owns the release version and workspace commands.

## Local development

```sh
pnpm install --frozen-lockfile
pnpm test
pnpm build
```

| Command | Purpose |
| --- | --- |
| `pnpm test:cli` | Run Go tests. |
| `pnpm test:install` | Test the shell installer transactionally. |
| `pnpm test:npm-install` | Test the npm downloader and archive parser offline. |
| `pnpm test:web` | Test the static server without binding a socket. |
| `pnpm check:repo-jobs` | Validate repository job definitions. |
| `pnpm check:versions` | Verify package and Go fallback version alignment. |
| `pnpm check:workflow-pins` | Require immutable action and job-image references. |
| `pnpm check:workflow-secret-scope` | Keep secrets at the consuming step. |
| `pnpm check:workflow-permissions` | Enforce reviewed minimal write scopes. |
| `pnpm check:security-config` | Verify scanning and dependency-update coverage. |
| `pnpm check:container-runtime` | Verify toolchain, image, user, volume, and credential boundaries. |
| `pnpm build:cli` | Build `bin/openknowledge`. |
| `pnpm build:web` | Build the website and exported wiki. |
| `pnpm dev:web` | Run the local website workflow. |

`pnpm test` runs every policy check plus installer, web, and CLI tests.

## Continuous integration

`.github/workflows/ci.yml` runs for pull requests, `main`, and manual dispatch.
It uses read-only repository permissions, cancels superseded runs, and:

1. installs frozen Go, Node, and pnpm dependencies;
2. verifies tidy Go modules;
3. runs `pnpm test` and `go vet`;
4. builds the CLI and website;
5. validates `Wiki/` with the built binary;
6. fails if generation changed tracked files.

Require the `CI / verify` check in branch protection.

Scheduled security automation lives in `.github/workflows/security.yml` and
`.github/dependabot.yml`. It covers Go and JavaScript CodeQL, `govulncheck`,
checksum-verified OSV Scanner, npm, Go modules, Actions, and Docker. Results may
change as vulnerability databases update.

## Website

`pnpm build:web` builds `packages/web/dist`, exports `Wiki/` to
`dist/wiki`, and publishes JSON schemas under `dist/schemas/cli/`. The exporter
uses the current Go source by default; set `OPENKNOWLEDGE_BIN` to test a specific
binary.

The build extracts the analytics head block from `packages/web/index.html` and
injects it into wiki pages. Other trusted head injection is available through:

```sh
OPENKNOWLEDGE_HEAD_FILE=./head.html pnpm build:web
OPENKNOWLEDGE_HEAD_HTML='<meta name="..." content="...">' pnpm build:web
OPENKNOWLEDGE_SCRIPT_SRC=/analytics.js pnpm build:web
```

`Wiki/openknowledge.toml` owns the deployed theme, source links, site URL, and
publication allowlist. Keep `Wiki/assets/openknowledge-site.css` aligned with
the landing-page visual system.

The production Node server serves only the built tree. It bounds methods,
headers, timeouts, and requests per socket; resolves real paths before reads;
and sends CSP, HSTS, frame denial, MIME sniffing prevention, and explicit cache
policies. Railway website deployment uses the repository `Dockerfile` and
`railway.json`; the final image runs as the unprivileged Node user.

## Release

The release version must match the root, npm, web, and Go fallback versions.
Run the manual workflow from the current default-branch tip:

```text
Actions → Release → Run workflow → version: 0.8.4
```

The workflow performs the complete quality gate before creating a tag. The
publication job alone receives release write, OIDC, and attestation
permissions. GoReleaser publishes checksums, archives, licenses, installer, and
signed provenance; npm publishes the matching wrapper with provenance.
Deployable projects build their own pinned runtime from the committed
`.openknowledge/runtime/Dockerfile`; releases do not publish role images.

Stable releases use npm `latest`; prereleases use `next`. Verify an archive:

```sh
gh attestation verify openknowledge_darwin_arm64.tar.gz \
  -R openknowledge-sh/openknowledge
```

Required external controls are `NPM_TOKEN`, default-branch protection, and a
tag ruleset that limits `v*` creation to the release workflow.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `package.json`
> - `.github/workflows/{ci,security,release,deploy-railway}.yml`
> - `.github/dependabot.yml`
> - `Dockerfile`
> - `.goreleaser.yaml`
> - `packages/web/scripts/`
> - `scripts/check-*.mjs`
>
> **Update notes**
>
> Update this page when workspace commands, CI gates, website publication, or
> release responsibilities change.
