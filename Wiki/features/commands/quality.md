---
type: Command Documentation
title: openknowledge quality report
description: Build evidence-backed knowledge quality metrics and concept priorities.
tags: [openknowledge, cli, command, quality, usage, feedback, eval, audit]
timestamp: 2026-08-21T00:00:00Z
---

# `openknowledge quality report`

Use `okn quality report` to combine current knowledge metadata with runtime
usage, grounded feedback, eval results, and audit findings.

## Usage

```sh
okn quality report [knowledge-base]
okn quality report Wiki \
  --usage /var/lib/openknowledge/usage \
  --feedback /var/lib/openknowledge/feedback \
  --eval eval-report.json \
  --audit audit-report.json
okn quality report Wiki --format markdown --out quality.md
okn quality report Wiki --json --out quality.json
```

The knowledge base can be a name or path. The default is `.`. The default OKF
specification is `latest`.

| Option | Default | Behavior |
| --- | --- | --- |
| `--usage <file-or-dir>` | none | Add strict usage event JSONL. Repeat this option as necessary. |
| `--feedback <file-or-dir>` | none | Add strict feedback event JSONL. Repeat this option as necessary. |
| `--eval <file>` | none | Add an eval report or comparison. Repeat this option as necessary. |
| `--audit <file>` | none | Add an audit report. Repeat this option as necessary. |
| `--spec <version>` | `latest` | Select the OKF version. |
| `--format text\|json\|markdown` | `text` | Select the output format. `--json` selects JSON. |
| `--out <file>` | stdout | Write JSON or Markdown atomically. Text output cannot use this option. |

## Input binding

The command builds the current context index and bundle SHA-256 before it
combines inputs. An eval report must match the current specification and index
revision. An eval comparison must match the current proposed revision.

An audit report must match the current specification and bundle SHA-256.
Feedback must reference a supplied usage event and match its identity and
selected evidence.

Duplicate usage or feedback event IDs are rejected. Duplicate eval report or
comparison identities are also rejected so repeated inputs cannot inflate a
metric; repeated audit finding IDs are counted once.

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

The north-star denominator counts selected evidence occurrences, not requests.
`current` excludes stale and deprecated concepts. `trusted` requires
`machine-confirmed` or `human-reviewed` trust. Eval coverage uses expected,
retrieved, or answer-cited source paths.

Without an eval report or comparison, coverage is `unavailable`, not false:
concepts keep `evalCoverageStatus: unavailable`, the north star remains
unavailable, and missing coverage does not create a priority by itself.

Metrics stay unavailable when their required observations are absent. Four
intervention-derived metrics are explicitly unavailable in v1:

- `detection-to-published-fix`
- `human-review-minutes-per-fix`
- `audit-false-positive-rate`
- `safely-automated-maintenance-rate`

These metrics require a unified intervention log or review outcome events.

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

Exit status `1` reports invalid input data, revision mismatch, or an
operational failure. Exit status `2` reports invalid command usage.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/quality_command.go`
> - `packages/cli/internal/quality/`
> - `packages/cli/schemas/v1/quality-report.schema.json`
>
> **Update notes**
>
> Update this page when quality inputs, metrics, priorities, or output contracts change.
