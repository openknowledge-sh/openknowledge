package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	page += viewerSourceForAssertions(t)
	page += getViewerBody(t, handler, "/"+viewerThemeScriptAsset)
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
	if !strings.Contains(page, `data-openknowledge-theme="default"`) ||
		!strings.Contains(page, `data-viewer-theme="default"`) ||
		!strings.Contains(page, `const defaultPreset = "default";`) ||
		!strings.Contains(page, `const defaultThemePreset = "default";`) {
		t.Fatalf("viewer file page should expose theme data:\n%s", page)
	}
	if !strings.Contains(page, `class="search-icon control-icon"`) {
		t.Fatalf("viewer header search should include its search icon:\n%s", page)
	}
	if !strings.Contains(page, `data-horizontal-stack checked`) ||
		!strings.Contains(page, `<strong>Horizontal stack</strong>`) ||
		strings.Contains(page, `data-navigation-mode-toggle`) {
		t.Fatalf("viewer should render horizontal stack as an enabled Document setting instead of a header control:\n%s", page)
	}
	if !strings.Contains(page, `data-graph-view-toggle`) ||
		!strings.Contains(page, `data-documents-view-toggle`) ||
		!strings.Contains(page, `data-sidebar-graph-slot`) ||
		!strings.Contains(page, `aria-label="Graph view"`) ||
		!strings.Contains(page, `aria-controls="knowledge-graph" aria-pressed="false"`) ||
		!strings.Contains(page, `id="knowledge-graph"`) {
		t.Fatalf("viewer should render separate Documents and Graph sidebar items:\n%s", page)
	}
	if !strings.Contains(page, `setGraphViewRequested(!graphViewRequested)`) ||
		!strings.Contains(page, `function bindDocumentsView()`) ||
		!strings.Contains(page, `graphViewToggle.setAttribute("aria-pressed", showGraph ? "true" : "false")`) ||
		!strings.Contains(page, `settingsSummary.textContent = "Graph settings"`) ||
		!strings.Contains(page, `mobileSidebar.addEventListener("change", syncSettingsDisclosure)`) ||
		!strings.Contains(page, `settingsDisclosure.open = !mobile`) ||
		!strings.Contains(page, `summary.setAttribute("role", "heading")`) {
		t.Fatalf("viewer sidebar views should preserve panels and collapse all graph settings on mobile:\n%s", page)
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
		!strings.Contains(page, `applyFrontmatterPreference`) {
		t.Fatalf("viewer should persist a global frontmatter visibility preference:\n%s", page)
	}
	if !strings.Contains(page, `function integratePanelFrontmatter(panel)`) ||
		!strings.Contains(page, `chrome.after(frontmatter)`) ||
		!strings.Contains(page, `frontmatter.classList.add("is-panel-integrated")`) ||
		strings.Contains(page, `actions.prepend(trigger)`) ||
		strings.Contains(page, `summary.hidden = true`) {
		t.Fatalf("viewer note panels should keep collapsed, full-width frontmatter below the compact header:\n%s", page)
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
		!strings.Contains(page, `dataset.viewerUnderlines = normalized.underlineLinks ? "on" : "off"`) {
		t.Fatalf("viewer should persist and apply reading and accessibility preferences:\n%s", page)
	}
	if !strings.Contains(page, `data-settings-reset`) ||
		!strings.Contains(page, `Reset to defaults`) ||
		!strings.Contains(page, `preference = normalizeThemePreference({ preset: defaultThemePreset, custom: customThemeDefaults })`) ||
		!strings.Contains(page, `accessibilityPreference = normalizeAccessibilityPreference(defaultAccessibilityPreference)`) ||
		!strings.Contains(page, `saveNavigationModePreference(navigationMode)`) {
		t.Fatalf("viewer settings should expose a persistent reset-to-defaults action:\n%s", page)
	}
	if !strings.Contains(page, `openknowledge.viewer.navigationMode`) ||
		!strings.Contains(page, `function shouldOpenBeside(shiftKey)`) ||
		!strings.Contains(page, `return shiftKey ? !besideByDefault : besideByDefault`) ||
		!strings.Contains(page, `horizontalStack.checked = navigationMode === "beside"`) {
		t.Fatalf("viewer should persist and apply replace or open-beside link behavior:\n%s", page)
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
	if !strings.Contains(page, `Clear filters`) || !strings.Contains(page, `function enhanceTables(scope)`) || !strings.Contains(page, `bindSortableTableHeader`) || !strings.Contains(page, `Filter table`) {
		t.Fatalf("viewer should embed rich table runtime controls:\n%s", page)
	}
	if !strings.Contains(page, `data-note-workspace`) || !strings.Contains(page, `data-note-path="index.md"`) {
		t.Fatalf("viewer file page did not include stacked note layout:\n%s", page)
	}
	if !strings.Contains(page, `is-single-panel`) {
		t.Fatalf("viewer should identify a lone open panel:\n%s", page)
	}
	if !strings.Contains(page, `minPanelWidth`) {
		t.Fatalf("viewer panels should expose a resizable width with a minimum width:\n%s", page)
	}
	if !strings.Contains(page, `data-panel-resize-handle`) || !strings.Contains(page, `note-resize-handle-left`) || !strings.Contains(page, `note-resize-handle-right`) {
		t.Fatalf("viewer panels should include left and right resize handles:\n%s", page)
	}
	if !strings.Contains(page, `syncPanelResizeHandles(panel)`) {
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
	if strings.Contains(page, `data-note-navigator`) ||
		strings.Contains(page, `function updateNoteNavigator()`) ||
		strings.Contains(page, `.workspace-note-nav`) {
		t.Fatalf("viewer should not duplicate open panels in a fixed bottom navigator:\n%s", page)
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
	if !strings.Contains(page, `class="viewer-document is-stack-mode"`) {
		t.Fatalf("viewer file page should start in stack panel mode:\n%s", page)
	}
	if !strings.Contains(page, `note-panel is-active-panel`) {
		t.Fatalf("viewer file page did not limit editor picker to the active panel:\n%s", page)
	}
	if !strings.Contains(page, `data-note-root="`) {
		t.Fatalf("viewer file page did not expose note root for editor deeplinks:\n%s", page)
	}
	if !strings.Contains(page, `data-close-panel`) {
		t.Fatalf("viewer file page did not include panel close control:\n%s", page)
	}
	if !strings.Contains(page, `data-note-narration`) ||
		!strings.Contains(page, `data-note-narration-stop`) ||
		!strings.Contains(page, `function narrationText(panel)`) ||
		!strings.Contains(page, `window.speechSynthesis.speak(utterance)`) {
		t.Fatalf("viewer file page should expose browser-native note narration controls:\n%s", page)
	}
	if !strings.Contains(page, `data-note-breadcrumbs data-note-path-value="index.md"`) ||
		!strings.Contains(page, `function createNoteBreadcrumbs(path, knowledgeBase)`) ||
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
	if !strings.Contains(page, `data-sidebar-toggle`) || !strings.Contains(page, `data-file-sidebar`) || !strings.Contains(page, `aria-label="File explorer"`) ||
		!strings.Contains(page, `class="file-sidebar-navigation"`) || !strings.Contains(page, `data-sidebar-settings-slot`) ||
		strings.Contains(page, `data-sidebar-close`) || strings.Contains(page, `file-sidebar-head`) || strings.Contains(page, `file-sidebar-brand`) {
		t.Fatalf("viewer file page did not include file explorer sidebar controls:\n%s", page)
	}
	if !strings.Contains(page, `id="viewer-search"`) || !strings.Contains(page, `data-primary-search`) || !strings.Contains(page, `data-search-url="/api/search"`) || !strings.Contains(page, `searchStaticNotes`) {
		t.Fatalf("viewer file page did not include top bar search:\n%s", page)
	}
	if !strings.Contains(page, `function groupSearchResults(items)`) ||
		!strings.Contains(page, `group.matchCount += 1`) ||
		!strings.Contains(page, `search-result-count`) {
		t.Fatalf("viewer search should group section matches into one result per document:\n%s", page)
	}
	if strings.Contains(page, `id="viewer-sidebar-search"`) || strings.Contains(page, `file-sidebar-search`) {
		t.Fatalf("viewer file sidebar should not include search:\n%s", page)
	}
	if !strings.Contains(page, `renderDefaultResults(true)`) || !strings.Contains(page, `defaultSearchResults()`) || !strings.Contains(page, `Top files`) {
		t.Fatalf("viewer search dropdown should stay open on focus with default results for an empty query:\n%s", page)
	}
	if !strings.Contains(page, `document.addEventListener("pointerdown"`) ||
		!strings.Contains(page, `!results.hidden && !search.contains(event.target)`) ||
		!strings.Contains(page, `document.addEventListener("focusin"`) {
		t.Fatalf("viewer search dropdown should dismiss on outside pointer and focus:\n%s", page)
	}
	if !strings.Contains(page, `search-result-title`) ||
		!strings.Contains(page, `search-result-snippet`) ||
		strings.Contains(page, `search-result-context`) ||
		strings.Contains(page, `Arrow keys navigate`) {
		t.Fatalf("viewer header search should use a streamlined result surface with clear hierarchy:\n%s", page)
	}
	if !strings.Contains(page, `function plainSearchExcerpt(value)`) ||
		!strings.Contains(page, `function searchResultTitle(item)`) ||
		!strings.Contains(page, `const showPath = item.path && String(item.path).toLocaleLowerCase() !== displayTitle.toLocaleLowerCase()`) ||
		!strings.Contains(page, `return parent + " / Index"`) ||
		!strings.Contains(page, `const keepOpenWhenEmpty = config.keepOpenWhenEmpty !== false`) ||
		!strings.Contains(page, `"Search unavailable."`) ||
		!strings.Contains(page, `label: "Retry"`) {
		t.Fatalf("viewer search should show readable results, visible recovery, and a viewport-safe mobile panel:\n%s", page)
	}
	if !strings.Contains(page, `isIndexMarkdownPath(path) ? baseScore * 0.55 : baseScore`) || !strings.Contains(page, `isIndexMarkdownPath(a.path) ? 1 : -1`) {
		t.Fatalf("viewer static search should rank index.md files below regular pages:\n%s", page)
	}
	if !strings.Contains(page, `renderResults(results, status, payload.results || [], query`) ||
		!strings.Contains(page, `renderSearchState(results, status, "Searching…", "loading"`) ||
		strings.Contains(page, `setResultsOpen(false);\n\n      if (staticNotes.length > 0)`) {
		t.Fatalf("viewer search should keep the dropdown open while typed queries are pending:\n%s", page)
	}
	if !strings.Contains(page, `initializeSearchAccessibility`) || !strings.Contains(page, `event.key === "ArrowDown"`) || !strings.Contains(page, `selectedSearchResult(results, activeIndex)`) || !strings.Contains(page, `aria-activedescendant`) {
		t.Fatalf("viewer search should expose combobox keyboard navigation:\n%s", page)
	}
	if !strings.Contains(page, `link.dataset.openBeside = event.shiftKey ? "true" : "false"`) ||
		!strings.Contains(page, `Links open beside. Hold Shift to replace the current panel.`) {
		t.Fatalf("viewer search should follow the selected link behavior and its Shift override:\n%s", page)
	}
	if !strings.Contains(page, `results.addEventListener("click"`) || !strings.Contains(page, `closeSearch(true)`) {
		t.Fatalf("viewer search dropdown should close on result activation:\n%s", page)
	}
	if !strings.Contains(page, `item.highlightURL || item.url`) ||
		!strings.Contains(page, `ok-highlight`) ||
		!strings.Contains(page, `applySearchHighlight`) ||
		!strings.Contains(page, `mark.ok-search-highlight`) {
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
	if !strings.Contains(page, `const mobileSidebar = window.matchMedia("(max-width: 720px)")`) ||
		!strings.Contains(page, "if (mobileSidebar.matches) {\n        setSidebarOpen(false);\n      }") {
		t.Fatalf("viewer file sidebar should close after opening a tree item only on mobile widths:\n%s", page)
	}
	if !strings.Contains(page, `data-sidebar-resize-handle`) ||
		!strings.Contains(page, `role="separator"`) ||
		!strings.Contains(page, `openknowledge.viewer.sidebarWidth.`) ||
		!strings.Contains(page, `fileSidebar.inert = !open;`) ||
		!strings.Contains(page, `resizeSidebarWithKeyboard`) {
		t.Fatalf("viewer file sidebar should be resizable, persistent, bounded, and inert while closed:\n%s", page)
	}
	if !strings.Contains(page, `id: "viewer.sidebar.toggle"`) ||
		!strings.Contains(page, `code: "KeyS"`) ||
		!strings.Contains(page, `metaOrCtrlKey: true`) ||
		!strings.Contains(page, `label: "⌘⌥S"`) ||
		!strings.Contains(page, `sidebarToggle.setAttribute("aria-keyshortcuts"`) {
		t.Fatalf("viewer file sidebar should register a primary-alt-s keyboard shortcut:\n%s", page)
	}
	if !strings.Contains(page, `<button class="sidebar-shortcut" data-sidebar-shortcut type="button" data-sidebar-toggle`) ||
		!strings.Contains(page, `document.querySelectorAll("[data-sidebar-shortcut]")`) ||
		!strings.Contains(page, `sidebarToggle.setAttribute("aria-label", open ? "Close file explorer" : "Open file explorer")`) ||
		strings.Contains(page, `sidebar-toggle-icon`) {
		t.Fatalf("viewer file sidebar should use the shortcut badge as its accessible toggle button:\n%s", page)
	}
	if !strings.Contains(page, `document.startViewTransition`) {
		t.Fatalf("viewer stack changes should use View Transitions when available:\n%s", page)
	}
	if !strings.Contains(page, `return !mobileSidebar.matches && !motionIsReduced();`) ||
		!strings.Contains(page, `animate && stackMotionIsEnabled() ? " is-entering" : ""`) {
		t.Fatalf("viewer navigation and sidebar motion should be disabled at mobile widths:\n%s", page)
	}
	if !strings.Contains(page, `clearEnteringPanels();`) || !strings.Contains(page, `document.body.classList.remove("is-view-transitioning")`) || !strings.Contains(page, `transition.updateCallbackDone`) {
		t.Fatalf("viewer stack transitions should clear fallback panel animations before showing the live DOM:\n%s", page)
	}
	if !strings.Contains(page, `data-empty-state aria-label="Knowledge graph"`) ||
		!strings.Contains(page, `data-knowledge-graph-sidebar`) ||
		!strings.Contains(page, `data-tree-path="workflows/docs.md"`) ||
		!strings.Contains(page, `tree-directory`) ||
		strings.Contains(page, `knowledge-empty-tree`) {
		t.Fatalf("viewer should keep the file tree in the explorer and graph details in the empty state:\n%s", page)
	}
	if !strings.Contains(page, `function prepareKnowledgeTrees()`) ||
		!strings.Contains(page, `function collapseKnowledgeTrees()`) ||
		!strings.Contains(page, `dataset.treeDirectoryPath`) ||
		!strings.Contains(page, `link.setAttribute("aria-current", "page")`) ||
		!strings.Contains(page, `data-sidebar-collapse aria-label="Collapse all"`) ||
		!strings.Contains(page, `collapse.addEventListener("click", collapseKnowledgeTrees)`) {
		t.Fatalf("viewer file tree should collapse directories and reveal the active file context:\n%s", page)
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
	if !strings.Contains(page, `createKnowledgeGraphCanvas`) || !strings.Contains(page, `graphCanvasPhysicsStep`) || !strings.Contains(page, `drawKnowledgeGraphCanvas`) || !strings.Contains(page, `graphCanvasHitTest`) || !strings.Contains(page, `requestAnimationFrame(tick)`) || !strings.Contains(page, `dataset.knowledgeGraphCanvas`) {
		t.Fatalf("viewer knowledge graph should render as an animated canvas graph:\n%s", page)
	}
	if !strings.Contains(page, `dataset.activeGraphPath`) || !strings.Contains(page, `graphNodeFullLabel`) || !strings.Contains(page, `const connected = active && (edge.source === active.path || edge.target === active.path)`) || !strings.Contains(page, `openGraphNode(stateByPath[activePath].node)`) {
		t.Fatalf("viewer canvas graph should separate hovered nodes and highlight connected edges:\n%s", page)
	}
	if !strings.Contains(page, `knowledge-graph-status`) ||
		!strings.Contains(page, `status.setAttribute("aria-live", "polite")`) ||
		!strings.Contains(page, `status.textContent = connectionLabel`) ||
		!strings.Contains(page, `context.fillStyle = activeNode && !settingsValue.colorGroups ? theme.nodeActive : nodeColor`) ||
		!strings.Contains(page, `function graphCanvasTheme()`) {
		t.Fatalf("viewer graph should show concise connection context and preserve folder colors:\n%s", page)
	}
	if !strings.Contains(page, `graphEaseInOut`) || !strings.Contains(page, `graphLimitVelocity`) || !strings.Contains(page, `context.globalAlpha = 1`) {
		t.Fatalf("viewer canvas graph should damp hover physics without dimming inactive nodes:\n%s", page)
	}
	if strings.Contains(page, `context.shadowBlur`) || strings.Contains(page, `context.strokeText(label`) {
		t.Fatalf("viewer canvas graph hover should avoid node shadows and label halo text:\n%s", page)
	}
	if !strings.Contains(page, `graphUniqueNodeLabels`) || !strings.Contains(page, `graphShortestUniquePathSuffix`) || !strings.Contains(page, `parts.slice(-2).join("/")`) {
		t.Fatalf("viewer knowledge graph should disambiguate generic node labels with path suffixes:\n%s", page)
	}
	if !strings.Contains(page, `renderKnowledgeGraph()`) {
		t.Fatalf("viewer empty state should render the knowledge graph:\n%s", page)
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

func TestViewerRendersMermaidBlocksWithLocalRuntime(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", strings.Join([]string{
		"# Architecture",
		"",
		"```mermaid",
		"graph TD",
		`  Input["<unsafe>"] --> Output`,
		"```",
		"",
	}, "\n"))

	handler := newViewerHandler(root)
	page := getViewerBody(t, handler, "/file/index.md")
	for _, expected := range []string{
		`data-language="mermaid" data-mermaid-source`,
		`&lt;unsafe&gt;`,
		`src="/assets/openknowledge/viewer.js"`,
	} {
		if !strings.Contains(page, expected) {
			t.Fatalf("viewer Mermaid integration missing %q:\n%s", expected, page)
		}
	}
	if strings.Contains(page, "<unsafe>") {
		t.Fatalf("viewer Mermaid source must remain escaped before browser rendering:\n%s", page)
	}

	api := getViewerJSON(t, handler, "/api/file/index.md")
	if !strings.Contains(api.Body, `data-language="mermaid" data-mermaid-source`) || !strings.Contains(api.Body, `&lt;unsafe&gt;`) {
		t.Fatalf("viewer API should preserve safe Mermaid source markup: %#v", api)
	}

	request := httptest.NewRequest(http.MethodGet, "/assets/openknowledge/viewer.js", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK ||
		!strings.Contains(response.Header().Get("Content-Type"), "javascript") ||
		!strings.Contains(response.Body.String(), `securityLevel:"strict"`) {
		t.Fatalf("unexpected bundled viewer response %d %q", response.Code, response.Header().Get("Content-Type"))
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

func TestViewerRendersOKFV02KnowledgeSignalsAndContracts(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n")
	writeViewerFile(t, root, "revenue.md", `---
type: Attested Computation
title: Revenue
runtime: python3
parameters:
  - { name: year, type: integer, required: true }
computation: https://example.test/revenue.py
executor: { resource: https://example.test/executor, receipt: [stdout, sha256] }
attester: { resource: https://example.test/attester }
generated: { by: process:nightly, at: 2026-08-01T09:00:00Z }
verified: { by: human:reviewer, at: 2026-08-03T09:00:00Z }
status: deprecated
stale_after: 2026-08-04T00:00:00Z
sources:
  - id: revenue-policy
    resource: https://example.test/policy
    title: Revenue policy
    author: team:finance
    usage_count: 7
---

# Revenue

Supported by policy.[^revenue-policy]

[^revenue-policy]: Revenue policy
`)

	handler := newViewerHandler(root)
	page := getViewerBody(t, handler, "/file/revenue.md")
	for _, expected := range []string{
		`data-okf02-signals`,
		`<dt class="ok-signal-label">Trust</dt>`,
		`data-signal-kind="trust" data-signal-value="human-reviewed"`,
		`Human reviewed`,
		`<dt class="ok-signal-label">Status</dt>`,
		`data-signal-value="deprecated"`,
		`<dt class="ok-signal-label">Freshness</dt>`,
		`data-signal-value="stale"`,
		`data-source-ledger`,
		`id="ok-source-revenue-policy"`,
		`href="https://example.test/policy"`,
		`href="#ok-source-revenue-policy"`,
		`data-computation-contract`,
		`python3`,
		`https://example.test/executor`,
		`https://example.test/attester`,
		`Execution is never automatic.`,
	} {
		if !strings.Contains(page, expected) {
			t.Fatalf("viewer OKF 0.2 surface missing %q:\n%s", expected, page)
		}
	}
	if strings.Contains(page, `data-signal-default="true"`) {
		t.Fatalf("viewer should not mark explicit OKF 0.2 signals as defaults:\n%s", page)
	}
	if strings.Contains(page, "Revenue policy</p>") {
		t.Fatalf("viewer should resolve the source footnote through structured metadata:\n%s", page)
	}

	api := getViewerJSON(t, handler, "/api/file/revenue.md")
	if !strings.Contains(api.Frontmatter, `data-okf02-signals`) || !strings.Contains(api.Body, `href="#ok-source-revenue-policy"`) {
		t.Fatalf("dynamic viewer panels should retain OKF 0.2 signals and source references: %#v", api)
	}
	if !strings.HasPrefix(api.Frontmatter, `<details class="ok-frontmatter" data-frontmatter>`) ||
		!strings.Contains(api.Frontmatter, `<div class="ok-frontmatter-body"><section class="ok-knowledge-signals"`) {
		t.Fatalf("viewer should keep OKF 0.2 signals inside the frontmatter disclosure: %s", api.Frontmatter)
	}
}

func TestViewerLabelsInferredOKFV02SignalDefaults(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n")
	writeViewerFile(t, root, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n")

	page := getViewerBody(t, newViewerHandler(root), "/file/guide.md")
	for _, expected := range []string{
		`<dt class="ok-signal-label">Trust</dt>`,
		`data-signal-kind="trust" data-signal-value="unverified">Unverified</span><span class="ok-signal-default" data-signal-default="true">Default</span>`,
		`<dt class="ok-signal-label">Status</dt>`,
		`data-signal-kind="status" data-signal-value="stable">Stable</span><span class="ok-signal-default" data-signal-default="true">Default</span>`,
	} {
		if !strings.Contains(page, expected) {
			t.Fatalf("viewer inferred OKF 0.2 signals missing %q:\n%s", expected, page)
		}
	}
	if strings.Count(page, `data-signal-default="true"`) != 2 {
		t.Fatalf("viewer should mark both inferred OKF 0.2 signals as defaults:\n%s", page)
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
	viewerRuntime := viewerSourceForAssertions(t)
	if !strings.Contains(page, `href="/raw/references/report.pdf"`) {
		t.Fatalf("viewer should rewrite PDF links to raw browser URLs:\n%s", page)
	}
	if !strings.Contains(page, `href="/file/src/tool.go"`) {
		t.Fatalf("viewer should rewrite code links to asset preview pages:\n%s", page)
	}
	if !strings.Contains(page, `class="code-block language-go" data-language="go"`) || !strings.Contains(page, `tok-keyword">func</span>`) {
		t.Fatalf("viewer should syntax-highlight fenced code blocks:\n%s", page)
	}
	if !strings.Contains(viewerRuntime, `function isPanelPreviewPath(path)`) || !strings.Contains(viewerRuntime, `isPanelPreviewPath(raw)`) {
		t.Fatalf("viewer stack runtime should recognize code and text panel links:\n%s", page)
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
	if !strings.Contains(codePage, `class="viewer-document is-stack-mode"`) ||
		!strings.Contains(codePage, `data-note-workspace`) ||
		!strings.Contains(codePage, `data-note-path="src/tool.go"`) ||
		!strings.Contains(codePage, `class="note-body asset-code"`) ||
		!strings.Contains(codePage, `class="code-block language-go" data-language="go"`) ||
		!strings.Contains(codePage, `tok-keyword">package</span>`) {
		t.Fatalf("viewer should render highlighted code files in the standard note stack:\n%s", codePage)
	}
	if strings.Contains(codePage, `class="viewer-document viewer-asset-document"`) {
		t.Fatalf("code files should not use the standalone asset page:\n%s", codePage)
	}

	codeAPI := getViewerJSON(t, handler, "/api/file/src/tool.go")
	if codeAPI.Path != "src/tool.go" || codeAPI.Kind != "code" || !strings.Contains(codeAPI.Body, `class="code-block language-go"`) {
		t.Fatalf("viewer API should return a highlighted code panel payload: %#v", codeAPI)
	}

	pdfPage := getViewerBody(t, handler, "/file/references/report.pdf")
	if !strings.Contains(pdfPage, `class="viewer-document viewer-asset-document"`) || !strings.Contains(pdfPage, `class="asset-frame"`) || !strings.Contains(pdfPage, `src="/raw/references/report.pdf"`) {
		t.Fatalf("direct PDF asset page should embed the raw browser PDF URL:\n%s", pdfPage)
	}
}

func TestViewerRendersRelativeMarkdownImagesFromRawAssets(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "guides/architecture.md", strings.Join([]string{
		"# Architecture",
		"",
		"![System **overview**](../assets/architecture.png \"System overview\")",
		"",
		"![External diagram](https://example.com/architecture.png)",
		"",
		"[![Evidence coverage](../assets/architecture.png)](evidence.md \"Open evidence details\")",
		"",
		"| Preview | Reference |",
		"| --- | --- |",
		"| ![Thumbnail](../assets/architecture.png) | [Source](../assets/architecture.png) |",
	}, "\n"))
	writeViewerFile(t, root, "assets/architecture.png", "not-a-real-png")

	handler := newViewerHandler(root)
	page := getViewerBody(t, handler, "/file/guides/architecture.md")
	for _, expected := range []string{
		`<img class="ok-markdown-image" src="/raw/assets/architecture.png" alt="System overview" title="System overview" loading="lazy" decoding="async">`,
		`<img class="ok-markdown-image" src="/raw/assets/architecture.png" alt="Thumbnail" loading="lazy" decoding="async">`,
		`<img class="ok-markdown-image" src="https://example.com/architecture.png" alt="External diagram" loading="lazy" decoding="async">`,
		`<a href="/file/guides/evidence.md" title="Open evidence details"><img class="ok-markdown-image" src="/raw/assets/architecture.png" alt="Evidence coverage" loading="lazy" decoding="async"></a>`,
		`<a href="/raw/assets/architecture.png">Source</a>`,
	} {
		if !strings.Contains(page, expected) {
			t.Fatalf("viewer image output should contain %q:\n%s", expected, page)
		}
	}
	viewerSource := viewerSourceForAssertions(t)
	if !strings.Contains(viewerSource, `.ok-markdown-image`) || !strings.Contains(viewerSource, `.ok-table .ok-markdown-image`) {
		t.Fatalf("viewer stylesheet should constrain document and table images:\n%s", viewerSource)
	}

	api := getViewerJSON(t, handler, "/api/file/guides/architecture.md")
	if !strings.Contains(api.Body, `src="/raw/assets/architecture.png"`) {
		t.Fatalf("viewer API should render relative image URLs: %#v", api)
	}

	aliasPage := getViewerBody(t, newViewerHandlerWithAlias(root, "handbook"), "/handbook/file/guides/architecture.md")
	if !strings.Contains(aliasPage, `src="/handbook/raw/assets/architecture.png"`) {
		t.Fatalf("aliased viewer should prefix relative image URLs:\n%s", aliasPage)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/raw/assets/architecture.png", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Header().Get("Content-Type"), "image/png") {
		t.Fatalf("viewer should serve the image asset with its media type, got %d %q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
}

func TestViewerBundleCacheReusesParsedASTAndClaimsUntilMarkdownChanges(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# One\n")
	cache := newViewerBundleCache(root)
	loads := 0
	claimProjections := 0
	cache.loadSnapshot = func(root string) (viewerBundleSnapshot, error) {
		loads++
		return parseViewerBundleSnapshot(root)
	}
	cache.projectClaims = func(bundle okf.ASTBundle, fileURL func(string) string) viewerClaimsData {
		claimProjections++
		return viewerClaimsFromAST(bundle, fileURL)
	}

	first, _, err := cache.loadViewerData("")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := cache.loadViewerData(""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cache.loadViewerData("/docs"); err != nil {
		t.Fatal(err)
	}
	if loads != 1 || claimProjections != 2 || len(first.Files) != 1 || !strings.Contains(first.Files[0].Body, "# One") {
		t.Fatalf("viewer should reuse one parsed AST and one claim projection per route, loads=%d projections=%d bundle=%#v", loads, claimProjections, first)
	}

	path := filepath.Join(root, "index.md")
	if err := os.WriteFile(path, []byte("# Two\n"), 0644); err != nil {
		t.Fatal(err)
	}
	updated, _, err := cache.loadViewerData("")
	if err != nil {
		t.Fatal(err)
	}
	if loads != 2 || claimProjections != 3 || len(updated.Files) != 1 || !strings.Contains(updated.Files[0].Body, "# Two") {
		t.Fatalf("viewer should refresh cached Markdown and claims, loads=%d projections=%d bundle=%#v", loads, claimProjections, updated)
	}
}

func TestViewerServesSandboxedSVGImagesWithTheirMediaType(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n\n![Diagram](assets/diagram.svg)\n")
	writeViewerFile(t, root, "assets/diagram.svg", `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script><rect width="10" height="10"/></svg>`)

	handler := newViewerHandler(root)
	page := getViewerBody(t, handler, "/file/index.md")
	if !strings.Contains(page, `src="/raw/assets/diagram.svg"`) {
		t.Fatalf("viewer should resolve the SVG image through the raw asset route:\n%s", page)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/raw/assets/diagram.svg", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Header().Get("Content-Type"), "image/svg+xml") {
		t.Fatalf("viewer should serve SVG image media, got %d %q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	if policy := recorder.Header().Get("Content-Security-Policy"); policy != "default-src 'none'; sandbox" {
		t.Fatalf("viewer should sandbox SVG image responses, got %q", policy)
	}
}

func TestViewerHTMLExportRendersAndCopiesRelativeMarkdownImages(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, ".openknowledge.toml", "[release]\noutputs = [\"viewer\"]\n\n[publish]\nassets = [\"assets/**\"]\n")
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Architecture](guides/architecture.md).\n")
	writeViewerFile(t, root, "guides/architecture.md", "---\ntype: Guide\n---\n\n# Architecture\n\n![System overview](../assets/architecture.png)\n")
	writeViewerFile(t, root, "assets/architecture.png", "not-a-real-png")

	result, err := writeViewerHTMLWithVersion(root, out, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	page := readViewerExportFile(t, out, "guides/architecture.html")
	if !strings.Contains(page, `<img class="ok-markdown-image" src="../assets/architecture.png" alt="System overview" loading="lazy" decoding="async">`) {
		t.Fatalf("static viewer should keep a portable relative image URL:\n%s", page)
	}
	if _, err := os.Stat(filepath.Join(out, "assets", "architecture.png")); err != nil {
		t.Fatalf("static viewer should copy the allowlisted image: %v; written=%#v", err, result.Written)
	}
	data := readViewerExportFile(t, out, viewerDataScriptAsset)
	if !strings.Contains(data, `src=\"../assets/architecture.png\"`) {
		t.Fatalf("static panel data should contain the rendered relative image URL:\n%s", data)
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
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Setup](guides/setup.md), [Agents](AGENTS.md), and [Features](features/index.md).\n\n```mermaid\ngraph LR\n  Home --> Setup\n```\n\n| Kind | Required |\n| --- | --- |\n| flag | no |\n| argument | yes |\n")
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
	viewerRuntime := viewerSourceForAssertions(t) +
		readViewerExportFile(t, out, viewerDataScriptAsset)
	index := indexHTML + viewerRuntime
	if !strings.Contains(indexHTML, `data-note-workspace`) ||
		strings.Contains(indexHTML, `<script type="application/json" data-static-notes`) ||
		strings.Contains(indexHTML, `<script type="application/json" data-knowledge-graph`) ||
		strings.Contains(indexHTML, `<script type="application/json" data-editor-options`) {
		t.Fatalf("expected exported index to reference shared viewer data without embedding the bundle:\n%s", indexHTML)
	}
	for _, src := range []string{
		`assets/openknowledge/viewer-theme.js`,
		`assets/openknowledge/viewer-data.js`,
		`assets/openknowledge/viewer.js`,
	} {
		if !strings.Contains(indexHTML, `src="`+src+`"`) {
			t.Fatalf("expected exported index to load same-origin script %s:\n%s", src, indexHTML)
		}
	}
	if !strings.Contains(indexHTML, `href="assets/openknowledge/viewer.css"`) {
		t.Fatalf("expected exported index to load the shared viewer stylesheet:\n%s", indexHTML)
	}
	if strings.Contains(indexHTML, `<script>`) {
		t.Fatalf("generated viewer code must not require executable inline scripts:\n%s", indexHTML)
	}
	if !strings.Contains(index, `data-viewer-theme="default"`) ||
		!strings.Contains(index, `const defaultPreset = "default";`) ||
		!strings.Contains(index, `const defaultThemePreset = "default";`) {
		t.Fatalf("expected exported viewer to use Night on first run and bootstrap saved theme choices before paint:\n%s", index)
	}
	if !strings.Contains(index, `class="ok-table" data-ok-table`) || !strings.Contains(index, `function enhanceTables(scope)`) || !strings.Contains(index, `.ok-table-tools`) || !strings.Contains(index, `.ok-table-filter-panel`) {
		t.Fatalf("expected exported viewer pages to include rich table markup and runtime:\n%s", index)
	}
	if !strings.Contains(indexHTML, `data-language="mermaid" data-mermaid-source`) ||
		!strings.Contains(indexHTML, `src="assets/openknowledge/viewer.js"`) ||
		!strings.Contains(readViewerExportFile(t, out, viewerAppScriptAsset), `securityLevel:"strict"`) {
		t.Fatalf("expected exported viewer pages to render Mermaid through the shared viewer bundle:\n%s", indexHTML)
	}
	if !strings.Contains(index, `href="guides/setup.html"`) || !strings.Contains(index, `href="AGENTS.html"`) || !strings.Contains(index, `href="features/index.html"`) {
		t.Fatalf("expected exported index to keep static HTML fallback link:\n%s", index)
	}
	if !strings.Contains(index, `"path":"guides/setup.md"`) || !strings.Contains(index, `"htmlPath":"guides/setup.html"`) || !strings.Contains(index, `"path":"AGENTS.md"`) || !strings.Contains(index, `"htmlPath":"features/index.html"`) {
		t.Fatalf("expected exported index to embed rendered note manifest:\n%s", index)
	}
	if !strings.Contains(index, `function fetchNote(path`) || !strings.Contains(index, `staticNotesByPath[path]`) {
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
	if !strings.Contains(index, `OpenKnowledgeStaticData`) || !strings.Contains(index, `"source":"index.md"`) || !strings.Contains(index, `"target":"guides/setup.md"`) {
		t.Fatalf("expected exported index to include static knowledge graph:\n%s", index)
	}
	if !strings.Contains(index, `class="powered-by-openknowledge"`) || !strings.Contains(index, `href="https://openknowledge.sh"`) || !strings.Contains(index, `aria-label="Powered by OpenKnowledge.sh"`) || !strings.Contains(index, `data-tooltip="Powered by OpenKnowledge.sh"`) {
		t.Fatalf("expected exported index to include linked OpenKnowledge logo attribution:\n%s", index)
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
		!strings.Contains(setup, `src="../assets/openknowledge/viewer-data.js"`) ||
		!strings.Contains(setup, `src="../assets/openknowledge/viewer.js"`) ||
		!strings.Contains(setup, `href="../assets/openknowledge/viewer.css"`) ||
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

func viewerSourceForAssertions(t *testing.T) string {
	t.Helper()
	var source strings.Builder
	for _, name := range []string{
		"theme.js",
		"styles/shell.css",
		"styles/search-workspace.css",
		"styles/graph-navigation.css",
		"styles/document.css",
		"styles/motion.css",
		"styles/responsive.css",
		"shortcuts.js",
		"app.js",
		"search.js",
	} {
		content, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "src", "viewer", name))
		if err != nil {
			t.Fatalf("read viewer source %s: %v", name, err)
		}
		source.Write(content)
		source.WriteByte('\n')
	}
	return source.String()
}

func TestViewerHTMLExportIsDeterministicAndMachineIndependent(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, root)
	writeViewerFile(t, root, "index.md", "# Home\n\nProject overview. Read [Setup](guides/setup.md).\n")
	writeViewerFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup\n---\n\n# Setup\n\nPortable instructions.\n")

	firstOut := filepath.Join(t.TempDir(), "first")
	secondOut := filepath.Join(t.TempDir(), "second")
	first, err := writeViewerHTMLWithVersion(root, firstOut, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := writeViewerHTMLWithVersion(root, secondOut, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(first.Written, "\n") != strings.Join(second.Written, "\n") {
		t.Fatalf("deterministic exports wrote different file sets:\nfirst=%#v\nsecond=%#v", first.Written, second.Written)
	}
	for _, name := range first.Written {
		firstBody, err := os.ReadFile(filepath.Join(firstOut, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		secondBody, err := os.ReadFile(filepath.Join(secondOut, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(firstBody, secondBody) {
			t.Fatalf("exported file %s changed between identical builds", name)
		}
	}

	data := readViewerExportFile(t, firstOut, viewerDataScriptAsset)
	if strings.Contains(data, `"available":true`) ||
		strings.Contains(data, `"iconURL":"data:image/png`) ||
		strings.Contains(data, `/Applications/`) {
		t.Fatalf("static viewer data must not expose build-machine editor state:\n%s", data)
	}
	indexHTML := readViewerExportFile(t, firstOut, "index.html")
	setupHTML := readViewerExportFile(t, firstOut, "guides/setup.html")
	if strings.Contains(indexHTML, "Portable instructions.") ||
		strings.Contains(setupHTML, "Project overview.") ||
		strings.Contains(indexHTML, `data-static-notes`) ||
		strings.Contains(setupHTML, `data-static-notes`) {
		t.Fatal("each HTML page should contain its current note and reference the shared note collection")
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
	writeViewerFile(t, root, ".openknowledge.toml", "[release]\noutputs = [\"viewer\"]\n\n[publish]\nassets = [\"assets/public/**\", \"**/*.md\"]\n")

	result, err := writeViewerHTMLWithVersion(root, out, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Written, ",") != "assets/openknowledge-bundle.tar.gz,assets/openknowledge/viewer-data.js,assets/openknowledge/viewer-theme.js,assets/openknowledge/viewer.css,assets/openknowledge/viewer.js,assets/public/logo.svg,index.html,llms.txt,openknowledge.json,public.html" {
		t.Fatalf("expected only published viewer files, got %#v", result.Written)
	}
	if content := readViewerExportFile(t, out, "assets/public/logo.svg"); content != "<svg/>\n" {
		t.Fatalf("unexpected published asset content: %q", content)
	}
	for _, hidden := range []string{"assets/private/diagram.svg", "secret.txt", ".openknowledge.toml"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(hidden))); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be absent from public site, got err=%v", hidden, err)
		}
	}

	index := readViewerExportFile(t, out, viewerDataScriptAsset)
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
	for _, hidden := range []string{"draft.md", "examples/index.md", "assets/private/diagram.svg", "secret.txt", ".openknowledge.toml"} {
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
	writeViewerFile(t, root, ".openknowledge.toml", "[release]\noutputs = [\"viewer\"]\n\n[html.site]\nbase_url = \"https://example.test/wiki/\"\n")
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
	writeViewerFile(t, root, ".openknowledge.toml", "[html.theme]\ncss = \"assets/theme.css\"\n")
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
		"--ok-sidebar-min-width",
		"--ok-sidebar-default-width",
		"--ok-sidebar-max-width",
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
	const themeFixture = "/* custom viewer theme */\n"

	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, ".openknowledge.toml", "[release]\noutputs = [\"viewer\"]\n\n[html.theme]\nname = \"landing\"\nstylesheet = \"assets/wiki-theme.css\"\n")
	writeViewerFile(t, root, "assets/wiki-theme.css", themeFixture)
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Setup](guides/setup.md).\n")
	writeViewerFile(t, root, "guides/setup.md", "---\ntype: Guide\n---\n\n# Setup\n\nBack to [Home](../index.md).\n")
	writeViewerFile(t, root, "references/report.pdf", "%PDF-1.4\n% test pdf\n")

	handler := newViewerHandler(root)
	page := getViewerBody(t, handler, "/file/index.md")
	if !strings.Contains(page, `data-openknowledge-theme="landing"`) || !strings.Contains(page, `data-viewer-theme="default"`) || !strings.Contains(page, `href="/raw/assets/wiki-theme.css"`) {
		t.Fatalf("viewer should link the configured theme stylesheet from the raw endpoint:\n%s", page)
	}

	asset := getViewerBody(t, handler, "/file/references/report.pdf")
	if !strings.Contains(asset, `data-openknowledge-theme="landing"`) || !strings.Contains(asset, `data-viewer-theme="default"`) || !strings.Contains(asset, `href="/raw/assets/wiki-theme.css"`) {
		t.Fatalf("viewer asset pages should link the configured theme stylesheet from the raw endpoint:\n%s", asset)
	}

	alias := getViewerBody(t, newViewerHandlerWithAlias(root, "project-memory"), "/project-memory/file/index.md")
	if !strings.Contains(alias, `data-openknowledge-theme="landing"`) || !strings.Contains(alias, `data-viewer-theme="default"`) || !strings.Contains(alias, `href="/project-memory/raw/assets/wiki-theme.css"`) {
		t.Fatalf("viewer alias pages should link the prefixed theme stylesheet from the raw endpoint:\n%s", alias)
	}

	listRoot := t.TempDir()
	writeViewerFile(t, listRoot, ".openknowledge.toml", "[html.theme]\nname = \"landing\"\nstylesheet = \"assets/wiki-theme.css\"\n")
	writeViewerFile(t, listRoot, "assets/wiki-theme.css", themeFixture)
	writeViewerFile(t, listRoot, "notes/readme.md", "---\ntype: Note\n---\n\n# Readme\n")
	listing := getViewerBody(t, newViewerHandler(listRoot), "/")
	if !strings.Contains(listing, `data-openknowledge-theme="landing"`) || !strings.Contains(listing, `data-viewer-theme="default"`) || !strings.Contains(listing, `href="/raw/assets/wiki-theme.css"`) {
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
	if !strings.Contains(index, `data-openknowledge-theme="landing"`) || !strings.Contains(index, `data-viewer-theme="default"`) || !strings.Contains(index, `href="assets/wiki-theme.css"`) {
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
	if theme != themeFixture {
		t.Fatalf("expected export to copy theme stylesheet, got:\n%s", theme)
	}
}

func TestViewerHTMLExportLinksConfiguredGitHubSource(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, ".openknowledge.toml", "[release]\noutputs = [\"viewer\"]\n\n[html.source]\ngithub_base = \"https://github.com/openknowledge-sh/openknowledge/blob/main\"\nentry = \"Wiki\"\n")
	writeViewerFile(t, root, "index.md", "# Home\n\nRead [Setup](guides/setup.md).\n")
	writeViewerFile(t, root, "guides/setup.md", "---\ntype: Guide\n---\n\n# Setup\n")

	if _, err := writeViewerHTMLWithVersion(root, out, "0.1"); err != nil {
		t.Fatal(err)
	}

	index := readViewerExportFile(t, out, "index.html")
	staticData := readViewerExportFile(t, out, viewerDataScriptAsset)
	if strings.Contains(index, `<div class="editor-picker" data-editor-picker>`) || strings.Contains(index, `<button class="editor-menu-trigger"`) {
		t.Fatalf("static export should not render the local editor dropdown:\n%s", index)
	}
	if !strings.Contains(index, `class="source-open"`) || !strings.Contains(index, `href="https://github.com/openknowledge-sh/openknowledge/blob/main/Wiki/index.md"`) {
		t.Fatalf("static export should link the current file to GitHub source:\n%s", index)
	}
	if !strings.Contains(staticData, `"sourceURL":"https://github.com/openknowledge-sh/openknowledge/blob/main/Wiki/guides/setup.md"`) {
		t.Fatalf("static note manifest should include GitHub source URLs for dynamic panels:\n%s", staticData)
	}

	setup := readViewerExportFile(t, out, "guides/setup.html")
	if !strings.Contains(setup, `href="https://github.com/openknowledge-sh/openknowledge/blob/main/Wiki/guides/setup.md"`) {
		t.Fatalf("nested static export should link its GitHub source:\n%s", setup)
	}
}

func TestViewerHTMLExportWritesDiscoveryFilesWithSiteURL(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "site")
	writeViewerFile(t, root, ".openknowledge.toml", "[release]\noutputs = [\"viewer\"]\n\n[html.site]\nbase_url = \"https://openknowledge.sh/wiki/\"\n")
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
	writeViewerFile(t, root, ".openknowledge.toml", "[release]\noutputs = [\"viewer\"]\n\n[html.site]\nbase_url = \"/wiki/\"\n")
	writeViewerFile(t, root, "index.md", "# Home\n")

	_, err := writeViewerHTMLWithVersion(root, filepath.Join(t.TempDir(), "site"), "0.1")
	if err == nil || !strings.Contains(err.Error(), "html.site.base_url must be an http(s) URL") {
		t.Fatalf("expected invalid site base URL to be rejected, got %v", err)
	}
}

func TestViewerThemeConfigReportsMissingStylesheetInOpen(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, ".openknowledge.toml", "[html.theme]\nname = \"landing\"\nstylesheet = \"assets/missing.css\"\n")
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
	writeViewerFile(t, root, ".openknowledge.toml", "[release]\noutputs = [\"viewer\"]\n\n[html.theme]\nstylesheet = \"../landing.css\"\n")
	writeViewerFile(t, root, "index.md", "# Home\n")

	if _, err := writeViewerHTMLWithVersion(root, filepath.Join(t.TempDir(), "site"), "0.1"); err == nil || !strings.Contains(err.Error(), "must stay inside the bundle") {
		t.Fatalf("expected outside theme stylesheet to be rejected, got %v", err)
	}
}

func TestViewerThemeConfigRejectsSymbolicLink(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "bundle")
	writeViewerFile(t, root, ".openknowledge.toml", "[html.theme]\nstylesheet = \"assets/theme.css\"\n")
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
	viewerRuntime := getViewerBody(t, handler, "/"+viewerAppScriptAsset)
	if !strings.Contains(index, "notes/details.md") || !strings.Contains(index, "workflows/docs.md") {
		t.Fatalf("viewer index fallback did not include markdown files:\n%s", index)
	}
	if !strings.Contains(index, `id="viewer-search"`) {
		t.Fatalf("viewer index fallback did not include search input:\n%s", index)
	}
	if !strings.Contains(viewerRuntime, `OpenKnowledgeShortcuts`) || !strings.Contains(viewerRuntime, `viewer.search.focus`) {
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
	viewerRuntime := getViewerBody(t, handler, "/"+viewerAppScriptAsset)
	if !strings.Contains(page, `<a class="brand" href="/project-memory/">Home</a>`) {
		t.Fatalf("viewer file brand should link to the alias root:\n%s", page)
	}
	if strings.Contains(page, `<span class="note-breadcrumb-label">project-memory</span>`) {
		t.Fatalf("single knowledge base should not prefix the note breadcrumb:\n%s", page)
	}
	if !strings.Contains(page, `href="/project-memory/file/workflows/docs.md"`) {
		t.Fatalf("viewer file did not prefix markdown links:\n%s", page)
	}
	if !strings.Contains(page, `href="/project-memory/raw/references/report.pdf"`) {
		t.Fatalf("viewer file did not prefix raw asset links:\n%s", page)
	}
	if !strings.Contains(page, `data-link-prefix="/project-memory"`) || (!strings.Contains(viewerRuntime, `linkPrefix + "/api/file/"`) && !strings.Contains(viewerRuntime, `/api/file/`)) {
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

func TestViewerRejectsTraversalAndNonPreviewAPI(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Home\n")
	writeViewerFile(t, root, "notes.txt", "not markdown\n")
	writeViewerFile(t, root, "data.bin", "not a preview\n")
	writeViewerFile(t, root, ".env", "TOKEN=secret\n")
	writeViewerFile(t, root, ".openknowledge.toml", "[html.theme]\nname = \"night\"\n")
	writeViewerFile(t, root, "openknowledge.toml", "legacy configuration\n")
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
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"kind":"text"`) {
		t.Fatalf("expected text file API to return a panel payload, code=%d body=%q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/file/data.bin", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected non-preview file API to return 404, got %d", recorder.Code)
	}

	for _, rawPath := range []string{"index.md", ".env", ".git/config", ".openknowledge.toml", "openknowledge.toml", "missing.txt"} {
		recorder = httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/raw/"+rawPath, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected private or non-asset raw path %s to return 404, got %d", rawPath, recorder.Code)
		}
	}
	for _, assetPath := range []string{".env", ".git/config", ".openknowledge.toml", "openknowledge.toml"} {
		recorder = httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/file/"+assetPath, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected private asset page %s to return 404, got %d", assetPath, recorder.Code)
		}
	}
	indexRecorder := httptest.NewRecorder()
	handler.ServeHTTP(indexRecorder, httptest.NewRequest(http.MethodGet, "/file/index.md", nil))
	for _, privateName := range []string{".env", ".openknowledge.toml", "openknowledge.toml", ".git"} {
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
	writeViewerFile(t, personal, "only-personal.md", "---\ntype: Note\n---\n\n# Personal note\n\nShared context lives here.\n")
	writeViewerFile(t, work, "index.md", "# Work\n\nSee [Guide](notes/guide.md).\n")
	writeViewerFile(t, work, "notes/guide.md", "---\ntype: Note\n---\n\n# Guide\n\nShared context supports validation before publishing.\n")
	writeViewerFile(t, work, "token-evidence.txt", "Production tokens use the declared format.")
	writeViewerFile(t, work, "auth.md", viewerTypedClaimFixture())

	handler := newRegistryViewerHandler([]okf.RegistryEntry{
		{Name: "personal", Path: personal, Access: "read"},
		{Name: "work", Path: work, Access: "write"},
	})

	rootResponse := httptest.NewRecorder()
	handler.ServeHTTP(rootResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	if rootResponse.Code != http.StatusFound || rootResponse.Header().Get("Location") != "/kb/personal/file/index.md" {
		t.Fatalf("registry root should open the unified workspace, code=%d location=%q", rootResponse.Code, rootResponse.Header().Get("Location"))
	}
	index := getViewerBody(t, handler, "/kb/personal/file/index.md")
	for _, required := range []string{
		"Knowledge bases",
		`data-knowledge-base-name="personal"`,
		`data-knowledge-base-name="work"`,
		`href="/kb/work/file/index.md"`,
		"only-personal.md",
	} {
		if !strings.Contains(index, required) {
			t.Fatalf("registry index missing %q:\n%s", required, index)
		}
	}

	workRootResponse := httptest.NewRecorder()
	handler.ServeHTTP(workRootResponse, httptest.NewRequest(http.MethodGet, "/kb/work/", nil))
	if workRootResponse.Code != http.StatusFound || workRootResponse.Header().Get("Location") != "/kb/work/file/index.md" {
		t.Fatalf("knowledge base root should keep the user in the workspace, code=%d location=%q", workRootResponse.Code, workRootResponse.Header().Get("Location"))
	}

	workPage := getViewerBody(t, handler, "/kb/work/file/index.md")
	if !strings.Contains(workPage, `href="/kb/work/file/notes/guide.md"`) {
		t.Fatalf("registry viewer did not prefix markdown links:\n%s", workPage)
	}
	if !strings.Contains(workPage, `<div class="editor-picker" data-editor-picker>`) {
		t.Fatalf("writable registry connection should expose editor controls:\n%s", workPage)
	}
	for _, required := range []string{
		`data-search-url="/api/search"`,
		`data-knowledge-base-name="personal"`,
		`data-knowledge-base-name="work"`,
		`data-knowledge-base="personal"`,
		`data-knowledge-base="work"`,
		`data-knowledge-base-connect aria-label="Connect knowledge base"`,
		`data-knowledge-base-dialog`,
		`data-note-path="index.md" data-note-title="Index" data-knowledge-base="work"`,
		`<span class="note-breadcrumb-label">work</span><span class="note-breadcrumb-separator" aria-hidden="true">/</span>`,
		`data-knowledge-graph data-url="/api/graph">{}</script>`,
		`data-claims-url="/kb/personal/api/claims"`,
		`data-claims-url="/kb/work/api/claims"`,
		`data-claims-data data-url="/kb/work/api/claims">{}</script>`,
	} {
		if !strings.Contains(workPage, required) {
			t.Fatalf("registry document workspace missing %q:\n%s", required, workPage)
		}
	}
	if count := strings.Count(workPage, `data-claims-view-toggle`); count != 2 {
		t.Fatalf("registry claims navigation should render once per knowledge base, got %d", count)
	}
	personalClaimsToggle := viewerButtonTagWith(workPage, `data-knowledge-base="personal" data-claims-url="/kb/personal/api/claims"`)
	if personalClaimsToggle == "" || strings.Contains(personalClaimsToggle, " hidden") {
		t.Fatalf("registry navigation should expose an empty claims view for a plain knowledge base: %s", personalClaimsToggle)
	}
	workClaimsToggle := viewerButtonTagWith(workPage, `data-knowledge-base="work" data-claims-url="/kb/work/api/claims"`)
	if workClaimsToggle == "" || strings.Contains(workClaimsToggle, " hidden") {
		t.Fatalf("registry navigation should expose claims for a claim-enabled knowledge base: %s", workClaimsToggle)
	}
	graph := getViewerBody(t, handler, "/api/graph")
	for _, required := range []string{`"knowledgeBase":"personal"`, `"knowledgeBase":"work"`, `"sourcePath":"index.md"`} {
		if !strings.Contains(graph, required) {
			t.Fatalf("lazy registry graph missing %q:\n%s", required, graph)
		}
	}
	claims := getViewerBody(t, handler, "/kb/work/api/claims")
	if !strings.Contains(claims, `"knowledgeBase":"work"`) || strings.Contains(claims, `"knowledgeBase":"personal"`) {
		t.Fatalf("knowledge-base claims endpoint must not combine sources:\n%s", claims)
	}
	globalClaims := httptest.NewRecorder()
	handler.ServeHTTP(globalClaims, httptest.NewRequest(http.MethodGet, "/api/claims", nil))
	if globalClaims.Code != http.StatusNotFound {
		t.Fatalf("top-level claims endpoint should not exist, got %d: %s", globalClaims.Code, globalClaims.Body.String())
	}
	if strings.Contains(workPage, `note-knowledge-base-marker`) || strings.Contains(workPage, `class="note-knowledge-base"`) {
		t.Fatalf("registry document breadcrumb should not use a colored knowledge-base marker:\n%s", workPage)
	}

	globalSearch := getViewerSearch(t, handler, "/api/search?q=shared&limit=12")
	if len(globalSearch.Results) < 2 {
		t.Fatalf("registry search should return results from both knowledge bases: %#v", globalSearch)
	}
	foundBases := map[string]bool{}
	for _, result := range globalSearch.Results {
		foundBases[result.KnowledgeBase] = true
		if !strings.HasPrefix(result.URL, "/kb/"+result.KnowledgeBase+"/file/") {
			t.Fatalf("registry search result URL did not preserve its knowledge base: %#v", result)
		}
	}
	if !foundBases["personal"] || !foundBases["work"] {
		t.Fatalf("registry search did not identify both knowledge bases: %#v", globalSearch)
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

func TestRegistryViewerConnectsLocalKnowledgeBaseAndGuidesSetup(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(okf.RegistryFileEnv, registryFile)
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "# Connected\n")
	handler := newRegistryViewerHandler(nil)

	request := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases", strings.NewReader(fmt.Sprintf(`{"path":%q,"name":"connected","access":"write"}`, root)))
	request.Host = "127.0.0.1:8080"
	request.Header.Set("Origin", "http://127.0.0.1:8080")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected connect endpoint to return 201, got %d: %s", response.Code, response.Body.String())
	}
	var connected viewerKnowledgeBaseConnectResponse
	if err := json.Unmarshal(response.Body.Bytes(), &connected); err != nil {
		t.Fatal(err)
	}
	if connected.Status != "connected" || connected.Name != "connected" || connected.URL != "/kb/connected/" {
		t.Fatalf("unexpected connect response: %#v", connected)
	}
	entry, found, err := okf.ResolveRegistryEntry("connected")
	if err != nil || !found || entry.Path != root || entry.Access != "write" {
		t.Fatalf("viewer did not record the local connection: entry=%#v found=%v err=%v", entry, found, err)
	}

	empty := t.TempDir()
	request = httptest.NewRequest(http.MethodPost, "/api/knowledge-bases", strings.NewReader(fmt.Sprintf(`{"path":%q}`, empty)))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"status":"needs_setup"`) || !strings.Contains(response.Body.String(), "okn setup") {
		t.Fatalf("expected setup guidance for an empty folder, got %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/knowledge-bases", strings.NewReader(fmt.Sprintf(`{"path":%q}`, root)))
	request.Host = "127.0.0.1:8080"
	request.Header.Set("Origin", "https://example.test")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected cross-origin registry mutation to be rejected, got %d", response.Code)
	}
}

func TestRegistryViewerEmptyRegistry(t *testing.T) {
	body := getViewerBody(t, newRegistryViewerHandler(nil), "/")
	if !strings.Contains(body, "Connect a knowledge space to browse its documents") || !strings.Contains(body, `data-knowledge-base-dialog`) {
		t.Fatalf("empty registry page did not explain the empty state:\n%s", body)
	}
}

func TestRegistryCombinedProjectionsReportUnavailableKnowledgeBases(t *testing.T) {
	available := t.TempDir()
	writeViewerFile(t, available, "index.md", "# Available\n")
	workspaces := []viewerWorkspace{
		{Name: "available", ResolvedRoot: available, URL: "/kb/available/"},
		{Name: "broken", Error: "index could not be loaded", URL: "/kb/broken/"},
	}

	var graph viewerGraphData
	if err := json.Unmarshal([]byte(registryGraphJSON(workspaces)), &graph); err != nil {
		t.Fatal(err)
	}
	if graph.Status != "partial" || len(graph.Nodes) == 0 || len(graph.Failures) != 1 || graph.Failures[0].KnowledgeBase != "broken" {
		t.Fatalf("global graph should identify partial results: %#v", graph)
	}

	var claims viewerClaimsData
	if err := json.Unmarshal([]byte(registryClaimsJSON(workspaces)), &claims); err != nil {
		t.Fatal(err)
	}
	if claims.ProjectionStatus != "partial" || len(claims.Failures) != 1 || claims.Failures[0].KnowledgeBase != "broken" {
		t.Fatalf("global claims should identify partial results: %#v", claims)
	}

	failed := []viewerWorkspace{{Name: "broken", Error: "index could not be loaded", URL: "/kb/broken/"}}
	if err := json.Unmarshal([]byte(registryGraphJSON(failed)), &graph); err != nil {
		t.Fatal(err)
	}
	if graph.Status != "failed" {
		t.Fatalf("graph with no available knowledge bases should fail, got %#v", graph)
	}
	if err := json.Unmarshal([]byte(registryClaimsJSON(failed)), &claims); err != nil {
		t.Fatal(err)
	}
	if claims.ProjectionStatus != "failed" {
		t.Fatalf("claims with no available knowledge bases should fail, got %#v", claims)
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
	rootResponse := httptest.NewRecorder()
	handler.ServeHTTP(rootResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	if rootResponse.Code != http.StatusFound || rootResponse.Header().Get("Location") != "/kb/docs/file/index.md" {
		t.Fatalf("viewer root did not reopen the unified workspace, code=%d location=%q", rootResponse.Code, rootResponse.Header().Get("Location"))
	}
	index := getViewerBody(t, handler, rootResponse.Header().Get("Location"))
	if !strings.Contains(index, `href="/kb/second/file/index.md"`) {
		t.Fatalf("viewer did not load a newly connected knowledge base into the workspace:\n%s", index)
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
