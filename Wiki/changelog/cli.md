---
type: Changelog
title: CLI Changelog
description: Release-level history for the Open Knowledge CLI.
tags: [openknowledge, cli, changelog]
timestamp: 2026-07-28T00:00:00Z
---

# CLI Changelog

Current behavior belongs in the [command reference](/features/commands/). This
page records release-level changes.

## Unreleased

### 2026-07-28 — Runtime-specific project integration

- `okn integration install` installed only the selected runtime skill.
- Session observation became opt-in through `--observe`.
- The new `status` command reported missing and modified managed files without
  changing them.
- The new `remove` command deleted unchanged owned files. It preserved user
  changes and unrelated hook settings.
- The integration manifest restricted managed paths to the selected runtime.
- `okn agent integrate` remained as a deprecated alias.
- Automated `okn setup --agent` installed the selected skill without an
  observation hook.
- Source: `packages/cli/cmd/openknowledge/{integrate_command,setup_command}.go`,
  `packages/cli/internal/integration/{integration,manage}.go`.
- Docs: `Wiki/features/commands/{integration,integrate,setup}.md`.

### 2026-07-28 — Focused README

- The README became a short product entrypoint.
- It now gives one setup path, one local usage path, and one publication path.
- Publication guidance now distinguishes a local public bundle from deployment
  and a live HTTP MCP endpoint. It also states the Markdown publication default.
- Detailed command, runtime, deployment, validation, and release information
  now stays in the current-state wiki.
- The product summary now matches the workflow groups in CLI help and the wiki.
- Docs: `README.md`, `Wiki/features/`.

### 2026-07-28 — Portable setup by default

- `okn setup` printed portable instructions by default.
- The new `--agent` flag ran the instructions, validated the result, and
  installed project integration.
- If `--runtime` was absent, interactive agent mode detected installed
  runtimes and asked the user to select one.
- Non-interactive agent mode required `--runtime`.
- Source: `packages/cli/cmd/openknowledge/{command_catalog,setup_command}.go`.
- Docs: `Wiki/features/commands/{index,setup}.md`, `Wiki/index.md`.

### 2026-07-28 — Mermaid diagrams in the viewer

- The viewer rendered fenced `mermaid` blocks as theme-aware diagrams on local
  and static pages. The viewer no longer displayed these blocks as ordinary
  code.
- The viewer kept the escaped source visible when Mermaid was unavailable or a
  diagram was invalid.
- The viewer configured Mermaid with strict security mode for generated SVG.
- Source: `packages/cli/internal/okf/markdown.go`,
  `packages/cli/cmd/openknowledge/{viewer,viewer_assets}.go`,
  `packages/cli/cmd/openknowledge/viewer_app.{js,css}`,
  `packages/cli/cmd/openknowledge/viewer_mermaid.min.js`.
- Docs: `Wiki/features/commands/view.md`,
  `Wiki/features/exporters/html.md`.

### 2026-07-28 — Focused onboarding and document-coherent retrieval

- The CLI reduced project activation to `openknowledge setup`. The
  zero-argument command used the current repository as its source and wrote
  `Wiki`.
- Explicit targets and alternate sources remained optional workflows. The
  viewer, publishing, registry, runtime, jobs, scaffold, and portable prompts
  also remained optional.
- Section ranking included whole-document evidence, filename relevance,
  stronger body evidence, and query coverage. These changes let overview
  documents compete with specialized pages.
- Context packing preserved the strongest lexical seeds. It added siblings
  from the same document and parent or child evidence.
- Context packing labeled non-lexical hierarchy as `document-context`. It
  truncated prioritized oversized evidence. It did not skip that evidence for
  lower-ranked sections.
- Source: `packages/cli/cmd/openknowledge/{command_catalog,setup_command}.go`,
  `packages/cli/internal/okf/{search_knowledge,context_selection,setup,from,new}.go`,
  `packages/cli/schemas/v1/search-context.schema.json`.
- Docs: `README.md`, `packages/web/index.html`,
  `Wiki/features/commands/{index,setup,search}.md`.

### 2026-07-28 — Onboarding, release, and verification hardening

- The `setup` command verified the selected agent executable before it started
  the interactive workflow.
