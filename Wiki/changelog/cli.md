---
type: Changelog
title: CLI Changelog
description: Maintained changelog memory for Open Knowledge CLI package changes.
tags: [openknowledge, cli, changelog]
timestamp: 2026-06-18T00:00:00Z
---

# CLI Changelog

This page records CLI-facing package changes in a developer-focused format.
Entries should summarize what changed, why it matters, source anchors, and docs
that were updated.

## Unreleased

### 2026-07-15 - Versioned agent artifact contracts

* Added `schemaVersion: "1"` to runtime-built agent run plans, dry-run JSON,
  persisted `plan.json`, and every lifecycle shape of `run.json`.
* Published closed Draft 2020-12 `agent-run-plan` and `agent-run-record`
  schemas covering nested commands, sandbox/output capabilities, concurrency,
  states, timings, logs, failures, and embedded plans.
* Added golden plan/record fixtures to the shared offline schema suite,
  including undeclared top-level and nested-field rejection.
* Added a real Git-backed `BuildRunPlan` and successful `RunJob` contract test
  that validates serialized runtime artifacts against their public schemas.
* Extended website schema distribution and machine-contract documentation to
  include both agent artifact contracts.
* Source anchors: `packages/cli/internal/agents/plan.go`,
  `packages/cli/internal/agents/runner.go`,
  `packages/cli/internal/agents/schema_contract_test.go`,
  `packages/cli/internal/agents/testdata/contracts/`,
  `packages/cli/internal/okf/schema_contract_test.go`, and
  `packages/cli/schemas/v1/agent-run-{plan,record}.schema.json`.
* Docs updated: `README.md`, command help,
  `packages/cli/schemas/v1/README.md`,
  `Wiki/features/commands/agents.md`,
  `Wiki/features/machine-contracts.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Secret-safe remote source URLs

* Added shared pre-I/O validation for connection, refresh, manifest archive,
  and redirect URLs before they can reach transport or durable provenance.
* Rejected HTTP userinfo, URL passwords, fragments, remote `file://` hosts,
  malformed queries, and recognized credential query fields without echoing
  their values in errors.
* Required Git secrets to resolve through SSH keys or credential helpers and
  made URL-embedded authentication explicitly unsupported for HTTP bundles.
* Preserved the ten-hop redirect bound and rejected credential-bearing
  destinations plus HTTPS-to-HTTP downgrade redirects.
* Added table-driven no-I/O rejection, no-secret-error, allowed transport, and
  redirect-boundary regression coverage.
* Source anchors: `packages/cli/cmd/openknowledge/main.go` and
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `README.md`, command help,
  `Wiki/features/commands/registry.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Enforced cross-process agent concurrency

* Promoted the reserved agent-job `concurrency` mapping into a strict contract
  with a global key and fail-closed `skip` policy validation.
* Added owner-private hashed lock files under the external agent state root and
  hold the lock across worktree creation, execution, verification, optional
  commit, and final artifact recording, including across daemon processes.
* Contending invocations now persist a private `run.json` with status `skipped`
  and a reason, while creating no worktree and executing no agent command.
* Included normalized concurrency state in dry-run and persisted run plans;
  missing policies default to `skip` when a key is present.
* Added parser, defaulting, validation, real lock-contention, persisted-record,
  and no-worktree regression coverage.
* Source anchors: `packages/cli/internal/agents/concurrency.go`,
  `packages/cli/internal/agents/frontmatter_schema.go`,
  `packages/cli/internal/agents/spec.go`,
  `packages/cli/internal/agents/plan.go`,
  `packages/cli/internal/agents/runner.go`, and
  `packages/cli/internal/agents/runner_test.go`.
* Docs updated: `README.md`, generated agent-job reference,
  `Wiki/features/commands/agents.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Bounded non-interactive Git transport

* Added one two-minute deadline across remote Git clone/fetch/checkout
  materialization so unattended connection and refresh jobs cannot hang
  indefinitely.
* Forced terminal and Git Credential Manager prompts off for remote Git
  subprocesses; credentials must resolve non-interactively.
* Replaced unbounded `CombinedOutput` allocation with a draining writer that
  retains at most 256 KiB and marks truncated diagnostics.
* Added subprocess-backed regression coverage for prompt overrides, timeout
  termination, and bounded high-volume error output.
* Source anchors: `packages/cli/cmd/openknowledge/main.go` and
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `README.md`, command help,
  `Wiki/features/commands/registry.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Live registry viewer snapshots

* Changed the long-running registry viewer from a startup-only connection list
  to a bounded, strictly validated snapshot loaded for every request.
* Rebuilds registry routing and bundle handlers when connections are added or
  removed or when refresh changes a path, access capability, or provenance;
  unchanged snapshots retain their existing handlers and search indexes.
* Returns HTTP `500` when the current registry cannot be loaded instead of
  silently continuing to expose stale routes.
* Added regression coverage for updated roots and access, new connections,
  removals, and registry loader failures.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go` and
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/view.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Versioned fail-closed registry persistence

* Added shared strict JSON decoding that rejects unknown fields, duplicate
  object keys at any depth, and trailing JSON; reused it for portable manifests,
  registry storage, and remote-cache provenance.
* Versioned all new registry writes with `schemaVersion: "1"`, retained
  unversioned registry reads only as a migration path, and upgraded them on the
  next successful atomic mutation.
* Added pre-mutation validation for registry versions, canonical absolute
  paths, key syntax, access values, and duplicate logical names; corrupt input
  is preserved unchanged on refusal.
* Fixed provenance verification so a sidecar's recorded `managedRoot` must
  match its actual generation instead of being silently overwritten.
* Capped registry reads at 8 MiB and individual provenance sidecars at 1 MiB
  before allocation and decoding.
* Published recursive Draft 2020-12 registry, cache-sidecar, and shared-source
  schemas under `/schemas/cli/storage/v1/` and validated current producer
  objects offline against them.
* Source anchors: `packages/cli/internal/okf/strict_json.go`,
  `packages/cli/internal/okf/registry.go`,
  `packages/cli/cmd/openknowledge/main.go`, and
  `packages/cli/schemas/storage/v1/`.
* Docs updated: `README.md`, `packages/cli/schemas/v1/README.md`,
  `Wiki/features/machine-contracts.md`, `Wiki/features/operations.md`,
  `Wiki/features/commands/registry.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Git revision and monorepo bundle selection

* Added `--git-ref <branch|tag|commit>` and `--git-subdir <path>` to
  `openknowledge connect` and `registry connect`.
* Fetches selected refs at depth one, checks out the resolved commit detached,
  and validates/registers the exact canonical repository subdirectory while
  retaining the complete repository as the managed generation.
* Included Git selectors in cache identity and recorded `gitRef`, `gitSubdir`,
  and exact `gitCommit` provenance so different repository views cannot alias.
* Made `registry refresh` retain selectors while atomically advancing moving
  branches/tags; offline status still verifies complete content, commit,
  working-tree cleanliness, and sidecar provenance.
* Rejected invalid refs, traversal/absolute/non-canonical subdirectories, and
  Git selectors on local, manifest, or archive inputs before materialization.
* Extended registry list/status schemas and golden coverage, plus end-to-end
  connect/refresh/integrity tests against a real monorepo-style Git repository.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/cli/internal/okf/registry.go`, and
  `packages/cli/schemas/v1/registry-{list,status}.schema.json`.
* Docs updated: `README.md`, `Wiki/features/commands/connect.md`,
  `Wiki/features/commands/registry.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Public read-only Go API

* Added the importable
  `github.com/openknowledge-sh/openknowledge/packages/cli/okf` package so Go
  ecosystem tools no longer need to spawn the CLI for in-process knowledge
  parsing and retrieval.
* Exposed version-aware AST and normalized parsing, validation and policy
  options, listing, deterministic search, budgeted context, source/search
  graphs, metadata, frontmatter, strict manifest decoding, and spec discovery.
* Kept all returned models as aliases of the real CLI core types, preventing a
  parallel SDK model or serializer from drifting away from command, schema,
  and MCP behavior.
* Kept registry/network mutation, extraction, publishing, and viewer lifecycle
  outside the read-only facade as explicit operational CLI boundaries.
* Added an external-package integration test that imports only the public path
  and exercises all major views against a real temporary bundle.
* Source anchors: `packages/cli/okf/` and
  `packages/cli/internal/okf/ast_frontmatter_types.go`.
* Docs updated: `README.md`, `packages/cli/okf/README.md`,
  `Wiki/features/go-api.md`, `Wiki/features/index.md`,
  `Wiki/features/tooling-model.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Schema-backed strict portable manifests

* Added the Draft 2020-12 schema for portable `openknowledge.json` discovery
  manifests with an independent manifest-protocol version and concrete OKF
  spec version.
* Made remote manifest decoding reject unknown fields, duplicate object keys,
  trailing JSON, and invalid semantic identity or archive fields before any
  archive is downloaded.
* Extended offline schema compilation to the complete recursive CLI schema
  tree and added positive runtime-manifest plus negative contract mutations.
* Made the website recursively verify each schema `$id` against its exact
  public route and publish the manifest schema at
  `/schemas/cli/manifest/v1/bundle.schema.json`.
* Source anchors: `packages/cli/internal/okf/archive.go`,
  `packages/cli/internal/okf/archive_test.go`,
  `packages/cli/internal/okf/schema_contract_test.go`,
  `packages/cli/schemas/manifest/v1/`, and
  `packages/web/scripts/schema-distribution.mjs`.
* Docs updated: `README.md`, `packages/cli/schemas/v1/README.md`,
  `Wiki/features/machine-contracts.md`, `Wiki/features/operations.md`,
  `Wiki/features/exporters/html.md`, `Wiki/features/exporters/tar.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Enforced and publicly distributed JSON schemas

* Added offline Draft 2020-12 compilation and semantic validation for every
  published CLI schema using the pinned test-only
  `github.com/santhosh-tekuri/jsonschema/v6` `v6.0.2` validator.
* Validated all golden fixtures plus representative non-empty AST, normalized
  bundle, source/search graph, list, search match/context, validation, and
  registry outputs from real builders.
* Added negative mutation coverage requiring every schema to reject
  undeclared top-level fields and nested document, metadata, Markdown, file,
  node, edge, source, result, check, and registry-entry fields.
* Replaced open-ended AST metadata/Markdown/diagnostic, graph node/edge,
  search-context source, and validation-check contracts with explicit closed
  Draft 2020-12 definitions; tightened numeric and relation constraints.
* Made the website build publish the schema tree at `/schemas/cli/v1/`, exactly
  matching the existing `https://openknowledge.sh/schemas/cli/v1/...` `$id`
  values, and added hermetic positive and mismatched-ID tests.
* The schema validator remains test-only and is not linked into the shipped
  CLI binary.
* Source anchors: `packages/cli/internal/okf/schema_contract_test.go`,
  `packages/cli/schemas/v1/`, `packages/cli/go.mod`,
  `packages/web/scripts/schema-distribution.mjs`,
  `packages/web/scripts/schema-distribution.test.mjs`, and
  `packages/web/scripts/build.mjs`.
* Docs updated: `README.md`, `packages/cli/schemas/v1/README.md`,
  `Wiki/features/machine-contracts.md`, `Wiki/features/tooling-model.md`,
  `Wiki/features/operations.md`, `Wiki/index.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Versioned registry discovery

* Added `openknowledge registry list --json` as a cheap machine-readable
  discovery API that does not inspect or validate connected bundle contents.
* Defined a `schemaVersion: "1"` envelope with the registry path and sorted
  entries containing name, path, effective read/write capability, managed
  state, and optional source provenance.
* Added the Draft 2020-12 `registry-list.schema.json` contract, a golden output
  fixture, schema registry coverage, empty-array guarantees, and command tests
  for sorting, local and managed metadata, help, and argument rejection.
* Added dedicated `registry list --help` and documented when consumers should
  select list discovery versus the deeper `registry status --json` integrity
  report.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/cli/cmd/openknowledge/testdata/contracts/registry-list.json`, and
  `packages/cli/schemas/v1/registry-list.schema.json`.
* Docs updated: `README.md`, `packages/cli/schemas/v1/README.md`,
  `Wiki/features/commands/registry.md`, `Wiki/features/commands/help.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Read-only MCP knowledge server

* Added `openknowledge mcp [key-or-path]` as a first-class stdio MCP server
  scoped to one canonical local or connected knowledge base.
* Implemented the stable MCP `2025-11-25` initialize lifecycle, compatibility
  negotiation for released earlier protocol revisions, ping, strict one-line
  JSON-RPC parsing, and resources/tools capability discovery.
* Exposed canonical bundle inventory files through paginated
  `resources/list`, exact text or base64 `resources/read`, and an empty
  resource-template list.
* Added read-only `openknowledge_search` and `openknowledge_validate` tools
  with strict input schemas, MCP annotations, structured results, and JSON
  text compatibility blocks.
* Confined resource reads to listed regular files inside the real bundle root,
  rejected `.git` guessing, traversal and nested symlinks, and bounded messages,
  search inputs, and individual resource reads.
* Added end-to-end protocol tests covering lifecycle, discovery, retrieval,
  validation, invalid requests and arguments, URI boundaries, pagination
  cursors, and resource limits.
* Source anchors: `packages/cli/cmd/openknowledge/mcp.go`,
  `packages/cli/cmd/openknowledge/mcp_test.go`, and
  `packages/cli/cmd/openknowledge/main.go`.
* Docs updated: `README.md`, `Wiki/index.md`,
  `Wiki/features/commands/index.md`, `Wiki/features/commands/mcp.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Hardened production static server

* Split the production web entrypoint from a directly testable static handler
  and added hermetic Node tests that require no listening socket.
* Limited requests to `GET` and `HEAD`, drained rejected bodies, handled
  malformed URLs deterministically, implemented real HEAD metadata, and added
  explicit no-store/error and short asset/HTML cache policies.
* Added CSP, HSTS, permissions, referrer, opener, framing, and MIME-sniffing
  defenses to every application response while retaining the trusted inline
  viewer/head-injection contract.
* Resolved final file targets through the real filesystem to prevent symlink
  escapes, expanded explicit MIME coverage, and contained stream errors and
  client disconnects.
* Bounded request headers, header/request/keep-alive time, and requests per
  socket.
* Added `pnpm test:web` coverage for files, directories, Wiki fallback, HEAD,
  invalid methods/URLs, redirects, caches, security headers, symlinks, and
  resource limits, then wired it into the root test gate.
* Source anchors: `packages/web/scripts/server.mjs`,
  `packages/web/scripts/server.test.mjs`,
  `packages/web/scripts/serve.mjs`, `packages/web/package.json`, and
  `package.json`.
