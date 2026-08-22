---
type: Feature Documentation
title: Knowledge CI Golden Path
description: Install, audit, fix, test, and publish agent knowledge from one Git repository.
tags: [openknowledge, golden-path, ci, mcp, lifecycle]
timestamp: 2026-08-22T00:00:00Z
---

# Knowledge CI Golden Path

Open Knowledge is the CI and runtime for agent knowledge. GitHub is the review
interface, Git is the audit log and rollback system, Markdown is the source of
record, and MCP distributes the last passing production generation.

## Command path

```sh
# Local knowledge and agent access
okn setup Wiki

# Recommended GitHub lifecycle
okn setup ci Wiki

# Find concrete risks and preserve one as agent work
okn audit Wiki --baseline .openknowledge/audit-sources.json \
  --format json --out .openknowledge/reports/audit.json
okn audit propose <finding-id> \
  --report .openknowledge/reports/audit.json --path Wiki

# Capture exact source bytes before a selector relies on them
okn evidence pin --document runbook.md --source policy --path Wiki

# Test the proposed change against the Git base
okn eval run .openknowledge/evals/knowledge.yaml Wiki \
  --base main --gate regressions --format markdown

# Configure production MCP and viewer publication
okn setup runtime Wiki
```

There is no broad `fix` command. A finding becomes one durable insight proposal.
The configured agent prepares a Git diff or pull request. It uses `okn claims`
for source-backed facts and lifecycle changes. This keeps semantic decisions in
the normal Git review workflow.

## What each layer guarantees

| Layer | Responsibility |
| --- | --- |
| Agent | Extract candidates, reuse IDs, link evidence, prepare Markdown changes, and explain impact. |
| Deterministic CLI | Validate structure, provenance, claim identity, lifecycle history, conflicts, source changes, and eval regressions. |
| Human or executable evidence | Decide meaning, authority, risky procedures, and unresolved conflicts. |
| GitHub | Show the diff and reports, enforce required checks, and record approval. |
| Production runtime | Publish immutable green generations, refuse insufficient evidence, and support rollback. |

An agent can create a `proposed` claim or preserve a disagreement as
`disputed`. It cannot silently remove verified history or make a new source
authoritative. A base-aware CI gate requires an approval identity for authority
changes. Hosted maintenance still routes every authority change to human review.

## Local and production access

`okn mcp` serves the current working tree. Agents need this mode while they edit
and test local knowledge.

The production runtime serves only its active immutable generation. Publication
requires configured GitHub checks. By default, it also rejects active proposed
or disputed claims. A failed publication leaves the previous green generation
active. `runtime rollback` can select an earlier verified generation.

When GitHub Actions owns maintenance, production requires its `knowledge-ci`
check. When the runtime owns maintenance, it performs the same structure,
claim-history, audit, and eval gates before publication and stores the reports
in runtime state. Only one maintenance executor is enabled.

## Evidence and source observation

Open Knowledge does not need a product-specific source connector. A source is a
declared resource with provenance. Local resources are content-fingerprinted.
Remote resources can remain manual, use HTTP metadata, fetch bounded content,
or use a pinned SHA-256. Network observation requires both source opt-in and the
`--observe-remote` command flag.

Open Knowledge reports a disagreement when evidence is insufficient. It does
not select the newest or most convenient source as truth.

## Acceptance proof

The CLI test suite contains one Golden Path acceptance test. It creates a Git
documentation repository, installs Knowledge CI, detects a changed authoritative
source, pins exact evidence, creates a durable finding proposal, applies and
verifies an evidence-backed claim, and runs answer plus unknown-question
abstention evals. It publishes a green immutable generation, activates it,
queries HTTP and MCP, verifies the generation-bound evidence bundle, records
grounded feedback, publishes a second generation, and rolls back to the first.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/golden_path_test.go`
> - `packages/cli/cmd/openknowledge/setup_product_command.go`
> - `packages/cli/cmd/openknowledge/audit_command.go`
> - `packages/cli/cmd/openknowledge/claims_command.go`
> - `packages/cli/cmd/openknowledge/runtime_command.go`
>
> **Update notes**
>
> Update this page when the end-to-end product contract or command path changes.
