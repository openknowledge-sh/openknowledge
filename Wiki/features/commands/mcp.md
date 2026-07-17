---
type: Command Documentation
title: openknowledge mcp
description: Serve one knowledge base as read-only MCP resources and tools.
tags: [openknowledge, cli, command, mcp, llm, retrieval]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge mcp`

Expose one local or connected knowledge base to an MCP client. The server is
read-only: clients can list and read files, search knowledge, and validate the
bundle.

## Usage

```sh
openknowledge mcp [key-or-path]
openknowledge mcp Wiki --spec 0.1
```

Example command-based client configuration:

```json
{
  "mcpServers": {
    "project-knowledge": {
      "command": "openknowledge",
      "args": ["mcp", "Wiki"]
    }
  }
}
```

The target defaults to the current directory and remains fixed for the server
lifetime.

## Protocol

The command implements MCP `2025-11-25` over stdio and negotiates released
versions back to `2024-11-05`. Each JSON-RPC message occupies one UTF-8 line;
stdout is reserved for protocol messages. The client must complete
`initialize` and `notifications/initialized` before using resources or tools.

The server advertises resources and tools. It does not advertise prompts,
sampling, subscriptions, elicitation, logging, or asynchronous tasks.

## Resources

`resources/list` returns the canonical bundle inventory in path order, up to
100 items per page. Resource URIs use `openknowledge://bundle/...`.
`resources/read` returns text for textual MIME types and base64 blobs for binary
files.

Reads are limited to regular files already present in the inventory. Traversal,
symlinks, guessed paths outside that inventory, and resources larger than 4 MiB
are rejected. Direct stdio MCP inventories all regular bundle files except
`.git`; it does not apply publication or `okf_targets.mcp` filtering. Only the
deployed HTTP surface reads the filtered `mcp/` projection.
`resources/templates/list` is empty because every resource is concrete.

## Tools

| Tool | Arguments | Result |
| --- | --- | --- |
| `openknowledge_search` | required `query`; optional `budget`, `limit`, `noExpand` | The same v1 context model as `search --format json`. |
| `openknowledge_validate` | none | The complete v1 validation report. |

Search defaults to 2,400 estimated tokens and 12 sources, with maximums of
32,000 tokens, 50 sources, and 4,096 query characters. Tool arguments are
strict. Validation findings are data; operational failures return `isError`.

One incoming message is limited to 1 MiB. Requests are processed sequentially.
Malformed batches, invalid IDs, lifecycle violations, bad cursors, and unknown
methods return protocol errors. Closing stdin exits normally.

## Deployed HTTP MCP

[`openknowledge runtime serve`](runtime.md) exposes the same read-only surface
at `<route>/_mcp` using MCP sessions over HTTP. It reads only the filtered
`mcp/` projection and supports public, bearer-token, or disabled access. The
runtime validates browser origins, limits bodies, sessions, concurrency, and
request duration; rate limiting belongs at the trusted ingress.

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