* Docs updated: `README.md`, `Wiki/features/operations.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Unprivileged production web runtime

* Selected the official Node image's built-in `node` user in the final
  production stage while retaining root-owned static assets and the high-port
  Railway listener.
* Added `pnpm check:container-runtime` to require a separate runtime stage and
  reject a missing, root, numeric-root, or unexpected final user.
* Wired the container policy into the root test gate and verified it failed on
  the previous Dockerfile before `USER node` was added.
* Source anchors: `Dockerfile`, `scripts/check-container-runtime.mjs`, and
  `package.json`.
* Docs updated: `README.md`, `Wiki/features/operations.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Transactional shell installation

* Replaced full tar extraction with stdout extraction of only the named
  `openknowledge` member after checksum verification, eliminating archive path
  writes during installation.
* Validated requested versions and checksum syntax, added bounded download
  connection/runtime retries, and required HTTPS with TLS 1.2 for default
  GitHub release traffic while preserving explicit local base-URL testing.
* Staged the candidate in the destination filesystem, executed its version
  preflight there for `noexec` temporary-directory compatibility, required a
  semantic and exact requested version, and atomically renamed it only after
  all checks passed.
* Preserved existing binaries across checksum, archive, execution, version, and
  destination failures; rejected non-file destinations and cleaned staging on
  every exit path.
* Added `pnpm test:install` coverage for success, mismatch/failure preservation,
  missing members, destination types, and cleanup, then wired it into the root
  test gate.
* Source anchors: `install`, `scripts/test-install.sh`, and `package.json`.
* Docs updated: `README.md`, `Wiki/features/installation.md`,
  `Wiki/features/operations.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Default-branch-only release dispatch

* Added an early release preflight that accepts only the repository default
  branch ref, refetches it, and requires the checked-out commit to equal its
  current tip before version resolution or any secret-bearing step.
* Made stale dispatches, feature-branch selections, and tag selections fail
  before tag creation, GoReleaser, or npm publication.
* Extended the workflow permission policy to lock checkout, default-branch
  verification, and version resolution as the verifier's first three steps.
* Documented the remaining administrative trust boundary: branch protection
  and a `v*` tag ruleset are required to constrain writers who could change the
  workflow itself, and those external settings are not inferred from repo code.
* Source anchors: `.github/workflows/release.yml` and
  `scripts/check-workflow-permissions.mjs`.
* Docs updated: `Wiki/features/operations.md` and `Wiki/changelog/cli.md`.

### 2026-07-15 - Pinned GoReleaser toolchain

* Replaced the GoReleaser action's dynamic `version: latest` download with the
  exact stable `v2.17.0` release, keeping the privileged publication job
  reproducible across reruns.
* Extended `pnpm check:workflow-pins` to reject `latest`, `current`, `nightly`,
  and wildcard aliases in workflow tool-version inputs.
* Verified the new guard failed on the previous release workflow before the
  concrete version was applied.
* Source anchors: `.github/workflows/release.yml` and
  `scripts/check-workflow-pins.mjs`.
* Docs updated: `Wiki/features/operations.md` and `Wiki/changelog/cli.md`.

### 2026-07-15 - Isolated release write capability

* Split release verification from GitHub publication: dependency setup, tests,
  builds, Wiki validation, npm package inspection, and credential preflight now
  run under the workflow's default read-only repository permission.
* Added a minimal `publish_release` job that receives `contents: write` only
  after verification and contains exactly checkout of the verified commit, tag
  preparation, and GoReleaser publication.
* Kept npm publication dependent on both verification and GitHub publication,
  while sourcing version, tag, and dist-tag values from the read-only verifier.
* Added `pnpm check:workflow-permissions` to reject unreviewed write scopes and
  privileged release-step expansion; verified its negative path with an unsafe
  write-capable job fixture.
* Source anchors: `.github/workflows/release.yml`,
  `scripts/check-workflow-permissions.mjs`, and `package.json`.
* Docs updated: `README.md`, `Wiki/features/operations.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Step-scoped workflow secrets

* Removed Railway credentials and configuration secrets from deploy job-level
  environment variables so checkout and future preparation steps cannot read
  them.
* Exposed only the token and service selector to the configuration check, then
  exposed the complete Railway environment solely to the actual deploy step.
* Added `pnpm check:workflow-secret-scope` to reject repository-secret
  expressions outside explicit consuming steps and to forbid blanket
  `secrets: inherit` forwarding.
* Added the secret-scope check to `pnpm test`, which makes the boundary part of
  CI, Railway verification, and release preflight; verified its negative path
  with an unsafe job-level regression fixture.
* Source anchors: `.github/workflows/deploy-railway.yml`,
  `scripts/check-workflow-secret-scope.mjs`, and `package.json`.
* Docs updated: `README.md`, `Wiki/features/operations.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Immutable Railway deploy image

* Replaced the mutable `ghcr.io/railwayapp/cli:latest` deploy environment with
  Railway CLI `4.65.0` pinned to its exact linux/amd64 GHCR manifest digest.
* Extended `pnpm check:workflow-pins` to reject undigested job containers and
  service images, closing the container equivalent of mutable action refs.
* Verified the guard first failed on the previous `latest` reference and then
  passed only after the digest pin was present.
* Source anchors: `.github/workflows/deploy-railway.yml`,
  `scripts/check-workflow-pins.mjs`, and `package.json`.
* Docs updated: `Wiki/features/operations.md` and `Wiki/changelog/cli.md`.

### 2026-07-15 - Immutable GitHub Action dependencies

* Replaced every mutable major-version action reference in CI, Railway deploy,
  and release workflows with the full commit SHA of its corresponding release.
* Retained human-readable release comments beside each SHA so dependency
  updates remain reviewable without sacrificing immutability.
* Added `pnpm check:workflow-pins`, which scans every workflow and rejects
  remote actions without a 40-character commit SHA as well as Docker actions
  without an SHA-256 digest.
* Added the workflow-pin check to `pnpm test`, so CI, deployment verification,
  and release quality gates prevent mutable action refs from returning.
* Source anchors: `.github/workflows/ci.yml`,
  `.github/workflows/deploy-railway.yml`, `.github/workflows/release.yml`,
  `scripts/check-workflow-pins.mjs`, and `package.json`.
* Docs updated: `README.md`, `Wiki/features/operations.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Enforced registry access capabilities

* Turned registry `read` and `write` values from display-only labels into
  enforced local authoring capabilities.
* Hid viewer editor deeplinks for read-only connections and allowed them for
  unregistered paths or local write connections.
* Guarded `rules apply` writes by canonical registered root while retaining
  dry-run behavior; nested roots use the most-specific connection and symlink
  aliases cannot bypass the boundary.
* Made managed manifest, archive, and Git cache generations unconditionally
  read-only, rejected remote write requests before materialization, and
  normalized legacy or forged managed write entries to read.
* Preserved managed provenance when a cache path is reconnected locally and
  rejected replacement snapshots that try to downgrade a managed connection.
* Kept registry administration and paths outside registered roots unaffected,
  and documented that the capability is not an operating-system ACL.
* Added registry, nested-root, symlink, legacy, no-materialization, command,
  and viewer regressions.
* Source anchors: `packages/cli/internal/okf/registry.go`,
  `packages/cli/internal/okf/registry_test.go`,
  `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/cli/cmd/openknowledge/viewer.go`, and
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/connect.md`,
  `Wiki/features/commands/registry.md`, `Wiki/features/commands/rules.md`,
  `Wiki/features/commands/view.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Unified strict project configuration

* Replaced separate validation, rules, theme, source, and site line scanners
  with one TOML v1-compatible typed `openknowledge.toml` parser.
* Added one fail-closed schema for `[rules]`, `[validation.rules]`,
  `[html.theme]`, `[html.source]`, and `[html.site]`; unknown sections/fields,
  duplicate keys, malformed syntax, wrong types, unknown rules, and invalid
  severities now fail consistently across consumers.
* Added standard TOML multiline arrays, inline comments, escaped/literal
  strings, and dotted-table behavior while retaining documented single-string
  shorthand for `rules.paths` and `rules.enabled`.
* Loaded all viewer/export HTML settings once per generation and removed legacy
  undocumented HTML key aliases.
* Applied the real bundle boundary to project config loading so a symbolic-link
  `openknowledge.toml` cannot redirect reads outside the bundle.
* Added shared parser, all-section projection, strictness, consumer parity,
  symlink, and no-partial-HTML regressions plus a dedicated configuration
  reference.
* Source anchors: `packages/cli/internal/okf/project_config.go`,
  `packages/cli/internal/okf/project_config_test.go`,
  `packages/cli/internal/okf/validation_policy.go`,
  `packages/cli/internal/okf/rule_catalog.go`,
  `packages/cli/cmd/openknowledge/viewer_theme.go`,
  `packages/cli/cmd/openknowledge/viewer.go`, and
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `README.md`, `Wiki/index.md`,
  `Wiki/features/configuration.md`, `Wiki/features/commands/rules.md`,
  `Wiki/features/commands/validate.md`,
  `Wiki/features/exporters/html.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Bounded agent process trees

* Added `verify.timeout` as a positive duration applied independently to each
  verification command, with a resolved default of `15m` in every run plan.
* Reported main-agent and verification timeouts distinctly from normal nonzero
  command exits.
* Replaced immediate-process-only cancellation with Unix process-group and
  Windows process-tree termination plus a bounded wait fallback, preventing
  background shell children from outliving timed-out jobs.
* Added schema, positive-duration, fast end-to-end verification timeout, and
  live descendant-process cleanup regressions.
* Cross-compiled the cancellation layer for Linux and Windows in addition to
  running the native macOS test suite.
* Source anchors: `packages/cli/internal/agents/spec.go`,
  `packages/cli/internal/agents/frontmatter_schema.go`,
  `packages/cli/internal/agents/plan.go`,
  `packages/cli/internal/agents/runner.go`,
  `packages/cli/internal/agents/process_group_unix.go`,
  `packages/cli/internal/agents/process_group_windows.go`,
  `packages/cli/internal/agents/process_group_other.go`,
  `packages/cli/internal/agents/process_group_unix_test.go`,
  `packages/cli/internal/agents/spec_test.go`, and
  `packages/cli/cmd/openknowledge/agents_command_test.go`.
* Docs updated: `Wiki/features/commands/agents.md` and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Strict agent job schema

* Added an explicit recursive schema for every supported agent-job top-level
  and nested frontmatter field.
* Rejected unknown fields, duplicate YAML keys, wrong mapping/scalar types,
  scalar list substitutes, and non-string list members instead of silently
  ignoring or coercing them.
* Rejected the parsed-but-unenforced `concurrency` surface so job authors cannot
  mistake an inert policy declaration for actual mutual exclusion.
* Made `schedule.cron` and `schedule.every` mutually exclusive and moved
  positive timeout/interval plus meaningful timezone checks into validation.
* Added table-driven schema regressions for typos, nested fields, booleans,
  lists, duplicates, concurrency, schedule ambiguity, duration bounds, and
  timezone dependencies.
* Source anchors: `packages/cli/internal/agents/frontmatter_schema.go`,
  `packages/cli/internal/agents/spec.go`, and
  `packages/cli/internal/agents/spec_test.go`.
* Docs updated: `Wiki/features/commands/agents.md` and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - External agent runtime state

* Moved agent run records and Git worktrees out of source repositories into a
  platform user-config state root, overridable with
  `OPENKNOWLEDGE_AGENTS_STATE_DIR`.
* Namespaced state by a readable repository basename plus a hash of its
  canonical path, preventing collisions between same-named repositories.
* Rejected state roots equal to or below the source repository after resolving
  symlinked existing parents, including paths whose final directories do not
  exist yet.
* Removed the runner's self-inflicted dirty checkout: multiple scheduled runs
  can now use the default `dirty_policy: fail` sequentially without runtime
  files blocking later jobs.
* Added two-run cleanliness, external-path, unsafe-root, no-write-on-refusal,
  symlink-canonicalization, and namespace-collision regressions.
* Source anchors: `packages/cli/internal/agents/plan.go`,
  `packages/cli/internal/agents/spec_test.go`,
  `packages/cli/internal/agents/templates.go`, and
  `packages/cli/cmd/openknowledge/agents_command_test.go`.
* Docs updated: `Wiki/features/commands/agents.md` and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Explicit agent environment capabilities

* Stopped host agent and verification processes from inheriting the complete
  Open Knowledge CLI environment, including unrelated ambient secrets.
* Added `sandbox.env` as an allowlist of variable names whose values may be
  inherited at execution time without being serialized into jobs or run plans.
* Gave host jobs private `HOME` and temp directories inside the owner-only run
  directory while retaining only a minimal cross-platform runtime environment.
* Forwarded only explicitly named variables into Docker containers and refused
  missing values before a real run starts.
* Rejected malformed, duplicate, and runner-managed environment names, including
  case variants that could bypass the boundary on case-insensitive platforms.
* Added host environment non-leakage, explicit inheritance, isolated path,
  Docker forwarding, schema, and filesystem mode regressions.
* Source anchors: `packages/cli/internal/agents/spec.go`,
  `packages/cli/internal/agents/runner.go`,
  `packages/cli/internal/agents/templates.go`,
  `packages/cli/internal/agents/spec_test.go`,
  `packages/cli/internal/agents/runner_test.go`, and
  `packages/cli/cmd/openknowledge/agents_command_test.go`.
* Docs updated: `Wiki/features/commands/agents.md` and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Private agent run artifacts

* Changed agent run and run-parent directory permissions from shared-readable
  defaults to owner-only `0700`.
* Forced copied jobs, rendered prompts, plans, run records, stdout/stderr logs,
  verification logs, and patches to `0600`, even when replacing an existing
  artifact with broader permissions.
* Added end-to-end mode assertions over every artifact produced by a successful
  host run, with platform-appropriate handling for Windows permissions.
* Source anchors: `packages/cli/internal/agents/runner.go` and
  `packages/cli/cmd/openknowledge/agents_command_test.go`.
* Docs updated: `Wiki/features/commands/agents.md` and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Hardened Docker agent sandbox

* Added `sandbox.network: none|bridge` and made Docker agent runs default to no
  network unless a job explicitly opts into Docker's bridge network.
* Dropped all Linux capabilities, prohibited new privileges, enabled an init
  process, and capped each Docker command at 512 PIDs while retaining the
  writable worktree mount required by agent jobs.
* Validated image values against option-shaped and whitespace-bearing input,
  and placed the image after an explicit Docker option boundary to prevent
  image fields from injecting `docker run` flags.
* Added exact argument-vector tests for direct and shell commands, default and
  opted-in networking, isolation flags, and image boundaries, plus sandbox
  schema regressions.
* Source anchors: `packages/cli/internal/agents/spec.go`,
  `packages/cli/internal/agents/plan.go`,
  `packages/cli/internal/agents/runner.go`,
  `packages/cli/internal/agents/templates.go`,
  `packages/cli/internal/agents/spec_test.go`, and
  `packages/cli/internal/agents/runner_test.go`.
* Docs updated: `Wiki/features/commands/agents.md` and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Fail-closed agent executor selection

* Restricted `agents run` and `agents daemon` executor overrides to the exact
  `host` and `docker` values already supported by job frontmatter.
* Rejected invalid overrides as CLI usage errors before job discovery or any
  process execution, preventing typos from falling through to host execution.
* Repeated executor normalization and validation in the run-plan API so direct
  package callers receive the same fail-closed behavior.
* Added command-level coverage for separated and `--executor=value` syntax on
  both run and daemon, plus package-level allowlist and planning regressions.
* Source anchors: `packages/cli/cmd/openknowledge/agents_command.go`,
  `packages/cli/cmd/openknowledge/agents_command_test.go`,
  `packages/cli/internal/agents/plan.go`, and
  `packages/cli/internal/agents/spec_test.go`.
* Docs updated: `Wiki/features/commands/agents.md` and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Reproducible portable archives

* Removed the destination basename from gzip headers and normalized gzip OS
  and timestamp metadata.
* Replaced host-derived tar headers with canonical regular-file headers: zero
  ownership and timestamps, empty owner names, stable PAX format, and `0644` or
  `0755` modes that retain executable intent.
* Made identical bundle content produce byte-identical archives and SHA-256
  values across output names, file timestamps, host identities, and
  non-executable permission differences.
* Added byte-for-byte and header-level regressions covering archive identity,
  owner metadata, timestamps, ordinary files, and executable files.
* Source anchors: `packages/cli/internal/okf/archive.go` and
  `packages/cli/internal/okf/archive_test.go`.
* Docs updated: `Wiki/features/commands/to.md`,
  `Wiki/features/exporters/tar.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Transactional exporter publication

