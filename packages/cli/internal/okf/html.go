package okf

import (
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type HTMLResult struct {
	Root    string   `json:"root"`
	Out     string   `json:"out"`
	Written []string `json:"written"`
}

type htmlPageData struct {
	Title string
	Path  string
	Body  template.HTML
}

func WriteHTML(root string, out string) (HTMLResult, error) {
	return WriteHTMLWithVersion(root, out, LatestSpecVersion)
}

func WriteHTMLWithVersion(root string, out string, version string) (HTMLResult, error) {
	return writeHTMLWithVersion(root, out, version, staticPageTemplate)
}

func WritePlainHTML(root string, out string) (HTMLResult, error) {
	return WritePlainHTMLWithVersion(root, out, LatestSpecVersion)
}

func WritePlainHTMLWithVersion(root string, out string, version string) (HTMLResult, error) {
	return writeHTMLWithVersion(root, out, version, plainPageTemplate)
}

func writeHTMLWithVersion(root string, out string, version string, pageTemplate *template.Template) (HTMLResult, error) {
	bundle, err := ParseBundleWithVersion(root, version)
	if err != nil {
		return HTMLResult{}, err
	}

	absoluteOut, err := filepath.Abs(out)
	if err != nil {
		return HTMLResult{}, err
	}

	var written []string
	for _, file := range bundle.Files {
		if !ShouldPublish(file) {
			continue
		}
		target := filepath.Join(absoluteOut, filepath.FromSlash(htmlPath(file.Path)))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return HTMLResult{}, err
		}

		page := htmlPageData{
			Title: file.Title,
			Path:  file.Path,
			Body:  template.HTML(RenderMarkdown(file.Body, file.Path, StaticHTMLLink)),
		}
		if page.Title == "" {
			page.Title = deriveTitle(file.Path)
		}

		var builder strings.Builder
		if err := pageTemplate.Execute(&builder, page); err != nil {
			return HTMLResult{}, err
		}
		if err := os.WriteFile(target, []byte(builder.String()), 0644); err != nil {
			return HTMLResult{}, err
		}
		written = append(written, relPath(absoluteOut, target))
	}

	sort.Strings(written)
	return HTMLResult{Root: bundle.Root, Out: absoluteOut, Written: written}, nil
}

var staticPageTemplate = template.Must(template.New("static-page").Funcs(template.FuncMap{
	"rootHref": func(currentPath string) string {
		currentHTML := htmlPath(currentPath)
		relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(currentHTML)), "index.html")
		if err != nil {
			return "index.html"
		}
		return filepath.ToSlash(relative)
	},
}).Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} - Open Knowledge</title>
  <style>` + staticHTMLCSS + `</style>
</head>
<body>
  <header>
    <a class="brand" href="{{rootHref .Path}}">Open Knowledge</a>
    <span>{{.Path}}</span>
  </header>
  <main>
    <article class="document">
      {{.Body}}
    </article>
  </main>
</body>
</html>`))

var plainPageTemplate = template.Must(template.New("plain-page").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{{.Title}}</title>
</head>
<body>
  <main>
    <article>
      {{.Body}}
    </article>
  </main>
</body>
</html>`))

const staticHTMLCSS = `
:root {
  color-scheme: light;
  --ink: #1f2724;
  --muted: #65736d;
  --line: #dfe5e1;
  --paper: #f8faf8;
  --panel: #ffffff;
  --accent: #0f7a4d;
  font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
* { box-sizing: border-box; }
body { margin: 0; color: var(--ink); background: var(--paper); line-height: 1.55; }
header { display: flex; justify-content: space-between; gap: 16px; padding: 14px 22px; border-bottom: 1px solid var(--line); background: var(--panel); color: var(--muted); font-size: 13px; }
.brand { color: var(--ink); font-weight: 700; text-decoration: none; }
main { width: min(960px, calc(100% - 32px)); margin: 0 auto; padding: 34px 0 56px; }
h1 { margin: 0 0 10px; font-size: 34px; line-height: 1.15; }
h2 { margin-top: 32px; padding-top: 16px; border-top: 1px solid var(--line); }
h3 { margin-top: 26px; }
.document { max-width: 780px; }
.document p, .document li { color: #2f3834; }
a { color: var(--accent); text-underline-offset: 3px; }
code { padding: 1px 4px; border-radius: 4px; background: #edf2ef; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: .92em; }
pre, .code-block { overflow-x: auto; padding: 14px; border: 1px solid var(--line); background: #111714; color: #f3f7f4; }
pre code, .code-block code { padding: 0; background: transparent; color: inherit; }
.code-block { border-radius: 6px; line-height: 1.6; tab-size: 2; }
.tok-keyword { color: #8fd3ff; font-weight: 700; }
.tok-string { color: #a7e08f; }
.tok-number { color: #ffd479; }
.tok-comment { color: #8c9a93; font-style: italic; }
ul, ol { padding-left: 22px; }
blockquote { margin: 20px 0; padding: 2px 0 2px 18px; border-left: 4px solid var(--line); color: var(--muted); }
blockquote p { color: inherit; }
hr { margin: 34px 0; border: 0; border-top: 1px solid var(--line); }
table { width: 100%; border-collapse: collapse; margin: 22px 0; font-size: 15px; }
th, td { padding: 10px 12px; border: 1px solid var(--line); text-align: left; vertical-align: top; }
th { background: #edf2ef; font-weight: 700; }
.ok-table-wrap { max-width: 100%; margin: 22px 0; overflow: hidden; border: 1px solid var(--line); border-radius: 6px; background: var(--panel); }
.ok-table-scroller { overflow-x: auto; }
.ok-table { min-width: max-content; margin: 0; border-collapse: separate; border-spacing: 0; }
.ok-table th, .ok-table td { border: 0; border-right: 1px solid var(--line); border-bottom: 1px solid var(--line); }
.ok-table th:last-child, .ok-table td:last-child { border-right: 0; }
.ok-table tbody tr:last-child td { border-bottom: 0; }
.ok-table [data-align="center"] { text-align: center; }
.ok-table [data-align="right"] { text-align: right; }
@media (max-width: 680px) {
  header { display: block; }
  header span { display: block; margin-top: 4px; overflow-wrap: anywhere; }
}
`
