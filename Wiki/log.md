# Bundle Update Log

## 2026-07-17

* **Canonical product interface**: Consolidated managed onboarding into
  `openknowledge setup [wiki]`, including source mode, validation, and project
  integration. Moved print-oriented tools under `openknowledge prompt`, renamed
  publishing to `export`, renamed the low-level `new` command to `scaffold`,
  removed duplicate registry mutation commands, and accepted the corresponding
  product-interface decision.

* **Project integrations and insights**: Added discovery-only global skills,
  project-scoped Codex/Claude/OpenCode hooks, atomic private Markdown
  insights, `agent integrate`, auto-discovered `agent insights`, direct and
  isolated local execution, knowledge-boundary/OKF verification, and an
  ordinary scheduled insights Job template without a new worker role.

* **Steered multi-harness runtime**: Generalized the local agent and scheduled
  job surfaces across Codex, Claude Code, Grok, and OpenCode, added executable
  setup/source workflows, strict runtime/model job definitions, per-harness
  credentials and workers, runtime-aware Railway deployment, official headless
  Grok Build support, and provider-configurable OpenCode support.
* **Agent and jobs split**: Added the human-facing `openknowledge agent` and
  one-shot `agent exec` flows with direct filesystem editing by default and
  opt-in retained worktree isolation. Replaced the experimental `agents`
  automation group with declarative `jobs`, renamed detached `spawn` to
  `start`, and aligned runtime roles, paths, environment variables, schemas,
  help, README, and Wiki documentation without compatibility aliases.
* **Codex executable discovery**: Made `openknowledge agent` probe and skip
  broken Codex wrappers, use supported macOS app-bundled binaries when
  available, and honor a fail-closed `OPENKNOWLEDGE_CODEX` override.

## 2026-07-15

* **CLI documentation drift audit**: Reconciled `README.md` and the command
  wiki with the shipped CLI surface. Documented registry Git selectors and
  Git staging limits, completed managed-agent usage and help coverage,
  corrected bundle metadata and exporter status wording, restored the graph
  output schema version in its example, and clarified managed remote cache
  deletion. A follow-up pass documented registry-key AST/export inputs and key
  grammar, H1-H3 search chunks, Markdown-only normalized JSON, complete AST and
  list issue shapes, agent job fields, MCP empty-file metadata, validation
  aliases, configuration loading, current development checks, and repaired
  navigation plus spec-compliance source anchors.

## 2026-07-07

* **Local agent job runner**: Added documentation for `openknowledge agents`
  as a local deterministic automation layer for Markdown-authored jobs with
  nested frontmatter, built-in job templates, Git worktree isolation, host or
  Docker execution, run records, verification commands, and daemon scheduling.
* **Custom rule catalogs and advisory review**: Documented wiki-local custom
  maintenance rules under `rules/`, deterministic `rule-catalog` validation,
  `[rules]` configuration in `openknowledge.toml`, and
  `openknowledge review rules` as the prompt-producing AI review surface
  separate from `openknowledge validate`. Classified `rules` and `review` with
  validation and inspection commands in the README, command index, root wiki
  index, and tooling model.
* **Source-to-wiki generation command**: Implemented `openknowledge from` as a
  shipped prompt-producing command for turning a source URL or path into an OKF
  Markdown bundle through a local agent. Documented `--out`, `--type`,
  `--about`, `--depth`, source provenance guidance, custom interview behavior,
  refresh expectations, and the fresh-bundle path that uses
  `openknowledge new` with `--no-agents --no-setup` when starter agent rules or
  a setup handoff are not needed.

## 2026-07-06

* **Source-to-wiki generation candidate**: Added candidate documentation for
  `openknowledge from`, including repository and website source adapters,
  `--type understanding`, `--type custom`, custom interview behavior,
  provenance metadata, refresh semantics, and the agent-prompt usage model for
  Codex, Claude Code, Cursor, Cowork, or other filesystem-capable local agents.
