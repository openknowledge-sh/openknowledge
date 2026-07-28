---
okf_version: "0.1"
okf_bundle_title: "Open Knowledge CLI Documentation"
---

# Open Knowledge CLI

Open Knowledge creates and maintains Git-native knowledge bases in Open
Knowledge Format (OKF). It also searches and publishes these knowledge bases.

## Get started

```sh
curl -fsSL https://openknowledge.sh/install | bash
okn setup
```

This documentation uses `okn` as the preferred command. The full
`openknowledge` command remains available.

- [Installation](features/installation.md)
- [Command reference](features/commands/)
- [`openknowledge.toml`](features/configuration.md)
- [CLI changelog](changelog/cli.md)

## Workflows

### Create, retrieve, and verify

Use [`setup`](features/commands/setup.md) to print portable wiki instructions.
Copy the complete output into an agent that already works in the project. Use
[`search`](features/commands/search.md) to retrieve knowledge. Use
[`validate`](features/commands/validate.md) to verify the wiki.

### Maintain and automate

Use [`agent`](features/commands/agent.md) to run an agent task. Use
[`integration`](features/commands/integration.md) to install one runtime skill. Use
[`insights`](features/commands/insights.md) to manage insight reviews. Use
[`jobs`](features/commands/jobs.md) to schedule maintenance jobs.

### Browse and publish

Use [`get`](features/commands/get.md) to read one document. Use
[`list`](features/commands/list.md) to inspect the content tree. Use
[`view`](features/commands/view.md) to browse the wiki.

Use [`mcp`](features/commands/mcp.md) to serve MCP tools. Use
[`export`](features/commands/export.md) to publish portable output.

### Connect and operate

Use [`connect`](features/commands/connect.md) to add a source. Use
[`disconnect`](features/commands/disconnect.md) to remove a source. Use
[`registry`](features/commands/registry.md) to inspect the registry.

Use [`runtime`](features/commands/runtime.md) to run services. Use
[`deploy`](features/commands/deploy.md) to deploy them.

### Use advanced tools

Use [`scaffold`](features/commands/scaffold.md) for deterministic bundle
creation. Other advanced tools are [`prompt`](features/commands/prompt.md),
[`ast`](features/commands/ast.md), and [`spec`](features/commands/spec.md).

## Reference

- [Tooling model](features/tooling-model.md)
- [Export formats](features/exporters/)
- [Machine-readable contracts](features/machine-contracts.md)
- [Go API](features/go-api.md)
- [Maintenance rules](rules/)
- [OKF v0.1 specification](SPEC.md)
