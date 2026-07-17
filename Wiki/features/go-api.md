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
report, err := okf.ValidateWithVersion("./Wiki", "0.1")
if err != nil {
    return err
}
if err := okf.RequireValidBundle(report); err != nil {
    return err
}

packet, err := okf.ResolveContextWithVersion(
    "./Wiki",
    "0.1",
    okf.ContextOptions{Query: "release workflow", Budget: 1200, Limit: 8},
)
```

## Surface

The package exposes:

- AST and normalized bundle parsing;
- validation policies, known rules, and the valid-bundle gate;
- inventory and bundle metadata;
- deterministic search, bounded context, and caller-supplied RRF federation;
- source and retrieval graphs;
- strict frontmatter and portable-manifest decoding;
- supported spec discovery and the embedded spec;
- bounded registry inventory, key/path resolution, and authoring capability.

Returned types alias the core models, so Go results, CLI JSON, MCP structured
content, and published schemas share field definitions. Retrieval results
include corpus revisions and content-addressed locators for stale-evidence
detection.

Functions without a version use `LatestSpecVersion`. Persisting integrations
should prefer `WithVersion` functions and store `SpecVersion`, `SchemaVersion`,
retrieval revision, and selected locators. See
[Machine-readable contracts](machine-contracts.md).

The API is intentionally read-only. It does not connect, refresh, mutate the
registry, extract archives, render HTML, or manage processes. Registry reads
never migrate or rewrite local storage.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/okf/doc.go`
> - `packages/cli/okf/types.go`
> - `packages/cli/okf/read.go`
> - `packages/cli/okf/registry.go`
> - `packages/cli/okf/read_test.go`
