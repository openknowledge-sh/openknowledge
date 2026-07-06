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

No uncategorized changes are currently tracked here. Next-release work is
classified under [v0.5.0 Candidate](#v050-candidate) until a release tag is
cut.

## v0.5.0 Candidate

Candidate V5 changes are commits after the `v0.4.0` release tag. Keep new
entries in this section until they are released.

### 2026-07-06 - Search command and search graph exports

* Added `openknowledge search <name-or-path> <query>` as the first-class query
  retrieval command for Open Knowledge bundles. It builds source-grounded
  Markdown heading chunks, scores them with BM25-style lexical ranking across
  metadata, heading paths, paths, and body text, and prints text or JSON
  results with snippets, source line ranges, scores, and matched fields.
* Added `openknowledge search --expand graph` to include lower-ranked outgoing
  local-link and backlink neighbor chunks in search results.
* Removed `openknowledge use --query`; `use` now stays focused on deterministic
  entrypoint and bundle-file loading. Removed query flags fail with a migration
  hint to `openknowledge search <bundle> <query>`.
* Added `openknowledge to graph --type source|search`. `source` is the default
  file/link graph. `search` exports a derivative chunk graph with source file
  nodes, heading chunk nodes, containment edges, reading-order edges, and
  chunk-level local-link edges.
* Updated the setup prompt, generated `SETUP.MD`, README setup prompt, and
  landing page prompt to leave users with the use/navigation loop:
  `openknowledge list`, `openknowledge search`, and `openknowledge open`.
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
  `Wiki/features/commands/use.md`, `Wiki/features/commands/setup.md`,
  `Wiki/features/commands/open.md`, `Wiki/features/commands/to.md`,
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
* The active note panel now shows a minimal formatted close shortcut hint to the
  left of the close button, and closing a panel moves focus to the previous
  panel when one exists.
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
