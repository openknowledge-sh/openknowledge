# Bundle Update Log

## 2026-06-20

* **Command change history rule**: Added a wiki maintenance rule requiring
  command pages to capture dated history entries for major command-surface
  changes such as added, removed, renamed, or behavior-changing flags,
  properties, output fields, and exit-code semantics.
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
