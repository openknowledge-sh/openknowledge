---
type: Feature
title: Extension Prototype
description: Use the complete local Open Knowledge Viewer inside an MCP App.
tags: [openknowledge, extension, mcp, development, registry]
timestamp: 2026-10-03T00:00:00Z
---

# Extension Prototype

Use the development prototype to open the complete local Open Knowledge Viewer inside an MCP App.
The installed local plugin starts its server on demand.
The HTTP development preview is optional.

## HTTP development preview

Use Node.js 22 or later and the repository dependencies. The default backend also requires Go.

From the repository root, start the development server:

```sh
corepack pnpm --filter @openknowledge-sh/extension dev
```

Open `http://127.0.0.1:4318/` in a browser or the Codex browser panel.
The viewer shows workspaces from the global Open Knowledge registry.
Use the viewer's existing navigation to select a workspace and document.
The same viewer serves registered bases and honors their access.

To register a local base, use these commands:

```sh
okn connect /absolute/path/to/wiki --as my-wiki --access write
okn registry list
```

To use another knowledge base or port, supply these options:

```sh
corepack pnpm --filter @openknowledge-sh/extension dev --root /absolute/path/to/wiki --port 4319
```

- `--root` selects one knowledge base instead of the global registry.
- `--port` selects the local server port. The default is `4318`.
- `--viewer-url` requires `--root`. It selects an existing loopback HTTP viewer for the same knowledge base.
- `OPENKNOWLEDGE_BIN` optionally selects an existing CLI executable. Without this variable, the backend uses `go run` from the repository.

The server listens on `127.0.0.1`. The MCP endpoint is `/mcp`.
This unauthenticated server is for local development.

## MCP Apps test host

Open `http://127.0.0.1:4318/host.html?mode=native` to test the direct viewer resource through the official `AppBridge`.
The test host uses HTTP MCP and a sandboxed app on a separate origin.
It displays received model context. No model connects to this test host.

Select **Reload MCP App** to load the current packaged UI resource.
The separate browser preview at `/` continues to use Vite live updates.
The default `/host.html` checks the development wrapper with its nested viewer iframe.

## Embedded viewer

The extension embeds the complete existing viewer.
The viewer provides its workspaces, document tree, search, and editor.
The extension adds host integration and active-document context.

The native UI resource contains the existing Go viewer's HTML with local JavaScript and styles inline.
A runtime adapter preserves viewer URLs and history.
The adapter sends reads and document saves through app-only MCP tools.
The native app uses no nested viewer iframe or browser loopback requests.
The resource CSP declares no frame, connection, or resource domains.
The ordinary browser viewer retains its existing framing protection.

The viewer opens writable Markdown pages directly in its WYSIWYG editor.
Read-only bases open without editor access.
The native app reuses the viewer's draft, Undo, Redo, and explicit Save.
The viewer's own navigation preserves draft sessions.
The extension does not register the viewer's browser WebMCP tools with the host.

The native adapter supports document Save as its write action.
Other write actions and remote document assets require the browser viewer.
Native resources do not subscribe to external file changes.
Reopen the MCP App to load external updates.

Select **Open in browser** for the browser viewer.
This action uses the host's negotiated `openLinks` capability.
The host selects the destination. The local preview opens a browser tab.

## Document context

Context actions require an active document.
In a compatible host, the selected page supplies context for the model.
The context action supplies the selected passage or saved page text.
Saved page context includes the knowledge base, page path, source URI, and revision.
Viewer selection context includes rendered text and an unsaved-change flag.
Viewer selection context does not claim a saved revision.

The action limits text to 24,000 characters.

The extension does not copy whole unsaved drafts.
**Use saved page** reads the active page from the backend again.
This action includes the latest saved edits and current source revision.

In the local preview, the action only displays the prepared context. It does not send context to a model.

## MCP retrieval

