<p align="center">
  <img src="docs/assets/openknowledge-readme-banner-wide.png" alt="Open Knowledge CLI banner" width="1160" height="125">
</p>

# Open Knowledge

Git-native CI and an MCP runtime for agent knowledge. Open Knowledge finds
stale, conflicting, and missing knowledge, prepares evidence-backed fixes,
runs deterministic retrieval and evidence regressions, and serves the current
production version with explicit passing or degraded health.

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
Markdown + Git ──► deterministic checks ──► evidence-backed PR
      ▲                                      │
      └──── rollback ◄── production MCP ◄────┘
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

This command uses the `base` preset by default. It creates a
lightweight knowledge base for repository documentation, architecture,
services, decisions, and changelog content.

To use an agent that already works inside the project, copy this prompt:

```text
Set up Open Knowledge for this workspace. Check the CLI with `okn version`.
If it is missing, install it from https://openknowledge.sh/install and verify
the installation. Then run `okn setup --prompt` and follow its complete task.
Ask me before product-level decisions. Finish with the printed `okn validate`
and `okn search` commands.
```

The generated task is the source of truth for the setup workflow. The default
target is `Wiki`.

To let the CLI launch an installed agent runtime, run:

```sh
okn setup --agent <codex|claude|opencode>
```

Agent mode runs the same generated task. It creates and validates the wiki.

### 3. Validate

When the agent finishes:

```sh
okn validate Wiki
```

### 4. Search or browse

```sh
okn search Wiki "release workflow"
okn view Wiki
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

Before the CLI sends its first event, it discloses anonymous usage and
sanitized error telemetry. Telemetry is enabled by default. Run
`okn telemetry show-payload` to inspect a sample. Put `--no-telemetry` before a
command to disable telemetry and save the opt-out. See
[Product Telemetry and Privacy](Wiki/features/telemetry.md).

## Optional upgrades

The local documentation workflow does not require typed claims, Knowledge CI,
or a production runtime.

Use the `trusted` preset when the knowledge base needs explicit
sources, evidence, claim lifecycle, or conflict checks:

```sh
okn setup --use-case trusted
```

Use `okn setup --use-case custom` when neither preset matches the intended
result. All presets use the same Markdown and OKF format. A preset changes the
setup task and starter content, not the knowledge format.

Add Knowledge CI when pull requests must verify knowledge quality. Add the
runtime when clients need a production MCP service:

```sh
okn setup github Wiki
okn setup runtime Wiki
```

The GitHub profile adds an Action-first workflow, source baseline, starter eval
questions, and config-driven gates. The runtime profile publishes immutable generations with
explicit passing or degraded health. See the complete
[Knowledge CI Golden Path](Wiki/features/golden-path.md).

## Workflows

### Start here

Use [`setup`](Wiki/features/commands/setup.md) for interactive onboarding. Use
`okn setup --prompt` for an existing agent. Use `okn setup --agent <runtime>`
to launch one. Use [`validate`](Wiki/features/commands/validate.md) to verify
the wiki. Use [`search`](Wiki/features/commands/search.md) to retrieve
knowledge. Use [`view`](Wiki/features/commands/view.md) to browse the wiki.

### Trust and govern

Use [`audit`](Wiki/features/commands/audit.md) to find knowledge risks. Use
[`claims`](Wiki/features/commands/claims.md) and
[`evidence`](Wiki/features/commands/evidence.md) for trusted facts. Use
[`eval`](Wiki/features/commands/eval.md) and
[`quality`](Wiki/features/commands/quality.md) to measure results.

### Query and interchange

Use [`query`](Wiki/features/commands/query.md) for explicit semantic queries.
Use [`export`](Wiki/features/commands/export.md) to create portable output.

### Publish and operate

Use [`mcp`](Wiki/features/commands/mcp.md) to serve MCP tools. Use
[`connect`](Wiki/features/commands/connect.md) and
[`registry`](Wiki/features/commands/registry.md) to manage sources. Use
[`automation`](Wiki/features/commands/automation.md) for jobs and deployments.

### Advanced internals

Use [`agent`](Wiki/features/commands/agent.md) for an agent task. Use
[`get`](Wiki/features/commands/get.md) and
[`list`](Wiki/features/commands/list.md) for direct inspection. Other advanced
tools are [`scaffold`](Wiki/features/commands/scaffold.md),
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
[release]
outputs = ["viewer"]
```

Export the reviewed content:

```sh
okn export html --out ./site Wiki
```

The bundle includes a static viewer, `llms.txt`, a connect manifest, and a
portable source archive. Add `"mcp"` to `release.outputs` when a deployed
runtime should also expose HTTP MCP. See [HTML export](Wiki/features/exporters/html.md)
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