- The CLI added exact recovery guidance for doctor and authentication
  failures.
- One command catalog replaced duplicate root command dispatch and help
  definitions.
- An explicit writer handled diagnostics. The CLI no longer replaced the
  process-wide standard error stream temporarily.
- The release workflow closed the shell injection path for manual release
  input.
- A YAML parser and regression tests replaced text-based permission scans.
- The shell installer rejected custom plain-HTTP mirrors before download. It
  retained `file://` only for controlled local transaction tests.
- The pull-request workflow added security scans, race tests, coverage runs,
  and CLI certification on Linux, macOS, and Windows.
- Verification added Node 18 compatibility, packed npm installation, browser
  setup, search, and keyboard journeys.
- Verification also added pre-tag GoReleaser snapshot checks.
- The verification suite added parser/archive fuzz targets.
- It also added 100/1,000/10,000-section search/index benchmarks.
- The README, website, and command or operations references clarified the
  canonical zero-argument `setup` path and runtime recovery.
- These references also clarified fail-closed publication permission and the
  default validation warning policy.
- Source: `packages/cli/cmd/openknowledge/{setup_command,command_catalog,cli_io}.go`,
  `packages/cli/internal/okf/{fuzz,search_benchmark}_test.go`,
  `packages/cli/internal/tools/checkworkflowpermissions/`,
  `.github/workflows/{ci,release,security}.yml`, `scripts/`,
  `packages/web/scripts/browser.e2e.mjs`.
- Docs: `README.md`, `packages/web/index.html`,
  `Wiki/features/commands/setup.md`, `Wiki/features/operations.md`.

### 2026-07-21 — Anchor-aware graph retrieval

- Machine-readable links preserved canonical Markdown fragments. Search graphs
  and one-hop outgoing or backlink expansion resolved each fragment to its
  content chunk.
- Lower-level headings resolved to their containing retrieval chunk. Missing
  fragments no longer resolved to an unrelated first chunk.
- Parallel source and search graph edges preserved repeated authored links.
  Each occurrence kept its href, label, target anchor, and source line.
- The strongest relationship-derived score promoted weak lexical matches.
  Search did not discard graph evidence or return a duplicate result.
- Each context index included an immutable BM25 corpus. Generation-scoped
  caches no longer tokenized the complete corpus for each query.
- Source: `packages/cli/internal/okf/ast_links.go`,
  `packages/cli/internal/okf/context_sections.go`,
  `packages/cli/internal/okf/graph.go`,
  `packages/cli/internal/okf/search_knowledge.go`,
  `packages/cli/schemas/v1/common.schema.json`,
  `packages/cli/schemas/v1/graph.schema.json`.
- Docs: `Wiki/features/exporters/graph.md`,
  `Wiki/features/commands/search.md`.

### 2026-07-19 — Generation-scoped runtime search indexes

- The runtime built each search context index once before it activated the
  immutable generation.
- The runtime reused the index for `_search` requests. It no longer parsed and
  validated the search projection for each query.
- A new content digest replaced the cached index atomically. A failed index
  build retained the last valid generation.
- Source: `packages/cli/cmd/openknowledge/runtime_serve.go`,
  `packages/cli/cmd/openknowledge/runtime_command_test.go`.
- Docs: `Wiki/features/commands/runtime.md`.

### 2026-07-18 — Static viewer CSP compatibility

- The exporter moved generated viewer JavaScript from executable inline
  `<script>` blocks to same-origin assets.
- Railway and runtime deployments then worked with the default
  `script-src 'self' https:` policy without `unsafe-inline`.
- Runtime viewer pages and assets used `Cache-Control: no-cache` for
  revalidation.
- Browsers no longer retained an older generation after a source-triggered
  deployment.
- Deployment-owned head injection remained explicit. Some deployments still
  required a deployment-specific nonce or hash for custom inline scripts.
- The `--script-src` option remained compatible with permitted external
  sources.
- Source: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `packages/cli/cmd/openknowledge/runtime_serve.go`,
  `packages/cli/cmd/openknowledge/runtime_command_test.go`.
- Docs: `Wiki/features/exporters/html.md`,
  `Wiki/features/commands/runtime.md`.

### 2026-07-18 — Private Railway endpoint reconciliation

