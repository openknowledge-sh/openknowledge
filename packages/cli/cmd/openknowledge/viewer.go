package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
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

	fmt.Printf("Open Knowledge view: http://%s%s\n", addr, viewerStartPath(absolute))
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
		if startPath := viewerStartPath(root); startPath != "/" {
			http.Redirect(response, request, startPath, http.StatusFound)
			return
		}
		renderViewerIndex(response, root)
	})
	mux.HandleFunc("/file/", func(response http.ResponseWriter, request *http.Request) {
		rel := strings.TrimPrefix(request.URL.Path, "/file/")
		renderViewerFile(response, request, root, rel)
	})
	mux.HandleFunc("/api/file/", func(response http.ResponseWriter, request *http.Request) {
		rel := strings.TrimPrefix(request.URL.Path, "/api/file/")
		renderViewerFileAPI(response, request, root, rel)
	})
	mux.HandleFunc("/api/editor-icon/", func(response http.ResponseWriter, request *http.Request) {
		editorID := strings.TrimPrefix(request.URL.Path, "/api/editor-icon/")
		renderViewerEditorIcon(response, request, editorID)
	})
	return mux
}

func viewerStartPath(root string) string {
	filePath, ok := safeMarkdownPath(root, "index.md")
	if !ok {
		return "/"
	}
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return "/"
	}
	return fileURL("index.md")
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
	Title       string
	Root        string
	Path        string
	FileURL     string
	Body        template.HTML
	Tree        []viewerTreeItem
	EditorsJSON template.JS
	StaticJSON  template.JS
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
}

func renderViewerFile(response http.ResponseWriter, request *http.Request, root string, rel string) {
	data, ok, err := viewerFile(root, rel)
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

func renderViewerFileAPI(response http.ResponseWriter, request *http.Request, root string, rel string) {
	data, ok, err := viewerFile(root, rel)
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

func viewerFile(root string, rel string) (viewerFileData, bool, error) {
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

	return viewerFileData{
		Title:       titleForMarkdownFile(cleanRel),
		Root:        root,
		Path:        cleanRel,
		FileURL:     fileURL(cleanRel),
		Body:        template.HTML(okf.RenderMarkdown(stripFrontmatter(string(content)), cleanRel, okf.ViewerLink)),
		Tree:        viewerTree(listing.Entries),
		EditorsJSON: viewerEditorsJSON(),
	}, true, nil
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
		})
	}
	return tree
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
<body class="viewer-document">
  <header>
    <a class="brand" href="/">Open Knowledge</a>
    <button class="view-mode-toggle" type="button" data-view-mode-toggle aria-label="Switch to focus view" aria-pressed="false" title="Switch to focus view">
      <svg class="view-mode-icon view-mode-icon-focus" data-view-mode-icon="focus" viewBox="0 0 24 24" aria-hidden="true">
        <rect x="6.5" y="4.5" width="11" height="15" rx="1.8"></rect>
      </svg>
      <svg class="view-mode-icon view-mode-icon-stack" data-view-mode-icon="stack" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M3.5 7.5h3.2v9H3.5"></path>
        <rect x="8" y="4.5" width="8" height="15" rx="1.8"></rect>
        <path d="M20.5 7.5h-3.2v9h3.2"></path>
      </svg>
    </button>
  </header>
  <main class="note-workspace" data-note-workspace data-note-root="{{.Root}}">
    <section class="knowledge-empty" data-empty-state aria-label="Knowledge base files" hidden>
      <div class="knowledge-empty-inner">
        <div class="knowledge-tree" role="tree">
          {{range .Tree}}
            {{if .Directory}}
              <div class="tree-row tree-directory" role="treeitem" aria-expanded="true" style="--indent: {{.Indent}}px">{{.Name}}</div>
            {{else}}
              <a class="tree-row tree-file" role="treeitem" href="{{.URL}}" data-tree-path="{{.Path}}" style="--indent: {{.Indent}}px">
                <span class="tree-file-name">{{.Name}}</span>
                <span class="tree-file-path">{{.Path}}</span>
              </a>
            {{end}}
          {{else}}
            <p class="empty">No Markdown files found.</p>
          {{end}}
        </div>
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
  {{if .StaticJSON}}<script type="application/json" data-static-notes>{{.StaticJSON}}</script>{{end}}
  <script>` + viewerJS + `</script>
