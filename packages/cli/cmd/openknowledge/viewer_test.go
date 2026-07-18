package main

import (
	"encoding/json"
	"errors"
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
	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Home\n\nSee [Workflow](workflows/docs.md) and [Concepts](concepts/).\n\n| Name | Kind | Count |\n| :--- | --- | ---: |\n| `path` | argument | 1 |\n| `--spec` | flag | 2 |\n")
	writeViewerFile(t, root, "workflows/docs.md", "---\ntype: Workflow\ntitle: Docs\n---\n\n# Docs\n\n- Update docs\n")
	writeViewerFile(t, root, "concepts/index.md", "# Concepts\n")

	handler := newViewerHandler(root)

	page := getViewerBody(t, handler, "/file/index.md")
	if !strings.Contains(page, `class="ok-frontmatter" data-frontmatter`) ||
		strings.Contains(page, `class="ok-frontmatter" data-frontmatter open`) ||
		!strings.Contains(page, `<code>okf_version</code>`) ||
		!strings.Contains(page, `data-frontmatter-type="string"`) {
		t.Fatalf("viewer should render typed frontmatter above the markdown body:\n%s", page)
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
	if !strings.Contains(page, `data-openknowledge-theme="default"`) ||
		!strings.Contains(page, `data-viewer-theme="night"`) ||
		!strings.Contains(page, `const defaultPreset = "night";`) ||
		!strings.Contains(page, `const defaultThemePreset = "night";`) ||
		!strings.Contains(page, `--ok-color-accent`) ||
		!strings.Contains(page, `--ok-font-body`) {
		t.Fatalf("viewer file page should expose theme data and root CSS variables:\n%s", page)
	}
	if !strings.Contains(page, `--ok-viewport-height: 100vh`) || !strings.Contains(page, `--ok-viewport-height: 100svh`) || !strings.Contains(page, `-webkit-text-size-adjust: 100%; text-size-adjust: 100%;`) {
		t.Fatalf("viewer should normalize iOS viewport height and text scaling:\n%s", page)
	}
	if !strings.Contains(page, `--ok-note-panel-default-width: min(calc(65ch + 68px), calc(100vw - 44px));`) ||
		!strings.Contains(page, `cssLengthPixels("var(--ok-note-panel-default-width)", 650)`) {
		t.Fatalf("viewer default panel width should follow a 65ch reading measure with matching resize fallback:\n%s", page)
	}
	if !strings.Contains(page, `body.viewer-document &gt; header { position: relative; height: var(--ok-header-height); min-height: 0; z-index: 6; justify-content: center; padding: 0 22px;`) &&
		!strings.Contains(page, `body.viewer-document > header { position: relative; height: var(--ok-header-height); min-height: 0; z-index: 6; justify-content: center; padding: 0 22px;`) {
		t.Fatalf("viewer document header should use a slim fixed height with centered contents:\n%s", page)
	}
	if !strings.Contains(page, `border-bottom: 0; background: var(--ok-color-viewer-header-bg);`) ||
		!strings.Contains(page, `.editor-open .editor-mark { border: 0; background: transparent; }`) {
		t.Fatalf("viewer document chrome should avoid redundant nested borders:\n%s", page)
	}
	if !strings.Contains(page, `.search.header-search { position: relative; z-index: 6; width: min(460px, 42vw); min-width: 240px; margin: 0; }`) {
		t.Fatalf("viewer header search should keep generic search margins from shifting it off center:\n%s", page)
	}
	if !strings.Contains(page, `.search.header-search { width: min(44vw, 280px); min-width: 0; margin-right: 44px; }`) {
		t.Fatalf("viewer mobile header search should reserve a separate settings-control slot:\n%s", page)
	}
	if !strings.Contains(page, `data-viewer-settings-trigger`) ||
		!strings.Contains(page, `data-theme-option="default"`) ||
		!strings.Contains(page, `data-theme-option="night"`) ||
		!strings.Contains(page, `data-theme-option="paper"`) ||
		!strings.Contains(page, `data-theme-option="ocean"`) ||
		!strings.Contains(page, `data-theme-option="rose"`) ||
		!strings.Contains(page, `data-theme-option="custom"`) ||
		!strings.Contains(page, `<span>Light</span>`) {
		t.Fatalf("viewer should render the settings theme selector with built-in and custom themes:\n%s", page)
	}
	if !strings.Contains(page, `openknowledge.viewer.theme`) ||
		!strings.Contains(page, `data-theme-custom-value="page"`) ||
		!strings.Contains(page, `data-theme-custom-value="surface"`) ||
		!strings.Contains(page, `data-theme-custom-value="text"`) ||
		!strings.Contains(page, `data-theme-custom-value="muted"`) ||
		!strings.Contains(page, `data-theme-custom-value="accent"`) ||
		!strings.Contains(page, `data-theme-custom-value="border"`) {
		t.Fatalf("viewer should persist theme choices and expose custom theme controls:\n%s", page)
	}
	if !strings.Contains(page, `data-frontmatter-visibility checked`) ||
		!strings.Contains(page, `openknowledge.viewer.frontmatter`) ||
		!strings.Contains(page, `applyFrontmatterPreference`) ||
		!strings.Contains(page, `body.is-frontmatter-hidden .ok-frontmatter`) {
		t.Fatalf("viewer should persist a global frontmatter visibility preference:\n%s", page)
	}
	if !strings.Contains(page, `data-accessibility-font`) ||
		!strings.Contains(page, `data-accessibility-size`) ||
		!strings.Contains(page, `data-accessibility-spacing`) ||
		!strings.Contains(page, `data-accessibility-motion`) ||
		!strings.Contains(page, `data-readable-line-length`) ||
		!strings.Contains(page, `data-high-contrast`) ||
		!strings.Contains(page, `data-underline-links`) {
		t.Fatalf("viewer should render system-level reading and accessibility controls:\n%s", page)
	}
	if !strings.Contains(page, `openknowledge.viewer.accessibility`) ||
		!strings.Contains(page, `applyAccessibilityPreference`) ||
		!strings.Contains(page, `motionIsReduced`) ||
		!strings.Contains(page, `body.is-links-underlined .note-body a`) ||
		!strings.Contains(page, `dataset.viewerUnderlines = normalized.underlineLinks ? "on" : "off"`) ||
		!strings.Contains(page, `html[data-viewer-underlines="off"] .note-body a`) ||
		!strings.Contains(page, `html[data-viewer-contrast="high"]`) {
		t.Fatalf("viewer should persist and apply reading and accessibility preferences:\n%s", page)
	}
	if !strings.Contains(page, `href="/file/workflows/docs.md"`) {
		t.Fatalf("viewer did not rewrite relative markdown link:\n%s", page)
	}
	if !strings.Contains(page, `href="/file/concepts/index.md"`) {
		t.Fatalf("viewer did not rewrite directory index link:\n%s", page)
	}
	if !strings.Contains(page, `class="ok-table-wrap" data-ok-table-wrap`) || !strings.Contains(page, `class="ok-table-scroller"`) || !strings.Contains(page, `class="ok-table" data-ok-table`) {
		t.Fatalf("viewer should render markdown tables with stable table wrappers:\n%s", page)
	}
	if !strings.Contains(page, `<th scope="col" data-align="left">Name</th>`) || !strings.Contains(page, `<td data-align="right">2</td>`) {
		t.Fatalf("viewer should preserve markdown table alignment metadata:\n%s", page)
	}
	if !strings.Contains(page, `.code-block[data-language] { padding-top: 34px; }`) || !strings.Contains(page, `content: attr(data-language)`) || !strings.Contains(page, `opacity: .78`) || !strings.Contains(page, `font: 400 .94em/1.6 var(--ok-font-mono)`) {
		t.Fatalf("viewer should style code blocks with a subtle language label and body-sized code text:\n%s", page)
	}
	if strings.Contains(page, `.code-block[data-language]::before { display: block; margin: -16px`) || strings.Contains(page, `radial-gradient(circle at 16px 50%`) {
		t.Fatalf("viewer code block label should not use the heavy header strip treatment:\n%s", page)
	}
	if !strings.Contains(page, `.tok-command { color: var(--ok-color-syntax-keyword); font-weight: inherit; }`) || !strings.Contains(page, `.tok-flag`) || !strings.Contains(page, `.tok-variable`) {
		t.Fatalf("viewer should include shell-specific syntax token styles without oversized command text:\n%s", page)
	}
	if !strings.Contains(page, `.ok-table-tools`) || !strings.Contains(page, `.ok-table-filter-menu`) || !strings.Contains(page, `Clear filters`) || !strings.Contains(page, `function enhanceTables(scope)`) || !strings.Contains(page, `bindSortableTableHeader`) || !strings.Contains(page, `Filter table`) {
		t.Fatalf("viewer should embed rich table styling and runtime controls:\n%s", page)
	}
	if !strings.Contains(page, `.ok-table-filter-trigger { display: inline-flex; height: 30px; align-items: center; gap: 8px; padding: 0 9px; border: 1px solid transparent; border-radius: 6px; background: transparent;`) {
		t.Fatalf("viewer table filters trigger should use a ghost button treatment:\n%s", page)
	}
	if !strings.Contains(page, `.ok-table code { overflow-wrap: anywhere; white-space: normal; word-break: break-word; }`) {
		t.Fatalf("viewer table inline code should wrap long paths inside cells:\n%s", page)
	}
	if !strings.Contains(page, `.note-chrome { position: sticky; top: 0; z-index: 5;`) || !strings.Contains(page, `.ok-table-tools { position: relative; z-index: 2;`) || !strings.Contains(page, `.ok-table-filter-menu[open] { z-index: 3; }`) {
		t.Fatalf("viewer note chrome should stay layered above sticky table controls while scrolling:\n%s", page)
	}
	if !strings.Contains(page, `data-note-workspace`) || !strings.Contains(page, `data-note-path="index.md"`) {
		t.Fatalf("viewer file page did not include stacked note layout:\n%s", page)
	}
	if !strings.Contains(page, `.document h2 { margin: 40px 0 13px; padding-top: 0; border-top: 0;`) || !strings.Contains(page, `.document li + li { margin-top: 6px; }`) || !strings.Contains(page, `.document h1 code, .document h2 code, .document h3 code`) {
		t.Fatalf("viewer document typography should distinguish sections and list items:\n%s", page)
	}
	if !strings.Contains(page, `is-single-panel`) || !strings.Contains(page, `justify-content: center`) {
		t.Fatalf("viewer should center a lone open panel before additional panels are opened:\n%s", page)
	}
	if !strings.Contains(page, `.note-workspace.is-single-panel .note-stack { box-sizing: border-box; flex-basis: 100%; min-width: 100%; justify-content: center; padding-left: max(24px, calc((100vw - 1180px) / 2)); padding-right: max(24px, calc((100vw - 1180px) / 2)); }`) {
		t.Fatalf("single-panel stack should use symmetric viewport gutters around the centered panel:\n%s", page)
	}
	if !strings.Contains(page, `.note-workspace.is-single-panel .note-stack { padding-left: 12px; padding-right: 12px; }`) {
		t.Fatalf("single-panel mobile stack should keep symmetric mobile gutters around the centered panel:\n%s", page)
	}
	if !strings.Contains(page, `display: flex; width: 100%; height: calc(var(--ok-viewport-height) - var(--ok-header-height))`) || !strings.Contains(page, `overflow: auto hidden`) {
		t.Fatalf("viewer workspace should use an Andy-style flex horizontal scroll container:\n%s", page)
	}
	if !strings.Contains(page, `display: flex; flex: 0 0 auto; align-self: stretch`) || strings.Contains(page, `.note-stack { position: relative; z-index: 1; display: flex; align-items: stretch; gap: 18px; min-width: max-content; height: 100%`) {
		t.Fatalf("viewer note stack should stretch inside the horizontal scroller without forcing full scrollbar height:\n%s", page)
	}
	if !strings.Contains(page, `.note-stack { position: relative; z-index: 1; display: flex; flex: 0 0 auto; align-self: stretch; align-items: stretch; gap: 16px; min-width: max-content; min-height: 0; padding: 16px max(24px, calc((100vw - 1180px) / 2)) 24px 24px; }`) {
		t.Fatalf("viewer note stack should use a compact top gutter so panels can extend vertically:\n%s", page)
	}
	if !strings.Contains(page, `.note-workspace.is-single-panel .note-stack, .note-workspace.is-multi-panel .note-stack { padding-bottom: 50px; }`) || !strings.Contains(page, `.note-workspace.is-single-panel .note-stack, .note-workspace.is-multi-panel .note-stack { padding-bottom: 18px; }`) {
		t.Fatalf("single and multi-panel stacks should reserve a compact bottom rail gap on desktop and reclaim it on mobile:\n%s", page)
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
	if !strings.Contains(page, `@media (max-width: 680px), (hover: none) and (pointer: coarse)`) || !strings.Contains(page, `.workspace-scroll-rail, .powered-by-openknowledge { display: none; }`) {
		t.Fatalf("viewer mobile and touch layouts should hide fixed bottom chrome instead of letting it conflict with Safari chrome:\n%s", page)
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
	if !strings.Contains(page, `data-note-breadcrumbs data-note-path-value="index.md"`) ||
		!strings.Contains(page, `function createNoteBreadcrumbs(path)`) ||
		!strings.Contains(page, `noteIndexPath(displayParts.slice(0, index + 1))`) ||
		!strings.Contains(page, `link.dataset.directLink = "true"`) ||
		!strings.Contains(page, `link.setAttribute("aria-current", "page")`) {
		t.Fatalf("viewer file page should enhance note paths into index-aware breadcrumb links:\n%s", page)
	}
	if !strings.Contains(page, `id: "viewer.panel.close"`) ||
		!strings.Contains(page, `code: "KeyW"`) ||
		!strings.Contains(page, `label: "⌘⌥W"`) ||
		!strings.Contains(page, `closeablePanel()`) ||
		!strings.Contains(page, `ariaKeyShortcut(panelCloseShortcut)`) ||
		!strings.Contains(page, `remaining[Math.min(Math.max(index - 1, 0), remaining.length - 1)]`) {
		t.Fatalf("viewer file page should close the focused panel with a primary-alt-w shortcut and focus the previous panel:\n%s", page)
	}
	if strings.Contains(page, `class="note-close-shortcut" data-panel-close-shortcut`) ||
		!strings.Contains(page, `aria-keyshortcuts="Meta+Alt+W"`) ||
		!strings.Contains(page, `title="Close index.md (⌘⌥W)"`) ||
		!strings.Contains(page, `closeButton.title = "Close note (" + panelCloseShortcut.label + ")"`) {
		t.Fatalf("viewer file page should expose the panel close shortcut through the close-button tooltip:\n%s", page)
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
	if !strings.Contains(page, `[item.path, item.type, item.heading].filter(Boolean).join(" - ")`) {
		t.Fatalf("viewer search results should expose canonical section headings in their metadata:\n%s", page)
	}
	if strings.Contains(page, `id="viewer-sidebar-search"`) || strings.Contains(page, `file-sidebar-search`) {
		t.Fatalf("viewer file sidebar should not include search:\n%s", page)
	}
	if !strings.Contains(page, `.search-results[hidden] { display: none; }`) || !strings.Contains(page, `renderDefaultResults(true)`) || !strings.Contains(page, `defaultSearchResults()`) || !strings.Contains(page, `Top files`) {
		t.Fatalf("viewer search dropdown should stay open on focus with default results for an empty query:\n%s", page)
	}
	if !strings.Contains(page, `document.addEventListener("pointerdown"`) ||
		!strings.Contains(page, `!results.hidden && !search.contains(event.target)`) ||
		!strings.Contains(page, `document.addEventListener("focusin"`) {
		t.Fatalf("viewer search dropdown should dismiss on outside pointer and focus:\n%s", page)
	}
	if !strings.Contains(page, `.header-search .search-results { position: absolute; top: calc(100% + 8px);`) ||
		!strings.Contains(page, `.header-search .search-result-title`) ||
		!strings.Contains(page, `.header-search .search-result-snippet`) {
		t.Fatalf("viewer header search should use a streamlined result surface with clear hierarchy:\n%s", page)
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
	if !strings.Contains(page, `item.highlightURL || item.url`) ||
		!strings.Contains(page, `ok-highlight`) ||
		!strings.Contains(page, `applySearchHighlight`) ||
		!strings.Contains(page, `mark.ok-search-highlight`) ||
		!strings.Contains(page, `.ok-search-highlight`) {
		t.Fatalf("viewer should support search result deep-link highlighting:\n%s", page)
	}
	if !strings.Contains(page, `search-shortcut`) ||
		!strings.Contains(page, `label: "⌘K"`) ||
		!strings.Contains(page, `event.metaKey || event.ctrlKey`) ||
		!strings.Contains(page, `primaryInput?.focus()`) {
		t.Fatalf("viewer file page did not include command-k search shortcut:\n%s", page)
	}
	if !strings.Contains(page, `window.OpenKnowledgeShortcuts`) ||
		!strings.Contains(page, `register: register`) ||
		!strings.Contains(page, `if (shortcut.label)`) ||
		!strings.Contains(page, `document.addEventListener("keydown", handleKeydown)`) ||
		!strings.Contains(page, `id: "viewer.search.focus"`) {
		t.Fatalf("viewer file page did not include the shared shortcut registry:\n%s", page)
	}
	if !strings.Contains(page, `.file-sidebar { position: fixed; top: 0; bottom: 0; left: 0; z-index: 7; display: flex; width: var(--ok-sidebar-width); flex-direction: column; border-right: 1px solid var(--ok-color-sidebar-border); background: var(--ok-color-sidebar);`) {
		t.Fatalf("viewer file sidebar should use a subtle divider against the document canvas:\n%s", page)
	}
	if !strings.Contains(page, `--ok-color-viewer-canvas: #f4f5f4`) || !strings.Contains(page, `background: var(--ok-color-viewer-canvas)`) || !strings.Contains(page, `--ok-color-sidebar: #f7f8f7`) || !strings.Contains(page, `--ok-color-sidebar-header: rgba(247, 248, 247, .94)`) {
		t.Fatalf("viewer sidebar should use the polished neutral shell palette:\n%s", page)
	}
	if !strings.Contains(page, `const mobileSidebar = window.matchMedia("(max-width: 680px)")`) ||
		!strings.Contains(page, "if (mobileSidebar.matches) {\n        setSidebarOpen(false);\n      }") {
		t.Fatalf("viewer file sidebar should close after opening a tree item only on mobile widths:\n%s", page)
	}
	if !strings.Contains(page, `body.viewer-document.is-sidebar-open &gt; header`) && !strings.Contains(page, `body.viewer-document.is-sidebar-open > header`) {
		t.Fatalf("viewer file sidebar should push the page header instead of overlaying it:\n%s", page)
	}
	if !strings.Contains(page, `body.viewer-document.is-sidebar-open &gt; .note-workspace`) && !strings.Contains(page, `body.viewer-document.is-sidebar-open > .note-workspace`) {
		t.Fatalf("viewer file sidebar should push the workspace instead of overlaying it:\n%s", page)
	}
	if !strings.Contains(page, `id: "viewer.sidebar.toggle"`) ||
		!strings.Contains(page, `code: "KeyS"`) ||
		!strings.Contains(page, `metaOrCtrlKey: true`) ||
		!strings.Contains(page, `label: "⌘⌥S"`) ||
		!strings.Contains(page, `sidebarToggle.setAttribute("aria-keyshortcuts"`) {
		t.Fatalf("viewer file sidebar should register a primary-alt-s keyboard shortcut:\n%s", page)
	}
	if !strings.Contains(page, `class="sidebar-shortcut" data-sidebar-shortcut`) ||
		!strings.Contains(page, `document.querySelectorAll("[data-sidebar-shortcut]")`) ||
		!strings.Contains(page, `.sidebar-shortcut { display: inline-flex; min-width: 44px; height: 22px;`) ||
		!strings.Contains(page, `.sidebar-shortcut { display: none; }`) {
		t.Fatalf("viewer file sidebar should show a visible shortcut badge:\n%s", page)
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
	if !strings.Contains(page, `.tree-directory { margin: 7px 0 1px; background: transparent;`) ||
		!strings.Contains(page, `.file-sidebar .tree-directory { background: transparent;`) ||
		!strings.Contains(page, `.tree-directory::before { content: none; }`) {
		t.Fatalf("viewer file tree should render directories as lightweight text rows:\n%s", page)
	}
	if strings.Contains(page, `tree-file-path`) || strings.Contains(page, `tree-file::before`) {
		t.Fatalf("viewer file tree should show file names without duplicate path text or md pseudo badges:\n%s", page)
	}
	if !strings.Contains(page, `tree-file-system`) || !strings.Contains(page, `>system</span>`) {
		t.Fatalf("viewer file tree should mark reserved markdown files with a system badge:\n%s", page)
	}
	if !strings.Contains(page, `.tree-file-name { flex: 0 1 auto;`) ||
		!strings.Contains(page, `.tree-file-system { flex: 0 0 auto;`) ||
		strings.Contains(page, `.tree-file-system { margin-left: auto;`) {
		t.Fatalf("viewer file tree should keep system badges adjacent to file names:\n%s", page)
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
	if strings.Contains(page, `context.shadowBlur`) || strings.Contains(page, `context.strokeText(label`) || strings.Contains(page, `--ok-color-graph-label-halo`) {
		t.Fatalf("viewer canvas graph hover should avoid node shadows and label halo text:\n%s", page)
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
	if !strings.Contains(api.Frontmatter, `<code>okf_version</code>`) || !strings.Contains(api.Frontmatter, `data-frontmatter-type="string"`) {
		t.Fatalf("viewer API did not include typed frontmatter: %#v", api)
	}
	if !strings.Contains(api.Body, "<h1>Home</h1>") || !strings.Contains(api.Body, `href="/file/workflows/docs.md"`) {
		t.Fatalf("viewer API did not render markdown body with rewritten links: %#v", api)
	}
	if !strings.Contains(api.Body, `class="ok-table" data-ok-table`) || !strings.Contains(api.Body, `<th scope="col" data-align="left">Name</th>`) || !strings.Contains(api.Body, `<td data-align="right">2</td>`) {
		t.Fatalf("viewer API did not render markdown table wrappers and alignment metadata: %#v", api)
	}
}

func TestViewerRendersStructuredFrontmatterByType(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n\nSee [Runbook](runbook.md).\n")
	writeViewerFile(t, root, "runbook.md", `---
type: Runbook
enabled: false
retries: 0
optional: null
tags: [docs, 7, true]
metadata:
  tags: [internal]
owner:
  name: Platform
  active: true
steps:
  - name: Validate
    required: true
empty:
empty_list: []
source: https://openknowledge.sh/wiki/
unsafe: "<script>alert('no')</script>"
---

# Runbook

Typed metadata stays visible.
`)

	handler := newViewerHandler(root)
	page := getViewerBody(t, handler, "/file/runbook.md")
	checks := []string{
		`<span class="ok-frontmatter-title">Frontmatter</span>`,
		`<span class="ok-frontmatter-count">12 fields</span>`,
		`data-frontmatter-type="boolean"`,
		`class="ok-frontmatter-scalar ok-frontmatter-boolean" data-value="false"`,
		`data-frontmatter-type="number"`,
		`class="ok-frontmatter-scalar ok-frontmatter-number">0</span>`,
		`data-frontmatter-type="null"`,
		`class="ok-frontmatter-chips ok-frontmatter-tags"`,
		`class="ok-frontmatter-tag-link" href="?ok-tag=docs" data-frontmatter-tag="docs" data-direct-link="true"`,
		`class="ok-frontmatter-list"`,
		`<code>empty</code>`,
		`<code>empty_list</code>`,
		`class="ok-frontmatter-scalar ok-frontmatter-empty">[]</span>`,
		`href="https://openknowledge.sh/wiki/"`,
		`&lt;script&gt;alert(&#39;no&#39;)&lt;/script&gt;`,
		`<h1>Runbook</h1>`,
	}
	for _, check := range checks {
		if !strings.Contains(page, check) {
			t.Fatalf("viewer frontmatter missing %q:\n%s", check, page)
		}
	}
	if strings.Contains(page, `class="ok-frontmatter-type"`) {
		t.Fatalf("viewer frontmatter should not render datatype badges:\n%s", page)
	}
	if strings.Contains(page, `data-frontmatter-tag="internal"`) {
		t.Fatalf("viewer frontmatter should facet only top-level tags:\n%s", page)
	}
	if strings.Contains(page, `<script>alert('no')</script>`) {
		t.Fatalf("viewer frontmatter should escape HTML values:\n%s", page)
	}

	typeIndex := strings.Index(page, `<code>type</code>`)
	enabledIndex := strings.Index(page, `<code>enabled</code>`)
	retriesIndex := strings.Index(page, `<code>retries</code>`)
	if typeIndex < 0 || enabledIndex <= typeIndex || retriesIndex <= enabledIndex {
		t.Fatalf("viewer should preserve authored top-level frontmatter order:\n%s", page)
	}

	api := getViewerJSON(t, handler, "/api/file/runbook.md")
	if !strings.Contains(api.Frontmatter, `data-frontmatter-type="object"`) || !strings.Contains(api.Frontmatter, `data-value="false"`) {
		t.Fatalf("viewer API should retain structured frontmatter for dynamic panels: %#v", api)
	}
	if !strings.Contains(api.Frontmatter, `data-frontmatter-tag="docs"`) {
		t.Fatalf("viewer API should retain navigable tag facets for dynamic panels: %#v", api)
	}
}

func TestViewerFrontmatterRendersFlowMappingsWithoutHidingMarkdown(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.1\"\nconfig: {mode: fast}\n---\n\n# Home\n\nStill readable.\n")

	page := getViewerBody(t, newViewerHandler(root), "/file/index.md")
	if strings.Contains(page, "Structured preview is unavailable") ||
		!strings.Contains(page, `<code>config</code>`) ||
		!strings.Contains(page, `<code>mode</code>`) ||
		!strings.Contains(page, `>fast</span>`) {
		t.Fatalf("viewer should render structured flow-mapping frontmatter:\n%s", page)
	}
	if !strings.Contains(page, "<h1>Home</h1>") || !strings.Contains(page, "Still readable.") {
		t.Fatalf("frontmatter rendering should not hide the markdown body:\n%s", page)
	}
}

func TestViewerRendersMarkdownExtensionFilesFromAST(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n\nSee [Guide](guide.markdown).\n")
	writeViewerFile(t, root, "guide.markdown", "---\r\ntype: Guide\r\ntitle: Guide\r\n---\r\n\r\n# Guide\r\n\r\nRendered from AST.\r\n")

	handler := newViewerHandler(root)

	page := getViewerBody(t, handler, "/file/guide.markdown")
	if strings.Contains(page, "type: Guide") || strings.Contains(page, "---\r\n") {
		t.Fatalf("viewer should not expose raw frontmatter delimiters, got:\n%s", page)
	}
	if !strings.Contains(page, `class="ok-frontmatter" data-frontmatter`) || strings.Contains(page, `class="ok-frontmatter" data-frontmatter open`) || !strings.Contains(page, `<code>type</code>`) {
		t.Fatalf("viewer should show parsed .markdown frontmatter:\n%s", page)
	}
	if !strings.Contains(page, "<h1>Guide</h1>") || !strings.Contains(page, "Rendered from AST.") {
		t.Fatalf("viewer did not render .markdown body:\n%s", page)
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
	if !strings.Contains(page, `class="code-block language-go" data-language="go"`) || !strings.Contains(page, `tok-keyword">func</span>`) {
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
	if !strings.Contains(codePage, `asset-code`) || !strings.Contains(codePage, `class="code-block language-go" data-language="go"`) || !strings.Contains(codePage, `tok-keyword">package</span>`) {
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
	enablePublicArtifactTest(t, root)
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Setup](guides/setup.md), [Agents](AGENTS.md), and [Features](features/index.md).\n\n| Kind | Required |\n| --- | --- |\n| flag | no |\n| argument | yes |\n")
	writeViewerFile(t, root, "AGENTS.md", "---\ntype: Guide\ntitle: Agents\n---\n\n# Agents\n")
	writeViewerFile(t, root, "features/index.md", "# Features\n")
	writeViewerFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup\nenabled: false\ntags: [docs, setup]\n---\n\n# Setup\n\nBack to [Home](../index.md).\n")

	result, err := writeViewerHTMLWithVersion(root, out, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Written) != 11 {
		t.Fatalf("expected exported viewer files plus discovery files, bundle manifest, and archive, got %#v", result.Written)
	}

	indexHTML := readViewerExportFile(t, out, "index.html")
	viewerRuntime := readViewerExportFile(t, out, viewerThemeScriptAsset) +
		readViewerExportFile(t, out, viewerShortcutsScriptAsset) +
		readViewerExportFile(t, out, viewerAppScriptAsset) +
		readViewerExportFile(t, out, viewerSearchScriptAsset)
	index := indexHTML + viewerRuntime
	if !strings.Contains(indexHTML, `data-note-workspace`) || !strings.Contains(indexHTML, `data-static-notes`) {
		t.Fatalf("expected exported index to include static viewer data:\n%s", indexHTML)
	}
	for _, src := range []string{
		`assets/openknowledge/viewer-theme.js`,
		`assets/openknowledge/viewer-shortcuts.js`,
		`assets/openknowledge/viewer-app.js`,
		`assets/openknowledge/viewer-search.js`,
	} {
		if !strings.Contains(indexHTML, `src="`+src+`"`) {
			t.Fatalf("expected exported index to load same-origin script %s:\n%s", src, indexHTML)
		}
	}
	if strings.Contains(indexHTML, `<script>`) {
		t.Fatalf("generated viewer code must not require executable inline scripts:\n%s", indexHTML)
	}
	if !strings.Contains(index, `data-viewer-theme="night"`) ||
		!strings.Contains(index, `const defaultPreset = "night";`) ||
		!strings.Contains(index, `const defaultThemePreset = "night";`) {
		t.Fatalf("expected exported viewer to use Night on first run and bootstrap saved theme choices before paint:\n%s", index)
	}
	if !strings.Contains(index, `class="ok-table" data-ok-table`) || !strings.Contains(index, `function enhanceTables(scope)`) || !strings.Contains(index, `.ok-table-tools`) || !strings.Contains(index, `.ok-table-filter-panel`) {
		t.Fatalf("expected exported viewer pages to include rich table markup and runtime:\n%s", index)
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
	if !strings.Contains(index, `"frontmatter":"\u003cdetails class=\"ok-frontmatter\"`) ||
		!strings.Contains(index, `openknowledge.viewer.frontmatter`) ||
		!strings.Contains(index, `(data.frontmatter || "") + data.body`) ||
		!strings.Contains(index, `htmlToText(note.frontmatter || "")`) {
		t.Fatalf("expected exported viewer runtime to carry typed frontmatter into dynamic panels:\n%s", index)
	}
	if !strings.Contains(index, `"tags":["docs","setup"]`) ||
		!strings.Contains(index, `function searchStaticTag(tag, excludePath)`) ||
		!strings.Contains(index, `tagSearchFromLocation()`) {
		t.Fatalf("expected exported viewer runtime to carry exact tag facets into static search:\n%s", index)
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
	llms := readViewerExportFile(t, out, "llms.txt")
	if !strings.Contains(llms, "# Home") || !strings.Contains(llms, "## Docs") ||
		!strings.Contains(llms, "- [Setup](guides/setup.html): guides/setup.md") ||
		!strings.Contains(llms, "- [Agents](AGENTS.html): AGENTS.md") {
		t.Fatalf("expected llms.txt to list exported wiki pages:\n%s", llms)
	}
	if _, err := os.Stat(filepath.Join(out, "sitemap.xml")); !os.IsNotExist(err) {
		t.Fatalf("expected sitemap.xml to be absent without html.site.base_url, got err=%v", err)
	}

	setup := readViewerExportFile(t, out, "guides/setup.html")
	if !strings.Contains(setup, `href="../index.html"`) {
		t.Fatalf("expected nested exported page to keep relative static fallback link:\n%s", setup)
	}
	if !strings.Contains(setup, `src="../assets/openknowledge/viewer-theme.js"`) ||
		!strings.Contains(setup, `src="../assets/openknowledge/viewer-app.js"`) ||
		strings.Contains(setup, `<script>`) {
		t.Fatalf("expected nested exported page to load same-origin scripts without executable inline code:\n%s", setup)
	}
	if !strings.Contains(setup, `class="ok-frontmatter" data-frontmatter`) ||
		strings.Contains(setup, `class="ok-frontmatter" data-frontmatter open`) ||
		!strings.Contains(setup, `data-value="false"`) ||
		!strings.Contains(setup, `class="ok-frontmatter-chips ok-frontmatter-tags"`) ||
		strings.Contains(setup, `class="ok-frontmatter-type"`) {
		t.Fatalf("expected nested exported page to render typed frontmatter:\n%s", setup)
	}

	manifestContent := readViewerExportFile(t, out, okf.BundleManifestRelPath)
	var manifest okf.BundleManifest
	if err := json.Unmarshal([]byte(manifestContent), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Type != okf.BundleManifestType || manifest.Archive != okf.BundleArchiveRelPath || manifest.ArchiveFormat != okf.BundleArchiveFormat {
		t.Fatalf("unexpected export manifest: %#v", manifest)
	}
	archivePath := filepath.Join(out, filepath.FromSlash(okf.BundleArchiveRelPath))
	hash, err := okf.SHA256File(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ArchiveSHA256 != hash {
		t.Fatalf("expected manifest hash %s to match archive hash %s", manifest.ArchiveSHA256, hash)
	}
	extracted := filepath.Join(t.TempDir(), "bundle")
	if err := okf.ExtractBundleArchive(archivePath, extracted); err != nil {
		t.Fatal(err)
	}
	validation, err := okf.Validate(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if len(validation.Errors) != 0 {
		t.Fatalf("expected exported archive to validate, got %#v", validation.Errors)
	}
}

func TestViewerHTMLExportSkipsUnpublishedPages(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Public](public.md) and [Draft](draft.md).\n")
	writeViewerFile(t, root, "public.md", "---\ntype: Guide\n---\n\n# Public\n")
	writeViewerFile(t, root, "draft.md", "---\ntype: Draft\nokf_publish: false\n---\n\n# Draft\n")
	writeViewerFile(t, root, "examples/index.md", "---\nokf_publish: false\n---\n\n# Examples\n")
	writeViewerFile(t, root, "assets/public/logo.svg", "<svg/>\n")
	writeViewerFile(t, root, "assets/private/diagram.svg", "<svg>private</svg>\n")
	writeViewerFile(t, root, "secret.txt", "do not publish\n")
	writeViewerFile(t, root, "openknowledge.toml", "[publish]\nenabled = true\nassets = [\"assets/public/**\", \"**/*.md\"]\n")

	result, err := writeViewerHTMLWithVersion(root, out, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Written, ",") != "assets/openknowledge-bundle.tar.gz,assets/openknowledge/viewer-app.js,assets/openknowledge/viewer-search.js,assets/openknowledge/viewer-shortcuts.js,assets/openknowledge/viewer-theme.js,assets/public/logo.svg,index.html,llms.txt,openknowledge.json,public.html" {
		t.Fatalf("expected only published viewer files, got %#v", result.Written)
	}
	if content := readViewerExportFile(t, out, "assets/public/logo.svg"); content != "<svg/>\n" {
		t.Fatalf("unexpected published asset content: %q", content)
	}
	for _, hidden := range []string{"assets/private/diagram.svg", "secret.txt", "openknowledge.toml"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(hidden))); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be absent from public site, got err=%v", hidden, err)
		}
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
	llms := readViewerExportFile(t, out, "llms.txt")
	if strings.Contains(llms, "draft.md") || strings.Contains(llms, "examples/index.md") {
		t.Fatalf("expected unpublished pages to be absent from llms.txt:\n%s", llms)
	}
	extracted := filepath.Join(t.TempDir(), "published-bundle")
	if err := okf.ExtractBundleArchive(filepath.Join(out, filepath.FromSlash(okf.BundleArchiveRelPath)), extracted); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(extracted, "public.md")); err != nil {
		t.Fatalf("expected published page in portable archive: %v", err)
	}
	for _, hidden := range []string{"draft.md", "examples/index.md", "assets/private/diagram.svg", "secret.txt", "openknowledge.toml"} {
		if _, err := os.Stat(filepath.Join(extracted, filepath.FromSlash(hidden))); !os.IsNotExist(err) {
			t.Fatalf("expected unpublished page %s to be absent from portable archive, got err=%v", hidden, err)
		}
	}
	if _, err := os.Stat(filepath.Join(extracted, "assets", "public", "logo.svg")); err != nil {
		t.Fatalf("expected allowlisted asset in portable archive: %v", err)
	}
}

func TestViewerHTMLExportHonorsPublicationTargets(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, "openknowledge.toml", "[publish]\nenabled = true\n\n[html.site]\nbase_url = \"https://example.test/wiki/\"\n")
	writeViewerFile(t, root, "index.md", "# Home\n")
	writeViewerFile(t, root, "no-search.md", "---\ntype: Guide\ntitle: No Search\nokf_targets:\n  search: false\n---\n\n# Unique Search Needle\n")
	writeViewerFile(t, root, "no-llms.md", "---\ntype: Guide\ntitle: No LLMS\nokf_targets:\n  llms: false\n---\n\n# No LLMS\n")
	writeViewerFile(t, root, "no-sitemap.md", "---\ntype: Guide\ntitle: No Sitemap\nokf_targets:\n  sitemap: false\n---\n\n# No Sitemap\n")
	writeViewerFile(t, root, "no-viewer.md", "---\ntype: Guide\ntitle: No Viewer\nokf_targets:\n  viewer: false\n---\n\n# No Viewer\n")

	if _, err := writeViewerHTMLWithVersion(root, out, "0.1"); err != nil {
		t.Fatal(err)
	}
	for _, visible := range []string{"index.html", "no-search.html", "no-llms.html", "no-sitemap.html"} {
		if _, err := os.Stat(filepath.Join(out, visible)); err != nil {
			t.Fatalf("expected viewer target %s: %v", visible, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "no-viewer.html")); !os.IsNotExist(err) {
		t.Fatalf("viewer=false page must be physically absent, got %v", err)
	}
	index := readViewerExportFile(t, out, "index.html")
	if strings.Contains(index, "Unique Search Needle") {
		t.Fatalf("search=false page leaked into static search payload:\n%s", index)
	}
	llms := readViewerExportFile(t, out, "llms.txt")
	if strings.Contains(llms, "no-llms.md") || strings.Contains(llms, "no-viewer.md") || !strings.Contains(llms, "no-sitemap.md") {
		t.Fatalf("unexpected llms target projection:\n%s", llms)
	}
	sitemap := readViewerExportFile(t, out, "sitemap.xml")
	if strings.Contains(sitemap, "no-sitemap.html") || strings.Contains(sitemap, "no-viewer.html") || !strings.Contains(sitemap, "no-llms.html") {
		t.Fatalf("unexpected sitemap target projection:\n%s", sitemap)
	}
}

func TestViewerHTMLExportRejectsInvalidBundleBeforeWriting(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, "index.md", "# Home\n")
	writeViewerFile(t, root, "invalid.md", "# Missing required concept frontmatter\n")
	writeViewerFile(t, out, "sentinel.txt", "previous generation\n")

	if _, err := writeViewerHTMLWithVersion(root, out, "0.1"); err == nil || !strings.Contains(err.Error(), "bundle validation failed") {
		t.Fatalf("expected invalid HTML publication refusal, got %v", err)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "sentinel.txt" {
		t.Fatalf("invalid publication must not write partial output: %#v", entries)
	}
}

func TestViewerHTMLExportRejectsUnknownProjectConfigBeforeWriting(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, "index.md", "# Home\n")
	writeViewerFile(t, root, "openknowledge.toml", "[html.theme]\ncss = \"assets/theme.css\"\n")
	if _, err := writeViewerHTMLWithVersion(root, out, "0.1"); err == nil || !strings.Contains(err.Error(), "fields in the document are missing in the target struct") {
		t.Fatalf("expected strict project config refusal, got %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("invalid project config must not publish HTML output, got %v", err)
	}
}

func TestViewerHTMLExportReplacesWholeGeneration(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, root)
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, "index.md", "# Home\n")
	writeViewerFile(t, root, "old.md", "---\ntype: Note\n---\n\n# Old\n")
	if _, err := writeViewerHTMLWithVersion(root, out, "0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "old.html")); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(root, "old.md")); err != nil {
		t.Fatal(err)
	}
	writeViewerFile(t, root, "new.md", "---\ntype: Note\n---\n\n# New\n")
	if _, err := writeViewerHTMLWithVersion(root, out, "0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "new.html")); err != nil {
		t.Fatalf("expected new generation page: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "old.html")); !os.IsNotExist(err) {
		t.Fatalf("expected stale page to be removed by generation swap, got %v", err)
	}
}

func TestViewerHTMLExportInsideBundleExcludesPreviousOutputFromArchive(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, root)
	out := filepath.Join(root, "site")
	writeViewerFile(t, root, "index.md", "# Home\n")
	if _, err := writeViewerHTMLWithVersion(root, out, "0.1"); err != nil {
		t.Fatal(err)
	}
	writeViewerFile(t, out, "leak.txt", "must not enter the source archive\n")

	if _, err := writeViewerHTMLWithVersion(root, out, "0.1"); err != nil {
		t.Fatal(err)
	}
	extracted := filepath.Join(t.TempDir(), "bundle")
	archivePath := filepath.Join(out, filepath.FromSlash(okf.BundleArchiveRelPath))
	if err := okf.ExtractBundleArchive(archivePath, extracted); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(extracted, "site")); !os.IsNotExist(err) {
		t.Fatalf("expected previous nested output to be excluded from the source archive, got %v", err)
	}
}

func TestViewerHTMLExportInjectsHeadHTMLWhenConfigured(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, root)
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Topic](notes/topic.md).\n")
	writeViewerFile(t, root, "notes/topic.md", "---\ntype: Note\n---\n\n# Topic\n")

	headHTML, err := loadHeadInjection(headInjectionOptions{
		HTML:       `<meta name="ok-static-head" content="1">`,
		ScriptSrcs: []string{"https://openknowledge.sh/analytics.js"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeViewerHTMLWithOptions(root, out, "0.1", viewerHTMLExportOptions{HeadHTML: headHTML}); err != nil {
		t.Fatal(err)
	}

	index := readViewerExportFile(t, out, "index.html")
	if !strings.Contains(index, `<meta name="ok-static-head" content="1">`) || !strings.Contains(index, `<script src="https://openknowledge.sh/analytics.js"></script>`) {
		t.Fatalf("expected exported index to include custom head HTML:\n%s", index)
	}

	nested := readViewerExportFile(t, out, "notes/topic.html")
	if !strings.Contains(nested, `<meta name="ok-static-head" content="1">`) || !strings.Contains(nested, `<script src="https://openknowledge.sh/analytics.js"></script>`) {
		t.Fatalf("expected nested exported page to include custom head HTML:\n%s", nested)
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
		"--ok-color-focus-ring",
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
	writeViewerFile(t, root, "openknowledge.toml", "[publish]\nenabled = true\n\n[html.theme]\nname = \"landing\"\nstylesheet = \"assets/wiki-theme.css\"\n")
	writeViewerFile(t, root, "assets/wiki-theme.css", ":root { --ok-color-accent: #3257ff; }\n")
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Setup](guides/setup.md).\n")
	writeViewerFile(t, root, "guides/setup.md", "---\ntype: Guide\n---\n\n# Setup\n\nBack to [Home](../index.md).\n")
	writeViewerFile(t, root, "references/report.pdf", "%PDF-1.4\n% test pdf\n")

	handler := newViewerHandler(root)
	page := getViewerBody(t, handler, "/file/index.md")
	if !strings.Contains(page, `data-openknowledge-theme="landing"`) || !strings.Contains(page, `data-viewer-theme="night"`) || !strings.Contains(page, `href="/raw/assets/wiki-theme.css"`) {
		t.Fatalf("viewer should link the configured theme stylesheet from the raw endpoint:\n%s", page)
	}

	asset := getViewerBody(t, handler, "/file/references/report.pdf")
	if !strings.Contains(asset, `data-openknowledge-theme="landing"`) || !strings.Contains(asset, `data-viewer-theme="night"`) || !strings.Contains(asset, `href="/raw/assets/wiki-theme.css"`) {
		t.Fatalf("viewer asset pages should link the configured theme stylesheet from the raw endpoint:\n%s", asset)
	}

	alias := getViewerBody(t, newViewerHandlerWithAlias(root, "project-memory"), "/project-memory/file/index.md")
	if !strings.Contains(alias, `data-openknowledge-theme="landing"`) || !strings.Contains(alias, `data-viewer-theme="night"`) || !strings.Contains(alias, `href="/project-memory/raw/assets/wiki-theme.css"`) {
		t.Fatalf("viewer alias pages should link the prefixed theme stylesheet from the raw endpoint:\n%s", alias)
	}

	listRoot := t.TempDir()
	writeViewerFile(t, listRoot, "openknowledge.toml", "[html.theme]\nname = \"landing\"\nstylesheet = \"assets/wiki-theme.css\"\n")
	writeViewerFile(t, listRoot, "assets/wiki-theme.css", ":root { --ok-color-accent: #3257ff; }\n")
	writeViewerFile(t, listRoot, "notes/readme.md", "---\ntype: Note\n---\n\n# Readme\n")
	listing := getViewerBody(t, newViewerHandler(listRoot), "/")
	if !strings.Contains(listing, `data-openknowledge-theme="landing"`) || !strings.Contains(listing, `data-viewer-theme="night"`) || !strings.Contains(listing, `href="/raw/assets/wiki-theme.css"`) {
		t.Fatalf("viewer index pages should link the configured theme stylesheet from the raw endpoint:\n%s", listing)
	}

	result, err := writeViewerHTMLWithVersion(root, out, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Written) != 10 {
		t.Fatalf("expected exported pages plus theme stylesheet, discovery file, manifest, and archive, got %#v", result.Written)
	}

	index := readViewerExportFile(t, out, "index.html")
	if !strings.Contains(index, `data-openknowledge-theme="landing"`) || !strings.Contains(index, `data-viewer-theme="night"`) || !strings.Contains(index, `href="assets/wiki-theme.css"`) {
		t.Fatalf("expected exported index to link copied theme stylesheet:\n%s", index)
	}
	if !strings.Contains(index, `<a class="brand" href="index.html">Home</a>`) {
		t.Fatalf("static export brand should link to the generated wiki index:\n%s", index)
	}
	if strings.Contains(index, root) || !strings.Contains(index, `data-note-root=""`) {
		t.Fatalf("static export should not expose local bundle root:\n%s", index)
	}
	if strings.Contains(index, `<div class="editor-picker" data-editor-picker>`) || strings.Contains(index, `data-source-open data-direct-link`) {
		t.Fatalf("static export without html.source should not render local editor controls or source buttons:\n%s", index)
	}

	setup := readViewerExportFile(t, out, "guides/setup.html")
	if !strings.Contains(setup, `href="../assets/wiki-theme.css"`) {
		t.Fatalf("expected nested exported page to link theme stylesheet relatively:\n%s", setup)
	}
	if !strings.Contains(setup, `<a class="brand" href="../index.html">Home</a>`) {
		t.Fatalf("nested static export brand should link back to the generated wiki index:\n%s", setup)
	}
	if strings.Contains(setup, root) || !strings.Contains(setup, `data-note-root=""`) {
		t.Fatalf("nested static export should not expose local bundle root:\n%s", setup)
	}

	theme := readViewerExportFile(t, out, "assets/wiki-theme.css")
	if !strings.Contains(theme, `--ok-color-accent: #3257ff`) {
		t.Fatalf("expected export to copy theme stylesheet, got:\n%s", theme)
	}
}

func TestViewerHTMLExportLinksConfiguredGitHubSource(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, "openknowledge.toml", "[publish]\nenabled = true\n\n[html.source]\ngithub_base = \"https://github.com/openknowledge-sh/openknowledge/blob/main\"\nentry = \"Wiki\"\n")
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Setup](guides/setup.md).\n")
	writeViewerFile(t, root, "guides/setup.md", "---\ntype: Guide\n---\n\n# Setup\n")

	if _, err := writeViewerHTMLWithVersion(root, out, "0.1"); err != nil {
		t.Fatal(err)
	}

	index := readViewerExportFile(t, out, "index.html")
	if strings.Contains(index, `<div class="editor-picker" data-editor-picker>`) || strings.Contains(index, `<button class="editor-menu-trigger"`) {
		t.Fatalf("static export should not render the local editor dropdown:\n%s", index)
	}
	if !strings.Contains(index, `class="source-open"`) || !strings.Contains(index, `href="https://github.com/openknowledge-sh/openknowledge/blob/main/Wiki/index.md"`) {
		t.Fatalf("static export should link the current file to GitHub source:\n%s", index)
	}
	if !strings.Contains(index, `"sourceURL":"https://github.com/openknowledge-sh/openknowledge/blob/main/Wiki/guides/setup.md"`) {
		t.Fatalf("static note manifest should include GitHub source URLs for dynamic panels:\n%s", index)
	}

	setup := readViewerExportFile(t, out, "guides/setup.html")
	if !strings.Contains(setup, `href="https://github.com/openknowledge-sh/openknowledge/blob/main/Wiki/guides/setup.md"`) {
		t.Fatalf("nested static export should link its GitHub source:\n%s", setup)
	}
}

func TestViewerHTMLExportWritesDiscoveryFilesWithSiteURL(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, "openknowledge.toml", "[publish]\nenabled = true\n\n[html.site]\nbase_url = \"https://openknowledge.sh/wiki/\"\n")
	writeViewerFile(t, root, "index.md", "---\nokf_bundle_title: \"Team Handbook\"\nokf_bundle_purpose: \"Knowledge for shipping product changes.\"\n---\n\n# Home\n\nRead [Setup](guides/setup.md).\n")
	writeViewerFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: \"Setup Guide\"\n---\n\n# Setup\n")

	result, err := writeViewerHTMLWithVersion(root, out, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	written := strings.Join(result.Written, ",")
	if !strings.Contains(written, "llms.txt") || !strings.Contains(written, "sitemap.xml") {
		t.Fatalf("expected discovery files in written list, got %#v", result.Written)
	}

	llms := readViewerExportFile(t, out, "llms.txt")
	if !strings.Contains(llms, "# Team Handbook") ||
		!strings.Contains(llms, "> Knowledge for shipping product changes.") ||
		!strings.Contains(llms, "- [Home](https://openknowledge.sh/wiki/index.html): index.md") ||
		!strings.Contains(llms, "- [Setup Guide](https://openknowledge.sh/wiki/guides/setup.html): guides/setup.md") {
		t.Fatalf("unexpected llms.txt:\n%s", llms)
	}

	sitemap := readViewerExportFile(t, out, "sitemap.xml")
	if !strings.Contains(sitemap, `<?xml version="1.0" encoding="UTF-8"?>`) ||
		!strings.Contains(sitemap, `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`) ||
		!strings.Contains(sitemap, `<loc>https://openknowledge.sh/wiki/index.html</loc>`) ||
		!strings.Contains(sitemap, `<loc>https://openknowledge.sh/wiki/guides/setup.html</loc>`) {
		t.Fatalf("unexpected sitemap.xml:\n%s", sitemap)
	}
}

func TestViewerSiteConfigRejectsInvalidBaseURL(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "openknowledge.toml", "[publish]\nenabled = true\n\n[html.site]\nbase_url = \"/wiki/\"\n")
	writeViewerFile(t, root, "index.md", "# Home\n")

	_, err := writeViewerHTMLWithVersion(root, filepath.Join(t.TempDir(), "site"), "0.1")
	if err == nil || !strings.Contains(err.Error(), "html.site.base_url must be an http(s) URL") {
		t.Fatalf("expected invalid site base URL to be rejected, got %v", err)
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
	writeViewerFile(t, root, "openknowledge.toml", "[publish]\nenabled = true\n\n[html.theme]\nstylesheet = \"../landing.css\"\n")
	writeViewerFile(t, root, "index.md", "# Home\n")

	if _, err := writeViewerHTMLWithVersion(root, filepath.Join(t.TempDir(), "site"), "0.1"); err == nil || !strings.Contains(err.Error(), "must stay inside the bundle") {
		t.Fatalf("expected outside theme stylesheet to be rejected, got %v", err)
	}
}

func TestViewerThemeConfigRejectsSymbolicLink(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "bundle")
	writeViewerFile(t, root, "openknowledge.toml", "[html.theme]\nstylesheet = \"assets/theme.css\"\n")
	writeViewerFile(t, root, "index.md", "# Home\n")
	outside := filepath.Join(base, "outside.css")
	if err := os.WriteFile(outside, []byte("body { display: none; }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "assets", "theme.css")); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}

	if _, err := writeViewerHTMLWithVersion(root, filepath.Join(base, "site"), "0.1"); err == nil || !strings.Contains(err.Error(), "symbolic links are not supported") {
		t.Fatalf("expected theme symlink to be rejected, got %v", err)
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
	if !strings.Contains(index, `window.OpenKnowledgeShortcuts`) || !strings.Contains(index, `id: "viewer.search.focus"`) {
		t.Fatalf("viewer index fallback should load the shared shortcut registry before search:\n%s", index)
	}
	if startPath := viewerStartPath(root); startPath != "/" {
		t.Fatalf("expected viewer start path to fall back to list, got %q", startPath)
	}
}

func TestViewerSearchAPI(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n\nRead the workflow docs.\n")
	writeViewerFile(t, root, "workflows/docs.md", "---\ntype: Workflow\ntitle: Docs Workflow\ntags: [docs]\n---\n\n# Docs\n\nRun validation before publishing.\n")

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
	if payload.Results[0].HighlightText != "validation" || payload.Results[0].HighlightURL != "/file/workflows/docs.md?ok-highlight=validation" {
		t.Fatalf("expected search result to include highlight deep link, got %#v", payload.Results[0])
	}
	if payload.Results[0].Heading != "Docs" || payload.Results[0].LineStart == 0 || payload.Results[0].LineEnd < payload.Results[0].LineStart || payload.Results[0].Locator == "" || payload.Results[0].ContentSHA256 == "" {
		t.Fatalf("expected canonical section identity and provenance, got %#v", payload.Results[0])
	}
}

func TestViewerSearchMatchesCanonicalKnowledgeRetrieval(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n\nSee [Runbook](workflows/runbook.md).\n")
	writeViewerFile(t, root, "workflows/runbook.md", "---\ntype: Workflow\ntitle: Incident Runbook\n---\n\n# Preparation\n\nCollect ordinary diagnostics.\n\n## Recovery\n\nRun quasarrestore before publishing.\n")
	writeViewerFile(t, root, "notes/related.md", "# Related\n\nBackground material.\n")

	expected, err := okf.SearchKnowledge(root, okf.SearchOptions{Query: "quasarrestore", Limit: 12, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}
	payload := getViewerSearch(t, newViewerHandler(root), "/api/search?q=quasarrestore&limit=12")
	if len(payload.Results) != len(expected.Results) {
		t.Fatalf("viewer returned %d results, canonical retrieval returned %d: viewer=%#v canonical=%#v", len(payload.Results), len(expected.Results), payload.Results, expected.Results)
	}
	for index, canonical := range expected.Results {
		viewer := payload.Results[index]
		if viewer.Path != canonical.Path || viewer.ID != canonical.ID || viewer.Locator != canonical.Locator || viewer.ContentSHA256 != canonical.ContentSHA256 || viewer.Heading != canonical.Heading || viewer.LineStart != canonical.LineStart || viewer.LineEnd != canonical.LineEnd || viewer.Score != canonical.Score || viewer.Neighbor != canonical.Neighbor || viewer.Relation != canonical.Relation || strings.Join(viewer.Matches, "\x00") != strings.Join(canonical.Matches, "\x00") {
			t.Fatalf("viewer result %d diverged from canonical retrieval:\nviewer: %#v\ncanonical: %#v", index, viewer, canonical)
		}
	}
}

func TestViewerSearchReturnsExactTagFacets(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	writeViewerFile(t, root, "notes/current.md", "---\ntype: Note\ntags: [docs]\n---\n\n# Current\n")
	writeViewerFile(t, root, "notes/match.md", "---\ntype: Guide\ntitle: Exact tag match\ndescription: Shares the docs tag.\ntags: [docs, guide]\n---\n\n# Match\n")
	writeViewerFile(t, root, "notes/subset.md", "---\ntype: Note\ntags: [docs-tools]\n---\n\n# Subset\n")
	writeViewerFile(t, root, "notes/body-only.md", "---\ntype: Note\ntags: [other]\n---\n\n# Body only\n\nMentions docs in the body.\n")

	handler := newViewerHandler(root)
	payload := getViewerSearch(t, handler, "/api/search?tag=docs&exclude=notes/current.md&limit=12")
	if payload.Query != "docs" || len(payload.Results) != 1 {
		t.Fatalf("expected one exact docs tag match, got %#v", payload)
	}
	result := payload.Results[0]
	if result.Path != "notes/match.md" || result.Type != "Guide" || result.URL != "/file/notes/match.md" || result.Snippet != "Shares the docs tag." {
		t.Fatalf("unexpected tag facet result: %#v", result)
	}
}

func TestViewerServesDirectAliasPath(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n\nSee [Workflow](workflows/docs.md) and [Report](references/report.pdf).\n")
	writeViewerFile(t, root, "workflows/docs.md", "---\ntype: Workflow\ntitle: Docs Workflow\ntags: [docs]\n---\n\n# Docs\n\nRun validation before publishing.\n")
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
	if !strings.Contains(page, `<a class="brand" href="/project-memory/">Home</a>`) {
		t.Fatalf("viewer file brand should link to the alias root:\n%s", page)
	}
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
	if payload.Results[0].HighlightURL != "/project-memory/file/workflows/docs.md?ok-highlight=validation" {
		t.Fatalf("unexpected prefixed highlight URL: %#v", payload.Results[0])
	}
	tagPayload := getViewerSearch(t, handler, "/project-memory/api/search?tag=docs")
	if len(tagPayload.Results) != 1 || tagPayload.Results[0].URL != "/project-memory/file/workflows/docs.md" {
		t.Fatalf("unexpected prefixed tag facet result: %#v", tagPayload)
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

	writeViewerFile(t, root, "notes/draft.md", "---\ntype: Note\ntitle: Draft Note\ntags: [draft]\n---\n\n# Draft\n\nFresh searchable content.\n")
	second := getViewerSearch(t, handler, "/api/search?q=draft")
	if len(second.Results) == 0 || second.Results[0].Path != "notes/draft.md" {
		t.Fatalf("expected refreshed search result, got %#v", second)
	}
	tagged := getViewerSearch(t, handler, "/api/search?tag=draft")
	if len(tagged.Results) != 1 || tagged.Results[0].Path != "notes/draft.md" {
		t.Fatalf("expected refreshed tag facet, got %#v", tagged)
	}
}

func TestViewerSearchRefreshesWhenSizeAndMtimeAreUnchanged(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	path := filepath.Join(root, "notes", "mutable.md")
	before := "---\ntype: Note\ntitle: Mutable\n---\n\n# Mutable\n\nOriginal alphaunique evidence.\n"
	after := "---\ntype: Note\ntitle: Mutable\n---\n\n# Mutable\n\nUpdated! bravounique evidence.\n"
	if len(before) != len(after) {
		t.Fatal("test fixture contents must have identical size")
	}
	writeViewerFile(t, root, "notes/mutable.md", before)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	handler := newViewerHandler(root)
	initial := getViewerSearch(t, handler, "/api/search?q=alphaunique")
	if len(initial.Results) != 1 || initial.Results[0].Path != "notes/mutable.md" {
		t.Fatalf("expected initial cached result, got %#v", initial)
	}
	if err := os.WriteFile(path, []byte(after), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	changedInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if changedInfo.Size() != info.Size() || !changedInfo.ModTime().Equal(info.ModTime()) {
		t.Fatalf("fixture must preserve size and mtime: before=%#v after=%#v", info, changedInfo)
	}

	refreshed := getViewerSearch(t, handler, "/api/search?q=bravounique")
	if len(refreshed.Results) != 1 || refreshed.Results[0].Path != "notes/mutable.md" {
		t.Fatalf("expected content fingerprint to refresh search, got %#v", refreshed)
	}
	stale := getViewerSearch(t, handler, "/api/search?q=alphaunique")
	if len(stale.Results) != 0 {
		t.Fatalf("expected stale term to leave the rebuilt index, got %#v", stale)
	}
}

func TestViewerRejectsTraversalAndNonMarkdownAPI(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	writeViewerFile(t, root, "notes.txt", "not markdown\n")
	writeViewerFile(t, root, ".env", "TOKEN=secret\n")
	writeViewerFile(t, root, "openknowledge.toml", "[html.theme]\nname = \"night\"\n")
	writeViewerFile(t, root, ".git/config", "[remote \"origin\"]\nurl = secret\n")
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
	if err := os.Symlink(outside, filepath.Join(root, "linked.md")); err == nil {
		if _, ok := safeMarkdownPath(root, "linked.md"); ok {
			t.Fatal("expected Markdown symbolic link to be rejected")
		}
		if _, ok := safeViewerPath(root, "linked.md"); ok {
			t.Fatal("expected asset symbolic link to be rejected")
		}
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/file/notes.txt", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected non-markdown file API to return 404, got %d", recorder.Code)
	}

	for _, rawPath := range []string{"index.md", ".env", ".git/config", "openknowledge.toml", "missing.txt"} {
		recorder = httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/raw/"+rawPath, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected private or non-asset raw path %s to return 404, got %d", rawPath, recorder.Code)
		}
	}
	for _, assetPath := range []string{".env", ".git/config", "openknowledge.toml"} {
		recorder = httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/file/"+assetPath, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected private asset page %s to return 404, got %d", assetPath, recorder.Code)
		}
	}
	indexRecorder := httptest.NewRecorder()
	handler.ServeHTTP(indexRecorder, httptest.NewRequest(http.MethodGet, "/file/index.md", nil))
	for _, privateName := range []string{".env", "openknowledge.toml", ".git"} {
		if strings.Contains(indexRecorder.Body.String(), privateName) {
			t.Fatalf("expected private asset %s to be absent from viewer tree", privateName)
		}
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/raw/notes.txt", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "not markdown\n" {
		t.Fatalf("expected inventory asset to remain available, code=%d body=%q", recorder.Code, recorder.Body.String())
	}
	for header, expected := range map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"Content-Security-Policy": "default-src 'none'; sandbox",
	} {
		if actual := recorder.Header().Get(header); actual != expected {
			t.Fatalf("expected %s=%q, got %q", header, expected, actual)
		}
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/raw/notes.txt", nil))
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected raw POST to return 405 with Allow, code=%d allow=%q", recorder.Code, recorder.Header().Get("Allow"))
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
		{Name: "personal", Path: personal, Access: "read"},
		{Name: "work", Path: work, Access: "write"},
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
	if !strings.Contains(workPage, `<div class="editor-picker" data-editor-picker>`) {
		t.Fatalf("writable registry connection should expose editor controls:\n%s", workPage)
	}
	personalPage := getViewerBody(t, handler, "/kb/personal/file/index.md")
	if strings.Contains(personalPage, `<div class="editor-picker" data-editor-picker>`) || strings.Contains(personalPage, `<a class="editor-open"`) {
		t.Fatalf("read-only registry connection must hide editor controls:\n%s", personalPage)
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

func TestReloadingRegistryViewerTracksConnectionChanges(t *testing.T) {
	first := t.TempDir()
	replacement := t.TempDir()
	second := t.TempDir()
	writeViewerFile(t, first, "index.md", "# First Generation\n")
	writeViewerFile(t, replacement, "index.md", "# Refreshed Generation\n")
	writeViewerFile(t, second, "index.md", "# Second Knowledge Base\n")

	entries := []okf.RegistryEntry{{Name: "docs", Path: first, Access: "read"}}
	var loadErr error
	handler := newReloadingRegistryViewerHandlerWithOptions(func() ([]okf.RegistryEntry, error) {
		return append([]okf.RegistryEntry(nil), entries...), loadErr
	}, viewerOptions{})

	initial := getViewerBody(t, handler, "/kb/docs/file/index.md")
	if !strings.Contains(initial, "First Generation") || strings.Contains(initial, `<div class="editor-picker" data-editor-picker>`) {
		t.Fatalf("unexpected initial read-only generation:\n%s", initial)
	}

	entries = []okf.RegistryEntry{
		{Name: "docs", Path: replacement, Access: "write"},
		{Name: "second", Path: second, Access: "read"},
	}
	refreshed := getViewerBody(t, handler, "/kb/docs/file/index.md")
	if !strings.Contains(refreshed, "Refreshed Generation") || !strings.Contains(refreshed, `<div class="editor-picker" data-editor-picker>`) {
		t.Fatalf("viewer did not reload path and access changes:\n%s", refreshed)
	}
	index := getViewerBody(t, handler, "/")
	if !strings.Contains(index, `href="/kb/second/"`) {
		t.Fatalf("viewer did not load a newly connected knowledge base:\n%s", index)
	}

	entries = entries[1:]
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/kb/docs/", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("viewer retained a disconnected route, got %d", recorder.Code)
	}

	loadErr = errors.New("registry unavailable")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), "registry unavailable") {
		t.Fatalf("viewer did not surface registry reload failure, code=%d body=%q", recorder.Code, recorder.Body.String())
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

func TestViewerNetworkAccessRequiresExplicitAuthenticatedOptIn(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "localhost", "[::1]"} {
		if !viewerHostIsLoopback(host) {
			t.Fatalf("expected %s to be loopback", host)
		}
		token, err := viewerAccessToken(host, false, "")
		if err != nil || token != "" {
			t.Fatalf("expected unauthenticated loopback default for %s, token=%q err=%v", host, token, err)
		}
	}
	for _, host := range []string{"0.0.0.0", "::", "192.0.2.10", "example.test", ""} {
		if viewerHostIsLoopback(host) {
			t.Fatalf("did not expect %s to be loopback", host)
		}
		if _, err := viewerAccessToken(host, false, ""); err == nil || !strings.Contains(err.Error(), "--allow-network") {
			t.Fatalf("expected non-loopback refusal for %q, got %v", host, err)
		}
		token, err := viewerAccessToken(host, true, "")
		if err != nil || validateViewerToken(token) != nil {
			t.Fatalf("expected generated network token for %q, token=%q err=%v", host, token, err)
		}
	}
	configured := "test-token-1234567890"
	if token, err := viewerAccessToken("0.0.0.0", true, configured); err != nil || token != configured {
		t.Fatalf("expected configured token, token=%q err=%v", token, err)
	}
	for _, invalid := range []string{"short", "contains spaces 12345", strings.Repeat("a", 257)} {
		if _, err := viewerAccessToken("127.0.0.1", false, invalid); err == nil {
			t.Fatalf("expected invalid token %q to be rejected", invalid)
		}
	}
}

func TestRunViewRefusesUnauthenticatedNetworkBindBeforeListening(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	_, stderr, code := captureMainOutput(t, func() int {
		return runView([]string{"--host", "0.0.0.0", "--no-browser", root})
	})
	if code != 2 || !strings.Contains(stderr, "--allow-network") {
		t.Fatalf("expected command-level network refusal, code=%d stderr=%q", code, stderr)
	}
	_, stderr, code = captureMainOutput(t, func() int {
		return runView([]string{"--token", "short", "--no-browser", root})
	})
	if code != 2 || !strings.Contains(stderr, "16 to 256") {
		t.Fatalf("expected command-level weak token refusal, code=%d stderr=%q", code, stderr)
	}
}

func TestSecureViewerHandlerAuthenticatesEveryRoute(t *testing.T) {
	const token = "test-token-1234567890"
	next := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("ok"))
	})
	handler := secureViewerHandler(next, token)

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/search", nil))
	if unauthorized.Code != http.StatusUnauthorized || unauthorized.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("expected bearer challenge, code=%d headers=%v", unauthorized.Code, unauthorized.Header())
	}
	for header, expected := range map[string]string{
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"X-Frame-Options":        "DENY",
	} {
		if actual := unauthorized.Header().Get(header); actual != expected {
			t.Fatalf("expected %s=%q, got %q", header, expected, actual)
		}
	}

	bearer := httptest.NewRecorder()
	bearerRequest := httptest.NewRequest(http.MethodGet, "/raw/asset.pdf", nil)
	bearerRequest.Header.Set("Authorization", "bearer "+token)
	handler.ServeHTTP(bearer, bearerRequest)
	if bearer.Code != http.StatusOK || bearer.Body.String() != "ok" {
		t.Fatalf("expected bearer access, code=%d body=%q", bearer.Code, bearer.Body.String())
	}

	bootstrap := httptest.NewRecorder()
	handler.ServeHTTP(bootstrap, httptest.NewRequest(http.MethodGet, "/wiki/?view=one&token="+token, nil))
	if bootstrap.Code != http.StatusSeeOther || bootstrap.Header().Get("Location") != "/wiki/?view=one" {
		t.Fatalf("expected token-stripping redirect, code=%d location=%q", bootstrap.Code, bootstrap.Header().Get("Location"))
	}
	cookies := bootstrap.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != viewerTokenCookie || cookies[0].Value != token || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected viewer session cookie: %#v", cookies)
	}

	cookieResponse := httptest.NewRecorder()
	cookieRequest := httptest.NewRequest(http.MethodGet, "/file/index.md", nil)
	cookieRequest.AddCookie(cookies[0])
	handler.ServeHTTP(cookieResponse, cookieRequest)
	if cookieResponse.Code != http.StatusOK {
		t.Fatalf("expected authenticated cookie access, got %d", cookieResponse.Code)
	}

	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/?token=wrong-token-123456", nil))
	if invalid.Code != http.StatusUnauthorized || invalid.Header().Get("Set-Cookie") != "" {
		t.Fatalf("expected invalid query token refusal, code=%d headers=%v", invalid.Code, invalid.Header())
	}
}

