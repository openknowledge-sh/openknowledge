---
type: Command Documentation
title: openknowledge view
description: Browse a local or connected knowledge base in the web viewer.
tags: [openknowledge, cli, command, viewer]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge view`

Start the local web viewer. Pass a path or registry key to open one knowledge
base; omit it to open the registry workspace selector.

## Usage

```sh
openknowledge view [key-or-path]
openknowledge view --no-browser Wiki
openknowledge view --host 127.0.0.1 --port 8080 Wiki
openknowledge view --allow-network --host 0.0.0.0 Wiki
```

| Option | Default | Description |
| --- | --- | --- |
| `--host <host>` | `127.0.0.1` | Listener host. |
| `--port <port>` | free port | Listener port. |
| `--allow-network` | off | Permit a non-loopback bind and require authentication. |
| `--token <token>` | environment/generated | Network access token; prefer `OPENKNOWLEDGE_VIEW_TOKEN`. |
| `--name <name>` | registry/folder name | Alias used for a direct path. |
| `--no-browser` | off | Print the URL without opening a browser. |
| `--head-file <file>` | environment | Trusted HTML injected into every page head. |
| `--head-html <html>` | environment | Trusted inline HTML injected into every page head. |
| `--script-src <src>` | environment | Trusted script URL; repeatable. |

Head injection also reads `OPENKNOWLEDGE_HEAD_FILE`,
`OPENKNOWLEDGE_HEAD_HTML`, and `OPENKNOWLEDGE_SCRIPT_SRC`.

## Viewer features

- Rendered Markdown with local-link navigation, stacked note panels, source
  graph, validation context, syntax-highlighted assets, and native media/PDF
  previews.
- AST-backed search using the same section ranking and one-hop link expansion
  as [`openknowledge search`](search.md). Results can deep-link and highlight
  the matching text.
- Typed YAML frontmatter inspector, tag facets, semantic tables with filtering
  and sorting, and directory breadcrumbs.
- Browser-local themes, typography, line length, contrast, motion, and link
  settings. These preferences never modify source Markdown.
- Local editor links for direct paths and writable local connections. Static
  exports use configured repository source links instead.

| Shortcut | Action |
| --- | --- |
| `⌘K` / `Ctrl+K` | Focus search. |
| `⌘⌥S` / `Ctrl+Alt+S` | Toggle the file explorer. |
| `⌘⌥W` / `Ctrl+Alt+W` | Close the focused note. |

The displayed brand comes from root metadata in this order:
`okf_bundle_title`, `okf_bundle_name`, `title`, then the first H1.

## Network and file safety

Loopback mode requires no token. Non-loopback and wildcard binds require
`--allow-network`; every route then shares token authentication. The initial
URL exchanges the token for an HttpOnly, SameSite cookie and redirects to a
clean URL. Remote clients may also send `Authorization: Bearer <token>`.

Raw routes serve only regular, non-Markdown bundle assets. They exclude
dotfiles, `.git`, `openknowledge.toml`, and symlinks. Markdown and asset
resolution cannot traverse outside the bundle root. Trusted head fragments are
inserted without escaping; use them only with content you control.

In registry mode, routing is rebuilt when the validated registry snapshot
changes. Search indexes are content-hashed and refreshed after source edits.
Registry or fingerprint failures return an error instead of serving stale or
partially trusted state.

Theme and source-link configuration comes from
[`openknowledge.toml`](/features/configuration.md). For deployment, use the
[HTML exporter](/features/exporters/html.md).

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/viewer.go`
> - `packages/cli/cmd/openknowledge/viewer_app.js`
> - `packages/cli/cmd/openknowledge/viewer_search.js`
> - `packages/cli/cmd/openknowledge/viewer_test.go`
> - `packages/cli/internal/okf/search.go`
>
> **Update notes**
>
> Update this page when viewer flags, routing, authentication, navigation, or
> file-serving behavior changes.
