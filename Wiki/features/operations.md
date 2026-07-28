---
type: Feature Documentation
title: CLI Operations
description: Develop, test, publish, and release the Open Knowledge CLI.
tags: [openknowledge, cli, operations, release]
timestamp: 2026-07-28T00:00:00Z
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
| `pnpm test:packed-npm` | Pack and install the exact npm publication artifact on the active Node version. |
| `pnpm test:web` | Test the static server. The test does not bind a socket. |
| `pnpm test:browser` | Exercise landing-page setup and exported-viewer search/keyboard journeys in Chromium. |
| `pnpm test:race` | Run all Go tests with the race detector. |
| `pnpm test:coverage` | Produce `coverage.out` for the Go packages. |
| `pnpm check:format` | Fail when committed Go files are not formatted. |
| `pnpm check:onboarding-docs` | Keep README, website, and wiki setup/publication guidance aligned. |
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

`pnpm test` runs all policy checks.
It also runs installer, web, and CLI tests.

## Continuous integration

`.github/workflows/ci.yml` runs for pull requests, `main`, and manual dispatch.
It uses read-only repository permissions.
It cancels a run when a newer run replaces it.
The workflow does these tasks:

1. Install frozen Go, Node, and pnpm dependencies.
2. Verify tidy Go modules.
3. Verify Go formatting.
4. Run the policy and unit test suite.
5. Run the race detector, coverage report, and `go vet`.
6. Build the CLI and website.
7. Test landing and viewer journeys in Chromium.
8. Validate `Wiki/` with the built binary.
9. Fail when generation changes tracked files.
10. Run CLI tests and builds on Linux, macOS, and Windows.
11. Verify npm and web behavior on Node 18.
12. Verify an installed packed artifact on Node 18.

Require the `CI / verify` check in branch protection.

Security automation runs for pull requests, `main`, and schedules.
`.github/workflows/security.yml` and `.github/dependabot.yml` define this automation.
It covers Go and JavaScript CodeQL, `govulncheck`, and checksum-verified OSV Scanner.
It also covers npm, Go modules, Actions, and Docker.
Results can change when vulnerability databases change.

## Website

`pnpm build:web` builds `packages/web/dist`.
It exports `Wiki/` to `dist/wiki`.
It publishes JSON schemas under `dist/schemas/cli/`.
By default, the exporter uses the current Go source.
Set `OPENKNOWLEDGE_BIN` to test a specified binary.

The build extracts the analytics head block from `packages/web/index.html`.
It injects this block into wiki pages.
Use these variables for other trusted head content:

```sh
OPENKNOWLEDGE_HEAD_FILE=./head.html pnpm build:web
OPENKNOWLEDGE_HEAD_HTML='<meta name="..." content="...">' pnpm build:web
OPENKNOWLEDGE_SCRIPT_SRC=/analytics.js pnpm build:web
```

`Wiki/openknowledge.toml` defines the deployed theme, source links, and site URL.
It also defines the publication asset list.
Keep `Wiki/assets/openknowledge-site.css` consistent with the landing-page visual system.

The production Node server serves only the built tree.
It limits methods, headers, timeouts, and requests per socket.
It resolves real paths before each read.
It sends CSP, HSTS, frame denial, MIME sniffing prevention, and explicit cache policies.
Railway website deployment uses the repository `Dockerfile` and `railway.json`.
The final image runs as the unprivileged Node user.

## Release

The release version must match the root, npm, web, and Go fallback versions.
Run the manual workflow from the current default-branch tip:

```text
Actions → Release → Run workflow → version: 0.8.4
```

The workflow completes the quality gate before it creates a tag.
The gate includes browser journeys, race tests, and a real packed npm installation.
It also includes a GoReleaser snapshot with all six supported OS and architecture archives.
Only the publication job receives release write, OIDC, and attestation permissions.
GoReleaser publishes checksums, archives, licenses, the installer, and signed provenance.
npm publishes the matching wrapper with provenance.
Deployable projects build a pinned runtime from the committed `.openknowledge/runtime/Dockerfile`.
Releases do not publish role images.

Stable releases use the npm `latest` tag.
Prereleases use the `next` tag.
Use this command to verify an archive:

```sh
gh attestation verify openknowledge_darwin_arm64.tar.gz \
  -R openknowledge-sh/openknowledge
```

The required external controls are `NPM_TOKEN` and default-branch protection.
A tag ruleset must limit `v*` creation to the release workflow.

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
> Update this page after a change to workspace commands or CI gates.
> Also update it after a change to website publication or release responsibilities.
