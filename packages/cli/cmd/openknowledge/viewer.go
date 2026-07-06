package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"mime"
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

func runView(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, viewHelpText())
		return 0
	}
	fs := flag.NewFlagSet("view", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	host := fs.String("host", "127.0.0.1", "host to bind")
	port := fs.Int("port", 0, "port to bind, or 0 for a free port")
	name := fs.String("name", "", "local alias name for direct path mode")
	noBrowser := fs.Bool("no-browser", false, "print the URL without opening a browser")
	headHTML := fs.String("head-html", os.Getenv("OPENKNOWLEDGE_HEAD_HTML"), "trusted HTML fragment to inject into <head>")
	headFile := fs.String("head-file", os.Getenv("OPENKNOWLEDGE_HEAD_FILE"), "trusted HTML fragment file to inject into <head>")
	scriptSrcs := stringListFlag(splitHeadList(os.Getenv("OPENKNOWLEDGE_SCRIPT_SRC")))
	fs.Var(&scriptSrcs, "script-src", "script src to inject into <head>; may be repeated")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "view accepts at most one path")
		return 2
	}

	headInjection, err := loadHeadInjection(headInjectionOptions{
		HTML:       *headHTML,
		File:       *headFile,
		ScriptSrcs: []string(scriptSrcs),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	options := viewerOptions{HeadHTML: headInjection}

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
		options.AliasName = aliasName
		handler = newViewerHandlerWithOptions(absolute, options)
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
		handler = newRegistryViewerHandlerWithOptions(entries, options)
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
	viewURL := viewerAliasDisplayURL(displayHost, displayPort, aliasNames)

	fmt.Printf("Open Knowledge view: %s\n", viewURL)
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
	return newViewerHandlerWithOptions(root, viewerOptions{AliasName: aliasName})
}

type viewerOptions struct {
	AliasName string
	HeadHTML  template.HTML
}

func newViewerHandlerWithOptions(root string, options viewerOptions) http.Handler {
	mux := http.NewServeMux()
	aliasName := options.AliasName
	searchCache := &viewerSearchCache{root: root}
	mux.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/" {
			if startPath := viewerStartPath(root); startPath != "/" {
				http.Redirect(response, request, startPath, http.StatusFound)
				return
			}
			renderViewerIndex(response, root, viewerFrame{}, "", filepath.Base(root), options)
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
			renderViewerIndex(response, root, viewerFrame{}, prefix, filepath.Base(root), options)
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
		if strings.HasPrefix(rest, "raw/") {
			renderViewerRaw(response, request, root, strings.TrimPrefix(rest, "raw/"))
			return
		}
		if strings.HasPrefix(rest, "file/") {
			renderViewerFile(response, request, root, strings.TrimPrefix(rest, "file/"), viewerFrame{}, prefix, options)
			return
		}
		http.NotFound(response, request)
	})
	mux.HandleFunc("/file/", func(response http.ResponseWriter, request *http.Request) {
		rel := strings.TrimPrefix(request.URL.Path, "/file/")
		renderViewerFile(response, request, root, rel, viewerFrame{}, "", options)
	})
	mux.HandleFunc("/api/file/", func(response http.ResponseWriter, request *http.Request) {
		rel := strings.TrimPrefix(request.URL.Path, "/api/file/")
		renderViewerFileAPI(response, request, root, rel, "")
	})
	mux.HandleFunc("/raw/", func(response http.ResponseWriter, request *http.Request) {
		rel := strings.TrimPrefix(request.URL.Path, "/raw/")
		renderViewerRaw(response, request, root, rel)
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
	return newRegistryViewerHandlerWithOptions(entries, viewerOptions{})
}

func newRegistryViewerHandlerWithOptions(entries []okf.RegistryEntry, options viewerOptions) http.Handler {
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
							Frame:     frame,
							Title:     entry.Name,
							BrandName: entry.Name,
							Root:      entry.Path,
							Theme:     viewerThemeData{Name: "default"},
							Error:     err.Error(),
							HeadHTML:  options.HeadHTML,
						})
						return
					}
					if startPath := viewerStartPathWithPrefix(root, prefix); startPath != prefix+"/" {
						http.Redirect(response, request, startPath, http.StatusFound)
						return
					}
					renderViewerIndex(response, root, frame, prefix, entry.Name, options)
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
				if strings.HasPrefix(rest, "raw/") {
					root, err := registryEntryRoot(entry)
					if err != nil {
						http.Error(response, err.Error(), http.StatusInternalServerError)
						return
					}
					renderViewerRaw(response, request, root, strings.TrimPrefix(rest, "raw/"))
					return
				}
				if strings.HasPrefix(rest, "file/") {
					root, err := registryEntryRoot(entry)
					if err != nil {
						http.Error(response, err.Error(), http.StatusInternalServerError)
						return
					}
					frame := registryFrame(entries, entry.Name, localAliasURL)
					renderViewerFile(response, request, root, strings.TrimPrefix(rest, "file/"), frame, prefix, options)
					return
				}
			}
			http.NotFound(response, request)
			return
		}
		if len(entries) == 0 {
			renderRegistryEmpty(response, options)
			return
		}
		renderRegistryIndex(response, entries, entries[0].Name, options)
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
			renderRegistryIndex(response, entries, entry.Name, options)
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
		if strings.HasPrefix(rest, "raw/") {
			root, err := registryEntryRoot(entry)
			if err != nil {
				http.Error(response, err.Error(), http.StatusInternalServerError)
				return
			}
			renderViewerRaw(response, request, root, strings.TrimPrefix(rest, "raw/"))
			return
		}
		if strings.HasPrefix(rest, "file/") {
			root, err := registryEntryRoot(entry)
			if err != nil {
				http.Error(response, err.Error(), http.StatusInternalServerError)
				return
			}
			frame := registryFrame(entries, entry.Name, workspaceURL)
			renderViewerFile(response, request, root, strings.TrimPrefix(rest, "file/"), frame, prefix, options)
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
	BrandName string
	HomeURL   string
	Root      string
	Theme     viewerThemeData
	Error     string
	SearchURL string
	Entries   []viewerEntry
	HeadHTML  template.HTML
}

