---
type: Command Documentation
title: openknowledge validate
description: Validate a knowledge base against an Open Knowledge Format spec.
tags: [openknowledge, cli, command, validation]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge validate`

Validate an OKF bundle. An error causes exit status `1`. A warning does not
cause a failure.

## Usage

```sh
openknowledge validate [key-or-path]
openknowledge validate --format json Wiki
openknowledge validate --format json --out report.json Wiki
openknowledge validate --rule link-target=error Wiki
openknowledge validate --quiet Wiki
```

| Option | Default | Description |
| --- | --- | --- |
| `key-or-path` | `.` | Registry key or bundle directory. |
| `--spec <version>` | `latest` | OKF spec version. |
| `--format <format>` | `text` | `text` or `json`. `--json` is an alias. |
| `--out <file>` | stdout | Atomically write a JSON report. Requires JSON output. |
| `--rule <id=severity>` | config/default | Override a rule. Repeatable. |
| `--quiet` | off | Print only errors. |

## Checks

| Rule | Default | Checks |
| --- | --- | --- |
| `bundle-read` | error | The target is a readable directory with no symlink escape. |
| `utf-8` | error | Markdown files contain valid UTF-8. |
| `frontmatter` | error | YAML frontmatter parses as one mapping. |
| `concept-frontmatter` | error | Concept pages include frontmatter. |
| `concept-type` | error | Concept pages define a non-empty `type`. |
| `index-frontmatter` | error | Non-root indexes use only allowed publication metadata. |
| `log-frontmatter` | error | `log.md` has no concept frontmatter. |
| `log-date` | error | Level-two log headings use `YYYY-MM-DD`. |
| `publish-metadata` | fixed error | Publication flags and targets use supported boolean values. |
| `insight-contract` | fixed error | Private insight metadata, targets, and status are valid. |
| `rule-catalog` | error | Custom maintenance rules and enabled IDs are valid. |
| `frontmatter-format` | warning | Parseable frontmatter follows clean formatting. |
| `markdown-syntax` | warning | Links, code spans, tables, and fences look complete. |
| `okf-version` | warning | Root `okf_version` matches the selected spec. |
| `link-target` | warning | Local Markdown links resolve inside the bundle. |

The scan includes `.md` and `.markdown` files. It skips `.git`. It classifies
`index.md` and `log.md` as reserved files.

A symbolic link below the bundle root fails the scan. This rule also applies
to links that have non-Markdown asset names.

## Severity policy

Configure persistent overrides in `openknowledge.toml`:

```toml
[validation.rules]
link-target = "error"
markdown-syntax = "off"
```

CLI `--rule` values have priority. Canonical severities are `off`, `warn`, and
`error`. An unknown rule or severity is a usage error. See
[`openknowledge.toml`](/features/configuration.md) for accepted compatibility
aliases and strict configuration behavior.

`publish-metadata` and `insight-contract` are mandatory checks. You cannot
override them with `--rule` or configuration.

## JSON report

JSON output uses `schemaVersion: "1"`. It includes the root, spec version,
active policy, check results, counts, and issues. Each issue can identify its
file, line, rule, severity, and message. The `validation.schema.json` file
defines the contract.

```json
{
  "schemaVersion": "1",
  "root": "/work/project-memory",
  "specVersion": "0.1",
  "summary": {
    "status": "pass",
    "errorCount": 0,
    "warningCount": 0,
    "issueCount": 0
  },
  "issues": []
}
```

Validation is deterministic. Use `openknowledge prompt review rules` for an
advisory rule review. That review does not affect validation status.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/validate.go`
> - `packages/cli/internal/okf/validation_checks.go`
> - `packages/cli/internal/okf/validation_policy.go`
> - `packages/cli/schemas/v1/validation.schema.json`
> - `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> Update this page when validation rules, severity, output, or exit behavior
> changes.
