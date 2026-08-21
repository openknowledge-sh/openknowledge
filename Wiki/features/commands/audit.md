---
type: Command Documentation
title: openknowledge audit
description: Find concrete knowledge risks and report evidence for each finding.
tags: [openknowledge, cli, command, audit, risk, ci]
timestamp: 2026-08-21T00:00:00Z
---

# `openknowledge audit`

Use `okn audit` to find deterministic knowledge risks. Each finding contains
a severity, impact, targets, and concrete evidence.

## Usage

```sh
okn audit [path]
okn audit Wiki --usage /var/lib/openknowledge/usage
okn audit Wiki --baseline .openknowledge/audit-sources.json --update-baseline
okn audit Wiki --fail-on high --format json --out audit-report.json
```

The default path is `.`. The default OKF spec is `latest`.

| Option | Default | Behavior |
| --- | --- | --- |
| `--usage <file-or-dir>` | none | Add private runtime usage events. Repeat this option as necessary. |
| `--baseline <file>` | none | Compare current source identities with a v1 baseline. |
| `--update-baseline` | off | Write the current baseline after the audit. Requires `--baseline`. |
| `--min-occurrences <n>` | `2` | Set the recurring unanswered-query threshold. |
| `--high-use-threshold <n>` | `5` | Set the used-unverified selection threshold. |
| `--fail-on none\|low\|medium\|high` | `none` | Fail at or above this severity. |
| `--spec <version>` | `latest` | Select the OKF version. |
| `--format text\|json` | `text` | Select output format. `--json` selects JSON. |
| `--out <file>` | stdout | Write JSON atomically. Requires JSON format. |

## Findings

The audit evaluates typed, parseable, nonreserved documents. It skips documents
with `okf_publish: false`. Current detectors produce these findings:

- High: stale knowledge, broken local dependencies, claim conflicts, missing
  local source resources, changed sources, recurring unanswered questions,
  and frequently used unverified or stale knowledge.
- Medium: missing structured sources, missing owners, identical normalized
  bodies, and duplicate normalized titles.

Use these optional frontmatter extensions for routing and conflict detection:

```yaml
owner: team:platform
# Or use: owners: [team:platform, human:reviewer]
claims:
  - id: deploy.region
    value: eu-west-1
```

`owner` and `owners` accept one owner or a list. A document without either
field receives a `missing-owner` finding.

Each claim uses an `id` and `value`. Different normalized values for the same
claim ID produce one high-severity `claim-conflict` finding.

## Sources and usage

A source baseline records source identities. A local source fingerprint uses
its file content. An absolute URL fingerprint uses its `last_modified` value.
The audit does not fetch network sources.

Use `--update-baseline` to create or replace the current baseline. A later run
reports changed source identities against that file.

Usage inputs use strict private `usage-event` v1 JSONL. The audit reports
recurring no-evidence clusters. It also reports selected documents that cross
the usage threshold while unverified or stale.

## CI and exit status

`--fail-on high` returns exit status `1` when the report contains a high-risk
finding. `medium` also fails on high findings. `low` fails on all findings.

Exit status `1` also reports an operational failure. Exit status `2` reports
invalid command usage.

Repository CI runs a high-risk audit gate for `Wiki`. It uploads
`knowledge-audit.json` as the `knowledge-audit-report` artifact for 14 days.
The CI job does not add baseline or usage inputs.

JSON reports follow `audit-report.schema.json` v1. Source baselines follow
`audit-source-baseline.schema.json` v1.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/audit_command.go`
> - `packages/cli/internal/audit/audit.go`
> - `packages/cli/schemas/v1/audit-report.schema.json`
> - `packages/cli/schemas/v1/audit-source-baseline.schema.json`
> - `.github/workflows/ci.yml`
>
> **Update notes**
>
> Update this page when audit findings, options, contracts, or CI gates change.
