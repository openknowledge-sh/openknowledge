package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestViewerRendersIndexAndMarkdownFile(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Home\n\nSee [Workflow](workflows/docs.md) and [Concepts](concepts/).\n")
	writeViewerFile(t, root, "workflows/docs.md", "---\ntype: Workflow\ntitle: Docs\n---\n\n# Docs\n\n- Update docs\n")
	writeViewerFile(t, root, "concepts/index.md", "# Concepts\n")

	handler := newViewerHandler(root)

	page := getViewerBody(t, handler, "/file/index.md")
	if strings.Contains(page, "okf_version") {
		t.Fatalf("viewer should strip frontmatter:\n%s", page)
	}
	if !strings.Contains(page, "<h1>Home</h1>") {
		t.Fatalf("viewer did not render heading:\n%s", page)
	}
	if strings.Contains(page, "<span>index.md</span>") {
		t.Fatalf("viewer file header should not repeat the current file path:\n%s", page)
	}
	if !strings.Contains(page, "body.viewer-document &gt; header") && !strings.Contains(page, "body.viewer-document > header") {
		t.Fatalf("viewer file page did not include seamless header override:\n%s", page)
	}
	if !strings.Contains(page, `href="/file/workflows/docs.md"`) {
		t.Fatalf("viewer did not rewrite relative markdown link:\n%s", page)
	}
	if !strings.Contains(page, `href="/file/concepts/index.md"`) {
		t.Fatalf("viewer did not rewrite directory index link:\n%s", page)
	}
	if !strings.Contains(page, `data-note-workspace`) || !strings.Contains(page, `data-note-path="index.md"`) {
		t.Fatalf("viewer file page did not include stacked note layout:\n%s", page)
	}
	if !strings.Contains(page, `note-panel is-active-panel`) || !strings.Contains(page, `.note-panel:not(.is-active-panel) .editor-picker`) {
		t.Fatalf("viewer file page did not limit editor picker to the active panel:\n%s", page)
	}
	if !strings.Contains(page, `data-note-root="`) {
		t.Fatalf("viewer file page did not expose note root for editor deeplinks:\n%s", page)
	}
	if !strings.Contains(page, `data-close-panel`) {
		t.Fatalf("viewer file page did not include panel close control:\n%s", page)
	}
	if !strings.Contains(page, `data-editor-picker`) || !strings.Contains(page, `data-editor-options`) {
		t.Fatalf("viewer file page did not include editor picker:\n%s", page)
	}
	if !strings.Contains(page, `data-editor-open`) || !strings.Contains(page, `data-editor-menu-trigger`) {
		t.Fatalf("viewer file page did not include split editor controls:\n%s", page)
	}
	if !strings.Contains(page, `editorDeepLink`) || !strings.Contains(page, `obsidian://open?path=`) {
		t.Fatalf("viewer file page did not include editor deeplink runtime:\n%s", page)
	}
	if !strings.Contains(page, `data-icon="chevron-down"`) || !strings.Contains(page, `data-icon="x"`) {
		t.Fatalf("viewer file page did not include SVG control icons:\n%s", page)
	}
	if !strings.Contains(page, `data-view-mode-toggle`) || !strings.Contains(page, `data-view-mode-icon="focus"`) || !strings.Contains(page, `data-view-mode-icon="stack"`) {
		t.Fatalf("viewer file page did not include focus/stack mode toggle:\n%s", page)
	}
	if !strings.Contains(page, `data-sidebar-toggle`) || !strings.Contains(page, `data-file-sidebar`) || !strings.Contains(page, `aria-label="File explorer"`) {
		t.Fatalf("viewer file page did not include file explorer sidebar controls:\n%s", page)
	}
	if !strings.Contains(page, `body.viewer-document.is-sidebar-open &gt; header`) && !strings.Contains(page, `body.viewer-document.is-sidebar-open > header`) {
		t.Fatalf("viewer file sidebar should push the page header instead of overlaying it:\n%s", page)
	}
	if !strings.Contains(page, `body.viewer-document.is-sidebar-open &gt; .note-workspace`) && !strings.Contains(page, `body.viewer-document.is-sidebar-open > .note-workspace`) {
		t.Fatalf("viewer file sidebar should push the workspace instead of overlaying it:\n%s", page)
	}
	if !strings.Contains(page, `title="Switch to focus view"`) || !strings.Contains(page, `.view-mode-icon-stack { display: none; }`) || !strings.Contains(page, `body[data-view-mode="focus"] .view-mode-icon-stack { display: block; }`) {
		t.Fatalf("viewer mode toggle should show the mode it will switch to:\n%s", page)
	}
	if !strings.Contains(page, `.is-focus-mode`) {
		t.Fatalf("viewer file page did not include focus mode styles:\n%s", page)
	}
	if strings.Contains(page, `body.viewer-document.is-focus-mode { height: auto`) || strings.Contains(page, `body.viewer-document.is-focus-mode .note-workspace { height: auto`) || !strings.Contains(page, `body.viewer-document.is-focus-mode .note-workspace { overflow: hidden; }`) || !strings.Contains(page, `body.viewer-document.is-focus-mode .note-panel.document`) || !strings.Contains(page, `overflow-y: auto;`) {
		t.Fatalf("viewer focus mode should keep page-level scroll locked and scroll inside the panel:\n%s", page)
	}
	if !strings.Contains(page, `data-empty-state`) || !strings.Contains(page, `data-tree-path="workflows/docs.md"`) || !strings.Contains(page, `tree-directory`) {
		t.Fatalf("viewer file page did not include knowledge tree empty state:\n%s", page)
	}
	if !strings.Contains(page, "/api/file/") {
		t.Fatalf("viewer file page did not include note API runtime:\n%s", page)
	}

	api := getViewerJSON(t, handler, "/api/file/index.md")
	if api.Path != "index.md" || api.Title != "Index" {
		t.Fatalf("unexpected viewer API metadata: %#v", api)
	}
	if !strings.Contains(api.Body, "<h1>Home</h1>") || !strings.Contains(api.Body, `href="/file/workflows/docs.md"`) {
		t.Fatalf("viewer API did not render markdown body with rewritten links: %#v", api)
	}
}