- The `--no-public-endpoint` option listed and deleted existing Railway service
  domains and custom domains.
- The option no longer trusted possibly stale local endpoint state.
- Source: `packages/cli/cmd/openknowledge/deploy_command.go`.

### 2026-07-18 — Railway prune removes service volumes

- The `--prune` option listed and deleted persistent volumes for omitted
  services before it deleted those services.
- This sequence prevented provider-orphaned agent state during migration to the
  immutable one-service topology.
- Source: `packages/cli/cmd/openknowledge/deploy_command.go`.

### 2026-07-18 — Immutable Railway deployment by default

- The default Railway topology changed to one `serve` service.
- Its multi-stage Docker image built and embedded the knowledge artifact for
  the source commit that triggered the build.
- The `--runtimes` option made Git polling, the private publisher, persistent
  agent state, and isolated workers explicit.
- Deployment no longer inferred enabled jobs.
- Default deployment requirements no longer included GitHub, model,
  artifact-sync, or exchange credentials.
- The project added a committed generated `runtime.toml`.
- The generated image started as `serve`. The image supported local tests
  without Railway-specific variables.
- The `--prune` option provided an explicit fail-closed migration path. It
  deleted publisher and worker services that the new topology omitted.
- Existing deployments required a reviewed dry run before migration. The
  migration command was
  `openknowledge deploy railway Wiki --prune --yes`.
- Source: `packages/cli/cmd/openknowledge/deploy_command.go`,
  `packages/cli/cmd/openknowledge/deploy_runtime_scaffold.go`.
- Docs: `README.md`, `Wiki/features/commands/deploy.md`,
  `Wiki/features/commands/runtime.md`.

### 2026-07-18 — Runtime log severity

- The runtime moved successful lifecycle events from standard error to standard
  output.
- Railway and other hosting platforms no longer classified healthy listening,
  synchronization, publication, or activation messages as errors.
- Usage diagnostics, failed passes, retained-generation warnings, and archive
  failures remained on standard error.
- Source: `packages/cli/cmd/openknowledge/runtime_command.go`,
  `packages/cli/cmd/openknowledge/runtime_private_api.go`,
  `packages/cli/cmd/openknowledge/runtime_serve.go`,
  `packages/cli/cmd/openknowledge/runtime_worker.go`.
- Docs: `Wiki/features/commands/runtime.md`.

## v0.7.2 — 2026-07-18

This cumulative section records the v0.7 release line. `v0.7.0` moved Railway
to a repository-owned runtime, `v0.7.1` removed a redundant source redeploy,
and `v0.7.2` completed non-root persistent-volume startup.

### 2026-07-18 — Repository-owned Railway runtime

- The CLI added `openknowledge deploy railway init`. This command generated a
  project-owned non-root runtime Dockerfile.
- The Dockerfile used independent Open Knowledge and agent CLI pins. The
  `--force` option was necessary to replace existing project choices.
- Railway provisioning changed from published GHCR role images to the target
  GitHub repository source.
- Services shared the committed Dockerfile. They retained separate roles,
  ingress, volumes, and credentials.
- The migration converted version 1 deployment state to repository sources in
  place.
- The release workflow no longer published runtime images.
- The Railway source connection became the initial deployment trigger. The CLI
  no longer issued an immediate conflicting redeploy.
- During startup, the generated entrypoint repaired persistent-volume
  ownership. It then dropped to UID/GID `10001`.
- The ownership repair also supported volumes that an older root-based runtime
  image created.
- Source: `packages/cli/cmd/openknowledge/deploy_runtime_scaffold.go`,
  `packages/cli/cmd/openknowledge/deploy_command.go`,
  `.github/workflows/release.yml`.

### 2026-07-18 — Railway non-root volume startup

- Publisher checkout, build, and lock state remained on ephemeral container
  storage.
- Published artifacts and exchange data remained on the persistent Railway
  volume.
- Worker state used a process-owned child directory below its mount.
- The runtime avoided redundant permission changes when its state directory was
  already private. It still restricted a permissive existing directory.
- An ephemeral Basic extra header authenticated private GitHub Smart HTTP clone
  and fetch operations. It replaced a rejected Bearer header.
