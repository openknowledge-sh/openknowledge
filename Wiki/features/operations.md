---
type: Feature Documentation
title: CLI Operations
description: Develop, test, publish, and release the Open Knowledge CLI.
tags: [openknowledge, cli, operations, release]
timestamp: 2026-08-08T00:00:00Z
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
| `pnpm test:web` | Run TypeScript checks, Oxlint, and static server tests. |
| `pnpm test:browser` | Exercise the production landing build and exported viewer over HTTP and `file://` in Chromium. |
| `pnpm test:race` | Run all Go tests with the race detector. |
| `pnpm test:coverage` | Produce `coverage.out` for the Go packages. |
| `pnpm build:viewer` | Build ignored viewer assets for Go embedding. |
| `pnpm check:format` | Fail when committed Go files are not formatted. |
| `pnpm check:onboarding-docs` | Keep README, website, and wiki setup/publication guidance aligned. |
| `pnpm check:repo-jobs` | Validate repository job definitions. |
| `pnpm check:versions` | Verify package and Go fallback version alignment. |
| `pnpm check:workflow-pins` | Require immutable action and job-image references. |
| `pnpm check:workflow-secret-scope` | Keep secrets at the consuming step. |
| `pnpm check:workflow-permissions` | Enforce reviewed minimal write scopes. |
| `pnpm check:security-config` | Verify scanning and dependency-update coverage. |
| `pnpm check:container-runtime` | Verify toolchain, image, user, volume, and credential boundaries. |
| `pnpm build:cli` | Build viewer assets and `bin/openknowledge`. |
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
9. Reject tracked viewer build output.
10. Build viewer assets before Go tests on Linux, macOS, and Windows.
11. Run CLI tests and builds on Linux, macOS, and Windows.
12. Verify npm and web behavior on Node 18.
13. Verify an installed packed artifact on Node 18.

Require the `CI / verify` check in branch protection.

Security automation runs for pull requests, `main`, and schedules.
`.github/workflows/security.yml` and `.github/dependabot.yml` define this automation.
It covers Go and JavaScript CodeQL, `govulncheck`, and checksum-verified OSV Scanner.
It also covers npm, Go modules, Actions, and Docker.
Results can change when vulnerability databases change.

## Website

`pnpm build:web` builds `packages/web/dist`.
Vite compiles the landing page from `packages/web/src/main.ts`.
It builds the shared viewer bundle from `packages/web/src/viewer`.
Vite writes the bundle directly into the ignored Go embed directory.
Git does not track the compiled viewer files.
Commands that build or test the CLI generate these files first.
It exports `Wiki/` to `dist/wiki`.
It publishes JSON schemas under `dist/schemas/cli/`.
By default, the exporter uses the current Go source.
Set `OPENKNOWLEDGE_BIN` to test a specified binary.

The landing page uses a first-party telemetry client after website consent.
The build does not inject product analytics into exported wiki pages.
Use these variables for trusted custom head content:

```sh
OPENKNOWLEDGE_HEAD_FILE=./head.html pnpm build:web
OPENKNOWLEDGE_HEAD_HTML='<meta name="..." content="...">' pnpm build:web
OPENKNOWLEDGE_SCRIPT_SRC=/analytics.js pnpm build:web
```

`Wiki/.openknowledge.toml` defines the deployed theme, source links, and site URL.
It also defines the publication asset list.
Keep `Wiki/assets/openknowledge-site.css` consistent with the landing-page visual system.

The production Node server serves only the built tree.
It limits methods, headers, timeouts, and requests per socket.
It resolves real paths before each read.
It sends CSP, HSTS, frame denial, MIME sniffing prevention, and explicit cache policies.
Railway website deployment uses the repository `Dockerfile` and `railway.json`.
The final image runs as the unprivileged Node user.

The server accepts bounded envelopes at `/api/telemetry`. It validates an exact
event allowlist and does not forward request identity.

The relay uses PostHog's batch capture protocol. It does not send the Open
Knowledge envelope directly and it does not use bearer authentication. Without
both variables below, the endpoint accepts and discards valid events:

```text
OPENKNOWLEDGE_TELEMETRY_UPSTREAM=https://eu.i.posthog.com/batch/
OPENKNOWLEDGE_TELEMETRY_TOKEN=phc_REPLACE_WITH_PROJECT_TOKEN
```

`OPENKNOWLEDGE_TELEMETRY_TOKEN` must be the PostHog **project token** from the
project settings, not a personal API key. The relay places it in PostHog's
`api_key` request field. The EU endpoint keeps ingestion in PostHog EU Cloud.
The upstream may also be the root ingestion host; the relay normalizes a root
URL to `/batch/`. A custom path, query, fragment, or URL-embedded credentials is
rejected.

### PostHog and Railway setup

