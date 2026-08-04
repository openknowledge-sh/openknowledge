---
type: Changelog
title: CLI Changelog
description: Release-level history for the Open Knowledge CLI.
tags: [openknowledge, cli, changelog]
timestamp: 2026-08-04T00:00:00Z
---

# CLI Changelog

Current behavior belongs in the [command reference](/features/commands/). This
page records release-level changes.

## Unreleased

### 2026-08-04 — End timed-out Windows job processes

- Windows job runs now stop the complete child process tree after a command
  timeout.
- The CLI releases private run logs before it returns.
- Source: `packages/cli/internal/agents/process_group_windows.go`,
  `packages/cli/internal/agents/runner.go`.
- Docs: `Wiki/features/commands/jobs.md`.

### 2026-08-04 — Keep Git refresh staging stable

- Git resource validation now tolerates temporary Git-internal files that
  disappear during a scan.
- Missing bundle files and missing staging roots still stop the refresh.
- Source: `packages/cli/cmd/openknowledge/registry_command.go`.
- Docs: `Wiki/features/commands/connect.md`,
  `Wiki/features/commands/registry.md`.

## v0.10.0 — 2026-08-04

Version 0.10 makes OKF 0.2 the default, unifies setup, expands provenance
views, and improves viewer navigation and local automation.

### 2026-08-04 — Navigate immediately on mobile

- On screens up to 680 px, sidebar and note transitions no longer delay
  navigation.
- Selecting a sidebar link closes the sidebar and shows its destination
  immediately.
- Source: `packages/web/src/viewer/`,
  `packages/web/scripts/browser.e2e.mjs`.
- Docs: `Wiki/features/commands/view.md`.

### 2026-08-04 — Use local knowledge consistently on Windows

- `okn connect` reads canonical Windows `file://` URLs for manifests and
  archives. Windows drive URLs use `file:///C:/path`.
- Insight creation reports stable slash-separated project paths on all
  operating systems.
- Source: `packages/cli/cmd/openknowledge/registry_command.go`,
  `packages/cli/cmd/openknowledge/insights_command.go`.
- Docs: `Wiki/features/commands/connect.md`,
  `Wiki/features/commands/insights.md`.

### 2026-08-04 — Adapt the viewer to your workspace

- The file explorer stays visible and opaque during open-beside navigation.
- Resize the explorer from its `25vw` default within its minimum and maximum
  limits.
- The viewer keeps the header, note workspace, and horizontal scroll rail in
  the main content column.
- Source: `packages/web/src/viewer/`,
  `packages/web/scripts/browser.e2e.mjs`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
- Docs: `Wiki/features/commands/view.md`.

### 2026-08-04 — Use OKF 0.2 across CLI and viewer surfaces

- `okn list` shows trust, lifecycle status, and stale content. List and graph
  JSON also expose a derived `okf02` contract.
- Graph output connects sources, computations, executors, and attesters with
  typed provenance edges.
- Viewer pages show freshness, provenance, linked sources, and Attested
  Computation contracts.
- Plain HTML includes semantic frontmatter and links source footnotes to
  structured source records.
- The CLI preserves executor and attester declarations. It never runs these
  resources automatically.
- Source: `packages/cli/internal/okf/okf_0_2_signals.go`,
  `packages/cli/internal/okf/graph.go`,
  `packages/cli/internal/okf/html_frontmatter.go`,
  `packages/cli/cmd/openknowledge/viewer_frontmatter.go`,
  `packages/web/src/viewer/`.
- Docs: `Wiki/features/spec-compliance.md`,
  `Wiki/features/commands/list.md`,
  `Wiki/features/commands/view.md`,
  `Wiki/features/exporters/graph.md`,
  `Wiki/features/exporters/html.md`,
  `Wiki/features/go-api.md`,
  `Wiki/features/machine-contracts.md`.

### 2026-08-03 — Create and validate OKF 0.2 knowledge bases by default

- The `latest` selector now uses OKF 0.2. Explicit OKF 0.1 reads remain
  available.
- The parser normalizes a single `verified` mapping for OKF 0.2 consumers.
- Validation reports malformed optional provenance, trust, lifecycle, source,
  and Attested Computation metadata as warnings.
- Each validation uses the rule profile for its selected OKF version. Shared
  configuration can contain rules from multiple profiles.
- New scaffolds use OKF 0.2. Use `okn scaffold --spec 0.1` to create a complete
  version-matched OKF 0.1 scaffold.
- The scaffold handoff validates the selected OKF version.
- New insights use OKF 0.2 lifecycle and `generated` metadata. Status changes
  convert legacy insight provenance to OKF 0.2.
- Source: `packages/cli/internal/okf/`,
  `packages/cli/internal/okf/assets/specs/0.2.md`,
  `packages/cli/internal/insights/`,
  `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/main_test.go`.
- Docs: `README.md`, `Wiki/SPEC.md`,
  `Wiki/features/spec-compliance.md`,
  `Wiki/features/commands/scaffold.md`,
  `Wiki/features/commands/insights.md`.

### 2026-08-02 — Complete setup in one workflow

- `okn setup` starts a terminal wizard. Without terminal input, it prints a
  complete task for an agent.
- Explicit flags print the task, start the wizard, or launch a supported agent
  harness.
- `okn setup complete` validates and connects the bundle. It also installs
  selected skills and configures optional observation.
- Setup commands now report status, repair managed files, and control
  observation.
- The CLI removed `integration`, `agent integrate`, `prompt setup`, and
  `prompt from`.