* **Clean navigation API**: Replaced the active CLI docs for deterministic
  reading and local viewing with `openknowledge get` and `openknowledge view`,
  removed the old command pages from navigation, and documented
  `openknowledge list --depth` plus non-Markdown asset listing.
* **Search command and graph retrieval**: Added command docs for
  `openknowledge search`, documented the removal of the previous query mode,
  and documented `openknowledge to graph --type search` as the derivative
  chunk graph layer for source-grounded retrieval.
* **Use/navigation and OKF views**: Updated the README, landing prompt, setup
  prompt, generated setup handoff, and tooling-model docs so `search` sits in
  the use/navigation layer and AST, JSON, source graph, and search graph are
  framed as different views of the same OKF bundle.

## 2026-06-22

* **Website product summary**: Added a landing page `Changelog` navigation link
  to the exported CLI changelog and tightened the final product section around
  the primary use case: a local Markdown wiki that lives with a project, gives
  agents durable context through fast search and related-context discovery, and
  stays current through a maintenance loop.

## 2026-06-20

* **CLI tooling model**: Reframed README and wiki navigation around the
  shipped layers of authoring and validation, local registry management, agent
  entrypoints, the local Markdown viewer, and export/publish. Added
  [features/tooling-model.md](features/tooling-model.md) to keep the product
  map explicit while marking GitHub or published-page remote materialization as
  planned registry-layer work.
* **Deployed wiki brand title**: Set `Wiki/index.md`
  `okf_bundle_title` to `Open Knowledge CLI Documentation` so the exported
  viewer header brand uses the documentation title, and clarified that
  `[html.source].entry` remains the repository path prefix for GitHub source
  links rather than a display title.
* **OKF, skills, and plugins**: Added
  [features/okf-skills-plugins.md](features/okf-skills-plugins.md) as a
  user-facing one-page comparison of raw OKF v0.1, agent skills, and plugins.
* **Mobile Safari viewer chrome**: Documented that the shared viewer uses
  small viewport sizing where supported, hides fixed bottom chrome on mobile or
  touch viewports, and keeps fenced shell code at body-sized text without extra
  command weight.
* **Static viewer brand link**: Documented that default
  `openknowledge to html` viewer exports link the header brand back to the
  generated `index.html` with a relative URL so `/wiki/` deployments stay
  inside the exported wiki.
* **Website release badge**: Documented that the landing page header links to
  the latest GitHub Release and hydrates its tag plus relative publish age from
  GitHub's latest release API at runtime.
* **Viewer panel reading width**: Set the default note panel width to a `65ch`
  reading measure plus horizontal panel padding in the built-in viewer theme,
  deployed wiki theme override, and resize fallback.
* **Viewer table filter trigger**: Documented that Markdown table filter
  dropdowns use a ghost trigger button in the shared viewer CSS for
  `openknowledge open` and default `openknowledge to html` exports.
* **Viewer table code wrapping**: Documented that inline code values in
  Markdown tables wrap inside cells in `openknowledge open` and default
  `openknowledge to html` viewer exports so evidence-heavy tables do not
  overflow their visual frame.
* **Spec compliance matrix**: Added
  [features/spec-compliance.md](features/spec-compliance.md) with a
  hard-rule OKF v0.1 compliance table that maps mandatory spec rules to CLI
  behavior, status emojis, source anchors, and focused tests where they exist.
* **Syntax highlighting examples**: Added
  [examples/syntax-highlighting.md](examples/syntax-highlighting.md) with
  fenced shell, Go, TypeScript, Python, JSON, YAML, CSS, and SQL blocks for
  manually checking viewer token colors.
* **Command change history rule**: Added a wiki maintenance rule requiring
  command pages to capture dated history entries for major command-surface
  changes such as added, removed, renamed, or behavior-changing flags,
  properties, output fields, and exit-code semantics.
