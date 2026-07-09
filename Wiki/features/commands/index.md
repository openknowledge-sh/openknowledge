# Commands

Each command page explains purpose, arguments or flags, use cases, and source
anchors. Keep this index as the quick map.

## Create Bundles

* [Setup](setup.md) - agent setup prompt generation.
* [New](new.md) - scaffold a local OKF bundle.
* [From](from.md) - print a source-to-wiki generation prompt for agents.

## Validate And Inspect Bundles

* [Validate](validate.md) - validate a bundle against OKF with configurable rule severities and optional JSON reports.
* [Rules](rules.md) - print and inspect canonical agent maintenance rules.
* [Review](review.md) - print advisory AI review prompts for maintenance rules.
* [Agents](agents.md) - experimental local agent job validation, planning,
  scheduling, and execution.
* [List](list.md) - print bundle inventory with optional JSON output.

## Connect And Resolve Bundles

* [Connect](connect.md) - add a local or remote OKF bundle to the local knowledge registry.
* [Disconnect](disconnect.md) - remove a connected bundle from the local registry.
* [Registry](registry.md) - manage bundle connections, listing, and path lookup.

## Use And Navigate Knowledge

* [Get](get.md) - print an exact Markdown file, entrypoint, or metadata.
* [Search](search.md) - build budget-bounded Markdown context or inspect ranked matches.
* [View](view.md) - local Markdown viewer.

## OKF Views And Publishing

* [AST](ast.md) - print the parsed OKF AST as JSON.
* [To](to.md) - conversion command group.
* [HTML exporter](/features/exporters/html.md)
* [JSON exporter](/features/exporters/json.md)
* [Tar exporter](/features/exporters/tar.md)
* [Graph exporter](/features/exporters/graph.md) - source and search graph views.

## Help And Version

* [Help](help.md) - root and command-specific help.
* [Spec](spec.md) - print embedded OKF specs.
* [Version](version.md) - print CLI version.
