---
type: Changelog
title: CLI Changelog
description: Release-level history for the Open Knowledge CLI.
tags: [openknowledge, cli, changelog]
timestamp: 2026-08-12T00:00:00Z
---

# CLI Changelog

Current behavior belongs in the [command reference](/features/commands/). This
page records release-level changes.

## Unreleased

### Maintenance routing

- Insights now carry normalized risk, approval, confidence, and owner routes.
  Low risk requires at least 0.95 confidence and permits automatic processing.
  Medium risk requires human approval. High risk requires expert approval and
  keeps declared knowledge targets evidence-only for automation.
- New `okn automation insights from-audit` creates stable, deduplicated
  insights from audit findings and routes target owners from OKF frontmatter.
- Hosted jobs carry bounded maintenance attestations. GitHub owners can request
  user or team review with `github:<login>` or `github-team:<slug>`.
- Optional `github.auto_merge_low_risk` creates ready low-risk pull requests
  and squash merges only after exact required checks succeed. Incomplete gates
  keep the exchange proposal for a later publisher retry.
- Source: `packages/cli/internal/insights/`,
  `packages/cli/cmd/openknowledge/insights_command.go`,
  `packages/cli/internal/agents/templates.go`,
  `packages/cli/cmd/openknowledge/runtime_worker.go`,
  `packages/cli/internal/runtime/config.go`,
  `packages/cli/internal/runtime/github.go`, and
  `packages/cli/schemas/v1/runtime-plan.schema.json`.
- Docs: `Wiki/features/commands/insights.md`,
  `Wiki/features/commands/jobs.md`, `Wiki/features/commands/runtime.md`, and
  `Wiki/features/machine-contracts.md`.

### Knowledge audit

- New `okn audit` reports deterministic evidence-backed risks for staleness,
  sources, ownership, local dependencies, duplicates, and structured claims.
- Optional source baselines detect changed local content or remote
  `last_modified` updates.
- Private usage inputs add recurring unanswered-question and high-use
  unverified findings.
- `--fail-on` provides severity gates. Repository CI now uploads the JSON
  report and fails on high-risk findings.
- New `owner`, `owners`, and `claims` frontmatter extensions support audit
  routing and structured conflict detection.
- Source: `packages/cli/cmd/openknowledge/audit_command.go`,
  `packages/cli/internal/audit/`,
  `packages/cli/schemas/v1/audit-report.schema.json`,
  `packages/cli/schemas/v1/audit-source-baseline.schema.json`, and
  `.github/workflows/ci.yml`.
- Docs: `Wiki/features/commands/audit.md`,
  `Wiki/features/commands/index.md`, and
  `Wiki/features/machine-contracts.md`.

### Publication gate

- Runtime publication can now require exact GitHub check names on the
  production commit. Missing, pending, and failed checks stop publication.
- The publisher verifies checks before it creates a production source bundle
  or generation. Manual publication cannot bypass configured required checks.
- Successful check names are bound into the generation content digest.
  Retrieval contracts and private usage events return `generation.checks`.
- Hosted job exchange can carry a bounded worker-reported passing eval summary
  for draft pull request and job check output. Raw reports remain private.
- Repository CI now runs knowledge eval on pull requests and `main` pushes.
  The push comparison uses the prior production SHA.
- Source: `packages/cli/internal/runtime/github.go`,
  `packages/cli/internal/runtime/generation.go`,
  `packages/cli/internal/runtime/config.go`,
  `packages/cli/cmd/openknowledge/runtime_command.go`,
  `packages/cli/cmd/openknowledge/runtime_worker.go`,
  `packages/cli/cmd/openknowledge/runtime_retrieval.go`,
  `packages/cli/internal/usage/`, `.github/workflows/ci.yml`, and
  `.github/workflows/knowledge-eval.yml`.
- Docs: `Wiki/features/commands/runtime.md`,
  `Wiki/features/commands/jobs.md`, `Wiki/features/commands/eval.md`,
  `Wiki/features/machine-contracts.md`, and
  `Wiki/features/knowledge-architecture.md`.

### Runtime usage

