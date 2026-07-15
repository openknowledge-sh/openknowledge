---
type: Exporter Documentation
title: HTML Exporter
description: Static HTML export behavior for Open Knowledge bundles.
tags: [openknowledge, cli, exporter, html]
timestamp: 2026-07-09T00:00:00Z
---

# HTML Exporter

The HTML exporter turns an OKF bundle into static pages. The default mode ships
the same viewer used by `openknowledge view`, so exported docs keep file
browsing, search, stacked panels, graph data, table controls, syntax
highlighting, typed frontmatter inspectors, and mobile layout behavior without
a local server. It also writes discovery and connection assets so agents can
index the published wiki and a deployed wiki can be added back to the local
registry.

Use `--plain` when the output should be only semantic HTML without viewer CSS,
JavaScript, search, graph data, or table controls.

## Command

```sh
openknowledge to html --out <folder> [path]
openknowledge to html --plain --out <folder> [path]
openknowledge to html --head-file <file> --out <folder> [path]
openknowledge to html --script-src <src> --out <folder> [path]
openknowledge to html --spec <version> --out <folder> [path]
```

## Arguments And Flags

| Name | Kind | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `path` | argument | no | current directory | Knowledge base root. |
| `--out` | flag | yes | none | Output folder for generated HTML files. |
| `--head-file` | flag | no | `OPENKNOWLEDGE_HEAD_FILE` | Trusted HTML fragment file to inject into default viewer HTML `<head>`. |
| `--head-html` | flag | no | `OPENKNOWLEDGE_HEAD_HTML` | Trusted HTML fragment to inject into default viewer HTML `<head>`. |
| `--plain` | flag | no | off | Generate plain semantic HTML without CSS, JavaScript, or viewer chrome. |
| `--script-src` | repeatable flag | no | `OPENKNOWLEDGE_SCRIPT_SRC` | Script `src` to inject into default viewer HTML `<head>`. Environment values may be comma- or newline-separated. |
| `--spec` | flag | no | `latest` | OKF spec version. |

## Behavior

Both HTML modes require the input bundle to validate without errors for the
selected spec; warnings remain allowed. Validation runs before output writes.
The default viewer therefore emits `openknowledge.json` only when its portable
archive satisfies the same validation gate enforced by remote `connect`.

Default viewer pages render YAML frontmatter as a typed, collapsible inspector
that starts collapsed above the Markdown body. The browser-local `Show frontmatter`
setting is enabled by default and controls inspector visibility for initial and
dynamically opened panels; it does not expand an inspector. Values use the
same content-aware presentation as `openknowledge view`: booleans retain a
state treatment, simple lists render as chips, and nested lists and maps render
recursively, without datatype badges. Plain exports continue to omit
frontmatter and viewer chrome.

Top-level `tags` values are exported as navigable facets. Selecting a tag opens
the shared search surface with exact same-tag matches from other published
notes. Exported note paths also render as segmented breadcrumbs whose directory
segments link only to published index documents that actually exist; the leaf
returns to a clean single-panel page.

Default viewer pages start with the Night theme when no valid browser-local
theme preference exists. A saved theme selection takes precedence on later
visits, and the lightweight head bootstrap restores built-in presets before the
viewer CSS paints. The previous light palette remains available as Light in the
viewer settings menu.

Markdown pages in the default viewer export share the local viewer's system-level
reading and accessibility settings: font family, text size, line spacing, motion,
readable line length, high contrast, and link underlining. Preferences are stored
in the browser and do not modify exported Markdown or apply to `--plain` output.

Both modes rewrite local Markdown links to generated `.html` targets and skip
files with `okf_publish: false`. Rendered Markdown comes from the parsed
Markdown AST rather than a separate body scan. It keeps list continuations
inside their parent item and emits semantic tables with alignment metadata.
HTML comments are hidden. The `<!-- okf-footer: agent-maintenance -->` marker
renders following content as a subdued maintenance footer in the default
viewer export.

Default viewer exports embed a static note manifest and graph data in each page
so search, panel navigation, source links, and enhanced table controls work on a
static host. The viewer can resolve pretty URLs such as `/agents` or
`/features/` back to generated notes, and the header brand links to the exported
`index.html` with a relative URL for subpath deployments. Portable static pages
do not expose the local filesystem path used during the build.

Default viewer exports can inject trusted custom `<head>` HTML into every
generated wiki page with `--head-file`, `--head-html`, repeatable
`--script-src`, or the matching `OPENKNOWLEDGE_HEAD_FILE`,
`OPENKNOWLEDGE_HEAD_HTML`, and `OPENKNOWLEDGE_SCRIPT_SRC` environment
variables. Use this for deployment-owned analytics, verification meta tags, or
small loader scripts. Script URLs may be relative, `http:`, or `https:`. Plain
exports remain semantic HTML and do not support custom head injection.

Default viewer exports include discovery files:

