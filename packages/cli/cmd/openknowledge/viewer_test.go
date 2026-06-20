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
	if !strings.Contains(page, `data-openknowledge-theme="default"`) || !strings.Contains(page, `--ok-color-accent`) || !strings.Contains(page, `--ok-font-body`) {
		t.Fatalf("viewer file page should expose theme data and root CSS variables:\n%s", page)
	}
	if !strings.Contains(page, `body.viewer-document &gt; header { position: relative; height: var(--ok-header-height); min-height: 0; justify-content: center; padding: 0 22px;`) &&
		!strings.Contains(page, `body.viewer-document > header { position: relative; height: var(--ok-header-height); min-height: 0; justify-content: center; padding: 0 22px;`) {
		t.Fatalf("viewer document header should use a slim fixed height with centered contents:\n%s", page)
	}
	if !strings.Contains(page, `.search.header-search { position: relative; z-index: 6; width: min(460px, 42vw); min-width: 240px; margin: 0; }`) {
		t.Fatalf("viewer header search should keep generic search margins from shifting it off center:\n%s", page)
	}
	if !strings.Contains(page, `.search.header-search { width: min(52vw, 320px); min-width: 0; }`) {
		t.Fatalf("viewer mobile header search should override the desktop minimum width:\n%s", page)
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
	if !strings.Contains(page, `is-single-panel`) || !strings.Contains(page, `justify-content: center`) {
		t.Fatalf("viewer should center a lone open panel before additional panels are opened:\n%s", page)
	}
	if !strings.Contains(page, `.note-workspace.is-single-panel .note-stack { box-sizing: border-box; flex-basis: 100%; min-width: 100%; justify-content: center; padding-left: max(22px, calc((100vw - 1180px) / 2)); padding-right: max(22px, calc((100vw - 1180px) / 2)); }`) {
		t.Fatalf("single-panel stack should use symmetric viewport gutters around the centered panel:\n%s", page)
	}
	if !strings.Contains(page, `.note-workspace.is-single-panel .note-stack { padding-left: 12px; padding-right: 12px; }`) {
		t.Fatalf("single-panel mobile stack should keep symmetric mobile gutters around the centered panel:\n%s", page)
	}
	if !strings.Contains(page, `display: flex; width: 100%; height: calc(100vh - var(--ok-header-height))`) || !strings.Contains(page, `overflow: auto hidden`) {
		t.Fatalf("viewer workspace should use an Andy-style flex horizontal scroll container:\n%s", page)
	}
	if !strings.Contains(page, `display: flex; flex: 0 0 auto; align-self: stretch`) || strings.Contains(page, `.note-stack { position: relative; z-index: 1; display: flex; align-items: stretch; gap: 18px; min-width: max-content; height: 100%`) {
		t.Fatalf("viewer note stack should stretch inside the horizontal scroller without forcing full scrollbar height:\n%s", page)
	}
	if !strings.Contains(page, `.note-stack { position: relative; z-index: 1; display: flex; flex: 0 0 auto; align-self: stretch; align-items: stretch; gap: 18px; min-width: max-content; min-height: 0; padding: 12px max(22px, calc((100vw - 1180px) / 2)) 22px 22px; }`) {
		t.Fatalf("viewer note stack should use a compact top gutter so panels can extend vertically:\n%s", page)
	}
	if !strings.Contains(page, `.note-workspace.is-single-panel .note-stack, .note-workspace.is-multi-panel .note-stack { padding-bottom: 50px; }`) || !strings.Contains(page, `.note-workspace.is-single-panel .note-stack, .note-workspace.is-multi-panel .note-stack { padding-bottom: 46px; }`) {
		t.Fatalf("single and multi-panel stacks should reserve a compact bottom rail gap:\n%s", page)
	}
	if !strings.Contains(page, `min-height: 0; padding: 0 34px 34px; overflow-x: hidden; overflow-y: auto`) || strings.Contains(page, `max-width: none; height: 100%; padding: 0 34px 34px`) {
		t.Fatalf("viewer panels should leave the horizontal scrollbar gutter to the workspace:\n%s", page)
	}
	if !strings.Contains(page, `--note-panel-width`) || !strings.Contains(page, `--ok-note-panel-min-width`) || !strings.Contains(page, `minPanelWidth`) {
		t.Fatalf("viewer panels should expose a resizable width with a minimum width:\n%s", page)
	}
	if !strings.Contains(page, `data-panel-resize-handle`) || !strings.Contains(page, `note-resize-handle-left`) || !strings.Contains(page, `note-resize-handle-right`) {
		t.Fatalf("viewer panels should include left and right resize handles:\n%s", page)
	}
	if !strings.Contains(page, `syncPanelResizeHandles(panel)`) || !strings.Contains(page, `--note-panel-scroll-top`) || !strings.Contains(page, `transform: translateY(var(--note-panel-scroll-top, 0px))`) {
		t.Fatalf("viewer panel resize handles should stay aligned while note content scrolls:\n%s", page)
	}
	if !strings.Contains(page, `panelWidthStorageKey`) || !strings.Contains(page, `readPanelWidths`) || !strings.Contains(page, `savePanelWidths`) || !strings.Contains(page, `writeCookie(panelWidthStorageKey`) {
		t.Fatalf("viewer panel widths should persist per knowledge base:\n%s", page)
	}
	if !strings.Contains(page, `startPanelResize`) || !strings.Contains(page, `resizePanelWithKeyboard`) || !strings.Contains(page, `workspace.scrollLeft = panelResize.startScrollLeft + (nextWidth - panelResize.startWidth)`) {
		t.Fatalf("viewer panels should resize from either edge and keep left-edge resizing anchored:\n%s", page)
	}
	if !strings.Contains(page, `function isSingleCenteredPanel(panel)`) || !strings.Contains(page, `function panelResizeWidthChange(edge, deltaX, centered)`) || !strings.Contains(page, `directionalDelta * (centered ? 2 : 1)`) {
		t.Fatalf("single centered panels should resize symmetrically around their center:\n%s", page)
	}
	if !strings.Contains(page, `if (edge === "left" && !isSingleCenteredPanel(panel))`) || !strings.Contains(page, `if (panelResize.edge === "left" && !panelResize.centered)`) {
		t.Fatalf("left-edge resize should only scroll-anchor multi-panel layouts:\n%s", page)
	}
	if !strings.Contains(page, `nextWidth = maxPanelWidth(panel)`) {
		t.Fatalf("keyboard resize should use the active panel when clamping maximum width:\n%s", page)
	}
	if !strings.Contains(page, `data-workspace-rail`) || !strings.Contains(page, `data-workspace-scroll-track`) || !strings.Contains(page, `data-workspace-scroll-thumb`) {
		t.Fatalf("viewer should include a custom bottom rail for horizontal panel browsing:\n%s", page)
	}
	if !strings.Contains(page, `.workspace-scroll-rail`) || !strings.Contains(page, `.workspace-scroll-thumb`) || !strings.Contains(page, `.note-workspace.is-single-panel, .note-workspace.is-multi-panel { scrollbar-width: none; }`) {
		t.Fatalf("viewer should style a custom rail and hide native workspace scrollbars around note panels:\n%s", page)
	}
	if !strings.Contains(page, `updateWorkspaceRail`) || !strings.Contains(page, `scrollWorkspaceFromRail`) || !strings.Contains(page, `aria-valuenow`) {
		t.Fatalf("viewer should synchronize the custom rail with workspace horizontal scroll:\n%s", page)
	}
	if !strings.Contains(page, `startRailDrag`) || !strings.Contains(page, `startRailTrackJump`) || !strings.Contains(page, `scrollRailWithKeyboard`) {
		t.Fatalf("viewer custom rail should support dragging, track clicks, and keyboard scrolling:\n%s", page)
	}
	if !strings.Contains(page, `(event.key || "").toLowerCase()`) {
		t.Fatalf("viewer custom rail should normalize keyboard events for horizontal scrolling:\n%s", page)
	}
	if !strings.Contains(page, `finishRailDrag`) || !strings.Contains(page, `window.addEventListener("pointerup", stopRailDrag)`) || !strings.Contains(page, `window.removeEventListener("pointerup", stopRailDrag)`) {
		t.Fatalf("viewer custom rail should clean up drag state even when the pointer leaves the thumb:\n%s", page)
	}
	if !strings.Contains(page, `startWorkspaceDrag`) || !strings.Contains(page, `pointerType !== "mouse"`) || !strings.Contains(page, `!closestElement(event.target, "[data-note-path]`) {
		t.Fatalf("viewer should support mouse drag scrolling from workspace gaps without stealing panel text selection:\n%s", page)
	}
	if !strings.Contains(page, `isSpacePanActive()`) || !strings.Contains(page, `window.addEventListener("keydown", startSpacePan, true)`) || !strings.Contains(page, `fromSpacePan: fromSpacePan`) || !strings.Contains(page, `consumeSuppressedWorkspaceClick(event)`) {
		t.Fatalf("viewer should support canvas-style Space+drag panning across note panels without activating links:\n%s", page)
	}
	if !strings.Contains(page, `.note-workspace.is-multi-panel.is-space-panning`) || !strings.Contains(page, `cursor: grab`) || !strings.Contains(page, `user-select: none`) {
		t.Fatalf("viewer should expose Space+drag panning cursor styles:\n%s", page)
	}
	if !strings.Contains(page, `class="viewer-document is-stack-mode"`) {
		t.Fatalf("viewer file page should start in stack panel mode:\n%s", page)
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
	if strings.Contains(page, `data-view-mode-toggle`) || strings.Contains(page, `data-view-mode-icon`) || strings.Contains(page, `is-focus-mode`) {
		t.Fatalf("viewer file page should always use stack panels without focus mode controls:\n%s", page)
	}
	if !strings.Contains(page, `data-sidebar-toggle`) || !strings.Contains(page, `data-file-sidebar`) || !strings.Contains(page, `aria-label="File explorer"`) {
		t.Fatalf("viewer file page did not include file explorer sidebar controls:\n%s", page)
	}
	if !strings.Contains(page, `id="viewer-search"`) || !strings.Contains(page, `data-primary-search`) || !strings.Contains(page, `data-search-url="/api/search"`) || !strings.Contains(page, `searchStaticNotes`) {
		t.Fatalf("viewer file page did not include top bar search:\n%s", page)
	}
	if strings.Contains(page, `id="viewer-sidebar-search"`) || strings.Contains(page, `file-sidebar-search`) {
		t.Fatalf("viewer file sidebar should not include search:\n%s", page)
	}
	if !strings.Contains(page, `.search-results[hidden] { display: none; }`) || !strings.Contains(page, `renderDefaultResults(true)`) || !strings.Contains(page, `defaultSearchResults()`) || !strings.Contains(page, `Top files`) {
		t.Fatalf("viewer search dropdown should stay open on focus with default results for an empty query:\n%s", page)
	}
	if !strings.Contains(page, `isIndexMarkdownPath(path) ? baseScore * 0.55 : baseScore`) || !strings.Contains(page, `isIndexMarkdownPath(a.path) ? 1 : -1`) {
		t.Fatalf("viewer static search should rank index.md files below regular pages:\n%s", page)
	}
	if !strings.Contains(page, `renderResults(results, status, payload.results || [], query`) || strings.Contains(page, `setResultsOpen(false);\n\n      if (staticNotes.length > 0)`) {
		t.Fatalf("viewer search should keep the dropdown open while typed queries are pending:\n%s", page)
	}
	if !strings.Contains(page, `initializeSearchAccessibility`) || !strings.Contains(page, `event.key === "ArrowDown"`) || !strings.Contains(page, `selectedSearchResult(results, activeIndex)`) || !strings.Contains(page, `aria-activedescendant`) {
		t.Fatalf("viewer search should expose combobox keyboard navigation:\n%s", page)
	}
	if !strings.Contains(page, `results.addEventListener("click"`) || !strings.Contains(page, `closeSearch(true)`) || !strings.Contains(page, `.search-result.is-active`) {
		t.Fatalf("viewer search dropdown should close on result activation and style keyboard selection:\n%s", page)
	}
	if !strings.Contains(page, `search-shortcut`) || !strings.Contains(page, `event.metaKey || event.ctrlKey`) || !strings.Contains(page, `primaryInput?.focus()`) {
		t.Fatalf("viewer file page did not include command-k search shortcut:\n%s", page)
	}
	if !strings.Contains(page, `.file-sidebar { position: fixed; top: 0; bottom: 0; left: 0; z-index: 5; display: flex; width: var(--ok-sidebar-width); flex-direction: column; border-right: 0; background: var(--ok-color-sidebar);`) {
		t.Fatalf("viewer file sidebar should not draw a vertical divider against the document canvas:\n%s", page)
	}
	if !strings.Contains(page, `--ok-color-viewer-canvas: #f0f0f0`) || !strings.Contains(page, `background: var(--ok-color-viewer-canvas)`) || !strings.Contains(page, `--ok-color-sidebar: #f0f0f0`) || !strings.Contains(page, `--ok-color-sidebar-header: #f0f0f0`) {
		t.Fatalf("viewer sidebar should share the document canvas surface:\n%s", page)
	}
	if strings.Contains(page, "openInitialNote(treeLink.dataset.treePath, true);\n      setSidebarOpen(false);") {
		t.Fatalf("viewer file sidebar should remain open after opening a tree item:\n%s", page)
	}
	if !strings.Contains(page, `body.viewer-document.is-sidebar-open &gt; header`) && !strings.Contains(page, `body.viewer-document.is-sidebar-open > header`) {
		t.Fatalf("viewer file sidebar should push the page header instead of overlaying it:\n%s", page)
	}
	if !strings.Contains(page, `body.viewer-document.is-sidebar-open &gt; .note-workspace`) && !strings.Contains(page, `body.viewer-document.is-sidebar-open > .note-workspace`) {
		t.Fatalf("viewer file sidebar should push the workspace instead of overlaying it:\n%s", page)
	}
	if !strings.Contains(page, `document.startViewTransition`) || !strings.Contains(page, `view-transition-name: note-workspace`) {
		t.Fatalf("viewer stack changes should use View Transitions when available:\n%s", page)
	}
	if !strings.Contains(page, `clearEnteringPanels();`) || !strings.Contains(page, `document.body.classList.remove("is-view-transitioning")`) || !strings.Contains(page, `transition.updateCallbackDone`) {
		t.Fatalf("viewer stack transitions should clear fallback panel animations before showing the live DOM:\n%s", page)
	}
	if !strings.Contains(page, `data-empty-state`) || !strings.Contains(page, `data-tree-path="workflows/docs.md"`) || !strings.Contains(page, `tree-directory`) {
		t.Fatalf("viewer file page did not include knowledge tree empty state:\n%s", page)
	}
	if strings.Contains(page, `tree-file-path`) || strings.Contains(page, `tree-file::before`) {
		t.Fatalf("viewer file tree should show file names without duplicate path text or md pseudo badges:\n%s", page)
	}
	if !strings.Contains(page, `tree-file-system`) || !strings.Contains(page, `>system</span>`) {
		t.Fatalf("viewer file tree should mark reserved markdown files with a system badge:\n%s", page)
	}
	if !strings.Contains(page, `data-knowledge-graph`) || !strings.Contains(page, `data-knowledge-graph-view`) || !strings.Contains(page, `"source":"index.md"`) || !strings.Contains(page, `"target":"workflows/docs.md"`) {
		t.Fatalf("viewer file page did not include connected knowledge graph data:\n%s", page)
	}
	if !strings.Contains(page, `graphLayoutPositions`) || !strings.Contains(page, `graphGroupCenters`) || strings.Contains(page, `ringNodes`) {
		t.Fatalf("viewer knowledge graph should use a grouped force layout instead of a fixed circle:\n%s", page)
	}
	if !strings.Contains(page, `resolveGraphCollisions`) || !strings.Contains(page, `graphNodeCollisionBox`) || !strings.Contains(page, `graphBoxOverlap`) {
		t.Fatalf("viewer knowledge graph should try to resolve node label collisions:\n%s", page)
	}
	if !strings.Contains(page, `createKnowledgeGraphCanvas`) || !strings.Contains(page, `graphCanvasPhysicsStep`) || !strings.Contains(page, `drawKnowledgeGraphCanvas`) || !strings.Contains(page, `graphCanvasHitTest`) || !strings.Contains(page, `requestAnimationFrame(tick)`) || !strings.Contains(page, `dataset.knowledgeGraphCanvas`) || !strings.Contains(page, `.knowledge-graph-canvas`) {
		t.Fatalf("viewer knowledge graph should render as an animated canvas graph:\n%s", page)
	}
	if !strings.Contains(page, `dataset.activeGraphPath`) || !strings.Contains(page, `graphNodeFullLabel`) || !strings.Contains(page, `graphStatesConnected`) || !strings.Contains(page, `window.location.href = fileURL(activePath)`) {
		t.Fatalf("viewer canvas graph should separate hovered nodes and highlight connected edges:\n%s", page)
	}
	if !strings.Contains(page, `graphEaseInOut`) || !strings.Contains(page, `graphLimitVelocity`) || !strings.Contains(page, `context.globalAlpha = 1`) {
		t.Fatalf("viewer canvas graph should damp hover physics without dimming inactive nodes:\n%s", page)
	}
	if !strings.Contains(page, `graphUniqueNodeLabels`) || !strings.Contains(page, `graphShortestUniquePathSuffix`) || !strings.Contains(page, `parts.slice(-2).join("/")`) {
		t.Fatalf("viewer knowledge graph should disambiguate generic node labels with path suffixes:\n%s", page)
	}
	if !strings.Contains(page, `.knowledge-empty-inner { display: grid`) || !strings.Contains(page, `grid-template-columns: minmax(260px, 30%) minmax(0, 1fr)`) || !strings.Contains(page, `renderKnowledgeGraph()`) {
		t.Fatalf("viewer empty state should render a narrow tree and wide graph layout:\n%s", page)
	}
	if !strings.Contains(page, `context.font = (activeNode ? "600 13px" : "400 12px") + " " + theme.fontBody`) || !strings.Contains(page, `fontBody: themeValue("--ok-font-body"`) || !strings.Contains(page, `String(label || "").length * 7.2`) {
		t.Fatalf("viewer knowledge graph labels should use smaller sans-serif typography:\n%s", page)
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

func TestViewerBrandUsesKnowledgeBaseMetadata(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.1\"\nokf_bundle_name: \"engineering-handbook\"\nokf_bundle_title: \"Engineering Handbook\"\n---\n\n# Home\n")

	page := getViewerBody(t, newViewerHandler(root), "/file/index.md")
	if !strings.Contains(page, `<a class="brand" href="/">Engineering Handbook</a>`) {
		t.Fatalf("viewer should use root okf_bundle_title as the document brand:\n%s", page)
	}
	if strings.Contains(page, `<a class="brand" href="/">Open Knowledge</a>`) {
		t.Fatalf("viewer should not use the product fallback when bundle metadata exists:\n%s", page)
	}

	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.1\"\nokf_bundle_name: \"engineering-handbook\"\n---\n\n# Home\n")
	page = getViewerBody(t, newViewerHandler(root), "/file/index.md")
	if !strings.Contains(page, `<a class="brand" href="/">engineering-handbook</a>`) {
		t.Fatalf("viewer should fall back to root okf_bundle_name when title is absent:\n%s", page)
	}

	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Team Wiki\n")
	page = getViewerBody(t, newViewerHandler(root), "/file/index.md")
	if !strings.Contains(page, `<a class="brand" href="/">Team Wiki</a>`) {
		t.Fatalf("viewer should fall back to the root index heading before the product name:\n%s", page)
	}
}

func TestViewerOpensPDFRawAndHighlightsCodeAssets(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", strings.Join([]string{
		"# Home",
		"",
		"See [Report](references/report.pdf) and [Tool](src/tool.go).",
		"",
		"```go",
		"package main",
		"func main() {",
		"  println(\"ok\")",
		"}",
		"```",
	}, "\n"))
	writeViewerFile(t, root, "references/report.pdf", "%PDF-1.4\n% test pdf\n")
	writeViewerFile(t, root, "src/tool.go", "package main\n\nfunc main() {\n\tprintln(\"ok\")\n}\n")

	handler := newViewerHandler(root)
	page := getViewerBody(t, handler, "/file/index.md")
	if !strings.Contains(page, `href="/raw/references/report.pdf"`) {
		t.Fatalf("viewer should rewrite PDF links to raw browser URLs:\n%s", page)
	}
	if !strings.Contains(page, `href="/file/src/tool.go"`) {
		t.Fatalf("viewer should rewrite code links to asset preview pages:\n%s", page)
	}
	if !strings.Contains(page, `class="code-block language-go"`) || !strings.Contains(page, `tok-keyword">func</span>`) {
		t.Fatalf("viewer should syntax-highlight fenced code blocks:\n%s", page)
	}
	if !strings.Contains(page, `function isMarkdownPath(path)`) {
		t.Fatalf("viewer stack runtime should distinguish markdown links from asset links:\n%s", page)
	}

	rawRecorder := httptest.NewRecorder()
	handler.ServeHTTP(rawRecorder, httptest.NewRequest(http.MethodGet, "/raw/references/report.pdf", nil))
	rawResponse := rawRecorder.Result()
	defer rawResponse.Body.Close()
	if rawResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected raw PDF to return 200, got %d", rawResponse.StatusCode)
	}
	if contentType := rawResponse.Header.Get("Content-Type"); !strings.Contains(contentType, "application/pdf") {
		t.Fatalf("expected raw PDF content type, got %q", contentType)
	}
	if rawResponse.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected raw assets to disable content sniffing")
	}

	codePage := getViewerBody(t, handler, "/file/src/tool.go")
	if !strings.Contains(codePage, `asset-code`) || !strings.Contains(codePage, `class="code-block language-go"`) || !strings.Contains(codePage, `tok-keyword">package</span>`) {
		t.Fatalf("viewer should render highlighted code asset preview:\n%s", codePage)
	}
	if !strings.Contains(codePage, `href="/raw/src/tool.go"`) {
		t.Fatalf("code asset preview should expose raw file link:\n%s", codePage)
	}

	pdfPage := getViewerBody(t, handler, "/file/references/report.pdf")
	if !strings.Contains(pdfPage, `class="asset-frame"`) || !strings.Contains(pdfPage, `src="/raw/references/report.pdf"`) {
		t.Fatalf("direct PDF asset page should embed the raw browser PDF URL:\n%s", pdfPage)
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

func TestViewerTreeMarksOnlyReservedMarkdownAsSystem(t *testing.T) {
	tree := viewerTreeWithURL([]okf.ListEntry{
		{Path: "AGENTS.md"},
		{Path: "index.md", Reserved: true},
		{Path: "notes/log.md", Reserved: true},
		{Path: "notes/runbook.md"},
	}, func(path string) string {
		return "/file/" + path
	})

	systemByPath := map[string]bool{}
	for _, item := range tree {
		if item.Directory {
			continue
		}
		systemByPath[item.Path] = item.System
	}

	for _, path := range []string{"index.md", "notes/log.md"} {
		if !systemByPath[path] {
			t.Fatalf("expected %s to be marked as a system markdown file: %#v", path, tree)
		}
	}
	for _, path := range []string{"AGENTS.md", "notes/runbook.md"} {
		if systemByPath[path] {
			t.Fatalf("expected %s to remain a regular markdown file: %#v", path, tree)
		}
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
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Setup](guides/setup.md), [Agents](AGENTS.md), and [Features](features/index.md).\n")
	writeViewerFile(t, root, "AGENTS.md", "---\ntype: Guide\ntitle: Agents\n---\n\n# Agents\n")
	writeViewerFile(t, root, "features/index.md", "# Features\n")
	writeViewerFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup\n---\n\n# Setup\n\nBack to [Home](../index.md).\n")

	result, err := writeViewerHTMLWithVersion(root, out, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Written) != 4 {
		t.Fatalf("expected four exported viewer files, got %#v", result.Written)
	}

	index := readViewerExportFile(t, out, "index.html")
	if !strings.Contains(index, `data-note-workspace`) || !strings.Contains(index, `data-static-notes`) {
		t.Fatalf("expected exported index to include static viewer app bundle:\n%s", index)
	}
	if !strings.Contains(index, `href="guides/setup.html"`) || !strings.Contains(index, `href="AGENTS.html"`) || !strings.Contains(index, `href="features/index.html"`) {
		t.Fatalf("expected exported index to keep static HTML fallback link:\n%s", index)
	}
	if !strings.Contains(index, `"path":"guides/setup.md"`) || !strings.Contains(index, `"htmlPath":"guides/setup.html"`) || !strings.Contains(index, `"path":"AGENTS.md"`) || !strings.Contains(index, `"htmlPath":"features/index.html"`) {
		t.Fatalf("expected exported index to embed rendered note manifest:\n%s", index)
	}
	if !strings.Contains(index, `function fetchNote(path)`) || !strings.Contains(index, `staticNotesByPath[path]`) {
		t.Fatalf("expected exported index to use static note runtime:\n%s", index)
	}
	if !strings.Contains(index, `function staticHTMLAliases(htmlPath)`) || !strings.Contains(index, `function staticRootPrefixFromCurrentURL`) || !strings.Contains(index, `function staticNotePathForHTMLPath`) {
		t.Fatalf("expected exported index to resolve hosted pretty URLs back to static notes:\n%s", index)
	}
	if !strings.Contains(index, `id="viewer-search"`) || !strings.Contains(index, `data-primary-search`) || !strings.Contains(index, `searchStaticNotes`) || !strings.Contains(index, `staticRelativeURL(item.path)`) {
		t.Fatalf("expected exported index to include static top bar search:\n%s", index)
	}
	if strings.Contains(index, `id="viewer-sidebar-search"`) || strings.Contains(index, `file-sidebar-search`) {
		t.Fatalf("expected exported index to omit sidebar search:\n%s", index)
	}
	if !strings.Contains(index, `data-knowledge-graph`) || !strings.Contains(index, `"source":"index.md"`) || !strings.Contains(index, `"target":"guides/setup.md"`) {
		t.Fatalf("expected exported index to include static knowledge graph:\n%s", index)
	}
	if !strings.Contains(index, `class="powered-by-openknowledge"`) || !strings.Contains(index, `href="https://openknowledge.sh"`) || !strings.Contains(index, `Powered by OpenKnowledge.sh`) {
		t.Fatalf("expected exported index to include OpenKnowledge.sh attribution:\n%s", index)
	}

	setup := readViewerExportFile(t, out, "guides/setup.html")
	if !strings.Contains(setup, `href="../index.html"`) {
		t.Fatalf("expected nested exported page to keep relative static fallback link:\n%s", setup)
	}
}

func TestViewerHTMLExportSkipsUnpublishedPages(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Public](public.md) and [Draft](draft.md).\n")
	writeViewerFile(t, root, "public.md", "---\ntype: Guide\n---\n\n# Public\n")
	writeViewerFile(t, root, "draft.md", "---\ntype: Draft\nokf_publish: false\n---\n\n# Draft\n")
	writeViewerFile(t, root, "examples/index.md", "---\nokf_publish: false\n---\n\n# Examples\n")

	result, err := writeViewerHTMLWithVersion(root, out, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Written, ",") != "index.html,public.html" {
		t.Fatalf("expected only published viewer files, got %#v", result.Written)
	}

	index := readViewerExportFile(t, out, "index.html")
	for _, hidden := range []string{`"path":"draft.md"`, `"path":"examples/index.md"`, `"target":"draft.md"`, `"target":"examples/index.md"`} {
		if strings.Contains(index, hidden) {
			t.Fatalf("expected unpublished page %s to be absent from static payload:\n%s", hidden, index)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "draft.html")); !os.IsNotExist(err) {
		t.Fatalf("expected draft.html to be absent, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "examples", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected examples/index.html to be absent, got err=%v", err)
	}
}

