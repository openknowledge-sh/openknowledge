# Commands

Each command page explains purpose, arguments or flags, use cases, and source
anchors. Keep this index as the quick map.

## Create Bundles

* [Setup](setup.md) - managed agent onboarding, validation, and integration.
* [Scaffold](scaffold.md) - scaffold a local OKF bundle.

## Validate And Inspect Bundles

* [Validate](validate.md) - validate a bundle against OKF with configurable rule severities and optional JSON reports.
* [Prompt](prompt.md) - advanced portable setup, source, rules, and review prompts.
* [Agent](agent.md) - experimental steered Codex, Claude Code, Grok, or OpenCode
  sessions, project integration, insight execution, and opt-in worktree isolation.
* [Agent integrate](integrate.md) - install global discovery skills or project-scoped
  skills and observation hooks.
* [Agent insights](insights.md) - capture, review, and execute private
  evidence-backed knowledge observations locally or through Jobs.
* [Jobs](jobs.md) - experimental declarative scheduling, isolated execution,
  observation, and control.
* [Runtime](runtime.md) - plan, build, serve, and reconcile self-hosted
  immutable knowledge-base generations.
* [Deploy](deploy.md) - validate and provision the isolated runtime on Railway
  with provider-generated, custom, or no-public endpoint modes.
* [List](list.md) - print bundle inventory with optional JSON output.

## Connect And Resolve Bundles

* [Connect](connect.md) - add a local or remote OKF bundle to the local knowledge registry.
* [Disconnect](disconnect.md) - remove a connected bundle from the local registry.
* [Registry](registry.md) - refreshes, offline integrity checks, listing, and path lookup.

## Use And Navigate Knowledge

* [Get](get.md) - print an exact Markdown file, entrypoint, or metadata.
* [Search](search.md) - build budget-bounded Markdown context or inspect ranked matches.
* [MCP](mcp.md) - serve one bundle as read-only MCP resources and tools over
  stdio or expose the same surface through the public runtime's HTTP endpoint.
* [View](view.md) - local Markdown viewer.

## OKF Views And Publishing

* [AST](ast.md) - print the parsed OKF AST as JSON.
* [Export](export.md) - HTML, JSON, tar, and graph output command group.
* [HTML exporter](/features/exporters/html.md)
* [JSON exporter](/features/exporters/json.md)
* [Tar exporter](/features/exporters/tar.md)
* [Graph exporter](/features/exporters/graph.md) - source and search graph views.

## Help And Version

* [Help](help.md) - root and command-specific help.
* [Spec](spec.md) - print embedded OKF specs.
* [Version](version.md) - print CLI version.
