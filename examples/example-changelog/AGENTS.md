# Repository instructions

This repository is an educational demo. Keep current behavior in `Wiki/features/`
and release history in `Wiki/changelog/`.

<!-- openknowledge:rules:start -->
## Open Knowledge Maintenance

This project has an Open Knowledge wiki at `Wiki`.

This Codex instruction block is managed by `openknowledge prompt rules apply`.

Before relevant work:
- Read `Wiki/index.md` and follow only links relevant to the task.
- Treat the wiki as durable project memory, not as a scratchpad.
- If the wiki is missing, stale, or wrong, say so instead of inventing facts.

Enabled rules:
- docs: Keep docs in sync with implementation.
- changelog: Track user-facing changes.

Docs rules:
- When behavior, APIs, commands, configs, or examples change, update the matching docs in the same task.
- Preserve source anchors or citations when docs depend on implementation details.
- Keep docs focused on shipped behavior; label planned work clearly.

Changelog rules:
- When user-facing behavior, command flags, output, validation, publishing, packaging, or setup changes, update changelog memory.
- Include what changed, why it matters, source anchors, and docs updated.
- Skip changelog entries for formatting-only edits or internal cleanup with no user-visible effect.

After wiki updates:
- Keep non-reserved Markdown files OKF-valid with YAML frontmatter and a non-empty `type`.
- Update `index.md` links when pages are added, moved, or removed.
- Update `log.md` when durable wiki knowledge changes.
- Run `openknowledge validate "Wiki"` before finishing.
<!-- openknowledge:rules:end -->
