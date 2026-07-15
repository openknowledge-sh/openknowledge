---
type: Command Documentation
title: openknowledge mcp
description: Serves one Open Knowledge bundle as read-only MCP resources and tools over stdio.
tags: [openknowledge, cli, command, mcp, llm, retrieval]
timestamp: 2026-07-15T00:00:00Z
---

# `openknowledge mcp`

`openknowledge mcp` exposes one local or connected Open Knowledge bundle to an
MCP client. It is the direct integration surface for agents and LLM hosts that
support the Model Context Protocol: clients can discover and read exact bundle
files as resources, build source-grounded context through a search tool, and
inspect bundle health through a validation tool.

The server is read-only. It does not expose authoring, connection, refresh,
export, viewer, or agent-job execution operations.

## Usage

```sh
openknowledge mcp [key-or-path]
openknowledge mcp --spec <version> [key-or-path]
openknowledge mcp --help
```

The target defaults to the current directory. A registry key resolves through
the same local registry as `get`, `search`, and `registry where`. The selected
path is resolved to its real directory before the protocol starts, so the
server remains scoped to that one knowledge base for its lifetime.

For an MCP host that accepts command-based server configuration, use the
equivalent of:

```json
{
  "mcpServers": {
    "project-knowledge": {
      "command": "openknowledge",
      "args": ["mcp", "personal"]
    }
  }
}
```

The exact configuration file and property names belong to the MCP host. The
server itself does not read client-specific configuration.

## Transport And Lifecycle

The command implements the stable MCP `2025-11-25` protocol over stdio. Each
UTF-8 JSON-RPC 2.0 message occupies one line, as required by the standard
stdio transport. Standard output is reserved exclusively for protocol
messages; startup and terminal errors go to standard error.

The client must send `initialize`, receive the server capabilities, then send
`notifications/initialized` before using resources or tools. `ping` is
available during initialization. The server negotiates the released protocol
versions `2024-11-05`, `2025-03-26`, `2025-06-18`, and `2025-11-25`; an
unknown requested version receives the latest supported version so the client
can decide whether to disconnect.

The server advertises `resources` and `tools` capabilities. It does not claim
resource subscriptions, list-change notifications, prompts, sampling,
elicitation, logging, or asynchronous tasks.

Protocol behavior follows the official MCP
[lifecycle](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle),
[stdio transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports),
[resources](https://modelcontextprotocol.io/specification/2025-11-25/server/resources),
and [tools](https://modelcontextprotocol.io/specification/2025-11-25/server/tools)
contracts.

## Resources

`resources/list` returns the regular files in the canonical bundle inventory,
including Markdown documents and non-Markdown assets. Results are sorted by
bundle-relative path, returned in pages of at most 100, and use an opaque
`nextCursor` when another page exists.

Each resource uses an `openknowledge://bundle/...` URI and includes its
bundle-relative name, known title and description, MIME type, and last-modified
annotation. Non-empty resources also include byte size; the JSON field is
omitted for empty files. Markdown is reported as `text/markdown`.

`resources/read` returns UTF-8 text for textual MIME types and base64 `blob`
content for binary files. Reads have these boundaries:

* the URI must use the canonical `openknowledge://bundle/` scheme and form
* the path must appear in the public bundle inventory
* traversal outside the selected root and symbolic links below it are rejected
* the target must still be a regular file
* one resource may contain at most 4 MiB

Files omitted from inventory, including `.git` contents, cannot be fetched by
guessing their URI.

`resources/templates/list` returns an empty template list because all exposed
resources are concrete bundle files.

## Tools

| Tool | Arguments | Result |
| --- | --- | --- |
| `openknowledge_search` | Required `query`; optional `budget`, `limit`, and `noExpand`. | The same machine-readable, budget-bounded context model as `openknowledge search --format json`, plus a JSON text block for older clients. |
| `openknowledge_validate` | No arguments. | The complete machine-readable validation report, plus a JSON text block for older clients. Validation findings are data, not a failed tool call. |

Both tools declare MCP read-only, non-destructive, idempotent, and closed-world
annotations. Search defaults to a 2,400-token budget and 12 sources. It limits
the requested budget to 32,000 tokens, the source count to 50, and the query to
4,096 Unicode characters. Unknown arguments fail strict input validation.

Operational failures are returned as MCP tool results with `isError: true`, so
the client can distinguish a valid protocol exchange from a failed operation.
Unknown tools and invalid arguments return JSON-RPC invalid-params errors.

## Resource Limits And Errors

The server processes requests sequentially and bounds one incoming protocol
line to 1 MiB. Oversized messages terminate the stream after a JSON-RPC parse
error. It rejects JSON-RPC batches, null or fractional request IDs, operations
before the initialized notification, duplicate initialization, malformed
parameters, invalid cursors, unknown methods, and missing resources with
standard or protocol-appropriate error codes.

Closing stdin ends the server normally. A transport read or write failure exits
with status `1`; invalid command arguments or an unsupported OKF spec exit with
status `2` before the protocol starts.

## Command Change History

* `2026-07-15`: Added the read-only MCP stdio server with lifecycle and version
  negotiation, exact paginated resources, source-grounded search, validation,
  strict schemas, canonical root confinement, and bounded messages and reads.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/mcp.go`
> * `packages/cli/cmd/openknowledge/mcp_test.go`
> * `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> Update this page when MCP lifecycle, resource metadata, tools, size limits,
> protocol behavior, or command flags change.
