---
okf_version: "0.1"
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
- [Command reference](features/commands/)
- [`.openknowledge.toml`](features/configuration.md)
- [CLI changelog](changelog/cli.md)

## Workflows

### Create, retrieve, and verify

Use [`setup`](features/commands/setup.md) to print an open-ended setup prompt.
Copy the output into an agent that already works in the project. Alternatively,
use `okn setup --agent --runtime <runtime>` to launch one. Use
[`search`](features/commands/search.md) to retrieve knowledge. Use
[`validate`](features/commands/validate.md) to verify the wiki.

### Work locally

Use [`agent`](features/commands/agent.md) to run an agent task. Use
[`setup`](features/commands/setup.md) to install skills and harness adapters.
Use [`get`](features/commands/get.md) to read one document. Use
[`list`](features/commands/list.md) to inspect the content tree. Use
[`view`](features/commands/view.md) to browse the wiki.

### Share and connect

Use [`mcp`](features/commands/mcp.md) to serve MCP tools. Use
[`export`](features/commands/export.md) to publish portable output.
Use [`connect`](features/commands/connect.md) to add a source. Use
[`disconnect`](features/commands/disconnect.md) to remove a source. Use
[`registry`](features/commands/registry.md) to inspect the registry.

### Automate and operate

Use [`automation`](features/commands/automation.md) for jobs, insights,
runtime services, and deployments.

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
