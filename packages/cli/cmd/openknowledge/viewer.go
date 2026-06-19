package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
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
	name := fs.String("name", "", "local alias name for direct path mode")
	localDomain := fs.String("local-domain", "open.knowledge", "local alias domain to print, or empty to disable")
	noBrowser := fs.Bool("no-browser", false, "print the URL without opening a browser")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "open accepts at most one path")
		return 2
	}

	var handler http.Handler
	var details func()
	aliasNames := []string{}
	if fs.NArg() == 1 {
		target := fs.Arg(0)
		absolute, err := resolveViewerRoot(target)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		aliasName := directViewerAliasName(target, absolute, *name)
		if aliasName != "" {
			aliasNames = append(aliasNames, aliasName)
		}
		handler = newViewerHandlerWithAlias(absolute, aliasName)
		details = func() {
			fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(absolute))
		}
	} else {
		entries, err := okf.RegistryEntries()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		for _, entry := range entries {
			aliasNames = append(aliasNames, entry.Name)
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

	displayHost, displayPort := displayHostPort(listener.Addr())
	viewURL, aliasURL := viewerDisplayURLs(displayHost, displayPort, *localDomain, aliasNames)

	fmt.Printf("Open Knowledge view: %s\n", viewURL)
	if aliasURL != "" && aliasURL != viewURL {
		fmt.Printf("Open Knowledge alias: %s\n", aliasURL)
	}
	details()
	fmt.Println(terminal.muted("Press Ctrl+C to stop."))

	if !*noBrowser {
		if err := openBrowser(viewURL); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not open browser: %v\n", err)
		}
	}

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

func directViewerAliasName(target string, root string, override string) string {
	if name := normalizeLocalAliasName(override); name != "" {
		return name
	}
	if !okf.LooksLikePath(target) {
		if entry, ok, err := okf.ResolveRegistryEntry(target); err == nil && ok {
			return entry.Name
		}
	}
	if name := registryAliasNameForRoot(root); name != "" {
		return name
	}
	return normalizeLocalAliasName(filepath.Base(filepath.Clean(root)))
}

func registryAliasNameForRoot(root string) string {
	root = filepath.Clean(root)
	entries, err := okf.RegistryEntries()
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		entryRoot, err := okf.ExpandUserPath(entry.Path)
		if err != nil {
			continue
		}
		absolute, err := filepath.Abs(entryRoot)
		if err != nil {
			continue
		}
		if filepath.Clean(absolute) == root {
			return entry.Name
		}
	}
	return ""
}

func newViewerHandler(root string) http.Handler {
	return newViewerHandlerWithAlias(root, "")
}

func newViewerHandlerWithAlias(root string, aliasName string) http.Handler {
	mux := http.NewServeMux()
	searchCache := &viewerSearchCache{root: root}
	mux.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/" {
			if startPath := viewerStartPath(root); startPath != "/" {
				http.Redirect(response, request, startPath, http.StatusFound)
				return
			}
			renderViewerIndex(response, root, viewerFrame{}, "", filepath.Base(root))
			return
		}
		rest, ok := parseDirectAliasRoute(request.URL.Path, aliasName)
		if !ok {
			http.NotFound(response, request)
			return
		}
		prefix := localAliasPrefix(aliasName)
		if rest == "" {
			if !strings.HasSuffix(request.URL.Path, "/") {
				http.Redirect(response, request, localAliasURL(aliasName), http.StatusFound)
				return
			}
			if startPath := viewerStartPathWithPrefix(root, prefix); startPath != prefix+"/" {
				http.Redirect(response, request, startPath, http.StatusFound)
				return
			}
			renderViewerIndex(response, root, viewerFrame{}, prefix, filepath.Base(root))
			return
		}
		if rest == "api/search" {
			renderViewerSearch(response, request, searchCache, prefix)
			return
		}
		if strings.HasPrefix(rest, "api/file/") {
			rel := strings.TrimPrefix(rest, "api/file/")
			renderViewerFileAPI(response, request, root, rel, prefix)
			return
		}
		if strings.HasPrefix(rest, "file/") {
			renderViewerFile(response, request, root, strings.TrimPrefix(rest, "file/"), viewerFrame{}, prefix)
			return
		}
		http.NotFound(response, request)
	})
	mux.HandleFunc("/file/", func(response http.ResponseWriter, request *http.Request) {
		rel := strings.TrimPrefix(request.URL.Path, "/file/")
		renderViewerFile(response, request, root, rel, viewerFrame{}, "")
	})
	mux.HandleFunc("/api/file/", func(response http.ResponseWriter, request *http.Request) {
		rel := strings.TrimPrefix(request.URL.Path, "/api/file/")
		renderViewerFileAPI(response, request, root, rel, "")
	})
	mux.HandleFunc("/api/search", func(response http.ResponseWriter, request *http.Request) {
		renderViewerSearch(response, request, searchCache, "")
	})
	mux.HandleFunc("/api/editor-icon/", func(response http.ResponseWriter, request *http.Request) {
		editorID := strings.TrimPrefix(request.URL.Path, "/api/editor-icon/")
		renderViewerEditorIcon(response, request, editorID)
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
			entry, rest, ok := parseRegistryAliasRoute(request.URL.Path, entries)
			if ok {
				prefix := localAliasPrefix(entry.Name)
				if rest == "" {
					if !strings.HasSuffix(request.URL.Path, "/") {
						http.Redirect(response, request, localAliasURL(entry.Name), http.StatusFound)
						return
					}
					root, err := registryEntryRoot(entry)
					frame := registryFrame(entries, entry.Name, localAliasURL)
					if err != nil {
						renderHTML(response, viewerIndexTemplate, viewerIndexData{
							Frame: frame,
							Title: entry.Name,
							Root:  entry.Path,
							Error: err.Error(),
						})
						return
					}
					if startPath := viewerStartPathWithPrefix(root, prefix); startPath != prefix+"/" {
						http.Redirect(response, request, startPath, http.StatusFound)
						return
					}
					renderViewerIndex(response, root, frame, prefix, entry.Name)
					return
				}
				if rest == "api/search" {
					cache, err := searchCacheForEntry(entry)
					if err != nil {
						http.Error(response, err.Error(), http.StatusInternalServerError)
						return
					}
					renderViewerSearch(response, request, cache, prefix)
					return
				}
				if strings.HasPrefix(rest, "api/file/") {
					root, err := registryEntryRoot(entry)
					if err != nil {
						http.Error(response, err.Error(), http.StatusInternalServerError)
						return
					}
					renderViewerFileAPI(response, request, root, strings.TrimPrefix(rest, "api/file/"), prefix)
					return
				}
				if strings.HasPrefix(rest, "file/") {
					root, err := registryEntryRoot(entry)
					if err != nil {
						http.Error(response, err.Error(), http.StatusInternalServerError)
						return
					}
					frame := registryFrame(entries, entry.Name, localAliasURL)
					renderViewerFile(response, request, root, strings.TrimPrefix(rest, "file/"), frame, prefix)
					return
				}
			}
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
		prefix := workspacePrefix(entry.Name)
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
			renderViewerSearch(response, request, cache, prefix)
			return
		}
		if strings.HasPrefix(rest, "api/file/") {
			root, err := registryEntryRoot(entry)
			if err != nil {
				http.Error(response, err.Error(), http.StatusInternalServerError)
				return
			}
			renderViewerFileAPI(response, request, root, strings.TrimPrefix(rest, "api/file/"), prefix)
			return
		}
		if strings.HasPrefix(rest, "file/") {
			root, err := registryEntryRoot(entry)
			if err != nil {
				http.Error(response, err.Error(), http.StatusInternalServerError)
				return
			}
			frame := registryFrame(entries, entry.Name, workspaceURL)
			renderViewerFile(response, request, root, strings.TrimPrefix(rest, "file/"), frame, prefix)
			return
		}
		http.NotFound(response, request)
	})
	mux.HandleFunc("/api/editor-icon/", func(response http.ResponseWriter, request *http.Request) {
		editorID := strings.TrimPrefix(request.URL.Path, "/api/editor-icon/")
		renderViewerEditorIcon(response, request, editorID)
	})
	return mux
}

func viewerStartPath(root string) string {
	return viewerStartPathWithPrefix(root, "")
}

func viewerStartPathWithPrefix(root string, prefix string) string {
	filePath, ok := safeMarkdownPath(root, "index.md")
	if !ok {
		return viewerPrefixRoot(prefix)
	}
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return viewerPrefixRoot(prefix)
	}
	return fileURLWithPrefix(prefix, "index.md")
}

func viewerPrefixRoot(prefix string) string {
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return "/"
	}
	return prefix + "/"
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
	Frame       viewerFrame
	Title       string
	Root        string
	Path        string
	FileURL     string
	LinkPrefix  string
	SearchURL   string
	Body        template.HTML
	Tree        []viewerTreeItem
	EditorsJSON template.JS
	StaticJSON  template.JS
	GraphJSON   template.JS
}

type viewerFilePayload struct {
	Title string `json:"title"`
	Path  string `json:"path"`
	Body  string `json:"body"`
}

type viewerStaticPayload struct {
	Title    string `json:"title"`
	Path     string `json:"path"`
	HTMLPath string `json:"htmlPath"`
	Body     string `json:"body"`
}

type viewerGraphData struct {
	Nodes []viewerGraphNode `json:"nodes"`
	Edges []viewerGraphEdge `json:"edges"`
}