* Changed default and plain HTML export to build complete sibling generations
  and switch them into place only after all files succeed.
* Removed stale pages and assets on successful rebuilds while preserving the
  complete previous site on generation failure.
* Rejected output directories that equal or contain the source bundle, and
  excluded supported nested outputs from the default viewer's portable source
  archive across repeated builds.
* Atomically replaced AST, validation JSON, normalized JSON, and graph files
  written through `--out` instead of truncating their destinations in place.
* Added helper, boundary, stale-generation, failure-preservation, and nested
  archive exclusion regressions.
* Source anchors: `packages/cli/internal/okf/atomic_output.go`,
  `packages/cli/internal/okf/html.go`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/ast_command.go`,
  `packages/cli/internal/okf/atomic_output_test.go`,
  `packages/cli/internal/okf/export_test.go`, and
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/to.md`,
  `Wiki/features/commands/ast.md`,
  `Wiki/features/commands/validate.md`,
  `Wiki/features/exporters/html.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Connectable portable export gate

* Required zero validation errors before standalone tar, plain HTML, or default
  connect-enabled HTML publication; warnings remain allowed.
* Applied the gate before output creation or replacement, preserving prior tar
  content and leaving HTML output untouched on invalid input.
* Unified producer behavior with remote `connect`, eliminating successful
  manifests whose archive would be rejected by the consumer.
* Added a shared validation failure formatter plus archive, plain HTML, and
  default viewer regressions for refusal and no partial writes.
* Source anchors: `packages/cli/internal/okf/validation_types.go`,
  `packages/cli/internal/okf/archive.go`,
  `packages/cli/internal/okf/html.go`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/internal/okf/archive_test.go`,
  `packages/cli/internal/okf/export_test.go`, and
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/to.md`,
  `Wiki/features/exporters/html.md`, `Wiki/features/exporters/tar.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Authenticated viewer network boundary

* Kept loopback as the zero-configuration default and refused wildcard or
  non-loopback `view --host` values without explicit `--allow-network`.
* Required one URL-safe access token across HTML, API, raw, and editor-icon
  routes for network binds; tokens may come from `--token`,
  `OPENKNOWLEDGE_VIEW_TOKEN`, or cryptographically secure generation.
* Added bearer authentication plus a browser bootstrap that exchanges the
  URL token for an HttpOnly SameSite=Strict cookie and removes the token from
  the address bar.
* Added constant-time comparisons, authentication challenges, no-store and
  common security headers, bounded request headers, and finite header/idle
  server timeouts.
* Added loopback/wildcard classification, explicit opt-in, token validation and
  generation, bearer, cookie, invalid token, redirect-stripping, header, and URL
  regression tests.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `packages/cli/cmd/openknowledge/main.go`, and
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/view.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Locked-down viewer asset serving

* Restricted viewer asset pages and `/raw/` to regular, non-Markdown files
  inside the real bundle boundary.
* Denied dotfile components, `.git`, `openknowledge.toml`, Markdown source,
  missing paths, directories, symlinks, and other non-regular entries; private
  assets are also absent from the viewer tree.
* Limited raw requests to `GET` and `HEAD`, returning `405` plus `Allow` for
  other methods, and added `nosniff`, no-referrer, and sandboxed CSP headers.
* Added route-level tests for regular assets, private/config paths, unknown
  files, Markdown, traversal, symlinks, method handling, headers, and tree
  visibility.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go` and
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/view.md` and `Wiki/changelog/cli.md`.

### 2026-07-15 - Real filesystem bundle boundaries

* Rejected symbolic links anywhere below a resolved knowledge bundle root so
  untrusted local or cloned bundles cannot redirect reads into host files.
* Added one real-path resolver for bundle-relative reads and applied it to
  `get`, viewer Markdown/raw serving, frontmatter reads, and local theme assets.
* Changed the tar walker and writer to reject symlinks and every other
  non-regular archive input rather than following it with `os.Stat`/`os.Open`.
* Hardened root metadata and declared-spec reads so an `index.md` symlink cannot
  bypass validation, including when `connect --no-validate` is used.
* Added parser, resolver, `get`, viewer, theme, and archive regressions proving
  outside content is refused and no archive is published.
* Source anchors: `packages/cli/internal/okf/paths.go`,
  `packages/cli/internal/okf/ast_bundle_parse.go`,
  `packages/cli/internal/okf/metadata.go`,
  `packages/cli/internal/okf/archive.go`,
  `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/viewer.go`, and
  `packages/cli/cmd/openknowledge/viewer_theme.go`.
* Docs updated: `README.md`, `Wiki/features/commands/get.md`,
  `Wiki/features/commands/validate.md`, `Wiki/features/commands/view.md`,
  `Wiki/features/commands/connect.md`, `Wiki/features/exporters/tar.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Publish-scoped portable HTML archive

* Fixed default HTML exports leaking Markdown marked `okf_publish: false`
  through the public `assets/openknowledge-bundle.tar.gz` remote-connect asset.
* Added a publish-scoped archive writer for HTML output while keeping standalone
  `openknowledge to tar` as the complete source-preserving bundle export.
* Added extraction-level regression coverage proving public Markdown remains
  connectable and unpublished Markdown is absent from the archive itself.
* Source anchors: `packages/cli/internal/okf/archive.go`,
  `packages/cli/cmd/openknowledge/viewer.go`, and
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/exporters/html.md`,
  `Wiki/features/exporters/tar.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Pull request quality gate

* Added a dedicated GitHub Actions CI workflow for pull requests, pushes to
  `main`, and manual verification; previously the full repository checks ran
  only in deployment and release workflows.
* Added read-only permissions, superseded-run cancellation, and a 20-minute job
  timeout.
* Made CI verify frozen dependencies, tidy Go modules, aligned package versions,
  the full Go test suite, `go vet`, CLI and website builds, Wiki validation with
  the newly built binary, and a clean generated-file diff.
* Source anchor: `.github/workflows/ci.yml`.
* Docs updated: `Wiki/features/operations.md` and `Wiki/changelog/cli.md`.

### 2026-07-15 - Atomic managed remote refresh

* Added `openknowledge registry refresh <key|path> [--force]` for managed
  manifest, archive, and Git connections.
* Exposed the new command and example in root help as well as registry-specific
  help so it is discoverable without prior documentation.
* Refresh refuses detected local cache changes unless `--force` is present,
  while invalid or missing current generations can still be repaired from the
  recorded source.
* Materialized each refresh into a distinct validated cache generation, then
  atomically replaced the exact registry snapshot before deleting old content.
  Fetch, validation, provenance, and registry failures preserve the previously
  active generation.
* Added exact-entry replacement collision guards plus end-to-end Git coverage
  for generation changes, current-cache health, local-change protection,
  forced replacement, failed-source rollback, interspersed flags, and local
  connection rejection.
* Source anchors: `packages/cli/internal/okf/registry.go`,
  `packages/cli/internal/okf/registry_test.go`,
  `packages/cli/cmd/openknowledge/main.go`, and
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/registry.md`,
  `Wiki/features/commands/connect.md`, `Wiki/features/tooling-model.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Offline registry integrity status

* Added `openknowledge registry status [key|path] [--json]` to check registered
  paths, OKF validation, cache provenance, and immutable local content identity
  without contacting remotes.
* Added deterministic managed-tree SHA-256 recording for new archives and Git
  clones. Status detects archive/cache edits even after extraction; Git sources
  additionally compare the exact commit and dirty working tree.
* Defined `ok`, `warnings`, `unverified`, `modified`, `invalid`, and `missing`
  states with nonzero exit status for the four unhealthy/unknown states.
* Added a versioned JSON report with summary counts, per-entry validation,
  identity, provenance and problems, plus a checked Draft 2020-12 schema at
  `packages/cli/schemas/v1/registry-status.schema.json`.
* Added deterministic content-hash, healthy/missing local, clean/modified Git,
  interspersed flag, help, and schema-version coverage.
* Source anchors: `packages/cli/internal/okf/content_hash.go`,
  `packages/cli/internal/okf/content_hash_test.go`,
  `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/main_test.go`, and
  `packages/cli/schemas/v1/registry-status.schema.json`.
* Docs updated: `README.md`, `Wiki/features/commands/registry.md`,
  `packages/cli/schemas/v1/README.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Transactional managed cache deletion

* Persisted and consumed the complete managed cache root so deleting an archive
  with a nested bundle root also removes its top-level files and provenance
  sidecar.
* Restricted deletion to direct children of the Open Knowledge cache whose
  registered bundle path is contained inside, preventing forged or stale
  registry metadata from deleting arbitrary local directories.
* Added exact expected-entry comparison inside the locked registry removal
  transaction so concurrent source, access, key, or path changes abort safely.
* Changed managed deletion to take the source cache lock and rename content to a
  tombstone before registry removal; registry failure restores content and
  provenance to their original paths.
* Added changed-snapshot, nested archive root, unrelated sibling preservation,
  tombstone cleanup, and out-of-cache refusal tests.
* Source anchors: `packages/cli/internal/okf/registry.go`,
  `packages/cli/internal/okf/registry_test.go`,
  `packages/cli/cmd/openknowledge/main.go`, and
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `Wiki/features/commands/disconnect.md`,
  `Wiki/features/commands/registry.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Atomic locked remote cache publication

* Added source-specific in-process and cross-process locks around cache reads,
  migration, download, clone, provenance persistence, and publication while
  allowing unrelated sources to proceed independently.
* Changed Git remotes to clone into staging, require a valid OKF bundle, capture
  the commit before publication, and leave no published target on failure.
* Changed archive and Git cache replacement to retain the old target until the
  new staging tree is complete, with deterministic rollback if publication
  fails.
* Restricted the cache root and lock files to owner-only permissions.
* Added concurrent same-source materialization, invalid Git, failed remote
  replacement, and failed publication rollback tests.
* Source anchors: `packages/cli/cmd/openknowledge/main.go` and
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `Wiki/features/commands/connect.md`,
  `Wiki/features/commands/registry.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Durable remote source identity

* Changed remote cache keys from alias-plus-source to normalized source
  identity, so reconnecting the same source under another key reuses one cache
  rather than cloning or downloading duplicate content.
* Expanded registry provenance with requested and resolved URLs, final manifest
  and archive URLs, concrete spec, archive SHA-256 or exact Git commit, fetch
  timestamp, and the complete managed cache root.
* Added a versioned owner-only provenance sidecar beside each new cache so a
  cache hit restores exact source identity instead of reclassifying website
  manifests as Git or discarding archive references.
* Kept legacy cache content in place; missing sidecars are migrated with the
  source types that can be established locally and ambiguous caches use
  `unknown` rather than fabricated provenance.
* Added registry round-trip, Git identity, redirect provenance, sidecar schema
  and permissions, alias-independent cache, and cache-hit retention tests.
* Source anchors: `packages/cli/internal/okf/registry.go`,
  `packages/cli/internal/okf/registry_test.go`,
  `packages/cli/cmd/openknowledge/main.go`, and
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `Wiki/features/commands/connect.md`,
  `Wiki/features/commands/registry.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Bounded atomic remote archives

* Limited remote manifests to 1 MiB and compressed archive downloads to 512
  MiB, rejecting both declared and streaming overflows without partial files.
* Limited tar extraction to 100,000 entries, 256 MiB per regular file, and 2
  GiB total expanded content in addition to existing traversal and entry-type
  checks.
* Changed extraction to a sibling staging transaction so failures leave no
  partial target and existing targets are refused instead of overlaid.
* Added regressions for content-length and streaming download overflow, every
  extraction limit, staging cleanup, and existing-target preservation.
* Source anchors: `packages/cli/internal/okf/archive.go`,
  `packages/cli/internal/okf/archive_test.go`,
  `packages/cli/cmd/openknowledge/main.go`, and
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `Wiki/features/commands/connect.md`,
  `Wiki/features/exporters/tar.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Enforced remote manifest integrity

* Replaced the duplicate remote manifest shape with the shared archive
  contract and now require type `openknowledge.bundle`, manifest version `1`, a
  concrete supported OKF spec, `archiveFormat: "tar.gz"`, an archive path, and
  a valid SHA-256 digest.
* Bound extracted-bundle validation to the manifest spec and reject archives
  whose root `okf_version` conflicts with that immutable declaration.
* Resolve relative archive URLs against the final manifest URL after redirects
  and preserve explicit non-404 manifest download failures instead of
  misreporting them as a missing manifest.
* Added deterministic coverage for every manifest field, spec mismatch,
  redirect-relative archives, checksum use, and HTTP server errors.
* Source anchors: `packages/cli/internal/okf/archive.go`,
  `packages/cli/internal/okf/archive_test.go`,
  `packages/cli/cmd/openknowledge/main.go`, and
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `Wiki/features/commands/connect.md`,
  `Wiki/features/exporters/tar.md`, `Wiki/features/exporters/html.md`, and
  `Wiki/changelog/cli.md`.

### 2026-07-15 - Atomic concurrent registry updates

* Serialized registry mutations across goroutines and processes, loaded one
  snapshot per transaction, and atomically replaced the JSON file so parallel
  connects and disconnects cannot lose entries or expose partial content.
* Restricted registry and lock files to owner-only permissions and added a
  cross-process regression test covering 16 simultaneous connections, valid
  JSON, complete entry retention, and temporary-file cleanup.
* Moved the `disconnect --delete-files` managed-entry guard inside the removal
  transaction so a concurrent registry change cannot redirect deletion toward
  non-managed files.
