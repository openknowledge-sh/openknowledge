---
type: Command Documentation
title: openknowledge view
description: Browse a local or connected knowledge base in the web viewer.
tags: [openknowledge, cli, command, viewer]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge view`

Start the local web viewer. Specify a path or registry key to open one
knowledge base. Omit the target to open the registry workspace selector.

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
| `--token <token>` | environment/generated | Network access token. Prefer `OPENKNOWLEDGE_VIEW_TOKEN`. |
| `--name <name>` | registry/folder name | Alias used for a direct path. |
| `--no-browser` | off | Print the URL. Do not open a browser. |
| `--head-file <file>` | environment | Trusted HTML injected into every page head. |
| `--head-html <html>` | environment | Trusted inline HTML injected into every page head. |
| `--script-src <src>` | environment | Trusted script URL. Repeatable. |

Head injection also reads `OPENKNOWLEDGE_HEAD_FILE`,
`OPENKNOWLEDGE_HEAD_HTML`, and `OPENKNOWLEDGE_SCRIPT_SRC`.

## Viewer features

- The viewer renders Markdown and follows local links. It renders fenced
  `mermaid` blocks as diagrams. It also shows note panels, source graphs,
  validation context, highlighted assets, and media or PDF previews.
- AST-based search uses the same section ranking as
  [`openknowledge search`](search.md). It also uses the same one-level link
  expansion. Results can link to and highlight the matching text.
- The viewer shows typed YAML frontmatter, tag filters, sortable tables, and
  directory breadcrumbs.
- You can change browser-local themes, typography, line length, contrast,
  motion, and link settings. These preferences do not change source Markdown.
- Direct paths and writable local connections provide local editor links.
  Static exports use configured repository source links.

| Shortcut | Action |
| --- | --- |
| `⌘K` / `Ctrl+K` | Focus search. |
| `⌘⌥S` / `Ctrl+Alt+S` | Toggle the file explorer. |
| `⌘⌥W` / `Ctrl+Alt+W` | Close the focused note. |

The viewer selects the displayed brand from root metadata in this order:
`okf_bundle_title`, `okf_bundle_name`, `title`, then the first H1.

## Network and file safety

Loopback mode does not require a token. A non-loopback or wildcard bind
requires
`--allow-network`. All routes then use token authentication.

The initial URL exchanges the token for an HttpOnly, SameSite cookie. It then
redirects to a clean URL. A remote client can also send
`Authorization: Bearer <token>`.

Raw routes serve only regular non-Markdown bundle assets. They exclude
dotfiles, `.git`, `openknowledge.toml`, and symlinks. Markdown and asset
resolution cannot leave the bundle root.

The viewer inserts trusted head fragments in their original form. Use only
content that you control.

In registry mode, the viewer rebuilds routes after the validated registry
snapshot changes. It refreshes content-hashed search indexes after source
edits. A registry or fingerprint failure returns an error. The viewer does not
serve stale or partly trusted state.

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
