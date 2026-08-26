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
okn audit Wiki --observe-remote --format markdown --out audit-report.md
okn audit Wiki --format json --out audit.json --markdown-out audit.md
okn audit propose <finding-id> --report audit-report.json --path Wiki
```

The default path is `.`. The default OKF spec is `latest`.

| Option | Default | Behavior |
| --- | --- | --- |
| `--usage <file-or-dir>` | none | Add private runtime usage events. Repeat this option as necessary. |
| `--baseline <file>` | none | Compare current source identities with a v1 baseline. |
| `--update-baseline` | off | Write the current baseline after the audit. Requires `--baseline`. |
| `--observe-remote` | off | Observe remote sources that opt in with `observe`. |
| `--min-occurrences <n>` | `2` | Set the recurring unanswered-query threshold. |
| `--high-use-threshold <n>` | `5` | Set the used-unverified selection threshold. |
| `--fail-on none\|low\|medium\|high` | `none` | Fail at or above this severity. |
| `--spec <version>` | `latest` | Select the OKF version. |
| `--format text\|json\|markdown` | `text` | Select output format. `--json` selects JSON. |
| `--out <file>` | stdout | Write JSON or Markdown atomically. |
| `--markdown-out <file>` | none | Also write Markdown from the same audit run. |

## Findings

The audit evaluates typed, parseable, nonreserved documents. It skips documents
with `okf_publish: false`. Current detectors produce these findings:

- High: stale knowledge, broken local dependencies, claim conflicts, stale or
  invalid typed claim evidence, missing local source resources, changed sources, recurring
  unanswered questions, and frequently used unverified knowledge.
- Medium: missing structured sources, missing owners, identical normalized
  bodies, duplicate titles, exact claim duplicates, and claims without an
  evidence reference.

Audit uses the same strict typed claim contract as validation:

```yaml
owner: team:platform
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces: {deploy: https://example.com/deploy/}
  entities: [{id: deploy:service}]
  predicates: [{id: deploy:region, object_kind: literal, datatype: xsd:string, maximum_count: 1}]
sources:
  - {id: runbook, resource: ./runbook.yaml}
claims:
  - id: deploy:claim/region/eu
    slot: deploy:slot/region
    subject: deploy:service
    predicate: deploy:region
    object: {value: eu-west-1, datatype: xsd:string}
    evidence:
      - {id: deploy:evidence/region/eu, source_ref: runbook, stance: supports, role: primary}
    status: supported
```

`owner` and `owners` accept one owner or a list. A document without either
field receives a `missing-owner` finding.

Use [Typed Claims v1](/features/claim-profile.md) for the claim contract.
Claim conflicts use slot, subject, predicate, typed scope, overlapping
validity, cardinality, and normalized typed objects. Audit adds exact
`claim_refs` dependents to the finding targets.

## Sources and usage

A source baseline records source identities. A local source fingerprint uses
its file content. Remote access is opt-in at both levels: the source selects an
observation mode and the command uses `--observe-remote`.

```yaml
sources:
  - id: policy
    resource: https://example.com/policy
    observe: metadata # manual, metadata, fetch, or pinned
```

`manual` is the default and uses declared metadata such as `last_modified`.
`metadata` sends a HEAD request and fingerprints ETag, Last-Modified, and
Content-Length. `fetch` sends a GET request and fingerprints at most 8 MiB of
content. `pinned` uses the source `sha256` and performs no request. A remote
failure produces an unavailable-source finding. Audit never accesses the
network without `--observe-remote`.

Use `--update-baseline` to create or replace the current baseline. A later run
reports changed source identities against that file.

The command refuses `--update-baseline` while a `source-changed` finding is
open. A changed source remains open until each typed claim that cites it has a
current evidence observation. Use `claims stale` to inspect the exact claim
IDs and `claims reconcile` after review. Sources without typed claims keep the
page-level source-change finding.

After review accepts a changed source and the dependent knowledge is fixed,
update the baseline in the same pull request. Until then, the changed-source
gate remains open and production publication fails closed.

`audit propose` converts exactly one finding from a saved JSON report into a
durable insight proposal. It preserves the finding ID, evidence, targets,
owner route, risk, and confidence. Run the returned
`okn automation insights run` command to let the configured agent prepare a
reviewable fix.

Usage inputs use strict private `usage-event` v1 JSONL. The audit reports
recurring no-evidence clusters. It also reports selected documents that cross
the usage threshold while unverified or stale.

## CI and exit status

`--fail-on high` returns exit status `1` when the report contains a high-risk
finding. `medium` also fails on high findings. `low` fails on all findings.

Exit status `1` also reports an operational failure. Exit status `2` reports
invalid command usage.

Repository CI runs validation, a base-aware claim lifecycle gate, and a
high-risk audit for `Wiki`. It uploads the audit and claim reports as the
`knowledge-audit-report` artifact for 14 days. `okn setup ci` generates a
complete workflow with the source baseline and answer regression reports.

This repository passes its tracked source baseline to the audit. Only
frontmatter `sources` entries participate in source-change detection. See
[Claim freshness](/features/claim-freshness.md) for the claim-level workflow.

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
