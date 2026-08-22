---
type: Feature Documentation
title: Eval Command
description: Test deterministic retrieval evidence against a versioned dataset.
tags: [openknowledge, cli, eval, retrieval, ci]
timestamp: 2026-08-21T00:00:00Z
---

# Eval

Use `eval run` to test retrieved sources and evidence against recorded questions.

```sh
okn eval run <dataset> [key-or-path]
okn eval run evals/deploy.yaml Wiki --format json --out reports/deploy.json
okn eval run evals/deploy.yaml Wiki --base main --gate regressions
okn eval run evals/deploy.yaml Wiki --format markdown --out reports/eval.md
okn eval run evals/deploy.yaml Wiki --answer-command ./answer-runner
```

`key-or-path` is a registry key or knowledge base directory. The default is
the current directory.

## Dataset v1

The command accepts one strict YAML document. Unknown fields, duplicate fields,
and unsupported versions cause dataset validation to fail.

```yaml
type: openknowledge.eval
version: 1
id: deploy
defaults:
  budget: 2400
  limit: 12
cases:
  - id: rollback
    question: How do we restore a failed release?
    agents: [release-agent, support-agent]
    context:
      no_expand: true
    expect:
      sources: [operations/rollback.md]
      evidence_contains: [Restore the previous release]
      evidence_excludes: [Delete production]
      min_sources: 1
      minimum_trust: human-reviewed
      allow_stale: false
      allowed_statuses: [stable]
      require_sources: true
      answer_contains: [Restore]
      answer_excludes: [Delete production]
      citation_sources: [operations/rollback.md]
      min_citations: 1
      min_groundedness: 1.0
      answer_decision: answer
      require_conflict_disclosure: false
      min_entailed_citations: 1
```

Each case must define at least one expectation. Case `context` values override
the dataset defaults. Omitted values use a budget of `2400`, a limit of `12`,
and link expansion.

The optional `agents` list identifies agent consumers for impact review. It
accepts up to 20 unique bounded IDs. Agent IDs can contain letters, numbers,
dots, underscores, and hyphens.

`sources` accepts bundle-relative paths and optional section fragments.
Evidence checks use case-insensitive substring matching across all retrieved
source Markdown. `min_sources` tests the number of retrieved sources.

Policy expectations test OKF 0.2 signals for every selected source:

| Field | Check |
| --- | --- |
| `minimum_trust` | Require `unverified`, `machine-confirmed`, or `human-reviewed` as the minimum trust tier. |
| `allow_stale` | Permit stale sources when `true`. Reject stale sources when `false`. |
| `allowed_statuses` | Permit only the unique listed `draft`, `stable`, or `deprecated` statuses. |
| `require_sources` | Require structured source provenance when `true`. |

The CLI derives trust, freshness, lifecycle status, and provenance from each
source frontmatter. Each declared field produces one aggregate check across
the selected sources. An omitted policy field does not create a check. A
policy check passes when retrieval selects no sources. Use `min_sources` when
a case must also retrieve evidence.

Answer checks require `--answer-command`. `answer_contains` and
`answer_excludes` use case-insensitive substring matching. `citation_sources`
accepts bundle-relative paths and optional section fragments.

`min_citations` counts valid citations. `min_groundedness` accepts a value from
`0` through `1`. A claim is grounded when it cites at least one retrieved
source with its exact locator.

`answer_decision` checks `answer` or `abstain`. An abstention must contain no
claims and at least one `refusalReasons` value. `require_conflict_disclosure`
checks whether the answer reports a nonempty `conflicts` list.
`min_entailed_citations` counts valid cited locators whose answer response has
an `entailed` attestation.

Entailment is an attestation from the trusted answer command. The CLI validates
the locator, status, and method and then scores the attestation. It does not
independently prove that the cited text semantically entails the claim.

## Options and results

| Option | Behavior |
| --- | --- |
| `--format text\|json\|markdown` | Select output. The default is `text`. |
| `--json` | Use JSON output. |
| `--out <file>` | Write JSON or Markdown atomically. Text output does not support this option. |
| `--spec <version>` | Select the OKF version. The default is `latest`. |
| `--base <git-ref>` | Compare the working knowledge base with a Git commit or ref. |
| `--gate all\|regressions` | Select comparison failures. The default is `all`. |
| `--answer-command <executable>` | Run an executable that implements the answer JSON protocol. |
| `--answer-arg <value>` | Pass one argument directly to the executable. Repeat this option as necessary. |
| `--answer-timeout <duration>` | Set a Go duration. The default is `2m`, and the maximum is `1h`. |

The JSON report uses `schemaVersion: "1"`. It records the dataset digest,
target revision, retrieved source locators, case metrics, and each check.
Policy checks use `minimum_trust`, `allow_stale`, `allowed_statuses`, and
`require_sources` as check kinds. A failed check records the rejected source
paths in `actual`.

An answer result contains `decision`, `text`, `claims`, `citedSources`,
`citationCount`, `validCitations`, `entailedCitations`, `claimCount`,
`groundedClaims`, and `groundedness`. It can also preserve refusal reasons,
conflicts, scope, applicability time, and uncertainty. Each claim
contains `text`, `citations`, and `grounded`. Each citation records its
`locator`, resolved `path`, validity, and optional entailment attestation.

Markdown output is a pull request report. It includes case status, answer
changes, groundedness, citation counts, cited sources, and failed checks.
Policy checks contribute to the check totals. Failed policy checks identify
the field and rejected sources.
Without `--out`, JSON and Markdown output go to stdout.

## Answer protocol

The CLI writes one `answer-request.schema.json` document to the command stdin.
The request uses `schemaVersion: "1"` and contains these fields:

- Dataset ID and SHA-256 digest
- Target root and retrieval revision
- Case IDs, questions, and retrieved sources
- Source Markdown, identity, locator, digest, relation, and display fields

The command writes one `answer-response.schema.json` document to stdout. It
must return one answer for each case and no other cases. Each answer contains
`caseId`, `decision`, `answer`, and `claims`. Each claim contains `text` and
`citations`. An answer may add refusal reasons, conflicts, scope,
`applicableAt`, uncertainty, and citation entailment attestations.

Each citation must equal a locator from that case request. The report marks
other citations as invalid. Groundedness is the fraction of claims that have
at least one valid citation.

The CLI starts the executable directly and does not use a shell. Arguments
remain separate values. The CLI rejects invalid JSON, excess output, timeout,
nonzero exit status, and incomplete case sets.

The answer command is trusted code. The CLI does not sandbox it. The command
can access inherited environment values, files, and available network services.
Retrieval stays deterministic. Answer reproducibility depends on the command
and its external services.

## Base comparison

`--base` evaluates the same current dataset against the base and proposed
revisions. The command does not load the dataset from the base revision.

The command resolves the Git ref to a commit. It extracts the knowledge base
from an immutable Git archive and removes the temporary snapshot after use.
The knowledge base must be inside its Git repository.

Each case has one classification:

- `improved`: Base failed and proposed passed.
- `regressed`: Base passed and proposed failed.
- `unchanged_pass`: Both revisions passed.
- `unchanged_fail`: Both revisions failed.

The `all` gate fails if the proposed revision has any failed case. The
`regressions` gate fails only if a case regressed. It permits unchanged
failures. `--gate regressions` requires `--base`.

JSON comparison output follows `eval-comparison.schema.json`. It includes both
revision identities, both case results, classifications, and gate totals.
With an answer command, the CLI invokes that command once for each revision.

The comparison also contains an `impact` object:

| Field | Content |
| --- | --- |
| `changedPaths` | Sorted bundle paths for added, removed, or content-changed regular files outside the private `.openknowledge` directory. |
| `affectedAgents` | Unique agents from affected cases. |
| `affectedQuestions` | Case IDs, questions, agents, linked paths, and impact reasons. |
| `uncoveredPaths` | Changed paths that no eval case links to the change. |

A case links a changed path through an expected, retrieved, or valid cited source.
The CLI removes a section fragment before it compares a source path.

An affected question has at least one of these reasons:

- `source_changed`: A linked source path changed.
- `retrieval_changed`: Retrieved source identity or content changed.
- `outcome_changed`: The case changed between pass and fail.
- `answer_changed`: The answer text changed.

An uncovered path has no link from an eval case through these source sets.
The impact object supports review and does not change gate status.

Text comparison output shows changed path, question, agent, and uncovered path
counts. Markdown adds the affected agent list, a question table, reasons,
linked paths, and the uncovered path list.

## GitHub Actions

Call the repository workflow for pull requests and production branch pushes:

```yaml
on:
  pull_request:
  push:
    branches: [main]

jobs:
  knowledge-eval:
    if: github.event_name == 'pull_request' || github.event.before != '0000000000000000000000000000000000000000'
    uses: ./.github/workflows/knowledge-eval.yml
    with:
      dataset: .openknowledge/evals/knowledge-ci.yaml
      target: Wiki
      base_ref: ${{ github.event.pull_request.base.sha || github.event.before }}
      gate: regressions
```

`dataset`, `target`, and `base_ref` are required. Dataset and target values are
repository-relative paths. Use an immutable commit SHA for `base_ref`.
`gate` defaults to `regressions`. `artifact_name` defaults to
`knowledge-eval-report`.

The workflow builds the current CLI and runs JSON and Markdown comparisons.
It adds the Markdown report to the GitHub job summary. It does not create a
pull request comment.

For a pull request, `base_ref` uses the pull request base SHA. For a push to
`main`, it uses `github.event.before`. The push run attaches the successful
check to the production commit.

The reusable job produces the exact check name
`knowledge-eval / Evaluate knowledge changes` in this repository. Configure
that name in `github.required_checks` to gate runtime publication.

The workflow always uploads `knowledge-eval.json` and `knowledge-eval.md` when
the files exist. The artifact retention period is 14 days. A failed gate keeps
the reports available and fails the workflow job.

Hosted maintenance jobs that create a pull request also copy their comparison
into `.openknowledge/reports/<run-id>/`. The directory contains `index.md`,
`eval.md`, `eval.json`, and `artifact.json`. The index and human report are
ordinary Markdown, so `okn view` can display them. The manifest follows the
durable artifact v1 contract. These files are committed with the proposed
knowledge change and remain available after an ephemeral worker exits.

The workflow has read-only repository permission and disables telemetry. It
does not configure an answer command. Use datasets without answer expectations.

| Exit status | Meaning |
| --- | --- |
| `0` | The selected single-run or comparison gate passed. |
| `1` | The selected gate failed, or the evaluation could not run. |
| `2` | Command usage or dataset validation failed. |

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/eval_command.go`
> - `packages/cli/internal/eval/`
> - `packages/cli/schemas/eval/v1/dataset.schema.json`
> - `packages/cli/schemas/eval/v1/answer-request.schema.json`
> - `packages/cli/schemas/eval/v1/answer-response.schema.json`
> - `packages/cli/schemas/v1/eval-report.schema.json`
> - `packages/cli/schemas/v1/eval-comparison.schema.json`
> - `.github/workflows/ci.yml`
> - `.github/workflows/knowledge-eval.yml`
>
> **Update notes**
>
> Update this page when the eval dataset, checks, output, or exit status changes.
