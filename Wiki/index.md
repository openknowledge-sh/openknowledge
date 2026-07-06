---
okf_version: "0.1"
okf_bundle_title: "Open Knowledge CLI Documentation"
---

# Open Knowledge CLI Documentation

This wiki documents the Open Knowledge CLI: how to install it, run commands,
publish bundles, and maintain CLI-facing documentation.

The deployed docs living on [https://openknowledge.sh/wiki/](https://openknowledge.sh/wiki/)
are an exported view of this OKF bundle using `openknowledge to html` and a
custom theme.

## Start Here

* [Installation](features/installation.md) - install the CLI through the shell
  installer, npm wrapper, or local development flow.
* [Commands](features/commands/) - command-by-command reference.
* [Exporters](features/exporters/) - HTML, JSON, tar, and graph exporter
  behavior.

## Changelog

* [CLI changelog](changelog/cli.md) - release-facing CLI changes.

## Commands

### Create Bundles

* [setup](features/commands/setup.md) - generate setup instructions for agents.
* [from](features/commands/from.md) - generate source-to-wiki instructions for
  agents.
* [new](features/commands/new.md) - scaffold a local OKF bundle.
* [agents](features/commands/agents.md) - create, validate, plan, schedule,
  and run local agent jobs.

### Validate And Inspect Bundles

* [validate](features/commands/validate.md) - validate a bundle against OKF.
* [list](features/commands/list.md) - inspect bundle inventory with inline
  validation context.

### Connect And Resolve Bundles

* [connect](features/commands/connect.md) - add a local or remote bundle to the
  local registry.
* [disconnect](features/commands/disconnect.md) - remove a registered bundle.
* [registry](features/commands/registry.md) - list, connect, disconnect, and
  resolve registry entries.

### Use And Navigate Knowledge

* [get](features/commands/get.md) - print an exact Markdown file, entrypoint,
  or metadata.
* [search](features/commands/search.md) - search source-grounded Markdown
  chunks with optional graph expansion.
* [view](features/commands/view.md) - browse a bundle in the local Markdown
  viewer.

### OKF Views And Publishing

* [ast](features/commands/ast.md) - print the parsed OKF AST as JSON.
* [to](features/commands/to.md) - export a bundle to HTML, JSON, tar, or graph.
* [HTML exporter](features/exporters/html.md) - default static viewer export
  and plain semantic HTML mode.
* [JSON exporter](features/exporters/json.md) - normalized bundle model.
* [Tar exporter](features/exporters/tar.md) - portable source bundle archive.
* [Graph exporter](features/exporters/graph.md) - source and search graph views
  of the same OKF bundle.

### Help And Version

* [help](features/commands/help.md) - inspect root and command-specific help.
* [spec](features/commands/spec.md) - print embedded OKF specs.
* [version](features/commands/version.md) - print the CLI version.

## Further Reading

* [Tooling model](features/tooling-model.md) - product-level map of authoring,
  connection, validation, use/navigation, OKF views, and publishing layers.
* [CLI operations](features/operations.md) - development commands, workspace
  layout, website export, deployment, and release notes.
* [OKF, skills, and plugins](features/okf-skills-plugins.md) - comparison of
  raw OKF bundles, agent skills, and plugins.
* [Spec compliance](features/spec-compliance.md) - CLI compliance matrix for
  the embedded OKF spec.

---

<!-- okf-footer: agent-maintenance -->

> **Agent maintenance**
>
> Use these pages when an agent is updating, validating, or extending this wiki.
>
> * [Agent rules](AGENTS.md) - when future agents should read and update this wiki.
> * [Workflows](workflows/) - repeatable update loops for docs and changelog maintenance.
> * [Feature docs workflow](workflows/feature-docs.md) - update command, exporter, setup, viewer, and README-facing docs.
> * [Changelog update workflow](workflows/changelog-updates.md) - update CLI changelog memory after release-facing changes.
> * [CLI changelog](changelog/cli.md) - maintained memory of CLI-facing changes.
> * [Examples](examples/) - viewer smoke-test files, including syntax highlighting, code, and PDF assets.
> * [Spec](SPEC.md) - local pinned copy of the Open Knowledge Format spec.
> * [Spec compliance](features/spec-compliance.md) - CLI compliance matrix for the embedded OKF spec.
> * [Log](log.md) - chronological update history.
> * [Decisions](decisions/) - setup and structure decisions for the wiki.
>
> **Source boundaries**
>
> The wiki summarizes repository facts. Use source files, tests, README content,
> and release notes as the source of truth. Keep raw copied material out of the
> wiki unless a future workflow explicitly needs a raw source area.
