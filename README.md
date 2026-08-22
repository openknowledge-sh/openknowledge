<p align="center">
  <img src="docs/assets/openknowledge-readme-banner-wide.png" alt="Open Knowledge CLI banner" width="1160" height="125">
</p>

# Open Knowledge

Git-native CI and an MCP runtime for agent knowledge. Open Knowledge finds
stale, conflicting, and missing knowledge, prepares evidence-backed fixes,
tests real agent answers, and serves the latest passing version.

[🌐 Website](https://openknowledge.sh) |
[📖 Documentation](https://openknowledge.sh/wiki/) |
[🧭 Commands](Wiki/features/commands/index.md) |
[📝 Changelog](https://openknowledge.sh/wiki/changelog/cli.html) |
[📐 OKF specification][okf-spec] |
[⚖️ License](LICENSE)

<p align="left">
  <a href="LICENSE"><img alt="License: Apache-2.0" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
  <a href="https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md"><img alt="OKF v0.2" src="https://img.shields.io/badge/OKF-v0.2-2f6feb"></a>
  <a href="https://openknowledge.sh"><img alt="Git-native Markdown wiki" src="https://img.shields.io/badge/wiki-git--native-0f766e"></a>
  <a href="Wiki/index.md"><img alt="Agent-ready docs" src="https://img.shields.io/badge/docs-agent--ready-6f42c1"></a>
</p>

## The knowledge lifecycle

```text
Markdown + Git ──► audit ──► evidence-backed PR ──► answer evals
      ▲                                                │
      └──── rollback ◄── production MCP ◄── green merge┘
```

## Quick start

### 1. Install

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

### 2. Create

Run the interactive setup wizard:

```sh
okn setup
```

To use an agent that already works inside the project, copy this prompt:

```text
Set up Open Knowledge for this workspace. Check the CLI with `okn version`.
If it is missing, install it from https://openknowledge.sh/install and verify
the installation. Then run `okn setup --prompt` and follow its complete task.
Ask me before product-level decisions. Finish with the printed `okn validate`
and `okn setup complete` commands.
```

The generated task is the source of truth for the setup workflow. The default
target is `Wiki`.

To let the CLI launch an installed agent runtime, run:

```sh
okn setup --agent <codex|claude|opencode>
```

Agent mode runs the same generated task. It validates and activates the wiki
through `okn setup complete`.

### 3. Validate

When the agent finishes:

```sh
okn validate Wiki
```

### 4. Add Knowledge CI and production MCP

```sh
okn setup ci Wiki
okn setup runtime Wiki
```

The CI profile adds a source baseline, starter eval questions, and GitHub gates.
The runtime profile publishes immutable passing generations. See the complete
[Knowledge CI Golden Path](Wiki/features/golden-path.md).

<details>
<summary>Alternative npm installation</summary>

```sh
npm install -g @openknowledge-sh/openknowledge
```

Both installers provide `okn` and `openknowledge`. This documentation uses
`okn`. See [Installation](Wiki/features/installation.md) for supported
platforms and verification details.

</details>

Before the CLI sends its first event, it discloses anonymous usage and
sanitized error telemetry. Telemetry is enabled by default. Run
`okn telemetry show-payload` to inspect a sample. Put `--no-telemetry` before a
command to disable telemetry and save the opt-out. See
[Product Telemetry and Privacy](Wiki/features/telemetry.md).

## Workflows

Start with the [Knowledge CI Golden Path](Wiki/features/golden-path.md) for the
complete setup, audit, repair, eval, and production MCP loop.

### Create, retrieve, and verify

Use [`setup`](Wiki/features/commands/setup.md) for interactive onboarding. Use
`okn setup --prompt` for an existing agent. Use `okn setup --agent <runtime>`
to launch one. Use
[`search`](Wiki/features/commands/search.md) to retrieve knowledge. Use
[`validate`](Wiki/features/commands/validate.md) to verify the wiki.

### Work locally

Use [`agent`](Wiki/features/commands/agent.md) to run an agent task. Use
[`get`](Wiki/features/commands/get.md) to read one document. Use
[`list`](Wiki/features/commands/list.md) to inspect the content tree. Use
[`view`](Wiki/features/commands/view.md) to browse the wiki.

### Share and connect

Use [`mcp`](Wiki/features/commands/mcp.md) to serve MCP tools. Use
[`export`](Wiki/features/commands/export.md) to publish portable output. Use
[`connect`](Wiki/features/commands/connect.md) to add a source. Use
[`disconnect`](Wiki/features/commands/disconnect.md) to remove a source. Use
[`registry`](Wiki/features/commands/registry.md) to inspect the registry.

### Automate and operate

Use [`automation`](Wiki/features/commands/automation.md) for jobs, insights,
runtime services, and deployments.

### Use advanced tools

Use [`scaffold`](Wiki/features/commands/scaffold.md) for deterministic bundle
creation. Other advanced tools are
[`prompt`](Wiki/features/commands/prompt.md),
[`ast`](Wiki/features/commands/ast.md), and
[`spec`](Wiki/features/commands/spec.md).

See the [command index](Wiki/features/commands/index.md) for all commands. Use
`okn <command> --help` for exact command syntax.

## Publish a wiki

`okn export html` creates a public distribution bundle in a local folder.
It does not deploy the bundle or start an MCP server.

Review the knowledge base before you create this bundle. After you enable
publication, Markdown is public unless it sets `okf_publish: false`. Add this
configuration to `Wiki/.openknowledge.toml`:

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
and [`.openknowledge.toml`](Wiki/features/configuration.md) for publication
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