- Runtime search can now record private local usage events for HTTP and MCP
  search. Recording and sanitized query capture are separate opt-in settings.
- Events use a private HMAC key and contain no user, session, IP address, or
  request header fields. The default event contains no query text.
- `okn automation insights from-usage` converts recurring no-evidence and
  policy-rejected clusters into stable private insights.
- Captured questions can produce a private strict eval dataset. Exclusive file
  creation prevents replacement of an existing dataset.
- New `usage-event.schema.json` defines the local event v1 contract.
- Source: `packages/cli/internal/runtime/config.go`,
  `packages/cli/cmd/openknowledge/runtime_serve.go`,
  `packages/cli/cmd/openknowledge/runtime_retrieval.go`,
  `packages/cli/cmd/openknowledge/insights_command.go`,
  `packages/cli/internal/usage/`, `packages/cli/internal/insights/`, and
  `packages/cli/schemas/v1/usage-event.schema.json`.
- Docs: `Wiki/features/commands/runtime.md`,
  `Wiki/features/commands/insights.md`, `Wiki/features/telemetry.md`, and
  `Wiki/features/machine-contracts.md`.

### Runtime retrieval

- Runtime `[[access_profiles]]` now give HTTP search and MCP separate
  environment-backed bearer tokens, published knowledge base allowlists, and
  agent, team, or use-case routing labels.
- A profile can replace the global retrieval policy. Configured profiles
  replace the legacy single MCP token and bind each MCP session to one profile.
- Runtime search and context responses now include access identity and an
  explicit `answer` or `refuse` decision. Refusals identify missing evidence,
  policy rejection, or insufficient MCP context budget.
- Refusal responses contain no selected evidence. They keep rejected
  candidates visible for review.
- `[serve.retrieval_policy]` now filters runtime evidence by minimum trust,
  staleness, lifecycle status, and structured source presence. Defaults remain
  permissive.
- Runtime HTTP search and MCP `openknowledge_search` enforce the policy for
  each candidate. Responses include rejected candidates and exact reasons.
- New v1 runtime search and context contracts include generation identity,
  trust, freshness, provenance, and selection metadata.
- MCP `resources/list` and `resources/read` remain exact access methods for the
  MCP projection. The retrieval policy does not make them access controls.
- Source: `packages/cli/internal/runtime/config.go`,
  `packages/cli/cmd/openknowledge/runtime_retrieval.go`,
  `packages/cli/cmd/openknowledge/runtime_serve.go`,
  `packages/cli/schemas/v1/runtime-plan.schema.json`,
  `packages/cli/schemas/v1/runtime-search.schema.json`, and
  `packages/cli/schemas/v1/runtime-context.schema.json`.
- Docs: `Wiki/features/commands/runtime.md` and
  `Wiki/features/machine-contracts.md`.

### Knowledge CI

- Eval dataset cases can now identify agent consumers with optional `agents`.
  Base comparisons map changed paths to affected questions and agents.
- Comparison reports now contain `changedPaths`, `affectedAgents`,
  `affectedQuestions`, and `uncoveredPaths`. Attribution uses expected,
  retrieved, and cited sources plus retrieval, outcome, and answer changes.
- Text and Markdown reports summarize review impact. Uncovered paths identify
  changed knowledge paths that have no eval case source link.
- Eval dataset v1 now supports `minimum_trust`, `allow_stale`,
  `allowed_statuses`, and `require_sources` expectations. Each policy check
  tests all selected sources against derived OKF 0.2 metadata.
- JSON and Markdown reports include policy check results. Failed checks identify
  the sources that violate trust, freshness, lifecycle, or provenance rules.
- `okn eval run` now tests deterministic retrieval evidence against strict,
  versioned YAML datasets. Expectations cover sources, included or excluded
  evidence, and minimum source counts.
- Text output summarizes each case. Versioned JSON reports bind results to the
  dataset digest and corpus revision. Markdown output creates a pull request
  report with status, answer changes, citations, groundedness, and failed checks.
- An explicit answer command can use retrieved source Markdown through a strict
  JSON stdin and stdout protocol. New expectations test answer text, citation
  sources, valid citation counts, and claim groundedness.
