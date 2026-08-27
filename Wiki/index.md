---
okf_version: "0.2"
okf_bundle_title: "Open Knowledge CLI Documentation"
---

# Open Knowledge CLI

Flexible knowledge bases in Markdown that your agents can create, retrieve,
maintain, validate, and publish.

## Get started

```sh
curl -fsSL https://openknowledge.sh/install | bash
okn setup
```

This documentation uses `okn` as the preferred command. The full
`openknowledge` command remains available.

The published documentation is at
[openknowledge.sh/wiki](https://openknowledge.sh/wiki/). It is the exported
version of this OKF wiki.

- [Installation](features/installation.md)
- [Product telemetry and privacy](features/telemetry.md)
- [Command reference](features/commands/)
- [`.openknowledge.toml`](features/configuration.md)
- [CLI changelog](changelog/cli.md)

## Workflows

### Start here

Use [`setup`](features/commands/setup.md) to create a knowledge base. Use
[`validate`](features/commands/validate.md) to verify it. Use
[`search`](features/commands/search.md) to retrieve knowledge. Use
[`view`](features/commands/view.md) to browse the knowledge base.

### Trust and govern

Use the `trusted` intent when you need source, evidence, lifecycle,
and conflict controls. Use [`audit`](features/commands/audit.md),
[`claims`](features/commands/claims.md), and
[`evidence`](features/commands/evidence.md) to apply these controls. Use
[`eval`](features/commands/eval.md) and
[`quality`](features/commands/quality.md) to measure results.

### Query and interchange

Use [`query`](features/commands/query.md) for explicit semantic queries. Use
[`export`](features/commands/export.md) to create portable output.

### Publish and operate

Use [`mcp`](features/commands/mcp.md) to serve MCP tools. Use
[`connect`](features/commands/connect.md) and
[`registry`](features/commands/registry.md) to manage sources. Use
[`automation`](features/commands/automation.md) for managed processes.

Knowledge CI and the production runtime are optional. See the
[Knowledge CI Golden Path](features/golden-path.md) when you need these
capabilities.

### Advanced internals

Use [`agent`](features/commands/agent.md) to run an agent task. Use
[`get`](features/commands/get.md) and [`list`](features/commands/list.md) for
direct inspection. Other tools include
[`scaffold`](features/commands/scaffold.md),
[`prompt`](features/commands/prompt.md),
[`ast`](features/commands/ast.md), and [`spec`](features/commands/spec.md).

## Reference

- [Tooling model](features/tooling-model.md)
- [Export formats](features/exporters/)
- [Machine-readable contracts](features/machine-contracts.md)
- [Typed Claims v1](features/claim-profile.md)
- [Go API](features/go-api.md)
- [Maintenance rules](rules/)
- [OKF v0.2 specification](SPEC.md)