func TestViewerEditorsIncludeCommonFallbacks(t *testing.T) {
	editors := viewerEditors()
	byID := make(map[string]viewerEditor, len(editors))
	for _, editor := range editors {
		byID[editor.ID] = editor
	}

	for _, editorID := range []string{"code", "cursor", "windsurf", "zed"} {
		if byID[editorID].Name == "" {
			t.Fatalf("expected common editor %q in fallback list: %#v", editorID, editors)
		}
	}
	if byID["zed"].Icon == "" {
		t.Fatalf("expected Zed to have a real icon fallback: %#v", byID["zed"])
	}
}

func TestViewerEditorIconFallbackRendersSVG(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")

	handler := newViewerHandler(root)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/editor-icon/zed", nil))
	response := recorder.Result()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from Zed icon fallback, got %d: %s", response.StatusCode, string(body))
	}
	if contentType := response.Header.Get("Content-Type"); !strings.Contains(contentType, "image/svg+xml") {
		t.Fatalf("expected SVG content type, got %q", contentType)
	}
	if !strings.Contains(string(body), "Zed Industries") || !strings.Contains(string(body), `<path fill="#084CCF"`) {
		t.Fatalf("expected Zed brand SVG, got %s", string(body))
	}
}

func TestViewerHTMLExportUsesStackAppBundle(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Setup](guides/setup.md).\n")
	writeViewerFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup\n---\n\n# Setup\n\nBack to [Home](../index.md).\n")

	result, err := writeViewerHTMLWithVersion(root, out, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Written) != 2 {
		t.Fatalf("expected two exported viewer files, got %#v", result.Written)
	}

	index := readViewerExportFile(t, out, "index.html")
	if !strings.Contains(index, `data-note-workspace`) || !strings.Contains(index, `data-static-notes`) {
		t.Fatalf("expected exported index to include static viewer app bundle:\n%s", index)
	}
	if !strings.Contains(index, `href="guides/setup.html"`) {
		t.Fatalf("expected exported index to keep static HTML fallback link:\n%s", index)
	}
	if !strings.Contains(index, `"path":"guides/setup.md"`) || !strings.Contains(index, `"htmlPath":"guides/setup.html"`) {
		t.Fatalf("expected exported index to embed rendered note manifest:\n%s", index)
	}
	if !strings.Contains(index, `function fetchNote(path)`) || !strings.Contains(index, `staticNotesByPath[path]`) {
		t.Fatalf("expected exported index to use static note runtime:\n%s", index)
	}

	setup := readViewerExportFile(t, out, "guides/setup.html")
	if !strings.Contains(setup, `href="../index.html"`) {
		t.Fatalf("expected nested exported page to keep relative static fallback link:\n%s", setup)
	}
}

