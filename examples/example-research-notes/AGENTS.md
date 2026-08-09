# Repository instructions

This repository is an educational demo. Keep raw source notes separate from
research synthesis. Preserve external links for source-dependent claims.

<!-- openknowledge:rules:start -->
## Open Knowledge Maintenance

This project has an Open Knowledge wiki at `Wiki`.

This Codex instruction block is managed by `openknowledge prompt rules apply`.

Before relevant work:
- Read `Wiki/index.md` and follow only links relevant to the task.
- Treat the wiki as durable project memory, not as a scratchpad.
- If the wiki is missing, stale, or wrong, say so instead of inventing facts.

Enabled rules:
- research: Import research with citations.

Research rules:
- Keep raw sources separate from synthesized wiki pages.
- Preserve source links, file paths, quotes, or citations for claims that depend on external material.
- Do not turn uncertain or unsupported research into asserted project knowledge.

After wiki updates:
- Keep non-reserved Markdown files OKF-valid with YAML frontmatter and a non-empty `type`.
- Update `index.md` links when pages are added, moved, or removed.
- Update `log.md` when durable wiki knowledge changes.
- Run `openknowledge validate "Wiki"` before finishing.
<!-- openknowledge:rules:end -->
