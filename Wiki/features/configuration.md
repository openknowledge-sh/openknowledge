---
type: Configuration Reference
title: .openknowledge.toml
description: Strict bundle-local configuration contract shared by Open Knowledge CLI features.
tags: [openknowledge, cli, configuration, toml]
timestamp: 2026-07-15T00:00:00Z
---

# `.openknowledge.toml`

`.openknowledge.toml` is an optional configuration file in the bundle.
Validation, maintenance rules, the local viewer, and static HTML publication use this file.
One TOML v1-compatible typed parser decodes the complete file.
Therefore, each consumer uses the same syntax and reports the same errors.

## Supported configuration

```toml
[rules]
paths = ["rules", "policy-rules"]
enabled = ["project", "writing", "docs"]

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

[release]
branch = "main"
policy = "follow-main"
outputs = ["viewer", "mcp"]

[publish]
assets = ["assets/public/**", "whitepapers/*.pdf"]

[maintenance]
mode = "off"
agent = "codex"
delivery = "pull-request"
auto_merge = false
```

| Field | Type | Behavior |
| --- | --- | --- |
| `rules.paths` | string or string array | Relative custom-rule directories. The default is `rules`. |
| `rules.enabled` | string or string array | Default canonical rule IDs for rules and review commands. The fallback is `project` and `writing`. |
| `validation.rules.<rule-id>` | string | Canonical severity for a known validation rule. Use `off`, `warn`, or `error`. The CLI accepts the compatibility aliases below. |
| `html.theme.name` | string | Viewer or export theme contract name. The default is `default`. |
| `html.theme.stylesheet` | string | Relative bundle CSS path or absolute HTTP(S) URL. |
| `html.source.github_base` | string | Absolute HTTP(S) repository source base URL. |
| `html.source.entry` | string | Optional relative repository path prefix. |
| `html.site.base_url` | string | Absolute HTTP(S) deployed root without query or fragment. |
| `publish.assets` | string or string array | Bundle-relative glob list for public non-Markdown files. `**` matches path segments recursively. |
| `release.branch` | string | Production branch watched by GitHub automation. The default is `main`. |
| `release.policy` | string | On the production branch, `follow-main` publishes a structurally valid generation with `degraded` health after quality failures; `last-passing` keeps the prior passing generation. The default is `follow-main`. Pull-request quality failures remain failing checks. |
| `release.outputs` | string array | Public projections: `viewer`, `mcp`, or both. Missing or empty means local-only. |
| `maintenance.mode` | string | `off`, `propose`, or `autonomous`. The default is `off`, so ordinary CI installs and invokes no model harness. |
| `maintenance.agent` | string | `codex`, `claude`, or `opencode`. The default is `codex`; it is used only when maintenance is active. |
| `maintenance.delivery` | string | Delivery transport. The supported value is `pull-request`. |
| `maintenance.auto_merge` | boolean | Ask GitHub to auto-merge a verified maintenance PR. `autonomous` always does this; the field can opt another active mode into the same delivery behavior. |

`rules.paths` and `rules.enabled` accept the existing single-string shorthand.
All other fields use the exact types in the table.
The shared TOML parser supports standard TOML features.
These features include quoted strings, escaped basic strings, comments,
multiline arrays, and dotted tables.

Validation severity values normalize `ignore`, `ignored`, and `none` to
`off`.
The values `warning` and `warnings` normalize to `warn`.
The values `err` and `errors` normalize to `error`.
Use the canonical values in new configuration.

## Strictness and safety

The parser reports an error for an unknown top-level section or nested field.
It also reports duplicate keys, malformed TOML, and incorrect value types.
Unknown validation rule IDs and invalid severity values are errors.
The configuration can contain configurable rules from each supported spec
profile. A command applies only rules from its selected profile. The command
ignores a known rule from another profile. Quote rule IDs that contain a dot
when using them as TOML keys:

```toml
[validation.rules]
"okf-0.2-metadata" = "error"
```

