---
type: Feature
title: Browser WebMCP
description: Expose viewer page tools to compatible browser agents through WebMCP.
tags: [openknowledge, viewer, exporter, webmcp]
timestamp: 2026-10-02T00:00:00Z
---

# Browser WebMCP

Use WebMCP to let a compatible browser agent list, search, read, and open pages in the current Open Knowledge viewer.
The local viewer also provides shared Markdown draft tools for writable knowledge bases.

## Use

Start the local viewer:

```sh
okn view Wiki
```

Alternatively, export the interactive viewer:

```sh
okn export html --out /tmp/openknowledge-site Wiki
```

Serve exported files over HTTPS or a local development server.
Use a browser with WebMCP support in a secure context, such as HTTPS or localhost.
The viewer registers tools when the browser provides `document.modelContext.registerTool`.
It also accepts the earlier `navigator.modelContext.registerTool` interface.
Unsupported browsers retain the ordinary viewer without these tools.

WebMCP uses the open browser page. It requires no Open Knowledge plugin installation, API key, or HTTP MCP endpoint.
The browser agent must support tool discovery and execution.
Browser availability and access controls follow the
[Chrome WebMCP API](https://developer.chrome.com/docs/ai/webmcp/imperative-api).

## Development

From the repository root, start the Vite development viewer:

```sh
pnpm dev:viewer --no-browser Wiki
```

Open the printed **Open Knowledge viewer development** URL.
Edit modules in `packages/web/src/viewer` to update the open browser page through Vite.
JavaScript changes can reload the page. Static HTML exports require a new build to include changes.
The development proxy streams source-change events to the viewer and preserves active drafts during source refreshes.
This workflow requires no OpenAI-specific SDK or development environment.

## Tools

| Tool | Arguments | Result |
| --- | --- | --- |
| `openknowledge_list_pages` | Optional `knowledgeBase`, `offset` (default `0`), and `limit` (default `50`, maximum `100`). | Page catalog with paths and URLs. |
| `openknowledge_search` | `query` and optional `limit` (default `12`, maximum `30`). | Viewer search results. |
| `openknowledge_read_page` | `path`, optional `knowledgeBase`, `offset` (default `0`), and `maxChars` (default `20000`, maximum `50000`). | Plain text from rendered content. |
| `openknowledge_open_page` | `path`, optional `knowledgeBase` and `beside` (default `false`). | Viewer navigation to the page. |

Use paths and knowledge-base identifiers from the page catalog or search results.
The tools reject unknown paths and knowledge bases.
For list and read results, use `nextOffset` to retrieve more content. A `null` value marks the end.
The list tool includes all available knowledge bases unless `knowledgeBase` selects one.
For read and open, an omitted `knowledgeBase` selects the current knowledge base.
The read tool includes rendered body text, frontmatter, and claims. It does not return original Markdown.
The open tool changes viewer navigation only. These browsing tools do not edit knowledge files.

## Shared editor tools

The local viewer registers these additional tools. Static exports omit them.
Existing writable Markdown pages open directly in the shared editor.
`openknowledge_open_editor` navigates to a page and focuses its draft.

| Tool | Arguments | Result |
| --- | --- | --- |
| `openknowledge_open_editor` | `path`, optional `knowledgeBase`. | Opens the page and its shared draft. |
| `openknowledge_get_editor_state` | Optional `offset` (default `0`) and `maxChars` (default `20000`, maximum `50000`). | Draft Markdown, selection, revisions, and save status. |
| `openknowledge_apply_edit` | `draftRevision`, `start`, `end`, `text`. | Replaces a range in the active draft without saving. |
| `openknowledge_undo_edit` | `draftRevision`. | Undoes the last human or agent draft change. |
| `openknowledge_save_document` | `draftRevision`. | Validates and saves the active draft to its existing file. |

Read the editor state before each edit, undo, or save.
Use its exact `draftRevision` to reject concurrent draft changes.
Edit ranges use UTF-16 offsets into the complete draft. The `end` offset is exclusive.
An empty `text` deletes the selected range. Equal `start` and `end` insert text.
Paginated state results use `nextOffset`, as page reads do.

The human edits the body in a WYSIWYG editor. The agent still reads and edits complete Markdown, including frontmatter.
Both interfaces share the draft and Undo history.
Human selections report `selection.scope: "block"`.
Their `start` and `end` cover the corresponding Markdown blocks, not an exact visible-text range.
`selection.selectedText` contains the selected visible text. `selection.text` contains the source range.
Use the returned Markdown to determine precise replacement offsets.
Draft changes remain unsaved until **Save** or `openknowledge_save_document` succeeds.
The editor shows agent changes immediately and permits human review through **Changes**.
This comparison expands below the editor, which remains visible.
Unsaved drafts survive navigation within the viewer.

Saving separately checks the SHA256 `sourceRevision` against the file on disk.
A changed source blocks a save with unsaved changes and preserves the draft for comparison.
An open draft without unsaved changes receives external source updates automatically.
Document validation errors block saves. Bundle-wide checks remain separate.
The editor limits UTF-8 documents to 1 MiB and does not create documents.
See [viewer editing](commands/view.md#edit-markdown) for recovery and conflict controls.

## Available pages

The local viewer uses the sidebar page catalog.
Interactive HTML exports use their embedded note collection.
Exported tools expose only notes enabled for both `viewer` and `search` publication.
They cannot read unpublished source files or arbitrary files from the host.
The `--plain` HTML export omits viewer JavaScript and does not register WebMCP tools.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/web/src/viewer/webmcp.js`
> - `packages/web/src/viewer/search-data.js`
> - `packages/web/src/viewer/app.js`
> - `packages/web/src/viewer/{editor,editor-state}.js`
> - `packages/web/src/viewer/{rich-editor,rich-markdown}.js`
> - `packages/cli/cmd/openknowledge/viewer_document.go`
> - `packages/cli/internal/authoring/documents.go`
> - `packages/web/scripts/dev-viewer.mjs`
> - `packages/cli/cmd/openknowledge/viewer_export.go`
>
> **Update notes**
>
> Keep browser WebMCP separate from the HTTP MCP server and MCP App extension prototype.
