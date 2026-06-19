# Bundle Update Log

## 2026-06-19

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
