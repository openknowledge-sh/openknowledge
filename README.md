<p align="center">
  <img src="docs/assets/openknowledge-readme-banner-wide.png" alt="Open Knowledge CLI banner" width="1160" height="125">
</p>

# Open Knowledge

Flexible knowledge bases in Markdown that your agents can create, retrieve,
validate, and publish.

[🌐 Website](https://openknowledge.sh) |
[📖 Documentation](Wiki/index.md) |
[🧭 Commands](Wiki/features/commands/index.md) |
[📝 Changelog](Wiki/changelog/cli.md) |
[📐 OKF specification][okf-spec] |
[⚖️ License](LICENSE)

<p align="left">
  <a href="LICENSE"><img alt="License: Apache-2.0" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
  <a href="https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md"><img alt="OKF v0.1" src="https://img.shields.io/badge/OKF-v0.1-2f6feb"></a>
  <a href="https://openknowledge.sh"><img alt="Git-native Markdown wiki" src="https://img.shields.io/badge/wiki-git--native-0f766e"></a>
  <a href="Wiki/index.md"><img alt="Agent-ready docs" src="https://img.shields.io/badge/docs-agent--ready-6f42c1"></a>
</p>

## From project to knowledge base

```text
Your project
    │
    ▼
 okn setup ──► your agent creates Wiki/
                         │
                         ▼
                  okn validate Wiki
                         │
             ┌───────────┼───────────┐
             ▼           ▼           ▼
          search        view      export html
```

## Quick start

### 1. Install

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

### 2. Create

```sh
okn setup
```

`okn setup` uses the current project as its source and `Wiki` as the default
target. Run it, then copy the complete printed prompt into the agent that
already works in your project.

### 3. Validate

When the agent finishes:

```sh
okn validate Wiki
```

<details>
<summary>Alternative npm installation</summary>

```sh
npm install -g @openknowledge-sh/openknowledge
```

Both installers provide `okn` and `openknowledge`. This documentation uses
`okn`. See [Installation](Wiki/features/installation.md) for supported
platforms and verification details.

</details>

<details>
<summary>Optional agent integration</summary>

```sh
okn integration install Wiki --runtime codex
```

This optional command installs only the selected runtime. Session observation
remains off unless you explicitly add `--observe`.

</details>

## Work with a knowledge base

| Task | Command | Result |
| --- | --- | --- |
| Search | `okn search Wiki "release workflow"` | Build source-grounded context |
| Validate | `okn validate Wiki` | Check the knowledge base |
| Browse | `okn view Wiki` | Start the local viewer |

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