func renderViewerIndex(response http.ResponseWriter, root string, frame viewerFrame, linkPrefix string, title string, options viewerOptions) {
	theme, themeErr := viewerThemeForServer(root, linkPrefix)
	brandName := viewerKnowledgeBaseName(root, title)
	if themeErr != nil {
		renderHTML(response, viewerIndexTemplate, viewerIndexData{
			Frame:     frame,
			Title:     title,
			BrandName: brandName,
			HomeURL:   viewerPrefixRoot(linkPrefix),
			Root:      root,
			Theme:     theme,
			Error:     themeErr.Error(),
			HeadHTML:  options.HeadHTML,
		})
		return
	}
	listing, err := okf.List(root)
	if err != nil {
		renderHTML(response, viewerIndexTemplate, viewerIndexData{
			Frame:     frame,
			Title:     title,
			BrandName: brandName,
			HomeURL:   viewerPrefixRoot(linkPrefix),
			Root:      root,
			Theme:     theme,
			Error:     err.Error(),
			HeadHTML:  options.HeadHTML,
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
		BrandName: brandName,
		HomeURL:   viewerPrefixRoot(linkPrefix),
		Root:      root,
		Theme:     theme,
		SearchURL: searchURLWithPrefix(linkPrefix),
		Entries:   entries,
		HeadHTML:  options.HeadHTML,
	})
}

type viewerFileData struct {
	Frame       viewerFrame
	Title       string
	BrandName   string
	HomeURL     string
	Root        string
	Path        string
	FileURL     string
	SourceURL   string
	LinkPrefix  string
	SearchURL   string
	Theme       viewerThemeData
	Body        template.HTML
	Tree        []viewerTreeItem
	EditorsJSON template.JS
	StaticJSON  template.JS
	GraphJSON   template.JS
	HeadHTML    template.HTML
}

type viewerAssetData struct {
	Title      string
	BrandName  string
	HomeURL    string
	Root       string
	Path       string
	RawURL     string
	Theme      viewerThemeData
	Kind       string
	MediaType  string
	Language   string
	Body       template.HTML
	PreviewURL string
	HeadHTML   template.HTML
}

type viewerFilePayload struct {
	Title string `json:"title"`
	Path  string `json:"path"`
	Body  string `json:"body"`
}

type viewerStaticPayload struct {
	Title     string `json:"title"`
	Path      string `json:"path"`
	HTMLPath  string `json:"htmlPath"`
	SourceURL string `json:"sourceURL,omitempty"`
	Body      string `json:"body"`
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

const maxViewerTextPreviewBytes = 1024 * 1024

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

func renderViewerFile(response http.ResponseWriter, request *http.Request, root string, rel string, frame viewerFrame, linkPrefix string, options viewerOptions) {
	if cleanRel, ok := cleanViewerRel(rel, true); ok && !isMarkdownFile(cleanRel) {
		renderViewerAsset(response, request, root, cleanRel, linkPrefix, options)
		return
	}

	data, ok, err := viewerFile(root, rel, frame, linkPrefix)
	if !ok {
		http.NotFound(response, request)
		return
	}
	if err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	data.HeadHTML = options.HeadHTML

	renderHTML(response, viewerFileTemplate, data)
}

func renderViewerAsset(response http.ResponseWriter, request *http.Request, root string, rel string, linkPrefix string, options viewerOptions) {
	data, ok, err := viewerAsset(root, rel, linkPrefix)
	if !ok {
		http.NotFound(response, request)
		return
	}
	if err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	data.HeadHTML = options.HeadHTML

	renderHTML(response, viewerAssetTemplate, data)
}

func viewerAsset(root string, rel string, linkPrefix string) (viewerAssetData, bool, error) {
	cleanRel, ok := cleanViewerRel(rel, false)
	if !ok || isMarkdownFile(cleanRel) {
		return viewerAssetData{}, false, nil
	}
	filePath, ok := safeViewerPath(root, cleanRel)
	if !ok {
		return viewerAssetData{}, false, nil
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return viewerAssetData{}, true, err
	}
	if info.IsDir() {
		return viewerAssetData{}, false, nil
	}

	mediaType := viewerMediaType(filePath)
	rawURL := rawURLWithPrefix(linkPrefix, cleanRel)
	theme, err := viewerThemeForServer(root, linkPrefix)
	if err != nil {
		return viewerAssetData{}, true, err
	}
	data := viewerAssetData{
		Title:      titleForAssetFile(cleanRel),
		BrandName:  viewerKnowledgeBaseName(root, ""),
		HomeURL:    viewerPrefixRoot(linkPrefix),
		Root:       root,
		Path:       cleanRel,
		RawURL:     rawURL,
		Theme:      theme,
		Kind:       viewerAssetKind(filePath, mediaType),
		MediaType:  mediaType,
		Language:   okf.CodeLanguageForPath(cleanRel),
		PreviewURL: rawURL,
	}

	if data.Kind == "code" || data.Kind == "text" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return viewerAssetData{}, true, err
		}
		if len(content) > maxViewerTextPreviewBytes || !viewerLooksText(content) {
			data.Kind = "download"
			return data, true, nil
		}
		language := data.Language
		if language == "" {
			language = "text"
		}
		data.Body = template.HTML(okf.RenderCodeBlock(string(content), language))
	}

	return data, true, nil
}

