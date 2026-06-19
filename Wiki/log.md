# Bundle Update Log

## 2026-06-19

* **Viewer examples**: Added `examples/` with a small Markdown page, Go source
  file, and PDF asset for manually testing code highlighting and browser PDF
  viewing in `openknowledge open Wiki`.
* **Viewer asset links and syntax highlighting**: Documented that
  `openknowledge open` highlights fenced code and code/text asset previews, and
  serves PDF/media references through raw bundle URLs for browser-native
  viewing.
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
