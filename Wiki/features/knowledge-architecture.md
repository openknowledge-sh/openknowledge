---
type: Feature Documentation
title: Knowledge Architecture
description: Canonical OKF source and shipped retrieval projections.
tags: [openknowledge, architecture, retrieval, graph, search]
timestamp: 2026-08-12T00:00:00Z
---

# Knowledge Architecture

An OKF Markdown bundle is the canonical knowledge corpus.
People and agents edit this source.
Indexes, graphs, exports, and runtime generations are rebuildable projections.

```mermaid
flowchart LR
  OKF["OKF Markdown and YAML"] --> AST["Validated AST and metadata"]
  AST --> Facts["Normalized semantic facts"]
  AST --> Search["BM25 and local vector indexes"]
  Facts --> RDF["RDF and bounded SPARQL"]
  Facts --> Rules["Mangle Datalog and proofs"]
  AST --> Graphs["Source and search graphs"]
  Search --> Hybrid["Explicit hybrid orchestration"]
  RDF --> Hybrid
  Rules --> Hybrid
  AST --> Publish["JSON, HTML, RDF, tar, MCP, runtime"]
```

Code and tests define the CLI behavior.
The Wiki is the canonical CLI reference.
The README is the product overview.

## Shipped layers

| Layer | Current behavior |
| --- | --- |
| Source | Filesystem-based Markdown concepts with YAML frontmatter and authored links. |
| Metadata | One normalized semantic fact model for typed claim occurrences, ontology terms, provenance, evidence, relations, lifecycle, and access. |
| Retrieval | Field-weighted section BM25, pluggable embeddings, metadata filters, deterministic reranking, one-hop link expansion, federated rank fusion, and token-budgeted context. |
| Semantic query | Lossless RDF 1.1 projection with bounded SPARQL `SELECT`/`ASK`; safe recursive Mangle Datalog with source-backed proof paths. |
| Hybrid | Intent-explicit BM25/vector/SPARQL/Datalog routes, reciprocal-rank fusion, lifecycle policy, and source joins. |
| Graphs | Structural graphs plus typed claim declaration and dependency edges; deterministic revision-named RDF graph. |
| Delivery | CLI, Go API, local/static viewer, read-only MCP, portable exports, and immutable runtime generations. |
| Maintenance | Agent setup, insights, validation, isolated jobs, and source-controlled review. |

Retrieval builds deterministic in-memory inverted and vector indexes. The
default vector provider uses hashed word and character features without a
model or network service. The Go API can supply another embedding provider.
Hybrid CLI queries can use an OpenAI-compatible HTTP endpoint. Vectors use
normalized `float32` values. The optional on-disk cache uses the exact input
digest and model fingerprint. Exact lookup uses postings. Prefix lookup uses a
vocabulary range. Fuzzy lookup can scan the vocabulary.

The graph layer contains claim occurrences and explicit declaration, reference,
supersession, contradiction, and derivation edges. The claim registry provides
entity and predicate identity. The semantic fact layer projects these same
assertions to RDF and Datalog without making the projections canonical.
SPARQL handles graph-pattern queries. Mangle handles safe recursive rules and
keeps asserted and derived facts distinct. The default profile does not apply
closed-world negation; callers must select that profile explicitly.

Search supports `type` and `tag` metadata filters. It combines lexical and
local vector candidates with a deterministic reranker. Hybrid query does not
infer a query plan from prose. It runs only caller-supplied text, SPARQL, and
Datalog routes, then fuses independent ranks and joins provenance back to
source sections. It does not ship a cross-encoder reranker, remote embedding
model, general OWL reasoner, or automatic GraphRAG reasoning agent.

The optional corpus schema validates allowed document types, paths, required
metadata, typed local link predicates, and migration records. Stable claim
entity IDs remain independent of filenames. Entity search and proposals are
deterministic control-plane tooling; they are not an automatic entity resolver.

## Maintenance and publication loop

```mermaid
flowchart LR
  Usage["Private usage events"] --> Insight["Stable gap insight"]
  Insight --> Job["Isolated job"]
  Job --> PR["Draft pull request"]
  PR --> Eval["Eval and GitHub checks"]
  Eval --> Approval["Approval and merge"]
  Approval --> Gate["Production required-check gate"]
  Gate --> Generation["Check-bound generation"]
  Generation --> Usage
```

`insights from-usage` converts recurring retrieval gaps into stable insights.
An isolated job can research an insight and propose a draft pull request.

The worker reports bounded before and proposed eval pass counts. The
publisher independently validates OKF, publication rules, and claim lifecycle
history before GitHub review.

The pull request lists claim changes, evidence IDs, document impact, eval
impact, and required human decisions.

An extracted, proposed, supported, or disputed claim disables low-risk
auto-merge. Rejection, supersession, or archival of a verified claim also
disables auto-merge.

GitHub approval and branch protection control the merge. After merge, the
publisher verifies required checks on the production commit. It then creates
the runtime generation and starts the next local usage cycle.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/context.go`
> - `packages/cli/internal/okf/context_types.go`
> - `packages/cli/internal/okf/search_knowledge.go`
> - `packages/cli/internal/okf/semantic_facts.go`
> - `packages/cli/internal/okf/embedding.go`
> - `packages/cli/internal/okf/embedding_http.go`
> - `packages/cli/internal/okf/rdf.go`
> - `packages/cli/internal/okf/sparql.go`
> - `packages/cli/internal/okf/datalog.go`
> - `packages/cli/internal/okf/hybrid.go`
> - `packages/cli/internal/okf/graph.go`
> - `packages/cli/internal/okf/ast_bundle_parse.go`
> - `packages/cli/internal/usage/`
> - `packages/cli/internal/insights/`
> - `packages/cli/internal/agents/`
> - `packages/cli/internal/runtime/generation.go`
> - `packages/cli/cmd/openknowledge/runtime_worker.go`
> - `.github/workflows/ci.yml`
>
> **Update notes**
>
> Update this page after a change to shipped retrieval or graph behavior.
