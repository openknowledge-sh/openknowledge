---
type: Configuration Reference
title: openknowledge.toml
description: Strict bundle-local configuration contract shared by Open Knowledge CLI features.
tags: [openknowledge, cli, configuration, toml]
timestamp: 2026-07-15T00:00:00Z
---

# `openknowledge.toml`

`openknowledge.toml` is the optional bundle-local configuration file shared by
validation, maintenance rules, the local viewer, and static HTML publication.
The CLI decodes the whole file with one TOML v1-compatible typed parser; each
consumer sees the same syntax and the same errors.

## Supported Configuration

```toml
[rules]
paths = ["rules", "policy-rules"]
enabled = ["docs", "changelog"]

[validation.rules]
link-target = "error"
markdown-syntax = "off"

[html.theme]
name = "landing"
stylesheet = "assets/wiki-theme.css"

[html.source]
github_base = "https://github.com/example/knowledge/blob/main"
entry = "Wiki"

[html.site]
base_url = "https://example.com/knowledge/"

[publish]
enabled = true
assets = ["assets/public/**", "whitepapers/*.pdf"]
```

| Field | Type | Behavior |
| --- | --- | --- |
| `rules.paths` | string or string array | Relative custom-rule directories; defaults to `rules`. |
| `rules.enabled` | string or string array | Default canonical rule IDs for rules and review commands. |
| `validation.rules.<rule-id>` | string | Canonical severity `off`, `warn`, or `error` for a known validation rule; compatibility aliases are accepted as described below. |
| `html.theme.name` | string | Viewer/export theme contract name; defaults to `default`. |
| `html.theme.stylesheet` | string | Relative bundle CSS path or absolute HTTP(S) URL. |
| `html.source.github_base` | string | Absolute HTTP(S) repository source base URL. |
| `html.source.entry` | string | Optional relative repository path prefix. |
| `html.site.base_url` | string | Absolute HTTP(S) deployed root without query or fragment. |
| `publish.enabled` | boolean | Explicit permission to create public artifacts. Defaults to `false`; public HTML and runtime generation fail closed until set to `true`. |
| `publish.assets` | string or string array | Bundle-relative glob allowlist for non-Markdown files copied into public HTML and portable public source artifacts. `**` matches path segments recursively. |

`rules.paths` and `rules.enabled` retain the existing single-string shorthand.
All other fields use the exact types above. Standard TOML features such as
single-quoted strings, escaped basic strings, comments, multiline arrays, and
dotted tables are parsed by the shared TOML implementation instead of
line-oriented approximations.

Validation severity values normalize `ignore`, `ignored`, and `none` to
`off`; `warning` and `warnings` to `warn`; and `err` and `errors` to `error`.
New configuration should use the canonical spellings.

## Strictness And Safety

Unknown top-level sections, unknown nested fields, duplicate keys, malformed
TOML, wrong value types, unknown validation rule IDs, and invalid severity
values are errors. A typo in one section is never silently ignored by a command
that consumes another section. This fail-closed behavior prevents a bundle from
appearing valid in one surface while publishing different configuration in
another.

HTML aliases accepted by older ad-hoc readers, such as `css`, `githubBase`, or
`site_url`, are not part of the contract. Use the canonical snake-case fields
shown above.

The config file is private viewer metadata: it is not listed or served through
asset/raw viewer routes. Bundle-root loading also applies the real filesystem
boundary and rejects a symbolic-link `openknowledge.toml` rather than following
it outside the bundle.

Public artifacts are allowlist-based. `[publish] enabled = true` is the
required bundle-level permission and is absent/false by default. After that
gate, Markdown is selected by `okf_publish` and optional `okf_targets`; every
non-Markdown file is excluded unless it matches `publish.assets`. Asset
patterns cannot re-include unpublished Markdown, and
`.git`, `.openknowledge`, and `openknowledge.toml` remain excluded even under a
broad pattern. A source repository that is itself public still exposes its Git
contents: `okf_publish: false` is an artifact filter, not repository secrecy.

Per-page `okf_publish` defaults to allowed only after the bundle gate succeeds;
literal `false` is an absolute deny for every public projection. Optional
target booleans default to `true`:

```yaml
okf_publish: true
okf_targets:
  viewer: true
  search: false
  mcp: false
  llms: true
  sitemap: true
```

Unknown targets, non-boolean target values, and non-boolean `okf_publish`
values fail validation and publication. Targets route already-public content;
they are not confidentiality boundaries. Use `okf_publish: false` when content
must be physically absent from every public generation.

## Consumer Behavior

* `openknowledge validate` applies `[validation.rules]` and uses `[rules]` for
  deterministic rule-catalog checks.
* `openknowledge prompt rules` and `openknowledge prompt review rules` use `[rules]` for
  custom catalog paths and default selection.
* `openknowledge view` uses `[html.theme]`.
* Default `openknowledge export html` uses `[html.theme]`, `[html.source]`, and
  `[html.site]` together through the same strict parser used during validation,
  and uses `[publish]` for the bundle gate and public asset allowlist.
* Plain HTML also requires `publish.enabled`; JSON, graph, and standalone tar
  output do not apply publication settings, and project configuration remains
  part of those source-oriented views.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/project_config.go`
> * `packages/cli/internal/okf/project_config_test.go`
> * `packages/cli/internal/okf/validation_policy.go`
> * `packages/cli/internal/okf/rule_catalog.go`
> * `packages/cli/cmd/openknowledge/viewer_theme.go`
> * `packages/cli/cmd/openknowledge/viewer.go`
>
> **Update notes**
>
> Update this page whenever a supported section, field, type, alias, default,
> or configuration consumer changes. CLI behavior changes also require a
> [CLI changelog](/changelog/cli.md) update.