Validation with `--spec 0.1` ignores this known 0.2-only rule. An unknown or
fixed rule still causes an error. An explicit CLI `--rule` must belong to the
selected profile.

A command does not ignore a typographical error in an unused section.
This behavior keeps the configuration consistent between all CLI surfaces.

HTML aliases from old readers are not part of the contract.
Examples are `css`, `githubBase`, and `site_url`.
Use the canonical snake-case fields in the table.

The configuration file is private viewer metadata.
Asset and raw viewer routes do not list or serve it.
Bundle-root loading applies the real file system boundary.
It rejects a symbolic-link `.openknowledge.toml` and does not follow the link outside the bundle.

The CLI loads only `.openknowledge.toml`.
It does not load the legacy plain `openknowledge.toml` file.
The legacy file remains private in viewer raw routes and public artifacts.

Public artifacts use an explicit list of permitted content.
`release.outputs` is the single bundle-level release decision. The default is
an empty list, so a new bundle is local-only. `viewer` permits the static site,
search, `llms.txt`, and sitemap projections. `mcp` permits the runtime MCP
projection. After this gate, `okf_publish` and optional `okf_targets` select Markdown.
A non-Markdown file must match `publish.assets`.
Asset patterns cannot include unpublished Markdown again.
The output always excludes `.git`, `.openknowledge`, `.openknowledge.toml`, and legacy `openknowledge.toml`.
A public source repository still exposes its Git content.
Thus, `okf_publish: false` is an artifact filter and not a confidentiality control.

Per-page `okf_publish` permits publication only after the matching release output succeeds.
The default value permits publication after that gate.
The literal value `false` prevents all public projections.
Optional target Boolean values default to `true`:

```yaml
okf_publish: true
okf_targets:
  viewer: true
  search: false
  mcp: false
  llms: true
  sitemap: true
```

An unknown target causes validation and publication to fail.
A non-Boolean target or `okf_publish` value also causes failure.
Targets route content that is already public.
They are not confidentiality boundaries.
Use `okf_publish: false` to exclude content from each public generation.

## Consumer behavior

* `okn validate` applies `[validation.rules]` against the selected OKF version.
  It uses `[rules]` for deterministic rule-catalog checks.
* `okn prompt rules` uses `[rules]` for custom catalog paths and default selection.
* `okn prompt review rules` also uses `[rules]`.
* `okn view` uses `[html.theme]`.
* Default `okn export html` uses `[html.theme]`, `[html.source]`, and `[html.site]`.
  It uses the same strict parser as validation. It requires `viewer` in
  `release.outputs` and uses `[publish].assets` for the public asset list.
  Plain HTML uses the same output requirement.
* JSON and graph exports ignore publication filters.
* A standalone tar export preserves the complete source, including this configuration.
* `okn automation github plan/run` and the root GitHub Action use `[release]`
  and `[maintenance]` as their desired-state contract.
* Runtime configuration registers the bundle path and route. Runtime build and
  serve load `release.outputs` from that bundle; runtime TOML has no second
  `publish` or `mcp` switch. `[serve].mcp_access` controls access only.
* `okn setup github` writes `release.branch` into the workflow trigger. Runtime
  scaffolding maps the same branch and policy into `worker.production_branch`
  and `runtime.release_policy`.
* `maintenance.mode = "off"` keeps pull-request and push checks deterministic.
  They do not need `OPENKNOWLEDGE_MODEL_TOKEN`, an answer command, or an
  embedding service. Scheduled and manual runs invoke an agent only after an
  explicit `propose` or `autonomous` setting.
* `autonomous` still uses a verified branch and pull request. The Action pushes
  the generated branch, opens the PR, and enables GitHub auto-merge. Structural
  validation remains a hard boundary.

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
> Update this page after a change to a supported section, field, type, alias,
> default, or configuration consumer.
> Also update the [CLI changelog](/changelog/cli.md) after a CLI behavior change.
