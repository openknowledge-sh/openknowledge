package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

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

	var handler http.Handler
	var details func()
	if fs.NArg() == 1 {
		absolute, err := resolveViewerRoot(fs.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		handler = newViewerHandler(absolute)
		details = func() {
			fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(absolute))
		}
	} else {
		entries, err := okf.RegistryEntries()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		handler = newRegistryViewerHandler(entries)
		details = func() {
			if path, err := okf.RegistryFile(); err == nil {
				fmt.Printf("%s %s\n", terminal.muted("registry"), terminal.path(path))
			}
			fmt.Printf("%s %d\n", terminal.muted("knowledge bases"), len(entries))
		}
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
	details()
	fmt.Println(terminal.muted("Press Ctrl+C to stop."))

	if err := http.Serve(listener, handler); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func resolveViewerRoot(root string) (string, error) {
	resolved, err := okf.ResolveKnowledgeRoot(root)
	if err != nil {
		return "", err
	}

	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", absolute)
	}
	return absolute, nil
}

func newViewerHandler(root string) http.Handler {
	mux := http.NewServeMux()
	searchCache := &viewerSearchCache{root: root}
	mux.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			http.NotFound(response, request)
			return
		}
		renderViewerIndex(response, root, viewerFrame{}, "", filepath.Base(root))
	})
	mux.HandleFunc("/file/", func(response http.ResponseWriter, request *http.Request) {
		rel := strings.TrimPrefix(request.URL.Path, "/file/")
		renderViewerFile(response, request, root, rel, viewerFrame{}, "")
	})
	mux.HandleFunc("/api/search", func(response http.ResponseWriter, request *http.Request) {
		renderViewerSearch(response, request, searchCache, "")
	})
	return mux
}

func newRegistryViewerHandler(entries []okf.RegistryEntry) http.Handler {
	mux := http.NewServeMux()
	searchCaches := make(map[string]*viewerSearchCache)
	var searchCachesMutex sync.Mutex
	searchCacheForEntry := func(entry okf.RegistryEntry) (*viewerSearchCache, error) {
		root, err := registryEntryRoot(entry)
		if err != nil {
			return nil, err
		}
		searchCachesMutex.Lock()
		defer searchCachesMutex.Unlock()
		cache := searchCaches[entry.Name]
		if cache == nil || cache.root != root {
			cache = &viewerSearchCache{root: root}
			searchCaches[entry.Name] = cache
		}
		return cache, nil
	}
	mux.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			http.NotFound(response, request)
			return
		}
		if len(entries) == 0 {
			renderRegistryEmpty(response)
			return
		}
		renderRegistryIndex(response, entries, entries[0].Name)
	})
	mux.HandleFunc("/kb/", func(response http.ResponseWriter, request *http.Request) {
		name, rest, ok := parseWorkspaceRoute(request.URL.Path)
		if !ok {
			http.NotFound(response, request)
			return
		}
		entry, found := registryEntryByName(entries, name)
		if !found {
			http.NotFound(response, request)
			return
		}
		if rest == "" {
			if !strings.HasSuffix(request.URL.Path, "/") {
				http.Redirect(response, request, workspaceURL(name), http.StatusFound)
				return
			}
			renderRegistryIndex(response, entries, entry.Name)
			return
		}
		if rest == "api/search" {
			cache, err := searchCacheForEntry(entry)
			if err != nil {
				http.Error(response, err.Error(), http.StatusInternalServerError)
				return
			}
			renderViewerSearch(response, request, cache, workspacePrefix(entry.Name))
			return
		}
		if strings.HasPrefix(rest, "file/") {
			root, err := registryEntryRoot(entry)
			if err != nil {
				http.Error(response, err.Error(), http.StatusInternalServerError)
				return
			}
			frame := registryFrame(entries, entry.Name)
			renderViewerFile(response, request, root, strings.TrimPrefix(rest, "file/"), frame, workspacePrefix(entry.Name))
			return
		}
		http.NotFound(response, request)
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

type viewerWorkspace struct {
	Name   string
	Root   string
	URL    string
	Active bool
}

type viewerFrame struct {
	Workspaces []viewerWorkspace
	ActiveName string
	ActiveURL  string
}

type viewerIndexData struct {
	Frame     viewerFrame
	Title     string
	Root      string
	Error     string
	SearchURL string
	Entries   []viewerEntry
}

func renderViewerIndex(response http.ResponseWriter, root string, frame viewerFrame, linkPrefix string, title string) {
	listing, err := okf.List(root)
	if err != nil {
		renderHTML(response, viewerIndexTemplate, viewerIndexData{
			Frame: frame,
			Title: title,
			Root:  root,
			Error: err.Error(),
		})
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
			URL:    fileURLWithPrefix(linkPrefix, entry.Path),
			Kind:   kind,
			Type:   entry.Type,
			Title:  entry.Title,
			Issues: entry.Issues,
		})
	}

	renderHTML(response, viewerIndexTemplate, viewerIndexData{
		Frame:     frame,
		Title:     title,
		Root:      root,
		SearchURL: searchURLWithPrefix(linkPrefix),
		Entries:   entries,
	})
}