</body>
</html>`))

const viewerJS = `
(function () {
  const workspace = document.querySelector("[data-note-workspace]");
  const stackEl = document.querySelector("[data-note-stack]");
  const emptyState = document.querySelector("[data-empty-state]");

  if (!workspace || !stackEl) {
    return;
  }

  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  const editorStorageKey = "openknowledge.viewer.editorOrder";
  const viewModeStorageKey = "openknowledge.viewer.viewMode";
  const editorOptions = readEditorOptions();
  const staticNotes = readStaticNotes();
  const staticNotesByPath = indexStaticNotes(staticNotes, "path");
  const staticNotePathByHTML = indexStaticNotePathsByHTML(staticNotes);
  let viewMode = readViewMode();

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

    if (url.origin !== window.location.origin || !url.pathname.startsWith("/file/")) {
      return null;
    }

    const raw = url.pathname.slice("/file/".length) || "index.md";
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
    return encodedNoteURL("/file/", path);
  }

  function apiURL(path) {
    return encodedNoteURL("/api/file/", path);
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

  function readViewMode() {
    try {
      return window.localStorage.getItem(viewModeStorageKey) === "focus" ? "focus" : "stack";
    } catch {
      return "stack";
    }
  }

  function writeViewMode(nextMode) {
    try {
      window.localStorage.setItem(viewModeStorageKey, nextMode);
    } catch {
      return;
    }
  }

  function isFocusMode() {
    return viewMode === "focus";
  }

  function focusStack(paths) {
    const source = Array.isArray(paths) ? paths : currentStack();
    return source.length ? [source[0]] : [];
  }

  function applyViewModeUI() {
    const focus = isFocusMode();
    document.body.dataset.viewMode = viewMode;
    document.body.classList.toggle("is-focus-mode", focus);
    document.body.classList.toggle("is-stack-mode", !focus);

    const toggle = document.querySelector("[data-view-mode-toggle]");
    if (toggle) {
      toggle.setAttribute("aria-pressed", focus ? "true" : "false");
      toggle.setAttribute("aria-label", focus ? "Switch to stack view" : "Switch to focus view");
      toggle.title = focus ? "Switch to stack view" : "Switch to focus view";
    }

    updateWorkspaceState();
    updateActiveLinks();
    updateTitle();
  }

  function setViewMode(nextMode, pushHistory) {
    viewMode = nextMode === "focus" ? "focus" : "stack";
    writeViewMode(viewMode);
    applyViewModeUI();

    const paths = isFocusMode() ? focusStack() : currentStack();
    updateHistory(paths, pushHistory);
    if (!isFocusMode()) {
      const all = panels();
      const last = all[all.length - 1];
      if (last) {
        scrollToPanel(last);
      }
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
    const currentPanel = activePanel() || (isFocusMode() ? all[0] : all[all.length - 1]);
    if (!currentPanel) {
      document.title = "Knowledge base - Open Knowledge";
      return;
    }
    const title = currentPanel?.dataset.noteTitle || currentPanel?.dataset.notePath || "Open Knowledge";
    document.title = title + " - Open Knowledge";
  }

  function updateHistory(paths, pushHistory) {
    const nextURL = stackURL(paths);
    const state = { stack: paths, viewMode };
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

    if (isFocusMode()) {
      return;
    }

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

  async function appendNote(path, animate) {
    let panel;
    try {
      panel = createPanel(await fetchNote(path), animate);
    } catch (error) {
      panel = createErrorPanel(path, error);
    }

    stackEl.append(panel);
    setActivePanel(panel);
    updateWorkspaceState();
    updateActiveLinks();
    updateTitle();
    scrollToPanel(panel);
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
    clearStack();
    await appendNote(targetPath, true);
    updateHistory(currentStack(), pushHistory);
  }

  function closePanel(panel, pushHistory) {
    const before = panels();
    const index = before.indexOf(panel);
    panel.remove();

    const remaining = panels();
    updateWorkspaceState();
    updateActiveLinks();
    updateTitle();
    updateHistory(currentStack(), pushHistory);

    if (!remaining.length) {
      return;
    }

    const nextPanel = remaining[Math.min(Math.max(index, 0), remaining.length - 1)];
    setActivePanel(nextPanel);
    scrollToPanel(nextPanel);
  }

  async function openFromPanel(sourcePanel, targetPath, pushHistory) {
    const all = panels();
    let sourceIndex = all.indexOf(sourcePanel);
    if (sourceIndex < 0) {
      sourceIndex = all.length - 1;
    }

    trimAfter(sourceIndex);
    await appendNote(targetPath, true);

    updateHistory(currentStack(), pushHistory);
  }

  async function restoreStack(paths) {
    clearStack();
    for (const path of paths) {
      await appendNote(path, false);
    }

    updateWorkspaceState();
    updateActiveLinks();
    updateTitle();
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
    if (treeLink) {
      if (isFocusMode()) {
        return;
      }
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
        return;
      }
      event.preventDefault();
      openInitialNote(treeLink.dataset.treePath, true);
      return;
    }

    const link = closestElement(event.target, "a[href]");
    if (!link || link.dataset.directLink === "true") {
      return;
    }
    if (isFocusMode()) {
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

  const viewModeToggle = document.querySelector("[data-view-mode-toggle]");
  if (viewModeToggle) {
    viewModeToggle.addEventListener("click", function () {
      setViewMode(isFocusMode() ? "stack" : "focus", true);
    });
  }

  window.addEventListener("popstate", function () {
    const paths = stackFromLocation();
    if (isFocusMode()) {
      restoreStack(focusStack(paths)).then(applyViewModeUI);
      return;
    }
    restoreStack(paths).then(applyViewModeUI);
  });

  document.addEventListener("click", function (event) {
    if (!closestElement(event.target, "[data-editor-picker]")) {
      closeEditorMenus();
    }
  });
  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape") {
      closeEditorMenus();
    }
  });

  const requestedStack = stackFromLocation();
  panels().forEach(bindPanel);
  ensureActivePanel();
  if (isFocusMode()) {
    const focusedStack = focusStack(requestedStack);
    updateHistory(focusedStack, false);
    if (focusedStack.length !== panels().length || focusedStack[0] !== panels()[0]?.dataset.notePath) {
      restoreStack(focusedStack).then(applyViewModeUI);
    } else {
      applyViewModeUI();
    }
  } else if (requestedStack.length !== 1 || requestedStack[0] !== panels()[0]?.dataset.notePath) {
    window.history.replaceState({ stack: requestedStack, viewMode }, "", window.location.href);
    restoreStack(requestedStack).then(applyViewModeUI);
  } else {
    window.history.replaceState({ stack: requestedStack, viewMode }, "", window.location.href);
    applyViewModeUI();
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
  font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
* { box-sizing: border-box; }
body { margin: 0; color: var(--ink); background: var(--paper); line-height: 1.55; }
body.viewer-document { height: 100vh; overflow: hidden; }
header { display: flex; min-height: var(--header-height); justify-content: space-between; align-items: center; gap: 16px; padding: 14px 22px; border-bottom: 1px solid var(--line); background: rgba(255, 255, 255, .92); color: var(--muted); font-size: 13px; }
body.viewer-document > header { border-bottom: 0; background: #eef1ee; }
.brand { color: var(--ink); font-weight: 700; text-decoration: none; }
.view-mode-toggle { display: inline-flex; flex: 0 0 auto; width: 34px; height: 34px; align-items: center; justify-content: center; border: 1px solid transparent; border-radius: 8px; background: transparent; color: #5f6d67; cursor: pointer; }
.view-mode-toggle:hover, .view-mode-toggle:focus-visible { border-color: #cbd5cf; background: #e7ece8; color: #25302b; outline: none; }
.view-mode-icon { display: block; width: 22px; height: 22px; fill: none; stroke: currentColor; stroke-linecap: round; stroke-linejoin: round; stroke-width: 1.8; }
.view-mode-icon-stack { display: none; }
body[data-view-mode="focus"] .view-mode-icon-focus { display: none; }
body[data-view-mode="focus"] .view-mode-icon-stack { display: block; }
.control-icon { display: block; fill: none; stroke: currentColor; stroke-linecap: round; stroke-linejoin: round; stroke-width: 2; }
main { width: min(960px, calc(100% - 32px)); margin: 0 auto; padding: 34px 0 56px; }
.note-workspace { position: relative; width: 100%; height: calc(100vh - var(--header-height)); margin: 0; padding: 0; overflow-x: auto; overflow-y: hidden; background: #eef1ee; scroll-behavior: smooth; overscroll-behavior-x: contain; }
.note-stack { position: relative; z-index: 1; display: flex; align-items: stretch; gap: 18px; min-width: max-content; height: 100%; padding: 22px max(22px, calc((100vw - 1180px) / 2)) 26px 22px; }
.note-workspace.is-empty { overflow-x: hidden; overflow-y: auto; }
.note-workspace.is-empty .note-stack { display: none; }
.knowledge-empty { position: absolute; inset: 0; z-index: 0; overflow: auto; background: #eef1ee; }
.knowledge-empty[hidden] { display: none; }
.knowledge-empty-inner { min-height: 100%; padding: 52px max(24px, calc((100vw - 960px) / 2)) 64px; }
.knowledge-tree { width: min(720px, 100%); color: #51605a; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 13px; }
.tree-row { display: flex; min-height: 30px; align-items: center; gap: 12px; padding: 4px 10px 4px var(--indent); border-radius: 6px; color: inherit; text-decoration: none; }
.tree-directory { margin: 7px 0 2px; background: #e1e6e2; color: #56645f; font-weight: 700; }
.tree-directory::before { color: #82908a; content: "/"; }
.tree-file::before { flex: 0 0 auto; padding: 1px 5px; border: 1px solid #cad4ce; border-radius: 4px; color: #708078; content: "md"; font-size: 10px; line-height: 1.2; }
.tree-file:hover { background: rgba(var(--accent-rgb), .09); color: var(--accent); }
.tree-file-name { flex: 0 0 auto; }
.tree-file-path { overflow: hidden; color: #7b8983; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
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
.note-panel.document { flex: 0 0 min(650px, calc(100vw - 44px)); max-width: none; height: 100%; padding: 0 34px 34px; overflow-y: auto; border: 1px solid var(--line); background: var(--panel); box-shadow: 0 18px 46px var(--shadow); outline: none; scroll-padding-top: 62px; }
body.viewer-document.is-focus-mode { height: auto; min-height: 100vh; overflow-x: hidden; overflow-y: auto; background: #eef1ee; }
body.viewer-document.is-focus-mode .note-workspace { height: auto; min-height: calc(100vh - var(--header-height)); overflow: visible; }
body.viewer-document.is-focus-mode .note-workspace:not(.is-empty) .note-stack { display: block; min-width: 0; height: auto; padding: 22px max(22px, calc((100vw - 980px) / 2)) 56px; }
body.viewer-document.is-focus-mode .note-panel.document { display: block; width: min(780px, calc(100vw - 44px)); max-width: 780px; min-height: calc(100vh - var(--header-height) - 48px); height: auto; margin: 0 auto; overflow: visible; }
body.viewer-document.is-focus-mode .note-panel:not(:first-child) { display: none; }
body.viewer-document.is-focus-mode .note-close { display: none; }
.note-panel:focus { border-color: rgba(var(--accent-rgb), .45); }
.note-panel.is-entering { animation: note-enter .28s cubic-bezier(.22, .8, .2, 1); }
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
@media (prefers-reduced-motion: reduce) {
  .note-workspace { scroll-behavior: auto; }
  .note-panel.is-entering { animation: none; }
  .note-body a { transition: none; }
}
@media (max-width: 680px) {
  header { display: block; }
  header span { display: block; margin-top: 4px; overflow-wrap: anywhere; }
  .row { grid-template-columns: 1fr; }
  body.viewer-document header { display: flex; min-height: var(--header-height); }
  .note-workspace { height: calc(100vh - 68px); }
  .note-stack { gap: 12px; padding: 12px; }
  .knowledge-empty-inner { padding: 28px 14px 44px; }
  .tree-file-path { display: none; }
  .note-panel.document { flex-basis: calc(100vw - 24px); padding: 0 22px 28px; }
  body.viewer-document.is-focus-mode .note-workspace { min-height: calc(100vh - var(--header-height)); }
  body.viewer-document.is-focus-mode .note-workspace:not(.is-empty) .note-stack { padding: 12px 12px 40px; }
  body.viewer-document.is-focus-mode .note-panel.document { width: calc(100vw - 24px); min-height: calc(100vh - var(--header-height) - 24px); }
  .note-chrome { margin: 0 -22px 22px; padding: 0 10px 0 22px; }
}
`
