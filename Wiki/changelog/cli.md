---
type: Changelog
title: CLI Changelog
description: Release-level history for the Open Knowledge CLI.
tags: [openknowledge, cli, changelog]
timestamp: 2026-07-18T00:00:00Z
---

# CLI Changelog

Current behavior belongs in the [command reference](/features/commands/). This
page records release-level changes.

## Unreleased

### 2026-07-18 — Static viewer CSP compatibility

- Moved generated viewer JavaScript from executable inline `<script>` blocks
  into same-origin export assets, so Railway and runtime deployments work with
  the default `script-src 'self' https:` policy without `unsafe-inline`.
- Kept deployment-owned head injection explicit: custom inline scripts may
  still require a deployment-specific nonce or hash, while `--script-src`
  remains compatible with allowed external sources.
- Source: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
- Docs: `Wiki/features/exporters/html.md`,
  `Wiki/features/commands/runtime.md`.

### 2026-07-18 — Private Railway endpoint reconciliation

- Made `--no-public-endpoint` enumerate and delete existing Railway service and
  custom domains instead of trusting possibly stale local endpoint state.
- Source: `packages/cli/cmd/openknowledge/deploy_command.go`.

### 2026-07-18 — Railway prune removes service volumes

- Made `--prune` enumerate and delete persistent volumes attached to omitted
  services before deleting those services, preventing provider-orphaned agent
  state during migration to the immutable one-service topology.
- Source: `packages/cli/cmd/openknowledge/deploy_command.go`.

### 2026-07-18 — Immutable Railway deployment by default

- Changed the default Railway topology to one `serve` service whose
  multi-stage Docker image builds and embeds the knowledge artifact for the
  triggering source commit.
- Made Git polling, the private publisher, persistent agent state, and isolated
  workers explicit through `--runtimes`; enabled jobs are no longer inferred
  during deployment.
- Removed GitHub, model, artifact-sync, and exchange credentials from the
  default deployment requirements.
- Added a committed generated `runtime.toml`, so the generated image starts as
  `serve` and can be tested locally without Railway-specific variables.
- Added `--prune` as an explicit, fail-closed migration path for deleting
  publisher and worker services omitted by the new topology. Existing
  deployments can migrate with a reviewed dry-run followed by
  `openknowledge deploy railway Wiki --prune --yes`.
- Source: `packages/cli/cmd/openknowledge/deploy_command.go`,
  `packages/cli/cmd/openknowledge/deploy_runtime_scaffold.go`.
- Docs: `README.md`, `Wiki/features/commands/deploy.md`,
  `Wiki/features/commands/runtime.md`.

### 2026-07-18 — Runtime log severity

- Moved successful runtime lifecycle events from standard error to standard
  output so Railway and other hosting platforms no longer classify healthy
  listening, synchronization, publication, or activation messages as errors.
- Kept usage diagnostics, failed passes, retained-generation warnings, and
  archive failures on standard error.
- Source: `packages/cli/cmd/openknowledge/runtime_command.go`,
  `packages/cli/cmd/openknowledge/runtime_private_api.go`,
  `packages/cli/cmd/openknowledge/runtime_serve.go`,
  `packages/cli/cmd/openknowledge/runtime_worker.go`.
- Docs: `Wiki/features/commands/runtime.md`.

### 2026-07-18 — Repository-owned Railway runtime

- Added `openknowledge deploy railway init` to generate a project-owned,
  non-root runtime Dockerfile with independent Open Knowledge and agent CLI
  pins; existing project choices require explicit `--force` to replace.
- Changed Railway provisioning from published GHCR role images to the target
  GitHub repository source. Services share the committed Dockerfile while
  retaining separate roles, ingress, volumes, and credentials.
- Migrated version 1 deployment state to repository sources in place and
  removed runtime-image publication from the release workflow.
- Treated Railway source connection as the initial deployment trigger instead
  of immediately issuing a conflicting redundant redeploy.
- Made the generated entrypoint repair persistent-volume ownership during
  startup and then drop to UID/GID `10001`, including for volumes created by an
  older root-based runtime image.
- Source: `packages/cli/cmd/openknowledge/deploy_runtime_scaffold.go`,
  `packages/cli/cmd/openknowledge/deploy_command.go`,
  `.github/workflows/release.yml`.

