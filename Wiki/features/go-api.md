---
type: Feature Documentation
title: Go API
description: Public read-only Go package for embedding Open Knowledge parsing, validation, retrieval, and graph views.
tags: [openknowledge, go, api, sdk, integration]
timestamp: 2026-07-15T00:00:00Z
---

# Go API

Go applications can import a supported read-only facade over the exact core
used by the CLI:

```go
import "github.com/openknowledge-sh/openknowledge/packages/cli/okf"
```

Before this package existed, the implementation lived only under Go's
`internal/` boundary. External tools therefore had to launch a subprocess and
decode CLI JSON even when they needed in-process parsing or retrieval.

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

* AST and normalized-bundle parsing
* validation, validation policy options, known rules, and the valid-bundle gate
* inventory listing and bundle metadata
* deterministic match search and budget-bounded source context
* source and retrieval graph construction
* strict frontmatter and portable-manifest decoding
* supported spec discovery and the pinned spec document

Returned structures are aliases of the core models, not copied SDK-specific
models. CLI behavior, public Go results, JSON schemas, and MCP structured
content therefore share one implementation and one set of JSON field tags.

## Versioning

Calls without an explicit version use `LatestSpecVersion`. Integrations that
persist data should prefer `WithVersion` functions and record both
`SpecVersion` and `SchemaVersion` from returned models.

The Go module is pre-v1, so source compatibility follows module semantic
versioning. Serialized compatibility is a separate contract documented under
[machine-readable contracts](machine-contracts.md). The portable manifest also
has its own independent protocol version and schema.

## Boundary

The public package is intentionally read-only. Registry mutation, network
materialization and refresh, archive extraction, HTML/viewer generation, and
process lifecycle remain operational CLI responsibilities. This keeps an
embedded parser or retrieval service from silently mutating user registry or
cache state.

## Verification

An external-package test imports only the public path and exercises validation,
AST and normalized parsing, inventory, search, context, graph, metadata,
validation options, strict manifest decoding, and spec discovery against a real
temporary bundle.

## Source Anchors

* `packages/cli/okf/doc.go`
* `packages/cli/okf/types.go`
* `packages/cli/okf/read.go`
* `packages/cli/okf/read_test.go`
* `packages/cli/okf/README.md`
* `packages/cli/internal/okf/`
