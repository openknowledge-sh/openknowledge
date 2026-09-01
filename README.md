<p align="center">
  <img src="docs/assets/openknowledge-readme-banner-wide.png" alt="Open Knowledge CLI banner" width="1160" height="125">
</p>

# Open Knowledge

Open Knowledge turns repository Markdown into managed, searchable knowledge
for people and agents. One lifecycle covers setup, checks, review, format
upgrades, and publication to a static viewer or MCP runtime.

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

### 2. Set up

Run the interactive setup wizard:

```sh
okn setup
```

Setup discovers Markdown across the repository. The Markdown does not need
YAML frontmatter or authored links. Select paths in the CLI, then create a
managed copy or adopt one directory in place.

If setup finds no Markdown, it creates a task for an agent. An installed agent
appears first. You can also copy the task or save it to a file.

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

### 3. Check

When the agent finishes:

```sh
okn check Wiki
```

`check` reports one status from the configured validation, link, freshness,
retrieval, claims, and publication layers.

### 4. Search, review, or browse

```sh
okn search Wiki "release workflow"
okn review Wiki
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

## Optional controls

The local knowledge workflow does not require typed claims, Knowledge CI, or
a production runtime.

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

Use [`setup`](Wiki/features/commands/setup.md) to create or adopt knowledge.
Use [`check`](Wiki/features/commands/check.md) to get one health status. Use
[`search`](Wiki/features/commands/search.md) to retrieve managed or unmanaged
Markdown. Use [`view`](Wiki/features/commands/view.md) to browse a bundle. Use
[`review`](Wiki/features/commands/review.md) for an optional agent review. Use
[`publish`](Wiki/features/commands/publish.md) for configured output. Use
[`upgrade`](Wiki/features/commands/upgrade.md) for OKF format changes.

### Advanced

The CLI retains direct validation, audit, claims, evidence, eval, quality,
query, export, MCP, connection, automation, and agent tools. Root help groups
these lower-level controls under **Advanced**.

See the [command index](Wiki/features/commands/index.md) for all commands. Use
`okn <command> --help` for exact command syntax.

## Publish a wiki

`okn publish` builds configured output only after `okn check` permits it.
Unmanaged Markdown and blocked knowledge cannot be published.

Review the knowledge base before you create this bundle. After you enable
publication, Markdown is public unless it sets `okf_publish: false`. Add this
configuration to `Wiki/.openknowledge.toml`:

```toml
[release]
outputs = ["viewer"]
```

Inspect the plan, then publish the reviewed content:

```sh
okn publish Wiki --target viewer --out ./site --plan
okn publish Wiki --target viewer --out ./site
```

The bundle includes a static viewer, `llms.txt`, a connect manifest, and a
portable source archive. Add `"mcp"` to `release.outputs` when a deployed
runtime should also expose HTTP MCP. See [HTML export](Wiki/features/exporters/html.md)
and [`.openknowledge.toml`](Wiki/features/configuration.md) for publication
filters and asset options. The advanced `okn export html` command remains
available for direct exporter control.

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