### 2026-07-18 — Railway non-root volume startup

- Kept publisher checkout, build, and lock state on ephemeral container storage;
  published artifacts and exchange data remain on the persistent Railway
  volume. Worker state uses a process-owned child directory below its mount.
- Avoided redundant permission changes when the runtime state directory is
  already private, while still tightening a permissive existing directory.
- Authenticated private GitHub Smart HTTP clone and fetch operations with an
  ephemeral Basic extra header instead of a rejected Bearer header; credentials
  remain absent from repository URLs and command arguments.
- Source: `packages/cli/cmd/openknowledge/deploy_command.go`,
  `packages/cli/cmd/openknowledge/runtime_worker.go`.

### 2026-07-18 — Short CLI alias

- Added `okn` as an installed alias for `openknowledge` in both the shell and
  npm installers while keeping the original command name.
- Made the shell installer refuse to overwrite an unrelated existing `okn`
  command.
- Source: `install`, `scripts/test-install.sh`, `packages/npm/`.

### 2026-07-18 — Railway CLI v5 deployment recovery

- Separated Railway progress diagnostics from JSON stdout so successful v5
  service creation records provider IDs instead of failing after mutation.
- Updated v5 volume creation to place the service selector before the nested
  subcommand and address the service by provider ID.
- Persisted the selected existing project before service creation, ensuring an
  interrupted first apply leaves recoverable secret-free state.
- Source: `packages/cli/cmd/openknowledge/deploy_command.go`.

### 2026-07-18 — CLI-led onboarding

- Established `openknowledge setup Wiki --from .` as the primary project-wiki
  activation path across the CLI, README, and website.
- Clarified that `setup` launches the selected agent, then validates the bundle
  and installs project integration. `scaffold` remains the deterministic,
  agent-free primitive.
- Documented `runtime build --out <dir>`, including its single-selection
  requirement and versioned result shape.
- Source: `packages/cli/cmd/openknowledge/{main,setup_command,runtime_command}.go`.

### 2026-07-17 — Workflow-oriented command surface

- Consolidated managed onboarding under `setup`; moved portable instructions
  to `prompt setup|from|rules|review`.
- Renamed `new` to `scaffold`, `to` to `export`, the experimental `agents`
  group to `jobs`, and detached `spawn` to `start`.
- Kept connection mutation at `connect` and `disconnect`; `registry` now owns
  listing, integrity status, refresh, and path resolution.
- Reorganized root help around create/maintain, use/publish, service, and
  validate/connect workflows.
- Source: `packages/cli/cmd/openknowledge/{main,setup_command,prompt_command}.go`.

### 2026-07-17 — Agents, insights, and jobs

- Added a steered local `agent` interface for Codex, Claude Code, and OpenCode,
  with interactive and non-interactive modes, executable discovery, `doctor`,
  model overrides, and optional isolated worktrees.
- Added project and global integration plus bounded observation hooks.
- Made `insights` the shared interface for deterministic capture, review,
  dismissal, direct execution, and scheduled processing of private knowledge
  gaps.
- Restricted job and service runtimes to the same three harnesses. Jobs now use
  strict runtime/model selection, per-harness credential scoping, external
  private state, observable detached runs, cancellation, and versioned records.
- Source: `packages/cli/cmd/openknowledge/{agent_command,insights_command,agents_command}.go`,
  `packages/cli/internal/{agents,insights,integration}/`.

### 2026-07-17 — Isolated runtime and Railway deployment

- Added immutable generation planning, building, serving, and private worker
  reconciliation for one repository and multiple routed knowledge bases.
- Split GitHub publication from model execution: publisher, serve, and one
  worker per harness use distinct images, credentials, volumes, and network
  boundaries.
- Added `deploy railway` with secret-free dry runs, explicit mutation consent,
  idempotent state, generated/custom/private endpoint modes, and worker
  inference from enabled jobs.
- Added authenticated private artifact and Git-bundle exchange for providers
  without shared volumes. Invalid updates retain the last verified generation.
- Source: `packages/cli/cmd/openknowledge/runtime_*.go`,
  `packages/cli/cmd/openknowledge/deploy_command.go`, `packages/cli/internal/runtime/`,
  `docker/runtime.Dockerfile`, `deploy/runtime/`.