* `llms.txt` - a Markdown index following the llms.txt proposal shape: H1 title,
  summary blockquote, details, and a `## Docs` section of Markdown links to
  published pages. When no site base URL is configured, links are relative to
  the export root.
* `sitemap.xml` - a Sitemap Protocol XML document for published pages when
  `[html.site].base_url` is configured. The sitemap is skipped without a base URL
  because sitemap `<loc>` entries must be absolute `http` or `https` URLs on one
  host.

Default viewer exports include remote-connect assets:

* `openknowledge.json` - an Open Knowledge manifest with type
  `openknowledge.bundle`, archive path, archive format, spec version, bundle
  name/title metadata when present, and archive SHA-256.
* `assets/openknowledge-bundle.tar.gz` - a portable source bundle archive
  generated with the same deterministic archive machinery as `openknowledge to
  tar`, but scoped to published Markdown: files marked `okf_publish: false` are
  excluded from the downloadable archive as well as HTML, static payloads,
  graphs, and discovery files.

`openknowledge connect <deployed-wiki-url>` discovers and validates the
versioned manifest, requires and verifies the archive hash, extracts the archive
safely, validates the extracted bundle against the manifest's concrete spec,
and registers the materialized bundle in the local registry.

Default viewer exports read optional settings from `openknowledge.toml`.
`[html.theme]` sets a theme name and stylesheet:

```toml
[html.theme]
name = "landing"
stylesheet = "assets/wiki-theme.css"

[html.site]
base_url = "https://openknowledge.sh/wiki/"
```

Exported pages include `data-openknowledge-theme="<name>"` on `<html>` and link
the stylesheet after the built-in viewer CSS. Local stylesheets must stay inside
the bundle; they are copied into the output and linked relatively from every
page. External `http` and `https` stylesheet URLs are linked as-is.

`[html.site].base_url` sets the deployed root URL for discovery files. It must be
an absolute `http` or `https` URL without a query string or fragment. The export
normalizes it with a trailing slash and uses it for absolute `llms.txt` links and
`sitemap.xml` `<loc>` entries. Omit it for portable local exports that should not
claim a canonical deployment URL.

Default viewer exports also suppress the local editor dropdown because deployed
HTML cannot open the build machine's local files. To show a source action in
exported pages, configure `[html.source]` in `openknowledge.toml`:

```toml
[html.source]
github_base = "https://github.com/openknowledge-sh/openknowledge/blob/main"
entry = "Wiki"
```

When `github_base` is present, each exported Markdown panel shows a GitHub
source button built from `github_base`, optional `entry`, and the Markdown file
path. When `[html.source]` is absent, exported pages render no local editor or
source action. `html.source.entry` is a repository path prefix, not the viewer
title.

The deployed viewer brand comes from root `index.md` metadata in this order:
`okf_bundle_title`, `okf_bundle_name`, `title`, then the first Markdown `#`
heading. The built-in theme contract lives in
`packages/cli/cmd/openknowledge/viewer_theme.css`; override `--ok-*` variables
there through a configured stylesheet instead of changing generated HTML.

## Use Cases

* Publish a portable static wiki.
* Publish visible, type-aware OKF metadata without exposing raw YAML delimiters.
* Expose `llms.txt` and `sitemap.xml` for agents and crawlers.
* Add deployment-owned analytics or verification tags to default viewer pages.
* Connect a deployed wiki back into a local registry.
* Produce minimal HTML for systems that should not include viewer JavaScript.
* Apply a deployable theme stylesheet without changing source Markdown.
* Link deployed pages to GitHub source without exposing local editor deeplinks.

## Change History

### 2026-07-15 - Connectable publication gate

Default and plain HTML exports now reject bundles with validation errors before
writing output. This prevents a successful static publication from advertising
a remote-connect archive that its consumer will reject. Warnings remain
publishable. Source anchors: `packages/cli/internal/okf/html.go`,
`packages/cli/internal/okf/validation_types.go`,
`packages/cli/cmd/openknowledge/viewer.go`,
`packages/cli/internal/okf/export_test.go`, and
`packages/cli/cmd/openknowledge/viewer_test.go`.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/html.go`
> * `packages/cli/internal/okf/markdown.go`
> * `packages/cli/internal/okf/export_test.go`
> * `packages/cli/cmd/openknowledge/main.go`
> * `packages/cli/cmd/openknowledge/viewer.go`
> * `packages/cli/cmd/openknowledge/viewer_frontmatter.go`
> * `packages/cli/cmd/openknowledge/viewer_discovery.go`
> * `packages/cli/cmd/openknowledge/viewer_theme.go`
> * `packages/cli/cmd/openknowledge/viewer_theme.css`
> * `packages/cli/cmd/openknowledge/viewer_test.go`
>
> **Update notes**
>
> When template structure, CSS, link rewriting, file naming, or export reporting
> changes, update this page and [CLI changelog](/changelog/cli.md).