- The CLI runs answer commands directly without a shell or sandbox. Retrieval
  remains deterministic. Answer reproducibility depends on the selected command.
- Failed expectations return exit status `1`, which supports continuous
  integration gates.
- `--base <git-ref>` now compares the working knowledge base with an immutable
  Git archive snapshot. Both revisions use the same current dataset.
- Each case is `improved`, `regressed`, `unchanged_pass`, or `unchanged_fail`.
  The `all` gate rejects all proposed failures. The `regressions` gate rejects
  only regressions.
- Jobs now accept structured `verify.eval` settings. The runner performs a
  native base comparison and retains private JSON and Markdown reports.
- The built-in `knowledge-eval` job template provides an agentless native gate.
- Eval gate failures now set `verification_failed`. Run plans and run records
  expose resolved eval configuration and result summaries.
- The repository includes a reusable `knowledge-eval.yml` workflow. It adds
  Markdown to the GitHub job summary and uploads both reports for 14 days.
- Source: `packages/cli/cmd/openknowledge/eval_command.go`,
  `packages/cli/internal/eval/`, `packages/cli/schemas/eval/v1/`,
  `packages/cli/schemas/v1/eval-report.schema.json`,
  `packages/cli/schemas/v1/eval-comparison.schema.json`,
  `packages/cli/internal/agents/`,
  `packages/cli/schemas/v1/job-run-plan.schema.json`,
  `packages/cli/schemas/v1/job-run-record.schema.json`, and
  `.github/workflows/knowledge-eval.yml`.
- Docs: `Wiki/features/commands/eval.md`, `Wiki/features/commands/jobs.md`, and
  `Wiki/features/machine-contracts.md`.

### Viewer

- Markdown note panels now provide browser-native text-to-speech controls for
  reader-facing content, with pause, resume, and stop states. The feature uses
  the browser-selected voice and requires no Open Knowledge-managed API key.
  That voice may be local or network-backed. Narration omits frontmatter,
  agent-only annotations, code blocks, Mermaid diagrams, and hidden UI.
- Source: `packages/cli/cmd/openknowledge/viewer_templates.go`,
  `packages/web/src/viewer/app.js`,
  `packages/web/src/viewer/styles/document.css`, and
  `packages/web/scripts/browser.e2e.mjs`.
- Docs: `Wiki/features/commands/view.md` and
  `Wiki/features/exporters/html.md`.
- `okn view` now watches local Markdown and asset files. It refreshes an open
  page after an add, update, rename, move, or deletion.
- Live reload preserves surviving document stacks, active documents, graph
  state, and scroll positions. It removes deleted paths before refresh.
- A content revision and authenticated Server-Sent Events stream coalesce save
  bursts. Static HTML exports do not include the live reload client.
- Source: `packages/cli/cmd/openknowledge/viewer_live_reload.go`,
  `packages/cli/cmd/openknowledge/viewer_live_reload_test.go`,
  `packages/web/src/viewer/live-reload.js`,
  `packages/web/src/viewer/app.js`, and
  `packages/web/scripts/viewer-live-reload.e2e.mjs`.
- Docs: `Wiki/features/commands/view.md`.

### Retrieval

- Search now uses a deterministic Go standard library in-memory inverted
  index. The index has one sorted vocabulary and field postings.
- Exact lookup no longer scans every section. Prefix lookup uses a vocabulary
  range. Fuzzy lookup can scan the vocabulary.
- Ranking, CLI output, and machine-readable contracts do not change.
- The public Go API now provides an immutable, revision-bound `ContextIndex`.
  `BuildContextIndex` and `BuildContextIndexWithVersion` create the snapshot.
- `ContextIndex.Search` and `ContextIndex.Resolve` reuse the snapshot for
  concurrent requests. These methods do not read the source files again.
- Existing `Search`, `SearchWithVersion`, `ResolveContext`, and
  `ResolveContextWithVersion` remain compatible one-shot helpers.
- Source: `packages/cli/internal/okf/context.go`,
  `packages/cli/internal/okf/search.go`,
  `packages/cli/internal/okf/search_knowledge.go`,
  `packages/cli/internal/okf/search_inverted_index_test.go`,
  `packages/cli/internal/okf/search_benchmark_test.go`,
  `packages/cli/okf/context_index.go`, `packages/cli/okf/read.go`, and
  `packages/cli/okf/read_test.go`.