func TestViewerStartsOnOpenIndexMarkdown(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	writeViewerFile(t, root, "notes/details.md", "# Details\n")

	handler := newViewerHandler(root)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected root to redirect to open index.md, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/file/index.md" {
		t.Fatalf("expected root redirect to /file/index.md, got %q", location)
	}
	if startPath := viewerStartPath(root); startPath != "/file/index.md" {
		t.Fatalf("expected viewer start path to open index.md, got %q", startPath)
	}
}

func TestViewerIndexFallsBackToListWithoutIndexMarkdown(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "notes/details.md", "# Details\n")
	writeViewerFile(t, root, "workflows/docs.md", "# Docs\n")

	handler := newViewerHandler(root)

	index := getViewerBody(t, handler, "/")
	if !strings.Contains(index, "notes/details.md") || !strings.Contains(index, "workflows/docs.md") {
		t.Fatalf("viewer index fallback did not include markdown files:\n%s", index)
	}
	if !strings.Contains(index, `id="viewer-search"`) {
		t.Fatalf("viewer index fallback did not include search input:\n%s", index)
	}
	if startPath := viewerStartPath(root); startPath != "/" {
		t.Fatalf("expected viewer start path to fall back to list, got %q", startPath)
	}
}

func TestViewerSearchAPI(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n\nRead the workflow docs.\n")
	writeViewerFile(t, root, "workflows/docs.md", "---\ntype: Workflow\ntitle: Docs Workflow\n---\n\n# Docs\n\nRun validation before publishing.\n")

	handler := newViewerHandler(root)
	payload := getViewerSearch(t, handler, "/api/search?q=validaton&limit=4")
	if payload.Query != "validaton" {
		t.Fatalf("expected query echo, got %#v", payload)
	}
	if len(payload.Results) == 0 {
		t.Fatalf("expected fuzzy search results, got %#v", payload)
	}
	if payload.Results[0].Path != "workflows/docs.md" || payload.Results[0].URL != "/file/workflows/docs.md" {
		t.Fatalf("unexpected search result: %#v", payload.Results[0])
	}
}