* **Website wiki export**: Documented that `pnpm build:web` publishes the
  colocated `Wiki/` bundle to `packages/web/dist/wiki` with
  `openknowledge to html`, and added the landing-matched
  `Wiki/assets/openknowledge-site.css` theme configuration. The web dev server
  now falls back to that generated `dist/wiki` output for `/wiki/` URLs.
* **Static export path hygiene**: Documented that default viewer HTML exports
  leave `data-note-root` empty so public static sites do not expose the local
  build machine's bundle path.
* **Local viewer theme parity**: Documented that `openknowledge open` applies
  `[html.theme]` to listing, file, asset preview, and alias-prefixed pages, and
  validates local theme CSS paths before rendering.
* **Mobile viewer header**: Documented the responsive header search fix that
  lets the shared viewer app CSS shrink the search field on narrow mobile
  widths instead of overlapping the knowledge base brand.
* **Mobile sidebar bottom chrome**: Documented that the shared viewer app CSS
  hides the fixed bottom rail and `Powered by OpenKnowledge.sh` attribution
  while the mobile sidebar is open so those elements do not overflow into the
  drawer.
* **Mobile sidebar file selection**: Documented that selecting a file from the
  shared viewer sidebar closes the drawer on mobile while preserving the desktop
  behavior where the sidebar stays open.
* **Static viewer pretty URLs**: Documented that default viewer HTML exports
  resolve host-rewritten pretty URLs such as `/agents` and `/features/` back to
  the embedded static note manifest so stacked-panel navigation keeps working
  on static hosts.
* **CLI operations migration**: Moved the remaining development and release
  notes from `docs/cli.md` into [features/operations.md](features/operations.md),
  and made the wiki the canonical home for CLI operational docs.

## 2026-06-19

* **Viewer examples**: Added `examples/` with a small Markdown page, Go source
  file, and PDF asset for manually testing code highlighting and browser PDF
  viewing in `openknowledge open Wiki`.
* **Viewer asset links and syntax highlighting**: Documented that
  `openknowledge open` highlights fenced code and code/text asset previews, and
  serves PDF/media references through raw bundle URLs for browser-native
  viewing.
* **Connected bundle command candidates**: Added candidate specifications for
  `openknowledge connect` and `openknowledge disconnect`, and aligned
  `list`, `where`, `registry`, and `use` docs around a path-keyed local
  knowledge registry for agent discovery.
* **New bundle metadata flags**: Updated `openknowledge new` documentation for
  optional `okf_bundle_*` root metadata flags and recorded that root validation
  accepts this Open Knowledge CLI layer while plain OKF bundles remain valid.
* **Use command candidate**: Recorded the candidate `openknowledge use`
  contract for optional `okf_bundle_*` root metadata, entrypoint frontmatter
  with `use_when`, `--info`, and fallback behavior for plain OKF bundles.
* **Repo skill subagents**: Updated the active repo-local wiki skill and agent
  rules to prefer focused lower-reasoning subagents for bounded wiki maintenance
  tasks when the runtime supports them.
* **Command docs audit**: Compared wiki command and exporter pages against the
  current CLI implementation and tightened docs for help aliases, scaffold
  defaults, registry storage, converter flags, JSON behavior, HTML viewer versus
  plain export modes, and installer environment variables.
* **Feature docs workflow**: Added a command documentation pattern based on
  current React, TanStack Query, Next.js, Vite, pnpm, npm, and GitHub CLI docs
  research.

## 2026-06-18

* **Setup**: Customized this bundle as the Open Knowledge CLI developer wiki.
* **Structure**: Added feature documentation, exporter pages, command pages, CLI changelog memory, workflows, and setup decisions.
* **Agent rules**: Replaced starter rules with maintenance rules for CLI feature docs and changelog updates.
* **Repo skill**: Added `.codex/skills/openknowledge-wiki/SKILL.md` and root `AGENTS.md` so future agents know when to maintain the wiki.
* **Initialization**: Created the Open Knowledge bundle scaffold.
* **Reference**: Stored a local pinned OKF spec copy in [SPEC.md](SPEC.md).
