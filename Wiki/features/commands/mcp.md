---
type: Command Documentation
title: openknowledge mcp
description: Serve one knowledge base as read-only MCP resources and tools.
tags: [openknowledge, cli, command, mcp, llm, retrieval]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge mcp`

Expose one local or connected knowledge base to an MCP client. The server is
read-only. Clients can list and read files. They can also search knowledge and
validate the bundle.

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
| `openknowledge_search` | Required `query`. Optional `budget`, `limit`, `noExpand`. | The same v1 context model as `search --format json`. |
| `openknowledge_validate` | none | The complete v1 validation report. |

Search defaults to 2,400 estimated tokens and 12 sources. Its maximums are
32,000 tokens, 50 sources, and 4,096 query characters. Tool arguments are
strict.

Validation findings are data. An operational failure returns `isError`.

One incoming message has a 1 MiB limit. The server processes requests in
sequence. A malformed batch, invalid ID, lifecycle violation, bad cursor, or
unknown method returns a protocol error. When you close stdin, the server
stops normally.

## Deployed HTTP MCP

[`openknowledge runtime serve`](runtime.md) exposes the same read-only surface
at `<route>/_mcp`. It uses MCP sessions over HTTP. It reads only the filtered
`mcp/` projection.

Access can be public, bearer-token, or disabled. The runtime validates browser
origins. It limits bodies, sessions, concurrency, and request duration. Apply
rate limits at the trusted ingress.

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