- Docs: `Wiki/features/knowledge-architecture.md` and
  `Wiki/features/go-api.md`.

### Rules and setup

- New knowledge bases now enable the built-in `project` and `writing` rules.
  The rule catalog also offers `iso-plain-language` as an optional rule.
- Setup uses the same default selection. Explicit CLI or
  `.openknowledge.toml` rule selections continue to replace the defaults.
- Ordinary setup and source setup now include complete selected-rule
  instructions. Their scaffold commands persist the exact selection.
- `okn scaffold --rules <rules>` persists an explicit built-in selection.
  Starter agent guidance includes those rules. The terminal validation handoff
  includes the created bundle path.
- Source: `packages/cli/internal/okf/rules.go`,
  `packages/cli/internal/okf/rule_catalog.go`,
  `packages/cli/internal/okf/new.go`,
  `packages/cli/internal/okf/from.go`,
  `packages/cli/internal/okf/setup.go`, and
  `packages/cli/cmd/openknowledge/setup_command.go`.
- Docs: `Wiki/features/commands/rules.md`,
  `Wiki/features/commands/setup.md`,
  `Wiki/features/commands/scaffold.md`, and
  `Wiki/features/configuration.md`.

### Content review and annotations

- `okn prompt review content` now prints a portable, advisory content-health
  review with deterministic bundle, page, Git, concern, and ordered-ruleset
  identity. Changed scope includes direct local Markdown dependencies; full
  scope works outside Git.
- Markdown now supports bounded `agent-context` annotations. The AST preserves
  their child blocks, viewers show them with subdued text, and
  ordinary reader search excludes their content. Legacy maintenance footers
  remain accepted through end-of-file.
- Source: `packages/cli/internal/okf/content_review.go`,
  `packages/cli/internal/okf/ast_markdown.go`,
  `packages/cli/internal/okf/markdown.go`, and
  `packages/cli/cmd/openknowledge/main.go`.
- Docs: `Wiki/features/commands/review.md`,
  `Wiki/features/commands/ast.md`,
  `Wiki/features/commands/view.md`, and
  `Wiki/features/exporters/html.md`.

### Job validation foundation

- Jobs can run deterministic preflight commands before an agent starts. A
  failed preflight prevents the harness and post-agent verification from
  running.
- Empty-prompt jobs with verification commands can omit the agent runtime. The
  new `content-validation` template uses this form and never selects model
  credentials.
- Published run-plan and run-record schemas now cover preflight artifacts,
  agentless plans, the `preflight_failed` status, and supported draft-PR output.
- Source: `packages/cli/internal/agents/spec.go`,
  `packages/cli/internal/agents/plan.go`,
  `packages/cli/internal/agents/runner.go`, and
  `packages/cli/internal/agents/templates.go`.
- Docs: `Wiki/features/commands/jobs.md`.

## v0.12.0 — 2026-08-11

Version 0.12 expands the multi-knowledge-base viewer. It supports non-Git setup,
makes HTML source archives optional, adds examples, and requires Node 20.

### Viewer

- Source and text files now open as syntax-highlighted note cards in the
  document stack. Markdown cards include collapsed typed frontmatter and OKF
  0.2 signals.
- The viewer sidebar now has separate **Documents**, **Graph**, and
  **Knowledge bases** items. The registry workspace lists connected knowledge
  base trees and can connect another local knowledge base.
- Registry search now covers all connected knowledge bases and identifies each
  result's source. Connections are read-only unless the user enables editor
  links. A folder without a knowledge base shows its `okn setup` command.
- The combined registry graph uses saved knowledge-base colors. It does not
  create links between knowledge bases. New controls support pan, zoom, node
  drag, filters, display options, force settings, and animation control.