func TestViewerURLWithTokenPreservesAliasPath(t *testing.T) {
	actual := viewerURLWithToken("http://127.0.0.1:57475/wiki/", "test-token-1234567890")
	if actual != "http://127.0.0.1:57475/wiki/?token=test-token-1234567890" {
		t.Fatalf("unexpected authenticated viewer URL: %s", actual)
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

func TestViewerInjectsHeadHTMLWhenConfigured(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "docs/page.md", "# Page\n")

	defaultBody := getViewerBody(t, newViewerHandler(root), "/")
	if strings.Contains(defaultBody, "ok-head-test") {
		t.Fatalf("viewer should not include custom head HTML by default:\n%s", defaultBody)
	}

	headHTML, err := loadHeadInjection(headInjectionOptions{
		HTML:       `<meta name="ok-head-test" content="inline">`,
		ScriptSrcs: []string{"/analytics.js"},
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := newViewerHandlerWithOptions(root, viewerOptions{HeadHTML: headHTML})
	index := getViewerBody(t, handler, "/")
	if !strings.Contains(index, `<meta name="ok-head-test" content="inline">`) {
		t.Fatalf("viewer index did not include custom head HTML:\n%s", index)
	}
	if !strings.Contains(index, `<script src="/analytics.js"></script>`) {
		t.Fatalf("viewer index did not include script src:\n%s", index)
	}

	page := getViewerBody(t, handler, "/file/docs/page.md")
	if !strings.Contains(page, `<meta name="ok-head-test" content="inline">`) {
		t.Fatalf("viewer page did not include custom head HTML:\n%s", page)
	}
}

func TestLoadHeadInjectionReadsFragmentFile(t *testing.T) {
	root := t.TempDir()
	headFile := filepath.Join(root, "head.html")
	if err := os.WriteFile(headFile, []byte(`<meta name="ok-head-file" content="1">`), 0644); err != nil {
		t.Fatal(err)
	}

	headHTML, err := loadHeadInjection(headInjectionOptions{File: headFile})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(headHTML), `name="ok-head-file"`) {
		t.Fatalf("expected head file content to be included, got %s", headHTML)
	}
}

func TestLoadHeadInjectionRejectsUnsupportedScriptScheme(t *testing.T) {
	_, err := loadHeadInjection(headInjectionOptions{
		ScriptSrcs: []string{"javascript:alert(1)"},
	})
	if err == nil {
		t.Fatal("expected unsupported script scheme to be rejected")
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