type viewerFileData struct {
	Frame viewerFrame
	Title string
	Root  string
	Path  string
	Body  template.HTML
}

func renderViewerFile(response http.ResponseWriter, request *http.Request, root string, rel string, frame viewerFrame, linkPrefix string) {
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
		Frame: frame,
		Title: titleForMarkdownFile(rel),
		Root:  root,
		Path:  rel,
		Body:  template.HTML(okf.RenderMarkdown(stripFrontmatter(string(content)), rel, viewerLinkWithPrefix(linkPrefix))),
	})
}

type viewerSearchResponse struct {
	Query   string               `json:"query"`
	Results []viewerSearchResult `json:"results"`
}

type viewerSearchResult struct {
	Path        string   `json:"path"`
	URL         string   `json:"url"`
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Type        string   `json:"type,omitempty"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Snippet     string   `json:"snippet,omitempty"`
	Score       float64  `json:"score"`
	Matches     []string `json:"matches,omitempty"`
}

type viewerSearchCache struct {
	root        string
	mutex       sync.Mutex
	fingerprint string
	index       okf.SearchIndex
}

func (cache *viewerSearchCache) Search(options okf.SearchOptions) ([]okf.SearchResult, error) {
	fingerprint, err := markdownFingerprint(cache.root)
	if err != nil {
		return nil, err
	}

	cache.mutex.Lock()
	if cache.fingerprint == fingerprint {
		index := cache.index
		cache.mutex.Unlock()
		return index.Search(options), nil
	}
	cache.mutex.Unlock()

	bundle, err := okf.ParseBundle(cache.root)
	if err != nil {
		return nil, err
	}
	index := okf.NewSearchIndex(bundle)

	cache.mutex.Lock()
	cache.fingerprint = fingerprint
	cache.index = index
	cache.mutex.Unlock()

	return index.Search(options), nil
}

func markdownFingerprint(root string) (string, error) {
	var builder strings.Builder
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isMarkdownFile(current) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		builder.WriteString(filepath.ToSlash(rel))
		builder.WriteByte('\x00')
		builder.WriteString(strconv.FormatInt(info.Size(), 10))
		builder.WriteByte('\x00')
		builder.WriteString(strconv.FormatInt(info.ModTime().UnixNano(), 10))
		builder.WriteByte('\n')
		return nil
	})
	if err != nil {
		return "", err
	}
	return builder.String(), nil
}

func renderViewerSearch(response http.ResponseWriter, request *http.Request, searchCache *viewerSearchCache, linkPrefix string) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := strings.TrimSpace(request.URL.Query().Get("q"))
	limit := 12
	if rawLimit := strings.TrimSpace(request.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 30 {
		limit = 30
	}

	if query == "" {
		writeViewerSearchJSON(response, viewerSearchResponse{Query: query})
		return
	}

	results, err := searchCache.Search(okf.SearchOptions{
		Query: query,
		Limit: limit,
		Fuzzy: true,
	})
	if err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}

	payload := viewerSearchResponse{
		Query:   query,
		Results: make([]viewerSearchResult, 0, len(results)),
	}
	for _, result := range results {
		payload.Results = append(payload.Results, viewerSearchResult{
			Path:        result.Path,
			URL:         fileURLWithPrefix(linkPrefix, result.Path),
			ID:          result.ID,
			Kind:        result.Kind,
			Type:        result.Type,
			Title:       result.Title,
			Description: result.Description,
			Snippet:     result.Snippet,
			Score:       result.Score,
			Matches:     result.Matches,
		})
	}

	writeViewerSearchJSON(response, payload)
}

func writeViewerSearchJSON(response http.ResponseWriter, payload viewerSearchResponse) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(response)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
	}
}

func renderRegistryEmpty(response http.ResponseWriter) {
	renderHTML(response, viewerIndexTemplate, viewerIndexData{
		Title: "Open Knowledge Registry",
		Error: "No registered knowledge bases. Add one with openknowledge registry add <name> <path>.",
	})
}

func renderRegistryIndex(response http.ResponseWriter, entries []okf.RegistryEntry, activeName string) {
	entry, found := registryEntryByName(entries, activeName)
	if !found {
		http.Error(response, "knowledge base not found", http.StatusNotFound)
		return
	}

	root, err := registryEntryRoot(entry)
	frame := registryFrame(entries, entry.Name)
	if err != nil {
		renderHTML(response, viewerIndexTemplate, viewerIndexData{
			Frame: frame,
			Title: entry.Name,
			Root:  entry.Path,
			Error: err.Error(),
		})
		return
	}
	renderViewerIndex(response, root, frame, workspacePrefix(entry.Name), entry.Name)
}

func parseWorkspaceRoute(requestPath string) (string, string, bool) {
	trimmed := strings.TrimPrefix(requestPath, "/kb/")
	if trimmed == requestPath || trimmed == "" {
		return "", "", false
	}

	namePart := trimmed
	rest := ""
	if slash := strings.Index(trimmed, "/"); slash >= 0 {
		namePart = trimmed[:slash]
		rest = trimmed[slash+1:]
	}
	name, err := url.PathUnescape(namePart)
	if err != nil || strings.TrimSpace(name) == "" {
		return "", "", false
	}
	return name, rest, true
}

func registryEntryByName(entries []okf.RegistryEntry, name string) (okf.RegistryEntry, bool) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return okf.RegistryEntry{}, false
}

func registryFrame(entries []okf.RegistryEntry, activeName string) viewerFrame {
	workspaces := make([]viewerWorkspace, 0, len(entries))
	for _, entry := range entries {
		workspaces = append(workspaces, viewerWorkspace{
			Name:   entry.Name,
			Root:   entry.Path,
			URL:    workspaceURL(entry.Name),
			Active: entry.Name == activeName,
		})
	}
	return viewerFrame{Workspaces: workspaces, ActiveName: activeName, ActiveURL: workspaceURL(activeName)}
}

func registryEntryRoot(entry okf.RegistryEntry) (string, error) {
	root, err := okf.ExpandUserPath(entry.Path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("registry entry %s has an empty path", entry.Name)
	}

	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", absolute)
	}
	return absolute, nil
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

func fileURLWithPrefix(prefix string, rel string) string {
	return strings.TrimRight(prefix, "/") + fileURL(rel)
}

func searchURLWithPrefix(prefix string) string {
	return strings.TrimRight(prefix, "/") + "/api/search"
}

func workspacePrefix(name string) string {
	return "/kb/" + url.PathEscape(name)
}

func workspaceURL(name string) string {
	return workspacePrefix(name) + "/"
}

func viewerLinkWithPrefix(prefix string) okf.LinkResolver {
	prefix = strings.TrimRight(prefix, "/")
	return func(currentRel string, href string) string {
		resolved := okf.ViewerLink(currentRel, href)
		if prefix != "" && strings.HasPrefix(resolved, "/file/") {
			return prefix + resolved
		}
		return resolved
	}
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
  <div class="app">
    {{if .Frame.Workspaces}}
      <aside class="sidebar">
        <a class="side-brand" href="/">Open Knowledge</a>
        <div class="sidebar-label">Knowledge bases</div>
        <nav class="workspaces" aria-label="Knowledge bases">
          {{range .Frame.Workspaces}}
            <a class="workspace{{if .Active}} active{{end}}" href="{{.URL}}">
              <span class="workspace-name">{{.Name}}</span>
              <span class="workspace-root">{{.Root}}</span>
            </a>
          {{end}}
        </nav>
      </aside>
    {{end}}
    <div class="content">
      <header>
        {{if .Frame.Workspaces}}<span class="current-workspace">{{.Frame.ActiveName}}</span>{{else}}<a class="brand" href="/">Open Knowledge</a>{{end}}
        {{if .Root}}<span>{{.Root}}</span>{{end}}
      </header>
      <main>
        <h1>{{.Title}}</h1>
        {{if .Error}}
          <p class="error">{{.Error}}</p>
        {{else}}
          <p class="lede">Local agentic wiki rendered from Markdown files.</p>
          <section class="search" role="search" aria-label="Search" data-search-url="{{.SearchURL}}">
            <label class="search-label" for="viewer-search">Search</label>
            <input id="viewer-search" class="search-input" type="search" autocomplete="off" spellcheck="false">
            <div id="viewer-search-status" class="search-status" aria-live="polite"></div>
            <div id="viewer-search-results" class="search-results" hidden></div>
          </section>
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
        {{end}}
      </main>
    </div>
  </div>
  <script>` + viewerSearchJS + `</script>
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
  <div class="app">
    {{if .Frame.Workspaces}}
      <aside class="sidebar">
        <a class="side-brand" href="/">Open Knowledge</a>
        <div class="sidebar-label">Knowledge bases</div>
        <nav class="workspaces" aria-label="Knowledge bases">
          {{range .Frame.Workspaces}}
            <a class="workspace{{if .Active}} active{{end}}" href="{{.URL}}">
              <span class="workspace-name">{{.Name}}</span>
              <span class="workspace-root">{{.Root}}</span>
            </a>
          {{end}}
        </nav>
      </aside>
    {{end}}
    <div class="content">
      <header>
        {{if .Frame.Workspaces}}<a class="brand" href="{{.Frame.ActiveURL}}">{{.Frame.ActiveName}}</a>{{else}}<a class="brand" href="/">Open Knowledge</a>{{end}}
        <span>{{.Path}}</span>
      </header>
      <main>
        <article class="document">
          {{.Body}}
        </article>
      </main>
    </div>
  </div>
</body>
</html>`))