### 2026-07-17 — Explicit publication contract

- Made public HTML, portable public source, and runtime generation fail closed
  unless `[publish] enabled = true`.
- Added `okf_targets.viewer|search|mcp|llms|sitemap` and separate runtime
  projections for viewer, search, and MCP.
- Limited public non-Markdown files to `[publish].assets`; project config,
  `.openknowledge` state, denied Markdown, and non-allowlisted assets remain
  absent from artifacts.
- Source: `packages/cli/internal/okf/{project_config,publish}.go`,
  `packages/cli/internal/runtime/generation.go`.

### 2026-07-15 — Machine and retrieval contracts

- Added versioned JSON envelopes and published Draft 2020-12 schemas for CLI
  errors, AST, validation, bundle/list/registry output, search/context,
  federation, graphs, jobs, portable manifests, and storage records.
- Added the root `--error-format text|json` diagnostic envelope.
- Added revision-bound search provenance with content digests and
  `okf+sha256://` locators, plus registry-wide reciprocal-rank fusion.
- Added a public read-only Go API for parsing, validation, retrieval, graphs,
  and registry resolution.
- Source: `packages/cli/schemas/`, `packages/cli/internal/okf/`,
  `packages/cli/okf/`.

### 2026-07-15 — Remote registry integrity

- Added strict versioned registry and provenance storage, offline integrity
  status, and atomic refresh and deletion.
- Added Git ref and monorepo subdirectory selection, source-addressed caches,
  bounded non-interactive transport, archive and staging-tree limits, and
  secret-safe URL handling.
- Remote materialization now uses locked sibling staging and transactional
  publication; failed refresh preserves the previous generation.
- Source: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/internal/okf/registry.go`, `packages/cli/schemas/storage/v1/`.

### 2026-07-15 — Viewer, packaging, and release hardening

- Unified local viewer search with canonical heading-section retrieval and
  content-bound cache invalidation; registry workspaces now follow live,
  validated snapshots.
- Hardened static serving, containers, release permissions, workflow pinning,
  scheduled security scanning, and npm/shell binary installation.
- Added reproducible portable archives, transactional export publication,
  signed release provenance, and default-branch-only release dispatch.
- Source: `packages/cli/cmd/openknowledge/viewer*.go`, `packages/web/`,
  `install`, `packages/npm/`, `.github/workflows/`, `Dockerfile`.

## v0.6.0 Candidate

### 2026-07-09 — Retrieval and viewer polish

- Made source-preserving Markdown context the default search output.
- Added typed frontmatter inspection, tag facets, breadcrumbs, reading and
  accessibility settings, improved search navigation, and visual polish.
- Aligned the public website and wiki landing experience with the
  LLM-oriented knowledge workflow.

## v0.5.0 — 2026-07-08

- Added source-to-wiki prompts, maintenance rules, advisory review, and the
  first experimental local job runner.
- Added exact `get`, structural `list`, ranked `search`, registry-backed `view`,
  and search graph workflows.
- Added static discovery files, analytics/head injection, and portable viewer
  connection assets.
- Expanded validation JSON output and configurable rule severities.

## v0.4.0 — 2026-06-23

- Added AST output, source and search graph exporters, query-oriented context,
  and key-or-path resolution across bundle commands.
- Improved viewer themes, search highlights, shortcuts, panel navigation,
  responsive layout, and graph presentation.
- Added website deployment, install redirects, and static wiki publication.

## v0.3.0 — 2026-06-20

- Added connected bundle commands and registry-backed local viewing.
- Added static HTML export, portable manifests, bundle metadata, theming,
  syntax highlighting, tables, asset previews, and source links.
- Strengthened UTF-8, frontmatter, Markdown, link, and reserved-file validation.
- Moved CLI documentation into this colocated OKF wiki.

## Initial wiki maintenance — 2026-06-18

- Added the repository wiki, embedded OKF specification, command references,
  update workflows, and validation loop.

---

<!-- okf-footer: agent-maintenance -->

> **Update notes**
>
> Add concise release-facing changes under `Unreleased`. Group related commits
> by user outcome; do not recreate per-command implementation logs.