func TestViewerServesDirectAliasPath(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n\nSee [Workflow](workflows/docs.md).\n")
	writeViewerFile(t, root, "workflows/docs.md", "---\ntype: Workflow\ntitle: Docs Workflow\n---\n\n# Docs\n\nRun validation before publishing.\n")

	handler := newViewerHandlerWithAlias(root, "project-memory")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/project-memory/", nil))
	if recorder.Code != http.StatusFound {
		t.Fatalf("expected alias root to redirect to index.md, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/project-memory/file/index.md" {
		t.Fatalf("expected alias redirect to prefixed index.md, got %q", location)
	}

	page := getViewerBody(t, handler, "/project-memory/file/index.md")
	if !strings.Contains(page, `href="/project-memory/file/workflows/docs.md"`) {
		t.Fatalf("viewer file did not prefix markdown links:\n%s", page)
	}
	if !strings.Contains(page, `data-link-prefix="/project-memory"`) || !strings.Contains(page, `linkPrefix + "/api/file/"`) {
		t.Fatalf("viewer file did not expose prefixed stack runtime:\n%s", page)
	}

	api := getViewerJSON(t, handler, "/project-memory/api/file/index.md")
	if !strings.Contains(api.Body, `href="/project-memory/file/workflows/docs.md"`) {
		t.Fatalf("viewer API did not prefix markdown links: %#v", api)
	}

	payload := getViewerSearch(t, handler, "/project-memory/api/search?q=validation")
	if len(payload.Results) == 0 || payload.Results[0].URL != "/project-memory/file/workflows/docs.md" {
		t.Fatalf("unexpected prefixed search result: %#v", payload)
	}
}

func TestViewerSearchRefreshesAfterMarkdownChanges(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")

	handler := newViewerHandler(root)
	first := getViewerSearch(t, handler, "/api/search?q=draft")
	if len(first.Results) != 0 {
		t.Fatalf("expected no draft results before file is written, got %#v", first)
	}

	writeViewerFile(t, root, "notes/draft.md", "---\ntype: Note\ntitle: Draft Note\n---\n\n# Draft\n\nFresh searchable content.\n")
	second := getViewerSearch(t, handler, "/api/search?q=draft")
	if len(second.Results) == 0 || second.Results[0].Path != "notes/draft.md" {
		t.Fatalf("expected refreshed search result, got %#v", second)
	}
}

func TestViewerRejectsTraversalAndNonMarkdown(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	writeViewerFile(t, root, "notes.txt", "not markdown\n")
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("# Outside\n"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := newViewerHandler(root)

	if _, ok := safeMarkdownPath(root, "../outside.md"); ok {
		t.Fatal("expected traversal path to be rejected")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/file/notes.txt", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected non-markdown file to return 404, got %d", recorder.Code)
	}
}

func TestRegistryViewerRendersWorkspaceSelectorAndSwitchesBases(t *testing.T) {
	personal := t.TempDir()
	work := t.TempDir()
	writeViewerFile(t, personal, "index.md", "# Personal\n")
	writeViewerFile(t, personal, "only-personal.md", "---\ntype: Note\n---\n\n# Personal note\n")
	writeViewerFile(t, work, "index.md", "# Work\n\nSee [Guide](notes/guide.md).\n")
	writeViewerFile(t, work, "notes/guide.md", "---\ntype: Note\n---\n\n# Guide\n\nRun validation before publishing.\n")

	handler := newRegistryViewerHandler([]okf.RegistryEntry{
		{Name: "personal", Path: personal},
		{Name: "work", Path: work},
	})

	index := getViewerBody(t, handler, "/")
	for _, required := range []string{
		"Knowledge bases",
		`href="/kb/personal/"`,
		`href="/kb/work/"`,
		"only-personal.md",
	} {
		if !strings.Contains(index, required) {
			t.Fatalf("registry index missing %q:\n%s", required, index)
		}
	}

	workIndex := getViewerBody(t, handler, "/kb/work/")
	if !strings.Contains(workIndex, `class="workspace active" href="/kb/work/"`) {
		t.Fatalf("work knowledge base was not active:\n%s", workIndex)
	}
	if !strings.Contains(workIndex, "notes/guide.md") || strings.Contains(workIndex, "only-personal.md") {
		t.Fatalf("work index did not switch file listing:\n%s", workIndex)
	}

	workPage := getViewerBody(t, handler, "/kb/work/file/index.md")
	if !strings.Contains(workPage, `href="/kb/work/file/notes/guide.md"`) {
		t.Fatalf("registry viewer did not prefix markdown links:\n%s", workPage)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/work/", nil))
	if recorder.Code != http.StatusFound {
		t.Fatalf("expected alias root to redirect to index.md, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/work/file/index.md" {
		t.Fatalf("expected alias redirect to prefixed index.md, got %q", location)
	}

	aliasPage := getViewerBody(t, handler, "/work/file/index.md")
	if !strings.Contains(aliasPage, `href="/work/file/notes/guide.md"`) {
		t.Fatalf("alias route did not prefix markdown links:\n%s", aliasPage)
	}

	aliasSearch := getViewerSearch(t, handler, "/work/api/search?q=validation")
	if len(aliasSearch.Results) == 0 || aliasSearch.Results[0].URL != "/work/file/notes/guide.md" {
		t.Fatalf("unexpected alias search result: %#v", aliasSearch)
	}
}

func TestRegistryViewerEmptyRegistry(t *testing.T) {
	body := getViewerBody(t, newRegistryViewerHandler(nil), "/")
	if !strings.Contains(body, "No registered knowledge bases") {
		t.Fatalf("empty registry page did not explain the empty state:\n%s", body)
	}
}

func TestViewerLocalAliasNameNormalization(t *testing.T) {
	tests := map[string]string{
		"Project Memory":      "project-memory",
		" project_memory.v1 ": "project_memory.v1",
		"Project/Memory/Test": "project-memory-test",
		"--Project Memory---": "project-memory",
		"":                    "",
	}

	for input, expected := range tests {
		if actual := normalizeLocalAliasName(input); actual != expected {
			t.Fatalf("normalizeLocalAliasName(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestDirectViewerAliasNameUsesRegistryPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv(okf.RegistryFileEnv, filepath.Join(t.TempDir(), "registry.json"))
	if _, err := okf.AddRegistryEntry("personal", root); err != nil {
		t.Fatal(err)
	}

	alias := directViewerAliasName(root, root, "")
	if alias != "personal" {
		t.Fatalf("expected registry name alias, got %q", alias)
	}
}

func TestViewerDisplayURLsUseReachableHostAsPrimary(t *testing.T) {
	viewURL, aliasURL := viewerDisplayURLs("127.0.0.1", "57475", "open.knowledge", []string{"wiki"})
	if viewURL != "http://127.0.0.1:57475/wiki/" {
		t.Fatalf("expected loopback view URL, got %q", viewURL)
	}
	if aliasURL != "http://open.knowledge:57475/wiki/" {
		t.Fatalf("expected local-domain alias URL, got %q", aliasURL)
	}

	viewURL, aliasURL = viewerDisplayURLs("127.0.0.1", "57475", "", []string{"wiki"})
	if viewURL != "http://127.0.0.1:57475/wiki/" || aliasURL != "" {
		t.Fatalf("expected alias domain disabled, got view=%q alias=%q", viewURL, aliasURL)
	}
}

func TestBrowserOpenCommand(t *testing.T) {
	tests := []struct {
		goos    string
		command string
		args    []string
	}{
		{goos: "darwin", command: "open", args: []string{"http://open.knowledge:3000/personal/"}},
		{goos: "linux", command: "xdg-open", args: []string{"http://open.knowledge:3000/personal/"}},
		{goos: "windows", command: "rundll32", args: []string{"url.dll,FileProtocolHandler", "http://open.knowledge:3000/personal/"}},
	}

	for _, test := range tests {
		command, args, ok := browserOpenCommand(test.goos, "http://open.knowledge:3000/personal/")
		if !ok || command != test.command || strings.Join(args, "\x00") != strings.Join(test.args, "\x00") {
			t.Fatalf("browserOpenCommand(%q) = %q %#v %v, want %q %#v true", test.goos, command, args, ok, test.command, test.args)
		}
	}

	if _, _, ok := browserOpenCommand("linux", " "); ok {
		t.Fatal("expected empty target to be rejected")
	}
}

func writeViewerFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func getViewerBody(t *testing.T, handler http.Handler, target string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	response := recorder.Result()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from %s, got %d: %s", target, response.StatusCode, string(body))
	}
	return string(body)
}

func getViewerJSON(t *testing.T, handler http.Handler, target string) viewerFilePayload {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	response := recorder.Result()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from %s, got %d: %s", target, response.StatusCode, string(body))
	}

	var payload viewerFilePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("expected viewer API JSON, got %s: %v", string(body), err)
	}
	return payload
}

func getViewerSearch(t *testing.T, handler http.Handler, target string) viewerSearchResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	response := recorder.Result()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from %s, got %d: %s", target, response.StatusCode, string(body))
	}

	var payload viewerSearchResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func readViewerExportFile(t *testing.T, root string, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