* Added complete third-party license notices to source, release archives, and
  the npm wrapper for the cross-platform locking and replacement libraries.
* Source anchors: `packages/cli/internal/okf/registry.go`,
  `packages/cli/internal/okf/registry_test.go`,
  `packages/cli/cmd/openknowledge/main.go`, `.goreleaser.yaml`, and
  `THIRD_PARTY_NOTICES.md`.
* Docs updated: `Wiki/features/commands/registry.md`,
  `Wiki/features/commands/disconnect.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Unified release versions and npm publishing

* Made the root `package.json` the release-version source of truth and aligned
  the CLI fallback, npm wrapper, and web workspace at the prepared `0.6.0`
  release version. `pnpm check:versions` and the default test task now reject
  drift.
* Moved Git tag creation behind a release quality gate covering tidy modules,
  version alignment, Go tests and vet, CLI/web builds, binary version
  injection, Wiki validation, and npm tarball inspection.
* Re-enabled npm publishing after GoReleaser with provenance, `latest` for
  stable releases, and `next` for prereleases. The workflow requires
  `NPM_TOKEN` before pushing a new release tag.
* Removed the mutating `go mod tidy` GoReleaser hook; module cleanliness is now
  a checked preflight invariant.
* Source anchors: `package.json`, `scripts/check-versions.mjs`, package
  manifests, `packages/cli/cmd/openknowledge/main.go`,
  `.github/workflows/release.yml`, and `.goreleaser.yaml`.
* Docs updated: `README.md`, `packages/npm/README.md`,
  `Wiki/features/operations.md`, `Wiki/features/installation.md`,
  `Wiki/features/commands/version.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Versioned machine-readable contracts

* Added `schemaVersion: "1"` to AST, normalized bundle JSON, source/search
  graphs, search context and match results, list inventories, and validation
  reports. `specVersion` remains the independent OKF format selector.
* Changed `list --json` from an unversioned top-level array to a versioned
  object containing `root` and `entries`.
* Added Draft 2020-12 schemas under `packages/cli/schemas/v1/`, a documented
  compatibility policy, golden snapshots for all seven contract families, and
  behavioral assertions at command boundaries.
* Source anchors: `packages/cli/internal/okf/machine_contract.go`, machine
  output type files, `packages/cli/internal/okf/machine_contract_test.go`,
  `packages/cli/internal/okf/testdata/contracts/`, and
  `packages/cli/schemas/v1/`.
* Docs updated: `README.md`, `Wiki/features/commands/ast.md`,
  `Wiki/features/commands/list.md`, `Wiki/features/commands/search.md`,
  `Wiki/features/commands/validate.md`, `Wiki/features/exporters/json.md`,
  `Wiki/features/exporters/graph.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Typed YAML frontmatter parsing

* Replaced the separate lightweight scalar and structured-subset frontmatter
  paths with one shared standard YAML parser for OKF validation, AST output,
  normalized JSON, viewer metadata, and experimental agent jobs.
* Invalid YAML at any nesting depth and non-mapping roots are now parse errors.
  Valid nested mappings and sequences, flow collections, block scalars, and
  typed scalar values are preserved instead of flattened or rejected as an
  unsupported subset.
* AST output exposes typed values in `frontmatter.data` alongside the compatible
  scalar `frontmatter.values` projection. Normalized JSON exposes typed values
  directly in each file's `frontmatter` object.
* Source anchors: `packages/cli/internal/okf/frontmatter.go`,
  `packages/cli/internal/okf/frontmatter_yaml.go`,
  `packages/cli/internal/okf/ast_frontmatter_types.go`,
  `packages/cli/internal/okf/bundle_types.go`,
  `packages/cli/internal/agents/spec.go`, and
  `packages/cli/cmd/openknowledge/viewer_frontmatter.go`.
* Docs updated: `README.md`, `Wiki/features/spec-compliance.md`,
  `Wiki/features/commands/validate.md`, `Wiki/features/commands/ast.md`,
  `Wiki/features/exporters/json.md`, `Wiki/features/commands/view.md`,
  `Wiki/features/commands/agents.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Nested agent command help

* Fixed `openknowledge agents new|list|validate|run|daemon --help` so each form
  prints dedicated subcommand usage and flags instead of the general `agents`
  overview.
* Added behavioral coverage for all five nested help routes.
* Source anchors: `packages/cli/cmd/openknowledge/agents_command.go` and
  `packages/cli/cmd/openknowledge/agents_command_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/agents.md`,
  `Wiki/features/commands/help.md`, and `Wiki/changelog/cli.md`.

### 2026-07-15 - Positional connection flags

* Fixed `connect` and `disconnect`, including their `registry` aliases, so
  documented positional-first forms such as
  `openknowledge connect ./wiki --as personal` and
  `openknowledge disconnect personal --delete-files` parse their flags.
  Flag-first forms remain supported.
* Added behavioral coverage for both flag orders across the top-level and
  registry command surfaces.
* Source anchors: `packages/cli/cmd/openknowledge/main.go` and
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `Wiki/features/commands/connect.md`,
  `Wiki/features/commands/disconnect.md`,
  `Wiki/features/commands/registry.md`, and `Wiki/changelog/cli.md`.

### 2026-07-10 - Published Wiki uses landing-page colors

* Fixed the deployed Wiki theme so its dark preset reliably overrides the
  built-in Night palette instead of retaining green links and green-tinted
  surfaces from the generic viewer theme.
* The published docs now use the landing page's blue accent, dark surfaces,
  focus states, selection colors, and matching Night theme swatch.
* Source anchors: `Wiki/assets/openknowledge-site.css`.
* Docs updated: `Wiki/changelog/cli.md`.

### 2026-07-10 - Homepage product positioning

* Restored the landing page's "LLM wiki tooling for agents and humans" hero and
  its concise Git-native Markdown description.
* Reframed the benefits and capabilities around the problems the product
  solves: locally navigable context, token-budgeted retrieval, trustworthy link
  and structure validation, and portable HTML, JSON, graph, and TAR output.
* Kept prompt-driven behavior explicit: `setup`, `from`, and `rules` generate
  instructions for local agents rather than calling a model themselves. No CLI
  behavior changed.
* Source anchors: `packages/web/index.html`.
* Docs updated: `packages/web/index.html`, `Wiki/changelog/cli.md`.

### 2026-07-09 - Viewer reading and accessibility settings

* Added system-level viewer controls for font family, text size, line spacing,
  motion reduction, readable line length, high contrast, and always-underlined
  links alongside the existing theme and frontmatter settings.
* Preferences are browser-local, persist through `localStorage` with a cookie
  fallback, and affect viewer presentation only; authored Markdown and editor
  deeplinks remain unchanged. The default viewer behavior is shared by local
  pages and static HTML exports, while `--plain` exports remain unchanged.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_app.css`, and
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/view.md`,
  `Wiki/features/exporters/html.md`, and `Wiki/changelog/cli.md`.

### 2026-07-09 - Homepage visual system polish

* Refined the project homepage without changing its product story: the existing
  electric-blue artwork now carries a cleaner transparent header, restrained
  release metadata, stronger sans-serif type hierarchy, and higher-contrast
  primary actions.
* Simplified the content body into a README-like flow with typographic section
  breaks, quiet line icons, a responsive command map, and code-first workflow
  examples. Hero actions use purpose-specific agent and document icons without
  redundant arrow glyphs.
* Promoted setup-prompt copying to the hero's primary action and removed the
  standalone setup section. The full agent prompt and shell fallback remain in
  a native disclosure that starts collapsed beneath the hero actions, alongside
  concise open-source and Apache-2.0 metadata. Copy feedback remains immediate
  even while the prompt is hidden, and lower section headings no longer repeat
  explanatory subcopy.
* The footer retains the concise project identity and links.
* Aligned the deployed Wiki theme's palette and typography with the homepage.
* Source anchors: `packages/web/index.html`, `packages/web/styles.css`,
  `packages/web/main.js`, `packages/web/logo-mark.png`,
  `packages/web/scripts/build.mjs`, and `Wiki/assets/openknowledge-site.css`.
* Docs updated: `Wiki/changelog/cli.md`.

