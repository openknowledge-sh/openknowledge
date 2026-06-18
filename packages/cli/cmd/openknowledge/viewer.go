package main

import (
	"flag"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func runOpen(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, openHelpText())
		return 0
	}
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	host := fs.String("host", "127.0.0.1", "host to bind")
	port := fs.Int("port", 0, "port to bind, or 0 for a free port")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "open accepts at most one path")
		return 2
	}

	root := "."
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}

	absolute, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	info, err := os.Stat(absolute)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "%s is not a directory\n", absolute)
		return 1
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(*host, strconv.Itoa(*port)))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	addr := listener.Addr().String()
	if strings.HasPrefix(addr, "0.0.0.0:") {
		addr = "127.0.0.1:" + strings.TrimPrefix(addr, "0.0.0.0:")
	}

	fmt.Printf("Open Knowledge view: http://%s/\n", addr)
	fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(absolute))
	fmt.Println(terminal.muted("Press Ctrl+C to stop."))

	if err := http.Serve(listener, newViewerHandler(absolute)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func newViewerHandler(root string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			http.NotFound(response, request)
			return
		}
		renderViewerIndex(response, root)
	})
	mux.HandleFunc("/file/", func(response http.ResponseWriter, request *http.Request) {
		rel := strings.TrimPrefix(request.URL.Path, "/file/")
		renderViewerFile(response, request, root, rel)
	})
	return mux
}

type viewerEntry struct {
	Path   string
	URL    string
	Kind   string
	Type   string
	Title  string
	Issues []okf.Issue
}

type viewerIndexData struct {
	Title   string
	Root    string
	Entries []viewerEntry
}

func renderViewerIndex(response http.ResponseWriter, root string) {
	listing, err := okf.List(root)
	if err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}

	entries := make([]viewerEntry, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		kind := entry.Kind
		if kind == "" && entry.Type != "" {
			kind = entry.Type
		}
		if kind == "" {
			kind = "concept"
		}
		entries = append(entries, viewerEntry{
			Path:   entry.Path,
			URL:    fileURL(entry.Path),
			Kind:   kind,
			Type:   entry.Type,
			Title:  entry.Title,
			Issues: entry.Issues,
		})
	}

	renderHTML(response, viewerIndexTemplate, viewerIndexData{
		Title:   filepath.Base(root),
		Root:    root,
		Entries: entries,
	})
}

type viewerFileData struct {
	Title string
	Root  string
	Path  string
	Body  template.HTML
}

func renderViewerFile(response http.ResponseWriter, request *http.Request, root string, rel string) {
	filePath, ok := safeMarkdownPath(root, rel)
	if !ok {
		http.NotFound(response, request)
		return
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		http.NotFound(response, request)
		return
	}

	renderHTML(response, viewerFileTemplate, viewerFileData{
		Title: titleForMarkdownFile(rel),
		Root:  root,
		Path:  rel,
		Body:  template.HTML(okf.RenderMarkdown(stripFrontmatter(string(content)), rel, okf.ViewerLink)),
	})
}

func renderHTML(response http.ResponseWriter, tmpl *template.Template, data any) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(response, data); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
	}
}

func safeMarkdownPath(root string, rel string) (string, bool) {
	if hasParentSegment(rel) {
		return "", false
	}
	clean := path.Clean("/" + rel)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." || clean == "" {
		clean = "index.md"
	}
	if !isMarkdownFile(clean) {
		return "", false
	}

	full := filepath.Join(root, filepath.FromSlash(clean))
	relative, err := filepath.Rel(root, full)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}

func isMarkdownFile(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	return extension == ".md" || extension == ".markdown"
}

func hasParentSegment(value string) bool {
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func fileURL(rel string) string {
	return "/file/" + strings.TrimPrefix(path.Clean("/"+rel), "/")
}

func titleForMarkdownFile(rel string) string {
	base := filepath.Base(rel)
	extension := filepath.Ext(base)
	base = strings.TrimSuffix(base, extension)
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.TrimSpace(base)
	if base == "" {
		return rel
	}
	words := strings.Fields(base)
	for index, word := range words {
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func stripFrontmatter(text string) string {
	if !strings.HasPrefix(text, "---\n") {
		return text
	}
	rest := text[len("---\n"):]
	index := strings.Index(rest, "\n---\n")
	if index < 0 {
		return text
	}
	return rest[index+len("\n---\n"):]
}

var viewerIndexTemplate = template.Must(template.New("viewer-index").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} - Open Knowledge</title>
  <style>` + viewerCSS + `</style>
</head>
<body>
  <header>
    <a class="brand" href="/">Open Knowledge</a>
    <span>{{.Root}}</span>
  </header>
  <main>
    <h1>{{.Title}}</h1>
    <p class="lede">Local agentic wiki rendered from Markdown files.</p>
    <section class="list">
      {{range .Entries}}
        <a class="row" href="{{.URL}}">
          <span class="path">{{.Path}}</span>
          <span class="meta">{{if .Type}}{{.Type}}{{else}}{{.Kind}}{{end}}{{if .Title}} - {{.Title}}{{end}}</span>
          {{if .Issues}}{{with index .Issues 0}}<span class="issue">{{.Message}}</span>{{end}}{{end}}
        </a>
      {{else}}
        <p class="empty">No Markdown files found.</p>
      {{end}}
    </section>
  </main>
</body>
</html>`))

var viewerFileTemplate = template.Must(template.New("viewer-file").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} - Open Knowledge</title>
  <style>` + viewerCSS + `</style>
</head>
<body>
  <header>
    <a class="brand" href="/">Open Knowledge</a>
    <span>{{.Path}}</span>
  </header>
  <main>
    <article class="document">
      {{.Body}}
    </article>
  </main>
</body>
</html>`))

const viewerCSS = `
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
hr { margin: 28px 0; border: 0; border-top: 1px solid var(--line); }
.lede { margin: 0 0 26px; color: var(--muted); }
.list { border-top: 1px solid var(--line); }
.row { display: grid; grid-template-columns: minmax(180px, 1fr) minmax(160px, .7fr); gap: 12px; padding: 12px 0; border-bottom: 1px solid var(--line); color: inherit; text-decoration: none; }
.row:hover .path { color: var(--accent); }
.path { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 14px; }
.meta, .issue { color: var(--muted); font-size: 13px; }
.issue { grid-column: 1 / -1; color: #a44b28; }
.document { max-width: 780px; }
.document p, .document li { color: #2f3834; }
a { color: var(--accent); text-underline-offset: 3px; }
strong { color: var(--ink); font-weight: 700; }
blockquote { margin: 20px 0; padding: 2px 0 2px 18px; border-left: 3px solid var(--line); color: var(--muted); }
blockquote p { color: var(--muted); }
code { padding: 1px 4px; border-radius: 4px; background: #edf2ef; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: .92em; }
pre { overflow-x: auto; padding: 14px; border: 1px solid var(--line); background: #111714; color: #f3f7f4; }
pre code { padding: 0; background: transparent; color: inherit; }
ul, ol { padding-left: 22px; }
.empty { color: var(--muted); }
@media (max-width: 680px) {
  header { display: block; }
  header span { display: block; margin-top: 4px; overflow-wrap: anywhere; }
  .row { grid-template-columns: 1fr; }
}
`