- Source: `packages/cli/cmd/openknowledge/viewer_templates.go`,
  `packages/cli/cmd/openknowledge/viewer.go`,
  `packages/cli/cmd/openknowledge/viewer_test.go`,
  `packages/web/src/viewer/app.js`,
  `packages/web/src/viewer/search.js`,
  `packages/web/src/viewer/styles/`,
  and `packages/web/scripts/browser.e2e.mjs`.
- Docs: `Wiki/features/commands/view.md` and
  `Wiki/features/exporters/html.md`.

### Export

- `okn export html --no-source-archive` now omits the portable source archive
  and its connect manifest. Viewer HTML exports include both files by default.
- Source: `packages/cli/cmd/openknowledge/main.go`,
  `packages/cli/cmd/openknowledge/viewer_export.go`, and
  `packages/cli/cmd/openknowledge/command_help.go`.
- Docs: `Wiki/features/commands/export.md`,
  `Wiki/features/exporters/html.md`, and `Wiki/features/exporters/tar.md`.

### Setup

- Setup and project skill installation now support knowledge bases outside Git
  repositories. Git remains optional for an OKF bundle.
- Source: `packages/cli/cmd/openknowledge/setup_command.go`,
  `packages/cli/cmd/openknowledge/setup_lifecycle_command.go`,
  `packages/cli/cmd/openknowledge/setup_skill_command.go`, and
  `packages/cli/internal/integration/`.
- Docs: `Wiki/features/commands/setup.md`,
  `Wiki/features/tooling-model.md`,
  `Wiki/features/knowledge-architecture.md`, and
  `Wiki/decisions/product-interface.md`.

### Website

- The landing and getting-started pages now use Google Advanced Consent Mode
  v2. Google receives cookieless measurements while `analytics_storage` is
  `denied`. Selecting **Allow** enables Analytics cookies. Advertising storage,
  user data, and personalization remain denied.
- Source: `packages/web/src/analytics.js`, `packages/web/index.html`,
  `packages/web/getting-started/index.html`, and
  `packages/web/scripts/browser.e2e.mjs`.
- Docs: `Wiki/features/telemetry.md` and `Wiki/features/operations.md`.
- The three examples cover project context, release memory, and research
  synthesis. Each example includes validation, search, export, and tests.
- `pnpm test:demos` verifies all three knowledge bases against the current CLI.
- The guides use ASD-STE100 rules and supported CLI command syntax.
- Source: `examples/`, `scripts/test-demo-knowledge-bases.sh`, and
  `package.json`.
- Docs: `Wiki/index.md`, `Wiki/features/operations.md`, and
  `Wiki/changelog/cli.md`.

### Compatibility

- Node 20 is now the minimum version for the npm wrapper, website development,
  and JavaScript tests. The baseline CI job verifies Node 20.
- Source: `package.json`, `packages/npm/package.json`,
  `packages/web/package.json`, `.github/workflows/ci.yml`, and
  `scripts/check-versions.mjs`.
- Docs: `packages/npm/README.md`, `Wiki/features/installation.md`,
  `Wiki/features/operations.md`, and `Wiki/changelog/cli.md`.

### Security

- Mermaid diagram rendering now uses Mermaid 11.16.1. Its dependency updates
  include DOMPurify 3.4.13 and nanoid 3.3.17 to fix seven known vulnerabilities.
- Source: `package.json`, `packages/web/package.json`, `pnpm-lock.yaml`.
- Docs: `Wiki/changelog/cli.md`.

## v0.11.0 — 2026-08-08

Version 0.11 adds privacy-safe product telemetry, standalone skill setup, a
new website guide, and a refreshed default viewer theme.

### Viewer

- The local viewer and interactive HTML exports now use the light blue Open
  Knowledge theme on the first visit and after a settings reset.
- The **Night** theme remains available. A saved browser preference overrides
  the new default.
- Source: `packages/cli/cmd/openknowledge/viewer_theme.css`,
  `Wiki/assets/openknowledge-site.css`,
  `packages/cli/cmd/openknowledge/viewer_templates.go`,
  `packages/web/src/viewer/`.
- Docs: `Wiki/features/commands/view.md`,
  `Wiki/features/exporters/html.md`.

### Release automation

- The release guide now documents the repository's four release jobs, input
  formats, credential preflight, exact commit handoff, and tag reuse rule.