Next-release work is classified under [v0.6.0 Candidate](#v060-candidate) until
a release tag is cut.

## v0.6.0 Candidate

### 2026-07-09 - Viewer tag facets and breadcrumb navigation

* Added an exact viewer tag index from top-level OKF `tags` arrays. Tag chips
  now open the shared search surface with same-tag notes, excluding the current
  note; static exports embed the same facet data for local/static parity.
* Replaced monolithic note-path links with segmented breadcrumbs. Directory
  segments link only when their index document exists, while the current-file
  segment returns to a clean single-panel URL.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_frontmatter.go`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_search.js`,
  `packages/cli/cmd/openknowledge/viewer_app.css`, and
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/view.md`,
  `Wiki/features/exporters/html.md`, and `Wiki/changelog/cli.md`.

### 2026-07-09 - Viewer visual system polish

* Refined the shared local and default static viewer visual system with quieter
  neutral surfaces, clearer panel elevation and document hierarchy, consistent
  control geometry, stronger focus states, and more coherent built-in themes.
  Commands, navigation, and content behavior remain unchanged.
* Made Night the first-run theme for local and default static viewer pages,
  renamed the previous light preset to Light in the settings UI, and restored a
  valid saved selection before the built-in CSS paints. Existing browser-local
  theme choices remain unchanged.
* Removed the redundant inner editor-button border and the document header rule
  so the note chrome and viewer shell read as a single, quieter surface.
* Improved top-bar search dismissal for outside pointer/focus interaction and
  streamlined its dropdown into a single shadowed surface with clearer result
  title, metadata, and snippet hierarchy.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_theme_bootstrap.js`,
  `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_theme.css`,
  `packages/cli/cmd/openknowledge/viewer_app.js`, and
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/view.md`,
  `Wiki/features/exporters/html.md`, and `Wiki/changelog/cli.md`.

### 2026-07-09 - Typed viewer frontmatter inspector

* Added a typed, per-note collapsible frontmatter inspector to Markdown note
  panels. It starts collapsed so the rendered document body remains the first
  focus, while OKF metadata stays one interaction away.
* Added a global `Show frontmatter` viewer setting. It is enabled by default,
  applies to open and newly opened panels, controls inspector visibility rather
  than expansion, and persists browser-locally through the viewer's local-storage
  and cookie fallback.
* Structured frontmatter values render recursively without visible datatype
  badges: booleans retain a state treatment, simple lists render as chips, and
  nested lists and maps retain their structure. Unsupported structured YAML
  falls back to compatible scalar values without hiding the Markdown body.
* Kept local and default static viewer behavior aligned; static search also
  includes the rendered frontmatter text. Plain HTML exports continue to omit
  viewer chrome and frontmatter presentation.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_frontmatter.go`,
  `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_search.js`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/view.md`,
  `Wiki/features/exporters/html.md`, and `Wiki/changelog/cli.md`.

### 2026-07-09 - Search context packets by default

* Changed `openknowledge search <name-or-path> <query>` to emit a bounded,
  source-preserving Markdown context packet by default. Context sources retain
  their authored Markdown, file and heading provenance, line range, score, and
  direct or related relationship to the query.
* Kept section-level BM25 ranking as the canonical retrieval layer. Search now
  includes one-hop existing local outgoing links and backlinks by default when
  related sections fit the remaining token budget and source limit;
  `--no-expand` opts out.
* Added `--budget <tokens>` with a `2400` default and `--matches` for the prior
  ranked match-list inspection view. The context-only budget flag cannot be
  combined with `--matches`. `--limit` continues to default to `12` and caps
  selected context sources or displayed matches.
* Changed `--format` to `markdown|json` with `markdown` as the default. Context
  JSON reports `root`, `query`, `budget`, `estimatedTokens`, `limit`,
  `sources`, and validation `issues`; each source carries its original
  Markdown and provenance. Removed the pre-v1 `--expand graph` flag and the
  `text` format name.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/cli/internal/okf/search_knowledge.go`,
  `packages/cli/internal/okf/search_types.go`,
  `packages/cli/internal/okf/context.go`,
  `packages/cli/internal/okf/context_selection.go`,
  `packages/cli/internal/okf/context_types.go`,
  `packages/cli/internal/okf/new.go`,
  `packages/cli/internal/okf/search_test.go`,
  `packages/cli/internal/okf/context_test.go`.
* Docs updated: `README.md`, `packages/web/index.html`, `Wiki/index.md`,
  `Wiki/features/commands/index.md`, `Wiki/features/commands/search.md`,
  `Wiki/features/commands/get.md`, `Wiki/features/commands/help.md`,
  `Wiki/features/tooling-model.md`, and `Wiki/changelog/cli.md`.

### 2026-07-08 - LLM wiki positioning copy

* Reworded homepage and README positioning from "Local LLM wiki" to "LLM wiki
  tooling" so the public copy does not overstate that the workflow is strictly
  local.
* Source anchors: `packages/web/index.html`, `packages/web/og.html`,
  `README.md`, `packages/npm/README.md`.
* Docs updated: `README.md`, `packages/npm/README.md`,
  `packages/web/index.html`, `packages/web/og.html`,
  `Wiki/changelog/cli.md`.

### 2026-07-08 - Homepage OKF note

* Added inline homepage hero metadata next to the release badge stating that
  Open Knowledge implements Google's Open Knowledge Format.
* Source anchors: `packages/web/index.html`, `packages/web/styles.css`.
* Docs updated: `packages/web/index.html`, `packages/web/styles.css`,
  `Wiki/changelog/cli.md`.

## v0.5.0 - 2026-07-08

Released v0.5.0 changes after the `v0.4.0` release tag.

### 2026-07-08 - First-party source examples

* Replaced generic placeholder source URLs in source-to-wiki, connect, and HTML
  export examples with Open Knowledge-owned URLs so copied examples do not
  encourage agents to fetch arbitrary external sources.
* Source anchors: `README.md`, `packages/web/index.html`,
  `packages/cli/cmd/openknowledge/main.go`,
  `Wiki/features/commands/from.md`, `Wiki/features/tooling-model.md`.
* Docs updated: `README.md`, `packages/web/index.html`,
  `Wiki/features/commands/from.md`, `Wiki/features/commands/connect.md`,
  `Wiki/features/exporters/html.md`, `Wiki/features/tooling-model.md`,
  `Wiki/changelog/cli.md`.

### 2026-07-08 - Safer agent prompt handoff

* Replaced command-substitution examples for setup and source-to-wiki prompts
  with a safer flow: run `openknowledge setup` or `openknowledge from`, then
  copy the printed prompt into the agent. The generated prompts and `from`
  help text now explicitly avoid shell substitution and piping for interactive
  agent CLIs because those patterns can be flagged by security tools.
* Source anchors: `packages/cli/internal/okf/setup.go`,
  `packages/cli/internal/okf/from.go`,
  `packages/cli/cmd/openknowledge/main.go`, `README.md`,
  `packages/web/index.html`.
* Docs updated: `README.md`, `packages/web/index.html`,
  `Wiki/features/commands/setup.md`, `Wiki/features/commands/from.md`,
  `Wiki/changelog/cli.md`.

### 2026-07-08 - Homepage source-to-wiki workflow

* Added a homepage common workflow for creating a knowledge base from a Git
  repository or docs page with `openknowledge from`.
* Source anchors: `packages/web/index.html`.
* Docs updated: `packages/web/index.html`, `Wiki/changelog/cli.md`.

### 2026-07-08 - Setup prompt view handoff

* Updated the README and homepage setup prompt copy so the post-setup handoff
  tells agents to inspect the wiki with `openknowledge list`,
  `openknowledge search`, and `openknowledge get`, then open it for the user
  with `openknowledge view`.
* Source anchors: `README.md`, `packages/web/index.html`.
* Docs updated: `README.md`, `packages/web/index.html`,
  `Wiki/features/commands/setup.md`, `Wiki/changelog/cli.md`.

### 2026-07-07 - Homepage README alignment

* Refreshed the homepage around the updated README story: shared the new banner
  image, moved the setup prompt into a clearer start section, added At A Glance
  capability cards, and added a command-map table with the experimental agents
  surface labeled.
* Source anchors: `packages/web/index.html`, `packages/web/styles.css`,
  `packages/web/scripts/build.mjs`, `packages/web/openknowledge-readme-banner.png`.
* Docs updated: `packages/web/index.html`, `packages/web/styles.css`,
  `packages/web/scripts/build.mjs`, `Wiki/changelog/cli.md`.

### 2026-07-07 - Top navigation links

* Added README and changelog links to the README top link strip, while the
  website header keeps the focused Wiki, Changelog, and GitHub navigation.
* Source anchors: `README.md`, `packages/web/index.html`.
* Docs updated: `README.md`, `packages/web/index.html`,
  `Wiki/changelog/cli.md`.

### 2026-07-07 - Experimental label for local agent jobs

* Marked `openknowledge agents` as experimental in root help, command-specific
  help, README command references, and wiki command docs while the job schema
  and scheduler behavior are still settling.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/agents_command.go`,
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/agents.md`,
  `Wiki/features/commands/help.md`, `Wiki/features/commands/index.md`,
  `Wiki/changelog/cli.md`.

### 2026-07-07 - Local agent job runner

* Added `openknowledge agents` with `new`, `list`, `validate`, `run`, and
  `daemon` subcommands for Markdown-authored local agent jobs.
* Added built-in agent job templates for docs audits, wiki health checks,
  release readiness checks, and custom jobs. `openknowledge agents new` lists
  templates, prints template Markdown, writes a selected template with `--out`,
  and prints the supported nested frontmatter syntax with `--reference`.
* Extended the existing frontmatter splitter with a structured nested
  frontmatter view while preserving the scalar metadata view used by OKF
  documents and validation.
* Agent runs now resolve deterministic run plans from job id, scheduled time,
  job file hash, and Git base SHA; create Git worktrees; support host and
  Docker executors; write run logs/records under `.openknowledge/agents/runs/`;
  and run configured verification commands.
* Source anchors: `packages/cli/cmd/openknowledge/agents_command.go`,
  `packages/cli/internal/agents/`,
  `packages/cli/internal/okf/frontmatter.go`,
  `packages/cli/internal/okf/frontmatter_structured.go`,
  `packages/cli/cmd/openknowledge/agents_command_test.go`,
  `packages/cli/internal/okf/frontmatter_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/agents.md`,
  `Wiki/features/commands/index.md`, `Wiki/features/commands/help.md`,
  `Wiki/index.md`, `Wiki/changelog/cli.md`.

### 2026-07-07 - Custom rule catalogs and advisory rule reviews

* Added wiki-local custom maintenance rules under `rules/` as OKF Markdown
  files. `openknowledge rules --list --path <wiki>` now includes valid custom
  rule IDs, and `openknowledge rules <id> --path <wiki>` can render them
  alongside built-in rules.
* Added deterministic `rule-catalog` validation for custom rule structure,
  including canonical IDs, summaries, instruction bullets, built-in collisions,
  duplicate custom IDs, configured rule paths, and configured default rule IDs.
* Added `[rules]` support in `openknowledge.toml`. `rules.paths` selects
  custom rule Markdown directories, and `rules.enabled` defines the default
  selected rules for `openknowledge rules`, `openknowledge rules apply`, and
  `openknowledge review rules`.
* Added `openknowledge review rules`, a prompt-producing advisory AI review
  workflow for selected built-in or custom rules. It does not call a model,
  mutate files, or affect `validate` status.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/internal/okf/rule_catalog.go`,
  `packages/cli/internal/okf/rules.go`,
  `packages/cli/internal/okf/ast_validate.go`,
  `packages/cli/internal/okf/validation_checks.go`,
  `packages/cli/internal/okf/validation_policy.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/cli/internal/okf/rules_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/rules.md`,
  `Wiki/features/commands/review.md`, `Wiki/features/commands/validate.md`,
  `Wiki/features/commands/setup.md`, `Wiki/features/commands/help.md`,
  `Wiki/features/commands/index.md`, `Wiki/features/tooling-model.md`,
  `Wiki/index.md`, `Wiki/log.md`, `Wiki/changelog/cli.md`.

### 2026-07-07 - Source-to-wiki prompt command

* Added `openknowledge from <source> --out <folder>` as a prompt-producing
  source-to-wiki command. It prints instructions for a local agent to inspect a
  GitHub repository, local path, or website, create or refresh an OKF bundle,
  preserve source provenance, validate the result, and hand back
  `list`/`search`/`get`/`view` commands.
* Added `--type understanding|custom`, with `understanding` as the default
  DeepWiki-style recipe and `custom` as the interview-driven recipe.
* Added `--about <goal>` for non-interactive custom generation goals and
  `--depth <count>` as a crawl or traversal hint.
* Added `openknowledge new --no-agents` and `--no-setup` for source-generated
  or otherwise task-driven bundles that do not need starter agent rules or an
  interactive setup handoff document. The `from` prompt now tells agents to use
  those flags when initializing a fresh output bundle.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/internal/okf/from.go`,
  `packages/cli/internal/okf/new.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/cli/internal/okf/from_test.go`,
  `packages/cli/internal/okf/validate_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/from.md`,
  `Wiki/features/commands/new.md`, `Wiki/features/commands/help.md`,
  `Wiki/features/commands/index.md`, `Wiki/features/tooling-model.md`,
  `Wiki/index.md`, `Wiki/log.md`, `Wiki/changelog/cli.md`.

### 2026-07-06 - Clean get/list/view navigation API

* Deprecated and removed the previous deterministic read command name outright;
  it is no longer registered in the dispatcher, shown in root help, or retained
  as an alias. `openknowledge get <name-or-path> [entry-or-file]` is the clean
  replacement for exact Markdown retrieval.
* `openknowledge get` can print an exact local Markdown file, a bundle default
  entrypoint, a named entrypoint, a bundle-relative Markdown file, or selected
  metadata with `--info`.
* Deprecated and removed the previous local viewer command name outright; it is
  no longer registered in the dispatcher, shown in root help, or retained as an
  alias. `openknowledge view [path]` is the clean replacement for launching the
  local viewer.
* Added `openknowledge list --depth <n>` for bounded tree inspection and
  expanded `openknowledge list` to include non-Markdown files as `asset`
  entries, so it can describe the whole knowledge base structure.
* Updated setup prompts, generated `SETUP.MD`, README setup copy, landing page
  setup copy, command help, wiki command pages, and the tooling model around
  the `get`, `list`, `search`, and `view` navigation loop.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/internal/okf/list.go`,
  `packages/cli/internal/okf/setup.go`,
  `packages/cli/internal/okf/new.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/cli/internal/okf/setup_test.go`,
  `README.md`, `packages/web/index.html`.
* Docs updated: `README.md`, `Wiki/features/commands/get.md`,
  `Wiki/features/commands/view.md`, `Wiki/features/commands/list.md`,
  `Wiki/features/commands/search.md`, `Wiki/features/commands/setup.md`,
  `Wiki/features/commands/help.md`, `Wiki/features/commands/index.md`,
  `Wiki/features/tooling-model.md`, `Wiki/index.md`,
  `Wiki/changelog/cli.md`.

### 2026-07-06 - Search command and search graph exports

* Added `openknowledge search <name-or-path> <query>` as the first-class query
  retrieval command for Open Knowledge bundles. It builds source-grounded
  Markdown heading chunks, scores them with BM25-style lexical ranking across
  metadata, heading paths, paths, and body text, and prints text or JSON
  results with snippets, source line ranges, scores, and matched fields.
* Added `openknowledge search --expand graph` to include lower-ranked outgoing
  local-link and backlink neighbor chunks in search results.
* Removed the previous query mode from deterministic entrypoint and bundle-file
  loading. Query retrieval now belongs to
  `openknowledge search <bundle> <query>`.
* Added `openknowledge to graph --type source|search`. `source` is the default
  file/link graph. `search` exports a derivative chunk graph with source file
  nodes, heading chunk nodes, containment edges, reading-order edges, and
  chunk-level local-link edges.
* Updated the setup prompt, generated `SETUP.MD`, README setup prompt, and
  landing page prompt to leave users with the use/navigation loop available at
  that point in the candidate series.
* Reframed the docs layer model around connection and bundle lifecycle,
  validation and inspection, use and navigation, and OKF views. Search now
  belongs with use/navigation, while AST, JSON, source graph, and search graph
  are described as different views of the same OKF bundle.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/internal/okf/setup.go`,
  `packages/cli/internal/okf/new.go`,
  `packages/web/index.html`,
  `packages/cli/internal/okf/search_knowledge.go`,
  `packages/cli/internal/okf/search_types.go`,
  `packages/cli/internal/okf/context_sections.go`,
  `packages/cli/internal/okf/graph.go`,
  `packages/cli/internal/okf/graph_types.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/cli/internal/okf/setup_test.go`,
  `packages/cli/internal/okf/search_test.go`,
  `packages/cli/internal/okf/export_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/search.md`,
  `Wiki/features/commands/setup.md`, `Wiki/features/commands/to.md`,
  `Wiki/features/exporters/graph.md`,
  `Wiki/features/exporters/index.md`, `Wiki/features/commands/index.md`,
  `Wiki/features/commands/help.md`, `Wiki/features/tooling-model.md`,
  `Wiki/features/index.md`, `Wiki/index.md`, `Wiki/changelog/cli.md`.

### 2026-07-05 - Agent maintenance rules command

* Added `openknowledge rules`, a print-only command that renders
  ready-to-paste Markdown instructions for agents maintaining an Open Knowledge
  wiki. It supports a comma-separated rules argument, `--path`, `--target`, and
  `--list`.
* The print path now inspects the selected wiki path and emits non-blocking
  warnings when the folder is missing, empty of Markdown, or not valid OKF.
  Each warning includes an agent action, such as creating the wiki, choosing a
  folder, adding Markdown files, or running validation. In an interactive
  terminal, warnings are highlighted with a `⚠ Warning:` marker, spaced apart
  from nearby output, and printed after the rendered rules; with pipes or
  redirection they go to stderr.
* Added `openknowledge rules apply` as the explicit mutation path for inserting
  or replacing an idempotent managed rules block in `AGENTS.md`, `CLAUDE.md`,
  Cursor rules, or a user-selected instruction file. In interactive mode it
  shows the generated block, then warns and asks for confirmation before
  changing an existing file unless `--yes` is passed.
* Added `openknowledge setup --rules <rules>` support so setup prompts can
  start from selected comma-separated maintenance rules while still letting the
  setup agent inspect context before creating files.
* Standardized the canonical maintenance rules as `project`, `docs`,
  `decisions`, `changelog`, `research`, `bugs`, `schemas`, `summary`, and
  `agents`.
* Updated the setup prompt, generated `SETUP.MD`, landing page, and README
  setup copy to tell agents to use `openknowledge rules --list` when they need
  the available rule list.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/internal/okf/rules.go`,
  `packages/cli/internal/okf/setup.go`,
  `packages/cli/internal/okf/new.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/cli/internal/okf/rules_test.go`,
  `packages/cli/internal/okf/setup_test.go`,
  `README.md`, `packages/web/index.html`.
* Docs updated: `README.md`, `packages/web/index.html`,
  `Wiki/features/commands/rules.md`, `Wiki/features/commands/setup.md`,
  `Wiki/features/commands/help.md`, `Wiki/features/commands/index.md`,
  `Wiki/changelog/cli.md`.

### 2026-07-04 - Static viewer head injection and wiki analytics

* Default `openknowledge to html` viewer exports now support trusted custom
  head injection with `--head-file`, `--head-html`, repeatable `--script-src`,
  and matching `OPENKNOWLEDGE_HEAD_FILE`, `OPENKNOWLEDGE_HEAD_HTML`, and
  `OPENKNOWLEDGE_SCRIPT_SRC` environment variables.
* The website wiki export now extracts the Google Analytics `gtag.js` snippet
  from `packages/web/index.html` and injects the same head HTML into generated
  wiki pages so the deployed landing page and wiki use the same measurement ID.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `packages/web/index.html`, `packages/web/scripts/wiki-export.mjs`.
* Docs updated: `README.md`, `Wiki/features/exporters/html.md`,
  `Wiki/features/commands/to.md`, `Wiki/features/operations.md`,
  `Wiki/changelog/cli.md`.

### 2026-07-04 - Static wiki discovery files

* Default `openknowledge to html` viewer exports now write `llms.txt` with a
  Markdown title, summary, details, and published page links for LLM-oriented
  consumers.
* Viewer exports also write `sitemap.xml` when `[html.site].base_url` is
  configured in `openknowledge.toml`; sitemap entries use absolute URLs for
  published pages only.
* The repository wiki now configures `https://openknowledge.sh/wiki/` as its
  published base URL so web builds can emit sitemap URLs for the deployed wiki.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_discovery.go`,
  `packages/cli/cmd/openknowledge/viewer_theme.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `Wiki/openknowledge.toml`.
* Docs updated: `README.md`, `Wiki/features/exporters/html.md`,
  `Wiki/features/commands/to.md`, `Wiki/changelog/cli.md`.

### 2026-07-03 - Validation JSON reports and rule severities

* `openknowledge validate` now supports `--format json`, `--json`, and
  `--format json --out <file>` for machine-readable validation reports with
  summary counts, policy metadata, check statuses, combined issues, errors, and
  warnings.
* Validation rule severities can be configured with `[validation.rules]` in
  `openknowledge.toml` and overridden per run with repeatable
  `--rule rule=off|warn|error` flags.
* Default validation behavior remains unchanged when no severity overrides are
  configured.
* Source anchors: `packages/cli/internal/okf/validation_policy.go`,
  `packages/cli/internal/okf/validation_types.go`,
  `packages/cli/internal/okf/ast_validate.go`,
  `packages/cli/cmd/openknowledge/main.go`.
* Docs updated: `README.md`, `Wiki/features/commands/validate.md`,
  `Wiki/features/commands/index.md`, `Wiki/changelog/cli.md`.

### 2026-07-03 - LLM wiki landing and README positioning

* Reworded the landing page title, heading, metadata, prompt copy, and product
  summary to lead with local LLM wiki positioning while keeping Open Knowledge
  CLI branding.
* Updated the root and npm READMEs with lightweight LLM wiki, LLM
  Wikipedia-style project memory, Karpathy-style local wiki, and portable OKF
  language.
* Source anchors: `packages/web/index.html`, `packages/web/og.html`,
  `README.md`, `packages/npm/README.md`.
* Docs updated: `README.md`, `packages/npm/README.md`,
  `packages/web/index.html`, `packages/web/og.html`,
  `Wiki/changelog/cli.md`.

### 2026-06-29 - Viewer and web build head injection

* Added trusted custom `<head>` injection to `openknowledge open` through
  `--head-file`, `--head-html`, repeatable `--script-src`, and matching
  `OPENKNOWLEDGE_HEAD_FILE`, `OPENKNOWLEDGE_HEAD_HTML`, and
  `OPENKNOWLEDGE_SCRIPT_SRC` environment variables.
* `pnpm build:web` now uses the same environment contract for the generated
  landing page while still exporting the repository wiki into `dist/wiki`.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/web/scripts/build.mjs`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/operations.md`, `Wiki/changelog/cli.md`.

### 2026-06-28 - Use query answer-ready briefing

* `openknowledge use <name-or-path> --query <text>` now prints a deterministic
  source-grounded briefing before original excerpts in Markdown output.
* Structured JSON output now includes an additive `briefing` object with a
  summary, cited key points, linked-neighbor context, gaps, and validation issue
  count.
* Markdown output now labels key point citations as `Source:` and each found
  entry/excerpt as `Origin: path:line-line`, making it obvious which file and
  line range each selected entry came from.
* This keeps query mode file-native and non-generative while making the output
  easier for agents to answer from directly.
* Source anchors: `packages/cli/internal/okf/context.go`,
  `packages/cli/internal/okf/context_briefing.go`,
  `packages/cli/internal/okf/context_types.go`,
  `packages/cli/cmd/openknowledge/main.go`.
* Docs updated: `README.md`, `Wiki/features/commands/use.md`,
  `Wiki/changelog/cli.md`.

## v0.4.0 - 2026-06-23

Released as Git tag `v0.4.0` from commit `335188f`. These entries were still
stored under `v0.4.0 Candidate` at tag time and are now classified as V4
release contents.

### 2026-06-23 - Viewer system badge spacing

* Adjusted local and static viewer file tree layout so reserved file `system`
  badges sit directly beside file names instead of being pushed to the far
  right of the row.
* This keeps short reserved entries such as `index.md` visually grouped with
  their badge while preserving ellipsis behavior for long file names.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-23 - Viewer panel close shortcut

* Added `Command+Option+W` / `Ctrl+Alt+W` as a local viewer shortcut for
  closing the focused note panel.
* The close button now exposes the formatted shortcut in its hover/focus
  tooltip instead of adding a separate inline badge, and closing a panel moves
  focus to the previous panel when one exists.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-22 - Validate and list key-or-path docs

* Updated `openknowledge validate` and `openknowledge list` help/docs to use
  `[key-or-path]` because both commands resolve registry keys through the
  shared knowledge-root resolver.
* Removed stale candidate text that described a no-argument connected-bundle
  overview for `openknowledge list`; the shipped no-argument behavior still
  lists the current directory.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/validate.md`,
  `Wiki/features/commands/list.md`, `Wiki/features/commands/index.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-22 - Website changelog link and product summary

* Added a `Changelog` link to the landing page navigation beside `Wiki`.
* Reworded the final landing page product summary around the primary use case:
  a local Markdown wiki that lives with a project, gives agents durable context,
  and stays useful through fast search, related-context discovery, and
  maintenance loops.
* Source anchors: `packages/web/index.html`, `packages/web/styles.css`,
  `Wiki/features/tooling-model.md`, `Wiki/index.md`, `README.md`.
* Docs updated: `Wiki/changelog/cli.md`, `Wiki/log.md`.

### 2026-06-22 - Graph exporter target

* Added `openknowledge to graph` to export AST-backed node and edge JSON for
  bundle files and existing local Markdown links.
* Reused the same graph construction path for the local/static viewer knowledge
  graph so viewer graph data and CLI graph export stay aligned.
* Source anchors: `packages/cli/internal/okf/graph.go`,
  `packages/cli/internal/okf/graph_types.go`,
  `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/viewer.go`.
* Docs updated: `README.md`, `Wiki/features/commands/to.md`,
  `Wiki/features/exporters/graph.md`, `Wiki/features/exporters/index.md`,
  `Wiki/features/commands/help.md`, `Wiki/features/tooling-model.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-22 - Minimal viewer sidebar folders

* Simplified local and static viewer file tree folder rows so directories render
  as lightweight bold text instead of filled blocks with marker prefixes.
* This makes nested folder groups less visually heavy while preserving file row
  hover states and the existing sidebar structure.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-22 - Viewer graph hover presentation

* Removed canvas graph node shadows and label halo strokes from the local and
  static viewer graph presentation.
* Active graph nodes now rely on a stronger border instead of shadow effects,
  making graph hover states quieter and theme output simpler.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_theme.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/changelog/cli.md`.

### 2026-06-22 - Viewer shortcut registry and sidebar toggle

* Added a lightweight local viewer shortcut registry so viewer commands can
  register keyboard shortcuts through one shared handler instead of separate
  document-level listeners.
* Search uses `Command+K` as its visible shortcut while still accepting
  `Ctrl+K` as a non-macOS fallback. The file explorer sidebar can now be toggled
  with `Command+Option+S`, still accepts `Ctrl+Alt+S` as a fallback, and ignores
  the shortcut while typing in editable controls.
* The file explorer button now shows the formatted sidebar shortcut next to the
  icon as `⌘⌥S`, matching the visible `⌘K` search shortcut badge.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_shortcuts.js`,
  `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_search.js`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-22 - AST parser includes Markdown structure

* The OKF AST parser now adds a `markdown` tree to each parsed document,
  including block order for paragraphs, headings, code, blockquotes, lists,
  tables, thematic breaks, comments, and maintenance footer markers; a nested
  section tree; headings with anchors and source lines; Markdown links/images;
  fenced code blocks; Mermaid detection; and Markdown syntax diagnostics.
* AST-backed search now indexes headings from the parsed Markdown tree, context
  section boundaries come from parsed Markdown sections, and resolved document
  links are derived from Markdown AST links instead of a separate raw-content
  pass. Validation now reports Markdown syntax warnings from AST diagnostics
  instead of scanning raw body content separately. HTML export now renders from
  Markdown AST blocks, and compatibility render/search adapters use the AST
  parser instead of separate Markdown scans. Bundle display-title fallback now
  reads the first parsed H1 from the Markdown AST, including the local viewer
  header brand fallback.
* Source anchors: `packages/cli/internal/okf/ast_markdown.go`,
  `packages/cli/internal/okf/ast_markdown_types.go`,
  `packages/cli/internal/okf/markdown.go`,
  `packages/cli/internal/okf/html.go`,
  `packages/cli/internal/okf/ast_validate.go`,
  `packages/cli/internal/okf/ast_links.go`,
  `packages/cli/internal/okf/search.go`,
  `packages/cli/internal/okf/metadata.go`,
  `packages/cli/internal/okf/context_sections.go`,
  `packages/cli/cmd/openknowledge/viewer.go`.
* Docs updated: `Wiki/features/commands/ast.md`,
  `Wiki/features/commands/open.md`,
  `Wiki/features/exporters/html.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-22 - Local viewer search deep-link highlights

* The local viewer search API now returns `highlightText` and `highlightURL`
  for reliable visible text matches while preserving the existing `url` field.
* Navigating to a local viewer file URL with `?ok-highlight=<text>` opens the
  note panel, scrolls to the first matching rendered text, and marks it in the
  document.
* Source anchors: `packages/cli/internal/okf/search.go`,
  `packages/cli/internal/okf/search_types.go`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_search.js`,
  `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_app.css`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - AST command prints parser output

* Added `openknowledge ast [path]` to print the parsed OKF AST as formatted
  JSON, with `--spec <version>` and `--out <file>` support.
* AST JSON now uses lower-camel-case field names and omits internal diagnostic
  causes, making parser output easier to inspect before validation or exporter
  conversion.
* Source anchors: `packages/cli/cmd/openknowledge/ast_command.go`,
  `packages/cli/cmd/openknowledge/ast_command_test.go`,
  `packages/cli/internal/okf/ast_document_types.go`,
  `packages/cli/internal/okf/ast_frontmatter_types.go`,
  `packages/cli/internal/okf/ast_metadata_types.go`.
* Docs updated: `Wiki/features/commands/ast.md`,
  `Wiki/features/commands/index.md`, `Wiki/features/commands/help.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Use query mode

* Added `openknowledge use <name-or-path> --query <text>` as the token-bounded
  bundle reading path for agents.
* Query mode builds section-level Markdown context from headings, scores
  sections with lexical metadata/path/heading/body matches, and prints original
  excerpts that fit an approximate token budget.
* The mode supports Markdown output by default and structured JSON with
  `--format json`; it does not use embeddings or generated summaries.
* Source anchors: `packages/cli/internal/okf/context.go`,
  `packages/cli/internal/okf/context_test.go`,
  `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/use.md`,
  `Wiki/features/commands/index.md`, `Wiki/features/commands/help.md`,
  `Wiki/features/tooling-model.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer theme selector

* The local and static file viewer header now has a settings button with theme
  choices for Default, Night, Paper, Ocean, Rose, and Custom.
* Custom themes let users set page, surface, text, muted, accent, and border
  colors; the preference persists in browser storage with a cookie fallback.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_theme.css`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Agent maintenance footers render quietly

* Markdown rendering now hides HTML comments instead of escaping them into
  visible text.
* The `<!-- okf-footer: agent-maintenance -->` marker now wraps following
  content in a subdued footer treatment for the local viewer and default HTML
  exports.
* Source anchors: `packages/cli/internal/okf/markdown.go`,
  `packages/cli/internal/okf/markdown_test.go`,
  `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/internal/okf/html.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Use accepts bundle-relative entry paths

* `openknowledge use <name-or-path> <entry>` now resolves the optional entry
  argument as a declared `okf_bundle_entry_<name>` first, then as a
  bundle-relative file path when no declared entrypoint matches.
* This lets agents read any specific file inside a connected or local bundle
  without requiring root index metadata for every possible entrypoint.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/main_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/use.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Markdown extension files use OKF AST paths

* The OKF scanner now includes files ending in `.markdown` in addition to
  case-insensitive `.md` files.
* `.markdown` files now derive extensionless document IDs in the normalized
  JSON bundle model, matching `.md` behavior.
* The local viewer renders `.markdown` files through the parsed OKF bundle AST,
  so frontmatter stripping, body rendering, link graph data, and tree data share
  the same document model used by validation and exporters.
* Source anchors: `packages/cli/internal/okf/validate.go`,
  `packages/cli/internal/okf/paths.go`,
  `packages/cli/internal/okf/export_test.go`,
  `packages/cli/internal/okf/validate_versions_test.go`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/validate.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Parsed link metadata feeds validation and JSON

* Parsed Markdown documents now carry extracted link metadata, so validation and
  the normalized JSON bundle model share the same local link resolution data.
* Directory links are marked existing when they resolve through an `index.md`
  file in the target directory, matching validator behavior.
* Source anchors: `packages/cli/internal/okf/document.go`,
  `packages/cli/internal/okf/bundle.go`,
  `packages/cli/internal/okf/validate.go`,
  `packages/cli/internal/okf/export_test.go`,
  `packages/cli/internal/okf/validate_test.go`.
* Docs updated: `Wiki/features/exporters/json.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Registry materializes published bundles

* `openknowledge connect` and `openknowledge registry connect` now resolve
  remote sources as Open Knowledge manifests, direct tar archives, or Git
  repositories, materialize them into the Open Knowledge cache, validate archive
  materializations, and register source metadata.
* Added `openknowledge to tar --out <file>` for portable bundle archives.
* Default viewer HTML exports now include `openknowledge.json` plus
  `assets/openknowledge-bundle.tar.gz`, allowing deployed static wikis to be
  connected by URL.
* Registry storage now writes path-keyed `connections`.
* `disconnect --delete-files` documentation now describes current CLI-managed
  remote cache deletion behavior.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/internal/okf/archive.go`,
  `packages/cli/internal/okf/registry.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `packages/cli/internal/okf/archive_test.go`,
  `packages/cli/internal/okf/registry_test.go`.
* Docs updated: `Wiki/features/commands/connect.md`,
  `Wiki/features/commands/registry.md`,
  `Wiki/features/commands/disconnect.md`,
  `Wiki/features/commands/to.md`, `Wiki/features/exporters/html.md`,
  `Wiki/features/exporters/tar.md`, `Wiki/features/exporters/index.md`,
  `Wiki/features/tooling-model.md`, `README.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Docs clarify CLI tooling layers

* Reframed README and wiki navigation around authoring, local registry
  management, agent entrypoints, the local Markdown viewer, and export/publish
  layers.
* Added a tooling model page that distinguishes shipped local connections from
  remote source materialization.
* Source anchors: `README.md`, `Wiki/index.md`,
  `Wiki/features/tooling-model.md`, `Wiki/features/index.md`.
* Docs updated: `README.md`, `Wiki/index.md`,
  `Wiki/features/tooling-model.md`, `Wiki/features/index.md`,
  `Wiki/changelog/cli.md`, `Wiki/log.md`.

### 2026-06-20 - Deployed wiki brand title

* Set the deployed wiki brand through `Wiki/index.md` `okf_bundle_title` so the
  static viewer header shows `Open Knowledge CLI Documentation`.
* Clarified that `[html.source].entry` is a repository path prefix for GitHub
  source URLs, not a display title.
* Source anchors: `Wiki/index.md`, `Wiki/openknowledge.toml`,
  `Wiki/features/exporters/html.md`, `Wiki/features/operations.md`.
* Docs updated: `Wiki/features/exporters/html.md`,
  `Wiki/features/operations.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Landing page loads Google tag

* Added the Google `gtag.js` snippet with measurement ID `G-62SWM7FC2J` to the
  landing page head.
* Source anchors: `packages/web/index.html`, `Wiki/features/operations.md`.
* Docs updated: `Wiki/features/operations.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Railway serves install redirect

* The website server now redirects `/install` and `/install/` to the latest
  GitHub Release installer asset directly from `packages/web/scripts/serve.mjs`.
* This restores `https://openknowledge.sh/install` on Railway by keeping the
  redirect in the app server.
* Source anchors: `packages/web/scripts/serve.mjs`,
  `Wiki/features/operations.md`.
* Docs updated: `Wiki/features/operations.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Main branch deploys to Railway

* Added a GitHub Actions Railway deployment workflow that runs on pushes to
  `main`.
* The workflow verifies the repo with `pnpm test` and `pnpm build` before
  deploying through Railway's CLI container with `railway up`.
* Configure `RAILWAY_TOKEN` as a repository secret and `RAILWAY_SERVICE` as the
  Railway service name or service ID; optional `RAILWAY_PROJECT_ID` can pin the
  project. The workflow sends `RAILWAY_ENVIRONMENT`, defaulting to `production`,
  because Railway requires an environment whenever `--project` is used. The
  previous `RAILWAY_SERVICE_ID` name is still accepted as a fallback, but it
  must not contain the project ID.
* Added `railway.json`, a production web `start` script, and a Dockerfile so
  Railway builds `packages/web/dist` with both Go and Node/pnpm available, then
  serves the generated static site on `0.0.0.0`.
* Source anchors: `.github/workflows/deploy-railway.yml`,
  `railway.json`, `Dockerfile`, `.dockerignore`,
  `packages/web/package.json`, `packages/web/scripts/serve.mjs`,
  `Wiki/features/operations.md`.
* Docs updated: `Wiki/features/operations.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Mobile viewer hides fixed bottom chrome

* The shared viewer CSS now uses `svh` viewport sizing where supported and
  hides the fixed bottom scroll rail plus `Powered by OpenKnowledge.sh`
  attribution on mobile or touch viewports, avoiding iOS Safari browser chrome
  conflicts that do not reproduce in desktop-width emulation.
* Fenced code blocks now use body-sized monospace text, and shell command
  tokens no longer add extra font weight, keeping shell snippets visually
  consistent on iOS Safari.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Website server redirects short wiki command URLs

* The website server now redirects short wiki command aliases such as
  `/wiki/disconnect.html` and `/wiki/disconnect` to the generated canonical
  command page under `/wiki/features/commands/`.
* `pnpm dev:web` mirrors the same fallback after checking for existing static
  files, so local preview matches the deployed URL behavior.
* Source anchors: `packages/web/scripts/serve.mjs`.
* Docs updated: `Wiki/features/operations.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Website robots and text MIME handling

* Added `robots.txt` to the website build output and allowed all crawlers.
* The website server now serves `.txt` assets as `text/plain; charset=utf-8`,
  so generated static text files get the expected content type.
* Source anchors: `packages/web/robots.txt`,
  `packages/web/scripts/build.mjs`, `packages/web/scripts/serve.mjs`.
* Docs updated: `Wiki/changelog/cli.md`.

## v0.3.0 - 2026-06-20

Released as Git tag `v0.3.0` from commit `0a136ac`. These entries were still
stored under `Unreleased` at tag time and are now classified as V3 release
contents.

### 2026-06-20 - Root index frontmatter stays permissive

* `openknowledge validate` now tolerates unknown root `index.md` frontmatter
  keys instead of rejecting anything except `okf_version`.
* Optional CLI bundle metadata remains in root `index.md` as `okf_bundle_*`
  keys, which keeps `openknowledge new`, `connect`, `use`, and viewer branding
  aligned with the existing frontmatter-based metadata contract.
* Source anchors: `packages/cli/internal/okf/validate.go`,
  `packages/cli/internal/okf/validate_test.go`,
  `packages/cli/internal/okf/new.go`,
  `packages/cli/internal/okf/metadata.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/validate.md`,
  `Wiki/features/spec-compliance.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Static viewer brand stays inside exported wiki

* Default `openknowledge to html` viewer exports now link the header brand to
  the generated wiki `index.html` with a relative URL instead of `/`.
* This keeps deployed wiki exports under subpaths such as `/wiki/` from sending
  users back to the website root when they click the wiki brand.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/exporters/html.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Validator rejects invalid UTF-8 Markdown

* `openknowledge validate` now reports invalid UTF-8 Markdown files as errors
  before frontmatter, Markdown body, or link parsing.
* The validation report includes a dedicated `UTF-8 content` check, and concept
  document conformance fails when a concept file is not valid UTF-8.
* Source anchors: `packages/cli/internal/okf/validate.go`,
  `packages/cli/internal/okf/validate_test.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/validate.md`,
  `Wiki/features/spec-compliance.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer panels default to comfortable reading width

* Default note panels now use a `65ch` reading measure plus horizontal panel
  padding, capped to the viewport, instead of a fixed pixel width.
* The same default is used by the built-in viewer theme, the deployed wiki
  theme override, and the resize fallback for panels without saved widths.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_theme.css`,
  `packages/cli/cmd/openknowledge/viewer_app.js`,
  `Wiki/assets/openknowledge-site.css`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`,
  `Wiki/examples/syntax-highlighting.md`.

### 2026-06-20 - Viewer table filters use a ghost trigger

* The Markdown table toolbar now renders its `Filters` dropdown trigger as a
  ghost button, keeping the control quieter until hover, focus, or open state.
* The change applies to `openknowledge open` and default
  `openknowledge to html` viewer exports because they share `viewer_app.css`.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer table code wraps inside cells

* Inline code inside Markdown tables now wraps long unbroken values such as
  source paths and test names, so evidence-heavy tables stay inside their
  visual frame instead of requiring oversized horizontal overflow.
* The fix applies to `openknowledge open` and default `openknowledge to html`
  viewer exports because they share `viewer_app.css`.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer mobile sidebar closes after file selection

* On narrow mobile widths, selecting a file from the viewer sidebar now opens
  the note and closes the sidebar so the panel is visible immediately.
* Desktop behavior is unchanged: the sidebar stays open while selecting files.
* The behavior applies to `openknowledge open` and default
  `openknowledge to html` viewer exports because they share `viewer_app.js`.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer mobile sidebar hides bottom chrome

* On narrow mobile widths, opening the file explorer sidebar now hides the
  fixed bottom scroll rail and `Powered by OpenKnowledge.sh` attribution instead
  of translating them sideways into or beyond the drawer.
* The fix applies to `openknowledge open` and default `openknowledge to html`
  viewer exports because they share `viewer_app.css`.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Website dev server refreshes wiki export

* `pnpm dev:web` now regenerates `packages/web/dist/wiki` on startup before
  serving `/wiki/`, so the local website preview uses the current
  `openknowledge to html` viewer export instead of a stale generated bundle.
* `pnpm build:web` and `pnpm dev:web` now use the current Go CLI source by
  default for wiki exports; `OPENKNOWLEDGE_BIN` remains an explicit override
  for testing a specific binary.
* Source anchors: `packages/web/scripts/wiki-export.mjs`,
  `packages/web/scripts/build.mjs`, `packages/web/scripts/serve.mjs`.
* Docs updated: `Wiki/features/operations.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer note chrome stays above table controls

* Sticky note-panel chrome now layers above rich Markdown table search, filter,
  and dropdown controls, so scrolling table-heavy documents no longer lets the
  table UI cover the panel title, breadcrumbs, editor/source actions, or close
  button.
* The fix applies to `openknowledge open` and default `openknowledge to html`
  viewer exports because they share `viewer_app.css`.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Markdown list continuations stay inside bullets

* The shared Markdown renderer now treats indented continuation lines after
  unordered or ordered list markers as part of the current list item, so
  soft-wrapped docs no longer render continuation text as standalone
  paragraphs.
* Viewer document CSS now gives Markdown headings and lists explicit spacing,
  making section breaks and multi-line bullets easier to distinguish in local
  viewer panels and default HTML viewer exports.
* Source anchors: `packages/cli/internal/okf/markdown.go`,
  `packages/cli/internal/okf/markdown_test.go`,
  `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/commands/to.md`, `Wiki/features/exporters/html.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer code blocks get language labels

* Fenced Markdown code blocks now render with `data-language` metadata and the
  shared viewer stylesheet presents the language as a subtle inline label while
  keeping syntax highlighting prominent.
* Shell fences now additionally color command names, flags, variable
  assignments, and `$VARIABLE` / `${VARIABLE}` references, making CLI docs much
  easier to scan.
* The treatment applies to `openknowledge open`, code/text asset previews, and
  default `openknowledge to html` viewer exports because they share the same
  Markdown renderer and viewer CSS.
* Source anchors: `packages/cli/internal/okf/markdown.go`,
  `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/internal/okf/markdown_test.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/commands/to.md`, `Wiki/features/exporters/html.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer renders rich Markdown tables

* Markdown table rendering now emits stable table wrappers, `ok-table` classes,
  `scope="col"` headers, and `data-align` metadata for Markdown alignment
  markers.
* `openknowledge open` and default `openknowledge to html` viewer exports now
  progressively enhance Markdown tables with horizontal scrolling, whole-table
  text filtering, compact dropdown column filters, sortable headers, row counts,
  and a clear filters control inside the dropdown.
* `openknowledge to html --plain` still omits viewer CSS and JavaScript, but it
  receives the same semantic rendered table structure without the rich toolbar.
* Source anchors: `packages/cli/internal/okf/markdown.go`,
  `packages/cli/internal/okf/html.go`,
  `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/internal/okf/markdown_test.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/open.md`,
  `Wiki/features/commands/to.md`, `Wiki/features/exporters/html.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Static viewer source button replaces editor deeplinks

* Default `openknowledge to html` viewer exports no longer render the local
  editor dropdown that opens build-machine file paths.
* Bundles can configure `[html.source]` in `openknowledge.toml` with
  `github_base` and optional `entry`; exported Markdown panels then show a
  single GitHub source button resolved from that base plus the file path.
* When `[html.source]` is absent, exported pages show no editor or source
  action. The local `openknowledge open` viewer still shows the editor picker.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_theme.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `Wiki/openknowledge.toml`.
* Docs updated: `README.md`, `Wiki/features/commands/open.md`,
  `Wiki/features/commands/to.md`, `Wiki/features/exporters/html.md`,
  `Wiki/features/operations.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Registry owns connection commands

* `openknowledge registry connect`, `openknowledge registry disconnect`, and
  `openknowledge registry where` now own connection creation, removal, listing,
  and path lookup under one namespace.
* The early `openknowledge registry add` and top-level `openknowledge where`
  surfaces were removed. Top-level `openknowledge connect` and
  `openknowledge disconnect` remain as aliases for the registry subcommands.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/main_test.go`,
  `packages/cli/cmd/openknowledge/viewer.go`.
* Docs updated: `README.md`, `Wiki/features/commands/registry.md`,
  `Wiki/features/commands/connect.md`,
  `Wiki/features/commands/disconnect.md`, `Wiki/features/commands/help.md`,
  `Wiki/features/commands/index.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Static viewer supports hosted pretty URLs

* Default `openknowledge to html` viewer exports now map extensionless,
  lowercase, and directory-index pretty URLs back to their embedded static note
  manifest entries.
* This keeps stacked-panel navigation working on static hosts that rewrite
  generated links such as `AGENTS.html` to `/agents` or
  `features/index.html` to `/features/`.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer mobile header search no longer overlaps brand

* The shared viewer CSS now lets the top-bar search field override its desktop
  minimum width on narrow mobile screens, so the search control stays beside
  the file explorer button and knowledge base brand instead of covering them.
* The fix applies to both `openknowledge open` and default
  `openknowledge to html` viewer exports because they share the same embedded
  viewer app stylesheet.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/commands/to.md`, `Wiki/features/exporters/html.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Local viewer validates and applies theme config

* `openknowledge open` now treats `[html.theme]` in `openknowledge.toml` like
  the default HTML viewer export does: listing pages, Markdown file pages,
  asset previews, and alias-prefixed pages set `data-openknowledge-theme` and
  link the configured stylesheet through the raw endpoint.
* Local theme CSS paths are validated before rendering, so missing, directory,
  or otherwise invalid local stylesheet paths surface as viewer errors instead
  of silently falling back to the default theme.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_theme.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Static HTML export hides local root path

* Default viewer HTML exports no longer write the local bundle root into
  `data-note-root`, so deployed static sites do not expose the build machine's
  filesystem path.
* The static viewer still keeps stable per-page storage behavior by falling
  back to the page URL when no note root is present.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Website publishes wiki export

* The static website build now runs `openknowledge to html --out packages/web/dist/wiki Wiki`,
  so the deployed landing page can link to the repository wiki at `/wiki/`.
* The landing top navigation now includes a `Wiki` link before the GitHub icon,
  and the generated wiki uses `Wiki/openknowledge.toml` plus
  `Wiki/assets/openknowledge-site.css` to match the landing page theme.
* `pnpm dev:web` now falls back to `packages/web/dist/wiki` for `/wiki/` URLs,
  so `http://127.0.0.1:4173/wiki/` works after `pnpm build:web` even when the
  dev server is serving source files from `packages/web`.
* Source anchors: `packages/web/scripts/build.mjs`, `packages/web/index.html`,
  `packages/web/scripts/serve.mjs`, `packages/web/styles.css`,
  `Wiki/openknowledge.toml`, `Wiki/assets/openknowledge-site.css`.
* Docs updated: `Wiki/features/operations.md`, `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer hostname alias output removed

* `openknowledge open` no longer prints the secondary `Open Knowledge alias`
  URL or accepts `--local-domain`; the printed `Open Knowledge view` loopback
  URL remains the browser target.
* Stable knowledge base names still appear as served path segments such as
  `/wiki/` or `/personal/`, which work without local DNS or `/etc/hosts`
  changes.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `README.md`, `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Connected bundle commands shipped

* `openknowledge connect`, `openknowledge disconnect`, and
  `openknowledge use` now implement the previously documented connected bundle
  command surface for local bundles.
* `connect` stores local bundle connections with metadata-derived keys,
  validation status output, `--as`, `--access`, and `--no-validate`.
* `disconnect` removes connections by key or path while keeping files by
  default and refusing deletion for non-managed local entries.
* `use` prints default or named agent entrypoint Markdown, falls back to root
  `index.md`, and supports `--info` metadata output.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/internal/okf/registry.go`,
  `packages/cli/internal/okf/metadata.go`.
* Docs updated: `README.md`, `Wiki/features/commands/connect.md`,
  `Wiki/features/commands/disconnect.md`, `Wiki/features/commands/use.md`,
  `Wiki/features/commands/registry.md`, `Wiki/features/commands/help.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer panels use taller canvas

* `openknowledge open` now uses smaller vertical stack gutters around note
  panels, letting panels extend farther within the slimmer top chrome.
* Single-panel and multi-panel layouts keep matching vertical gaps while still
  reserving a compact bottom gap for the custom horizontal rail.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer sidebar surface flattened

* `openknowledge open` now renders the file explorer sidebar on the same
  neutral canvas color as the document workspace.
* The sidebar no longer draws a vertical border between itself and the shifted
  page content, making the open drawer feel seamless with the wiki background.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_theme.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer attribution reduced

* `openknowledge open` now renders the bottom-right
  `Powered by OpenKnowledge.sh` attribution at a smaller size so it reads as
  secondary viewer chrome.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.css`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer top bar height tightened

* `openknowledge open` document pages now render the top bar at the configured
  viewer header height instead of adding vertical padding on top of it.
* Header controls, the knowledge base brand, and the primary search field stay
  vertically centered in the slimmer chrome.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer app assets split from Go source

* The built-in `openknowledge open` viewer app CSS and JavaScript now live in
  normal source files (`viewer_app.css`, `viewer_app.js`, and
  `viewer_search.js`) instead of large raw string constants in `viewer.go`.
* The files are still embedded into the Go binary at build time, preserving the
  existing single-binary viewer behavior while making syntax highlighting and
  editing practical.
* Source anchors: `packages/cli/cmd/openknowledge/viewer_assets.go`,
  `packages/cli/cmd/openknowledge/viewer_app.css`,
  `packages/cli/cmd/openknowledge/viewer_app.js`,
  `packages/cli/cmd/openknowledge/viewer_search.js`,
  `packages/cli/cmd/openknowledge/viewer.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer website attribution

* `openknowledge open` document pages and default viewer HTML exports now show
  a bottom-right `Powered by OpenKnowledge.sh` link to the project website.
* The attribution sits alongside the viewer's bottom chrome and shifts with the
  file sidebar so it remains visible without covering the panel scroll rail.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer overview graph spacing

* `openknowledge open` now gives the empty-workspace file tree roughly 30% of
  the desktop overview width, leaving more room for the knowledge graph.
* Knowledge graph labels now use smaller sans-serif typography instead of the
  heavier monospace style, making labels under nodes read more quietly.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer resize handles follow panel scroll

* `openknowledge open` now keeps note panel resize handles aligned with the
  visible panel edges when the note content is scrolled vertically.
* This prevents the resize bars from disappearing at the top of long notes
  after a user scrolls inside a panel.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer knowledge base brand

* `openknowledge open` document and asset pages now show the knowledge base
  display name in the header instead of always showing `Open Knowledge`.
* The viewer prefers root `index.md` metadata in this order:
  `okf_bundle_title`, `okf_bundle_name`, root index title metadata, then the
  first root index H1, with `Open Knowledge` reserved as the final fallback.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - CLI docs moved into wiki

* The remaining operational notes from `docs/cli.md` now live in
  `Wiki/features/operations.md`, with install deployment notes kept in
  `Wiki/features/installation.md`.
* The wiki feature-docs workflow now points future docs work at the canonical
  wiki pages instead of the retired `docs/cli.md` file.
* Source anchors: `Wiki/features/operations.md`,
  `Wiki/features/installation.md`, `Wiki/workflows/feature-docs.md`.
* Docs updated: `Wiki/features/index.md`, `Wiki/log.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Export publish metadata

* `openknowledge validate` now accepts `okf_publish` metadata on `index.md`
  files, so public-view-only exclusions such as `okf_publish: false` do not
  make reserved index files invalid.
* `openknowledge to html` and `openknowledge to html --plain` now skip files
  whose frontmatter declares `okf_publish: false`; the default viewer export
  also omits unpublished files from its static note manifest and graph data.
* Nested `index.md` files still reject concept-style frontmatter such as
  `type: Index`.
* Source anchors: `packages/cli/internal/okf/validate.go`,
  `packages/cli/internal/okf/bundle.go`,
  `packages/cli/internal/okf/html.go`,
  `packages/cli/internal/okf/export_test.go`,
  `packages/cli/internal/okf/validate_test.go`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/validate.md`,
  `Wiki/features/exporters/html.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-20 - Viewer single-panel centering and resize

* A lone open note panel now uses symmetric viewport gutters so its center
  aligns exactly with the workspace center instead of drifting from asymmetric
  stack padding.
* Resizing a lone panel now expands or shrinks it around that center, so the
  dragged edge follows the pointer and the opposite edge moves the same amount
  in the opposite direction.
* Multi-panel resize behavior keeps the existing edge-anchored scroll handling
  for left-to-right pane browsing.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - HTML viewer export theming

* `openknowledge to html` default viewer exports now read optional
  `[html.theme]` settings from `openknowledge.toml` in the bundle root.
* Theme config supports `name` for `data-openknowledge-theme` and `stylesheet`
  (or `css`) for a deployable theme CSS file. Local stylesheets are constrained
  to the bundle, copied into the output folder, and linked relatively from every
  generated page; external `http` and `https` stylesheets are linked as-is.
* The default theme now lives in
  `packages/cli/cmd/openknowledge/viewer_theme.css`, which is embedded into
  the viewer app. The local viewer and default HTML export derive colors, fonts,
  graph colors, syntax colors, and viewer dimensions from its documented
  `--ok-*` variables.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_theme.go`,
  `packages/cli/cmd/openknowledge/viewer_theme.css`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/open.md`, `Wiki/features/commands/to.md`,
  `Wiki/features/exporters/html.md`, `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer resizable panels restored

* Note panels in the local viewer can be resized horizontally from either
  vertical edge, with a minimum width to keep notes readable.
* Panel widths are stored per note and restored when that note is opened again;
  notes without a saved width keep the existing default panel size.
* Right-edge resize handles now stay aligned with the panel edge after resizing
  instead of drifting into the note body when the panel has a vertical scrollbar.
* Single-panel workspaces now use the same bottom rail gap as multi-panel
  workspaces and no longer show a native horizontal scrollbar from one-sided
  stack padding.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer asset links and syntax highlighting

* `openknowledge open` now syntax-highlights fenced code blocks in rendered
  Markdown and highlights common code/text files opened through the local
  viewer.
* Local links to code/text assets open escaped source preview pages, while local
  PDF, image, audio, and video references resolve to bundle-scoped raw URLs so
  the browser can use native PDF and media viewers.
* Raw asset responses are constrained to files under the knowledge root and set
  `X-Content-Type-Options: nosniff`; active code-like raw types are served as
  plain text.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `packages/cli/internal/okf/markdown.go`,
  `packages/cli/internal/okf/markdown_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - New bundle metadata flags

* `openknowledge new` now accepts optional `--bundle-name`, `--bundle-title`,
  `--bundle-purpose`, repeatable `--bundle-tag`, and repeatable
  `--bundle-entry name=path` flags.
* The scaffold writes those values into root `index.md` as flat
  `okf_bundle_*` metadata while preserving the default minimal scaffold when no
  metadata flags are provided.
* Validation now accepts `okf_bundle_*` keys in the bundle-root `index.md` as
  an Open Knowledge CLI metadata layer; plain OKF bundles with only
  `okf_version: "0.1"` remain valid, and nested `index.md` files still cannot
  use frontmatter.
* Source anchors: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/internal/okf/new.go`,
  `packages/cli/internal/okf/validate.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/new.md`,
  `Wiki/features/commands/validate.md`,
  `Wiki/features/commands/help.md`,
  `Wiki/features/commands/use.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer knowledge graph canvas physics

* The empty-state knowledge graph now renders as an animated canvas graph
  instead of static SVG, allowing lightweight physics to keep nodes responsive
  after the deterministic initial layout.
* Hover and keyboard focus now ease the active node label and separation forces
  in and out, with velocity clamping and damping to reduce jitter in displaced
  nodes.
* Non-active nodes keep their default visual style during hover; the emphasis is
  on the active node and its direct connections.
* Default graph lines are visually lighter so the connected-edge highlight is
  the main emphasis during graph exploration.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer markdown links with code labels

* Markdown links whose labels contain inline code spans, for example a React
  docs link whose visible label includes `useEffect` as code, now render as
  clickable anchors in `openknowledge open` instead of leaking the raw Markdown
  syntax.
* Inline code spans that contain link-looking text remain literal code and are
  not converted into anchors.
* Source anchors: `packages/cli/internal/okf/markdown.go`,
  `packages/cli/internal/okf/markdown_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer knowledge graph clustering

* The empty-state knowledge graph now uses a deterministic force-style layout
  so linked notes cluster together instead of being arranged in a fixed circle.
* The graph layout now runs collision passes against node and label bounds to
  reduce overlapping note names when the graph has enough room to separate them.
* Generic `index` graph labels now include path suffix context, such as
  `commands/index`, to distinguish nested index files.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer multi-panel horizontal scrolling

* Multi-panel document stacks now use an Andy Matuschak-style horizontal flex
  scroll container plus a custom always-visible bottom rail for horizontal
  movement on mouse or trackpad devices.
* The rail thumb can be dragged, the rail track can be clicked, and the focused
  thumb supports keyboard scrolling.
* The gray workspace gaps support mouse drag scrolling left and right while
  preserving normal text selection inside note panels.
* Holding `Space` now enables canvas-style mouse panning across note panels, so
  sideways dragging scrolls the stack without opening links under the pointer.
* Browser-aborted View Transition animations no longer surface as viewer app
  errors after the stack DOM update has already completed.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Setup skill subagent guidance

* Updated the setup prompt and generated `SETUP.MD` so repo-scoped or
  user-scoped skills should include guidance for spawning focused subagents
  with lower reasoning effort for bounded wiki maintenance tasks when the
  runtime supports that.
* Source anchors: `packages/cli/internal/okf/setup.go`,
  `packages/cli/internal/okf/new.go`,
  `packages/cli/internal/okf/setup_test.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/setup.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer single-panel centering

* The panel viewer now centers a lone open panel in the workspace.
* Opening a second panel removes the single-panel centering and keeps the
  existing left-to-right stack browsing behavior.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer file tree system badges

* The viewer file explorer now shows only the filename in each file row instead
  of repeating the full relative path on the right.
* Removed the generic `md` badge and replaced it with a right-aligned `system`
  badge only for reserved Markdown files such as `index.md` and `log.md`.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer search dropdown focus and keyboard controls

* The viewer search dropdown now opens on focus with top file entries for an
  empty query and stays open while typed search requests are pending, avoiding
  flicker between keystrokes.
* The dropdown closes when a result is activated, including after pending search
  requests resolve.
* Search now gives `index.md` files lower priority than comparable regular
  pages in both the local search API and exported static HTML.
* The document viewer header keeps its vertical padding so the top-bar search
  aligns with the logo.
* Search results can be selected with `ArrowDown`/`ArrowUp` and opened with
  `Enter` while focus stays in the search field.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `packages/cli/internal/okf/search.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Viewer search moved out of sidebar

* Removed the duplicate search box from the file explorer sidebar; viewer
  search now lives only in the top bar.
* `Command+K` on macOS and `Ctrl+K` elsewhere still focus the top-bar search,
  and exported static HTML keeps the same search behavior.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Empty workspace graph overview

* The panel viewer empty workspace now uses a 50/50 overview with the file tree
  on the left and a connected graph of Markdown files on the right.
* The graph is built from local Markdown links and graph nodes open files as
  panels, including in exported static HTML.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Sidebar search restored in viewer

* The panel viewer file explorer now includes a search box above the file tree.
* The top bar now includes the primary search field, focused by `Command+K` on
  macOS and `Ctrl+K` elsewhere.
* Local `openknowledge open` pages use the existing `/api/search` endpoint, and
  exported static HTML searches the embedded note manifest in-browser.
* Search result clicks open as panels and keep the sidebar open.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Local viewer always uses panel stack

* Removed the local viewer focus-mode toggle so document browsing always uses
  the horizontally scrollable panel stack.
* File-tree and Markdown link navigation now consistently append or replace
  panels instead of switching into a single-page layout.
* Stack View Transitions now clear fallback panel-entry animation classes before
  the live DOM is shown again, avoiding a second flash after the transition.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`.
* Docs updated: `Wiki/features/commands/open.md`,
  `Wiki/changelog/cli.md`.

### 2026-06-19 - Reachable local viewer URL

* `openknowledge open` now prints and opens the actual listener URL as the
  `Open Knowledge view` line, defaulting to `127.0.0.1`, so direct path aliases
  such as `/wiki/` remain reachable without local DNS setup.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/main.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/open.md`, `Wiki/changelog/cli.md`.

### 2026-06-18 - Markdown and frontmatter validation warnings

* `openknowledge validate` now checks Markdown syntax for malformed links,
  unclosed code spans, invalid table separators, and unclosed fenced code blocks.
* Parseable frontmatter formatting issues, such as duplicate keys or delimiter
  whitespace, are reported as warnings; frontmatter that cannot be parsed
  remains an error.
* Source anchors: `packages/cli/internal/okf/validate.go`,
  `packages/cli/internal/okf/frontmatter.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/validate.md`, `Wiki/changelog/cli.md`.

### 2026-06-18 - Registry-backed local viewer

* Changed `openknowledge open` without a path to open the Open Knowledge
  Registry viewer, with a left workspace selector for registered knowledge
  bases.
* Kept `openknowledge open <path-or-name>` as the direct viewer for one
  knowledge base.
* Source anchors: `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/main.go`.
* Docs updated: `README.md`,
  `Wiki/features/commands/open.md`, `Wiki/features/commands/registry.md`.

### 2026-06-18 - Context-aware setup interview prompt

* Updated `openknowledge setup` so agents inspect the current workspace or
  target folder and relevant runtime-exposed memories before asking questions.
* The setup prompt and generated `SETUP.MD` now tell agents to ask only missing,
  context-specific questions instead of repeating a fixed questionnaire.
* Source anchors: `packages/cli/internal/okf/setup.go`,
  `packages/cli/internal/okf/new.go`, `packages/cli/cmd/openknowledge/main.go`.
* Docs updated: `README.md`, `packages/web/index.html`,
  `Wiki/features/commands/setup.md`.

### 2026-06-18 - Wiki maintenance loop initialized

* Created a colocated Open Knowledge wiki at `Wiki/`.
* Added command, exporter, installation, workflow, and changelog seed pages.
* Added root `AGENTS.md` and repo skill `.codex/skills/openknowledge-wiki/SKILL.md`
  so future agents update this wiki when touching CLI behavior.

## Baseline Command Surface

As of the wiki setup, the CLI exposes:

* `openknowledge setup`
* `openknowledge new`
* `openknowledge registry list`
* `openknowledge registry add`
* `openknowledge where`
* `openknowledge open`
* `openknowledge to html`
* `openknowledge to json`
* `openknowledge spec`
* `openknowledge validate`
* `openknowledge list`
* `openknowledge version`

## Entry Template

```md
### YYYY-MM-DD - Short change title

* What changed:
* Why it matters:
* Source anchors:
* Docs updated:
```

## Update Rules

Add an entry when a change affects command behavior, arguments or flags, help
text, validation rules, export output, viewer behavior, setup prompts, registry
semantics, release packaging, npm wrapper behavior, or developer-facing docs.

Do not add entries for purely internal refactors unless they alter user-visible
or developer-relevant behavior.
