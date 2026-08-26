# Open Knowledge Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

The entry user is a technical team that wants searchable, maintainable product
and codebase documentation without adopting a separate knowledge platform.
Teams can add trust and runtime capabilities when their sources, scale, or
agent workflows require them. The product serves four roles:

- engineers and documentation owners maintaining a shared codebase view;
- agent builders who need a safe knowledge runtime;
- knowledge owners and experts who should review only decisions that need
  human judgment;
- AI agents that need current, permitted, source-grounded context.

## Product Purpose

Open Knowledge starts as searchable, Git-native Markdown documentation and can
grow into CI and a runtime for agent knowledge. The same bundle can add
concrete risk detection, evidence-backed maintenance, answer verification, and
trusted releases without a format migration.

Success at the entry layer means a user can create useful documentation,
validate it, and retrieve a relevant answer before choosing integrations or
governance concepts. At the trusted layer, success means the maintenance loop
closes: real usage exposes a gap or risk, maintenance produces an attributable
change, evals establish its impact, the runtime publishes it safely, and
subsequent usage shows whether the outcome improved.

## Positioning

Open Knowledge uses progressive disclosure: ordinary documentation needs only
Markdown, validation, search, and optional browsing. The same knowledge base
can later add provenance, typed claims, continuous audit, agentic maintenance,
Knowledge CI, runtime retrieval, and usage-grounded observability. This
cumulative lifecycle keeps the source readable, Git-native, model-agnostic,
and portable.

## Operating Context

Teams begin locally with Markdown and the Open Knowledge Format. Lightweight
bundles can remain local and use Git review, validation, search, and the viewer.
Teams that need stronger assurance can add deterministic evals in CI, publish
immutable runtime generations, and retrieve knowledge over HTTP or MCP. Usage,
refusal, and grounded feedback events remain private operational data unless a
team explicitly exports a report.

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
2. Deliver a useful local result before asking users to configure optional
   trust, integration, or production layers.
3. Show concrete problems, impact, and evidence instead of magic scores.
4. Test knowledge changes by their effect on agent answers before publication.
5. Automate mechanical work while preserving explicit human judgment
   boundaries.
6. Improve the knowledge base from real usage and attributable outcomes.

## Accessibility & Inclusion

Human-facing local and exported web surfaces should remain keyboard operable,
screen-reader legible, responsive, and printable without requiring network
assets or client-side execution for core content.