- Source: `.github/workflows/release.yml`.
- Docs: `Wiki/features/operations.md`.

### Telemetry

- The CLI now discloses default-on anonymous usage and sanitized error telemetry
  before it sends the first event. Telemetry commands inspect the payload,
  report status, and save an opt-in or opt-out.
- `--no-telemetry` saves an opt-out before the command runs. Installer
  preflight and continuous integration do not send events.
- The first-party website relay validates an exact event allowlist. Website
  page and copy events require consent.
- The `/install` redirect records an aggregate source and client family without
  a browser or installation identifier.
- The relay now maps accepted events to PostHog's batch ingestion protocol,
  keeps the project token server-side, and disables person-profile processing.
- The relay now maps each `cli_error` to a synthetic PostHog `$exception` event.
  Native issue grouping receives no raw message, stack trace, path, or output.
- The CLI schema does not change. The CLI does not include the PostHog Go SDK
  or a PostHog project token.
- Product telemetry stays separate from opt-in local session observation.
- Source: `packages/cli/internal/telemetry/`,
  `packages/cli/cmd/openknowledge/telemetry_command.go`, `install`, and
  `packages/web/`.
- Docs: `Wiki/features/telemetry.md` and
  `Wiki/features/commands/telemetry.md`.

### Setup

- `okn setup skill` now installs global or project instructions without the
  complete knowledge-base setup flow. Interactive use selects the scope,
  project target when required, and detected harnesses.
- Noninteractive use supports `--scope`, repeatable `--harness`, and
  `--project`. A global installation does not require a Wiki or registry entry.
- Source: `packages/cli/cmd/openknowledge/setup_skill_command.go`.
- Docs: `Wiki/features/commands/setup.md`.

## v0.10.0 — 2026-08-04

Version 0.10 makes OKF 0.2 the default, unifies setup, expands provenance
views, and improves viewer navigation, registry refreshes, and local
automation.

### OKF 0.2

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
- `okn list` shows trust, lifecycle status, and stale content. List and graph
  JSON also expose a derived `okf02` contract.
- Graph output connects sources, computations, executors, and attesters with
  typed provenance edges.
- Viewer pages and HTML exports show provenance, linked sources, and Attested
  Computation contracts.
- The CLI preserves executor and attester declarations. It never runs these
  resources automatically.

### Setup

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
- The generated task starts with an open-ended setup interview for `Wiki`.
- The README and website explain setup with an existing agent or a selected
  CLI runtime. The website copy action includes complete agent instructions.
- The README workflow diagram includes MCP.

### Viewer

- The **Frontmatter** disclosure contains OKF 0.2 trust, lifecycle,
  provenance, source, and computation details.
- Authored Markdown starts the visible document without a separate metadata
  block above it.
- Mobile sidebar and note navigation now show destinations immediately without
  transition delays.
- The file explorer stays visible during open-beside navigation. Resize it
  from its `25vw` default within its limits.
- The viewer keeps the header, note workspace, and horizontal scroll rail in
  the main content column.
- Open a Mermaid diagram with a click, Enter, or Space. The viewport dialog
  provides zoom, pan, **Fit**, and **100%** controls.
- The local viewer and interactive HTML exports use the same diagram controls.

### Automation

- A scheduled job runs once for each job ID and schedule slot. Repository or
  job-file changes do not replay that slot.
- Each private jobs worker uses its runtime-specific state directory.
- Workers remove terminal worktrees and large temporary artifacts after
  proposal export. Publishers remove branch bundles after publication.
- On Windows, a command timeout stops the complete child process tree.
- The CLI releases private run logs when a run finishes.

### Registry

- Registry refreshes tolerate temporary Git files that disappear during a
  scan.
- A refresh still stops when a bundle file or staging root is missing.

### Distribution

- Source distributions exclude compiled viewer JavaScript and CSS.
- Supported build, test, security, and release workflows generate viewer
  assets when required.

### Compatibility

- `okn connect` reads canonical Windows `file://` URLs for manifests and
  archives. Windows drive URLs use `file:///C:/path`.
- Insight creation reports stable slash-separated project paths on all
  operating systems.

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
