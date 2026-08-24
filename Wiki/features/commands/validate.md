---
type: Command Documentation
title: openknowledge validate
description: Validate a knowledge base against an Open Knowledge Format spec.
tags: [openknowledge, cli, command, validation]
timestamp: 2026-08-24T00:00:00Z
---

# `openknowledge validate`

Validate an OKF bundle. An error causes exit status `1`. A warning does not
cause a failure.

## Usage

```sh
okn validate [key-or-path]
okn validate --profile okf Wiki
okn validate --format json Wiki
okn validate --format json --out report.json Wiki
okn validate --rule link-target=error Wiki
okn validate --quiet Wiki
```

| Option | Default | Description |
| --- | --- | --- |
| `key-or-path` | `.` | Registry key or bundle directory. |
| `--spec <version>` | `latest` | OKF spec version. |
| `--profile <profile>` | `bundle` | Validation scope. Use `bundle` or `okf`. |
| `--format <format>` | `text` | `text` or `json`. `--json` is an alias. |
| `--out <file>` | stdout | Atomically write a JSON report. Requires JSON output. |
| `--rule <id=severity>` | config/default | Override a rule. Repeatable. |
| `--quiet` | off | Print only errors. |

## Validation profiles

The default `bundle` profile validates the selected OKF version. It also runs
these Open Knowledge extension checks:

- `publish-metadata`.
- `insight-contract`.
- `rule-catalog`.
- `claim-profile`.
- `corpus-schema`.

Use `--profile okf` to validate only the selected Open Knowledge Format. This
profile does not run the Open Knowledge extension checks.

The profile does not select an OKF version. Use `--spec` to select the version.
An unsupported profile is a usage error.

## Checks

| Rule | Versions | Default | Checks |
| --- | --- | --- | --- |
| `bundle-read` | 0.1, 0.2 | error | The target is a readable directory with no symlink escape. |
| `utf-8` | 0.1, 0.2 | error | Markdown files contain valid UTF-8. |
| `frontmatter` | 0.1, 0.2 | error | YAML frontmatter parses as one mapping. |
| `concept-frontmatter` | 0.1, 0.2 | error | Concept pages include frontmatter. |
| `concept-type` | 0.1, 0.2 | error | Concept pages define a non-empty `type`. |
| `index-frontmatter` | 0.1, 0.2 | error | Non-root indexes use only allowed publication metadata. |
| `log-frontmatter` | 0.1, 0.2 | error | `log.md` has no concept frontmatter. |
| `log-date` | 0.1, 0.2 | error | Level-two log headings use `YYYY-MM-DD`. |
| `publish-metadata` | 0.1, 0.2 | fixed error | Publication flags and targets use supported boolean values. |
| `insight-contract` | 0.1, 0.2 | fixed error | Private insight metadata, targets, provenance, and lifecycle are valid for the selected version. |
| `claim-profile` | 0.1, 0.2 | fixed error | Active Typed Claims v1 data follows its ontology, evidence, lifecycle, relation, and reference contract. |
| `corpus-schema` | 0.1, 0.2 | fixed error | Active Corpus Schema v1 data follows its document, path, metadata, link, and migration contract. |
| `rule-catalog` | 0.1, 0.2 | error | Custom maintenance rules and enabled IDs are valid. |
| `frontmatter-format` | 0.1, 0.2 | warning | Parseable frontmatter follows clean formatting. |
| `markdown-syntax` | 0.1, 0.2 | warning | Links, code spans, tables, and fences look complete. |
| `okf-version` | 0.1, 0.2 | warning | Root `okf_version` matches the selected spec. |
| `okf-0.2-metadata` | 0.2 | warning | Optional 0.2 metadata follows its defined shapes. |
| `link-target` | 0.1, 0.2 | warning | Local Markdown links resolve inside the bundle. |

The scan includes `.md` and `.markdown` files. It skips `.git`. It classifies
`index.md` and `log.md` as reserved files.

A symbolic link below the bundle root fails the scan. This rule also applies
to links that have non-Markdown asset names.

Text and JSON reports group checks under **OKF core** and
**Open Knowledge extensions**. The `okf` profile reports only the **OKF core**
group.

## Severity policy

Configure persistent overrides in `.openknowledge.toml`:

```toml
[validation.rules]
link-target = "error"
markdown-syntax = "off"
```

CLI `--rule` values have priority. Canonical severities are `off`, `warn`, and
`error`. Every checker rule belongs to an explicit spec version.
Configuration can contain a configurable rule from any supported spec version.
Validation applies only rules from the selected OKF version and validation
profile. It ignores known inactive rules. An explicit CLI override must belong
to the selected profile. See
[`.openknowledge.toml`](/features/configuration.md) for accepted compatibility
aliases and strict configuration behavior.

The bundle profile makes `publish-metadata`, `insight-contract`,
`claim-profile`, and `corpus-schema` mandatory. You cannot override these
checks with `--rule` or configuration. The `rule-catalog` check remains
configurable.

## JSON report

JSON output uses `schemaVersion: "1"`. It includes the root, spec version,
active policy, check results, counts, and issues. Each issue can identify its
file, line, rule, severity, and message. The `validation.schema.json` file
defines the contract.

```json
{
  "schemaVersion": "1",
  "root": "/work/project-memory",
  "specVersion": "0.2",
  "summary": {
    "status": "pass",
    "errorCount": 0,
    "warningCount": 0,
    "issueCount": 0
  },
  "issues": []
}
```

Validation is deterministic. Use `okn prompt review rules` for an
advisory rule review. That review does not affect validation status.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/validate.go`
> - `packages/cli/internal/okf/validation_checks.go`
> - `packages/cli/internal/okf/validation_policy.go`
> - `packages/cli/internal/okf/validation_profiles.go`
> - `packages/cli/schemas/v1/validation.schema.json`
> - `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> Update this page when validation rules, severity, output, or exit behavior
> changes.