const viewerSearchJS = `
(() => {
  const input = document.getElementById("viewer-search");
  const results = document.getElementById("viewer-search-results");
  const status = document.getElementById("viewer-search-status");
  const search = input?.closest(".search");
  if (!input || !results || !status || !search) return;
  const searchURL = search.dataset.searchUrl || "/api/search";

  let timer = 0;
  let controller = null;

  input.addEventListener("input", () => {
    window.clearTimeout(timer);
    timer = window.setTimeout(runSearch, 140);
  });

  async function runSearch() {
    const query = input.value.trim();
    if (!query) {
      status.textContent = "";
      results.hidden = true;
      results.replaceChildren();
      if (controller) controller.abort();
      return;
    }

    if (controller) controller.abort();
    controller = new AbortController();
    status.textContent = "Searching...";

    try {
      const response = await fetch(searchURL + "?q=" + encodeURIComponent(query) + "&limit=12", {
        signal: controller.signal,
      });
      if (!response.ok) throw new Error("search request failed");
      const payload = await response.json();
      renderResults(payload.results || [], query);
    } catch (error) {
      if (error.name === "AbortError") return;
      status.textContent = "Search failed.";
      results.hidden = true;
    }
  }

  function renderResults(items, query) {
    results.replaceChildren();
    if (items.length === 0) {
      status.textContent = "No results for \"" + query + "\".";
      results.hidden = true;
      return;
    }

    status.textContent = items.length + " result" + (items.length === 1 ? "" : "s");
    results.hidden = false;
    for (const item of items) {
      const link = document.createElement("a");
      link.className = "search-result";
      link.href = item.url;

      const title = document.createElement("span");
      title.className = "search-result-title";
      title.textContent = item.title || item.path;
      link.append(title);

      const meta = document.createElement("span");
      meta.className = "search-result-meta";
      meta.textContent = item.path + (item.type ? " - " + item.type : "");
      link.append(meta);

      if (item.snippet) {
        const snippet = document.createElement("span");
        snippet.className = "search-result-snippet";
        snippet.textContent = item.snippet;
        link.append(snippet);
      }

      results.append(link);
    }
  }
})();
`

