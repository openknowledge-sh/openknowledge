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

## Commands

### Create And Inspect Bundles

* [setup](features/commands/setup.md) - generate setup instructions for agents.
* [new](features/commands/new.md) - scaffold a local OKF bundle.

### Validate Bundles

* [validate](features/commands/validate.md) - validate a bundle against OKF.

### Connect And Resolve Bundles

* [connect](features/commands/connect.md) - add a local or remote bundle to the
  local registry.
* [disconnect](features/commands/disconnect.md) - remove a registered bundle.
* [registry](features/commands/registry.md) - list, connect, disconnect, and
  resolve registry entries.

### Use And Browse Knowledge

* [list](features/commands/list.md) - print bundle inventory.
* [use](features/commands/use.md) - print an entrypoint, bundle file, metadata,
  or query excerpts.
* [open](features/commands/open.md) - browse a bundle in the local Markdown
  viewer.

### Export And Inspect

* [ast](features/commands/ast.md) - print the parsed OKF AST as JSON.
* [to](features/commands/to.md) - export a bundle to HTML, JSON, or tar.
* [HTML exporter](features/exporters/html.md) - default static viewer export
  and plain semantic HTML mode.
* [JSON exporter](features/exporters/json.md) - normalized bundle model.
* [Tar exporter](features/exporters/tar.md) - portable source bundle archive.

### Help And Version

* [help](features/commands/help.md) - inspect root and command-specific help.
* [spec](features/commands/spec.md) - print embedded OKF specs.
* [version](features/commands/version.md) - print the CLI version.

## Further Reading

* [Tooling model](features/tooling-model.md) - product-level map of authoring,
  registry, entrypoint, viewer, and export layers.
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
