---
okf_version: "0.1"
---

# Wiki

This Open Knowledge bundle is the developer wiki for the Open Knowledge CLI.
It records CLI feature documentation, exporter behavior, and a changelog loop
for package changes that affect the command-line tool.

## Human-Oriented CLI Docs

Use these pages to install, run, inspect, and operate the Open Knowledge CLI.

* [Feature documentation](features/) - developer-focused pages for commands, exporters, and installation.
* [Installation](features/installation.md) - shell installer, npm wrapper, and local install assumptions.
* [CLI operations](features/operations.md) - development commands, workspace layout, and release workflow notes.
* [Commands](features/commands/) - command-by-command reference pages.
* [Exporters](features/exporters/) - bundle conversion targets and candidate exporters.

## Agent Maintenance

Use these pages when an agent is updating, validating, or extending this wiki.

* [Agent rules](AGENTS.md) - when future agents should read and update this wiki.
* [Workflows](workflows/) - repeatable update loops for docs and changelog maintenance.
* [Feature docs workflow](workflows/feature-docs.md) - update command, exporter, setup, viewer, and README-facing docs.
* [Changelog update workflow](workflows/changelog-updates.md) - update CLI changelog memory after release-facing changes.
* [CLI changelog](changelog/cli.md) - maintained memory of CLI-facing changes.
* [Examples](examples/) - viewer smoke-test files, including syntax highlighting, code, and PDF assets.
* [Spec](SPEC.md) - local pinned copy of the Open Knowledge Format spec.
* [Log](log.md) - chronological update history.
* [Decisions](decisions/) - setup and structure decisions for the wiki.

## Source Boundaries

The wiki summarizes repository facts. Use source files, tests, README content,
and release notes as the source of truth. Keep raw copied material out of the
wiki unless a future workflow explicitly needs a raw source area.