- Credentials remained absent from repository URLs and command arguments.
- Source: `packages/cli/cmd/openknowledge/deploy_command.go`,
  `packages/cli/cmd/openknowledge/runtime_worker.go`.

### 2026-07-18 — Short CLI alias

- The shell and npm installers added `okn` as an installed alias for
  `openknowledge`. They retained the original command name.
- The shell installer refused to overwrite an unrelated existing `okn`
  command.
- Source: `install`, `scripts/test-install.sh`, `packages/npm/`.

### 2026-07-18 — Railway CLI v5 deployment recovery

- The CLI separated Railway progress diagnostics from JSON standard output.
- Successful v5 service creation then recorded provider IDs. It did not fail
  after mutation.
- The v5 volume command placed the service selector before the nested
  subcommand. It addressed the service by provider ID.
- The CLI saved the selected existing project before service creation.
- An interrupted first apply left recoverable secret-free state.
- Source: `packages/cli/cmd/openknowledge/deploy_command.go`.

### 2026-07-18 — CLI-led onboarding

- The CLI, README, and website established
  `openknowledge setup Wiki --from .` as the primary project-wiki activation
  path.
- Documentation clarified that `setup` launched the selected agent. The command
  then validated the bundle and installed project integration.
- The `scaffold` command remained the deterministic agent-free primitive.
- Documentation added `runtime build --out <dir>`. It included the
  single-selection requirement and versioned result shape.
- Source: `packages/cli/cmd/openknowledge/{main,setup_command,runtime_command}.go`.

### 2026-07-17 — Workflow-oriented command surface

- The CLI consolidated managed onboarding under `setup`.
- Portable instructions moved to `prompt setup|from|rules|review`.
- The CLI renamed `new` to `scaffold` and `to` to `export`.
- It renamed the experimental `agents` group to `jobs`. It also renamed
  detached `spawn` to `start`.
- Connection changes remained under `connect` and `disconnect`.
- The `registry` command owned listing, integrity status, refresh, and path
  resolution.
- Root help used create/maintain, use/publish, service, and validate/connect
  workflows.
- Source: `packages/cli/cmd/openknowledge/{main,setup_command,prompt_command}.go`.

### 2026-07-17 — Agents, insights, and jobs

- The CLI added a steered local `agent` interface for Codex, Claude Code, and
  OpenCode.
- The interface supported interactive and non-interactive modes, executable
  discovery, `doctor`, model overrides, and optional isolated worktrees.
- The CLI added project and global integration with bounded observation hooks.
- The `insights` command became the shared interface for deterministic capture,
  review, dismissal, direct execution, and scheduled processing of private
  knowledge gaps.
- Job and service runtimes supported the same three harnesses.
- Jobs used strict runtime and model selection, per-harness credential scoping,
  external private state, observable detached runs, cancellation, and versioned
  records.
- Source: `packages/cli/cmd/openknowledge/{agent_command,insights_command,agents_command}.go`,
  `packages/cli/internal/{agents,insights,integration}/`.

### 2026-07-17 — Isolated runtime and Railway deployment

- The runtime added immutable generation planning, building, serving, and
  private worker reconciliation.
- These features supported one repository and multiple routed knowledge bases.
- The runtime separated GitHub publication from model execution.
- The publisher, serve service, and each harness worker used distinct images,
  credentials, volumes, and network boundaries.
- The CLI added `deploy railway` with secret-free dry runs and explicit
  mutation consent.
- The command also provided idempotent state, generated, custom, or private
  endpoint modes, and worker inference from enabled jobs.
- The runtime added authenticated private artifact and Git-bundle exchange for
  providers without shared volumes.
- Invalid updates retained the last verified generation.
- Source: `packages/cli/cmd/openknowledge/runtime_*.go`,
  `packages/cli/cmd/openknowledge/deploy_command.go`, `packages/cli/internal/runtime/`,
  `docker/runtime.Dockerfile`, `deploy/runtime/`.

### 2026-07-17 — Explicit publication contract

- Public HTML, portable public source, and runtime generation failed closed
  unless `[publish] enabled = true`.