func renderViewerRaw(response http.ResponseWriter, request *http.Request, root string, rel string) {
	filePath, ok := safeViewerPath(root, rel)
	if !ok {
		http.NotFound(response, request)
		return
	}
	file, err := os.Open(filePath)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(response, request)
		return
	}

	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Content-Type", viewerSafeRawMediaType(filePath))
	http.ServeContent(response, request, info.Name(), info.ModTime(), file)
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
	bundle, err := okf.ParseBundle(root)
	if err != nil {
		return viewerFileData{}, true, err
	}
	file, ok := viewerBundleFileByPath(bundle.Files, cleanRel)
	if !ok {
		return viewerFileData{}, false, nil
	}
	entries := viewerEntriesFromBundleFiles(bundle.Files)
	graphJSON := viewerGraphJSONFromBundleFiles(bundle.Files, entries, func(path string) string {
		return fileURLWithPrefix(linkPrefix, path)
	})
	theme, err := viewerThemeForServer(root, linkPrefix)
	if err != nil {
		return viewerFileData{}, true, err
	}

	return viewerFileData{
		Frame:       frame,
		Title:       titleForMarkdownFile(cleanRel),
		BrandName:   viewerKnowledgeBaseName(root, ""),
		HomeURL:     viewerPrefixRoot(linkPrefix),
		Root:        root,
		Path:        cleanRel,
		FileURL:     fileURLWithPrefix(linkPrefix, cleanRel),
		SourceURL:   "",
		LinkPrefix:  strings.TrimRight(linkPrefix, "/"),
		SearchURL:   searchURLWithPrefix(linkPrefix),
		Theme:       theme,
		Body:        template.HTML(okf.RenderMarkdown(file.Body, cleanRel, viewerLinkWithPrefix(linkPrefix))),
		Tree:        viewerTreeWithURL(entries, func(path string) string { return fileURLWithPrefix(linkPrefix, path) }),
		EditorsJSON: viewerEditorsJSON(),
		GraphJSON:   graphJSON,
	}, true, nil
}

func viewerBundleFileByPath(files []okf.BundleFile, path string) (okf.BundleFile, bool) {
	for _, file := range files {
		if file.Path == path {
			return file, true
		}
	}
	return okf.BundleFile{}, false
}

func viewerEntriesFromBundleFiles(files []okf.BundleFile) []okf.ListEntry {
	entries := make([]okf.ListEntry, 0, len(files))
	for _, file := range files {
		entries = append(entries, okf.ListEntry{
			ID:          file.ID,
			Path:        file.Path,
			Kind:        file.Kind,
			Reserved:    file.Reserved,
			Type:        file.Type,
			Title:       file.Title,
			Description: file.Description,
			Resource:    file.Resource,
			Issues:      file.Issues,
		})
	}
	return entries
}

type viewerSearchResponse struct {
	Query   string               `json:"query"`
	Results []viewerSearchResult `json:"results"`
}

