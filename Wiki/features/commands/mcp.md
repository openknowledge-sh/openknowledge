---
type: Command Documentation
title: openknowledge mcp
description: Serve one knowledge base as read-only MCP resources and tools.
tags: [openknowledge, cli, command, mcp, llm, retrieval]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge mcp`

Expose one local or connected knowledge base to an MCP client. The server is
read-only. Clients can list and read files. They can search, run explicit
semantic queries, and validate the bundle.

## Usage

```sh
okn mcp [key-or-path]
okn mcp Wiki --spec 0.2
```

Example command-based client configuration:

```json
{
  "mcpServers": {
    "project-knowledge": {
      "command": "okn",
      "args": ["mcp", "Wiki"]
    }
  }
}
```

The default target is the current directory. The target does not change during
the server lifetime.

## Protocol

The command implements MCP `2025-11-25` over stdio. It negotiates released
versions back to `2024-11-05`. Each JSON-RPC message uses one UTF-8 line.
Stdout contains only protocol messages.

The client must complete `initialize` and `notifications/initialized` before it
uses resources or tools.

The server advertises resources and tools. It does not advertise prompts,
sampling, subscriptions, elicitation, logging, or asynchronous tasks.

## Resources

`resources/list` returns the bundle inventory in path order. One page contains
a maximum of 100 items. Resource URIs use `openknowledge://bundle/...`.
`resources/read` returns text for textual MIME types. It returns base64 data for
binary files.

Reads can access only regular files in the inventory. The server rejects
traversal, symlinks, guessed paths, and resources larger than 4 MiB.

Direct stdio MCP inventories all regular bundle files except `.git`. It does
not apply publication or `okf_targets.mcp` filters. Only deployed HTTP MCP
reads the filtered `mcp/` projection.

`resources/templates/list` is empty because each resource is concrete.

## Tools

| Tool | Arguments | Result |
| --- | --- | --- |
| `openknowledge_search` | Required `query`. Optional `budget`, `limit`, `noExpand`, and `filters`. | The same v1 context model as `search --format json`. |
| `openknowledge_query` | Optional `text`, `sparql`, `datalog`, and `limit`. At least one query is required. | The v1 hybrid query result with retrieved text, asserted facts, derived facts, provenance, and proofs. |
| `openknowledge_validate` | none | The complete v1 validation report. |
| `openknowledge_claims_find` | Required `query`. | Ranked existing claim IDs and occurrences. |
| `openknowledge_claims_stale` | none | Exact stale claim occurrences and stale evidence IDs. |
| `openknowledge_claims_impact` | Required `claimId`. | Sources, documents, dependencies, and eval cases. |
| `openknowledge_claims_propose` | Document, ID, value, source, reason, confidence, and optional scope data. | A digest-bound `proposed` claim document. |

The claim proposal tool does not edit the knowledge base. Apply its output
with [`okn claims apply`](claims.md) after review.

Search sources include optional typed claim metadata. A `section_ref` claim
appears only in its bound retrieval section. Document-wide claims and
`claim_refs` remain available in all document sections.

Search defaults to 2,400 estimated tokens and 12 sources. Its maximums are
32,000 tokens, 50 sources, and 4,096 query characters. Tool arguments are
strict.

Semantic query defaults to 12 fused results and accepts at most 50 over MCP. Text,
SPARQL, Datalog atoms, and rule programs have separate byte limits. The tool
runs only the supplied routes. Direct MCP clients cannot self-issue access
grants, so structured evaluation is public-only and fails closed for restricted
sources. See [`okn query`](query.md) for the language profiles and result model.

`filters.types` and `filters.tags` restrict candidates before retrieval. Values
in one list use OR matching. The two lists use AND matching.

Validation findings are data. An operational failure returns `isError`.

One incoming message has a 1 MiB limit. The server processes requests in
sequence. A malformed batch, invalid ID, lifecycle violation, bad cursor, or
unknown method returns a protocol error. When you close stdin, the server
stops normally.

## Deployed HTTP MCP

[`okn automation runtime serve`](runtime.md) exposes the same read-only surface
at `<route>/_mcp`. It uses MCP sessions over HTTP. It reads only the filtered
`mcp/` projection.

Access can be public or bearer-token protected. Omit `mcp` from the bundle's
`release.outputs` to disable the endpoint. The runtime validates browser
origins. It limits bodies, sessions, concurrency, and request duration. Apply
rate limits at the trusted ingress.

Runtime retrieval rejects sections with applicable enforced claims that are
not verified. Claim trust and freshness also contribute to policy checks.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/mcp.go`
> - `packages/cli/cmd/openknowledge/mcp_test.go`
> - `packages/cli/cmd/openknowledge/runtime_serve.go`
>
> **Update notes**
>
> Update this page when MCP versions, lifecycle, resources, tools, limits, or
> transports change.