const viewerCSS = `
:root {
  color-scheme: light;
  --ink: #1f2724;
  --muted: #65736d;
  --line: #dfe5e1;
  --paper: #f8faf8;
  --panel: #ffffff;
  --panel-soft: #f1f5f2;
  --accent: #0f7a4d;
  --accent-soft: #e6f3ec;
  --danger: #a44b28;
  font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
* { box-sizing: border-box; }
body { margin: 0; color: var(--ink); background: var(--paper); line-height: 1.55; }
.app { min-height: 100vh; display: flex; align-items: stretch; }
.sidebar { width: 286px; flex: 0 0 286px; height: 100vh; position: sticky; top: 0; overflow-y: auto; padding: 18px 14px; border-right: 1px solid var(--line); background: var(--panel-soft); }
.side-brand, .brand { color: var(--ink); font-weight: 700; text-decoration: none; }
.side-brand { display: inline-flex; margin: 0 10px 20px; }
.sidebar-label { margin: 0 10px 8px; color: var(--muted); font-size: 12px; font-weight: 700; text-transform: uppercase; }
.workspaces { display: grid; gap: 4px; }
.workspace { display: block; min-width: 0; padding: 9px 10px 9px 12px; border-left: 3px solid transparent; color: var(--ink); text-decoration: none; }
.workspace:hover { background: var(--panel); }
.workspace.active { border-left-color: var(--accent); background: var(--panel); }
.workspace-name { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 700; }
.workspace-root { display: block; margin-top: 2px; overflow-wrap: anywhere; color: var(--muted); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; line-height: 1.35; }
.content { flex: 1; min-width: 0; }
header { display: flex; justify-content: space-between; gap: 16px; min-height: 51px; padding: 14px 22px; border-bottom: 1px solid var(--line); background: var(--panel); color: var(--muted); font-size: 13px; }
header span { min-width: 0; overflow-wrap: anywhere; }
.current-workspace { color: var(--ink); font-weight: 700; }
main { width: min(960px, calc(100% - 32px)); margin: 0 auto; padding: 34px 0 56px; }
h1 { margin: 0 0 10px; font-size: 34px; line-height: 1.15; }
h2 { margin-top: 32px; padding-top: 16px; border-top: 1px solid var(--line); }
h3 { margin-top: 26px; }
.lede { margin: 0 0 26px; color: var(--muted); }
.error { margin: 0 0 26px; color: var(--danger); }
.search { margin: 0 0 24px; }
.search-label { display: block; margin: 0 0 8px; color: var(--muted); font-size: 13px; font-weight: 700; }
.search-input { width: 100%; min-height: 42px; padding: 9px 12px; border: 1px solid var(--line); border-radius: 6px; background: var(--panel); color: var(--ink); font: inherit; }
.search-input:focus { outline: 2px solid color-mix(in srgb, var(--accent) 34%, transparent); outline-offset: 2px; border-color: var(--accent); }
.search-status { min-height: 20px; margin-top: 8px; color: var(--muted); font-size: 13px; }
.search-results { display: grid; gap: 0; margin-top: 8px; border-top: 1px solid var(--line); }
.search-result { display: grid; gap: 2px; padding: 10px 0; border-bottom: 1px solid var(--line); color: inherit; text-decoration: none; }
.search-result:hover .search-result-title { color: var(--accent); }
.search-result-title { font-weight: 700; }
.search-result-meta { color: var(--muted); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 13px; overflow-wrap: anywhere; }
.search-result-snippet { color: #2f3834; font-size: 14px; }
.list { border-top: 1px solid var(--line); }
.row { display: grid; grid-template-columns: minmax(180px, 1fr) minmax(160px, .7fr); gap: 12px; padding: 12px 0; border-bottom: 1px solid var(--line); color: inherit; text-decoration: none; }
.row:hover .path { color: var(--accent); }
.path { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 14px; }
.meta, .issue { color: var(--muted); font-size: 13px; }
.issue { grid-column: 1 / -1; color: var(--danger); }
.document { max-width: 780px; }
.document p, .document li { color: #2f3834; }
a { color: var(--accent); text-underline-offset: 3px; }
code { padding: 1px 4px; border-radius: 4px; background: #edf2ef; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: .92em; }
pre { overflow-x: auto; padding: 14px; border: 1px solid var(--line); background: #111714; color: #f3f7f4; }
pre code { padding: 0; background: transparent; color: inherit; }
ul, ol { padding-left: 22px; }
blockquote { margin: 20px 0; padding: 2px 0 2px 18px; border-left: 4px solid var(--line); color: var(--muted); }
blockquote p { color: inherit; }
hr { margin: 34px 0; border: 0; border-top: 1px solid var(--line); }
table { width: 100%; border-collapse: collapse; margin: 22px 0; font-size: 15px; }
th, td { padding: 10px 12px; border: 1px solid var(--line); text-align: left; vertical-align: top; }
th { background: #edf2ef; font-weight: 700; }
.empty { color: var(--muted); }
@media (max-width: 780px) {
  .app { display: block; }
  .sidebar { width: auto; height: auto; position: static; border-right: 0; border-bottom: 1px solid var(--line); }
  .workspaces { grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); }
  header { display: block; }
  header span { display: block; margin-top: 4px; overflow-wrap: anywhere; }
  .row { grid-template-columns: 1fr; }
}
`