- Source-based setup no longer requires a predefined knowledge-base type.
- Bundle configuration now uses `.openknowledge.toml`. The CLI does not load
  the legacy name.
- Both configuration names remain private in viewer and publication output.
- Source: `packages/cli/cmd/openknowledge/setup_command.go`,
  `packages/cli/cmd/openknowledge/setup_lifecycle_command.go`,
  `packages/cli/internal/integration/`, `packages/cli/internal/okf/`.
- Docs: `README.md`, `Wiki/features/commands/setup.md`,
  `Wiki/features/configuration.md`.

### 2026-07-31 — Explore Mermaid diagrams in detail

- Open a rendered Mermaid diagram with a click, Enter, or Space.
- A viewport dialog provides toolbar, wheel, pinch, drag, and arrow-key
  controls for zoom and pan.
- **Fit** shows the complete diagram. **100%** centers it at the original
  scale.
- The local viewer and interactive HTML exports use the same controls.
- Source: `packages/web/src/viewer/`,
  `packages/web/scripts/browser.e2e.mjs`.
- Docs: `Wiki/features/commands/view.md`,
  `Wiki/features/exporters/html.md`.

### 2026-07-31 — Run each scheduled job once

- A scheduled job runs once for each job ID and schedule slot. Repository or
  job-file changes do not replay that slot.
- Each private jobs worker uses its runtime-specific state directory.
- Workers remove terminal worktrees and large temporary artifacts after
  proposal export.
- Publishers remove branch bundles after publication.
- Source: `packages/cli/internal/agents/plan.go`,
  `packages/cli/cmd/openknowledge/runtime_worker.go`.
- Docs: `Wiki/features/commands/jobs.md`,
  `Wiki/features/commands/runtime.md`.

### 2026-07-31 — Start setup with an existing agent

- The generated task starts with an open-ended setup interview for `Wiki`.
- The README and website show setup with an existing agent or a selected CLI
  runtime.
- The website copy action includes agent installation, verification, and setup
  instructions.
- The README workflow diagram includes MCP.
- Source: `packages/cli/cmd/openknowledge/setup_command.go`,
  `packages/web/index.html`.
- Docs: `README.md`, `Wiki/features/commands/setup.md`.

## v0.9.0 — 2026-07-30

### 2026-07-30 — Automatic release version update

- The manual release workflow now updates all package and CLI versions from
  the requested release version.
- The workflow runs the release checks before it creates and pushes the version
  commit. Release tags and npm packages use that verified commit.
- Source: `.github/workflows/release.yml`,
  `scripts/set-release-version.mjs`.
- Docs: `Wiki/features/operations.md`.

### 2026-07-30 — Streamlined multi-panel viewer

- The viewer no longer duplicates open panels in a fixed bottom navigator.
- Links open beside the active panel by default. The current panel mode remains
  available through the header control.
- Each panel keeps its close control. The horizontal scroll rail remains
  available for multi-panel navigation.
- Source: `packages/web/src/viewer/`,
  `packages/web/scripts/browser.e2e.mjs`.
- Docs: `Wiki/features/commands/view.md`.

### 2026-07-29 — Focused viewer navigation

- A persistent header control now selects the current panel mode or the
  open-beside mode. Shift temporarily uses the other mode.
- A visible navigator now lists open panels and can close one panel or all
  panels.
- Search results now group section matches by document. The file explorer now
  reveals the active branch and can collapse directories.
- The knowledge graph now reports its selected note and connection count.
  A left detail panel now keeps graph instructions outside the canvas.
  High-contrast mode also applies to graph colors.
- Viewer settings now provide a reset-to-defaults action.
- Source: `packages/web/src/viewer/`,
  `packages/web/scripts/browser.e2e.mjs`.
- Docs: `Wiki/features/commands/view.md`.

### 2026-07-29 — Unified product story

- The website, README, npm package, wiki, setup prompt, and root help now use one
  product description.
- The description presents flexible Markdown knowledge bases that agents can
  create, retrieve, maintain, validate, and publish for different use cases.
- Source: `packages/web/`, `packages/npm/README.md`,
  `packages/cli/{cmd/openknowledge,internal/okf/setup.go}`.
- Docs: `README.md`, `Wiki/index.md`.

### 2026-07-28 — Shared Vite viewer build

- Vite built the landing page and the shared viewer assets.
- The web workspace added TypeScript checks and Oxlint.
- Local and static viewers used the same generated JavaScript and CSS bundle.
- Static HTML pages referenced one shared data file instead of embedding the
  complete note collection in every page.
- Static exports became independent of installed editor applications and
  supported nested pages through direct `file://` URLs.
- Browser tests covered Mermaid errors and direct file viewing.
- Source: `packages/web/src/`, `packages/web/vite*.config.ts`,
  `packages/cli/cmd/openknowledge/{viewer_export,viewer_assets}.go`.
- Docs: `Wiki/features/{exporters/html,commands/view,operations}.md`.

### 2026-07-28 — Explicit automation namespace

- `okn automation` became the canonical namespace for `jobs`, `insights`,
  `runtime`, and `deploy`.
- Root help separated local work, sharing, and automation.
- The old top-level forms remained functional as hidden compatibility aliases.
- JSON error envelopes reported the canonical automation command identity for
  both forms.
- Generated job and deployment commands used the automation namespace.
- Source: `packages/cli/cmd/openknowledge/{automation_command,command_catalog}.go`.
- Docs: `Wiki/features/commands/{automation,index,help}.md`,
  `Wiki/features/tooling-model.md`.

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
  `packages/web/src/viewer/`.
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
