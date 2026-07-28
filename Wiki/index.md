---
okf_version: "0.1"
okf_bundle_title: "Open Knowledge CLI Documentation"
---

# Open Knowledge CLI

Open Knowledge creates, searches, publishes, and maintains Git-native knowledge
bases in Open Knowledge Format (OKF).

## Get started

```sh
curl -fsSL https://openknowledge.sh/install | bash
openknowledge setup
```

- [Installation](features/installation.md)
- [Command reference](features/commands/)
- [`openknowledge.toml`](features/configuration.md)
- [CLI changelog](changelog/cli.md)

## Workflows

| Goal | Start here |
| --- | --- |
| Create, retrieve, and verify | [`setup`](features/commands/setup.md), [`search`](features/commands/search.md), [`validate`](features/commands/validate.md) |
| Maintain and automate | [`agent`](features/commands/agent.md), [`insights`](features/commands/insights.md), [`jobs`](features/commands/jobs.md) |
| Browse and publish | [`get`](features/commands/get.md), [`list`](features/commands/list.md), [`view`](features/commands/view.md), [`mcp`](features/commands/mcp.md), [`export`](features/commands/export.md) |
| Connect and operate | [`connect`](features/commands/connect.md), [`disconnect`](features/commands/disconnect.md), [`registry`](features/commands/registry.md), [`runtime`](features/commands/runtime.md), [`deploy`](features/commands/deploy.md) |

Advanced tools include [`scaffold`](features/commands/scaffold.md),
[`prompt`](features/commands/prompt.md), [`ast`](features/commands/ast.md), and
[`spec`](features/commands/spec.md).

## Reference

- [Tooling model](features/tooling-model.md)
- [Export formats](features/exporters/)
- [Machine-readable contracts](features/machine-contracts.md)
- [Go API](features/go-api.md)
- [OKF v0.1 specification](SPEC.md)