1. Create or select a PostHog EU Cloud project. In its project settings, copy
   the project token shown with the event-ingestion configuration.
2. Open the deployed web service in Railway. Under **Variables**, add the two
   variables above with the real project token. Do not add a PostHog personal
   API key and do not expose the token through a `VITE_` variable.
3. Redeploy the service so the Node server reads the variables. No repository
   or Railway configuration-file change is required.
4. In PostHog, open **Activity** or **Live events**. Visit the deployed homepage,
   accept analytics, and copy the setup prompt. Confirm `web_page_viewed` and
   `setup_prompt_copied` arrive with `$process_person_profile = false`.
5. Follow the displayed install path and confirm
   `install_redirect_requested`. Then run `okn telemetry status` and a normal
   CLI command. Confirm CLI events arrive without paths, arguments, content,
   output, error messages, hostnames, client IP fields, or raw user agents.
6. If no events arrive, confirm the project is an EU project, the endpoint is
   exactly `https://eu.i.posthog.com/batch/`, and the value is the project token.
   A wrong token or host is intentionally invisible to clients because relay
   delivery failures never affect website or CLI behavior.

For a pre-production CLI check, point one invocation at the deployed relay:

```sh
OPENKNOWLEDGE_TELEMETRY_ENDPOINT=https://YOUR_DOMAIN/api/telemetry okn version
```

The command must behave normally even if ingestion is unavailable. Use
`okn telemetry show-payload` to inspect a representative allowlisted CLI
payload before validating delivery.

### PostHog insights

Create a dashboard with these insights:

| Insight | PostHog definition |
| --- | --- |
| Website setup interest | Funnel from `web_page_viewed` to `setup_prompt_copied`, unique by `distinct_id` |
| Observable install attempts | Trend of `install_redirect_requested`, broken down by `source` and `client_family` |
| CLI activation | Funnel from `cli_first_command` to `cli_setup_completed` to `cli_first_meaningful_use`, unique by `distinct_id` |
| Daily active installations | Daily trend of `cli_daily_active`, counting unique `distinct_id` values |
| Meaningful-use retention | Retention from `cli_first_meaningful_use` to `cli_daily_active` |
| Sanitized failure trends | Trend of `cli_error`, broken down by `error_kind`, `command`, and `app_version` |
| Sanitized error issues | PostHog Error Tracking issues from `$exception`, filtered by `surface = cli` |
| Version adoption | Trend of `cli_command_completed`, broken down by `app_version` |

Do not build a single person-level funnel from website visit through CLI use.
Web, server, and CLI identifiers are deliberately separate, so such a funnel
would imply attribution the system does not observe. Compare those stages as
aggregate rates instead. The relay creates synthetic `$exception` events from
`cli_error`. These issues contain no error message, stack trace, path, arguments,
or output. Do not count both event types in one failure metric.

## Release

Run the manual workflow from the current default-branch tip:

```text
Actions → Release → Run workflow → version: 0.11.0
```

The version input accepts `0.11.0`, `v0.11.0`, or a prerelease such as
`v0.11.0-rc.1`. The workflow serializes release runs and does not cancel an
active release.

The `verify` job requires the current default-branch tip. It then completes
these tasks before any repository write:

1. Update the root, npm, web, and Go fallback versions.
2. Install frozen dependencies and verify version alignment.
3. Verify tidy Go modules, Go formatting, policy checks, tests, and builds.
4. Run race tests, `go vet`, and Chromium browser journeys.
5. Build the release binary and validate `Wiki/` with that binary.
6. Install the packed npm artifact in an isolated test project.
7. Build and inspect a six-archive GoReleaser snapshot.
8. Verify `NPM_TOKEN` with the npm registry.

The `commit_release` job applies the same version update to the verified source.
It pushes a version commit only when the tracked version files change.
The `publish_release` job checks out that exact commit and prepares the release
tag. It reuses the tag only when the tag already points to that commit.

The publication job rebuilds viewer assets before GoReleaser runs. GoReleaser
publishes six archives, checksums, licenses, and the installer. The workflow
attests the archives from `dist/checksums.txt`.

The final `npm` job checks out the release tag. It verifies the package version
before it publishes the wrapper with npm provenance. Stable releases use the
npm `latest` tag. Prereleases use the `next` tag.

Only `commit_release` and `publish_release` receive repository write access.
Only `publish_release` receives attestation access. The publication and npm
jobs receive only their required OIDC access.

Deployable projects build a pinned runtime from the committed
`.openknowledge/runtime/Dockerfile`. Releases do not publish role images.

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
> - `packages/web/vite*.config.ts`
> - `packages/web/src/`
> - `scripts/check-*.mjs`
>
> **Update notes**
>
> Update this page after a change to workspace commands or CI gates.
> Also update it after a change to website publication or release responsibilities.