type viewerSearchResult struct {
	Path          string   `json:"path"`
	URL           string   `json:"url"`
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Type          string   `json:"type,omitempty"`
	Title         string   `json:"title"`
	Description   string   `json:"description,omitempty"`
	Snippet       string   `json:"snippet,omitempty"`
	HighlightText string   `json:"highlightText,omitempty"`
	HighlightURL  string   `json:"highlightURL,omitempty"`
	Score         float64  `json:"score"`
	Matches       []string `json:"matches,omitempty"`
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
		resultURL := fileURLWithPrefix(linkPrefix, result.Path)
		payload.Results = append(payload.Results, viewerSearchResult{
			Path:          result.Path,
			URL:           resultURL,
			ID:            result.ID,
			Kind:          result.Kind,
			Type:          result.Type,
			Title:         result.Title,
			Description:   result.Description,
			Snippet:       result.Snippet,
			HighlightText: result.HighlightText,
			HighlightURL:  viewerHighlightURL(resultURL, result.HighlightText),
			Score:         result.Score,
			Matches:       result.Matches,
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

func viewerHighlightURL(base string, highlightText string) string {
	if strings.TrimSpace(highlightText) == "" {
		return ""
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return ""
	}
	query := parsed.Query()
	query.Set("ok-highlight", highlightText)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func renderRegistryEmpty(response http.ResponseWriter, options viewerOptions) {
	renderHTML(response, viewerIndexTemplate, viewerIndexData{
		Title:     "Open Knowledge Registry",
		BrandName: "Open Knowledge",
		HomeURL:   "/",
		Theme:     viewerThemeData{Name: "default"},
		Error:     "No registered knowledge bases. Add one with openknowledge registry connect <path> --as <key>.",
		HeadHTML:  options.HeadHTML,
	})
}

func renderRegistryIndex(response http.ResponseWriter, entries []okf.RegistryEntry, activeName string, options viewerOptions) {
	entry, found := registryEntryByName(entries, activeName)
	if !found {
		http.Error(response, "knowledge base not found", http.StatusNotFound)
		return
	}

	root, err := registryEntryRoot(entry)
	frame := registryFrame(entries, entry.Name, workspaceURL)
	if err != nil {
		renderHTML(response, viewerIndexTemplate, viewerIndexData{
			Frame:     frame,
			Title:     entry.Name,
			BrandName: entry.Name,
			Root:      entry.Path,
			Theme:     viewerThemeData{Name: "default"},
			Error:     err.Error(),
			HeadHTML:  options.HeadHTML,
		})
		return
	}
	renderViewerIndex(response, root, frame, workspacePrefix(entry.Name), entry.Name, options)
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

type viewerHTMLExportOptions struct {
	HeadHTML template.HTML
}

func writeViewerHTMLWithVersion(root string, out string, version string) (okf.HTMLResult, error) {
	return writeViewerHTMLWithOptions(root, out, version, viewerHTMLExportOptions{})
}

func writeViewerHTMLWithOptions(root string, out string, version string, options viewerHTMLExportOptions) (okf.HTMLResult, error) {
	bundle, err := okf.ParseBundleWithVersion(root, version)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	themeConfig, err := loadViewerThemeConfig(bundle.Root)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	sourceConfig, err := loadViewerSourceConfig(bundle.Root)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	siteConfig, err := loadViewerSiteConfig(bundle.Root)
	if err != nil {
		return okf.HTMLResult{}, err
	}

	absoluteOut, err := filepath.Abs(out)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	themeAsset, err := copyViewerThemeStylesheet(bundle.Root, absoluteOut, themeConfig)
	if err != nil {
		return okf.HTMLResult{}, err
	}

	staticJSON, err := viewerStaticFilesJSON(bundle.Files, sourceConfig)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	editorsJSON := viewerEditorsStaticJSON()
	graphJSON := viewerStaticGraphJSON(bundle.Files)

	var written []string
	for _, file := range bundle.Files {
		if !okf.ShouldPublish(file) {
			continue
		}
		target := filepath.Join(absoluteOut, filepath.FromSlash(viewerHTMLPath(file.Path)))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return okf.HTMLResult{}, err
		}

		data := viewerFileData{
			Title:       titleForMarkdownFile(file.Path),
			BrandName:   viewerKnowledgeBaseNameFromFiles(bundle.Files, ""),
			HomeURL:     viewerStaticRelativeURL(file.Path, "index.md"),
			Root:        "",
			Path:        file.Path,
			FileURL:     viewerStaticRelativeURL(file.Path, file.Path),
			SourceURL:   viewerSourceURL(sourceConfig, file.Path),
			Body:        template.HTML(viewerStaticFileBody(file)),
			Tree:        viewerStaticTree(bundle.Files, file.Path),
			Theme:       viewerThemeForStaticPage(themeConfig, file.Path),
			EditorsJSON: editorsJSON,
			StaticJSON:  staticJSON,
			GraphJSON:   graphJSON,
			HeadHTML:    options.HeadHTML,
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
	if themeAsset != "" {
		written = append(written, themeAsset)
	}
	archiveResult, err := writeViewerExportBundleAssets(bundle.Root, absoluteOut, version)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	written = append(written, archiveResult...)
	discoveryResult, err := writeViewerDiscoveryFiles(bundle.Files, absoluteOut, siteConfig)
	if err != nil {
		return okf.HTMLResult{}, err
	}
	written = append(written, discoveryResult...)

	sort.Strings(written)
	return okf.HTMLResult{Root: bundle.Root, Out: absoluteOut, Written: written}, nil
}

func writeViewerExportBundleAssets(root string, out string, version string) ([]string, error) {
	archiveRel := okf.BundleArchiveRelPath
	archivePath := filepath.Join(out, filepath.FromSlash(archiveRel))
	archive, err := okf.WriteBundleTarGzipWithVersion(root, archivePath, version, []string{out})
	if err != nil {
		return nil, err
	}

	manifest, err := okf.BundleManifestForArchive(root, version, archiveRel, archive.SHA256)
	if err != nil {
		return nil, err
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	manifestData = append(manifestData, '\n')
	manifestPath := filepath.Join(out, filepath.FromSlash(okf.BundleManifestRelPath))
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, err
	}

	return []string{
		filepath.ToSlash(okf.BundleArchiveRelPath),
		filepath.ToSlash(okf.BundleManifestRelPath),
	}, nil
}

func viewerRelPath(root string, target string) string {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(relative)
}

func viewerStaticFilesJSON(files []okf.BundleFile, sourceConfig viewerSourceConfig) (template.JS, error) {
	payload := make([]viewerStaticPayload, 0, len(files))
	for _, file := range files {
		if !okf.ShouldPublish(file) {
			continue
		}
		payload = append(payload, viewerStaticPayload{
			Title:     titleForMarkdownFile(file.Path),
			Path:      file.Path,
			HTMLPath:  viewerHTMLPath(file.Path),
			SourceURL: viewerSourceURL(sourceConfig, file.Path),
			Body:      viewerStaticFileBody(file),
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
		if !okf.ShouldPublish(file) {
			continue
		}
		entries = append(entries, okf.ListEntry{Path: file.Path})
	}
	return viewerTreeWithURL(entries, func(path string) string {
		return viewerStaticRelativeURL(currentPath, path)
	})
}

func viewerGraphJSONFromBundleFiles(files []okf.BundleFile, entries []okf.ListEntry, fileURL func(string) string) template.JS {
	graph := viewerGraphFromBundleFiles(files, entries, fileURL)
	data, err := json.Marshal(graph)
	if err != nil {
		return `{"nodes":[],"edges":[]}`
	}
	return template.JS(data)
}

func viewerStaticGraphJSON(files []okf.BundleFile) template.JS {
	entries := make([]okf.ListEntry, 0, len(files))
	publishedFiles := make([]okf.BundleFile, 0, len(files))
	for _, file := range files {
		if !okf.ShouldPublish(file) {
			continue
		}
		publishedFiles = append(publishedFiles, file)
		entries = append(entries, okf.ListEntry{Path: file.Path, Title: file.Title})
	}
	return viewerGraphJSONFromBundleFiles(publishedFiles, entries, func(path string) string {
		return viewerStaticRelativeURL("index.md", path)
	})
}

func viewerGraphFromBundleFiles(files []okf.BundleFile, entries []okf.ListEntry, fileURL func(string) string) viewerGraphData {
	graph := okf.GraphFromBundle(okf.Bundle{Files: files})
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
	for _, edge := range graph.Edges {
		if !paths[edge.Source] || !paths[edge.Target] {
			continue
		}
		key := edge.Source + "\x00" + edge.Target
		if seenEdges[key] {
			continue
		}
		seenEdges[key] = true
		edges = append(edges, viewerGraphEdge{
			Source: edge.Source,
			Target: edge.Target,
			Label:  edge.Label,
		})
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

func safeViewerPath(root string, rel string) (string, bool) {
	clean, ok := cleanViewerRel(rel, false)
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

func cleanViewerRel(rel string, defaultIndex bool) (string, bool) {
	if hasParentSegment(rel) {
		return "", false
	}
	clean := path.Clean("/" + rel)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." || clean == "" {
		if !defaultIndex {
			return "", false
		}
		clean = "index.md"
	}
	return clean, true
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

func isCodePreviewFile(name string) bool {
	return okf.CodeLanguageForPath(name) != "" && !isMarkdownFile(name)
}

func isTextPreviewFile(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	switch extension {
	case ".txt", ".log", ".csv", ".tsv", ".ini", ".env", ".gitignore", ".dockerignore":
		return true
	default:
		return false
	}
}

func viewerAssetKind(filePath string, mediaType string) string {
	extension := strings.ToLower(filepath.Ext(filePath))
	if extension == ".pdf" || strings.HasPrefix(mediaType, "application/pdf") {
		return "pdf"
	}
	if isCodePreviewFile(filePath) {
		return "code"
	}
	if isTextPreviewFile(filePath) || strings.HasPrefix(mediaType, "text/") {
		return "text"
	}
	if strings.HasPrefix(mediaType, "image/") {
		return "image"
	}
	if strings.HasPrefix(mediaType, "video/") {
		return "video"
	}
	if strings.HasPrefix(mediaType, "audio/") {
		return "audio"
	}
	return "download"
}

func viewerMediaType(filePath string) string {
	extension := strings.ToLower(filepath.Ext(filePath))
	if extension == ".mov" {
		return "video/quicktime"
	}
	mediaType := mime.TypeByExtension(extension)
	if mediaType != "" {
		return mediaType
	}
	return "application/octet-stream"
}

func viewerSafeRawMediaType(filePath string) string {
	if isCodePreviewFile(filePath) || isTextPreviewFile(filePath) {
		return "text/plain; charset=utf-8"
	}
	extension := strings.ToLower(filepath.Ext(filePath))
	switch extension {
	case ".html", ".htm", ".svg", ".js", ".mjs", ".cjs":
		return "text/plain; charset=utf-8"
	default:
		return viewerMediaType(filePath)
	}
}

func viewerLooksText(content []byte) bool {
	sample := content
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	return !bytes.Contains(sample, []byte{0})
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

func rawURL(rel string) string {
	return "/raw/" + strings.TrimPrefix(path.Clean("/"+rel), "/")
}

func rawURLWithPrefix(prefix string, rel string) string {
	return strings.TrimRight(prefix, "/") + rawURL(rel)
}

func viewerContentURLWithPrefix(prefix string, rel string) string {
	if isMarkdownFile(rel) || isCodePreviewFile(rel) || isTextPreviewFile(rel) {
		return fileURLWithPrefix(prefix, rel)
	}
	return rawURLWithPrefix(prefix, rel)
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
		return viewerResolveLinkWithPrefix(prefix, currentRel, href)
	}
}

func viewerResolveLinkWithPrefix(prefix string, currentRel string, href string) string {
	trimmed := strings.TrimSpace(href)
	if trimmed == "" {
		return "#"
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Scheme != "" {
		if parsed.Scheme == "http" || parsed.Scheme == "https" {
			return trimmed
		}
		return "#"
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
		return trimmed
	}

	linkPath := trimmed
	suffix := ""
	if hash := strings.Index(linkPath, "#"); hash >= 0 {
		suffix = linkPath[hash:] + suffix
		linkPath = linkPath[:hash]
	}
	if query := strings.Index(linkPath, "?"); query >= 0 {
		suffix = linkPath[query:] + suffix
		linkPath = linkPath[:query]
	}

	target := viewerLinkTargetRel(currentRel, linkPath)
	if target == "" {
		return suffix
	}
	return viewerContentURLWithPrefix(prefix, target) + suffix
}

func viewerLinkTargetRel(sourceRel string, href string) string {
	target := strings.TrimSpace(href)
	if target == "" {
		return ""
	}

	var clean string
	if strings.HasPrefix(target, "/") {
		clean = filepath.ToSlash(filepath.Clean(strings.TrimPrefix(target, "/")))
	} else {
		base := filepath.Dir(sourceRel)
		if base == "." {
			base = ""
		}
		clean = filepath.ToSlash(filepath.Clean(filepath.Join(base, target)))
	}
	if clean == "." {
		clean = ""
	}
	if strings.HasSuffix(target, "/") {
		clean = filepath.ToSlash(filepath.Join(clean, "index.md"))
	}
	return clean
}

func viewerAliasDisplayURL(host string, port string, aliasNames []string) string {
	if len(aliasNames) == 1 {
		return viewerDisplayURL(host, port, localAliasPrefix(aliasNames[0]))
	}
	return viewerDisplayURL(host, port, "")
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

func titleForAssetFile(rel string) string {
	title := titleForMarkdownFile(rel)
	extension := strings.TrimPrefix(filepath.Ext(rel), ".")
	if extension == "" {
		return title
	}
	return title + " ." + extension
}

func viewerKnowledgeBaseName(root string, fallback string) string {
	bundle, err := okf.ParseBundle(root)
	if err == nil {
		if name := viewerKnowledgeBaseNameFromFiles(bundle.Files, fallback); name != "" {
			return name
		}
	}
	if name := strings.TrimSpace(fallback); name != "" {
		return name
	}
	return "Open Knowledge"
}

func viewerKnowledgeBaseNameFromFiles(files []okf.BundleFile, fallback string) string {
	for _, file := range files {
		if file.Path != "index.md" {
			continue
		}
		for _, key := range []string{"okf_bundle_title", "okf_bundle_name", "title"} {
			if name := strings.TrimSpace(file.Frontmatter[key]); name != "" {
				return name
			}
		}
		if name := firstMarkdownHeading(file.Body); name != "" {
			return name
		}
		break
	}
	return strings.TrimSpace(fallback)
}

func firstMarkdownHeading(body string) string {
	markdown := okf.ParseASTMarkdown(body, 1)
	for _, heading := range markdown.Headings {
		if heading.Level == 1 {
			return strings.TrimSpace(heading.Text)
		}
	}
	return ""
}

type headInjectionOptions struct {
	HTML       string
	File       string
	ScriptSrcs []string
}

func loadHeadInjection(options headInjectionOptions) (template.HTML, error) {
	var snippets []string
	if file := strings.TrimSpace(options.File); file != "" {
		content, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read head file: %w", err)
		}
		snippets = append(snippets, string(content))
	}
	if fragment := strings.TrimSpace(options.HTML); fragment != "" {
		snippets = append(snippets, fragment)
	}
	for _, src := range options.ScriptSrcs {
		script, err := scriptTag(src)
		if err != nil {
			return "", err
		}
		snippets = append(snippets, script)
	}
	return template.HTML(strings.Join(snippets, "\n  ")), nil
}

func splitHeadList(value string) []string {
	var result []string
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	}) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func scriptTag(src string) (string, error) {
	trimmed := strings.TrimSpace(src)
	if !validScriptSrc(trimmed) {
		return "", fmt.Errorf("unsupported script src: %s", src)
	}
	return `<script src="` + html.EscapeString(trimmed) + `"></script>`, nil
}

func validScriptSrc(src string) bool {
	if src == "" {
		return false
	}
	parsed, err := url.Parse(src)
	if err != nil {
		return false
	}
	return parsed.Scheme == "" || parsed.Scheme == "http" || parsed.Scheme == "https"
}

var viewerIndexTemplate = template.Must(template.New("viewer-index").Parse(`<!doctype html>
<html lang="en" data-openknowledge-theme="{{.Theme.Name}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} - Open Knowledge</title>
  <style>` + viewerCSS + `</style>
  {{if .Theme.Stylesheet}}<link rel="stylesheet" href="{{.Theme.Stylesheet}}">{{end}}
  {{.HeadHTML}}
</head>
<body>
  <header>
    {{if .Frame.Workspaces}}<a class="brand" href="{{.Frame.ActiveURL}}">{{.BrandName}}</a>{{else}}<a class="brand" href="{{.HomeURL}}">{{.BrandName}}</a>{{end}}
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
  <script>` + viewerShortcutsJS + `</script>
  <script>` + viewerSearchJS + `</script>
</body>
</html>`))

var viewerAssetTemplate = template.Must(template.New("viewer-asset").Parse(`<!doctype html>
<html lang="en" data-openknowledge-theme="{{.Theme.Name}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} - Open Knowledge</title>
  <style>` + viewerCSS + `</style>
  {{if .Theme.Stylesheet}}<link rel="stylesheet" href="{{.Theme.Stylesheet}}">{{end}}
  {{.HeadHTML}}
</head>
<body class="viewer-document viewer-asset-document">
  <header>
    <div class="header-left">
      <a class="brand" href="{{.HomeURL}}">{{.BrandName}}</a>
    </div>
    <a class="asset-open-raw" href="{{.RawURL}}" data-direct-link="true">Open raw</a>
  </header>
  <main class="asset-workspace">
    <article class="document asset-panel">
      <div class="note-chrome">
        <a class="note-path" href="{{.RawURL}}" data-direct-link="true">{{.Path}}</a>
        <span class="asset-kind">{{.MediaType}}</span>
      </div>
      <div class="asset-body asset-{{.Kind}}">
        {{if eq .Kind "pdf"}}
          <iframe class="asset-frame" src="{{.PreviewURL}}" title="{{.Path}}"></iframe>
        {{else if eq .Kind "image"}}
          <img class="asset-image" src="{{.PreviewURL}}" alt="{{.Path}}">
        {{else if eq .Kind "video"}}
          <video class="asset-video" src="{{.PreviewURL}}" controls preload="metadata"></video>
        {{else if eq .Kind "audio"}}
          <audio class="asset-audio" src="{{.PreviewURL}}" controls preload="metadata"></audio>
        {{else if or (eq .Kind "code") (eq .Kind "text")}}
          {{.Body}}
        {{else}}
          <p class="asset-download">This file type is not previewed inline. Open the raw file in the browser.</p>
        {{end}}
      </div>
    </article>
  </main>
</body>
</html>`))

var viewerFileTemplate = template.Must(template.New("viewer-file").Parse(`<!doctype html>
<html lang="en" data-openknowledge-theme="{{.Theme.Name}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} - Open Knowledge</title>
  <style>` + viewerCSS + `</style>
  {{if .Theme.Stylesheet}}<link rel="stylesheet" href="{{.Theme.Stylesheet}}">{{end}}
  {{.HeadHTML}}
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
      <kbd class="sidebar-shortcut" data-sidebar-shortcut aria-hidden="true">⌘⌥S</kbd>
      <a class="brand" href="{{.HomeURL}}">{{.BrandName}}</a>
    </div>
    <section class="search header-search" role="search" aria-label="Search files" data-search-url="{{.SearchURL}}" data-primary-search>
      <label class="sr-only" for="viewer-search">Search</label>
      <div class="search-field">
        <input id="viewer-search" class="search-input" type="search" autocomplete="off" spellcheck="false" placeholder="Search">
        <kbd class="search-shortcut" data-search-shortcut>⌘K</kbd>
      </div>
      <div class="search-status" aria-live="polite"></div>
      <div class="search-results" hidden></div>
    </section>
    <div class="viewer-settings" data-viewer-settings>
      <button class="viewer-settings-trigger" type="button" data-viewer-settings-trigger aria-haspopup="dialog" aria-expanded="false" aria-label="Viewer settings" title="Settings">
        <svg class="viewer-settings-icon control-icon" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Z"></path>
          <path d="M19.4 15a1.8 1.8 0 0 0 .36 1.98l.04.04a2.1 2.1 0 0 1-2.97 2.97l-.04-.04a1.8 1.8 0 0 0-1.98-.36 1.8 1.8 0 0 0-1.1 1.65V21.3a2.1 2.1 0 0 1-4.2 0v-.06a1.8 1.8 0 0 0-1.1-1.65 1.8 1.8 0 0 0-1.98.36l-.04.04a2.1 2.1 0 0 1-2.97-2.97l.04-.04A1.8 1.8 0 0 0 3.8 15a1.8 1.8 0 0 0-1.65-1.1H2.1a2.1 2.1 0 0 1 0-4.2h.06A1.8 1.8 0 0 0 3.8 8a1.8 1.8 0 0 0-.36-1.98l-.04-.04A2.1 2.1 0 0 1 6.37 3l.04.04A1.8 1.8 0 0 0 8.4 3.4a1.8 1.8 0 0 0 1.1-1.65V1.7a2.1 2.1 0 0 1 4.2 0v.06a1.8 1.8 0 0 0 1.1 1.65 1.8 1.8 0 0 0 1.98-.36l.04-.04a2.1 2.1 0 0 1 2.97 2.97l-.04.04A1.8 1.8 0 0 0 19.4 8a1.8 1.8 0 0 0 1.65 1.1h.06a2.1 2.1 0 0 1 0 4.2h-.06A1.8 1.8 0 0 0 19.4 15Z"></path>
        </svg>
      </button>
      <div class="viewer-settings-menu" data-viewer-settings-menu role="dialog" aria-label="Theme settings" hidden>
        <div class="viewer-settings-title">Theme</div>
        <div class="theme-options" role="radiogroup" aria-label="Theme">
          <button class="theme-option" type="button" data-theme-option="default" role="radio" aria-checked="false">
            <span class="theme-swatch theme-swatch-default" aria-hidden="true"></span>
            <span>Default</span>
          </button>
          <button class="theme-option" type="button" data-theme-option="night" role="radio" aria-checked="false">
            <span class="theme-swatch theme-swatch-night" aria-hidden="true"></span>
            <span>Night</span>
          </button>
          <button class="theme-option" type="button" data-theme-option="paper" role="radio" aria-checked="false">
            <span class="theme-swatch theme-swatch-paper" aria-hidden="true"></span>
            <span>Paper</span>
          </button>
          <button class="theme-option" type="button" data-theme-option="ocean" role="radio" aria-checked="false">
            <span class="theme-swatch theme-swatch-ocean" aria-hidden="true"></span>
            <span>Ocean</span>
          </button>
          <button class="theme-option" type="button" data-theme-option="rose" role="radio" aria-checked="false">
            <span class="theme-swatch theme-swatch-rose" aria-hidden="true"></span>
            <span>Rose</span>
          </button>
          <button class="theme-option" type="button" data-theme-option="custom" role="radio" aria-checked="false">
            <span class="theme-swatch theme-swatch-custom" aria-hidden="true"></span>
            <span>Custom</span>
          </button>
        </div>
        <div class="theme-custom-fields" data-theme-custom-fields hidden>
          <label>Page <input type="color" data-theme-custom-value="page"></label>
          <label>Surface <input type="color" data-theme-custom-value="surface"></label>
          <label>Text <input type="color" data-theme-custom-value="text"></label>
          <label>Muted <input type="color" data-theme-custom-value="muted"></label>
          <label>Accent <input type="color" data-theme-custom-value="accent"></label>
          <label>Border <input type="color" data-theme-custom-value="border"></label>
        </div>
      </div>
    </div>
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
  <main id="note-workspace" class="note-workspace" data-note-workspace data-note-root="{{.Root}}" data-link-prefix="{{.LinkPrefix}}">
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
            {{if .SourceURL}}
            <a class="source-open" href="{{.SourceURL}}" data-source-open data-direct-link="true" target="_blank" rel="noreferrer" aria-label="Open {{.Path}} on GitHub" title="Open on GitHub">
              <svg class="source-icon control-icon" data-icon="github" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M12 .5a12 12 0 0 0-3.79 23.39c.6.11.82-.26.82-.58v-2.17c-3.34.73-4.04-1.42-4.04-1.42-.55-1.39-1.34-1.76-1.34-1.76-1.09-.75.08-.73.08-.73 1.2.08 1.84 1.24 1.84 1.24 1.07 1.83 2.8 1.3 3.49.99.11-.78.42-1.3.76-1.6-2.67-.3-5.47-1.33-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.13-.3-.54-1.52.11-3.18 0 0 1.01-.32 3.3 1.23a11.4 11.4 0 0 1 6 0c2.29-1.55 3.3-1.23 3.3-1.23.65 1.66.24 2.88.12 3.18.77.84 1.23 1.91 1.23 3.22 0 4.61-2.81 5.63-5.48 5.92.43.37.81 1.1.81 2.22v3.29c0 .32.22.69.83.58A12 12 0 0 0 12 .5Z"></path>
              </svg>
            </a>
            {{else if .Root}}
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
            {{end}}
            <kbd class="note-close-shortcut" data-panel-close-shortcut aria-hidden="true">⌘⌥W</kbd>
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
  <div class="workspace-scroll-rail" data-workspace-rail aria-hidden="true" hidden>
    <div class="workspace-scroll-track" data-workspace-scroll-track>
      <button class="workspace-scroll-thumb" type="button" data-workspace-scroll-thumb aria-label="Scroll notes horizontally" aria-controls="note-workspace" aria-orientation="horizontal" aria-valuemin="0" aria-valuemax="0" aria-valuenow="0" role="scrollbar"></button>
    </div>
  </div>
  <a class="powered-by-openknowledge" href="https://openknowledge.sh" target="_blank" rel="noreferrer">Powered by OpenKnowledge.sh</a>
  <script type="application/json" data-editor-options>{{.EditorsJSON}}</script>
  <script type="application/json" data-knowledge-graph>{{.GraphJSON}}</script>
  {{if .StaticJSON}}<script type="application/json" data-static-notes>{{.StaticJSON}}</script>{{end}}
  <script>` + viewerShortcutsJS + `</script>
  <script>` + viewerJS + `</script>
  <script>` + viewerSearchJS + `</script>
</body>
</html>`))

var viewerCSS = viewerDefaultThemeCSS + "\n" + viewerAppCSS