func TestViewerDefaultThemeCSSDefinesSupportedVariables(t *testing.T) {
	required := []string{
		"--ok-font-body",
		"--ok-font-mono",
		"--ok-header-height",
		"--ok-mobile-header-height",
		"--ok-sidebar-width",
		"--ok-note-panel-default-width",
		"--ok-note-panel-min-width",
		"--ok-color-text",
		"--ok-color-document-text",
		"--ok-color-muted",
		"--ok-color-border",
		"--ok-color-page",
		"--ok-color-surface",
		"--ok-color-accent",
		"--ok-color-accent-rgb",
		"--ok-color-accent-strong",
		"--ok-color-accent-soft",
		"--ok-color-accent-softer",
		"--ok-color-accent-selected",
		"--ok-color-accent-focus",
		"--ok-color-accent-focus-strong",
		"--ok-color-accent-border",
		"--ok-color-accent-border-strong",
		"--ok-color-shadow",
		"--ok-color-danger",
		"--ok-color-header-bg",
		"--ok-color-viewer-canvas",
		"--ok-color-viewer-header-bg",
		"--ok-color-control-text",
		"--ok-color-control-hover-text",
		"--ok-color-control-hover-border",
		"--ok-color-control-hover-bg",
		"--ok-color-close-text",
		"--ok-color-close-hover-border",
		"--ok-color-close-hover-bg",
		"--ok-color-sidebar",
		"--ok-color-sidebar-header",
		"--ok-color-sidebar-row",
		"--ok-color-sidebar-border",
		"--ok-color-sidebar-text",
		"--ok-color-sidebar-tree-hover-bg",
		"--ok-color-search-input-border",
		"--ok-color-search-input-bg",
		"--ok-color-search-shortcut-border",
		"--ok-color-search-shortcut-bg",
		"--ok-color-search-shortcut-text",
		"--ok-color-search-popover-border",
		"--ok-color-search-popover-bg",
		"--ok-color-search-popover-shadow",
		"--ok-color-search-result-border",
		"--ok-color-search-result-hover-border",
		"--ok-color-search-result-hover-bg",
		"--ok-color-card-border",
		"--ok-color-card-bg",
		"--ok-color-card-hover-bg",
		"--ok-color-rail-track",
		"--ok-color-rail-thumb",
		"--ok-color-rail-thumb-hover",
		"--ok-color-tree-text",
		"--ok-color-tree-directory-bg",
		"--ok-color-tree-directory-text",
		"--ok-color-tree-directory-marker",
		"--ok-color-tree-badge-border",
		"--ok-color-tree-badge-text",
		"--ok-color-note-resize-hitarea",
		"--ok-color-note-resize-active",
		"--ok-color-note-chrome-bg",
		"--ok-color-note-close-text",
		"--ok-color-note-close-hover-border",
		"--ok-color-note-close-hover-text",
		"--ok-color-editor-trigger-border",
		"--ok-color-editor-trigger-bg",
		"--ok-color-editor-trigger-text",
		"--ok-color-editor-trigger-shadow",
		"--ok-color-editor-trigger-separator",
		"--ok-color-editor-trigger-focus-border",
		"--ok-color-editor-mark-border",
		"--ok-color-editor-mark-bg",
		"--ok-color-editor-mark-text",
		"--ok-color-editor-caret",
		"--ok-color-editor-menu-border",
		"--ok-color-editor-menu-bg",
		"--ok-color-editor-menu-shadow",
		"--ok-color-editor-menu-item-text",
		"--ok-color-editor-menu-item-hover-bg",
		"--ok-color-editor-option-border",
		"--ok-color-editor-option-bg",
		"--ok-color-editor-option-text",
		"--ok-color-editor-menu-separator",
		"--ok-color-code-inline-bg",
		"--ok-color-code-block-bg",
		"--ok-color-code-block-text",
		"--ok-color-syntax-keyword",
		"--ok-color-syntax-string",
		"--ok-color-syntax-number",
		"--ok-color-syntax-comment",
		"--ok-color-graph-edge",
		"--ok-color-graph-edge-muted",
		"--ok-color-graph-edge-active",
		"--ok-color-graph-node-bg",
		"--ok-color-graph-node-border",
		"--ok-color-graph-node-active-border",
		"--ok-color-graph-node-shadow",
		"--ok-color-graph-node-active-shadow",
		"--ok-color-graph-label-halo",
		"--ok-color-graph-label",
		"--ok-color-graph-label-active",
	}

	for _, variable := range required {
		if !strings.Contains(viewerDefaultThemeCSS, variable+":") {
			t.Fatalf("expected default theme CSS to define %s:\n%s", variable, viewerDefaultThemeCSS)
		}
	}
	for _, forbidden := range []string{"--ink:", "--muted:", "--line:", "--paper:", "--panel:", "--accent:"} {
		if strings.Contains(viewerDefaultThemeCSS, forbidden) {
			t.Fatalf("default theme CSS should not expose legacy alias %s:\n%s", forbidden, viewerDefaultThemeCSS)
		}
	}
}

