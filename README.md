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
default target. Run it yourself, then copy the complete printed task into the
agent that already works in your project.

When the agent finishes, validate the wiki:

```sh
okn validate Wiki
```

Optionally install a project skill for that agent runtime:

```sh
okn integration install Wiki --runtime codex
```

This installs only the selected runtime. Session observation remains off unless
you explicitly add `--observe`.

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

The [command index](Wiki/features/commands/index.md) separates local work from
sharing and automation. Use `okn automation` for jobs, insights, hosted
runtimes, and deployments.

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
