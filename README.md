<p align="center">
  <img src="docs/assets/openknowledge-readme-banner-wide.png" alt="Open Knowledge CLI banner" width="1160" height="125">
</p>

# Open Knowledge

Open Knowledge creates and maintains Git-native knowledge bases in plain
Markdown. It can search, validate, connect, and publish the same knowledge.

[Website](https://openknowledge.sh) |
[Documentation](Wiki/index.md) |
[Commands](Wiki/features/commands/index.md) |
[Changelog](Wiki/changelog/cli.md) |
[OKF specification][okf-spec] |
[License](LICENSE)

<p align="left">
  <a href="LICENSE"><img alt="License: Apache-2.0" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
  <a href="https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md"><img alt="OKF v0.1" src="https://img.shields.io/badge/OKF-v0.1-2f6feb"></a>
</p>

## Get started

Install the CLI:

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

Print portable instructions for a project wiki:

```sh
okn setup
```

`okn setup` uses the current project as its source. It uses `Wiki` as the
default target. It prints instructions and does not start an agent.

Run the instructions with an installed agent runtime:

```sh
okn setup --agent
```

The command detects Codex, Claude Code, and OpenCode. It asks you to select an
available runtime. For non-interactive use, select the runtime:

```sh
okn setup --agent --runtime codex
```

If setup cannot start a runtime, inspect the installation:

```sh
okn agent doctor --runtime <runtime>
```

The setup workflow creates the wiki, validates it, and installs project
integration.

You can also install the CLI from npm:

```sh
npm install -g @openknowledge-sh/openknowledge
```

Both installers provide `okn` and `openknowledge`. This documentation uses
`okn`. See [Installation](Wiki/features/installation.md) for supported
platforms and verification details.

## Use the knowledge base

Search for source-grounded context:

```sh
okn search Wiki "release workflow"
```

Validate the knowledge base:

```sh
okn validate Wiki
```

Browse it locally:

```sh
okn view Wiki
```

Use `okn <command> --help` for exact command syntax.

## What Open Knowledge does

### Create and maintain

- [`setup`](Wiki/features/commands/setup.md) prints or runs wiki setup
  instructions.
- [`agent`](Wiki/features/commands/agent.md) runs a supported agent harness.
- [`insights`](Wiki/features/commands/insights.md) captures and resolves
  knowledge gaps.
- [`jobs`](Wiki/features/commands/jobs.md) runs experimental maintenance jobs.

### Retrieve and verify

- [`search`](Wiki/features/commands/search.md) builds source-grounded context.
- [`get`](Wiki/features/commands/get.md) reads an exact document.
- [`list`](Wiki/features/commands/list.md) inspects the content tree.
- [`validate`](Wiki/features/commands/validate.md) checks OKF conformance.

### Browse and publish

- [`view`](Wiki/features/commands/view.md) starts the local viewer.
- [`mcp`](Wiki/features/commands/mcp.md) serves read-only MCP tools.
- [`export`](Wiki/features/commands/export.md) creates HTML, JSON, graph, or tar
  output.

### Connect and operate

- [`connect`](Wiki/features/commands/connect.md) adds a local or remote source.
- [`registry`](Wiki/features/commands/registry.md) inspects connected sources.
- [`runtime`](Wiki/features/commands/runtime.md) runs isolated services.
- [`deploy`](Wiki/features/commands/deploy.md) provisions a runtime on Railway.

See the [command index](Wiki/features/commands/index.md) for all commands.

## Publish a wiki

`okn export html` creates a public distribution bundle in a local folder.
It does not deploy the bundle or start an MCP server.

Review the knowledge base before you create this bundle. After you enable
publication, Markdown is public unless it sets `okf_publish: false`. Add this
configuration to `Wiki/openknowledge.toml`:

```toml
[publish]
enabled = true
```

Export the reviewed content:

```sh
okn export html --out ./site Wiki
```

The bundle includes a static viewer, `llms.txt`, a connect manifest, and a
portable source archive. A deployed runtime can also expose filtered search
and HTTP MCP projections. See [HTML export](Wiki/features/exporters/html.md)
and [`openknowledge.toml`](Wiki/features/configuration.md) for publication
filters and asset options.

## Documentation

- [Tooling model](Wiki/features/tooling-model.md)
- [Command reference](Wiki/features/commands/index.md)
- [Export formats](Wiki/features/exporters/index.md)
- [Machine-readable contracts](Wiki/features/machine-contracts.md)
- [Go API](Wiki/features/go-api.md)
- [Runtime operations](Wiki/features/commands/runtime.md)
- [Railway deployment](Wiki/features/commands/deploy.md)
- [CLI changelog](Wiki/changelog/cli.md)

The wiki describes current behavior. The changelog records release history.

## Development

```sh
pnpm install --frozen-lockfile
pnpm test
pnpm build
```

See [CLI operations](Wiki/features/operations.md) for focused test, build,
website, and release commands.

## License and attribution

Open Knowledge is licensed under Apache-2.0.

The embedded OKF specification is Apache-2.0 material from
`GoogleCloudPlatform/knowledge-catalog`. See
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md) for attribution.

[okf-spec]: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