- The project added `okf_targets.viewer|search|mcp|llms|sitemap`.
- The runtime used separate projections for the viewer, search, and MCP.
- Public non-Markdown files included only `[publish].assets`.
- Artifacts excluded project configuration, `.openknowledge` state, denied
  Markdown, and assets outside the allowlist.
- Source: `packages/cli/internal/okf/{project_config,publish}.go`,
  `packages/cli/internal/runtime/generation.go`.

### 2026-07-15 — Machine and retrieval contracts

- The CLI added versioned JSON envelopes and published Draft 2020-12 schemas.
- The schemas covered CLI errors, AST, validation, bundle, list, registry,
  search, context, federation, graphs, jobs, portable manifests, and storage
  records.
- The root command added the `--error-format text|json` diagnostic envelope.
- Search added revision-bound provenance with content digests and
  `okf+sha256://` locators.
- Search also added registry-wide reciprocal-rank fusion.
- The project added a public read-only Go API. The API supported parsing,
  validation, retrieval, graphs, and registry resolution.
- Source: `packages/cli/schemas/`, `packages/cli/internal/okf/`,
  `packages/cli/okf/`.

### 2026-07-15 — Remote registry integrity

- The registry added strict versioned registry and provenance storage.
- It also added offline integrity status with atomic refresh and deletion.
- The registry added Git ref and monorepo subdirectory selection.
- It also added source-addressed caches, bounded non-interactive transport,
  archive limits, staging-tree limits, and secret-safe URL handling.
- Remote materialization used locked sibling staging and transactional
  publication.
- A failed refresh preserved the previous generation.
- Source: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/internal/okf/registry.go`, `packages/cli/schemas/storage/v1/`.

### 2026-07-15 — Viewer, packaging, and release hardening

- The project unified local viewer search with canonical heading-section
  retrieval and content-bound cache invalidation.
- Registry workspaces followed live validated snapshots.
- The project strengthened static serving, containers, release permissions,
  workflow pins, scheduled security scans, and npm or shell binary
  installation.
- The release process added reproducible portable archives and transactional
  export publication.
- It also added signed release provenance and default-branch-only release
  dispatch.
- Source: `packages/cli/cmd/openknowledge/viewer*.go`, `packages/web/`,
  `install`, `packages/npm/`, `.github/workflows/`, `Dockerfile`.

## v0.6.1 — 2026-07-18

- The runtime corrected Railway persistent-volume ownership for isolated
  runtime roles.

## v0.6.0 — 2026-07-18

### 2026-07-09 — Retrieval and viewer polish

- Search used source-preserving Markdown context as its default output.
- The viewer added typed frontmatter inspection, tag facets, and breadcrumbs.
- It also added reading and accessibility settings, better search navigation,
  and visual updates.
- The public website and wiki landing page matched the LLM-oriented knowledge
  workflow.

## v0.5.0 — 2026-07-08

- The CLI added source-to-wiki prompts, maintenance rules, advisory review, and
  the first experimental local job runner.
- It added exact `get`, structural `list`, ranked `search`,
  registry-backed `view`, and search graph workflows.
- The viewer added static discovery files, analytics and head injection, and
  portable viewer connection assets.
- Validation expanded its JSON output and added configurable rule severities.

## v0.4.0 — 2026-06-23

- The CLI added AST output, source and search graph exporters, and
  query-oriented context.
- Bundle commands added key-or-path resolution.
- The viewer improved themes, search highlights, shortcuts, panel navigation,
  responsive layout, and graph presentation.
- The project added website deployment, install redirects, and static wiki
  publication.

## v0.3.0 — 2026-06-20

- The CLI added connected bundle commands and registry-backed local viewing.
- The exporter added static HTML, portable manifests, and bundle metadata.
- The viewer added themes, syntax highlighting, tables, asset previews, and
  source links.
- Validation strengthened its UTF-8, frontmatter, Markdown, link, and
  reserved-file checks.
- The project moved CLI documentation into this colocated OKF wiki.

## Initial wiki maintenance — 2026-06-18

- The project added the repository wiki, embedded OKF specification, command
  references, update workflows, and validation loop.

---

<!-- okf-footer: agent-maintenance -->

> **Update notes**
>
> Add concise release-facing changes under `Unreleased`.
>
> Group related commits by user outcome.
>
> Do not recreate per-command implementation logs.