The backend uses the existing Go MCP server for page inventory, source text, and search.
The extension exposes four read-only tools to the model.
Two app-only tools supply native viewer reads and document saves.
`openknowledge_viewer_get` reads routes within the current viewer.
`openknowledge_viewer_save` forwards document Save to the viewer's existing API.
Save requires write access, a managed document, a matching source revision, and successful OKF validation.

MCP page tools use the `knowledgeBase` alias from the library inventory.
In registry mode, page list and read operations require this alias.
Without an alias, `openknowledge_search` searches all registered bases.
The search result reports unavailable bases separately.

## Host integration

The MCP App declares `global` and `thread` entrypoints for the library tool.
These declarations request a sidebar application and a conversation panel in compatible hosts.
Codex CLI installation and MCP startup are verified for local plugin version `0.0.6`.
The official Codex app-server discovers six tools and reads the full viewer resource with inline assets.
The resource declares no frame or connection domains.
Full viewer display in the native sidebar and conversation panel remains unverified.
The Codex browser preview and test host do not verify these native surfaces.

Incoming host deep links use `/bases/<encoded-alias>?path=<encoded-relative-path>`.
The app reads this route from the host's `openai/deepLink` context.
For example, `/bases/my-wiki?path=docs%2Findex.md` selects a base and opens its page.
The app does not provide a share-link control or composer mentions.

To build the local package, run:

```sh
corepack pnpm --filter @openknowledge-sh/extension package:local
```

The script creates `packages/extension/dist/local-marketplace`.
The package includes a Node.js executable, bundled server, native viewer adapter, development UI, CLI binary, and plugin assets.
The script copies its Node.js 22 or later runtime and the CLI from `OPENKNOWLEDGE_BIN`.
Without `OPENKNOWLEDGE_BIN`, the script builds the CLI with Go.
The package excludes knowledge files and user configuration.

Version `0.0.6` uses stdio. Codex starts the packaged MCP server on demand.
The installed plugin does not require the HTTP development server on port `4318`.

The generated MCP configuration uses the plugin-contained `./bin/node` command.
Windows packages use `./bin/node.exe`.
The configuration launches `${PLUGIN_ROOT}/server/main.mjs --stdio`.
The server uses the included CLI binary.
The included runtime and CLI target the build host's operating system and architecture.
This local package is not a portable binary distribution.

To register the marketplace and install the plugin, use the absolute marketplace path:

```sh
codex plugin marketplace add /absolute/path/to/openknowledge/packages/extension/dist/local-marketplace
codex plugin add openknowledge-local@openknowledge-dev
```

After a package rebuild, use the same `codex plugin add` command to update an existing installation.

To verify the installed plugin through Codex's loader, run:

```sh
corepack pnpm --filter @openknowledge-sh/extension test:codex
```

The check verifies the installed version, stdio startup, six tools, and full viewer resource.
The check requires inline assets and no local frames or connection domains.
Set `CODEX_BIN` to select the Codex CLI executable.

To test the direct resource in Chromium, set `OPENKNOWLEDGE_BIN` to a built CLI.
Run `corepack pnpm --filter @openknowledge-sh/extension test:native-browser`.
This check covers viewer navigation, search, drafts, Save, context, and local images with browser connections and nested frames blocked.

## Development updates

Vite updates the extension wrapper when its UI source files change.
The development server also rebuilds the packaged UI resource for the next host resource request.
The HTTP preview uses the viewer's existing live reload and draft protection.
Native resources require a new resource request to load external document updates.

After extension code changes, rebuild the local package.
Reinstall the package.
Refresh the connection to use the new package.
The development preview remains separate from the installed plugin.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/extension/{package,plugin,mcp}.json`
> - `packages/extension/scripts/{dev,build,package-local}.mjs`
> - `packages/extension/server/{main,mcp,knowledge,native-viewer,viewer-panel-proxy}.mjs`
> - `packages/extension/web/{native-viewer.js,native-runtime.mjs,library-state.mjs,app.js,host.html,host.js}`
> - `packages/extension/test/{package.test,native-browser,codex}.mjs`
>
> **Update notes**
>
> Keep development preview behavior separate from verified native host behavior.