func TestViewerThemeConfigLinksServerAndStaticExport(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, "openknowledge.toml", "[html.theme]\nname = \"landing\"\nstylesheet = \"assets/wiki-theme.css\"\n")
	writeViewerFile(t, root, "assets/wiki-theme.css", ":root { --ok-color-accent: #3257ff; }\n")
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Setup](guides/setup.md).\n")
	writeViewerFile(t, root, "guides/setup.md", "# Setup\n\nBack to [Home](../index.md).\n")
	writeViewerFile(t, root, "references/report.pdf", "%PDF-1.4\n% test pdf\n")

	handler := newViewerHandler(root)
	page := getViewerBody(t, handler, "/file/index.md")
	if !strings.Contains(page, `data-openknowledge-theme="landing"`) || !strings.Contains(page, `href="/raw/assets/wiki-theme.css"`) {
		t.Fatalf("viewer should link the configured theme stylesheet from the raw endpoint:\n%s", page)
	}

	asset := getViewerBody(t, handler, "/file/references/report.pdf")
	if !strings.Contains(asset, `data-openknowledge-theme="landing"`) || !strings.Contains(asset, `href="/raw/assets/wiki-theme.css"`) {
		t.Fatalf("viewer asset pages should link the configured theme stylesheet from the raw endpoint:\n%s", asset)
	}

	alias := getViewerBody(t, newViewerHandlerWithAlias(root, "project-memory"), "/project-memory/file/index.md")
	if !strings.Contains(alias, `data-openknowledge-theme="landing"`) || !strings.Contains(alias, `href="/project-memory/raw/assets/wiki-theme.css"`) {
		t.Fatalf("viewer alias pages should link the prefixed theme stylesheet from the raw endpoint:\n%s", alias)
	}

	listRoot := t.TempDir()
	writeViewerFile(t, listRoot, "openknowledge.toml", "[html.theme]\nname = \"landing\"\nstylesheet = \"assets/wiki-theme.css\"\n")
	writeViewerFile(t, listRoot, "assets/wiki-theme.css", ":root { --ok-color-accent: #3257ff; }\n")
	writeViewerFile(t, listRoot, "notes/readme.md", "---\ntype: Note\n---\n\n# Readme\n")
	listing := getViewerBody(t, newViewerHandler(listRoot), "/")
	if !strings.Contains(listing, `data-openknowledge-theme="landing"`) || !strings.Contains(listing, `href="/raw/assets/wiki-theme.css"`) {
		t.Fatalf("viewer index pages should link the configured theme stylesheet from the raw endpoint:\n%s", listing)
	}

	result, err := writeViewerHTMLWithVersion(root, out, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Written) != 3 {
		t.Fatalf("expected exported pages plus theme stylesheet, got %#v", result.Written)
	}

	index := readViewerExportFile(t, out, "index.html")
	if !strings.Contains(index, `data-openknowledge-theme="landing"`) || !strings.Contains(index, `href="assets/wiki-theme.css"`) {
		t.Fatalf("expected exported index to link copied theme stylesheet:\n%s", index)
	}
	if strings.Contains(index, root) || !strings.Contains(index, `data-note-root=""`) {
		t.Fatalf("static export should not expose local bundle root:\n%s", index)
	}

	setup := readViewerExportFile(t, out, "guides/setup.html")
	if !strings.Contains(setup, `href="../assets/wiki-theme.css"`) {
		t.Fatalf("expected nested exported page to link theme stylesheet relatively:\n%s", setup)
	}
	if strings.Contains(setup, root) || !strings.Contains(setup, `data-note-root=""`) {
		t.Fatalf("nested static export should not expose local bundle root:\n%s", setup)
	}

	theme := readViewerExportFile(t, out, "assets/wiki-theme.css")
	if !strings.Contains(theme, `--ok-color-accent: #3257ff`) {
		t.Fatalf("expected export to copy theme stylesheet, got:\n%s", theme)
	}
}