type viewerGraphNode struct {
	Path  string `json:"path"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type viewerGraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
}

type viewerEditor struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Short     string `json:"short"`
	Available bool   `json:"available"`
	Icon      string `json:"icon,omitempty"`
}

type knownViewerEditor struct {
	ID       string
	Name     string
	Short    string
	Commands []string
	Apps     []string
}

type viewerEditorBrandIcon struct {
	Title string
	Hex   string
	Path  string
}

type viewerTreeItem struct {
	Name      string
	Path      string
	URL       string
	Depth     int
	Indent    int
	Directory bool
	System    bool
}

func renderViewerFile(response http.ResponseWriter, request *http.Request, root string, rel string, frame viewerFrame, linkPrefix string) {
	data, ok, err := viewerFile(root, rel, frame, linkPrefix)
	if !ok {
		http.NotFound(response, request)
		return
	}
	if err != nil {
		http.NotFound(response, request)
		return
	}

	renderHTML(response, viewerFileTemplate, data)
}

func renderViewerFileAPI(response http.ResponseWriter, request *http.Request, root string, rel string, linkPrefix string) {
	data, ok, err := viewerFile(root, rel, viewerFrame{}, linkPrefix)
	if !ok {
		http.NotFound(response, request)
		return
	}
	if err != nil {
		http.NotFound(response, request)
		return
	}

	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(response).Encode(viewerFilePayload{
		Title: data.Title,
		Path:  data.Path,
		Body:  string(data.Body),
	}); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
	}
}

func viewerFile(root string, rel string, frame viewerFrame, linkPrefix string) (viewerFileData, bool, error) {
	cleanRel, ok := cleanMarkdownRel(rel)
	if !ok {
		return viewerFileData{}, false, nil
	}

	filePath, ok := safeMarkdownPath(root, cleanRel)
	if !ok {
		return viewerFileData{}, false, nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return viewerFileData{}, true, err
	}
	listing, err := okf.List(root)
	if err != nil {
		return viewerFileData{}, true, err
	}
	graphJSON := viewerGraphJSON(root, listing.Entries, func(path string) string {
		return fileURLWithPrefix(linkPrefix, path)
	})

	return viewerFileData{
		Frame:       frame,
		Title:       titleForMarkdownFile(cleanRel),
		Root:        root,
		Path:        cleanRel,
		FileURL:     fileURLWithPrefix(linkPrefix, cleanRel),
		LinkPrefix:  strings.TrimRight(linkPrefix, "/"),
		SearchURL:   searchURLWithPrefix(linkPrefix),
		Body:        template.HTML(okf.RenderMarkdown(stripFrontmatter(string(content)), cleanRel, viewerLinkWithPrefix(linkPrefix))),
		Tree:        viewerTreeWithURL(listing.Entries, func(path string) string { return fileURLWithPrefix(linkPrefix, path) }),
		EditorsJSON: viewerEditorsJSON(),
		GraphJSON:   graphJSON,
	}, true, nil
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
	frame := registryFrame(entries, entry.Name, workspaceURL)
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

func parseRegistryAliasRoute(requestPath string, entries []okf.RegistryEntry) (okf.RegistryEntry, string, bool) {
	name, rest, ok := parseLocalAliasRoute(requestPath)
	if !ok {
		return okf.RegistryEntry{}, "", false
	}
	entry, found := registryEntryByName(entries, name)
	if !found {
		return okf.RegistryEntry{}, "", false
	}
	return entry, rest, true
}

func parseDirectAliasRoute(requestPath string, aliasName string) (string, bool) {
	name, rest, ok := parseLocalAliasRoute(requestPath)
	if !ok || aliasName == "" || name != aliasName {
		return "", false
	}
	return rest, true
}

func parseLocalAliasRoute(requestPath string) (string, string, bool) {
	trimmed := strings.TrimPrefix(requestPath, "/")
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

func registryFrame(entries []okf.RegistryEntry, activeName string, urlFor func(string) string) viewerFrame {
	workspaces := make([]viewerWorkspace, 0, len(entries))
	for _, entry := range entries {
		workspaces = append(workspaces, viewerWorkspace{
			Name:   entry.Name,
			Root:   entry.Path,
			URL:    urlFor(entry.Name),
			Active: entry.Name == activeName,
		})
	}
	return viewerFrame{Workspaces: workspaces, ActiveName: activeName, ActiveURL: urlFor(activeName)}
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

func writeViewerHTMLWithVersion(root string, out string, version string) (okf.HTMLResult, error) {
	bundle, err := okf.ParseBundleWithVersion(root, version)
	if err != nil {
		return okf.HTMLResult{}, err
	}

	absoluteOut, err := filepath.Abs(out)
	if err != nil {
		return okf.HTMLResult{}, err
	}

	staticJSON, err := viewerStaticFilesJSON(bundle.Files)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	editorsJSON := viewerEditorsStaticJSON()
	graphJSON := viewerStaticGraphJSON(bundle.Files)

	var written []string
	for _, file := range bundle.Files {
		target := filepath.Join(absoluteOut, filepath.FromSlash(viewerHTMLPath(file.Path)))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return okf.HTMLResult{}, err
		}

		data := viewerFileData{
			Title:       titleForMarkdownFile(file.Path),
			Root:        bundle.Root,
			Path:        file.Path,
			FileURL:     viewerStaticRelativeURL(file.Path, file.Path),
			Body:        template.HTML(viewerStaticFileBody(file)),
			Tree:        viewerStaticTree(bundle.Files, file.Path),
			EditorsJSON: editorsJSON,
			StaticJSON:  staticJSON,
			GraphJSON:   graphJSON,
		}

		var builder strings.Builder
		if err := viewerFileTemplate.Execute(&builder, data); err != nil {
			return okf.HTMLResult{}, err
		}
		if err := os.WriteFile(target, []byte(builder.String()), 0644); err != nil {
			return okf.HTMLResult{}, err
		}
		written = append(written, viewerRelPath(absoluteOut, target))
	}

	sort.Strings(written)
	return okf.HTMLResult{Root: bundle.Root, Out: absoluteOut, Written: written}, nil
}

func viewerRelPath(root string, target string) string {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(relative)
}

func viewerStaticFilesJSON(files []okf.BundleFile) (template.JS, error) {
	payload := make([]viewerStaticPayload, 0, len(files))
	for _, file := range files {
		payload = append(payload, viewerStaticPayload{
			Title:    titleForMarkdownFile(file.Path),
			Path:     file.Path,
			HTMLPath: viewerHTMLPath(file.Path),
			Body:     viewerStaticFileBody(file),
		})
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return template.JS(data), nil
}

func viewerStaticFileBody(file okf.BundleFile) string {
	return okf.RenderMarkdown(file.Body, file.Path, okf.StaticHTMLLink)
}

func viewerHTMLPath(markdownPath string) string {
	extension := filepath.Ext(markdownPath)
	if extension == "" {
		return filepath.ToSlash(filepath.Join(markdownPath, "index.html"))
	}
	return strings.TrimSuffix(markdownPath, extension) + ".html"
}

func viewerStaticRelativeURL(currentPath string, targetPath string) string {
	currentHTML := viewerHTMLPath(currentPath)
	targetHTML := viewerHTMLPath(targetPath)
	relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(currentHTML)), filepath.FromSlash(targetHTML))
	if err != nil {
		return filepath.ToSlash(targetHTML)
	}
	return filepath.ToSlash(relative)
}

func viewerStaticTree(files []okf.BundleFile, currentPath string) []viewerTreeItem {
	entries := make([]okf.ListEntry, 0, len(files))
	for _, file := range files {
		entries = append(entries, okf.ListEntry{Path: file.Path})
	}
	return viewerTreeWithURL(entries, func(path string) string {
		return viewerStaticRelativeURL(currentPath, path)
	})
}

func viewerGraphJSON(root string, entries []okf.ListEntry, fileURL func(string) string) template.JS {
	graph := viewerGraphFromEntries(root, entries, fileURL)
	data, err := json.Marshal(graph)
	if err != nil {
		return `{"nodes":[],"edges":[]}`
	}
	return template.JS(data)
}

func viewerStaticGraphJSON(files []okf.BundleFile) template.JS {
	entries := make([]okf.ListEntry, 0, len(files))
	for _, file := range files {
		entries = append(entries, okf.ListEntry{Path: file.Path, Title: file.Title})
	}
	graph := viewerGraphFromBundleFiles(files, entries, func(path string) string {
		return viewerStaticRelativeURL("index.md", path)
	})
	data, err := json.Marshal(graph)
	if err != nil {
		return `{"nodes":[],"edges":[]}`
	}
	return template.JS(data)
}

func viewerGraphFromEntries(root string, entries []okf.ListEntry, fileURL func(string) string) viewerGraphData {
	files := make([]okf.BundleFile, 0, len(entries))
	for _, entry := range entries {
		contentPath, ok := safeMarkdownPath(root, entry.Path)
		if !ok {
			continue
		}
		content, err := os.ReadFile(contentPath)
		if err != nil {
			continue
		}
		files = append(files, okf.BundleFile{
			Path:  entry.Path,
			Title: entry.Title,
			Links: okf.ExtractLinks(root, entry.Path, string(content)),
		})
	}
	return viewerGraphFromBundleFiles(files, entries, fileURL)
}

func viewerGraphFromBundleFiles(files []okf.BundleFile, entries []okf.ListEntry, fileURL func(string) string) viewerGraphData {
	titles := make(map[string]string, len(entries))
	paths := make(map[string]bool, len(entries))
	for _, entry := range entries {
		paths[entry.Path] = true
		titles[entry.Path] = entry.Title
	}

	nodes := make([]viewerGraphNode, 0, len(entries))
	for _, entry := range entries {
		title := strings.TrimSpace(titles[entry.Path])
		if title == "" {
			title = titleForMarkdownFile(entry.Path)
		}
		nodes = append(nodes, viewerGraphNode{
			Path:  entry.Path,
			Title: title,
			URL:   fileURL(entry.Path),
		})
	}

	seenEdges := map[string]bool{}
	var edges []viewerGraphEdge
	for _, file := range files {
		for _, link := range file.Links {
			if link.Kind != "local" || !paths[link.TargetPath] || link.TargetPath == file.Path {
				continue
			}
			key := file.Path + "\x00" + link.TargetPath
			if seenEdges[key] {
				continue
			}
			seenEdges[key] = true
			edges = append(edges, viewerGraphEdge{
				Source: file.Path,
				Target: link.TargetPath,
				Label:  link.Label,
			})
		}
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source == edges[j].Source {
			return edges[i].Target < edges[j].Target
		}
		return edges[i].Source < edges[j].Source
	})

	return viewerGraphData{Nodes: nodes, Edges: edges}
}

func viewerEditorsJSON() template.JS {
	data, err := json.Marshal(viewerEditors())
	if err != nil {
		return "[]"
	}
	return template.JS(data)
}

func viewerEditorsStaticJSON() template.JS {
	data, err := json.Marshal(viewerEditorsForStatic())
	if err != nil {
		return "[]"
	}
	return template.JS(data)
}

func viewerEditors() []viewerEditor {
	known := knownViewerEditors()

	available := make([]viewerEditor, 0, len(known))
	fallback := make([]viewerEditor, 0, len(known))
	for _, editor := range known {
		item := viewerEditor{
			ID:        editor.ID,
			Name:      editor.Name,
			Short:     editor.Short,
			Available: viewerEditorAvailable(editor),
		}
		if viewerEditorHasIcon(editor) {
			item.Icon = "/api/editor-icon/" + editor.ID
		}
		if item.Available {
			available = append(available, item)
		} else {
			fallback = append(fallback, item)
		}
	}
	return append(available, fallback...)
}

func viewerEditorsForStatic() []viewerEditor {
	editors := viewerEditors()
	for index, editor := range editors {
		if icon, ok := viewerEditorStaticIcon(editor.ID); ok {
			editors[index].Icon = icon
		} else {
			editors[index].Icon = ""
		}
	}
	return editors
}

func knownViewerEditors() []knownViewerEditor {
	return []knownViewerEditor{
		{ID: "cursor", Name: "Cursor", Short: "Cu", Commands: []string{"cursor"}, Apps: []string{"Cursor"}},
		{ID: "code", Name: "Visual Studio Code", Short: "VS", Commands: []string{"code"}, Apps: []string{"Visual Studio Code"}},
		{ID: "windsurf", Name: "Windsurf", Short: "Ws", Commands: []string{"windsurf"}, Apps: []string{"Windsurf"}},
		{ID: "zed", Name: "Zed", Short: "Zd", Commands: []string{"zed"}, Apps: []string{"Zed"}},
		{ID: "sublime", Name: "Sublime Text", Short: "Su", Commands: []string{"subl"}, Apps: []string{"Sublime Text"}},
		{ID: "obsidian", Name: "Obsidian", Short: "Ob", Commands: []string{"obsidian"}, Apps: []string{"Obsidian"}},
		{ID: "textedit", Name: "TextEdit", Short: "Tx", Apps: []string{"TextEdit"}},
		{ID: "bbedit", Name: "BBEdit", Short: "BB", Commands: []string{"bbedit"}, Apps: []string{"BBEdit"}},
		{ID: "nova", Name: "Nova", Short: "Nv", Apps: []string{"Nova"}},
		{ID: "intellij", Name: "IntelliJ IDEA", Short: "IJ", Commands: []string{"idea"}, Apps: []string{"IntelliJ IDEA", "IntelliJ IDEA CE"}},
		{ID: "webstorm", Name: "WebStorm", Short: "WS", Commands: []string{"webstorm"}, Apps: []string{"WebStorm"}},
		{ID: "neovim", Name: "Neovim", Short: "Nv", Commands: []string{"nvim"}, Apps: []string{"Neovim"}},
		{ID: "vim", Name: "Vim", Short: "Vi", Commands: []string{"vim", "mvim"}, Apps: []string{"MacVim"}},
		{ID: "emacs", Name: "Emacs", Short: "Em", Commands: []string{"emacs"}, Apps: []string{"Emacs"}},
	}
}

func viewerEditorAvailable(editor knownViewerEditor) bool {
	for _, command := range editor.Commands {
		if _, err := exec.LookPath(command); err == nil {
			return true
		}
	}

	_, ok := viewerEditorAppPath(editor)
	return ok
}

func viewerEditorHasIcon(editor knownViewerEditor) bool {
	if _, ok := viewerEditorIconSource(editor); ok {
		return true
	}
	_, ok := viewerEditorBrandIconByID(editor.ID)
	return ok
}

func renderViewerEditorIcon(response http.ResponseWriter, request *http.Request, editorID string) {
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if editorID == "" || strings.Contains(editorID, "/") || hasParentSegment(editorID) {
		http.NotFound(response, request)
		return
	}

	editor, ok := knownViewerEditorByID(editorID)
	if !ok {
		http.NotFound(response, request)
		return
	}
	source, ok := viewerEditorIconSource(editor)
	if ok {
		png, err := viewerEditorPNGIcon(editor.ID, source)
		if err == nil {
			response.Header().Set("Content-Type", "image/png")
			response.Header().Set("Cache-Control", "private, max-age=86400")
			http.ServeFile(response, request, png)
			return
		}
	}

	icon, ok := viewerEditorBrandIconByID(editor.ID)
	if !ok {
		http.NotFound(response, request)
		return
	}

	response.Header().Set("Cache-Control", "private, max-age=86400")
	response.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	_, _ = fmt.Fprintf(
		response,
		`<svg xmlns="http://www.w3.org/2000/svg" role="img" viewBox="0 0 24 24"><title>%s</title><path fill="#%s" d="%s"/></svg>`,
		template.HTMLEscapeString(icon.Title),
		template.HTMLEscapeString(icon.Hex),
		template.HTMLEscapeString(icon.Path),
	)
}

func knownViewerEditorByID(editorID string) (knownViewerEditor, bool) {
	for _, editor := range knownViewerEditors() {
		if editor.ID == editorID {
			return editor, true
		}
	}
	return knownViewerEditor{}, false
}

func viewerEditorAppPath(editor knownViewerEditor) (string, bool) {
	home, _ := os.UserHomeDir()
	applicationRoots := []string{"/Applications", "/System/Applications"}
	if home != "" {
		applicationRoots = append(applicationRoots, filepath.Join(home, "Applications"))
	}

	for _, app := range editor.Apps {
		for _, root := range applicationRoots {
			appPath := filepath.Join(root, app+".app")
			info, err := os.Stat(appPath)
			if err == nil && info.IsDir() {
				return appPath, true
			}
		}
	}
	return "", false
}

func viewerEditorIconSource(editor knownViewerEditor) (string, bool) {
	appPath, ok := viewerEditorAppPath(editor)
	if !ok {
		return "", false
	}

	resources := filepath.Join(appPath, "Contents", "Resources")
	for _, candidate := range viewerEditorIconCandidates(appPath) {
		if candidate == "" {
			continue
		}
		iconPath := filepath.Join(resources, candidate)
		if filepath.Ext(iconPath) == "" {
			iconPath += ".icns"
		}
		info, err := os.Stat(iconPath)
		if err == nil && !info.IsDir() {
			return iconPath, true
		}
	}

	matches, err := filepath.Glob(filepath.Join(resources, "*.icns"))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

func viewerEditorIconCandidates(appPath string) []string {
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")
	content, err := os.ReadFile(plistPath)
	if err != nil {
		return nil
	}
	plist := string(content)
	return []string{
		plistStringValue(plist, "CFBundleIconFile"),
		plistStringValue(plist, "CFBundleIconName"),
	}
}

func plistStringValue(plist string, key string) string {
	keyTag := "<key>" + key + "</key>"
	keyIndex := strings.Index(plist, keyTag)
	if keyIndex < 0 {
		return ""
	}
	rest := plist[keyIndex+len(keyTag):]
	start := strings.Index(rest, "<string>")
	if start < 0 {
		return ""
	}
	rest = rest[start+len("<string>"):]
	end := strings.Index(rest, "</string>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

func viewerEditorPNGIcon(editorID string, source string) (string, error) {
	if _, err := exec.LookPath("sips"); err != nil {
		return "", err
	}

	sourceInfo, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(os.TempDir(), "openknowledge-editor-icons")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", err
	}
	target := filepath.Join(cacheDir, editorID+"-"+strconv.FormatInt(sourceInfo.ModTime().Unix(), 10)+".png")
	if targetInfo, err := os.Stat(target); err == nil && !targetInfo.IsDir() {
		return target, nil
	}

	if err := exec.Command("sips", "-s", "format", "png", source, "--out", target).Run(); err != nil {
		return "", err
	}
	return target, nil
}

func viewerEditorStaticIcon(editorID string) (string, bool) {
	if icon, ok := viewerEditorBrandIconByID(editorID); ok {
		svg := fmt.Sprintf(
			`<svg xmlns="http://www.w3.org/2000/svg" role="img" viewBox="0 0 24 24"><title>%s</title><path fill="#%s" d="%s"/></svg>`,
			template.HTMLEscapeString(icon.Title),
			template.HTMLEscapeString(icon.Hex),
			template.HTMLEscapeString(icon.Path),
		)
		return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg)), true
	}

	editor, ok := knownViewerEditorByID(editorID)
	if !ok {
		return "", false
	}
	source, ok := viewerEditorIconSource(editor)
	if !ok {
		return "", false
	}
	png, err := viewerEditorPNGIcon(editor.ID, source)
	if err != nil {
		return "", false
	}
	content, err := os.ReadFile(png)
	if err != nil {
		return "", false
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(content), true
}

func viewerEditorBrandIconByID(editorID string) (viewerEditorBrandIcon, bool) {
	icons := map[string]viewerEditorBrandIcon{
		"cursor": {
			Title: "Cursor",
			Hex:   "000000",
			Path:  "M11.503.131 1.891 5.678a.84.84 0 0 0-.42.726v11.188c0 .3.162.575.42.724l9.609 5.55a1 1 0 0 0 .998 0l9.61-5.55a.84.84 0 0 0 .42-.724V6.404a.84.84 0 0 0-.42-.726L12.497.131a1.01 1.01 0 0 0-.996 0M2.657 6.338h18.55c.263 0 .43.287.297.515L12.23 22.918c-.062.107-.229.064-.229-.06V12.335a.59.59 0 0 0-.295-.51l-9.11-5.257c-.109-.063-.064-.23.061-.23",
		},
		"windsurf": {
			Title: "Windsurf",
			Hex:   "0B100F",
			Path:  "M23.55 5.067c-1.2038-.002-2.1806.973-2.1806 2.1765v4.8676c0 .972-.8035 1.7594-1.7597 1.7594-.568 0-1.1352-.286-1.4718-.7659l-4.9713-7.1003c-.4125-.5896-1.0837-.941-1.8103-.941-1.1334 0-2.1533.9635-2.1533 2.153v4.8957c0 .972-.7969 1.7594-1.7596 1.7594-.57 0-1.1363-.286-1.4728-.7658L.4076 5.1598C.2822 4.9798 0 5.0688 0 5.2882v4.2452c0 .2147.0656.4228.1884.599l5.4748 7.8183c.3234.462.8006.8052 1.3509.9298 1.3771.313 2.6446-.747 2.6446-2.0977v-4.893c0-.972.7875-1.7593 1.7596-1.7593h.003a1.798 1.798 0 0 1 1.4718.7658l4.9723 7.0994c.4135.5905 1.05.941 1.8093.941 1.1587 0 2.1515-.9645 2.1515-2.153v-4.8948c0-.972.7875-1.7594 1.7596-1.7594h.194a.22.22 0 0 0 .2204-.2202v-4.622a.22.22 0 0 0-.2203-.2203Z",
		},
		"zed": {
			Title: "Zed Industries",
			Hex:   "084CCF",
			Path:  "M2.25 1.5a.75.75 0 0 0-.75.75v16.5H0V2.25A2.25 2.25 0 0 1 2.25 0h20.095c1.002 0 1.504 1.212.795 1.92L10.764 14.298h3.486V12.75h1.5v1.922a1.125 1.125 0 0 1-1.125 1.125H9.264l-2.578 2.578h11.689V9h1.5v9.375a1.5 1.5 0 0 1-1.5 1.5H5.185L2.562 22.5H21.75a.75.75 0 0 0 .75-.75V5.25H24v16.5A2.25 2.25 0 0 1 21.75 24H1.655C.653 24 .151 22.788.86 22.08L13.19 9.75H9.75v1.5h-1.5V9.375A1.125 1.125 0 0 1 9.375 8.25h5.314l2.625-2.625H5.625V15h-1.5V5.625a1.5 1.5 0 0 1 1.5-1.5h13.19L21.438 1.5z",
		},
		"sublime": {
			Title: "Sublime Text",
			Hex:   "FF9800",
			Path:  "M20.953.004a.397.397 0 0 0-.18.017L3.225 5.585c-.175.055-.323.214-.402.398a.42.42 0 0 0-.06.22v5.726a.42.42 0 0 0 .06.22c.079.183.227.341.402.397l7.454 2.364-7.454 2.363c-.255.08-.463.374-.463.655v5.688c0 .282.208.444.463.363l17.55-5.565c.237-.075.426-.336.452-.6.003-.022.013-.04.013-.065V12.06c0-.281-.208-.575-.463-.656L13.4 9.065l7.375-2.339c.255-.08.462-.375.462-.656V.384c0-.211-.117-.355-.283-.38z",
		},
		"obsidian": {
			Title: "Obsidian",
			Hex:   "7C3AED",
			Path:  "M19.355 18.538a68.967 68.959 0 0 0 1.858-2.954.81.81 0 0 0-.062-.9c-.516-.685-1.504-2.075-2.042-3.362-.553-1.321-.636-3.375-.64-4.377a1.707 1.707 0 0 0-.358-1.05l-3.198-4.064a3.744 3.744 0 0 1-.076.543c-.106.503-.307 1.004-.536 1.5-.134.29-.29.6-.446.914l-.31.626c-.516 1.068-.997 2.227-1.132 3.59-.124 1.26.046 2.73.815 4.481.128.011.257.025.386.044a6.363 6.363 0 0 1 3.326 1.505c.916.79 1.744 1.922 2.415 3.5zM8.199 22.569c.073.012.146.02.22.02.78.024 2.095.092 3.16.29.87.16 2.593.64 4.01 1.055 1.083.316 2.198-.548 2.355-1.664.114-.814.33-1.735.725-2.58l-.01.005c-.67-1.87-1.522-3.078-2.416-3.849a5.295 5.295 0 0 0-2.778-1.257c-1.54-.216-2.952.19-3.84.45.532 2.218.368 4.829-1.425 7.531zM5.533 9.938c-.023.1-.056.197-.098.29L2.82 16.059a1.602 1.602 0 0 0 .313 1.772l4.116 4.24c2.103-3.101 1.796-6.02.836-8.3-.728-1.73-1.832-3.081-2.55-3.831zM9.32 14.01c.615-.183 1.606-.465 2.745-.534-.683-1.725-.848-3.233-.716-4.577.154-1.552.7-2.847 1.235-3.95.113-.235.223-.454.328-.664.149-.297.288-.577.419-.86.217-.47.379-.885.46-1.27.08-.38.08-.72-.014-1.043-.095-.325-.297-.675-.68-1.06a1.6 1.6 0 0 0-1.475.36l-4.95 4.452a1.602 1.602 0 0 0-.513.952l-.427 2.83c.672.59 2.328 2.316 3.335 4.711.09.21.175.43.253.653z",
		},
		"intellij": {
			Title: "IntelliJ IDEA",
			Hex:   "000000",
			Path:  "M0 0v24h24V0zm3.723 3.111h5v1.834h-1.39v6.277h1.39v1.834h-5v-1.834h1.444V4.945H3.723zm11.055 0H17v6.5c0 .612-.055 1.111-.222 1.556-.167.444-.39.777-.723 1.11-.277.279-.666.557-1.11.668a3.933 3.933 0 0 1-1.445.278c-.778 0-1.444-.167-1.944-.445a4.81 4.81 0 0 1-1.279-1.056l1.39-1.555c.277.334.555.555.833.722.277.167.611.278.945.278.389 0 .721-.111 1-.389.221-.278.333-.667.333-1.278zM2.222 19.5h9V21h-9z",
		},
		"webstorm": {
			Title: "WebStorm",
			Hex:   "000000",
			Path:  "M0 0v24h24V0H0zm17.889 2.889c1.444 0 2.667.444 3.667 1.278l-1.111 1.667c-.889-.611-1.722-1-2.556-1s-1.278.389-1.278.889v.056c0 .667.444.889 2.111 1.333 2 .556 3.111 1.278 3.111 3v.056c0 2-1.5 3.111-3.611 3.111-1.5-.056-3-.611-4.167-1.667l1.278-1.556c.889.722 1.833 1.222 2.944 1.222.889 0 1.389-.333 1.389-.944v-.056c0-.556-.333-.833-2-1.278-2-.5-3.222-1.056-3.222-3.056v-.056c0-1.833 1.444-3 3.444-3zm-16.111.222h2.278l1.5 5.778 1.722-5.778h1.667l1.667 5.778 1.5-5.778h2.333l-2.833 9.944H9.723L8.112 7.277l-1.667 5.778H4.612L1.779 3.111zm.5 16.389h9V21h-9v-1.5z",
		},
		"neovim": {
			Title: "Neovim",
			Hex:   "57A143",
			Path:  "M2.214 4.954v13.615L7.655 24V10.314L3.312 3.845 2.214 4.954zm4.999 17.98l-4.557-4.548V5.136l.59-.596 3.967 5.908v12.485zm14.573-4.457l-.862.937-4.24-6.376V0l5.068 5.092.034 13.385zM7.431.001l12.998 19.835-3.637 3.637L3.787 3.683 7.43 0z",
		},
		"vim": {
			Title: "Vim",
			Hex:   "019733",
			Path:  "M24 11.986h-.027l-4.318-4.318 4.303-4.414V1.461l-.649-.648h-8.198l-.66.605v1.045L12.015.027V0L12 .014 11.986 0v.027l-1.29 1.291-.538-.539H2.035l-.638.692v1.885l.616.616h.72v5.31L.027 11.987H0L.014 12 0 12.014h.027l2.706 2.706v6.467l.907.523h2.322l1.857-1.904 4.166 4.166V24l.015-.014.014.014v-.028l2.51-2.509h.485c.111 0 .211-.07.25-.179l.146-.426c.028-.084.012-.172-.037-.239l1.462-1.462-.612 1.962c-.043.141.036.289.177.332.025.008.052.012.078.012h1.824c.106-.001.201-.064.243-.163l.165-.394c.025-.065.024-.138-.004-.203-.027-.065-.08-.116-.146-.142-.029-.012-.062-.019-.097-.02h-.075l.84-2.644h1.232l-1.016 3.221c-.043.141.036.289.176.332.025.008.052.012.079.012h2.002c.11 0 .207-.066.248-.17l.164-.428c.051-.138-.021-.29-.158-.341-.029-.011-.06-.017-.091-.017h-.145l1.131-3.673c.027-.082.012-.173-.039-.24l-.375-.504-.003-.005c-.051-.064-.127-.102-.209-.102h-1.436c-.071 0-.141.03-.19.081l-.4.439h-.624l-.042-.046 4.445-4.445H24L23.986 12l.014-.014zM9.838 21.139l1.579-4.509h-.501l.297-.304h1.659l-1.563 4.555h.623l-.079.258H9.838zm3.695-7.516l.15.151-.269.922-.225.226h-.969l-.181-.181.311-.871.288-.247h.895zM5.59 20.829H3.877l-.262-.15V3.091H2.379l-.1-.1V1.815l.143-.154h7.371l.213.214v1.108l-.142.173H8.785v8.688l8.807-8.688h-2.086l-.175-.188V1.805l.121-.111h7.49l.132.133v1.07L12.979 13.25h-.373c-.015-.001-.028 0-.042.001l-.02.003c-.045.01-.086.03-.119.06l-.343.295-.004.003c-.033.031-.059.069-.073.111l-.296.83-6.119 6.276zm14.768-3.952l.474-.519h1.334l.309.415-1.265 4.107h.493l-.08.209H19.84l1.124-3.564h-2.015l-1.077 3.391h.424l-.073.174h-1.605l1.107-3.548h-2.096l-1.062 3.339h.436l-.072.209H13.27l1.514-4.46H14.198l.091-.271h1.65l.519.537h.906l.491-.554h1.061l.489.535h.953z",
		},
		"emacs": {
			Title: "GNU Emacs",
			Hex:   "7F5AB6",
			Path:  "M12 24C5.448 24 .118 18.617.118 12S5.448 0 12 0c6.552 0 11.882 5.383 11.882 12S18.552 24 12 24zM12 .661C5.813.661.779 5.748.779 12S5.813 23.339 12 23.339c6.187 0 11.221-5.086 11.221-11.339S18.187.661 12 .661zM8.03 20.197s.978.069 2.236-.042c.51-.045 2.444-.235 3.891-.552 0 0 1.764-.377 2.707-.725.987-.364 1.524-.673 1.766-1.11-.011-.09.074-.408-.381-.599-1.164-.488-2.514-.4-5.185-.457-2.962-.102-3.948-.598-4.472-.997-.503-.405-.25-1.526 1.907-2.513 1.086-.526 5.345-1.496 5.345-1.496-1.434-.709-4.109-1.955-4.659-2.224-.482-.236-1.254-.591-1.421-1.021-.19-.413.448-.768.804-.87 1.147-.331 2.766-.536 4.24-.56.741-.012.861-.059.861-.059 1.022-.17 1.695-.869 1.414-1.976-.252-1.13-1.579-1.795-2.84-1.565-1.188.217-4.05 1.048-4.05 1.048 3.539-.031 4.131.028 4.395.398.156.218-.071.518-1.015.672-1.027.168-3.163.37-3.163.37-2.049.122-3.492.13-3.925 1.046C6.202 7.564 6.787 8.094 7.043 8.425c1.082 1.204 2.646 1.853 3.652 2.331.379.18 1.49.52 1.49.52-3.265-.18-5.619.823-7.001 1.977-1.562 1.445-.871 3.168 2.33 4.228 1.891.626 2.828.921 5.648.667 1.661-.09 1.923-.036 1.939.1.023.192-1.845.669-2.355.816-1.298.374-4.699 1.129-4.716 1.133z",
		},
	}
	icon, ok := icons[editorID]
	return icon, ok
}

func viewerTree(entries []okf.ListEntry) []viewerTreeItem {
	return viewerTreeWithURL(entries, fileURL)
}

func viewerTreeWithURL(entries []okf.ListEntry, urlFor func(string) string) []viewerTreeItem {
	var tree []viewerTreeItem
	seenDirectories := make(map[string]bool)
	for _, entry := range entries {
		segments := strings.Split(entry.Path, "/")
		for index := 0; index < len(segments)-1; index++ {
			directoryPath := strings.Join(segments[:index+1], "/")
			if seenDirectories[directoryPath] {
				continue
			}
			seenDirectories[directoryPath] = true
			tree = append(tree, viewerTreeItem{
				Name:      segments[index],
				Path:      directoryPath,
				Depth:     index,
				Indent:    10 + index*22,
				Directory: true,
			})
		}
		tree = append(tree, viewerTreeItem{
			Name:   segments[len(segments)-1],
			Path:   entry.Path,
			URL:    urlFor(entry.Path),
			Depth:  len(segments) - 1,
			Indent: 10 + (len(segments)-1)*22,
			System: viewerSystemMarkdownFile(entry),
		})
	}
	return tree
}

func viewerSystemMarkdownFile(entry okf.ListEntry) bool {
	if entry.Reserved {
		return true
	}
	name := filepath.Base(entry.Path)
	return strings.EqualFold(name, "index.md") || strings.EqualFold(name, "log.md")
}

func renderHTML(response http.ResponseWriter, tmpl *template.Template, data any) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(response, data); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
	}
}

func safeMarkdownPath(root string, rel string) (string, bool) {
	clean, ok := cleanMarkdownRel(rel)
	if !ok {
		return "", false
	}

	full := filepath.Join(root, filepath.FromSlash(clean))
	relative, err := filepath.Rel(root, full)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}

func cleanMarkdownRel(rel string) (string, bool) {
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
	return clean, true
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

func localAliasPrefix(name string) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	return "/" + url.PathEscape(name)
}

func localAliasURL(name string) string {
	prefix := localAliasPrefix(name)
	if prefix == "" {
		return "/"
	}
	return prefix + "/"
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

func viewerAliasDisplayURL(host string, port string, aliasNames []string) string {
	if len(aliasNames) == 1 {
		return viewerDisplayURL(host, port, localAliasPrefix(aliasNames[0]))
	}
	return viewerDisplayURL(host, port, "")
}

func viewerDisplayURLs(host string, port string, localDomain string, aliasNames []string) (string, string) {
	viewURL := viewerAliasDisplayURL(host, port, aliasNames)
	domain := strings.TrimSpace(localDomain)
	if domain == "" {
		return viewURL, ""
	}
	return viewURL, viewerAliasDisplayURL(domain, port, aliasNames)
}

func viewerDisplayURL(host string, port string, basePath string) string {
	return viewerDisplayBaseURL(host, port) + strings.TrimPrefix(viewerDisplayPath(basePath), "/")
}

func viewerDisplayBaseURL(host string, port string) string {
	hostPort := host
	if port != "" && port != "80" {
		hostPort = net.JoinHostPort(host, port)
	} else if port == "80" && strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		hostPort = "[" + host + "]"
	}
	return "http://" + hostPort + "/"
}

func viewerDisplayPath(basePath string) string {
	clean := strings.TrimRight(basePath, "/")
	if clean == "" {
		return "/"
	}
	return clean + "/"
}

func displayHostPort(address net.Addr) (string, string) {
	host, port, err := net.SplitHostPort(address.String())
	if err != nil {
		return address.String(), ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return host, port
}

func normalizeLocalAliasName(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	separator := false
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
			if separator && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			builder.WriteRune(char)
			separator = false
		case char >= 'A' && char <= 'Z':
			if separator && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			builder.WriteRune(char + ('a' - 'A'))
			separator = false
		case char >= '0' && char <= '9':
			if separator && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			builder.WriteRune(char)
			separator = false
		case char == '-' || char == '_' || char == '.':
			if builder.Len() > 0 {
				builder.WriteRune(char)
			}
			separator = false
		default:
			separator = true
		}
	}
	return strings.Trim(builder.String(), "-_.")
}

func openBrowser(target string) error {
	command, args, ok := browserOpenCommand(runtime.GOOS, target)
	if !ok {
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	cmd := exec.Command(command, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func browserOpenCommand(goos string, target string) (string, []string, bool) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", nil, false
	}
	switch goos {
	case "darwin":
		return "open", []string{target}, true
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", target}, true
	default:
		return "xdg-open", []string{target}, true
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
  <header>
    {{if .Frame.Workspaces}}<a class="brand" href="{{.Frame.ActiveURL}}">{{.Frame.ActiveName}}</a>{{else}}<a class="brand" href="/">Open Knowledge</a>{{end}}
    <span>{{.Root}}</span>
  </header>
  <main>
    {{if .Frame.Workspaces}}
      <section class="workspaces" aria-label="Knowledge bases">
        <div class="sidebar-label">Knowledge bases</div>
        <nav class="workspace-list">
          {{range .Frame.Workspaces}}
            <a class="workspace{{if .Active}} active{{end}}" href="{{.URL}}">
              <span class="workspace-name">{{.Name}}</span>
              <span class="workspace-root">{{.Root}}</span>
            </a>
          {{end}}
        </nav>
      </section>
    {{end}}
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
<body class="viewer-document is-stack-mode">
  <header>
    <div class="header-left">
      <button class="sidebar-toggle" type="button" data-sidebar-toggle aria-label="Open file explorer" aria-expanded="false" title="File explorer">
        <svg class="sidebar-toggle-icon control-icon" viewBox="0 0 24 24" aria-hidden="true">
          <rect x="3.5" y="4.5" width="17" height="15" rx="2"></rect>
          <path d="M9 4.5v15"></path>
          <path d="M6 8h.01"></path>
          <path d="M6 11h.01"></path>
          <path d="M6 14h.01"></path>
        </svg>
      </button>
      <a class="brand" href="/">Open Knowledge</a>
    </div>
    <section class="search header-search" role="search" aria-label="Search files" data-search-url="{{.SearchURL}}" data-primary-search>
      <label class="sr-only" for="viewer-search">Search</label>
      <div class="search-field">
        <input id="viewer-search" class="search-input" type="search" autocomplete="off" spellcheck="false" placeholder="Search">
        <kbd class="search-shortcut">⌘K</kbd>
      </div>
      <div class="search-status" aria-live="polite"></div>
      <div class="search-results" hidden></div>
    </section>
  </header>
  <aside class="file-sidebar" data-file-sidebar aria-label="File explorer" aria-hidden="true">
    <div class="file-sidebar-head">
      <span>Files</span>
      <button class="file-sidebar-close" type="button" data-sidebar-close aria-label="Close file explorer" title="Close">
        <svg class="note-close-icon control-icon" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M18 6 6 18"></path>
          <path d="m6 6 12 12"></path>
        </svg>
      </button>
    </div>
    <div class="file-sidebar-tree knowledge-tree" role="tree">
      {{range .Tree}}
        {{if .Directory}}
          <div class="tree-row tree-directory" role="treeitem" aria-expanded="true" style="--indent: {{.Indent}}px">{{.Name}}</div>
        {{else}}
          <a class="tree-row tree-file" role="treeitem" href="{{.URL}}" data-tree-path="{{.Path}}" style="--indent: {{.Indent}}px">
            <span class="tree-file-name">{{.Name}}</span>
            {{if .System}}<span class="tree-file-system">system</span>{{end}}
          </a>
        {{end}}
      {{else}}
        <p class="empty">No Markdown files found.</p>
      {{end}}
    </div>
  </aside>
  <main class="note-workspace" data-note-workspace data-note-root="{{.Root}}" data-link-prefix="{{.LinkPrefix}}">
    <section class="knowledge-empty" data-empty-state aria-label="Knowledge base files" hidden>
      <div class="knowledge-empty-inner">
        <div class="knowledge-empty-pane knowledge-empty-tree">
          <div class="knowledge-tree" role="tree" aria-label="Knowledge base files">
            {{range .Tree}}
              {{if .Directory}}
                <div class="tree-row tree-directory" role="treeitem" aria-expanded="true" style="--indent: {{.Indent}}px">{{.Name}}</div>
              {{else}}
                <a class="tree-row tree-file" role="treeitem" href="{{.URL}}" data-tree-path="{{.Path}}" style="--indent: {{.Indent}}px">
                  <span class="tree-file-name">{{.Name}}</span>
                  {{if .System}}<span class="tree-file-system">system</span>{{end}}
                </a>
              {{end}}
            {{else}}
              <p class="empty">No Markdown files found.</p>
            {{end}}
          </div>
        </div>
        <div class="knowledge-empty-pane knowledge-empty-graph" data-knowledge-graph-view aria-label="Knowledge graph"></div>
      </div>
    </section>
    <section class="note-stack" data-note-stack aria-label="Open notes">
      <article class="document note-panel is-active-panel" data-note-path="{{.Path}}" data-note-title="{{.Title}}" tabindex="-1">
        <div class="note-chrome">
          <a class="note-path" href="{{.FileURL}}" data-direct-link="true">{{.Path}}</a>
          <div class="note-actions">
            <div class="editor-picker" data-editor-picker>
              <div class="editor-trigger" data-editor-trigger role="group">
                <a class="editor-open" href="#" data-editor-open data-direct-link="true" aria-label="Open {{.Path}} in editor" title="Open in editor">
                  <span class="editor-mark" data-editor-mark aria-hidden="true">--</span>
                </a>
                <button class="editor-menu-trigger" type="button" data-editor-menu-trigger aria-haspopup="menu" aria-expanded="false" aria-label="Choose editor" title="Choose editor">
                  <svg class="editor-caret control-icon" data-icon="chevron-down" viewBox="0 0 24 24" aria-hidden="true">
                    <path d="m6 9 6 6 6-6"></path>
                  </svg>
                </button>
              </div>
              <div class="editor-menu" data-editor-menu role="menu" hidden></div>
            </div>
            <a class="note-close" href="#" data-close-panel aria-label="Close {{.Path}}" title="Close note" role="button">
              <svg class="note-close-icon control-icon" data-icon="x" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M18 6 6 18"></path>
                <path d="m6 6 12 12"></path>
              </svg>
            </a>
          </div>
        </div>
        <div class="note-body">
          {{.Body}}
        </div>
      </article>
    </section>
  </main>
  <script type="application/json" data-editor-options>{{.EditorsJSON}}</script>
  <script type="application/json" data-knowledge-graph>{{.GraphJSON}}</script>
  {{if .StaticJSON}}<script type="application/json" data-static-notes>{{.StaticJSON}}</script>{{end}}
  <script>` + viewerJS + `</script>
  <script>` + viewerSearchJS + `</script>
</body>
</html>`))

