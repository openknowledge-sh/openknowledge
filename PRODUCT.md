# Open Knowledge Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

The primary user is a technical team already deploying AI agents whose
answers depend on rapidly changing internal knowledge. The product serves
three roles:

- agent builders who need a safe knowledge runtime;
- knowledge owners and experts who should review only decisions that need
  human judgment;
- AI agents that need current, permitted, source-grounded context.

## Product Purpose

Open Knowledge is the CI and runtime for agent knowledge. It turns Markdown
knowledge into an open, versioned, testable system that can detect concrete
risks, propose evidence-backed maintenance, verify answer impact, and serve a
trusted release to any agent.

Success means the maintenance loop closes: real usage exposes a gap or risk,
maintenance produces an attributable change, evals establish its impact, the
runtime publishes it safely, and subsequent usage shows whether the outcome
improved.

## Positioning

Open Knowledge does more than index existing content. It actively improves the
knowledge base while keeping the source readable, Git-native, model-agnostic,
and portable. Its differentiator is one lifecycle spanning repository,
continuous audit, agentic maintenance, Knowledge CI, runtime retrieval, and
usage-grounded observability.

## Operating Context

Teams work locally with Markdown and the Open Knowledge Format, review changes
through Git and pull requests, run deterministic validation and evals in CI,
publish immutable runtime generations, and retrieve knowledge over HTTP or
MCP. Usage, refusal, and grounded feedback events remain private operational
data unless a team explicitly exports a report.

## Capabilities and Constraints

- Every concept can carry provenance, ownership, freshness, lifecycle, trust,
  relationships, and history.
- Every automated change must preserve its reason, evidence, diff, confidence,
  eval result, risk, and approval boundary.
- Every runtime use must remain attributable to an immutable generation and
  selected evidence.
- Low-risk deterministic maintenance may be automated; medium-risk changes
  require human approval; high-risk changes remain evidence-only until an
  expert decides.
- Runtime retrieval is permission-aware and can refuse when policy-compliant
  evidence is unavailable.
- Quality reporting exposes concrete metrics and evidence, never a single
  opaque global score.
- The product does not aim to replace general-purpose wikis or chat products,
  and it does not introduce a proprietary knowledge format.

## Evidence on Hand

The authoritative product brief for this record was supplied in the Codex
task on 2026-08-21. Repository code, strict schemas, tests, and `Wiki/` provide
implementation evidence. No customer claims, commercial benchmarks,
testimonials, or brand image assets were supplied and future surfaces must not
fabricate them.

## Product Principles

1. Keep knowledge open, readable, Git-native, and vendor-independent.
2. Show concrete problems, impact, and evidence instead of magic scores.
3. Test knowledge changes by their effect on agent answers before publication.
4. Automate mechanical work while preserving explicit human judgment
   boundaries.
5. Improve the knowledge base from real usage and attributable outcomes.

## Accessibility & Inclusion

Human-facing local and exported web surfaces should remain keyboard operable,
screen-reader legible, responsive, and printable without requiring network
assets or client-side execution for core content.
