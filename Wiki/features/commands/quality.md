---
type: Command Documentation
title: openknowledge quality
description: Build evidence-backed quality reports and retain an agentic intervention audit log.
tags: [openknowledge, cli, command, quality, usage, feedback, eval, audit, interventions]
timestamp: 2026-08-21T00:00:00Z
---

# `openknowledge quality`

Use `okn quality report` to combine current knowledge metadata with runtime
usage, grounded feedback, eval results, and audit findings.

## Usage

```sh
okn quality report [knowledge-base]
okn quality report Wiki \
  --usage /var/lib/openknowledge/usage \
  --feedback /var/lib/openknowledge/feedback \
  --eval eval-report.json \
  --audit audit-report.json \
  --intervention /var/lib/openknowledge/interventions
okn quality report Wiki --format markdown --out quality.md
okn quality report Wiki --json --out quality.json
okn quality report Wiki --format html --out quality.html
```

The knowledge base can be a name or path. The default is `.`. The default OKF
specification is `latest`.

| Option | Default | Behavior |
| --- | --- | --- |
| `--usage <file-or-dir>` | none | Add strict usage event JSONL. Repeat this option as necessary. |
| `--feedback <file-or-dir>` | none | Add strict feedback event JSONL. Repeat this option as necessary. |
| `--eval <file>` | none | Add an eval report or comparison. Repeat this option as necessary. |
| `--audit <file>` | none | Add an audit report. Repeat this option as necessary. |
| `--intervention <file-or-dir>` | none | Add strict intervention event JSONL. Repeat this option as necessary. |
| `--spec <version>` | `latest` | Select the OKF version. |
| `--format text\|json\|markdown\|html` | `text` | Select the output format. `--json` selects JSON. |
| `--out <file>` | stdout | Write JSON, Markdown, or HTML atomically. HTML requires this option. Text cannot use it. |

## Input binding

The command builds the current context index and bundle SHA-256 before it
combines inputs. An eval report must match the current specification and index
revision. An eval comparison must match the current proposed revision.

An audit report must match the current specification and bundle SHA-256.
Feedback must reference a supplied usage event and match its identity and
selected evidence.

Duplicate usage, feedback, or intervention event IDs are rejected. Duplicate
eval report or comparison identities are also rejected so repeated inputs
cannot inflate a metric; repeated audit finding IDs are counted once.

When one usage event has multiple feedback events, the report uses the latest
feedback. It uses the lexically later feedback ID to resolve equal timestamps.

## Metrics

The report does not calculate a global quality score. Each metric has a
`measured` or `unavailable` status, a unit, and an evidence note.

| Metric | Measurement |
| --- | --- |
| `agent-used-current-trusted-eval-covered` | Selected evidence occurrences that are current, trusted, and linked to an eval case. |
| `trusted-answer-rate` | Answers whose complete selected evidence set is current and trusted. |
| `unanswered-question-rate` | Usage events without selected evidence, including policy refusals. |
| `negative-feedback-rate` | Negative latest feedback divided by all latest feedback. |
| `unanswered-question-rate-change` | Latest generation unanswered rate against the preceding observed generation. |
| `eval-answer-accuracy` | Passing standalone eval cases and proposed comparison cases. |
| `answer-accuracy-change` | Weighted proposed accuracy against matching base accuracy. |
| `conflicts-detected` | `claim-conflict` findings in supplied audit reports. |
| `detection-to-published-fix` | Mean hours from detection to a verified published intervention. |
| `human-review-minutes-per-fix` | Mean recorded approved-review minutes for verified published fixes. |
| `audit-false-positive-rate` | Explicit false-positive outcomes divided by classified audit-finding interventions. |
| `safely-automated-maintenance-rate` | Verified low-risk, auto-approved publications without a later rollback divided by all published interventions. |

The north-star denominator counts selected evidence occurrences, not requests.
`current` excludes stale and deprecated concepts. `trusted` requires
`machine-confirmed` or `human-reviewed` trust. Eval coverage uses expected,
retrieved, or answer-cited source paths.

Without an eval report or comparison, coverage is `unavailable`, not false:
concepts keep `evalCoverageStatus: unavailable`, the north star remains
unavailable, and missing coverage does not create a priority by itself.

Metrics stay unavailable when their required observations are absent. The four
intervention-derived metrics remain unavailable until their exact lifecycle
evidence is supplied; the report does not infer review time, finding outcome,
or publication safety from a PR or audit report alone.

## Intervention audit log

Append a strict event to a private JSONL log:

```sh
okn quality interventions append \
  --log /var/lib/openknowledge/interventions \
  --event intervention.json
```

The log directory is mode `0700`; daily JSONL files are mode `0600`. The
append command rejects unknown JSON fields, duplicate event IDs, unsafe target
paths, unsorted evidence, invalid risk and approval pairings, and impossible
lifecycle transitions.

Every intervention begins with `detected`. It can progress through `proposed`
and an optional `reviewed` stage to verified `published`, or end as
`dismissed` or `failed`. Only a published intervention can become
`rolled-back`. Timestamps must increase within one lifecycle.

Events bind one intervention ID to the knowledge base, actor, source, route,
targets, and evidence. Risk and approval are fixed as `low/auto`,
`medium/human`, or `high/expert`; human and expert routes require owners.

`reviewed` events carry a decision and review duration. `published` events
carry the generation, digest, successful checks, verification state, and
whether publication was automated. Automated publication is accepted only
for low-risk auto-approved work. Terminal audit-finding events explicitly
classify the finding as `confirmed` or `false-positive`.

The strict event contract is `intervention-event.schema.json` v1.

Hosted runtime maintenance writes `detected` and `proposed` automatically for
validated agent runs. A low-risk automatic run receives `published` only when
the exact GitHub squash commit becomes an active, check-bound runtime
generation. Human and expert review outcomes remain explicit append events.

## Concept priorities

Each concept records sources, trust, lifecycle state, eval coverage, use,
feedback, and audit findings. It receives `high`, `medium`, `low`, or `none`
priority. The report does not reduce these observations to one score.

High priority includes high audit findings, risky negative feedback, or used
knowledge that is not current or trusted. Used knowledge without eval coverage
has medium priority. Unused knowledge without eval coverage has low priority
only when coverage was measured from a current-revision eval input.

Text output shows all metrics and up to ten concrete priorities. Markdown
shows all concrete priorities and eval comparison changes. JSON returns the
complete `quality-report.schema.json` v1 object.

## HTML dashboard

`--format html` writes one self-contained offline dashboard. The file includes
inline styles and client-side controls. It does not load external resources or
change any metric calculation.

The first view shows the bundle identity and health ledger. The actionable
priority queue follows it. Search matches concept paths, titles, and risk
reasons. Priority and eval coverage filters update the queue in the browser.

The dashboard also shows generation outcomes, eval comparison changes, and the
complete metric ledger. Responsive styles support narrow screens. Print
styles remove interactive controls and prepare the ledger for print output.

Exit status `1` reports invalid input data, revision mismatch, or an
operational failure. Exit status `2` reports invalid command usage.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/quality_command.go`
> - `packages/cli/internal/quality/`
> - `packages/cli/internal/intervention/`
> - `packages/cli/internal/quality/html.go`
> - `packages/cli/schemas/v1/quality-report.schema.json`
> - `packages/cli/schemas/v1/intervention-event.schema.json`
>
> **Update notes**
>
> Update this page when quality inputs, metrics, priorities, or output contracts change.
