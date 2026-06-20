# Bundle Update Log

## 2026-06-20

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