func TestViewerThemeConfigReportsMissingStylesheetInOpen(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "openknowledge.toml", "[html.theme]\nname = \"landing\"\nstylesheet = \"assets/missing.css\"\n")
	writeViewerFile(t, root, "index.md", "# Home\n")

	handler := newViewerHandler(root)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/file/index.md", nil))
	response := recorder.Result()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected missing open theme stylesheet to return 500, got %d: %s", response.StatusCode, string(body))
	}
	if !strings.Contains(string(body), "theme stylesheet assets/missing.css") {
		t.Fatalf("expected missing stylesheet error to mention the configured theme path, got: %s", string(body))
	}
}

func TestViewerThemeConfigRejectsStylesheetOutsideBundle(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "openknowledge.toml", "[html.theme]\nstylesheet = \"../landing.css\"\n")
	writeViewerFile(t, root, "index.md", "# Home\n")

	if _, err := writeViewerHTMLWithVersion(root, filepath.Join(t.TempDir(), "site"), "0.1"); err == nil || !strings.Contains(err.Error(), "must stay inside the bundle") {
		t.Fatalf("expected outside theme stylesheet to be rejected, got %v", err)
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
	writeViewerFile(t, root, "index.md", "# Home\n\nSee [Workflow](workflows/docs.md) and [Report](references/report.pdf).\n")
	writeViewerFile(t, root, "workflows/docs.md", "---\ntype: Workflow\ntitle: Docs Workflow\n---\n\n# Docs\n\nRun validation before publishing.\n")
	writeViewerFile(t, root, "references/report.pdf", "%PDF-1.4\n% test pdf\n")

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
	if !strings.Contains(page, `href="/project-memory/raw/references/report.pdf"`) {
		t.Fatalf("viewer file did not prefix raw asset links:\n%s", page)
	}
	if !strings.Contains(page, `data-link-prefix="/project-memory"`) || !strings.Contains(page, `linkPrefix + "/api/file/"`) {
		t.Fatalf("viewer file did not expose prefixed stack runtime:\n%s", page)
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/project-memory/raw/references/report.pdf", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected prefixed raw asset to return 200, got %d", recorder.Code)
	}

	api := getViewerJSON(t, handler, "/project-memory/api/file/index.md")
	if !strings.Contains(api.Body, `href="/project-memory/file/workflows/docs.md"`) {
		t.Fatalf("viewer API did not prefix markdown links: %#v", api)
	}
	if !strings.Contains(api.Body, `href="/project-memory/raw/references/report.pdf"`) {
		t.Fatalf("viewer API did not prefix raw links: %#v", api)
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

func TestViewerRejectsTraversalAndNonMarkdownAPI(t *testing.T) {
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
	if _, ok := safeViewerPath(root, "../outside.md"); ok {
		t.Fatal("expected asset traversal path to be rejected")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/file/notes.txt", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected non-markdown file API to return 404, got %d", recorder.Code)
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
	if _, _, err := okf.ConnectRegistryEntry("personal", root, "read", true); err != nil {
		t.Fatal(err)
	}

	alias := directViewerAliasName(root, root, "")
	if alias != "personal" {
		t.Fatalf("expected registry name alias, got %q", alias)
	}
}

func TestViewerAliasDisplayURLUsesReachableHost(t *testing.T) {
	viewURL := viewerAliasDisplayURL("127.0.0.1", "57475", []string{"wiki"})
	if viewURL != "http://127.0.0.1:57475/wiki/" {
		t.Fatalf("expected loopback view URL, got %q", viewURL)
	}

	viewURL = viewerAliasDisplayURL("127.0.0.1", "57475", []string{"wiki", "docs"})
	if viewURL != "http://127.0.0.1:57475/" {
		t.Fatalf("expected registry view URL without a single alias path, got %q", viewURL)
	}
}

func TestBrowserOpenCommand(t *testing.T) {
	tests := []struct {
		goos    string
		command string
		args    []string
	}{
		{goos: "darwin", command: "open", args: []string{"http://127.0.0.1:3000/personal/"}},
		{goos: "linux", command: "xdg-open", args: []string{"http://127.0.0.1:3000/personal/"}},
		{goos: "windows", command: "rundll32", args: []string{"url.dll,FileProtocolHandler", "http://127.0.0.1:3000/personal/"}},
	}

	for _, test := range tests {
		command, args, ok := browserOpenCommand(test.goos, "http://127.0.0.1:3000/personal/")
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