const viewerSearchJS = `
(() => {
  const searches = Array.from(document.querySelectorAll(".search"));
  if (searches.length === 0) return;
  const staticNotes = readStaticNotes();
  const primarySearch = document.querySelector("[data-primary-search]") || searches[0];
  const primaryInput = primarySearch?.querySelector(".search-input");

  searches.forEach(bindSearch);

  document.addEventListener("keydown", (event) => {
    if ((event.metaKey || event.ctrlKey) && !event.altKey && event.key.toLowerCase() === "k") {
      event.preventDefault();
      primaryInput?.focus();
      primaryInput?.select();
    }
  });

  function bindSearch(search) {
    const input = search.querySelector(".search-input");
    const results = search.querySelector(".search-results");
    const status = search.querySelector(".search-status");
    if (!input || !results || !status) {
      return;
    }
    const searchURL = search.dataset.searchUrl || "/api/search";
    let timer = 0;
    let controller = null;
    let activeIndex = -1;
    let sequence = 0;

    initializeSearchAccessibility(input, results);
    closeSearch(false);

    input.addEventListener("input", () => {
      window.clearTimeout(timer);
      setActiveResult(-1, false);
      if (!input.value.trim()) {
        renderDefaultResults(true);
        return;
      }
      timer = window.setTimeout(runSearch, 140);
    });
    input.addEventListener("focus", () => {
      if (!input.value.trim()) {
        renderDefaultResults(true);
        return;
      }
      if (searchResultLinks(results).length > 0) {
        setResultsOpen(true);
      } else {
        runSearch();
      }
    });
    input.addEventListener("keydown", (event) => {
      const links = searchResultLinks(results);
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        if (!links.length) {
          return;
        }
        event.preventDefault();
        const direction = event.key === "ArrowDown" ? 1 : -1;
        const nextIndex = activeIndex < 0
          ? (direction > 0 ? 0 : links.length - 1)
          : (activeIndex + direction + links.length) % links.length;
        setActiveResult(nextIndex, true);
        setResultsOpen(true);
        return;
      }
      if (event.key === "Enter") {
        const link = selectedSearchResult(results, activeIndex);
        if (!link) {
          return;
        }
        event.preventDefault();
        link.click();
        closeSearch(true);
        return;
      }
      if (event.key === "Escape" && (!results.hidden || input.value)) {
        event.preventDefault();
        closeSearch(true);
      }
    });
    results.addEventListener("mousemove", (event) => {
      const link = closestSearchResult(event.target);
      if (!link) {
        return;
      }
      const index = searchResultLinks(results).indexOf(link);
      if (index >= 0) {
        setActiveResult(index, false);
      }
    });
    results.addEventListener("focusin", (event) => {
      const link = closestSearchResult(event.target);
      if (!link) {
        return;
      }
      const index = searchResultLinks(results).indexOf(link);
      if (index >= 0) {
        setActiveResult(index, false);
      }
    });
    results.addEventListener("click", (event) => {
      const link = closestSearchResult(event.target);
      if (!link || isModifiedClick(event)) {
        return;
      }
      closeSearch(true);
    });

    async function runSearch() {
      const query = input.value.trim();
      if (!query) {
        renderDefaultResults(document.activeElement === input);
        return;
      }

      const requestID = ++sequence;
      setActiveResult(-1, false);

      if (staticNotes.length > 0) {
        renderResults(results, status, searchStaticNotes(query), query, setResultsOpen, setActiveResult);
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
        if (requestID !== sequence || input.value.trim() !== query) {
          return;
        }
        renderResults(results, status, payload.results || [], query, setResultsOpen, setActiveResult);
      } catch (error) {
        if (error.name === "AbortError") return;
        status.textContent = "Search failed.";
        setActiveResult(-1, false);
        setResultsOpen(false);
      }
    }

    function renderDefaultResults(open) {
      sequence += 1;
      window.clearTimeout(timer);
      if (controller) {
        controller.abort();
        controller = null;
      }
      const items = defaultSearchResults();
      status.textContent = items.length ? "Top files" : "";
      renderResults(results, status, items, "", setResultsOpen, setActiveResult, {
        emptyStatus: "",
        keepOpenWhenEmpty: open,
        statusText: items.length ? "Top files" : "",
      });
      setResultsOpen(open && items.length > 0);
    }

    function closeSearch(clearInput) {
      sequence += 1;
      window.clearTimeout(timer);
      if (controller) {
        controller.abort();
        controller = null;
      }
      if (clearInput) {
        input.value = "";
      }
      status.textContent = "";
      results.replaceChildren();
      setActiveResult(-1, false);
      setResultsOpen(false);
    }

    function setResultsOpen(open) {
      results.hidden = !open;
      input.setAttribute("aria-expanded", open ? "true" : "false");
      if (!open) {
        input.removeAttribute("aria-activedescendant");
      }
    }

    function setActiveResult(index, scroll) {
      const links = searchResultLinks(results);
      activeIndex = links.length ? (index + links.length) % links.length : -1;
      links.forEach((link, linkIndex) => {
        const selected = linkIndex === activeIndex;
        link.classList.toggle("is-active", selected);
        link.setAttribute("aria-selected", selected ? "true" : "false");
        if (selected) {
          input.setAttribute("aria-activedescendant", link.id);
          if (scroll) {
            link.scrollIntoView({ block: "nearest" });
          }
        }
      });
      if (activeIndex < 0) {
        input.removeAttribute("aria-activedescendant");
      }
    }
  }

  function renderResults(results, status, items, query, setResultsOpen, setActiveResult, options) {
    const config = options || {};
    results.replaceChildren();
    if (items.length === 0) {
      status.textContent = config.emptyStatus ?? "No results for \"" + query + "\".";
      setActiveResult(-1, false);
      setResultsOpen(Boolean(config.keepOpenWhenEmpty));
      return;
    }

    status.textContent = config.statusText || (items.length + " result" + (items.length === 1 ? "" : "s"));
    setResultsOpen(true);
    items.forEach((item, index) => {
      const link = document.createElement("a");
      link.className = "search-result";
      link.href = item.url || staticRelativeURL(item.path);
      link.id = results.id + "-option-" + index;
      link.setAttribute("role", "option");
      link.setAttribute("aria-selected", "false");

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
    });
    setActiveResult(0, false);
  }

  function defaultSearchResults() {
    const seen = new Set();
    const items = [];
    const links = Array.from(document.querySelectorAll("[data-tree-path]"));
    for (const link of links) {
      const path = link.dataset.treePath || "";
      if (!path || seen.has(path)) {
        continue;
      }
      seen.add(path);
      const title = link.querySelector(".tree-file-name")?.textContent?.trim() || path;
      items.push({
        path,
        title,
        url: link.getAttribute("href") || link.href,
      });
      if (items.length >= 12) {
        break;
      }
    }
    return items.sort(function (a, b) {
      if (isIndexMarkdownPath(a.path) !== isIndexMarkdownPath(b.path)) {
        return isIndexMarkdownPath(a.path) ? 1 : -1;
      }
      return 0;
    });
  }

  function isIndexMarkdownPath(path) {
    return String(path || "").split("/").pop().toLowerCase() === "index.md";
  }

  function initializeSearchAccessibility(input, results) {
    if (!results.id) {
      results.id = (input.id || "viewer-search") + "-results-" + Math.random().toString(36).slice(2);
    }
    results.setAttribute("role", "listbox");
    input.setAttribute("role", "combobox");
    input.setAttribute("aria-autocomplete", "list");
    input.setAttribute("aria-controls", results.id);
    input.setAttribute("aria-expanded", "false");
  }

  function searchResultLinks(results) {
    return Array.from(results.querySelectorAll(".search-result[href]"));
  }

  function selectedSearchResult(results, activeIndex) {
    const links = searchResultLinks(results);
    if (!links.length) {
      return null;
    }
    return links[activeIndex >= 0 ? activeIndex : 0] || links[0];
  }

  function closestSearchResult(target) {
    if (!target) {
      return null;
    }
    if (target.closest) {
      return target.closest(".search-result[href]");
    }
    return target.parentElement ? target.parentElement.closest(".search-result[href]") : null;
  }

  function isModifiedClick(event) {
    return event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey;
  }

  function readStaticNotes() {
    const source = document.querySelector("[data-static-notes]");
    if (!source) {
      return [];
    }
    try {
      const parsed = JSON.parse(source.textContent || "[]");
      return Array.isArray(parsed) ? parsed : [];
    } catch {
      return [];
    }
  }

  function searchStaticNotes(query) {
    const normalizedQuery = normalizeSearchText(query);
    return staticNotes
      .map(function (note) {
        const bodyText = htmlToText(note.body || "");
        const title = note.title || note.path || "";
        const path = note.path || "";
        const haystack = normalizeSearchText([title, path, bodyText].join(" "));
        const titleMatch = normalizeSearchText(title).includes(normalizedQuery);
        const pathMatch = normalizeSearchText(path).includes(normalizedQuery);
        const bodyMatch = haystack.includes(normalizedQuery);
        if (!bodyMatch) {
          return null;
        }
        const baseScore = (titleMatch ? 3 : 0) + (pathMatch ? 2 : 0) + 1;
        return {
          path,
          title,
          snippet: staticSnippet(bodyText, query),
          score: isIndexMarkdownPath(path) ? baseScore * 0.55 : baseScore,
        };
      })
      .filter(Boolean)
      .sort(function (a, b) {
        if (b.score !== a.score) {
          return b.score - a.score;
        }
        if (isIndexMarkdownPath(a.path) !== isIndexMarkdownPath(b.path)) {
          return isIndexMarkdownPath(a.path) ? 1 : -1;
        }
        return a.path.localeCompare(b.path);
      })
      .slice(0, 12);
  }

  function normalizeSearchText(value) {
    return String(value || "").toLowerCase();
  }

  function htmlToText(html) {
    const element = document.createElement("div");
    element.innerHTML = html;
    return element.textContent || "";
  }

  function staticSnippet(text, query) {
    const value = String(text || "").replace(/\s+/g, " ").trim();
    if (!value) {
      return "";
    }
    const index = value.toLowerCase().indexOf(String(query || "").toLowerCase());
    const start = Math.max(0, index < 0 ? 0 : index - 48);
    const end = Math.min(value.length, start + 140);
    return (start > 0 ? "..." : "") + value.slice(start, end) + (end < value.length ? "..." : "");
  }

  function staticRelativeURL(targetPath) {
    const currentPath = document.querySelector("[data-note-path]")?.dataset.notePath || "index.md";
    const currentHTML = staticHTMLPath(currentPath);
    const targetHTML = staticHTMLPath(targetPath);
    const currentDirectory = currentHTML.includes("/") ? currentHTML.slice(0, currentHTML.lastIndexOf("/") + 1) : "";
    return relativeStaticPath(currentDirectory, targetHTML);
  }

  function staticHTMLPath(path) {
    const extensionIndex = String(path || "").lastIndexOf(".");
    if (extensionIndex < 0) {
      return normalizeStaticPath(path + "/index.html");
    }
    return normalizeStaticPath(path.slice(0, extensionIndex) + ".html");
  }

  function relativeStaticPath(fromDirectory, targetPath) {
    const fromParts = normalizeStaticPath(fromDirectory).split("/").filter(Boolean);
    const targetParts = normalizeStaticPath(targetPath).split("/").filter(Boolean);
    while (fromParts.length && targetParts.length && fromParts[0] === targetParts[0]) {
      fromParts.shift();
      targetParts.shift();
    }
    const relativeParts = fromParts.map(function () { return ".."; }).concat(targetParts);
    return relativeParts.join("/") || ".";
  }

  function normalizeStaticPath(value) {
    const parts = String(value || "").replace(/\\/g, "/").split("/");
    const normalized = [];
    parts.forEach(function (part) {
      if (!part || part === ".") {
        return;
      }
      if (part === "..") {
        normalized.pop();
        return;
      }
      normalized.push(part);
    });
    return normalized.join("/");
  }
})();
`

