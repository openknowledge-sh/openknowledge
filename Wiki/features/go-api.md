---
type: Feature Documentation
title: Go API
description: Embed the read-only Open Knowledge parser, validation, retrieval, and graph core.
tags: [openknowledge, go, api, sdk, integration]
timestamp: 2026-07-18T00:00:00Z
---

# Go API

Import the supported read-only facade used by the CLI:

```go
import "github.com/openknowledge-sh/openknowledge/packages/cli/okf"
```

## Example

```go
report, err := okf.ValidateWithVersion("./Wiki", "0.2")
if err != nil {
    return err
}
if err := okf.RequireValidBundle(report); err != nil {
    return err
}

packet, err := okf.ResolveContextWithVersion(
    "./Wiki",
    "0.2",
    okf.ContextOptions{Query: "release workflow", Budget: 1200, Limit: 8},
)
```

## Surface

The package exposes these functions and models:

- AST and normalized bundle parsing
- validation policies, known rules, and the valid-bundle gate
- inventory and bundle metadata
- deterministic search, bounded context, and caller-supplied RRF federation
- source and retrieval graphs
- OKF 0.2 derived trust, lifecycle, source, and Attested Computation signals
- strict frontmatter and portable-manifest decoding
- supported spec discovery and the embedded spec
- bounded registry inventory, key or path resolution, and authoring capability

Returned types alias the core models. Therefore, Go results, CLI JSON, MCP
structured content, and published schemas use the same field definitions.
Retrieval results include corpus revisions and content-addressed locators.
Use these values to detect stale evidence.

Functions without a version use `LatestSpecVersion`. For persistent
integrations, prefer `WithVersion` functions. Store `SpecVersion`,
`SchemaVersion`, the retrieval revision, and the selected locators. See
[Machine-readable contracts](machine-contracts.md) for more information.

Validation rule discovery and override parsing follow the same contract. Use
`KnownValidationRulesForVersion`, `IsKnownValidationRuleForVersion`,
`ParseValidationRuleOverrideForVersion`, and
`SetValidationRuleSeverityForVersion` when selecting a spec explicitly.
The known-rule catalog includes fixed-severity rules;
`IsValidationRuleOverrideableForVersion` distinguishes rules that accept
policy overrides. `ValidationOptions` can contain configurable rules from
multiple profiles. Validation applies only options from its selected profile.
The unversioned helpers use `LatestSpecVersion`.

The supported spec versions are `0.1` and `0.2`. `LatestSpecVersion` is
`0.2`. The 0.2 AST normalizes a bare `verified` mapping to a one-item list.
Use `DeriveOKFV02Signals` to derive the same `okf02` contract used by list and
graph output. `OKFV02SourceFootnotes` maps source IDs to stable HTML anchors.
Signal derivation does not execute computation, executor, or attester
resources.

The API is read-only. It does not connect sources, refresh sources, or change
the registry. It does not extract archives, render HTML, or manage processes.
Registry reads do not migrate or rewrite local storage.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/okf/doc.go`
> - `packages/cli/okf/types.go`
> - `packages/cli/okf/read.go`
> - `packages/cli/okf/registry.go`
> - `packages/cli/okf/read_test.go`
