# Open Knowledge extension

An MCP App that embeds the complete existing Open Knowledge Viewer beside the
conversation. The viewer provides its own workspaces, document tree, search,
and editor. The extension adds host integration and active-document context.
The installed plugin starts its server on demand. The HTTP development preview
is optional.

## HTTP development preview

Use Node.js 22+ (24 recommended), Go, and Corepack pnpm 10+.
Run from the repository root:

```sh
corepack pnpm install --frozen-lockfile
corepack pnpm --filter @openknowledge-sh/extension dev
```

- [Live UI preview](http://127.0.0.1:4318/): Vite updates UI edits in place.
- [Direct viewer test host](http://127.0.0.1:4318/host.html?mode=native): loads
  the native Viewer resource through HTTP MCP and the official MCP Apps bridge.
  Use **Reload MCP App** after a UI build. Received model context appears in the
  inspector. No model is connected to this test host.
- [Iframe preview test host](http://127.0.0.1:4318/host.html): checks the separate
  development wrapper with its nested viewer iframe.
- MCP endpoint: `http://127.0.0.1:4318/mcp`.

These previews work in the Codex in-app browser. They do not verify the native sidebar.
The preview context button does not send a chat message.

```sh
corepack pnpm --filter @openknowledge-sh/extension dev --root /absolute/path/to/wiki --port 4319
```

Without `--root`, the viewer shows workspaces from the global Open Knowledge
registry. Select a workspace and document through the viewer's existing
navigation. The same viewer serves registered bases and honors their access.

To register a local base, use the CLI:

```sh
okn connect /absolute/path/to/wiki --as my-wiki --access write
okn registry list
```

`--root` selects one base and isolates the app from the other registry entries.
Set `OPENKNOWLEDGE_BIN` to use a built CLI instead of `go run`.
Restart the development server after server-code edits. UI edits rebuild
`dist/app.html` automatically. Document updates use the viewer's existing
live reload and draft protection.

To use an existing viewer development server, supply its knowledge base URL:

```sh
corepack pnpm --filter @openknowledge-sh/extension dev --root "$PWD/Wiki" --viewer-url http://127.0.0.1:5176/docs/
```

`--viewer-url` requires `--root`. The root and URL must refer to the same
knowledge base. The extension preserves the URL prefix and does not start or
stop that viewer. Only loopback HTTP URLs are accepted.

The native MCP App resource contains the existing Go viewer's complete HTML
with local JavaScript and styles inline. A runtime adapter preserves viewer
URLs and history, then sends reads and document saves through app-only MCP
tools. The native resource uses no nested viewer iframe or browser loopback
requests. Its resource CSP declares no frame, connection, or resource domains.
The ordinary browser viewer retains its existing framing protection.

The viewer keeps its workspaces, document tree, search, editor, Undo, validation,
and explicit Save. Its own navigation preserves draft sessions. Read-only bases
open without editor access. **Use saved page** reads the active page again,
including its latest saved edits and current source revision.

The native adapter supports document Save as its write action. Other write
actions and remote document assets require the browser viewer. Native resources
do not subscribe to external file changes. Reopen the MCP App to load external
updates. The HTTP preview retains the viewer's live reload and draft protection.

**Open in browser** opens the original viewer through the host's negotiated
`openLinks` capability. The host chooses where to display that link. In the
local preview, the action opens a browser tab. Full viewer display in the native
Codex panel remains unverified.

The local server binds only to loopback and rejects foreign browser origins.
Do not expose it publicly: it has no user authentication and inventories local
Markdown through direct stdio MCP, without publication filters.

## Install the local plugin

Build and install the local package from the repository root:

```sh
corepack pnpm --filter @openknowledge-sh/extension package:local
codex plugin marketplace add ./packages/extension/dist/local-marketplace
codex plugin add openknowledge-local@openknowledge-dev
```

Version `0.0.6` uses stdio. Codex starts the packaged MCP server on demand.
The installed plugin does not require the HTTP development server on port
`4318`. The earlier HTTP package depended on that listener, which prevented
startup when the server was absent after a desktop restart.

The package contains a Node.js executable, bundled server, native viewer adapter,
development UI, Open Knowledge CLI binary, and plugin assets. `package:local`
builds the UI and server, copies its Node.js 22+ runtime, then copies
`OPENKNOWLEDGE_BIN` or builds the CLI with Go. It excludes knowledge files and
user configuration.

The generated MCP configuration uses the plugin-contained `./bin/node`
command (`./bin/node.exe` on Windows) and launches
`${PLUGIN_ROOT}/server/main.mjs --stdio` with the included
CLI binary. Codex rejects absolute stdio command paths in plugin configurations.
The included runtime and CLI target the build host's operating system and
architecture. This local package is not a portable binary distribution.

After rebuilding, run the same `codex plugin add` command to update an existing
installation.

On this Mac, if the PATH `codex` wrapper is broken, the application bundles a CLI:

```sh
/Applications/ChatGPT.app/Contents/Resources/codex-cli/bin/codex plugin list --marketplace openknowledge-dev
```

Restart the desktop application if the installed plugin is not visible, and
use a new conversation. The server declares `global` and `thread` entrypoints
for compatible hosts. Codex CLI installation and MCP startup are verified for
local plugin version `0.0.6`. The official Codex app-server discovers six tools
and reads the full viewer resource with inline assets. The resource declares
no frame or connection domains. Full viewer display in the native sidebar
remains unverified. The browser preview and test host do not verify native
surfaces.

For hosted ChatGPT testing, run the HTTP development server and connect it
through **Secure MCP Tunnel** in developer mode. A cloud host cannot reach
this machine's `127.0.0.1` directly.
Tunnel setup needs a Platform tunnel and its runtime credential. Do not put
credentials in this package. Rebuild and reinstall the local package after
extension code changes. Refresh the connection to use the new package.
The development preview remains separate from the installed plugin.

See [OpenAI testing guidance](https://developers.openai.com/plugins/deploy/connect-chatgpt)
and [Secure MCP Tunnel](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels).

## Verify

```sh
corepack pnpm --filter @openknowledge-sh/extension test
# Install Chromium once before the browser checks:
corepack pnpm exec playwright install chromium
corepack pnpm --filter @openknowledge-sh/extension test:browser
# With OPENKNOWLEDGE_BIN set to a built CLI, check the direct viewer resource:
corepack pnpm --filter @openknowledge-sh/extension test:native-browser
# With the local plugin installed, check Codex's actual plugin loader:
corepack pnpm --filter @openknowledge-sh/extension test:codex
# With the development server running:
corepack pnpm --filter @openknowledge-sh/extension test:live
```

The tests cover MCP initialization, entrypoints, strict tool arguments,
base and page boundaries, registry changes, origin checks, editor URL prefixes,
host capability negotiation, rebuilt resources, and bundled JS syntax.
The package test checks isolated stdio startup, UI resource requests before
tool calls, process cleanup, and restart without the HTTP preview.
`test:codex` checks the installed version, stdio startup, six tools, and full
viewer resource through the official Codex loader. It checks inline assets and
the absence of local frames and connection domains. Set `CODEX_BIN` to the
Codex CLI executable when the PATH wrapper is unavailable.
The live test uses a real MCP SDK client against the configured wiki and viewer.
Set `OKN_EXTENSION_URL` for a different local MCP URL.
The browser test uses temporary knowledge bases. It checks document navigation,
viewer drafts, Undo, Save, selected context, and the isolated MCP Apps test host.
It also checks omitted host CSP reports, page addresses from another MCP
process, and fallback after an enforced CSP denial in the iframe preview.
The native browser test checks the full viewer, search, local images, drafts,
Undo, Redo, Save, read-only bases, context, and deep links with frames and HTTP
connections blocked. It also checks an opaque host origin.

## Scope

The existing viewer supplies navigation, workspace selection, document search,
and editing. The extension supplies host entrypoints, viewer embedding, and
page or selection context. Saved page context identifies the knowledge base,
page path, source URI, and revision. Without a selection, context reads the
currently active page from the backend. Selection context includes rendered
text and an unsaved-change flag. It does not claim a saved revision. Text is
limited to 24,000 characters.

MCP page operations use `knowledgeBase` aliases from the library inventory and
relative page paths from the selected base. In registry mode, listing or reading
pages requires an alias. `openknowledge_search` searches all registered bases
when no alias is supplied and reports unavailable bases separately.

Incoming host deep links use the app-relative route
`/bases/<encoded-alias>?path=<encoded-relative-path>`. For example,
`/bases/my-wiki?path=docs%2Findex.md` selects a base and opens its page.
The app accepts this route through the host's `openai/deepLink` context.
There is no share-link control in this implementation.

The four model-facing MCP tools remain read-only. Two app-only tools supply
native viewer reads and document saves: `openknowledge_viewer_get` and
`openknowledge_viewer_save`. Save retains the viewer's write-access, managed
document, source-revision, and OKF validation checks. The extension does not
register the viewer's browser WebMCP tools with its host. Composer mentions are
outside this implementation.

Build once and serve MCP over stdio when a host needs that transport:

```sh
corepack pnpm --filter @openknowledge-sh/extension build
node packages/extension/server/main.mjs --stdio --root /absolute/path/to/wiki
```

Use the direct `node` command in an MCP client configuration to keep stdout free
of package-manager banners. The standard Open Knowledge CLI is unchanged.
