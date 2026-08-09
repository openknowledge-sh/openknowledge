# Repository instructions

This repository is an educational demo. Keep claims about Wikipedia connected
to the English Wikipedia policy pages listed in `Wiki/sources/index.md`.

<!-- openknowledge:rules:start -->
## Open Knowledge Maintenance

This project has an Open Knowledge wiki at `Wiki`.

This Codex instruction block is managed by `openknowledge prompt rules apply`.

Before relevant work:
- Read `Wiki/index.md` and follow only links relevant to the task.
- Treat the wiki as durable project memory, not as a scratchpad.
- If the wiki is missing, stale, or wrong, say so instead of inventing facts.

Enabled rules:
- project: General project knowledge.
- docs: Keep docs in sync with implementation.
- decisions: Record important decisions.

Project rules:
- Before non-trivial work, read the wiki index and follow only links relevant to the task.
- After work creates durable project knowledge, update or add the matching concept pages.
- Keep the wiki structure small and shaped around the project's real workflows.

Docs rules:
- When behavior, APIs, commands, configs, or examples change, update the matching docs in the same task.
- Preserve source anchors or citations when docs depend on implementation details.
- Keep docs focused on shipped behavior; label planned work clearly.

Decisions rules:
- When a meaningful technical or product decision is made, record the context, options, chosen path, and tradeoffs.
- Link decisions to affected concepts, workflows, commands, systems, or source files.
- Do not rewrite decision history to hide old context; append clarifications or superseding decisions.

After wiki updates:
- Keep non-reserved Markdown files OKF-valid with YAML frontmatter and a non-empty `type`.
- Update `index.md` links when pages are added, moved, or removed.
- Update `log.md` when durable wiki knowledge changes.
- Run `openknowledge validate "Wiki"` before finishing.
<!-- openknowledge:rules:end -->