const viewerJS = `
(function () {
  const workspace = document.querySelector("[data-note-workspace]");
  const stackEl = document.querySelector("[data-note-stack]");
  const emptyState = document.querySelector("[data-empty-state]");
  const fileSidebar = document.querySelector("[data-file-sidebar]");
  const sidebarToggle = document.querySelector("[data-sidebar-toggle]");
  const sidebarClose = document.querySelector("[data-sidebar-close]");

  if (!workspace || !stackEl) {
    return;
  }

  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  const editorStorageKey = "openknowledge.viewer.editorOrder";
  const editorOptions = readEditorOptions();
  const staticNotes = readStaticNotes();
  const staticNotesByPath = indexStaticNotes(staticNotes, "path");
  const staticNotePathByHTML = indexStaticNotePathsByHTML(staticNotes);
  const knowledgeGraph = readKnowledgeGraph();
  const linkPrefix = normalizeLinkPrefix(workspace.dataset.linkPrefix || "");

  function panels() {
    return Array.prototype.slice.call(stackEl.querySelectorAll("[data-note-path]"));
  }

  function closestElement(target, selector) {
    if (!target) {
      return null;
    }
    if (target.closest) {
      return target.closest(selector);
    }
    return target.parentElement ? target.parentElement.closest(selector) : null;
  }

  function setSidebarOpen(open) {
    document.body.classList.toggle("is-sidebar-open", open);
    if (fileSidebar) {
      fileSidebar.setAttribute("aria-hidden", open ? "false" : "true");
    }
    if (sidebarToggle) {
      sidebarToggle.setAttribute("aria-expanded", open ? "true" : "false");
    }
  }

  function notePathFromHref(href, sourcePath) {
    if (isStaticBundle()) {
      return staticNotePathFromHref(href, sourcePath);
    }

    let url;
    try {
      url = new URL(href, window.location.href);
    } catch {
      return null;
    }

    const filePrefix = serverFilePrefix();
    if (url.origin !== window.location.origin || !url.pathname.startsWith(filePrefix)) {
      return null;
    }

    const raw = url.pathname.slice(filePrefix.length) || "index.md";
    try {
      return decodeURIComponent(raw);
    } catch {
      return raw;
    }
  }

  function encodedNoteURL(prefix, path) {
    return prefix + path.split("/").map(encodeURIComponent).join("/");
  }

  function fileURL(path) {
    if (isStaticBundle()) {
      return staticRelativeURL(path);
    }
    return encodedNoteURL(serverFilePrefix(), path);
  }

  function apiURL(path) {
    return encodedNoteURL(serverAPIPrefix(), path);
  }

  function serverFilePrefix() {
    return linkPrefix + "/file/";
  }

  function serverAPIPrefix() {
    return linkPrefix + "/api/file/";
  }

  function normalizeLinkPrefix(value) {
    const trimmed = String(value || "").replace(/\/+$/, "");
    if (!trimmed) {
      return "";
    }
    return trimmed.startsWith("/") ? trimmed : "/" + trimmed;
  }

  function readStaticNotes() {
    const source = document.querySelector("[data-static-notes]");
    if (!source) {
      return [];
    }
    try {
      const parsed = JSON.parse(source.textContent || "[]");
      return Array.isArray(parsed) ? parsed : [];
    } catch {
      return [];
    }
  }

  function readKnowledgeGraph() {
    const source = document.querySelector("[data-knowledge-graph]");
    if (!source) {
      return { nodes: [], edges: [] };
    }
    try {
      const parsed = JSON.parse(source.textContent || "{}");
      return {
        nodes: Array.isArray(parsed.nodes) ? parsed.nodes : [],
        edges: Array.isArray(parsed.edges) ? parsed.edges : [],
      };
    } catch {
      return { nodes: [], edges: [] };
    }
  }

  function renderKnowledgeGraph() {
    const graphView = document.querySelector("[data-knowledge-graph-view]");
    if (!graphView) {
      return;
    }
    graphView.replaceChildren();
    if (!knowledgeGraph.nodes.length) {
      const empty = document.createElement("p");
      empty.className = "empty";
      empty.textContent = "No Markdown files found.";
      graphView.append(empty);
      return;
    }

    const width = 900;
    const height = 640;
    const centerX = width / 2;
    const centerY = height / 2;
    const radius = Math.max(180, Math.min(270, 118 + knowledgeGraph.nodes.length * 5));
    const positions = Object.create(null);
    const indexPath = knowledgeGraph.nodes.find(function (node) { return node.path === "index.md"; })?.path;
    const ringNodes = knowledgeGraph.nodes.filter(function (node) { return node.path !== indexPath; });

    if (indexPath) {
      positions[indexPath] = { x: centerX, y: centerY };
    }
    ringNodes.forEach(function (node, index) {
      const angle = (-Math.PI / 2) + (index / Math.max(ringNodes.length, 1)) * Math.PI * 2;
      positions[node.path] = {
        x: centerX + Math.cos(angle) * radius,
        y: centerY + Math.sin(angle) * radius,
      };
    });

    const svg = createSVGElement("svg");
    svg.setAttribute("class", "knowledge-graph-svg");
    svg.setAttribute("viewBox", "0 0 " + width + " " + height);
    svg.setAttribute("role", "img");
    svg.setAttribute("aria-label", "Connected graph of Markdown files");

    const edges = createSVGElement("g");
    edges.setAttribute("class", "knowledge-graph-edges");
    knowledgeGraph.edges.forEach(function (edge) {
      const source = positions[edge.source];
      const target = positions[edge.target];
      if (!source || !target) {
        return;
      }
      const line = createSVGElement("line");
      line.setAttribute("x1", source.x.toFixed(1));
      line.setAttribute("y1", source.y.toFixed(1));
      line.setAttribute("x2", target.x.toFixed(1));
      line.setAttribute("y2", target.y.toFixed(1));
      edges.append(line);
    });
    svg.append(edges);

    const nodes = createSVGElement("g");
    nodes.setAttribute("class", "knowledge-graph-nodes");
    knowledgeGraph.nodes.forEach(function (node) {
      const point = positions[node.path];
      if (!point) {
        return;
      }
      const group = createSVGElement("a");
      group.setAttribute("href", fileURL(node.path));
      group.dataset.graphPath = node.path;
      group.setAttribute("class", "knowledge-graph-node" + (node.path === indexPath ? " is-index-node" : ""));

      const title = createSVGElement("title");
      title.textContent = node.title || node.path;
      group.append(title);

      const circle = createSVGElement("circle");
      circle.setAttribute("cx", point.x.toFixed(1));
      circle.setAttribute("cy", point.y.toFixed(1));
      circle.setAttribute("r", node.path === indexPath ? "16" : "10");
      group.append(circle);

      const label = createSVGElement("text");
      label.setAttribute("x", point.x.toFixed(1));
      label.setAttribute("y", (point.y + (node.path === indexPath ? 31 : 25)).toFixed(1));
      label.textContent = graphNodeLabel(node);
      group.append(label);

      nodes.append(group);
    });
    svg.append(nodes);
    graphView.append(svg);
  }

  function createSVGElement(name) {
    return document.createElementNS("http://www.w3.org/2000/svg", name);
  }

  function graphNodeLabel(node) {
    const raw = node.title || node.path || "";
    const label = raw.includes("/") ? raw.slice(raw.lastIndexOf("/") + 1) : raw;
    return label.length > 18 ? label.slice(0, 17) + "..." : label;
  }

  function indexStaticNotes(notes, key) {
    const indexed = Object.create(null);
    notes.forEach(function (note) {
      if (note && typeof note[key] === "string") {
        indexed[note[key]] = note;
      }
    });
    return indexed;
  }

  function indexStaticNotePathsByHTML(notes) {
    const indexed = Object.create(null);
    notes.forEach(function (note) {
      if (note && typeof note.htmlPath === "string" && typeof note.path === "string") {
        indexed[normalizeStaticPath(note.htmlPath)] = note.path;
      }
    });
    return indexed;
  }

  function isStaticBundle() {
    return staticNotes.length > 0;
  }

  function staticHTMLPath(path) {
    const extensionIndex = String(path).lastIndexOf(".");
    if (extensionIndex < 0) {
      return normalizeStaticPath(path + "/index.html");
    }
    return normalizeStaticPath(path.slice(0, extensionIndex) + ".html");
  }

  function staticRelativeURL(targetPath) {
    const currentPath = currentStack()[0] || document.querySelector("[data-note-path]")?.dataset.notePath || "index.md";
    const currentHTML = staticHTMLPath(currentPath);
    const targetHTML = staticHTMLPath(targetPath);
    const currentDirectory = currentHTML.includes("/") ? currentHTML.slice(0, currentHTML.lastIndexOf("/") + 1) : "";
    return relativeStaticPath(currentDirectory, targetHTML);
  }

  function relativeStaticPath(fromDirectory, targetPath) {
    const fromParts = normalizeStaticPath(fromDirectory).split("/").filter(Boolean);
    const targetParts = normalizeStaticPath(targetPath).split("/").filter(Boolean);
    while (fromParts.length && targetParts.length && fromParts[0] === targetParts[0]) {
      fromParts.shift();
      targetParts.shift();
    }
    const relativeParts = fromParts.map(function () { return ".."; }).concat(targetParts);
    return relativeParts.join("/") || ".";
  }

  function normalizeStaticPath(value) {
    const parts = String(value || "").replace(/\\/g, "/").split("/");
    const normalized = [];
    parts.forEach(function (part) {
      if (!part || part === ".") {
        return;
      }
      if (part === "..") {
        normalized.pop();
        return;
      }
      normalized.push(part);
    });
    return normalized.join("/");
  }

  function staticNotePathFromHref(href, sourcePath) {
    const raw = String(href || "").trim();
    if (!raw || raw.startsWith("#")) {
      return null;
    }

    const withoutFragment = raw.split("#")[0].split("?")[0];
    if (!withoutFragment) {
      return sourcePath || null;
    }

    if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(withoutFragment) && !withoutFragment.startsWith("/")) {
      const sourceHTML = staticHTMLPath(sourcePath || currentStack()[0] || "index.md");
      const sourceDirectory = sourceHTML.includes("/") ? sourceHTML.slice(0, sourceHTML.lastIndexOf("/") + 1) : "";
      return staticNotePathByHTML[normalizeStaticPath(sourceDirectory + withoutFragment)] || null;
    }

    let url;
    try {
      url = new URL(withoutFragment, window.location.href);
    } catch {
      return null;
    }
    if (url.origin !== window.location.origin) {
      return null;
    }

    return staticNotePathByHTML[staticRelativeHTMLPathFromURL(url)] || null;
  }

  function staticRelativeHTMLPathFromURL(url) {
    const currentPath = document.querySelector("[data-note-path]")?.dataset.notePath || currentStack()[0] || "index.md";
    const currentHTML = staticHTMLPath(currentPath);
    let currentURLPath = safeDecodePath(window.location.pathname);
    let targetURLPath = safeDecodePath(url.pathname);
    const rootPrefix = currentURLPath.endsWith(currentHTML)
      ? currentURLPath.slice(0, currentURLPath.length - currentHTML.length)
      : currentURLPath.slice(0, currentURLPath.lastIndexOf("/") + 1);
    if (rootPrefix && targetURLPath.startsWith(rootPrefix)) {
      targetURLPath = targetURLPath.slice(rootPrefix.length);
    }
    return normalizeStaticPath(targetURLPath);
  }

  function safeDecodePath(value) {
    try {
      return decodeURIComponent(value || "");
    } catch {
      return value || "";
    }
  }

  function absoluteNotePath(notePath) {
    const root = workspace.dataset.noteRoot || "";
    const separator = root.includes("\\") ? "\\" : "/";
    const cleanRoot = root.replace(/[\\/]+$/, "");
    const localPath = String(notePath || "").split("/").join(separator);
    return cleanRoot ? cleanRoot + separator + localPath : localPath;
  }

  function encodedAbsolutePath(absolutePath) {
    const normalized = String(absolutePath || "").replace(/\\/g, "/");
    const leadingSlash = normalized.startsWith("/") ? "/" : "";
    return leadingSlash + normalized.split("/").filter(Boolean).map(encodeURIComponent).join("/");
  }

  function fileDeepLink(absolutePath) {
    const encoded = encodedAbsolutePath(absolutePath);
    return "file://" + (encoded.startsWith("/") ? "" : "/") + encoded;
  }

  function editorDeepLink(editor, notePath) {
    const absolutePath = absoluteNotePath(notePath);
    const encodedPath = encodedAbsolutePath(absolutePath);
    const editorPath = encodedPath.startsWith("/") ? encodedPath : "/" + encodedPath;
    const fileLink = fileDeepLink(absolutePath);

    switch (editor.id) {
      case "code":
        return "vscode://file" + editorPath;
      case "cursor":
        return "cursor://file" + editorPath;
      case "windsurf":
        return "windsurf://file" + editorPath;
      case "zed":
        return "zed://file" + editorPath;
      case "obsidian":
        return "obsidian://open?path=" + encodeURIComponent(absolutePath);
      case "sublime":
        return "sublime://open?url=" + encodeURIComponent(fileLink);
      case "bbedit":
        return "bbedit://open?url=" + encodeURIComponent(fileLink);
      case "nova":
        return "nova://open?path=" + encodeURIComponent(absolutePath);
      case "intellij":
        return "idea://open?file=" + encodeURIComponent(absolutePath);
      case "webstorm":
        return "webstorm://open?file=" + encodeURIComponent(absolutePath);
      default:
        return fileLink;
    }
  }

  function readEditorOptions() {
    const fallback = [
      { id: "code", name: "Visual Studio Code", short: "VS", available: false },
      { id: "cursor", name: "Cursor", short: "Cu", available: false },
      { id: "windsurf", name: "Windsurf", short: "Ws", available: false },
      { id: "zed", name: "Zed", short: "Zd", available: false }
    ];
    const source = document.querySelector("[data-editor-options]");
    if (!source) {
      return fallback;
    }
    try {
      const parsed = JSON.parse(source.textContent || "[]");
      return Array.isArray(parsed) && parsed.length ? parsed : fallback;
    } catch {
      return fallback;
    }
  }

  function editorByID(editorID) {
    return editorOptions.find(function (editor) {
      return editor.id === editorID;
    }) || editorOptions[0];
  }

  function editorFallbackLabel(editor) {
    return editor.short || editor.name.slice(0, 2);
  }

  function renderEditorMark(mark, editor) {
    mark.replaceChildren();
    mark.dataset.hasIcon = editor.icon ? "true" : "false";

    if (!editor.icon) {
      mark.textContent = editorFallbackLabel(editor);
      return;
    }

    const image = document.createElement("img");
    image.className = "editor-icon";
    image.src = editor.icon;
    image.alt = "";
    image.decoding = "async";
    image.draggable = false;
    image.addEventListener("error", function () {
      mark.dataset.hasIcon = "false";
      mark.replaceChildren();
      mark.textContent = editorFallbackLabel(editor);
    }, { once: true });
    mark.append(image);
  }

  function controlIcon(name, className) {
    const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    svg.setAttribute("class", className + " control-icon");
    svg.setAttribute("data-icon", name);
    svg.setAttribute("viewBox", "0 0 24 24");
    svg.setAttribute("aria-hidden", "true");

    if (name === "chevron-down") {
      const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
      path.setAttribute("d", "m6 9 6 6 6-6");
      svg.append(path);
      return svg;
    }

    const first = document.createElementNS("http://www.w3.org/2000/svg", "path");
    first.setAttribute("d", "M18 6 6 18");
    const second = document.createElementNS("http://www.w3.org/2000/svg", "path");
    second.setAttribute("d", "m6 6 12 12");
    svg.append(first, second);
    return svg;
  }

  function readEditorOrder() {
    let stored = [];
    try {
      stored = JSON.parse(window.localStorage.getItem(editorStorageKey) || "[]");
    } catch {
      stored = [];
    }
    if (!Array.isArray(stored)) {
      stored = [];
    }

    const known = new Set(editorOptions.map(function (editor) {
      return editor.id;
    }));
    const ordered = stored.filter(function (editorID, index) {
      return typeof editorID === "string" && known.has(editorID) && stored.indexOf(editorID) === index;
    });
    editorOptions.forEach(function (editor) {
      if (!ordered.includes(editor.id)) {
        ordered.push(editor.id);
      }
    });
    return ordered;
  }

  function orderedEditors() {
    return readEditorOrder().map(editorByID).filter(Boolean);
  }

  function activeEditor() {
    return orderedEditors()[0] || editorOptions[0];
  }

  function savePrimaryEditor(editorID) {
    const nextOrder = [editorID].concat(readEditorOrder().filter(function (candidateID) {
      return candidateID !== editorID;
    }));
    try {
      window.localStorage.setItem(editorStorageKey, JSON.stringify(nextOrder));
    } catch {
      return;
    }
  }

  function createEditorPicker() {
    const picker = document.createElement("div");
    picker.className = "editor-picker";
    picker.dataset.editorPicker = "";

    const trigger = document.createElement("div");
    trigger.className = "editor-trigger";
    trigger.dataset.editorTrigger = "";
    trigger.setAttribute("role", "group");

    const openLink = document.createElement("a");
    openLink.className = "editor-open";
    openLink.href = "#";
    openLink.dataset.editorOpen = "";
    openLink.dataset.directLink = "true";
    openLink.title = "Open in editor";

    const mark = document.createElement("span");
    mark.className = "editor-mark";
    mark.dataset.editorMark = "";
    mark.setAttribute("aria-hidden", "true");
    mark.textContent = "--";
    openLink.append(mark);

    const menuButton = document.createElement("button");
    menuButton.className = "editor-menu-trigger";
    menuButton.type = "button";
    menuButton.dataset.editorMenuTrigger = "";
    menuButton.setAttribute("aria-haspopup", "menu");
    menuButton.setAttribute("aria-expanded", "false");
    menuButton.setAttribute("aria-label", "Choose editor");
    menuButton.title = "Choose editor";
    menuButton.append(controlIcon("chevron-down", "editor-caret"));

    trigger.append(openLink, menuButton);

    const menu = document.createElement("div");
    menu.className = "editor-menu";
    menu.dataset.editorMenu = "";
    menu.hidden = true;
    menu.setAttribute("role", "menu");

    picker.append(trigger, menu);
    return picker;
  }

  function renderEditorPicker(picker) {
    const trigger = picker.querySelector("[data-editor-trigger]");
    const openLink = picker.querySelector("[data-editor-open]");
    const menuButton = picker.querySelector("[data-editor-menu-trigger]");
    const mark = picker.querySelector("[data-editor-mark]");
    const menu = picker.querySelector("[data-editor-menu]");
    const ordered = orderedEditors();
    const selected = ordered[0];
    const panel = picker.closest("[data-note-path]");
    const notePath = panel?.dataset.notePath || "";
    if (!trigger || !openLink || !menuButton || !mark || !menu || !selected || !notePath) {
      return;
    }

    renderEditorMark(mark, selected);
    trigger.setAttribute("aria-label", "Editor: " + selected.name);
    openLink.href = editorDeepLink(selected, notePath);
    openLink.title = "Open " + notePath + " in " + selected.name;
    openLink.setAttribute("aria-label", "Open " + notePath + " in " + selected.name);
    menuButton.title = "Choose editor";
    menuButton.setAttribute("aria-label", "Choose editor for " + notePath);
    picker.dataset.primaryEditor = selected.id;

    menu.replaceChildren();
    appendEditorMenuItem(menu, selected, true);
    if (ordered.length > 1) {
      const separator = document.createElement("div");
      separator.className = "editor-menu-separator";
      separator.setAttribute("role", "separator");
      menu.append(separator);
    }
    ordered.slice(1).forEach(function (editor) {
      appendEditorMenuItem(menu, editor, false);
    });
  }

  function appendEditorMenuItem(menu, editor, selected) {
    const item = document.createElement("button");
    item.className = "editor-menu-item" + (selected ? " is-selected" : "");
    item.type = "button";
    item.dataset.editorOption = editor.id;
    item.setAttribute("role", "menuitemradio");
    item.setAttribute("aria-checked", selected ? "true" : "false");

    const mark = document.createElement("span");
    mark.className = "editor-option-mark";
    renderEditorMark(mark, editor);

    const label = document.createElement("span");
    label.className = "editor-option-label";
    label.textContent = editor.name;

    item.append(mark, label);
    menu.append(item);
  }

  function renderAllEditorPickers() {
    document.querySelectorAll("[data-editor-picker]").forEach(renderEditorPicker);
  }

  function setEditorMenuOpen(picker, open) {
    const menuButton = picker.querySelector("[data-editor-menu-trigger]");
    const menu = picker.querySelector("[data-editor-menu]");
    if (!menuButton || !menu) {
      return;
    }
    if (open) {
      closeEditorMenus(picker);
      renderEditorPicker(picker);
    }
    menu.hidden = !open;
    menuButton.setAttribute("aria-expanded", open ? "true" : "false");
  }

  function closeEditorMenus(exceptPicker) {
    document.querySelectorAll("[data-editor-picker]").forEach(function (picker) {
      if (picker === exceptPicker) {
        return;
      }
      setEditorMenuOpen(picker, false);
    });
  }

  function activePanel() {
    return stackEl.querySelector(".note-panel.is-active-panel");
  }

  function setActivePanel(panel) {
    if (!panel || !stackEl.contains(panel)) {
      return;
    }

    panels().forEach(function (item) {
      const active = item === panel;
      item.classList.toggle("is-active-panel", active);
      item.dataset.activePanel = active ? "true" : "false";
      if (!active) {
        item.querySelectorAll("[data-editor-picker]").forEach(function (picker) {
          setEditorMenuOpen(picker, false);
        });
      }
    });
    updateTitle();
  }

  function ensureActivePanel() {
    const all = panels();
    if (!all.length) {
      return;
    }
    if (!activePanel()) {
      setActivePanel(all[all.length - 1]);
    }
  }

  function bindEditorPicker(picker) {
    if (!picker || picker.dataset.editorBound === "true") {
      return;
    }
    picker.dataset.editorBound = "true";
    renderEditorPicker(picker);

    const menuButton = picker.querySelector("[data-editor-menu-trigger]");
    const menu = picker.querySelector("[data-editor-menu]");
    if (!menuButton || !menu) {
      return;
    }

    menuButton.addEventListener("click", function (event) {
      event.preventDefault();
      event.stopPropagation();
      setEditorMenuOpen(picker, menu.hidden);
    });
    menuButton.addEventListener("keydown", function (event) {
      if (event.key !== "ArrowDown" && event.key !== "Enter" && event.key !== " ") {
        return;
      }
      event.preventDefault();
      setEditorMenuOpen(picker, true);
      const firstItem = menu.querySelector("[data-editor-option]");
      if (firstItem) {
        firstItem.focus();
      }
    });

    menu.addEventListener("click", function (event) {
      const item = closestElement(event.target, "[data-editor-option]");
      if (!item) {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      savePrimaryEditor(item.dataset.editorOption);
      renderAllEditorPickers();
      closeEditorMenus();
    });
    menu.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        event.preventDefault();
        setEditorMenuOpen(picker, false);
        menuButton.focus();
        return;
      }
      if (event.key !== "Enter" && event.key !== " ") {
        return;
      }
      const item = closestElement(event.target, "[data-editor-option]");
      if (!item) {
        return;
      }
      event.preventDefault();
      savePrimaryEditor(item.dataset.editorOption);
      renderAllEditorPickers();
      closeEditorMenus();
      menuButton.focus();
    });
  }

  function currentStack() {
    return panels().map(function (panel) {
      return panel.dataset.notePath;
    });
  }

  function stackFromLocation() {
    const params = new URLSearchParams(window.location.search);
    if (params.get("empty") === "1") {
      return [];
    }
    const base = notePathFromHref(window.location.href) || currentStack()[0] || "index.md";
    return [base].concat(params.getAll("stack").filter(Boolean));
  }

  function stackURL(paths) {
    if (!paths.length) {
      const emptyURL = new URL(fileURL("index.md"), window.location.href);
      emptyURL.searchParams.set("empty", "1");
      return emptyURL;
    }

    const url = new URL(fileURL(paths[0] || "index.md"), window.location.href);
    paths.slice(1).forEach(function (path) {
      url.searchParams.append("stack", path);
    });
    return url;
  }

  function updateWorkspaceState() {
    const isEmpty = panels().length === 0;
    workspace.classList.toggle("is-empty", isEmpty);
    if (emptyState) {
      emptyState.hidden = !isEmpty;
    }
    ensureActivePanel();
    updateCloseLinks();
  }

  function updateCloseLinks() {
    const paths = currentStack();
    panels().forEach(function (panel, index) {
      const closeLink = panel.querySelector("[data-close-panel]");
      if (!closeLink) {
        return;
      }
      const nextPaths = paths.filter(function (_path, pathIndex) {
        return pathIndex !== index;
      });
      closeLink.href = stackURL(nextPaths).href;
    });
  }

  function updateTitle() {
    const all = panels();
    const currentPanel = activePanel() || all[all.length - 1];
    if (!currentPanel) {
      document.title = "Knowledge base - Open Knowledge";
      return;
    }
    const title = currentPanel?.dataset.noteTitle || currentPanel?.dataset.notePath || "Open Knowledge";
    document.title = title + " - Open Knowledge";
  }

  function updateHistory(paths, pushHistory) {
    const nextURL = stackURL(paths);
    const state = { stack: paths };
    if (pushHistory) {
      window.history.pushState(state, "", nextURL);
    } else {
      window.history.replaceState(state, "", nextURL);
    }
  }

  function updateActiveLinks() {
    const all = panels();
    all.forEach(function (panel, index) {
      panel.querySelectorAll(".note-body a.is-active-note").forEach(function (link) {
        link.classList.remove("is-active-note");
        link.removeAttribute("aria-current");
      });
    });

    all.forEach(function (panel, index) {
      const nextPath = all[index + 1]?.dataset.notePath;
      if (!nextPath) {
        return;
      }

      panel.querySelectorAll(".note-body a[href]").forEach(function (link) {
        if (notePathFromHref(link.getAttribute("href") || link.href, panel.dataset.notePath) === nextPath) {
          link.classList.add("is-active-note");
          link.setAttribute("aria-current", "true");
        }
      });
    });
  }

  function scrollToPanel(panel) {
    setActivePanel(panel);
    window.requestAnimationFrame(function () {
      panel.scrollIntoView({
        block: "nearest",
        inline: "end",
        behavior: reduceMotion.matches ? "auto" : "smooth"
      });
      panel.focus({ preventScroll: true });
    });
  }

  async function fetchNote(path) {
    if (isStaticBundle()) {
      const note = staticNotesByPath[path];
      if (!note) {
        throw new Error("Could not open " + path);
      }
      return note;
    }

    const response = await fetch(apiURL(path), {
      headers: { "Accept": "application/json" }
    });
    if (!response.ok) {
      throw new Error("Could not open " + path);
    }
    return response.json();
  }

  function createPanel(data, animate) {
    const panel = document.createElement("article");
    panel.className = "document note-panel" + (animate ? " is-entering" : "");
    panel.dataset.notePath = data.path;
    panel.dataset.noteTitle = data.title || data.path;
    panel.tabIndex = -1;

    const chrome = document.createElement("div");
    chrome.className = "note-chrome";

    const pathLink = document.createElement("a");
    pathLink.className = "note-path";
    pathLink.href = fileURL(data.path);
    pathLink.dataset.directLink = "true";
    pathLink.textContent = data.path;
    chrome.append(pathLink);

    const actions = document.createElement("div");
    actions.className = "note-actions";
    actions.append(createEditorPicker());

    const closeButton = document.createElement("a");
    closeButton.className = "note-close";
    closeButton.href = "#";
    closeButton.dataset.closePanel = "";
    closeButton.setAttribute("role", "button");
    closeButton.setAttribute("aria-label", "Close " + data.path);
    closeButton.title = "Close note";
    closeButton.append(controlIcon("x", "note-close-icon"));
    actions.append(closeButton);
    chrome.append(actions);

    const body = document.createElement("div");
    body.className = "note-body";
    body.innerHTML = data.body;

    panel.append(chrome, body);
    bindPanel(panel);
    return panel;
  }

  function bindPanel(panel) {
    panel.querySelectorAll("[data-editor-picker]").forEach(bindEditorPicker);

    const closeButton = panel.querySelector("[data-close-panel]");
    if (!closeButton || closeButton.dataset.closeBound === "true") {
      return;
    }
    closeButton.dataset.closeBound = "true";
    closeButton.addEventListener("click", function (event) {
      event.preventDefault();
      event.stopPropagation();
      closePanel(panel, true);
    });
    closeButton.addEventListener("keydown", function (event) {
      if (event.key !== " " && event.key !== "Enter") {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      closePanel(panel, true);
    });
  }

  function createErrorPanel(path, error) {
    const message = document.createElement("p");
    message.className = "note-error";
    const detail = error instanceof Error ? error.message : "";
    message.textContent = detail === "Failed to fetch"
      ? "Could not reach the local viewer server while opening " + path + ". Restart openknowledge open and refresh this page."
      : detail || "Could not open " + path;
    return createPanel({
      title: "Not found",
      path,
      body: message.outerHTML
    }, true);
  }

  async function panelForPath(path, animate) {
    try {
      return createPanel(await fetchNote(path), animate);
    } catch (error) {
      return createErrorPanel(path, error);
    }
  }

  function appendPanel(panel) {
    stackEl.append(panel);
    setActivePanel(panel);
    updateWorkspaceState();
    updateActiveLinks();
    updateTitle();
    scrollToPanel(panel);
  }

  async function appendNote(path, animate) {
    appendPanel(await panelForPath(path, animate));
  }

  function canUseStackTransition() {
    return !reduceMotion.matches && typeof document.startViewTransition === "function";
  }

  function clearEnteringPanels() {
    stackEl.querySelectorAll(".note-panel.is-entering").forEach(function (panel) {
      panel.classList.remove("is-entering");
    });
  }

  async function runStackTransition(mutator) {
    if (!canUseStackTransition()) {
      return mutator();
    }

    document.body.classList.add("is-view-transitioning");
    try {
      const transition = document.startViewTransition(mutator);
      await transition.finished;
    } finally {
      clearEnteringPanels();
      document.body.classList.remove("is-view-transitioning");
    }
  }

  function clearStack() {
    panels().forEach(function (panel) {
      panel.remove();
    });
    updateWorkspaceState();
    updateActiveLinks();
    updateTitle();
  }

  function trimAfter(index) {
    panels().slice(index + 1).forEach(function (panel) {
      panel.remove();
    });
    updateWorkspaceState();
    updateActiveLinks();
    updateTitle();
  }

  async function openInitialNote(targetPath, pushHistory) {
    const panel = await panelForPath(targetPath, true);
    await runStackTransition(function () {
      clearStack();
      appendPanel(panel);
      updateHistory(currentStack(), pushHistory);
    });
  }

  async function closePanel(panel, pushHistory) {
    const before = panels();
    const index = before.indexOf(panel);
    let nextPanel;

    await runStackTransition(function () {
      panel.remove();

      const remaining = panels();
      updateWorkspaceState();
      updateActiveLinks();
      updateTitle();
      updateHistory(currentStack(), pushHistory);

      if (!remaining.length) {
        return;
      }

      nextPanel = remaining[Math.min(Math.max(index, 0), remaining.length - 1)];
      setActivePanel(nextPanel);
    });

    if (!nextPanel) {
      return;
    }
    scrollToPanel(nextPanel);
  }

  async function openFromPanel(sourcePanel, targetPath, pushHistory) {
    const panel = await panelForPath(targetPath, true);
    await runStackTransition(function () {
      const all = panels();
      let sourceIndex = all.indexOf(sourcePanel);
      if (sourceIndex < 0) {
        sourceIndex = all.length - 1;
      }

      trimAfter(sourceIndex);
      appendPanel(panel);

      updateHistory(currentStack(), pushHistory);
    });
  }

  async function restoreStack(paths) {
    const loadedPanels = [];
    for (const path of paths) {
      loadedPanels.push(await panelForPath(path, false));
    }

    await runStackTransition(function () {
      clearStack();
      loadedPanels.forEach(function (panel) {
        stackEl.append(panel);
      });
      ensureActivePanel();
      updateWorkspaceState();
      updateActiveLinks();
      updateTitle();
      const active = activePanel();
      if (active) {
        scrollToPanel(active);
      }
    });
  }

  workspace.addEventListener("click", function (event) {
    const clickedPanel = closestElement(event.target, "[data-note-path]");
    if (clickedPanel) {
      setActivePanel(clickedPanel);
    }

    const closeButton = closestElement(event.target, "[data-close-panel]");
    if (closeButton) {
      const panel = closeButton.closest("[data-note-path]");
      if (!panel) {
        return;
      }
      event.preventDefault();
      closePanel(panel, true);
      return;
    }

    const treeLink = closestElement(event.target, "[data-tree-path]");
    const graphLink = closestElement(event.target, "[data-graph-path]");
    if (treeLink || graphLink) {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
        return;
      }
      event.preventDefault();
      openInitialNote(treeLink?.dataset.treePath || graphLink.dataset.graphPath, true);
      return;
    }

    const link = closestElement(event.target, "a[href]");
    if (!link || link.dataset.directLink === "true") {
      return;
    }
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
      return;
    }

    const sourcePanel = link.closest("[data-note-path]");
    if (!sourcePanel) {
      return;
    }

    const targetPath = notePathFromHref(link.getAttribute("href") || link.href, sourcePanel.dataset.notePath);
    if (!targetPath) {
      return;
    }

    event.preventDefault();
    openFromPanel(sourcePanel, targetPath, true);
  });

  workspace.addEventListener("focusin", function (event) {
    const focusedPanel = closestElement(event.target, "[data-note-path]");
    if (focusedPanel) {
      setActivePanel(focusedPanel);
    }
  });

  if (sidebarToggle) {
    sidebarToggle.addEventListener("click", function () {
      setSidebarOpen(!document.body.classList.contains("is-sidebar-open"));
    });
  }
  if (sidebarClose) {
    sidebarClose.addEventListener("click", function () {
      setSidebarOpen(false);
      sidebarToggle?.focus();
    });
  }
  if (fileSidebar) {
    fileSidebar.addEventListener("click", function (event) {
      const treeLink = closestElement(event.target, "[data-tree-path]");
      const link = treeLink || closestElement(event.target, "a[href]");
      if (!link) {
        return;
      }
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
        return;
      }
      const targetPath = treeLink?.dataset.treePath || notePathFromHref(link.getAttribute("href") || link.href);
      if (!targetPath) {
        return;
      }
      event.preventDefault();
      closeSearchResults(link);
      openInitialNote(targetPath, true);
    });
  }

  function closeSearchResults(source) {
    const search = closestElement(source, ".search");
    if (!search) {
      return;
    }
    const input = search.querySelector(".search-input");
    const results = search.querySelector(".search-results");
    const status = search.querySelector(".search-status");
    if (input) {
      input.value = "";
    }
    if (status) {
      status.textContent = "";
    }
    if (results) {
      results.hidden = true;
      results.replaceChildren();
    }
  }

  window.addEventListener("popstate", function () {
    const paths = stackFromLocation();
    restoreStack(paths);
  });

  document.addEventListener("click", function (event) {
    const searchResult = closestElement(event.target, ".search-result[href]");
    if (searchResult) {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
        return;
      }
      const targetPath = notePathFromHref(searchResult.getAttribute("href") || searchResult.href);
      if (targetPath) {
        event.preventDefault();
        closeSearchResults(searchResult);
        openInitialNote(targetPath, true);
        return;
      }
    }

    if (!closestElement(event.target, "[data-editor-picker]")) {
      closeEditorMenus();
    }
  });
  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape") {
      closeEditorMenus();
      setSidebarOpen(false);
    }
  });

  const requestedStack = stackFromLocation();
  renderKnowledgeGraph();
  panels().forEach(bindPanel);
  ensureActivePanel();
  if (requestedStack.length !== 1 || requestedStack[0] !== panels()[0]?.dataset.notePath) {
    window.history.replaceState({ stack: requestedStack }, "", window.location.href);
    restoreStack(requestedStack);
  } else {
    window.history.replaceState({ stack: requestedStack }, "", window.location.href);
    updateWorkspaceState();
    updateActiveLinks();
    updateTitle();
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
  --accent: #0f7a4d;
  --accent-rgb: 15, 122, 77;
  --shadow: rgba(24, 34, 30, .12);
  --header-height: 52px;
  --sidebar-width: min(340px, calc(100vw - 36px));
  --sidebar-bg: #e2e2e2;
  --sidebar-head-bg: #d8d8d8;
  --sidebar-row-bg: #d0d0d0;
  font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
* { box-sizing: border-box; }
body { margin: 0; color: var(--ink); background: var(--paper); line-height: 1.55; }
body.viewer-document { height: 100vh; overflow: hidden; }
header { display: flex; min-height: var(--header-height); justify-content: space-between; align-items: center; gap: 16px; padding: 14px 22px; border-bottom: 1px solid var(--line); background: rgba(255, 255, 255, .92); color: var(--muted); font-size: 13px; }
body.viewer-document > header { position: relative; justify-content: center; border-bottom: 0; background: #f0f0f0; transition: transform .22s cubic-bezier(.22, .8, .2, 1); }
body.viewer-document.is-sidebar-open > header { transform: translateX(var(--sidebar-width)); }
.header-left { position: absolute; left: 22px; top: 50%; display: inline-flex; min-width: 0; align-items: center; gap: 10px; transform: translateY(-50%); }
.brand { color: var(--ink); font-weight: 700; text-decoration: none; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); clip-path: inset(50%); white-space: nowrap; }
.sidebar-toggle { display: inline-flex; flex: 0 0 auto; width: 32px; height: 32px; align-items: center; justify-content: center; border: 1px solid transparent; border-radius: 7px; background: transparent; color: #666666; cursor: pointer; }
.sidebar-toggle:hover, .sidebar-toggle:focus-visible, body.is-sidebar-open .sidebar-toggle { border-color: #cdcdcd; background: #e4e4e4; color: #2f2f2f; outline: none; }
.sidebar-toggle-icon { width: 19px; height: 19px; }
.control-icon { display: block; fill: none; stroke: currentColor; stroke-linecap: round; stroke-linejoin: round; stroke-width: 2; }
.file-sidebar { position: fixed; top: 0; bottom: 0; left: 0; z-index: 5; display: flex; width: var(--sidebar-width); flex-direction: column; border-right: 1px solid #c7c7c7; background: var(--sidebar-bg); box-shadow: none; transform: translateX(-100%); transition: transform .22s cubic-bezier(.22, .8, .2, 1); }
body.is-sidebar-open .file-sidebar { transform: translateX(0); }
.file-sidebar-head { display: flex; min-height: var(--header-height); align-items: center; justify-content: space-between; gap: 12px; padding: 0 14px 0 18px; border-bottom: 1px solid #c7c7c7; background: var(--sidebar-head-bg); color: #4f4f4f; font-size: 12px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase; }
.file-sidebar-close { display: inline-flex; flex: 0 0 auto; width: 30px; height: 30px; align-items: center; justify-content: center; border: 1px solid transparent; border-radius: 6px; background: transparent; color: #707070; cursor: pointer; }
.file-sidebar-close:hover, .file-sidebar-close:focus-visible { border-color: #c4c4c4; background: #e5e5e5; color: #2f2f2f; outline: none; }
.header-search { position: relative; z-index: 6; width: min(460px, 42vw); min-width: 240px; margin: 0; }
.search-field { position: relative; display: flex; align-items: center; }
.header-search .search-input { min-height: 34px; padding: 6px 48px 6px 11px; border-color: #c9c9c9; border-radius: 7px; background: #f9f9f9; font-size: 13px; }
.header-search .search-shortcut { position: absolute; right: 7px; display: inline-flex; min-width: 32px; height: 22px; align-items: center; justify-content: center; border: 1px solid #d1d1d1; border-radius: 5px; background: #eeeeee; color: #666666; font: 600 11px/1 ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; pointer-events: none; }
.header-search .search-status { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); clip-path: inset(50%); white-space: nowrap; }
.header-search .search-results { position: absolute; top: calc(100% + 8px); left: 0; right: 0; z-index: 7; gap: 5px; max-height: min(430px, 58vh); overflow: auto; padding: 6px; border: 1px solid #d4d4d4; border-radius: 8px; background: #ffffff; box-shadow: 0 18px 42px rgba(30, 30, 30, .16); }
.header-search .search-result { padding: 8px 9px; border-color: #e0e0e0; border-radius: 6px; }
.header-search .search-result:hover, .header-search .search-result:focus-visible, .header-search .search-result.is-active { border-color: #c7c7c7; background: #f0f0f0; }
.file-sidebar-tree { flex: 1 1 auto; width: 100%; overflow: auto; padding: 4px 10px 18px 8px; }
main { width: min(960px, calc(100% - 32px)); margin: 0 auto; padding: 34px 0 56px; }
.workspaces { margin: 0 0 28px; }
.sidebar-label { margin: 0 0 8px; color: var(--muted); font-size: 12px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase; }
.workspace-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 9px; }
.workspace { display: grid; gap: 2px; min-width: 0; padding: 10px 11px; border: 1px solid #dce4df; border-radius: 6px; background: #fff; color: inherit; text-decoration: none; }
.workspace:hover, .workspace:focus-visible, .workspace.active { border-color: rgba(var(--accent-rgb), .36); background: #f2f7f4; outline: none; }
.workspace-name { overflow: hidden; color: var(--ink); font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.workspace-root { overflow: hidden; color: var(--muted); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.note-workspace { position: relative; width: 100%; height: calc(100vh - var(--header-height)); margin: 0; padding: 0; overflow-x: auto; overflow-y: hidden; background: #f0f0f0; scroll-behavior: smooth; overscroll-behavior-x: contain; transition: transform .22s cubic-bezier(.22, .8, .2, 1); }
body.viewer-document.is-sidebar-open > .note-workspace { transform: translateX(var(--sidebar-width)); }
.note-stack { position: relative; z-index: 1; display: flex; align-items: stretch; gap: 18px; min-width: max-content; height: 100%; padding: 22px max(22px, calc((100vw - 1180px) / 2)) 26px 22px; }
.note-workspace.is-empty { overflow-x: hidden; overflow-y: auto; }
.note-workspace.is-empty .note-stack { display: none; }
.knowledge-empty { position: absolute; inset: 0; z-index: 0; overflow: auto; background: #f0f0f0; }
.knowledge-empty[hidden] { display: none; }
.knowledge-empty-inner { display: grid; min-height: 100%; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 28px; padding: 32px max(24px, calc((100vw - 1420px) / 2)) 42px; }
.knowledge-empty-pane { min-width: 0; min-height: 0; }
.knowledge-empty-tree { overflow: auto; }
.knowledge-empty-tree .knowledge-tree { width: 100%; }
.knowledge-empty-graph { position: sticky; top: 26px; align-self: start; min-height: min(640px, calc(100vh - 132px)); overflow: hidden; }
.knowledge-graph-svg { display: block; width: 100%; height: min(640px, calc(100vh - 132px)); min-height: 440px; }
.knowledge-graph-edges line { stroke: #c8c8c8; stroke-width: 1.4; vector-effect: non-scaling-stroke; }
.knowledge-graph-node { color: #5b6661; cursor: pointer; text-decoration: none; }
.knowledge-graph-node circle { fill: #f8f8f8; stroke: #aeb8b2; stroke-width: 1.6; vector-effect: non-scaling-stroke; transition: fill .16s ease, stroke .16s ease, transform .16s ease; transform-box: fill-box; transform-origin: center; }
.knowledge-graph-node text { fill: #5f6b66; font: 600 13px/1 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; text-anchor: middle; pointer-events: none; }
.knowledge-graph-node:hover circle, .knowledge-graph-node:focus-visible circle { fill: #ffffff; stroke: var(--accent); transform: scale(1.16); }
.knowledge-graph-node:hover text, .knowledge-graph-node:focus-visible text { fill: #26302c; }
.knowledge-graph-node.is-index-node circle { fill: #ffffff; stroke: #89958f; stroke-width: 2; }
.knowledge-tree { width: min(720px, 100%); color: #51605a; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 13px; }
.tree-row { display: flex; min-height: 30px; align-items: center; gap: 12px; padding: 4px 10px 4px var(--indent); border-radius: 6px; color: inherit; text-decoration: none; }
.tree-directory { margin: 7px 0 2px; background: #e1e6e2; color: #56645f; font-weight: 700; }
.file-sidebar .tree-directory { background: var(--sidebar-row-bg); color: #4f4f4f; }
.tree-directory::before { color: #82908a; content: "/"; }
.tree-file:hover { background: rgba(var(--accent-rgb), .09); color: var(--accent); }
.file-sidebar .tree-file:hover { background: #d6d6d6; }
.tree-file-name { flex: 1 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tree-file-system { margin-left: auto; flex: 0 0 auto; padding: 1px 5px; border: 1px solid #cad4ce; border-radius: 4px; color: #708078; font-size: 10px; line-height: 1.2; text-transform: lowercase; }
h1 { margin: 0 0 10px; font-size: 34px; line-height: 1.15; }
h2 { margin-top: 32px; padding-top: 16px; border-top: 1px solid var(--line); }
h3 { margin-top: 26px; }
hr { margin: 28px 0; border: 0; border-top: 1px solid var(--line); }
.lede { margin: 0 0 26px; color: var(--muted); }
.error { color: #a44b28; }
.search { position: relative; margin: 0 0 22px; }
.search-label { display: block; margin: 0 0 7px; color: var(--muted); font-size: 12px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase; }
.search-input { width: 100%; min-height: 42px; border: 1px solid #ced8d2; border-radius: 6px; background: #fff; color: var(--ink); font: inherit; padding: 8px 11px; }
.search-input:focus { border-color: rgba(var(--accent-rgb), .5); box-shadow: 0 0 0 3px rgba(var(--accent-rgb), .12); outline: none; }
.search-status { min-height: 20px; margin-top: 8px; color: var(--muted); font-size: 13px; }
.search-results { display: grid; gap: 7px; margin-top: 8px; }
.search-results[hidden] { display: none; }
.search-result { display: grid; gap: 2px; padding: 10px 11px; border: 1px solid #dce4df; border-radius: 6px; background: #fff; color: inherit; text-decoration: none; }
.search-result:hover, .search-result:focus-visible, .search-result.is-active { border-color: rgba(var(--accent-rgb), .35); background: #f2f7f4; outline: none; }
.search-result-title { color: var(--ink); font-weight: 700; }
.search-result-meta, .search-result-snippet { color: var(--muted); font-size: 13px; }
.list { border-top: 1px solid var(--line); }
.row { display: grid; grid-template-columns: minmax(180px, 1fr) minmax(160px, .7fr); gap: 12px; padding: 12px 0; border-bottom: 1px solid var(--line); color: inherit; text-decoration: none; }
.row:hover .path { color: var(--accent); }
.path { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 14px; }
.meta, .issue { color: var(--muted); font-size: 13px; }
.issue { grid-column: 1 / -1; color: #a44b28; }
.document { max-width: 780px; }
.note-panel.document { flex: 0 0 min(650px, calc(100vw - 44px)); max-width: none; height: 100%; padding: 0 34px 34px; overflow-y: auto; border: 1px solid var(--line); background: var(--panel); box-shadow: 0 18px 46px var(--shadow); outline: none; scroll-padding-top: 62px; }
.note-panel:focus { border-color: rgba(var(--accent-rgb), .45); }
.note-panel.is-entering { animation: note-enter .28s cubic-bezier(.22, .8, .2, 1); }
body.is-view-transitioning .note-panel.is-entering { animation: none; }
.note-chrome { position: sticky; top: 0; z-index: 1; display: flex; min-height: 48px; align-items: center; justify-content: space-between; gap: 14px; margin: 0 -34px 24px; padding: 0 12px 0 34px; border-bottom: 1px solid var(--line); background: rgba(255, 255, 255, .96); }
.note-path { flex: 1 1 auto; min-width: 0; overflow: hidden; color: var(--muted); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; text-decoration: none; text-overflow: ellipsis; white-space: nowrap; }
.note-path:hover { color: var(--accent); }
.note-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 7px; }
.editor-picker { position: relative; flex: 0 0 auto; }
.note-panel:not(.is-active-panel) .editor-picker { display: none; }
.editor-trigger { display: inline-flex; min-width: 58px; height: 30px; align-items: stretch; justify-content: center; overflow: hidden; border: 1px solid #cbd6d0; border-radius: 7px; background: #f8faf8; color: #52615b; line-height: 1; box-shadow: 0 1px 0 rgba(255, 255, 255, .7) inset; }
.editor-open, .editor-menu-trigger { display: inline-flex; height: 100%; align-items: center; justify-content: center; border: 0; background: transparent; color: inherit; cursor: pointer; font: inherit; line-height: 1; text-decoration: none; }
.editor-open { min-width: 32px; padding: 0 6px; }
.editor-menu-trigger { width: 25px; padding: 0; border-left: 1px solid #dde6e1; }
.editor-open:hover, .editor-open:focus-visible, .editor-menu-trigger:hover, .editor-menu-trigger:focus-visible { background: #edf2ef; color: #1f2724; outline: none; }
.editor-trigger:focus-within { border-color: #b8c7bf; outline: 2px solid rgba(var(--accent-rgb), .18); outline-offset: 2px; }
.editor-mark { display: inline-flex; min-width: 20px; height: 20px; align-items: center; justify-content: center; border: 1px solid #dde6e1; border-radius: 5px; background: #ffffff; color: #1f2724; font-size: 10px; font-weight: 700; line-height: 1; }
.editor-mark[data-has-icon="true"] { min-width: 22px; height: 22px; background: #ffffff; }
.editor-icon { display: block; width: 18px; height: 18px; border-radius: 4px; object-fit: contain; }
.editor-caret { width: 14px; height: 14px; color: #5f6d67; }
.editor-menu { position: absolute; top: calc(100% + 8px); right: 0; z-index: 4; width: max-content; min-width: 210px; max-width: min(280px, calc(100vw - 36px)); padding: 6px; border: 1px solid #d8e0db; border-radius: 8px; background: #ffffff; box-shadow: 0 16px 38px rgba(24, 34, 30, .18); }
.editor-menu[hidden] { display: none; }
.editor-menu-item { display: flex; width: 100%; min-height: 34px; align-items: center; gap: 10px; padding: 6px 9px; border: 0; border-radius: 6px; background: transparent; color: #25302b; cursor: pointer; font: inherit; text-align: left; }
.editor-menu-item:hover, .editor-menu-item:focus-visible { background: #eef3f0; outline: none; }
.editor-menu-item.is-selected { background: rgba(var(--accent-rgb), .1); color: #075e39; }
.editor-option-mark { display: inline-flex; flex: 0 0 24px; height: 22px; align-items: center; justify-content: center; border: 1px solid #d6dfda; border-radius: 5px; background: #f7faf8; color: #46534e; font-size: 10px; font-weight: 700; line-height: 1; }
.editor-option-label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.editor-menu-separator { height: 1px; margin: 6px 4px; background: #e2e8e4; }
.note-close { display: inline-flex; flex: 0 0 auto; width: 28px; height: 28px; align-items: center; justify-content: center; border: 1px solid transparent; border-radius: 6px; background: transparent; color: #7c8a84; cursor: pointer; font: inherit; line-height: 1; text-decoration: none; }
.note-close:hover, .note-close:focus-visible { border-color: #cad4ce; background: #edf2ef; color: #26302c; outline: none; }
.note-close-icon { width: 16px; height: 16px; }
.note-body { padding-bottom: 10vh; }
.document p, .document li { color: #2f3834; }
a { color: var(--accent); text-underline-offset: 3px; }
.note-body a { border-radius: 4px; transition: background-color .16s ease, color .16s ease; }
.note-body a:hover, .note-body a.is-active-note { background: rgba(var(--accent-rgb), .13); }
.note-body a.is-active-note { color: #075e39; }
strong { color: var(--ink); font-weight: 700; }
blockquote { margin: 20px 0; padding: 2px 0 2px 18px; border-left: 3px solid var(--line); color: var(--muted); }
blockquote p { color: var(--muted); }
code { padding: 1px 4px; border-radius: 4px; background: #edf2ef; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: .92em; }
pre { overflow-x: auto; padding: 14px; border: 1px solid var(--line); background: #111714; color: #f3f7f4; }
pre code { padding: 0; background: transparent; color: inherit; }
ul, ol { padding-left: 22px; }
.empty { color: var(--muted); }
.note-error { color: #a44b28; }
@keyframes note-enter {
  from { opacity: .001; transform: translateX(34px) scale(.985); }
  to { opacity: 1; transform: translateX(0) scale(1); }
}
@keyframes stack-view-old {
  from { opacity: 1; transform: translateX(0) scale(1); }
  to { opacity: .72; transform: translateX(-18px) scale(.992); }
}
@keyframes stack-view-new {
  from { opacity: .001; transform: translateX(22px) scale(.992); }
  to { opacity: 1; transform: translateX(0) scale(1); }
}
@supports (view-transition-name: none) {
  .note-workspace { view-transition-name: note-workspace; }
  ::view-transition-old(root), ::view-transition-new(root) { animation: none; }
  ::view-transition-old(note-workspace), ::view-transition-new(note-workspace) {
    animation-duration: .24s;
    animation-timing-function: cubic-bezier(.22, .8, .2, 1);
    mix-blend-mode: normal;
  }
  ::view-transition-old(note-workspace) { animation-name: stack-view-old; }
  ::view-transition-new(note-workspace) { animation-name: stack-view-new; }
}
@media (prefers-reduced-motion: reduce) {
  .note-workspace { scroll-behavior: auto; }
  .note-panel.is-entering { animation: none; }
  .note-body a { transition: none; }
}
@media (max-width: 680px) {
  header { display: block; }
  body:not(.viewer-document) header span { display: block; margin-top: 4px; overflow-wrap: anywhere; }
  .row { grid-template-columns: 1fr; }
  body.viewer-document header { display: flex; min-height: 68px; justify-content: flex-end; padding: 12px; }
  .header-left { left: 12px; max-width: 42%; }
  .brand { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .header-search { width: min(52vw, 320px); min-width: 0; }
  .header-search .search-shortcut { display: none; }
  .header-search .search-input { padding-right: 10px; }
  .header-search .search-results { left: auto; width: min(320px, calc(100vw - 24px)); }
  .note-workspace { height: calc(100vh - 68px); }
  .note-stack { gap: 12px; padding: 12px; }
  .knowledge-empty-inner { grid-template-columns: 1fr; gap: 22px; padding: 28px 14px 44px; }
  .knowledge-empty-graph { position: relative; top: auto; min-height: 380px; }
  .knowledge-graph-svg { height: 380px; min-height: 380px; }
  .note-panel.document { flex-basis: calc(100vw - 24px); padding: 0 22px 28px; }
  .note-chrome { margin: 0 -22px 22px; padding: 0 10px 0 22px; }
}
`
