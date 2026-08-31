import { bindMermaidViewport, closeMermaidViewport } from "./mermaid-viewport.js";

(function () {
  const workspace = document.querySelector("[data-note-workspace]");
  const stackEl = document.querySelector("[data-note-stack]");
  const emptyState = document.querySelector("[data-empty-state]");
  const fileSidebar = document.querySelector("[data-file-sidebar]");
  const sidebarToggle = document.querySelector("[data-sidebar-toggle]");
  const sidebarResizeHandle = document.querySelector("[data-sidebar-resize-handle]");
  const documentsViewToggle = document.querySelector("[data-documents-view-toggle]");
  const claimsViewToggles = Array.from(document.querySelectorAll("[data-claims-view-toggle]"));
  const claimsWorkspace = document.querySelector("[data-claims-workspace]");
  const claimsTitle = document.querySelector("[data-claims-title]");
  const claimsList = document.querySelector("[data-claims-list]");
  const claimsDetail = document.querySelector("[data-claims-detail]");
  const claimsFilters = document.querySelector("[data-claims-filters]");
  const claimsQuery = document.querySelector("[data-claims-query]");
  const knowledgeBasesToggle = document.querySelector("[data-knowledge-bases-toggle]");
  const knowledgeBaseList = document.querySelector("#knowledge-base-list");
  const knowledgeBaseDialog = document.querySelector("[data-knowledge-base-dialog]");
  const knowledgeBaseForm = document.querySelector("[data-knowledge-base-form]");
  const settings = document.querySelector("[data-viewer-settings]");
  const settingsTrigger = document.querySelector("[data-viewer-settings-trigger]");
  const settingsMenu = document.querySelector("[data-viewer-settings-menu]");
  const horizontalStack = document.querySelector("[data-horizontal-stack]");
  const graphViewToggle = document.querySelector("[data-graph-view-toggle]");
  const customThemeFields = document.querySelector("[data-theme-custom-fields]");
  const frontmatterVisibility = document.querySelector("[data-frontmatter-visibility]");
  const accessibilityFont = document.querySelector("[data-accessibility-font]");
  const accessibilitySize = document.querySelector("[data-accessibility-size]");
  const accessibilitySpacing = document.querySelector("[data-accessibility-spacing]");
  const accessibilityMotion = document.querySelector("[data-accessibility-motion]");
  const readableLineLength = document.querySelector("[data-readable-line-length]");
  const highContrast = document.querySelector("[data-high-contrast]");
  const underlineLinks = document.querySelector("[data-underline-links]");
  const scrollRail = document.querySelector("[data-workspace-rail]");
  const scrollTrack = document.querySelector("[data-workspace-scroll-track]");
  const scrollThumb = document.querySelector("[data-workspace-scroll-thumb]");

  if (!workspace || !stackEl) {
    return;
  }

  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  const mobileSidebar = window.matchMedia("(max-width: 720px)");
  const editorStorageKey = "openknowledge.viewer.editorOrder";
  const themeStorageKey = "openknowledge.viewer.theme";
  const frontmatterStorageKey = "openknowledge.viewer.frontmatter";
  const accessibilityStorageKey = "openknowledge.viewer.accessibility";
  const navigationModeStorageKey = "openknowledge.viewer.navigationMode";
  const knowledgeBaseColorStorageKey = "openknowledge.viewer.knowledgeBaseColors";
  const defaultNavigationMode = "beside";
  let navigationMode = defaultNavigationMode;
  let graphViewRequested = false;
  let knowledgeGraphRendered = false;
  let claimsViewRequested = false;
  let selectedClaimKey = "";
  let claimsInspectorContext = null;
  const linkPrefix = normalizeLinkPrefix(workspace.dataset.linkPrefix || "");
  const currentKnowledgeBase = String(workspace.dataset.knowledgeBase || document.body.dataset.activeKnowledgeBase || "").trim();
  const viewerStorageScope = graphHash(workspace.dataset.noteRoot || linkPrefix || window.location.pathname).toString(36);
  const liveReloadStateKey = "openknowledge.viewer.liveReload." + viewerStorageScope;
  const panelWidthStorageKey = "openknowledge.viewer.panelWidths." + viewerStorageScope;
  const sidebarWidthStorageKey = "openknowledge.viewer.sidebarWidth." + viewerStorageScope;
  const graphSettingsStorageKey = "openknowledge.viewer.graphSettings." + viewerStorageScope;
  const editorOptions = readEditorOptions();
  const panelWidths = readPanelWidths();
  let sidebarWidth = readSidebarWidth();
  const staticNotes = readStaticNotes();
  const liveReloadRestoreState = readLiveReloadState();
  const staticNotesByPath = indexStaticNotes(staticNotes, "path");
  const staticNotePathByHTML = indexStaticNotePathsByHTML(staticNotes);
  const knownNotePaths = collectKnownNotePaths();
  const knowledgeGraph = readKnowledgeGraph();
  const claimsData = readClaimsData();
  let knowledgeGraphLoadPromise = null;
  let activeClaimsKnowledgeBase = currentKnowledgeBase;
  const claimsDataCache = new Map();
  const claimsDataLoadPromises = new Map();
  const themePresets = ["default", "night", "paper", "ocean", "rose", "custom"];
  const defaultThemePreset = "default";
  const knowledgeBasePalette = ["#0a4a9c", "#9a4d0f", "#08745d", "#8a3f75", "#5e55a5", "#a0353f", "#316b24", "#8b5f00"];
  const accessibilityFonts = {
    system: 'Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI Variable", "Segoe UI", sans-serif',
    readable: 'Verdana, Tahoma, Arial, sans-serif',
    serif: 'Iowan Old Style, Baskerville, "Times New Roman", serif',
    mono: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace'
  };
  const accessibilityFontSizes = {
    small: { value: "14px", scale: "0.9" },
    default: { value: "15.5px", scale: "1" },
    large: { value: "18px", scale: "1.16" },
    "extra-large": { value: "21px", scale: "1.35" }
  };
  const accessibilityLineHeights = {
    default: { value: "1.62", letterSpacing: "normal" },
    relaxed: { value: "1.82", letterSpacing: ".02em" },
    spacious: { value: "2.05", letterSpacing: ".04em" }
  };
  const defaultAccessibilityPreference = {
    font: "system",
    size: "default",
    spacing: "default",
    motion: "system",
    readableLineLength: true,
    highContrast: false,
    underlineLinks: false
  };
  const customThemeDefaults = {
    page: "#edf5fa",
    surface: "#fbfdff",
    text: "#082b63",
    muted: "#496887",
    accent: "#0a4a9c",
    border: "#c8d9e8"
  };
  const customThemeVariables = {
    page: ["--ok-color-page", "--ok-color-header-bg", "--ok-color-viewer-canvas", "--ok-color-viewer-header-bg", "--ok-color-sidebar", "--ok-color-sidebar-header"],
    surface: ["--ok-color-surface", "--ok-color-note-chrome-bg", "--ok-color-search-input-bg", "--ok-color-search-popover-bg", "--ok-color-editor-trigger-bg", "--ok-color-editor-menu-bg", "--ok-color-card-bg", "--ok-color-editor-mark-bg", "--ok-color-graph-node-bg"],
    text: ["--ok-color-text", "--ok-color-document-text", "--ok-color-control-hover-text", "--ok-color-editor-mark-text", "--ok-color-code-block-bg", "--ok-color-graph-label-active"],
    muted: ["--ok-color-muted", "--ok-color-control-text", "--ok-color-close-text", "--ok-color-sidebar-text", "--ok-color-search-shortcut-text", "--ok-color-tree-text", "--ok-color-tree-badge-text", "--ok-color-note-close-text", "--ok-color-editor-trigger-text", "--ok-color-graph-label"],
    accent: ["--ok-color-accent", "--ok-color-accent-strong", "--ok-color-focus-ring", "--ok-color-graph-node-active-border"],
    border: ["--ok-color-border", "--ok-color-control-hover-border", "--ok-color-close-hover-border", "--ok-color-sidebar-border", "--ok-color-search-input-border", "--ok-color-search-shortcut-border", "--ok-color-search-popover-border", "--ok-color-card-border", "--ok-color-tree-badge-border", "--ok-color-note-close-hover-border", "--ok-color-editor-trigger-border", "--ok-color-editor-trigger-separator", "--ok-color-editor-menu-border", "--ok-color-editor-menu-separator", "--ok-color-graph-node-border"]
  };
  let mermaidRenderQueue = Promise.resolve();
  let mermaidRenderID = 0;
  let mermaidRequestID = 0;
  let mermaidThemeTimer = 0;
  let workspaceRailFrame = 0;
  const narrationState = {
    panel: null,
    chunks: [],
    index: 0,
    status: "idle",
    token: 0,
  };
  const panelCloseShortcut = {
    id: "viewer.panel.close",
    code: "KeyW",
    metaOrCtrlKey: true,
    altKey: true,
    label: "⌘⌥W",
    ariaKeyShortcut: "Meta+Alt+W",
    when: function () {
      return Boolean(closeablePanel());
    },
    run: function () {
      const panel = closeablePanel();
      if (panel) {
        closePanel(panel, true);
      }
    }
  };

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

  function clamp(value, min, max) {
    return Math.min(Math.max(value, min), max);
  }

  function readPanelWidths() {
    const stored = readStoredJSON(panelWidthStorageKey);
    if (stored && typeof stored === "object" && !Array.isArray(stored)) {
      return stored;
    }
    return {};
  }

  function savePanelWidths() {
    const serialized = JSON.stringify(panelWidths);
    try {
      window.localStorage.setItem(panelWidthStorageKey, serialized);
    } catch {
      // Browser storage can be disabled in private or file-export contexts.
    }
    writeCookie(panelWidthStorageKey, serialized);
  }

  function readSidebarWidth() {
    const stored = readStoredJSON(sidebarWidthStorageKey);
    const value = stored && typeof stored === "object" ? stored.width : stored;
    return normalizeSidebarWidth(value);
  }

  function saveSidebarWidth() {
    if (!sidebarWidth) {
      return;
    }
    const serialized = JSON.stringify(sidebarWidth);
    try {
      window.localStorage.setItem(sidebarWidthStorageKey, serialized);
    } catch {
      // Browser storage can be disabled in private or file-export contexts.
    }
    writeCookie(sidebarWidthStorageKey, serialized);
  }

  function readStoredJSON(key) {
    const sources = [readLocalStorage(key), readCookie(key)];
    for (const source of sources) {
      if (!source) {
        continue;
      }
      try {
        return JSON.parse(source);
      } catch {
        // Ignore malformed storage and keep the viewer usable.
      }
    }
    return null;
  }

  function readLocalStorage(key) {
    try {
      return window.localStorage.getItem(key);
    } catch {
      return null;
    }
  }

  function readLiveReloadState() {
    let raw = "";
    try {
      raw = window.sessionStorage.getItem(liveReloadStateKey) || "";
      window.sessionStorage.removeItem(liveReloadStateKey);
    } catch {
      return null;
    }
    if (!raw) {
      return null;
    }
    try {
      const state = JSON.parse(raw);
      if (!state || state.version !== 1 || Date.now() - Number(state.createdAt || 0) > 30000) {
        return null;
      }
      return state;
    } catch {
      return null;
    }
  }

  function saveLiveReloadState(state) {
    try {
      window.sessionStorage.setItem(liveReloadStateKey, JSON.stringify(state));
    } catch {
      // A refresh remains useful when session storage is unavailable.
    }
  }

  function readCookie(name) {
    const prefix = encodeURIComponent(name) + "=";
    const parts = document.cookie ? document.cookie.split("; ") : [];
    for (const part of parts) {
      if (part.startsWith(prefix)) {
        try {
          return decodeURIComponent(part.slice(prefix.length));
        } catch {
          return null;
        }
      }
    }
    return null;
  }

  function writeCookie(name, value) {
    try {
      document.cookie = encodeURIComponent(name) + "=" + encodeURIComponent(value) + "; Max-Age=31536000; Path=/; SameSite=Lax";
    } catch {
      // Cookies are best-effort; localStorage still covers same-origin exports.
    }
  }

  function knowledgeBaseNames() {
    const names = [];
    document.querySelectorAll("[data-knowledge-base-name]").forEach(function (element) {
      const name = String(element.dataset.knowledgeBaseName || "").trim();
      if (name && !names.includes(name)) {
        names.push(name);
      }
    });
    if (currentKnowledgeBase && !names.includes(currentKnowledgeBase)) {
      names.unshift(currentKnowledgeBase);
    }
    return names;
  }

  function readKnowledgeBaseColorOverrides() {
    const stored = readStoredJSON(knowledgeBaseColorStorageKey);
    if (!stored || typeof stored !== "object" || Array.isArray(stored)) {
      return {};
    }
    const colors = {};
    Object.keys(stored).forEach(function (name) {
      if (isHexColor(stored[name])) {
        colors[name] = stored[name].toLowerCase();
      }
    });
    return colors;
  }

  let knowledgeBaseColorOverrides = readKnowledgeBaseColorOverrides();

  function knowledgeBaseColor(name) {
    const normalizedName = String(name || "").trim();
    if (knowledgeBaseColorOverrides[normalizedName]) {
      return knowledgeBaseColorOverrides[normalizedName];
    }
    const names = knowledgeBaseNames();
    const index = Math.max(0, names.indexOf(normalizedName));
    return knowledgeBasePalette[index % knowledgeBasePalette.length];
  }

  function saveKnowledgeBaseColors() {
    const serialized = JSON.stringify(knowledgeBaseColorOverrides);
    try {
      window.localStorage.setItem(knowledgeBaseColorStorageKey, serialized);
    } catch {
      // Browser storage can be disabled in private contexts.
    }
    writeCookie(knowledgeBaseColorStorageKey, serialized);
  }

  function applyKnowledgeBaseColors() {
    document.querySelectorAll("[data-knowledge-base]").forEach(function (element) {
      const name = String(element.dataset.knowledgeBase || "").trim();
      if (name) {
        element.style.setProperty("--knowledge-base-color", knowledgeBaseColor(name));
      }
    });
    document.querySelectorAll("[data-knowledge-base-name]").forEach(function (element) {
      const name = String(element.dataset.knowledgeBaseName || "").trim();
      const input = element.querySelector("[data-knowledge-base-color]");
      if (input && name) {
        input.value = knowledgeBaseColor(name);
      }
    });
  }

  window.OpenKnowledgeKnowledgeBases = {
    color: knowledgeBaseColor,
  };

  function normalizeNavigationMode(value) {
    return value === "beside" || value === "replace" ? value : defaultNavigationMode;
  }

  function readNavigationModePreference() {
    return normalizeNavigationMode(readStoredJSON(navigationModeStorageKey));
  }

  function saveNavigationModePreference(value) {
    const serialized = JSON.stringify(normalizeNavigationMode(value));
    try {
      window.localStorage.setItem(navigationModeStorageKey, serialized);
    } catch {
      // Browser storage can be disabled in private or file-export contexts.
    }
    writeCookie(navigationModeStorageKey, serialized);
  }

  function navigationModeTitle() {
    return navigationMode === "beside"
      ? "Links open beside. Hold Shift to replace the current panel."
      : "Links open in the current panel. Hold Shift to open beside.";
  }

  function applyNavigationMode(value) {
    navigationMode = normalizeNavigationMode(value);
    document.documentElement.dataset.viewerNavigationMode = navigationMode;
    if (horizontalStack) {
      horizontalStack.checked = navigationMode === "beside";
    }
    updateLinkBehaviorHints();
  }

  function shouldOpenBeside(shiftKey) {
    const besideByDefault = navigationMode === "beside";
    return shiftKey ? !besideByDefault : besideByDefault;
  }

  function bindNavigationMode() {
    applyNavigationMode(readNavigationModePreference());
    if (!horizontalStack || horizontalStack.dataset.modeBound === "true") {
      return;
    }
    horizontalStack.dataset.modeBound = "true";
    horizontalStack.addEventListener("change", function () {
      const next = horizontalStack.checked ? "beside" : "replace";
      saveNavigationModePreference(next);
      applyNavigationMode(next);
    });
  }

  function graphViewIsVisible() {
    return !claimsViewRequested && (graphViewRequested || panels().length === 0);
  }

  function claimsViewIsVisible() {
    return claimsViewRequested && claimsViewCanOpen(claimsData, activeClaimsKnowledgeBase);
  }

  function workspaceViewName() {
    if (claimsViewIsVisible()) {
      return "claims";
    }
    return graphViewIsVisible() ? "graph" : "notes";
  }

  function syncWorkspaceViewURL(pushHistory) {
    const url = new URL(window.location.href);
    const view = workspaceViewName();
    if (view === "claims") {
      url.searchParams.set("view", "claims");
      const selected = claimByKey(selectedClaimKey);
      if (selected) {
        url.searchParams.set("claim", selected.id);
      } else {
        url.searchParams.delete("claim");
      }
      const claimsKnowledgeBase = String(selected?.knowledgeBase || activeClaimsKnowledgeBase || "").trim();
      if (claimsKnowledgeBase) {
        url.searchParams.set("knowledge-base", claimsKnowledgeBase);
      } else {
        url.searchParams.delete("knowledge-base");
      }
    } else if (view === "graph") {
      url.searchParams.set("view", "graph");
      url.searchParams.delete("claim");
      url.searchParams.delete("knowledge-base");
    } else {
      url.searchParams.delete("view");
      url.searchParams.delete("claim");
      url.searchParams.delete("knowledge-base");
    }
    const state = { stack: currentStackTargets(), view: view, claim: selectedClaimKey, claimsKnowledgeBase: activeClaimsKnowledgeBase };
    window.history[pushHistory ? "pushState" : "replaceState"](state, "", url);
  }

  function setGraphViewRequested(value, updateLocation) {
    if (typeof updateLocation === "undefined") {
      updateLocation = true;
    }
    graphViewRequested = Boolean(value);
    if (graphViewRequested) {
      claimsViewRequested = false;
    }
    updateWorkspaceState();
    updateTitle();
    if (updateLocation) {
      syncWorkspaceViewURL(false);
    }
  }

  function setClaimsViewRequested(value, updateLocation) {
    claimsViewRequested = Boolean(value) && claimsViewCanOpen(claimsData, activeClaimsKnowledgeBase);
    if (claimsViewRequested) {
      graphViewRequested = false;
      renderClaimsWorkspace();
    }
    updateWorkspaceState();
    updateTitle();
    if (updateLocation) {
      syncWorkspaceViewURL(false);
    }
  }

  function bindGraphView() {
    if (!graphViewToggle || graphViewToggle.dataset.graphViewBound === "true") {
      return;
    }
    graphViewToggle.dataset.graphViewBound = "true";
    graphViewToggle.addEventListener("click", function () {
      setGraphViewRequested(!graphViewRequested);
      if (mobileSidebar.matches) {
        setSidebarOpen(false);
      }
    });
  }

  function bindDocumentsView() {
    if (!documentsViewToggle || documentsViewToggle.dataset.documentsViewBound === "true") {
      return;
    }
    documentsViewToggle.dataset.documentsViewBound = "true";
    documentsViewToggle.addEventListener("click", function () {
      if (panels().length === 0) {
        fileSidebar?.querySelector("[data-tree-path]")?.focus();
        return;
      }
      claimsViewRequested = false;
      setGraphViewRequested(false, true);
      if (mobileSidebar.matches) {
        setSidebarOpen(false);
      }
    });
  }

  function bindClaimsView() {
    claimsViewToggles.forEach(function (toggle) {
      if (toggle.dataset.claimsViewBound === "true") {
        return;
      }
      toggle.dataset.claimsViewBound = "true";
      toggle.addEventListener("click", async function () {
        const knowledgeBase = claimsToggleKnowledgeBase(toggle);
        const closeCurrent = claimsViewRequested && knowledgeBase === activeClaimsKnowledgeBase;
        if (!closeCurrent) {
          toggle.setAttribute("aria-busy", "true");
          try {
            await ensureClaimsData(knowledgeBase);
          } finally {
            toggle.removeAttribute("aria-busy");
          }
        }
        refreshClaimsViewToggles();
        setClaimsViewRequested(!closeCurrent, true);
        if (mobileSidebar.matches) {
          setSidebarOpen(false);
        }
      });
    });
    refreshClaimsViewToggles();
  }

  function claimsToggleKnowledgeBase(toggle) {
    return String(toggle?.dataset.knowledgeBase || currentKnowledgeBase || "").trim();
  }

  function claimsToggleForKnowledgeBase(knowledgeBase) {
    const requested = String(knowledgeBase || "").trim();
    return claimsViewToggles.find(function (toggle) {
      return claimsToggleKnowledgeBase(toggle) === requested;
    }) || (claimsViewToggles.length === 1 ? claimsViewToggles[0] : null);
  }

  function claimsDataURL(knowledgeBase) {
    const toggle = claimsToggleForKnowledgeBase(knowledgeBase);
    const toggleURL = String(toggle?.dataset.claimsUrl || "");
    if (toggleURL) {
      return toggleURL;
    }
    const source = document.querySelector("[data-claims-data]");
    return String(source?.dataset.url || "");
  }

  function refreshClaimsViewToggles() {
    claimsViewToggles.forEach(function (toggle) {
      const knowledgeBase = claimsToggleKnowledgeBase(toggle);
      const dataURL = String(toggle.dataset.claimsUrl || "") || claimsDataURL(knowledgeBase);
      const loaded = !dataURL || claimsDataCache.has(knowledgeBase);
      const cached = loaded
        ? claimsDataCache.get(knowledgeBase)
          || (knowledgeBase === activeClaimsKnowledgeBase ? claimsData : null)
        : null;
      if (loaded) {
        toggle.hidden = !claimsWorkspaceAvailable(cached);
      }
      const count = toggle.querySelector("[data-claims-navigation-count]");
      if (count) {
        count.textContent = !cached || cached.claims.length === 0
          ? ""
          : String(cached.claims.length);
      }
      const label = knowledgeBase ? "Claims for " + knowledgeBase : "Claims";
      toggle.title = projectionHasProblems(cached) ? projectionFailureSummary(cached) : label;
    });
  }

  function claimIdentity(claim) {
    const identity = String(claim?.key || ((claim?.knowledgeBase || "") + "\u0000" + (claim?.id || "")));
    return identity.charCodeAt(0) === 0 ? identity.slice(1) : identity;
  }

  function claimDocumentIdentity(claim) {
    return encodeURIComponent(String(claim?.knowledgeBase || "")) + "|" + encodeURIComponent(String(claim?.declaringPath || ""));
  }

  function claimDocumentLabel(claim) {
    const knowledgeBase = String(claim?.knowledgeBase || "");
    const prefix = knowledgeBase && knowledgeBase !== activeClaimsKnowledgeBase ? knowledgeBase + " / " : "";
    return prefix + String(claim?.declaringPath || "");
  }

  function projectionHasProblems(data) {
    return (data?.status === "partial" || data?.status === "failed")
      && Array.isArray(data?.failures)
      && data.failures.length > 0;
  }

  function claimsWorkspaceAvailable(data) {
    return Array.isArray(data?.claims) && data.claims.length > 0;
  }

  function claimsViewCanOpen(data) {
    return claimsWorkspaceAvailable(data);
  }

  function projectionFailureSummary(data) {
    const failures = Array.isArray(data?.failures) ? data.failures : [];
    if (!failures.length) {
      return "";
    }
    const names = failures.map(function (failure) {
      return String(failure?.knowledgeBase || "Unknown knowledge space");
    });
    return names.length === 1
      ? names[0] + " is unavailable"
      : names.length + " knowledge spaces are unavailable";
  }

  function claimByKey(key) {
    return claimsData.claims.find(function (claim) {
      return claimIdentity(claim) === key;
    }) || null;
  }

  function claimByID(id, knowledgeBase) {
    const exactKnowledgeBase = String(knowledgeBase || "");
    return claimsData.claims.find(function (claim) {
      return claim.id === id && (!exactKnowledgeBase || !claim.knowledgeBase || claim.knowledgeBase === exactKnowledgeBase);
    }) || claimsData.claims.find(function (claim) {
      return claim.id === id;
    }) || null;
  }

  function claimStatement(claim) {
    if (claim?.projection?.metric) {
      return [
        claim?.subject?.label || claim?.subject?.id,
        claim.projection.metric,
        claim.projection.value
      ]
        .filter(Boolean)
        .join(" — ");
    }
    return [
      claim?.subject?.label || claim?.subject?.id,
      claim?.predicate?.label || claim?.predicate?.id,
      claimMetricLabel(claim),
      claim?.object?.label
    ]
      .filter(Boolean)
      .join(" — ");
  }

  function claimMetricLabel(claim) {
    return claim?.object?.quantityKindLabel || claim?.object?.quantityKind || "";
  }

  function sameKnowledgeBase(left, right) {
    const knowledgeBase = function (value) {
      return value && typeof value === "object" ? String(value.knowledgeBase || "") : String(value || "");
    };
    return knowledgeBase(left) === knowledgeBase(right);
  }

  function entityByID(id, knowledgeBase) {
    return claimsData.entities.find(function (entity) {
      return entity.id === id && sameKnowledgeBase(entity, knowledgeBase);
    }) || null;
  }

  function predicateByID(id, knowledgeBase) {
    return claimsData.predicates.find(function (predicate) {
      return predicate.id === id && sameKnowledgeBase(predicate, knowledgeBase);
    }) || null;
  }

  function sourceForEvidence(claim, evidence) {
    return claimsData.sources.find(function (source) {
      return source.id === evidence?.sourceRef && source.declaringPath === claim?.declaringPath && sameKnowledgeBase(source, claim);
    }) || null;
  }

  function metricEntityForClaim(claim) {
    const metric = (claim?.scope || []).find(function (item) {
      const id = String(item.dimension?.id || "").toLocaleLowerCase();
      return id.split(/[/#:]/).pop() === "metric" && item.value?.ref;
    });
    return metric ? entityByID(metric.value.ref, claim.knowledgeBase) : null;
  }

  function contextualTermButton(label, kind, target, claim, className) {
    const button = createClaimsElement("button", "claims-term-button " + (className || ""), label);
    button.type = "button";
    button.addEventListener("click", function () {
      claimsInspectorContext = {
        kind: kind,
        key: target.key,
        originClaimKey: claimIdentity(claim)
      };
      renderClaimsWorkspace();
      window.requestAnimationFrame(focusClaimsInspectorHeading);
    });
    return button;
  }

  function focusClaimsInspectorHeading() {
    const heading = claimsDetail?.querySelector("h2");
    if (heading) {
      heading.tabIndex = -1;
      heading.focus();
    }
  }

  function appendInteractiveClaimStatement(parent, claim, classPrefix) {
    const prefix = classPrefix || "";
    const subject = entityByID(claim?.subject?.id, claim?.knowledgeBase);
    const metricEntity = metricEntityForClaim(claim);
    const predicate = predicateByID(claim?.predicate?.id, claim?.knowledgeBase);
    const objectEntity = claim?.object?.ref ? entityByID(claim.object.ref, claim.knowledgeBase) : null;
    parent.append(subject
      ? contextualTermButton(claim.subject.label || claim.subject.id, "entity", subject, claim, prefix + "subject")
      : createClaimsElement("span", prefix + "subject", claim?.subject?.label || claim?.subject?.id || "Unknown subject"));
    if (claim?.projection?.metric) {
      const separator = createClaimsElement("span", prefix + "separator", "—");
      separator.setAttribute("aria-hidden", "true");
      parent.append(separator, metricEntity
        ? contextualTermButton(claim.projection.metric, "entity", metricEntity, claim, prefix + "metric")
        : createClaimsElement("span", prefix + "metric", claim.projection.metric));
      const valueLine = createClaimsElement("span", prefix + "value-line");
      valueLine.append(createClaimsElement("strong", prefix + "object", claim.projection.value || claim?.object?.label || "—"));
      parent.append(valueLine);
      return;
    }
    parent.append(predicate
      ? contextualTermButton(claim.predicate.label || claim.predicate.id, "predicate", predicate, claim, prefix + "predicate")
      : createClaimsElement("span", prefix + "predicate", claim?.predicate?.label || claim?.predicate?.id || "Unknown predicate"));
    const metric = claimMetricLabel(claim);
    if (metric) {
      parent.append(createClaimsElement("span", prefix + "metric", metric + ":"));
    }
    parent.append(objectEntity
      ? contextualTermButton(claim.object.label || claim.object.ref, "entity", objectEntity, claim, prefix + "object")
      : createClaimsElement("strong", prefix + "object", claim?.object?.label || "—"));
  }

  function appendClaimStatement(parent, claim, classPrefix) {
    const prefix = classPrefix || "";
    if (claim?.projection?.metric) {
      const separator = createClaimsElement("span", prefix + "separator", "—");
      separator.setAttribute("aria-hidden", "true");
      parent.append(
        createClaimsElement("span", prefix + "subject", claim?.subject?.label || claim?.subject?.id || "Unknown subject"),
        separator,
        createClaimsElement("span", prefix + "metric", claim.projection.metric)
      );
      const valueLine = createClaimsElement("span", prefix + "value-line");
      valueLine.append(createClaimsElement("strong", prefix + "object", claim.projection.value || claim?.object?.label || "—"));
      parent.append(valueLine);
      return;
    }
    parent.append(
      createClaimsElement("span", prefix + "subject", claim?.subject?.label || claim?.subject?.id || "Unknown subject"),
      createClaimsElement("span", prefix + "predicate", claim?.predicate?.label || claim?.predicate?.id || "Unknown predicate")
    );
    const metric = claimMetricLabel(claim);
    if (metric) {
      parent.append(createClaimsElement("span", prefix + "metric", metric + ":"));
    }
    parent.append(createClaimsElement("strong", prefix + "object", claim?.object?.label || "—"));
  }

  function claimRelationEntries(claim) {
    const relations = claim?.relations || {};
    return [
      ["Supersedes", relations.supersedes],
      ["Contradicts", relations.contradicts],
      ["Derived from", relations.derivedFrom]
    ].flatMap(function (entry) {
      return (Array.isArray(entry[1]) ? entry[1] : []).map(function (id) {
        return { label: entry[0], id: id, direction: "outgoing" };
      });
    });
  }

  function incomingClaimRelations(claim) {
    if (!claim) {
      return [];
    }
    const incomingLabels = {
      supersedes: "Superseded by",
      contradicts: "Contradicted by",
      derivedFrom: "Source for"
    };
    const entries = [];
    claimsData.claims.forEach(function (candidate) {
      if (String(candidate.knowledgeBase || "") !== String(claim.knowledgeBase || "")) {
        return;
      }
      const relations = candidate.relations || {};
      Object.keys(incomingLabels).forEach(function (kind) {
        if (Array.isArray(relations[kind]) && relations[kind].includes(claim.id)) {
          entries.push({ label: incomingLabels[kind], id: candidate.id, key: claimIdentity(candidate), direction: "incoming" });
        }
      });
    });
    return entries;
  }

  function createClaimsElement(tag, className, textValue) {
    const element = document.createElement(tag);
    if (className) {
      element.className = className;
    }
    if (typeof textValue === "string") {
      element.textContent = textValue;
    }
    return element;
  }

  function appendClaimBadge(parent, value, kind) {
    if (!value) {
      return;
    }
    const badge = createClaimsElement("span", "claims-badge", value);
    badge.dataset.kind = kind || value;
    parent.append(badge);
  }

  function appendClaimDefinition(list, label, value, options) {
    if (value === undefined || value === null || value === "" || (Array.isArray(value) && value.length === 0)) {
      return;
    }
    const row = createClaimsElement("div", "claims-definition");
    row.append(createClaimsElement("dt", "", label));
    const description = createClaimsElement("dd");
    if (options?.mono) {
      description.classList.add("is-mono");
    }
    if (options?.multiline) {
      String(value).split("\n").forEach(function (line) {
        description.append(createClaimsElement("span", "", line));
      });
    } else {
      description.textContent = Array.isArray(value) ? value.join(", ") : String(value);
    }
    row.append(description);
    list.append(row);
  }

  function appendClaimLinkDefinition(list, label, value, kind, target, claim, options) {
    if (!target) {
      appendClaimDefinition(list, label, value, options);
      return;
    }
    const row = createClaimsElement("div", "claims-definition");
    row.append(createClaimsElement("dt", "", label));
    const description = createClaimsElement("dd");
    if (options?.mono) {
      description.classList.add("is-mono");
    }
    description.append(contextualTermButton(value, kind, target, claim));
    row.append(description);
    list.append(row);
  }

  function claimsFilterOptions() {
    const options = {
      status: new Map(), subject: new Map(), predicate: new Map(), owner: new Map(),
      stance: new Map(), document: new Map()
    };
    claimsData.claims.forEach(function (claim) {
      options.status.set(claim.status, claim.status);
      options.subject.set(claim.subject?.id, claim.subject?.label || claim.subject?.id);
      options.predicate.set(claim.predicate?.id, claim.predicate?.label || claim.predicate?.id);
      (claim.owners || []).forEach(function (owner) { options.owner.set(owner, owner); });
      (claim.evidence || []).forEach(function (evidence) { options.stance.set(evidence.stance, evidence.stance); });
      options.document.set(claimDocumentIdentity(claim), claimDocumentLabel(claim));
    });
    return options;
  }

  function populateClaimsFilters() {
    if (!claimsFilters || claimsFilters.dataset.populated === "true") {
      return;
    }
    claimsFilters.dataset.populated = "true";
    const options = claimsFilterOptions();
    claimsFilters.querySelectorAll("[data-claims-filter]").forEach(function (select) {
      const key = select.dataset.claimsFilter;
      const values = options[key];
      if (!(values instanceof Map)) {
        return;
      }
      Array.from(values.entries()).sort(function (left, right) {
        return String(left[1]).localeCompare(String(right[1]));
      }).forEach(function (entry) {
        const option = document.createElement("option");
        option.value = entry[0];
        option.textContent = entry[1];
        select.append(option);
      });
    });
  }

  function currentClaimsFilters() {
    const values = { query: String(claimsQuery?.value || "").trim().toLocaleLowerCase() };
    claimsFilters?.querySelectorAll("[data-claims-filter]").forEach(function (select) {
      values[select.dataset.claimsFilter] = select.value;
    });
    return values;
  }

  function claimMatchesFilters(claim, filters) {
    if (filters.status && claim.status !== filters.status) {
      return false;
    }
    if (filters.subject && claim.subject?.id !== filters.subject) {
      return false;
    }
    if (filters.predicate && claim.predicate?.id !== filters.predicate) {
      return false;
    }
    if (filters.owner && !(claim.owners || []).includes(filters.owner)) {
      return false;
    }
    if (filters.stance && !(claim.evidence || []).some(function (evidence) { return evidence.stance === filters.stance; })) {
      return false;
    }
    if (filters.document && claimDocumentIdentity(claim) !== filters.document) {
      return false;
    }
    if (filters.validation === "valid" && (claim.issues || []).length > 0) {
      return false;
    }
    if (filters.validation === "invalid" && (claim.issues || []).length === 0) {
      return false;
    }
    if (filters.validation === "stale" && !claim.stale) {
      return false;
    }
    if (!filters.query) {
      return true;
    }
    const haystack = [
      claimStatement(claim), claim.id, claim.slot, claim.subject?.id, claim.predicate?.id,
      claim.declaringPath, claim.knowledgeBase, (claim.owners || []).join(" "),
      (claim.scope || []).map(function (item) { return [item.dimension?.id, item.dimension?.label, item.value?.ref, item.value?.label].join(" "); }).join(" "),
      (claim.evidence || []).map(function (evidence) { return [evidence.sourceRef, evidence.stance, evidence.role].join(" "); }).join(" ")
    ].join(" ").toLocaleLowerCase();
    return haystack.includes(filters.query);
  }

  function filteredClaims() {
    const filters = currentClaimsFilters();
    return claimsData.claims.filter(function (claim) {
      return claimMatchesFilters(claim, filters);
    });
  }

  function renderClaimsList(matches) {
    if (!claimsList) {
      return;
    }
    claimsList.replaceChildren();
    matches.forEach(function (claim) {
      const row = createClaimsElement("button", "claims-result");
      row.type = "button";
      row.dataset.claimKey = claimIdentity(claim);
      row.setAttribute("role", "option");
      row.setAttribute("aria-selected", claimIdentity(claim) === selectedClaimKey ? "true" : "false");
      const statement = createClaimsElement("span", "claims-result-statement");
      appendClaimStatement(statement, claim, "claims-result-");
      row.append(statement);
      const showSource = Boolean(claim.knowledgeBase) && knowledgeBaseNames().length > 1;
      if (!claim.projection?.metric || showSource || (claim.issues || []).length > 0) {
        const meta = createClaimsElement("span", "claims-result-meta");
        if (!claim.projection?.metric) {
          appendClaimBadge(meta, claim.status, claim.status);
          if (claim.stale) {
            appendClaimBadge(meta, "stale", "stale");
          }
        }
        if (!claim.projection?.metric || showSource) {
          meta.append(createClaimsElement("span", "claims-result-path", claimDocumentLabel(claim)));
        }
        if ((claim.issues || []).length > 0) {
          appendClaimBadge(meta, String(claim.issues.length) + " issues", "invalid");
        }
        row.append(meta);
      }
      row.addEventListener("click", function () {
        selectClaim(claimIdentity(claim), true);
      });
      claimsList.append(row);
    });
    if (matches.length === 0) {
      const empty = createClaimsElement("div", "claims-empty");
      if (claimsData.status === "failed") {
        empty.append(createClaimsElement("strong", "", "Claims are unavailable."));
        empty.append(createClaimsElement("span", "", projectionFailureSummary(claimsData) + ". Refresh to try again."));
      } else if (claimsData.claims.length === 0) {
        const label = activeClaimsKnowledgeBase ? " in " + activeClaimsKnowledgeBase : "";
        empty.append(createClaimsElement("strong", "", "No claims" + label + "."));
        empty.append(createClaimsElement("span", "", "Add typed claims to this knowledge space, then refresh the viewer."));
      } else {
        empty.append(createClaimsElement("strong", "", claimsData.status === "partial"
          ? "No available claims match these filters."
          : "No claims match these filters."));
        empty.append(createClaimsElement("span", "", claimsData.status === "partial"
          ? projectionFailureSummary(claimsData) + ". Clear a filter or search the available results."
          : "Clear a filter or search for another statement, ID, owner, or document."));
      }
      claimsList.append(empty);
    }
  }

  function claimsUsingEntity(entity) {
    return claimsData.claims.filter(function (claim) {
      if (!sameKnowledgeBase(claim, entity)) {
        return false;
      }
      return claim.subject?.id === entity.id || claim.object?.ref === entity.id || (claim.scope || []).some(function (item) {
        return item.value?.ref === entity.id;
      });
    });
  }

  function claimsUsingPredicate(predicate) {
    return claimsData.claims.filter(function (claim) {
      return sameKnowledgeBase(claim, predicate) && (claim.predicate?.id === predicate.id || (claim.scope || []).some(function (item) {
        return item.dimension?.id === predicate.id;
      }));
    });
  }

  function claimsUsingSource(source) {
    return claimsData.claims.filter(function (claim) {
      return sameKnowledgeBase(claim, source) && claim.declaringPath === source.declaringPath && (claim.evidence || []).some(function (evidence) {
        return evidence.sourceRef === source.id;
      });
    });
  }

  function createContextClaimList(title, claims, source) {
    if (claims.length === 0) {
      return null;
    }
    const section = createClaimsElement("section", "claims-context-section");
    section.append(createClaimsElement("h3", "", title + " (" + claims.length + ")"));
    const list = createClaimsElement("div", "claims-context-list");
    claims.forEach(function (claim) {
      const row = createClaimsElement("button", "claims-context-row");
      row.type = "button";
      row.append(createClaimsElement("strong", "", claimStatement(claim)));
      const evidence = source ? (claim.evidence || []).find(function (item) { return item.sourceRef === source.id; }) : null;
      row.append(createClaimsElement("span", "", evidence
        ? [evidence.stance, evidence.role, claim.declaringPath].filter(Boolean).join(" · ")
        : [claim.status, claim.declaringPath].filter(Boolean).join(" · ")));
      row.addEventListener("click", function () {
        selectClaim(claimIdentity(claim), true);
        window.requestAnimationFrame(focusClaimsInspectorHeading);
      });
      list.append(row);
    });
    section.append(list);
    return section;
  }

  function renderClaimsContextInspector(context) {
    if (!claimsDetail || !context) {
      return;
    }
    const collection = context.kind === "entity" ? claimsData.entities : context.kind === "predicate" ? claimsData.predicates : claimsData.sources;
    const target = collection.find(function (item) { return item.key === context.key; });
    if (!target) {
      claimsInspectorContext = null;
      renderClaimInspector(claimByKey(selectedClaimKey));
      return;
    }
    claimsDetail.replaceChildren();
    const back = createClaimsElement("button", "claims-context-back", "Back to claim");
    back.type = "button";
    back.addEventListener("click", function () {
      claimsInspectorContext = null;
      renderClaimsWorkspace();
      window.requestAnimationFrame(focusClaimsInspectorHeading);
    });
    const header = createClaimsElement("header", "claims-context-header");
    header.append(createClaimsElement("h2", "", target.label || target.title || target.id));
    header.append(createClaimsElement("code", "claims-context-id", target.id));
    const definitions = createClaimsElement("dl", "claims-definition-list claims-context-definitions");
    let relatedClaims = [];
    if (context.kind === "entity") {
      appendClaimDefinition(definitions, "Types", target.types || [], { mono: true });
      appendClaimDefinition(definitions, "Aliases", target.altLabels || []);
      appendClaimDefinition(definitions, "Deprecated", target.deprecated ? "yes" : "no");
      appendClaimDefinition(definitions, "Replaced by", target.replacedBy, { mono: true });
      relatedClaims = claimsUsingEntity(target);
    } else if (context.kind === "predicate") {
      appendClaimDefinition(definitions, "Object kind", target.objectKind);
      appendClaimDefinition(definitions, "Datatype", target.datatype, { mono: true });
      appendClaimDefinition(definitions, "Quantity kind", target.quantityKind, { mono: true });
      appendClaimDefinition(definitions, "Canonical unit", target.canonicalUnit, { mono: true });
      appendClaimDefinition(definitions, "Required scope", target.requiredScope || [], { mono: true });
      appendClaimDefinition(definitions, "Maximum count", target.maximumCount ? String(target.maximumCount) : "");
      relatedClaims = claimsUsingPredicate(target);
    } else {
      appendClaimDefinition(definitions, "Resource", target.resource, { mono: true });
      appendClaimDefinition(definitions, "Role", target.role);
      appendClaimDefinition(definitions, "Observation", target.observe);
      appendClaimDefinition(definitions, "Digest", target.sha256, { mono: true });
      appendClaimDefinition(definitions, "Author", target.author);
      appendClaimDefinition(definitions, "Access", target.access || []);
      appendClaimDefinition(definitions, "Last modified", target.lastModified);
      appendClaimDefinition(definitions, "Usage count", target.usageCount === undefined ? "" : String(target.usageCount));
      appendClaimDefinition(definitions, "Usage window", target.usageWindow ? target.usageWindow.from + " – " + target.usageWindow.to : "");
      relatedClaims = claimsUsingSource(target);
    }
    const actions = createClaimsElement("div", "claims-inspector-actions");
    if (context.kind === "source") {
      const documentLink = createClaimsElement("a", "claims-source-link", "Open declaring document");
      documentLink.href = target.documentURL;
      documentLink.dataset.claimDocumentPath = target.declaringPath;
      if (target.knowledgeBase) {
        documentLink.dataset.knowledgeBase = target.knowledgeBase;
      }
      actions.append(documentLink);
    }
    const related = createContextClaimList(context.kind === "source" ? "Evidence for" : "Used by", relatedClaims, context.kind === "source" ? target : null);
    claimsDetail.append(back, header);
    if (actions.childElementCount) {
      claimsDetail.append(actions);
    }
    if (definitions.childElementCount) {
      claimsDetail.append(definitions);
    }
    if (related) {
      claimsDetail.append(related);
    }
  }

  function createClaimHistory(claim) {
    const occurrences = claimsData.claims.filter(function (candidate) {
      return candidate.slot && candidate.slot === claim.slot && sameKnowledgeBase(candidate, claim);
    });
    if (occurrences.length < 2) {
      return null;
    }
    const section = createClaimsElement("details", "claims-inspector-disclosure claims-inspector-history");
    section.append(createClaimsElement("summary", "", "History (" + occurrences.length + ")"));
    const list = createClaimsElement("div", "claims-relationship-list");
    occurrences.forEach(function (occurrence) {
      const row = createClaimsElement("button", "claims-relationship-row");
      row.type = "button";
      row.append(createClaimsElement("span", "claims-relationship-label", claimIdentity(occurrence) === claimIdentity(claim) ? "Current" : (occurrence.status || "Occurrence")));
      row.append(createClaimsElement("strong", "", claimStatement(occurrence)));
      row.addEventListener("click", function () {
        selectClaim(claimIdentity(occurrence), true);
        window.requestAnimationFrame(focusClaimsInspectorHeading);
      });
      list.append(row);
    });
    section.append(list);
    return section;
  }

  function createClaimImpact(claim) {
    const references = claimsData.references.filter(function (reference) {
      return reference.claimId === claim.id && sameKnowledgeBase(reference, claim);
    });
    const sourceResources = (claim.evidence || []).map(function (evidence) {
      return sourceForEvidence(claim, evidence)?.resource;
    }).filter(Boolean);
    const sharedClaims = claimsData.claims.filter(function (candidate) {
      if (claimIdentity(candidate) === claimIdentity(claim) || !sameKnowledgeBase(candidate, claim)) {
        return false;
      }
      return (candidate.evidence || []).some(function (evidence) {
        const source = sourceForEvidence(candidate, evidence);
        return source && sourceResources.includes(source.resource);
      });
    });
    const dependentPaths = Array.from(new Set((claim.dependents || []).concat(references.map(function (reference) { return reference.path; }))));
    if (dependentPaths.length === 0 && sharedClaims.length === 0) {
      return null;
    }
    const count = dependentPaths.length + sharedClaims.length;
    const section = createClaimsElement("details", "claims-inspector-disclosure claims-inspector-impact");
    section.append(createClaimsElement("summary", "", "Impact (" + count + ")"));
    const list = createClaimsElement("div", "claims-relationship-list");
    dependentPaths.forEach(function (path) {
      const reference = references.find(function (item) { return item.path === path; });
      const row = createClaimsElement(reference ? "a" : "div", "claims-relationship-row");
      row.append(createClaimsElement("span", "claims-relationship-label", "Referenced by"));
      row.append(createClaimsElement("strong", "", path));
      if (reference) {
        row.href = reference.url;
      }
      list.append(row);
    });
    sharedClaims.forEach(function (candidate) {
      const row = createClaimsElement("button", "claims-relationship-row");
      row.type = "button";
      row.append(createClaimsElement("span", "claims-relationship-label", "Shared source"));
      row.append(createClaimsElement("strong", "", claimStatement(candidate)));
      row.addEventListener("click", function () {
        selectClaim(claimIdentity(candidate), true);
        window.requestAnimationFrame(focusClaimsInspectorHeading);
      });
      list.append(row);
    });
    section.append(list);
    return section;
  }

  function renderClaimInspector(claim) {
    if (!claimsDetail) {
      return;
    }
    claimsDetail.replaceChildren();
    if (!claim) {
      claimsDetail.append(createClaimsElement("div", "claims-empty", "Select a claim to inspect its evidence and provenance."));
      return;
    }
    const heading = createClaimsElement("header", "claims-inspector-header");
    const title = createClaimsElement("h2", "claims-inspector-statement");
    appendInteractiveClaimStatement(title, claim, "claims-inspector-");
    heading.append(title);
    if (!claim.projection?.metric) {
      const badges = createClaimsElement("div", "claims-inspector-badges");
      appendClaimBadge(badges, claim.status, claim.status);
      appendClaimBadge(badges, claim.trustTier, "trust");
      if (claim.stale) {
        appendClaimBadge(badges, "stale", "stale");
      }
      heading.append(badges);
    }

    const actions = createClaimsElement("div", "claims-inspector-actions");
    const source = createClaimsElement("a", "claims-source-link", "Open source document");
    source.href = claim.documentURL;
    source.dataset.claimDocumentPath = claim.declaringPath;
    if (claim.knowledgeBase) {
      source.dataset.knowledgeBase = claim.knowledgeBase;
    }
    actions.append(source);

    const definitions = createClaimsElement("dl", "claims-definition-list");
    appendClaimDefinition(definitions, "Claim ID", claim.id, { mono: true });
    appendClaimDefinition(definitions, "Slot", claim.slot, { mono: true });
    appendClaimLinkDefinition(definitions, "Subject", claim.subject?.id, "entity", entityByID(claim.subject?.id, claim.knowledgeBase), claim, { mono: true });
    appendClaimLinkDefinition(definitions, "Predicate", claim.predicate?.id, "predicate", predicateByID(claim.predicate?.id, claim.knowledgeBase), claim, { mono: true });
    appendClaimDefinition(definitions, "Datatype", claim.object?.datatype, { mono: true });
    appendClaimDefinition(definitions, "Quantity kind", claim.object?.quantityKind, { mono: true });
    appendClaimDefinition(definitions, "Unit", claim.object?.unit, { mono: true });
    appendClaimDefinition(definitions, "Scope", (claim.scope || []).map(function (item) {
      return (item.dimension?.label || item.dimension?.id) + ": " + (item.value?.label || "—");
    }).join("\n"), { multiline: true });
    appendClaimDefinition(definitions, "Valid time", claim.validTime?.from || claim.validTime?.until
      ? [claim.validTime?.from ? "from " + claim.validTime.from : "", claim.validTime?.until ? "until " + claim.validTime.until : ""].filter(Boolean).join(" ")
      : "");
    appendClaimDefinition(definitions, "Owners", claim.owners || []);
    appendClaimDefinition(definitions, "Section", claim.sectionRef, { mono: true });

    const evidenceSection = createClaimsElement("section", "claims-inspector-section");
    evidenceSection.append(createClaimsElement("h3", "", "Evidence"));
    if ((claim.evidence || []).length === 0) {
      evidenceSection.append(createClaimsElement("p", "claims-empty-inline", "No evidence records are attached."));
    } else {
      const evidenceList = createClaimsElement("div", "claims-evidence-list");
      claim.evidence.forEach(function (evidence) {
        const item = createClaimsElement("article", "claims-evidence");
        const itemHead = createClaimsElement("div", "claims-evidence-head");
        const evidenceSource = sourceForEvidence(claim, evidence);
        itemHead.append(evidenceSource
          ? contextualTermButton(evidenceSource.title || evidence.sourceRef || evidence.id, "source", evidenceSource, claim, "claims-evidence-source")
          : createClaimsElement("strong", "", evidence.sourceRef || evidence.id));
        appendClaimBadge(itemHead, evidence.stance, evidence.stance);
        item.append(itemHead);
        const meta = createClaimsElement("p", "", [evidence.role, evidence.observedAt].filter(Boolean).join(" · "));
        if (meta.textContent) {
          item.append(meta);
        }
        if (evidence.selector) {
          const selector = createClaimsElement("code", "claims-selector", evidence.selector.type + (evidence.selector.exact ? ": " + evidence.selector.exact : ""));
          item.append(selector);
        }
        evidenceList.append(item);
      });
      evidenceSection.append(evidenceList);
    }

    const provenance = createClaimsElement("section", "claims-inspector-section");
    provenance.append(createClaimsElement("h3", "", "Provenance and lifecycle"));
    const provenanceList = createClaimsElement("dl", "claims-definition-list is-compact");
    appendClaimDefinition(provenanceList, "Status", claim.status);
    appendClaimDefinition(provenanceList, "Trust tier", claim.trustTier);
    appendClaimDefinition(provenanceList, "Freshness", claim.stale ? "stale" : "current");
    appendClaimDefinition(provenanceList, "Document", claimDocumentLabel(claim), { mono: true });
    if (claim.verification) {
      appendClaimDefinition(provenanceList, "Verification", [claim.verification.by, claim.verification.method, claim.verification.at].filter(Boolean).join(" · "));
    }
    appendClaimDefinition(provenanceList, "Evidence refs", claim.verification?.evidenceRefs || []);
    appendClaimDefinition(provenanceList, "Decisions", (claim.decisions || []).map(function (decision) {
      return [decision.action, decision.by, decision.at, decision.reason].filter(Boolean).join(" · ");
    }).join("\n"), { multiline: true });
    appendClaimDefinition(provenanceList, "Dependents", claim.dependents || []);
    provenance.append(provenanceList);

    const metadata = createClaimsElement("section", "claims-inspector-metadata");
    metadata.append(createClaimsElement("h3", "claims-inspector-section-title", "Evidence and metadata"));
    const metadataBody = createClaimsElement("div", "claims-inspector-metadata-body");
    metadataBody.append(definitions);
    if ((claim.issues || []).length > 0) {
      const issues = createClaimsElement("section", "claims-inspector-section claims-issues");
      issues.append(createClaimsElement("h3", "", "Needs attention"));
      const list = document.createElement("ul");
      claim.issues.forEach(function (issue) {
        list.append(createClaimsElement("li", "", issue.message));
      });
      issues.append(list);
      metadataBody.append(issues);
    }
    metadataBody.append(evidenceSection, provenance);
    metadata.append(metadataBody);
    claimsDetail.append(heading, actions, metadata);
    const relationships = createClaimRelationships(claim);
    if (relationships) {
      claimsDetail.append(relationships);
    }
    const history = createClaimHistory(claim);
    if (history) {
      claimsDetail.append(history);
    }
    const impact = createClaimImpact(claim);
    if (impact) {
      claimsDetail.append(impact);
    }
  }

  function relationClaimFor(claim, relation) {
    if (relation.key) {
      return claimByKey(relation.key);
    }
    const knowledgeBase = String(claim?.knowledgeBase || "");
    return claimsData.claims.find(function (candidate) {
      return candidate.id === relation.id && String(candidate.knowledgeBase || "") === knowledgeBase;
    }) || null;
  }

  function createClaimRelationships(claim) {
    const relations = incomingClaimRelations(claim).concat(claimRelationEntries(claim));
    if (relations.length === 0) {
      return null;
    }
    const section = createClaimsElement("details", "claims-inspector-relationships");
    section.append(createClaimsElement("summary", "", "Relationships (" + relations.length + ")"));
    const relationList = createClaimsElement("div", "claims-relationship-list");
    relations.forEach(function (relation) {
      const related = relationClaimFor(claim, relation);
      const row = createClaimsElement(related ? "button" : "div", "claims-relationship-row");
      row.dataset.direction = relation.direction;
      row.append(createClaimsElement("span", "claims-relationship-label", relation.label));
      row.append(createClaimsElement("strong", "", related ? claimStatement(related) : relation.id));
      if (related) {
        row.type = "button";
        row.addEventListener("click", function () {
          selectClaim(claimIdentity(related), true);
          window.requestAnimationFrame(function () {
            const nextHeading = claimsDetail?.querySelector("h2");
            if (nextHeading) {
              nextHeading.tabIndex = -1;
              nextHeading.focus();
            }
          });
        });
      } else {
        row.classList.add("is-unresolved");
      }
      relationList.append(row);
    });
    section.append(relationList);
    return section;
  }

  function selectClaim(key, updateLocation) {
    const claim = claimByKey(key);
    if (!claim) {
      return;
    }
    selectedClaimKey = claimIdentity(claim);
    claimsInspectorContext = null;
    renderClaimsWorkspace();
    if (updateLocation) {
      syncWorkspaceViewURL(true);
    }
  }

  function renderClaimsWorkspace() {
    if (!claimsWorkspace) {
      return;
    }
    populateClaimsFilters();
    const matches = filteredClaims();
    if (!claimByKey(selectedClaimKey) || !matches.some(function (claim) { return claimIdentity(claim) === selectedClaimKey; })) {
      selectedClaimKey = matches.length > 0 ? claimIdentity(matches[0]) : "";
    }
    const selected = claimByKey(selectedClaimKey);
    renderClaimsList(matches);
    if (claimsInspectorContext) {
      renderClaimsContextInspector(claimsInspectorContext);
    } else {
      renderClaimInspector(selected);
    }
    const summary = document.querySelector("[data-claims-summary]");
    if (summary) {
      const count = matches.length === claimsData.claims.length
        ? claimsData.claims.length + " claims"
        : matches.length + " of " + claimsData.claims.length;
      const scopedCount = projectionHasProblems(claimsData)
        ? count + " · " + projectionFailureSummary(claimsData)
        : count;
      summary.textContent = activeClaimsKnowledgeBase ? activeClaimsKnowledgeBase + " · " + scopedCount : scopedCount;
    }
    if (claimsTitle) {
      claimsTitle.textContent = activeClaimsKnowledgeBase ? "Claims in " + activeClaimsKnowledgeBase : "Claims";
    }
  }

  function bindClaimsWorkspace() {
    if (!claimsFilters || claimsFilters.dataset.bound === "true") {
      return;
    }
    claimsFilters.dataset.bound = "true";
    const render = function () { renderClaimsWorkspace(); };
    claimsQuery?.addEventListener("input", render);
    claimsFilters.querySelectorAll("select").forEach(function (select) {
      select.addEventListener("change", function () {
        const key = select.dataset.claimsFilter;
        claimsFilters.querySelectorAll('[data-claims-filter="' + key + '"]').forEach(function (peer) {
          peer.value = select.value;
        });
        render();
      });
    });
    claimsFilters.addEventListener("reset", function () {
      window.requestAnimationFrame(renderClaimsWorkspace);
    });
    renderClaimsWorkspace();
  }

  function organizeSidebarControls() {
    const graphSlot = fileSidebar?.querySelector("[data-sidebar-graph-slot]");
    const settingsSlot = fileSidebar?.querySelector("[data-sidebar-settings-slot]");
    if (graphSlot && graphViewToggle) {
      const sidebarGraphToggle = graphViewToggle.cloneNode(true);
      sidebarGraphToggle.removeAttribute("data-graph-view-toggle");
      sidebarGraphToggle.dataset.sidebarGraphToggle = "";
      sidebarGraphToggle.addEventListener("click", function () {
        graphViewToggle.click();
      });
      graphSlot.replaceWith(sidebarGraphToggle);
    }
    if (settingsSlot && settings) {
      settingsSlot.replaceWith(settings);
    }
  }

  function readThemePreference() {
    const stored = readStoredJSON(themeStorageKey);
    if (stored && typeof stored === "object" && !Array.isArray(stored)) {
      return normalizeThemePreference(stored);
    }
    return normalizeThemePreference({ preset: defaultThemePreset, custom: customThemeDefaults });
  }

  function normalizeThemePreference(value) {
    const preset = themePresets.includes(value.preset) ? value.preset : defaultThemePreset;
    const custom = Object.assign({}, customThemeDefaults);
    Object.keys(customThemeDefaults).forEach(function (key) {
      if (isHexColor(value.custom && value.custom[key])) {
        custom[key] = value.custom[key].toLowerCase();
      }
    });
    return { preset: preset, custom: custom };
  }

  function saveThemePreference(preference) {
    const normalized = normalizeThemePreference(preference);
    const serialized = JSON.stringify(normalized);
    try {
      window.localStorage.setItem(themeStorageKey, serialized);
    } catch {
      // Browser storage can be disabled in private or file-export contexts.
    }
    writeCookie(themeStorageKey, serialized);
  }

  function readFrontmatterPreference() {
    const stored = readStoredJSON(frontmatterStorageKey);
    if (stored && typeof stored === "object" && !Array.isArray(stored) && typeof stored.visible === "boolean") {
      return stored.visible;
    }
    return true;
  }

  function saveFrontmatterPreference(visible) {
    const serialized = JSON.stringify({ visible: Boolean(visible) });
    try {
      window.localStorage.setItem(frontmatterStorageKey, serialized);
    } catch {
      // Browser storage can be disabled in private or file-export contexts.
    }
    writeCookie(frontmatterStorageKey, serialized);
  }

  function readAccessibilityPreference() {
    const stored = readStoredJSON(accessibilityStorageKey);
    if (stored && typeof stored === "object" && !Array.isArray(stored)) {
      return normalizeAccessibilityPreference(stored);
    }
    return normalizeAccessibilityPreference(defaultAccessibilityPreference);
  }

  function normalizeAccessibilityPreference(value) {
    const source = value && typeof value === "object" ? value : {};
    return {
      font: Object.prototype.hasOwnProperty.call(accessibilityFonts, source.font) ? source.font : defaultAccessibilityPreference.font,
      size: Object.prototype.hasOwnProperty.call(accessibilityFontSizes, source.size) ? source.size : defaultAccessibilityPreference.size,
      spacing: Object.prototype.hasOwnProperty.call(accessibilityLineHeights, source.spacing) ? source.spacing : defaultAccessibilityPreference.spacing,
      motion: ["system", "reduced", "full"].includes(source.motion) ? source.motion : defaultAccessibilityPreference.motion,
      readableLineLength: typeof source.readableLineLength === "boolean" ? source.readableLineLength : defaultAccessibilityPreference.readableLineLength,
      highContrast: typeof source.highContrast === "boolean" ? source.highContrast : defaultAccessibilityPreference.highContrast,
      underlineLinks: typeof source.underlineLinks === "boolean" ? source.underlineLinks : defaultAccessibilityPreference.underlineLinks
    };
  }

  function saveAccessibilityPreference(preference) {
    const normalized = normalizeAccessibilityPreference(preference);
    const serialized = JSON.stringify(normalized);
    try {
      window.localStorage.setItem(accessibilityStorageKey, serialized);
    } catch {
      // Browser storage can be disabled in private or file-export contexts.
    }
    writeCookie(accessibilityStorageKey, serialized);
  }

  function applyAccessibilityPreference(preference) {
    const normalized = normalizeAccessibilityPreference(preference);
    const size = accessibilityFontSizes[normalized.size];
    const spacing = accessibilityLineHeights[normalized.spacing];
    document.documentElement.dataset.viewerFont = normalized.font;
    document.documentElement.dataset.viewerFontSize = normalized.size;
    document.documentElement.dataset.viewerMotion = normalized.motion;
    document.documentElement.dataset.viewerContrast = normalized.highContrast ? "high" : "normal";
    document.documentElement.dataset.viewerUnderlines = normalized.underlineLinks ? "on" : "off";
    document.documentElement.style.setProperty("--ok-font-body", accessibilityFonts[normalized.font]);
    document.documentElement.style.setProperty("--ok-document-font-size", size.value);
    document.documentElement.style.setProperty("--ok-document-scale", size.scale);
    document.documentElement.style.setProperty("--ok-document-line-height", spacing.value);
    document.documentElement.style.setProperty("--ok-document-letter-spacing", spacing.letterSpacing);
    document.documentElement.style.setProperty("--ok-note-body-max-width", normalized.readableLineLength ? "70ch" : "none");
    document.body.classList.toggle("is-high-contrast", normalized.highContrast);
    document.body.classList.toggle("is-links-underlined", normalized.underlineLinks);
    syncAccessibilityControls(normalized);
  }

  function syncAccessibilityControls(preference) {
    if (accessibilityFont) {
      accessibilityFont.value = preference.font;
    }
    if (accessibilitySize) {
      accessibilitySize.value = preference.size;
    }
    if (accessibilitySpacing) {
      accessibilitySpacing.value = preference.spacing;
    }
    if (accessibilityMotion) {
      accessibilityMotion.value = preference.motion;
    }
    if (readableLineLength) {
      readableLineLength.checked = preference.readableLineLength;
    }
    if (highContrast) {
      highContrast.checked = preference.highContrast;
    }
    if (underlineLinks) {
      underlineLinks.checked = preference.underlineLinks;
    }
  }

  function motionIsReduced() {
    const preference = document.documentElement.dataset.viewerMotion || "system";
    return preference === "reduced" || (preference === "system" && reduceMotion.matches);
  }

  function applyFrontmatterPreference(visible) {
    document.body.classList.toggle("is-frontmatter-hidden", !visible);
    if (frontmatterVisibility) {
      frontmatterVisibility.checked = visible;
    }
  }

  function isHexColor(value) {
    return /^#[0-9a-f]{6}$/i.test(String(value || ""));
  }

  function hexToRGB(value) {
    const hex = isHexColor(value) ? value.slice(1) : customThemeDefaults.accent.slice(1);
    return [
      parseInt(hex.slice(0, 2), 16),
      parseInt(hex.slice(2, 4), 16),
      parseInt(hex.slice(4, 6), 16)
    ];
  }

  function colorMix(hex, otherHex, amount) {
    const base = hexToRGB(hex);
    const other = hexToRGB(otherHex);
    const next = base.map(function (component, index) {
      return Math.round(component + (other[index] - component) * amount);
    });
    return "#" + next.map(function (component) {
      return component.toString(16).padStart(2, "0");
    }).join("");
  }

  function readableCodeBlockText(background) {
    return colorLuminance(background) > 0.48 ? "#121715" : "#f3f7f4";
  }

  function colorLuminance(value) {
    const rgb = hexToRGB(value);
    return (0.2126 * rgb[0] + 0.7152 * rgb[1] + 0.0722 * rgb[2]) / 255;
  }

  function applyThemePreference(preference) {
    const normalized = normalizeThemePreference(preference);
    document.documentElement.dataset.viewerTheme = normalized.preset;
    const darkCustomTheme = normalized.preset === "custom" && colorLuminance(normalized.custom.surface) <= 0.48;
    document.documentElement.style.colorScheme = normalized.preset === "night" || darkCustomTheme ? "dark" : "light";
    clearCustomThemeVariables();
    if (normalized.preset === "custom") {
      applyCustomThemeVariables(normalized.custom);
    }
    syncThemeControls(normalized);
    scheduleMermaidThemeRender();
  }

  function clearCustomThemeVariables() {
    Object.keys(customThemeVariables).forEach(function (key) {
      customThemeVariables[key].forEach(function (name) {
        document.documentElement.style.removeProperty(name);
      });
    });
    [
      "--ok-color-accent-rgb",
      "--ok-color-accent-soft",
      "--ok-color-accent-softer",
      "--ok-color-accent-selected",
      "--ok-color-accent-focus",
      "--ok-color-accent-focus-strong",
      "--ok-color-accent-border",
      "--ok-color-accent-border-strong",
      "--ok-color-shadow",
      "--ok-color-code-inline-bg",
      "--ok-color-code-block-text",
      "--ok-color-control-hover-bg",
      "--ok-color-sidebar-row",
      "--ok-color-sidebar-tree-hover-bg",
      "--ok-color-search-result-hover-bg",
      "--ok-color-editor-menu-item-hover-bg",
      "--ok-color-graph-edge",
      "--ok-color-graph-edge-muted",
      "--ok-color-graph-edge-active"
    ].forEach(function (name) {
      document.documentElement.style.removeProperty(name);
    });
  }

  function applyCustomThemeVariables(custom) {
    Object.keys(customThemeVariables).forEach(function (key) {
      customThemeVariables[key].forEach(function (name) {
        document.documentElement.style.setProperty(name, custom[key]);
      });
    });
    const accentRGB = hexToRGB(custom.accent).join(", ");
    document.documentElement.style.setProperty("--ok-color-accent-rgb", accentRGB);
    document.documentElement.style.setProperty("--ok-color-accent-soft", "rgba(" + accentRGB + ", .11)");
    document.documentElement.style.setProperty("--ok-color-accent-softer", "rgba(" + accentRGB + ", .065)");
    document.documentElement.style.setProperty("--ok-color-accent-selected", "rgba(" + accentRGB + ", .09)");
    document.documentElement.style.setProperty("--ok-color-accent-focus", "rgba(" + accentRGB + ", .12)");
    document.documentElement.style.setProperty("--ok-color-accent-focus-strong", "rgba(" + accentRGB + ", .18)");
    document.documentElement.style.setProperty("--ok-color-accent-border", "rgba(" + accentRGB + ", .35)");
    document.documentElement.style.setProperty("--ok-color-accent-border-strong", "rgba(" + accentRGB + ", .5)");
    document.documentElement.style.setProperty("--ok-color-shadow", "rgba(" + hexToRGB(custom.text).join(", ") + ", .1)");
    document.documentElement.style.setProperty("--ok-color-code-inline-bg", colorMix(custom.surface, custom.accent, 0.1));
    document.documentElement.style.setProperty("--ok-color-code-block-text", readableCodeBlockText(custom.text));
    document.documentElement.style.setProperty("--ok-color-control-hover-bg", colorMix(custom.page, custom.text, 0.08));
    document.documentElement.style.setProperty("--ok-color-sidebar-row", colorMix(custom.page, custom.text, 0.11));
    document.documentElement.style.setProperty("--ok-color-sidebar-tree-hover-bg", colorMix(custom.page, custom.accent, 0.13));
    document.documentElement.style.setProperty("--ok-color-search-result-hover-bg", colorMix(custom.surface, custom.accent, 0.08));
    document.documentElement.style.setProperty("--ok-color-editor-menu-item-hover-bg", colorMix(custom.surface, custom.accent, 0.08));
    const mutedRGB = hexToRGB(custom.muted).join(", ");
    document.documentElement.style.setProperty("--ok-color-graph-edge", "rgba(" + mutedRGB + ", .28)");
    document.documentElement.style.setProperty("--ok-color-graph-edge-muted", "rgba(" + mutedRGB + ", .11)");
    document.documentElement.style.setProperty("--ok-color-graph-edge-active", "rgba(" + accentRGB + ", .78)");
  }

  function syncThemeControls(preference) {
    document.querySelectorAll("[data-theme-option]").forEach(function (button) {
      const selected = button.dataset.themeOption === preference.preset;
      button.classList.toggle("is-selected", selected);
      button.setAttribute("aria-checked", selected ? "true" : "false");
    });
    if (customThemeFields) {
      customThemeFields.hidden = preference.preset !== "custom";
      customThemeFields.querySelectorAll("[data-theme-custom-value]").forEach(function (input) {
        const key = input.dataset.themeCustomValue;
        if (preference.custom[key]) {
          input.value = preference.custom[key];
        }
      });
    }
  }

  function minPanelWidth() {
    return Math.min(360, Math.max(260, window.innerWidth - 24));
  }

  function singlePanelHorizontalGap() {
    return window.innerWidth <= 680 ? 12 : Math.max(22, (window.innerWidth - 1180) / 2);
  }

  function isSingleCenteredPanel(panel) {
    const all = panels();
    return Boolean(panel && all.length === 1 && all[0] === panel);
  }

  function maxPanelWidth(panel) {
    if (isSingleCenteredPanel(panel)) {
      return Math.max(minPanelWidth(), window.innerWidth - singlePanelHorizontalGap() * 2);
    }
    return Math.max(defaultPanelWidth(), 1180);
  }

  function defaultPanelWidth() {
    return Math.max(minPanelWidth(), cssLengthPixels("var(--ok-note-panel-default-width)", 650));
  }

  function cssLengthPixels(value, fallback) {
    const probe = document.createElement("div");
    probe.style.position = "absolute";
    probe.style.left = "-10000px";
    probe.style.top = "-10000px";
    probe.style.visibility = "hidden";
    probe.style.pointerEvents = "none";
    probe.style.width = value;
    document.body.append(probe);
    const width = probe.getBoundingClientRect().width;
    probe.remove();
    return Number.isFinite(width) && width > 0 ? width : fallback;
  }

  function normalizePanelWidth(value, panel) {
    const numeric = Number(value);
    if (!Number.isFinite(numeric)) {
      return null;
    }
    return Math.round(clamp(numeric, minPanelWidth(), maxPanelWidth(panel)));
  }

  function minSidebarWidth() {
    return cssLengthPixels("var(--ok-sidebar-min-width)", 280);
  }

  function maxSidebarWidth() {
    const configuredMaximum = cssLengthPixels("var(--ok-sidebar-max-width)", 560);
    return Math.max(minSidebarWidth(), Math.min(configuredMaximum, window.innerWidth - 320));
  }

  function defaultSidebarWidth() {
    return cssLengthPixels("var(--ok-sidebar-width)", window.innerWidth * 0.25);
  }

  function normalizeSidebarWidth(value) {
    const numeric = Number(value);
    if (!Number.isFinite(numeric) || numeric <= 0) {
      return null;
    }
    return Math.round(clamp(numeric, minSidebarWidth(), maxSidebarWidth()));
  }

  function appliedSidebarWidth(value) {
    return normalizeSidebarWidth(value) || normalizeSidebarWidth(defaultSidebarWidth()) || minSidebarWidth();
  }

  function applySidebarWidth(value) {
    const width = appliedSidebarWidth(value);
    document.body.style.setProperty("--ok-sidebar-user-width", width + "px");
    if (sidebarResizeHandle) {
      sidebarResizeHandle.setAttribute("aria-valuemin", String(Math.round(minSidebarWidth())));
      sidebarResizeHandle.setAttribute("aria-valuemax", String(Math.round(maxSidebarWidth())));
      sidebarResizeHandle.setAttribute("aria-valuenow", String(width));
    }
    queueWorkspaceRailUpdate();
    return width;
  }

  function setSidebarWidth(value) {
    sidebarWidth = appliedSidebarWidth(value);
    return applySidebarWidth(sidebarWidth);
  }

  function savedPanelWidth(panel) {
    return normalizePanelWidth(panelWidths[panel.dataset.notePath], panel);
  }

  function applyPanelWidth(panel) {
    const width = savedPanelWidth(panel);
    if (!width) {
      panel.style.removeProperty("--note-panel-width");
      delete panel.dataset.panelWidth;
      return;
    }
    panel.style.setProperty("--note-panel-width", width + "px");
    panel.dataset.panelWidth = String(width);
  }

  function setSidebarOpen(open) {
    document.body.classList.toggle("is-sidebar-open", open);
    if (fileSidebar) {
      fileSidebar.setAttribute("aria-hidden", open ? "false" : "true");
      fileSidebar.inert = !open;
    }
    if (sidebarToggle) {
      sidebarToggle.setAttribute("aria-expanded", open ? "true" : "false");
      sidebarToggle.setAttribute("aria-label", open ? "Close file explorer" : "Open file explorer");
    }
    if (open) {
      syncKnowledgeTrees(activePanel()?.dataset.notePath || currentStack()[0] || "", true, activePanel()?.dataset.knowledgeBase);
    }
  }

  function toggleSidebar() {
    setSidebarOpen(!document.body.classList.contains("is-sidebar-open"));
  }

  function knowledgeBasePrefix(name) {
    const normalizedName = String(name || "").trim();
    if (!normalizedName || normalizedName === currentKnowledgeBase) {
      return linkPrefix;
    }
    const links = document.querySelectorAll("[data-tree-path][data-knowledge-base]");
    for (const link of links) {
      if (String(link.dataset.knowledgeBase || "").trim() !== normalizedName) {
        continue;
      }
      try {
        const url = new URL(link.getAttribute("href") || link.href, window.location.href);
        const marker = "/file/";
        const index = url.pathname.indexOf(marker);
        if (index >= 0) {
          return url.pathname.slice(0, index);
        }
      } catch {
        return linkPrefix;
      }
    }
    return linkPrefix;
  }

  function noteTarget(path, knowledgeBase) {
    return {
      path: String(path || "index.md"),
      knowledgeBase: String(knowledgeBase || currentKnowledgeBase || "").trim(),
    };
  }

  function noteTargetFromHref(href, sourcePath, sourceKnowledgeBase) {
    if (isStaticBundle()) {
      const path = staticNotePathFromHref(href, sourcePath);
      return path ? noteTarget(path, sourceKnowledgeBase) : null;
    }

    let url;
    try {
      url = new URL(href, window.location.href);
    } catch {
      return null;
    }

    if (url.origin !== window.location.origin) {
      return null;
    }

    const prefixes = knowledgeBaseNames().map(function (name) {
      return { knowledgeBase: name, prefix: knowledgeBasePrefix(name) + "/file/" };
    });
    prefixes.push({ knowledgeBase: currentKnowledgeBase, prefix: serverFilePrefix() });
    prefixes.sort(function (left, right) { return right.prefix.length - left.prefix.length; });
    const match = prefixes.find(function (candidate) {
      return url.pathname.startsWith(candidate.prefix);
    });
    if (!match) {
      return null;
    }

    const raw = url.pathname.slice(match.prefix.length) || "index.md";
    if (!isPanelPreviewPath(raw)) {
      return null;
    }
    try {
      return noteTarget(decodeURIComponent(raw), match.knowledgeBase || sourceKnowledgeBase);
    } catch {
      return noteTarget(raw, match.knowledgeBase || sourceKnowledgeBase);
    }
  }

  function notePathFromHref(href, sourcePath, sourceKnowledgeBase) {
    return noteTargetFromHref(href, sourcePath, sourceKnowledgeBase)?.path || null;
  }

  function encodedNoteURL(prefix, path) {
    return prefix + path.split("/").map(encodeURIComponent).join("/");
  }

  function isPanelPreviewPath(path) {
    return /\.(md|markdown|go|js|mjs|cjs|jsx|ts|mts|cts|tsx|json|jsonc|html|htm|css|sh|bash|zsh|py|rb|rs|java|kt|kts|swift|sql|yml|yaml|toml|xml|svg|txt|log|csv|tsv|ini|env|gitignore|dockerignore)$/i.test(String(path || "").split("?")[0].split("#")[0]);
  }

  function fileURL(path, knowledgeBase) {
    if (isStaticBundle()) {
      return staticRelativeURL(path);
    }
    return encodedNoteURL(serverFilePrefix(knowledgeBase), path);
  }

  function apiURL(path, knowledgeBase) {
    return encodedNoteURL(serverAPIPrefix(knowledgeBase), path);
  }

  function serverFilePrefix(knowledgeBase) {
    return knowledgeBasePrefix(knowledgeBase) + "/file/";
  }

  function serverAPIPrefix(knowledgeBase) {
    return knowledgeBasePrefix(knowledgeBase) + "/api/file/";
  }

  function normalizeLinkPrefix(value) {
    const trimmed = String(value || "").replace(/\/+$/, "");
    if (!trimmed) {
      return "";
    }
    return trimmed.startsWith("/") ? trimmed : "/" + trimmed;
  }

  function readStaticNotes() {
    const shared = window.OpenKnowledgeStaticData?.notes;
    if (Array.isArray(shared)) {
      return shared;
    }
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

  function collectKnownNotePaths() {
    const paths = new Set();
    staticNotes.forEach(function (note) {
      if (note.path) {
        paths.add(note.path);
      }
    });
    document.querySelectorAll("[data-tree-path]").forEach(function (link) {
      if (link.dataset.treePath && (!currentKnowledgeBase || !link.dataset.knowledgeBase || link.dataset.knowledgeBase === currentKnowledgeBase)) {
        paths.add(link.dataset.treePath);
      }
    });
    return paths;
  }

  function prepareKnowledgeTrees() {
    document.querySelectorAll(".knowledge-tree").forEach(function (tree) {
      if (tree.dataset.treePrepared === "true") {
        return;
      }
      tree.dataset.treePrepared = "true";
      const directoryStack = [];
      Array.from(tree.querySelectorAll(".tree-row")).forEach(function (row) {
        const indent = treeRowIndent(row);
        while (directoryStack.length && directoryStack[directoryStack.length - 1].indent >= indent) {
          directoryStack.pop();
        }
        row.dataset.treeParentPath = directoryStack[directoryStack.length - 1]?.path || "";
        if (!row.classList.contains("tree-directory")) {
          return;
        }
        const name = String(row.textContent || "").trim();
        const path = directoryStack.concat([{ name: name }]).map(function (item) {
          return item.name;
        }).join("/");
        row.dataset.treeDirectoryPath = path;
        row.setAttribute("aria-expanded", "false");
        row.tabIndex = 0;
        row.title = "Expand " + path;
        const icon = controlIcon("chevron-down", "tree-directory-icon");
        icon.setAttribute("aria-hidden", "true");
        row.prepend(icon);
        row.addEventListener("click", function () {
          setTreeDirectoryExpanded(row, row.getAttribute("aria-expanded") !== "true");
        });
        row.addEventListener("keydown", function (event) {
          if (event.key === "ArrowRight") {
            event.preventDefault();
            setTreeDirectoryExpanded(row, true);
            return;
          }
          if (event.key === "ArrowLeft") {
            event.preventDefault();
            setTreeDirectoryExpanded(row, false);
            return;
          }
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            setTreeDirectoryExpanded(row, row.getAttribute("aria-expanded") !== "true");
          }
        });
        directoryStack.push({ indent: indent, name: name, path: path });
      });
      syncTreeVisibility(tree);
    });
    ensureSidebarCollapseControl();
  }

  function prepareKnowledgeBases() {
    applyKnowledgeBaseColors();
    if (knowledgeBasesToggle && knowledgeBaseList) {
      knowledgeBasesToggle.addEventListener("click", function () {
        const expanded = knowledgeBasesToggle.getAttribute("aria-expanded") !== "false";
        knowledgeBasesToggle.setAttribute("aria-expanded", expanded ? "false" : "true");
        knowledgeBaseList.hidden = expanded;
      });
    }
    document.querySelectorAll("[data-knowledge-base-disclosure]").forEach(function (disclosure) {
      disclosure.addEventListener("click", function () {
        const expanded = disclosure.getAttribute("aria-expanded") === "true";
        disclosure.setAttribute("aria-expanded", expanded ? "false" : "true");
        const tree = document.getElementById(disclosure.getAttribute("aria-controls"));
        if (tree) {
          tree.hidden = expanded;
        }
      });
    });
    document.querySelectorAll("[data-knowledge-base-color]").forEach(function (input) {
      input.addEventListener("input", function () {
        const group = input.closest("[data-knowledge-base-name]");
        const name = String(group?.dataset.knowledgeBaseName || "").trim();
        if (!name || !isHexColor(input.value)) {
          return;
        }
        knowledgeBaseColorOverrides[name] = input.value.toLowerCase();
        saveKnowledgeBaseColors();
        applyKnowledgeBaseColors();
        window.dispatchEvent(new CustomEvent("openknowledge:knowledge-base-color"));
      });
    });
    bindKnowledgeBaseDialog();
  }

  function bindKnowledgeBaseDialog() {
    const trigger = document.querySelector("[data-knowledge-base-connect]");
    if (!trigger || !knowledgeBaseDialog || !knowledgeBaseForm) {
      return;
    }
    const status = knowledgeBaseForm.querySelector("[data-knowledge-base-form-status]");
    const submit = knowledgeBaseForm.querySelector("[type='submit']");
    const pathInput = knowledgeBaseForm.elements.namedItem("path");
    trigger.addEventListener("click", function () {
      knowledgeBaseForm.reset();
      renderKnowledgeBaseFormStatus(status, "", "");
      knowledgeBaseDialog.showModal();
      window.requestAnimationFrame(function () {
        pathInput?.focus();
      });
    });
    knowledgeBaseDialog.querySelectorAll("[data-knowledge-base-dialog-close]").forEach(function (button) {
      button.addEventListener("click", function () {
        knowledgeBaseDialog.close();
        trigger.focus();
      });
    });
    knowledgeBaseForm.addEventListener("submit", async function (event) {
      event.preventDefault();
      if (!knowledgeBaseForm.reportValidity()) {
        return;
      }
      const nameInput = knowledgeBaseForm.elements.namedItem("name");
      const writeInput = knowledgeBaseForm.elements.namedItem("writeAccess");
      const payload = {
        path: String(pathInput?.value || "").trim(),
        name: String(nameInput?.value || "").trim(),
        access: writeInput?.checked ? "write" : "read",
      };
      submit.disabled = true;
      renderKnowledgeBaseFormStatus(status, "Connecting…", "");
      try {
        const response = await fetch("/api/knowledge-bases", {
          method: "POST",
          headers: { "Content-Type": "application/json", "Accept": "application/json" },
          body: JSON.stringify(payload),
        });
        const result = await response.json().catch(function () { return {}; });
        if (response.ok && result.url) {
          renderKnowledgeBaseFormStatus(status, "Connected " + (result.name || "knowledge base") + ".", "");
          window.location.assign(result.url);
          return;
        }
        if (result.status === "needs_setup" && result.command) {
          renderKnowledgeBaseSetupStatus(status, result.message, result.command);
          return;
        }
        renderKnowledgeBaseFormStatus(status, result.message || "Could not connect this folder. Check the path and try again.", "error");
      } catch {
        renderKnowledgeBaseFormStatus(status, "Could not reach the local viewer. Restart okn view and try again.", "error");
      } finally {
        submit.disabled = false;
      }
    });
  }

  function renderKnowledgeBaseFormStatus(status, message, kind) {
    if (!status) {
      return;
    }
    status.replaceChildren();
    status.textContent = message;
    if (kind) {
      status.dataset.kind = kind;
    } else {
      delete status.dataset.kind;
    }
  }

  function renderKnowledgeBaseSetupStatus(status, message, command) {
    renderKnowledgeBaseFormStatus(status, "", "");
    const text = document.createElement("span");
    text.textContent = message || "Create the knowledge base, then connect this folder again.";
    const code = document.createElement("code");
    code.className = "knowledge-base-setup-command";
    code.textContent = command;
    status.append(text, code);
  }

  function treeRowIndent(row) {
    const inline = row.style.getPropertyValue("--indent");
    const value = parseFloat(inline);
    return Number.isFinite(value) ? value : 0;
  }

  function setTreeDirectoryExpanded(directory, expanded) {
    directory.setAttribute("aria-expanded", expanded ? "true" : "false");
    directory.title = (expanded ? "Collapse " : "Expand ") + directory.dataset.treeDirectoryPath;
    const tree = directory.closest(".knowledge-tree");
    if (tree) {
      syncTreeVisibility(tree);
    }
  }

  function syncTreeVisibility(tree) {
    const directories = Object.create(null);
    tree.querySelectorAll("[data-tree-directory-path]").forEach(function (directory) {
      directories[directory.dataset.treeDirectoryPath] = directory;
    });
    tree.querySelectorAll(".tree-row").forEach(function (row) {
      const parentPath = row.dataset.treeParentPath || "";
      const parentParts = parentPath.split("/").filter(Boolean);
      let current = "";
      let hidden = false;
      parentParts.forEach(function (part) {
        current = current ? current + "/" + part : part;
        const parent = directories[current];
        if (parent && parent.getAttribute("aria-expanded") !== "true") {
          hidden = true;
        }
      });
      row.hidden = hidden;
    });
  }

  function collapseKnowledgeTrees() {
    document.querySelectorAll("[data-tree-directory-path]").forEach(function (directory) {
      directory.setAttribute("aria-expanded", "false");
      directory.title = "Expand " + directory.dataset.treeDirectoryPath;
    });
    document.querySelectorAll(".knowledge-tree").forEach(syncTreeVisibility);
    document.querySelectorAll("[data-knowledge-base-disclosure]").forEach(function (disclosure) {
      disclosure.setAttribute("aria-expanded", "false");
      const tree = document.getElementById(disclosure.getAttribute("aria-controls"));
      if (tree) {
        tree.hidden = true;
      }
    });
  }

  function syncKnowledgeTrees(path, scrollCurrent, knowledgeBase) {
    const normalizedPath = String(path || "");
    const normalizedKnowledgeBase = String(knowledgeBase || currentKnowledgeBase || "");
    const directoryParts = normalizedPath.split("/").slice(0, -1);
    const ancestors = new Set();
    directoryParts.forEach(function (_part, index) {
      ancestors.add(directoryParts.slice(0, index + 1).join("/"));
    });
    document.querySelectorAll(".knowledge-tree").forEach(function (tree) {
      const treeKnowledgeBase = tree.closest("[data-knowledge-base-name]")?.dataset.knowledgeBaseName || tree.dataset.knowledgeBase || "";
      const sameKnowledgeBase = !normalizedKnowledgeBase || !treeKnowledgeBase || treeKnowledgeBase === normalizedKnowledgeBase;
      tree.querySelectorAll("[data-tree-directory-path]").forEach(function (directory) {
        if (sameKnowledgeBase && ancestors.has(directory.dataset.treeDirectoryPath)) {
          directory.setAttribute("aria-expanded", "true");
          directory.title = "Collapse " + directory.dataset.treeDirectoryPath;
        }
      });
      tree.querySelectorAll("[data-tree-path]").forEach(function (link) {
        const current = sameKnowledgeBase && link.dataset.treePath === normalizedPath;
        link.classList.toggle("is-current-file", current);
        if (current) {
          link.setAttribute("aria-current", "page");
          link.title = link.dataset.treePath;
        } else {
          link.removeAttribute("aria-current");
          link.title = navigationModeTitle();
        }
      });
      syncTreeVisibility(tree);
    });
    document.querySelectorAll("[data-knowledge-base-name]").forEach(function (group) {
      const active = group.dataset.knowledgeBaseName === normalizedKnowledgeBase;
      group.classList.toggle("is-active", active);
      if (!active) {
        return;
      }
      const disclosure = group.querySelector("[data-knowledge-base-disclosure]");
      const content = disclosure ? document.getElementById(disclosure.getAttribute("aria-controls")) : null;
      disclosure?.setAttribute("aria-expanded", "true");
      if (content) {
        content.hidden = false;
      }
    });
    if (scrollCurrent && fileSidebar) {
      const current = Array.from(fileSidebar.querySelectorAll("[data-tree-path]")).find(function (link) {
        return link.dataset.treePath === normalizedPath && (!normalizedKnowledgeBase || !link.dataset.knowledgeBase || link.dataset.knowledgeBase === normalizedKnowledgeBase);
      });
      if (current) {
        window.requestAnimationFrame(function () {
          current.scrollIntoView({ block: "center", behavior: motionIsReduced() ? "auto" : "smooth" });
        });
      }
    }
  }

  function ensureSidebarCollapseControl() {
    const actions = fileSidebar?.querySelector("[data-sidebar-tree-actions]");
    if (!actions) {
      return;
    }
    actions.classList.add("file-sidebar-actions");
    let collapse = actions.querySelector("[data-sidebar-collapse]");
    if (!collapse) {
      collapse = document.createElement("button");
      collapse.type = "button";
      collapse.className = "file-sidebar-icon-action";
      collapse.dataset.sidebarCollapse = "";
      collapse.setAttribute("aria-label", "Collapse all");
      collapse.title = "Collapse all";
      collapse.append(controlIcon("collapse", "file-sidebar-collapse-icon"));
      actions.append(collapse);
    }
    if (collapse.dataset.collapseBound === "true") {
      return;
    }
    collapse.dataset.collapseBound = "true";
    collapse.addEventListener("click", collapseKnowledgeTrees);
  }

  function noteIndexPath(parts) {
    const base = parts.join("/");
    return [base + "/index.md", base + "/index.markdown"].find(function (candidate) {
      return knownNotePaths.has(candidate);
    }) || "";
  }

  function createNoteBreadcrumbs(path, knowledgeBase) {
    const normalizedPath = String(path || "index.md");
    const pathParts = normalizedPath.split("/").filter(Boolean);
    const leaf = pathParts[pathParts.length - 1] || "index.md";
    const isDirectoryIndex = /^index\.(md|markdown)$/i.test(leaf) && pathParts.length > 1;
    const displayParts = isDirectoryIndex ? pathParts.slice(0, -1) : pathParts;
    const knowledgeBasePrefix = knowledgeBaseNames().length > 1
      ? String(knowledgeBase || "").trim()
      : "";
    const breadcrumbs = document.createElement("nav");
    breadcrumbs.className = "note-path note-breadcrumbs";
    breadcrumbs.dataset.noteBreadcrumbs = "";
    breadcrumbs.dataset.breadcrumbsReady = "true";
    breadcrumbs.dataset.notePathValue = normalizedPath;
    breadcrumbs.setAttribute("aria-label", "Note path");
    breadcrumbs.title = normalizedPath;

    if (knowledgeBasePrefix) {
      const prefix = document.createElement("span");
      prefix.className = "note-breadcrumb-label";
      prefix.textContent = knowledgeBasePrefix;
      breadcrumbs.append(prefix);

      const separator = document.createElement("span");
      separator.className = "note-breadcrumb-separator";
      separator.setAttribute("aria-hidden", "true");
      separator.textContent = "/";
      breadcrumbs.append(separator);
    }

    displayParts.forEach(function (part, index) {
      if (index > 0) {
        const separator = document.createElement("span");
        separator.className = "note-breadcrumb-separator";
        separator.setAttribute("aria-hidden", "true");
        separator.textContent = "/";
        breadcrumbs.append(separator);
      }

      const isLast = index === displayParts.length - 1;
      const isCurrent = isLast;
      const directoryTarget = noteIndexPath(displayParts.slice(0, index + 1));
      const targetPath = isCurrent
        ? normalizedPath
        : directoryTarget;
      const label = isCurrent && !isDirectoryIndex
        ? part.replace(/\.(md|markdown)$/i, "")
        : part;

      if (targetPath) {
        const link = document.createElement("a");
        link.className = "note-breadcrumb-link" + (isCurrent ? " note-breadcrumb-current" : "");
        link.href = fileURL(targetPath);
        link.dataset.directLink = "true";
        link.textContent = label;
        if (isCurrent) {
          link.setAttribute("aria-current", "page");
        }
        breadcrumbs.append(link);
        return;
      }

      const text = document.createElement("span");
      text.className = "note-breadcrumb-label";
      text.textContent = label;
      breadcrumbs.append(text);
    });
    return breadcrumbs;
  }

  function renderPanelBreadcrumbs(panel) {
    const existing = panel.querySelector("[data-note-breadcrumbs], .note-path");
    if (!existing || existing.dataset.breadcrumbsReady === "true") {
      return;
    }
    existing.replaceWith(createNoteBreadcrumbs(panel.dataset.notePath, panel.dataset.knowledgeBase));
  }

  function readKnowledgeGraph() {
    const shared = window.OpenKnowledgeStaticData?.graph;
    if (shared) {
      return {
        nodes: Array.isArray(shared.nodes) ? shared.nodes : [],
        edges: Array.isArray(shared.edges) ? shared.edges : [],
        status: String(shared.status || "complete"),
        failures: Array.isArray(shared.failures) ? shared.failures : [],
      };
    }
    const source = document.querySelector("[data-knowledge-graph]");
    if (!source) {
      return { nodes: [], edges: [], status: "complete", failures: [] };
    }
    try {
      const parsed = JSON.parse(source.textContent || "{}");
      return {
        nodes: Array.isArray(parsed.nodes) ? parsed.nodes : [],
        edges: Array.isArray(parsed.edges) ? parsed.edges : [],
        status: String(parsed.status || "complete"),
        failures: Array.isArray(parsed.failures) ? parsed.failures : [],
      };
    } catch {
      return { nodes: [], edges: [], status: "complete", failures: [] };
    }
  }

  function normalizeClaimsData(value) {
    const parsed = value && typeof value === "object" ? value : {};
    return {
      schemaVersion: String(parsed.schemaVersion || "1"),
      profilePresent: Boolean(parsed.profilePresent),
      claims: Array.isArray(parsed.claims) ? parsed.claims : [],
      references: Array.isArray(parsed.references) ? parsed.references : [],
      entities: Array.isArray(parsed.entities) ? parsed.entities : [],
      predicates: Array.isArray(parsed.predicates) ? parsed.predicates : [],
      sources: Array.isArray(parsed.sources) ? parsed.sources : [],
      issues: Array.isArray(parsed.issues) ? parsed.issues : [],
      status: String(parsed.status || "complete"),
      failures: Array.isArray(parsed.failures) ? parsed.failures : []
    };
  }

  function readClaimsData() {
    const empty = normalizeClaimsData({});
    const shared = window.OpenKnowledgeStaticData?.claims;
    if (shared && typeof shared === "object") {
      return normalizeClaimsData(shared);
    }
    const source = document.querySelector("[data-claims-data]");
    if (!source) {
      return empty;
    }
    try {
      return normalizeClaimsData(JSON.parse(source.textContent || "{}"));
    } catch {
      return empty;
    }
  }

  function ensureKnowledgeGraphData() {
    const source = document.querySelector("[data-knowledge-graph]");
    const dataURL = String(source?.dataset.url || "");
    if (!dataURL) {
      return Promise.resolve(knowledgeGraph);
    }
    if (!knowledgeGraphLoadPromise) {
      knowledgeGraphLoadPromise = fetch(dataURL, { headers: { "Accept": "application/json" } })
        .then(function (response) {
          if (!response.ok) {
            throw new Error("Graph request failed with status " + response.status);
          }
          return response.json();
        })
        .then(function (parsed) {
          knowledgeGraph.nodes = Array.isArray(parsed.nodes) ? parsed.nodes : [];
          knowledgeGraph.edges = Array.isArray(parsed.edges) ? parsed.edges : [];
          knowledgeGraph.status = String(parsed.status || "complete");
          knowledgeGraph.failures = Array.isArray(parsed.failures) ? parsed.failures : [];
          source.dataset.url = "";
          return knowledgeGraph;
        })
        .catch(function (error) {
          knowledgeGraphLoadPromise = null;
          throw error;
        });
    }
    return knowledgeGraphLoadPromise;
  }

  function activateClaimsData(nextData, knowledgeBase) {
    const nextKnowledgeBase = String(knowledgeBase || "").trim();
    const switchedKnowledgeBase = nextKnowledgeBase !== activeClaimsKnowledgeBase;
    Object.assign(claimsData, normalizeClaimsData(nextData));
    activeClaimsKnowledgeBase = nextKnowledgeBase;
    if (switchedKnowledgeBase) {
      selectedClaimKey = "";
      claimsInspectorContext = null;
      if (claimsQuery) {
        claimsQuery.value = "";
      }
      claimsFilters?.querySelectorAll("select").forEach(function (select) { select.value = ""; });
    }
    if (claimsFilters) {
      claimsFilters.dataset.populated = "false";
      const options = claimsFilterOptions();
      claimsFilters.querySelectorAll("[data-claims-filter]").forEach(function (select) {
        if (options[select.dataset.claimsFilter] instanceof Map) {
          Array.from(select.options).slice(1).forEach(function (option) { option.remove(); });
        }
      });
    }
    refreshClaimsViewToggles();
    return claimsData;
  }

  function ensureClaimsData(knowledgeBase) {
    const requestedKnowledgeBase = String(knowledgeBase || activeClaimsKnowledgeBase || currentKnowledgeBase || "").trim();
    const dataURL = claimsDataURL(requestedKnowledgeBase);
    if (!dataURL) {
      activeClaimsKnowledgeBase = requestedKnowledgeBase;
      return Promise.resolve(claimsData);
    }
    if (claimsDataCache.has(requestedKnowledgeBase)) {
      return Promise.resolve(activateClaimsData(claimsDataCache.get(requestedKnowledgeBase), requestedKnowledgeBase));
    }
    if (!claimsDataLoadPromises.has(requestedKnowledgeBase)) {
      const load = fetch(dataURL, { headers: { "Accept": "application/json" } })
        .then(function (response) {
          if (!response.ok) {
            throw new Error("Claims request failed with status " + response.status);
          }
          return response.json();
        })
        .then(function (parsed) {
          const normalized = normalizeClaimsData(parsed);
          claimsDataCache.set(requestedKnowledgeBase, normalized);
          return normalized;
        })
        .catch(function (error) {
          const failed = normalizeClaimsData({
            status: "failed",
            failures: [{ knowledgeBase: requestedKnowledgeBase, message: error.message }]
          });
          claimsDataCache.set(requestedKnowledgeBase, failed);
          return failed;
        })
        .finally(function () {
          claimsDataLoadPromises.delete(requestedKnowledgeBase);
        });
      claimsDataLoadPromises.set(requestedKnowledgeBase, load);
    }
    return claimsDataLoadPromises.get(requestedKnowledgeBase).then(function (loaded) {
      return activateClaimsData(loaded, requestedKnowledgeBase);
    });
  }

  const defaultGraphSettings = Object.freeze({
    arrows: false,
    colorGroups: true,
    labelThreshold: 78,
    nodeSize: 100,
    linkThickness: 100,
    centerForce: 34,
    repelForce: 100,
    linkForce: 100,
  });

  function graphSettingNumber(value, fallback, min, max) {
    const number = Number(value);
    return clamp(Number.isFinite(number) ? number : fallback, min, max);
  }

  function normalizeGraphSettings(value) {
    const candidate = value && typeof value === "object" ? value : {};
    return {
      arrows: Boolean(candidate.arrows),
      colorGroups: candidate.colorGroups !== false,
      labelThreshold: graphSettingNumber(candidate.labelThreshold, defaultGraphSettings.labelThreshold, 0, 100),
      nodeSize: graphSettingNumber(candidate.nodeSize, defaultGraphSettings.nodeSize, 50, 180),
      linkThickness: graphSettingNumber(candidate.linkThickness, defaultGraphSettings.linkThickness, 40, 260),
      centerForce: graphSettingNumber(candidate.centerForce, defaultGraphSettings.centerForce, 0, 100),
      repelForce: graphSettingNumber(candidate.repelForce, defaultGraphSettings.repelForce, 0, 200),
      linkForce: graphSettingNumber(candidate.linkForce, defaultGraphSettings.linkForce, 0, 200),
    };
  }

  function readGraphSettings() {
    return normalizeGraphSettings(readStoredJSON(graphSettingsStorageKey));
  }

  function saveGraphSettings(settingsValue) {
    try {
      window.localStorage.setItem(graphSettingsStorageKey, JSON.stringify(settingsValue));
    } catch {
      // Browser storage can be disabled in private or file-export contexts.
    }
  }

  function renderKnowledgeGraph() {
    const graphView = document.querySelector("[data-knowledge-graph-view]");
    const graphSidebar = document.querySelector("[data-knowledge-graph-sidebar]");
    if (!graphView) {
      return;
    }
    graphView.replaceChildren();
    graphSidebar?.replaceChildren();
    if (graphSidebar) {
      graphSidebar.hidden = true;
    }

    const info = document.createElement("div");
    info.className = "knowledge-graph-info";
    const title = document.createElement("h2");
    title.textContent = "Graph view";
    const help = document.createElement("p");
    help.textContent = "Drag to pan, scroll to zoom, or select a node to inspect it.";
    const status = document.createElement("p");
    status.className = "knowledge-graph-status";
    status.dataset.knowledgeGraphStatus = "";
    status.setAttribute("aria-live", "polite");
    const graphCount = knowledgeGraph.nodes.length
      ? knowledgeGraph.nodes.length + (knowledgeGraph.nodes.length === 1 ? " item" : " items")
      : "No items";
    status.textContent = projectionHasProblems(knowledgeGraph)
      ? graphCount + " · " + projectionFailureSummary(knowledgeGraph)
      : graphCount;
    info.append(title, help, status);
    (graphSidebar || graphView).append(info);

    if (!knowledgeGraph.nodes.length) {
      const onboarding = document.createElement("section");
      onboarding.className = "knowledge-graph-onboarding";
      const onboardingTitle = document.createElement("h2");
      const onboardingCopy = document.createElement("p");
      if (knowledgeGraph.status === "failed") {
        onboardingTitle.textContent = "Graph is unavailable";
        onboardingCopy.textContent = projectionFailureSummary(knowledgeGraph) + ". Refresh to try again.";
        onboarding.append(onboardingTitle, onboardingCopy);
      } else if (knowledgeGraph.status === "partial") {
        onboardingTitle.textContent = "Graph is incomplete";
        onboardingCopy.textContent = projectionFailureSummary(knowledgeGraph) + ". No items are available from the remaining knowledge spaces.";
        onboarding.append(onboardingTitle, onboardingCopy);
      } else if (knowledgeBaseNames().length === 0 && document.querySelector("[data-knowledge-base-connect]")) {
        onboardingTitle.textContent = "Bring your knowledge into one workspace";
        onboardingCopy.textContent = "Connect a knowledge space to search, browse documents, inspect claims, and explore its graph here.";
        const action = document.createElement("button");
        action.type = "button";
        action.className = "knowledge-graph-onboarding-action";
        action.textContent = "Connect knowledge space";
        action.addEventListener("click", function () {
          document.querySelector("[data-knowledge-base-connect]")?.click();
        });
        onboarding.append(onboardingTitle, onboardingCopy, action);
      } else {
        onboardingTitle.textContent = "No documents yet";
        onboardingCopy.textContent = "Add Markdown files to a connected knowledge space, then refresh this view.";
        onboarding.append(onboardingTitle, onboardingCopy);
      }
      graphView.append(onboarding);
      return;
    }

    const graphScale = Math.max(1, Math.sqrt(knowledgeGraph.nodes.length / 80));
    const worldWidth = Math.round(1200 * graphScale);
    const worldHeight = Math.round(820 * graphScale);
    const labelsByPath = graphUniqueNodeLabels(knowledgeGraph.nodes);
    const positions = graphLayoutPositions(knowledgeGraph, worldWidth, worldHeight, labelsByPath);
    const settingsValue = readGraphSettings();
    const canvas = document.createElement("canvas");
    canvas.className = "knowledge-graph-canvas";
    canvas.dataset.knowledgeGraphCanvas = "true";
    canvas.tabIndex = 0;
    canvas.setAttribute("role", "img");
    canvas.setAttribute("aria-label", "Interactive graph of Markdown files. Drag to pan, scroll to zoom, use arrow keys to select a note, and press Enter to open it.");
    graphView.append(canvas);

    const controller = createKnowledgeGraphCanvas(canvas, knowledgeGraph, positions, labelsByPath, worldWidth, worldHeight, status, settingsValue);
    graphView.append(createKnowledgeGraphViewportActions(controller, graphSidebar));
    if (graphSidebar) {
      graphSidebar.append(createKnowledgeGraphControls(controller, settingsValue, knowledgeGraph));
    }
    controller.start();
  }

  function createKnowledgeGraphControls(controller, settingsValue, graph) {
    const controls = document.createElement("div");
    controls.className = "knowledge-graph-controls";
    const bindings = [];

    const filterSection = graphControlSection("Filters", true);
    const filterLabel = document.createElement("label");
    filterLabel.className = "knowledge-graph-filter";
    const filterText = document.createElement("span");
    filterText.className = "sr-only";
    filterText.textContent = "Filter notes";
    const filterInput = document.createElement("input");
    filterInput.type = "search";
    filterInput.placeholder = "Filter notes…";
    filterInput.autocomplete = "off";
    filterInput.dataset.graphFilter = "";
    filterInput.addEventListener("input", function () {
      controller.setFilter(filterInput.value);
    });
    filterLabel.append(filterText, filterInput);
    filterSection.body.append(filterLabel);

    const typeChoices = Array.from(new Set(graph.nodes.map(graphNodeCategory))).sort();
    if (typeChoices.length > 1) {
      filterSection.body.append(graphFilterChoiceGroup("Content", typeChoices, function (value) {
        return value.charAt(0).toUpperCase() + value.slice(1);
      }, controller.setVisibleKinds, false, bindings));
    }
    const knowledgeBaseChoices = Array.from(new Set(graph.nodes.map(function (node) {
      return String(node?.knowledgeBase || "").trim();
    }).filter(Boolean))).sort();
    if (knowledgeBaseChoices.length > 1) {
      filterSection.body.append(graphFilterChoiceGroup("Knowledge spaces", knowledgeBaseChoices, function (value) {
        return value;
      }, controller.setVisibleKnowledgeBases, true, bindings));
    }

    const groupSection = graphControlSection("Groups", true);
    groupSection.body.append(graphToggleControl(knowledgeBaseChoices.length > 1 ? "Color nodes by knowledge space" : "Color nodes by folder", settingsValue.colorGroups, function (checked) {
      settingsValue.colorGroups = checked;
      controller.setSetting("colorGroups", checked);
      saveGraphSettings(settingsValue);
    }, function (checked) {
      settingsValue.colorGroups = checked;
    }, bindings));

    const displaySection = graphControlSection("Display", true);
    displaySection.body.append(graphToggleControl("Show arrows", settingsValue.arrows, function (checked) {
      settingsValue.arrows = checked;
      controller.setSetting("arrows", checked);
      saveGraphSettings(settingsValue);
    }, function (checked) {
      settingsValue.arrows = checked;
    }, bindings));
    displaySection.body.append(graphRangeControl("Text fade threshold", "labelThreshold", settingsValue.labelThreshold, 0, 100, 1, function (value) {
      return Math.round(value) + "%";
    }, controller, settingsValue, bindings));
    displaySection.body.append(graphRangeControl("Node size", "nodeSize", settingsValue.nodeSize, 50, 180, 1, function (value) {
      return Math.round(value) + "%";
    }, controller, settingsValue, bindings));
    displaySection.body.append(graphRangeControl("Link thickness", "linkThickness", settingsValue.linkThickness, 40, 260, 1, function (value) {
      return Math.round(value) + "%";
    }, controller, settingsValue, bindings));

    const forceSection = graphControlSection("Forces", false);
    forceSection.body.append(graphRangeControl("Center force", "centerForce", settingsValue.centerForce, 0, 100, 1, function (value) {
      return Math.round(value) + "%";
    }, controller, settingsValue, bindings));
    forceSection.body.append(graphRangeControl("Repel force", "repelForce", settingsValue.repelForce, 0, 200, 1, function (value) {
      return Math.round(value) + "%";
    }, controller, settingsValue, bindings));
    forceSection.body.append(graphRangeControl("Link force", "linkForce", settingsValue.linkForce, 0, 200, 1, function (value) {
      return Math.round(value) + "%";
    }, controller, settingsValue, bindings));

    const graphActions = document.createElement("div");
    graphActions.className = "knowledge-graph-actions";
    const animation = graphActionButton(controller.isRunning() ? "Pause" : "Resume", function () {
      controller.setRunning(!controller.isRunning());
    });
    animation.dataset.graphAnimation = "";
    animation.setAttribute("aria-pressed", controller.isRunning() ? "true" : "false");
    controller.onRunningChange(function (running) {
      animation.textContent = running ? "Pause" : "Resume";
      animation.setAttribute("aria-pressed", running ? "true" : "false");
    });
    const reset = graphActionButton("Reset graph", function () {
      Object.assign(settingsValue, defaultGraphSettings);
      bindings.forEach(function (binding) {
        binding(settingsValue);
      });
      filterInput.value = "";
      controller.reset(settingsValue);
      saveGraphSettings(settingsValue);
    });
    graphActions.append(animation, reset);

    const controlSections = [filterSection.details, groupSection.details, displaySection.details, forceSection.details];
    controlSections.forEach(function (section) {
      const summary = section.querySelector("summary");
      summary.addEventListener("click", function (event) {
        if (mobileSidebar.matches) {
          event.preventDefault();
        }
      });
    });

    const settingsDisclosure = document.createElement("details");
    settingsDisclosure.className = "knowledge-graph-settings";
    const settingsSummary = document.createElement("summary");
    settingsSummary.textContent = "Graph settings";
    const settingsBody = document.createElement("div");
    settingsBody.className = "knowledge-graph-settings-body";
    settingsBody.append(...controlSections, graphActions);
    settingsDisclosure.append(settingsSummary, settingsBody);

    const syncSettingsDisclosure = function () {
      const mobile = mobileSidebar.matches;
      settingsDisclosure.open = !mobile;
      controlSections.forEach(function (section) {
        const summary = section.querySelector("summary");
        section.open = mobile || section.dataset.desktopOpen === "true";
        if (mobile) {
          summary.tabIndex = -1;
          summary.setAttribute("role", "heading");
          summary.setAttribute("aria-level", "3");
        } else {
          summary.removeAttribute("tabindex");
          summary.removeAttribute("role");
          summary.removeAttribute("aria-level");
        }
      });
    };
    mobileSidebar.addEventListener("change", syncSettingsDisclosure);
    syncSettingsDisclosure();

    controls.append(settingsDisclosure);
    return controls;
  }

  function graphControlSection(title, open) {
    const details = document.createElement("details");
    details.className = "knowledge-graph-control-section";
    details.dataset.desktopOpen = open ? "true" : "false";
    details.open = open;
    const summary = document.createElement("summary");
    summary.textContent = title;
    const body = document.createElement("div");
    body.className = "knowledge-graph-control-body";
    details.append(summary, body);
    return { details: details, body: body };
  }

  function graphFilterChoiceGroup(labelText, values, format, onChange, useKnowledgeBaseColor, bindings) {
    const fieldset = document.createElement("fieldset");
    fieldset.className = "knowledge-graph-filter-group";
    const legend = document.createElement("legend");
    legend.textContent = labelText;
    fieldset.append(legend);
    const selected = new Set(values);
    const inputs = [];
    values.forEach(function (value) {
      const label = document.createElement("label");
      const input = document.createElement("input");
      input.type = "checkbox";
      input.checked = true;
      input.value = value;
      inputs.push(input);
      const marker = document.createElement("span");
      marker.className = "knowledge-graph-filter-marker";
      if (useKnowledgeBaseColor) {
        marker.style.backgroundColor = knowledgeBaseColor(value);
      }
      const text = document.createElement("span");
      text.textContent = format(value);
      input.addEventListener("change", function () {
        if (input.checked) {
          selected.add(value);
        } else {
          selected.delete(value);
        }
        onChange(Array.from(selected));
      });
      label.append(input, marker, text);
      fieldset.append(label);
    });
    bindings?.push(function () {
      selected.clear();
      values.forEach(function (value) { selected.add(value); });
      inputs.forEach(function (input) { input.checked = true; });
      onChange(Array.from(selected));
    });
    return fieldset;
  }

  function graphNodeCategory(node) {
    const kind = String(node?.kind || "").toLowerCase();
    if (kind === "claim") {
      return "claims";
    }
    if (kind === "entity") {
      return "entities";
    }
    return "documents";
  }

  function graphToggleControl(labelText, checked, onChange, onReset, bindings) {
    const label = document.createElement("label");
    label.className = "knowledge-graph-toggle";
    const text = document.createElement("span");
    text.textContent = labelText;
    const input = document.createElement("input");
    input.type = "checkbox";
    input.checked = checked;
    input.addEventListener("change", function () {
      onChange(input.checked);
    });
    bindings.push(function (settingsValue) {
      const key = labelText === "Show arrows" ? "arrows" : "colorGroups";
      input.checked = Boolean(settingsValue[key]);
      onReset(input.checked);
    });
    label.append(text, input);
    return label;
  }

  function graphRangeControl(labelText, key, value, min, max, step, format, controller, settingsValue, bindings) {
    const label = document.createElement("label");
    label.className = "knowledge-graph-range";
    label.htmlFor = "knowledge-graph-" + key;
    const header = document.createElement("span");
    header.className = "knowledge-graph-range-header";
    const text = document.createElement("span");
    text.textContent = labelText;
    const output = document.createElement("output");
    output.textContent = format(value);
    const input = document.createElement("input");
    input.id = "knowledge-graph-" + key;
    input.type = "range";
    input.min = String(min);
    input.max = String(max);
    input.step = String(step);
    input.value = String(value);
    input.addEventListener("input", function () {
      const nextValue = Number(input.value);
      settingsValue[key] = nextValue;
      output.textContent = format(nextValue);
      controller.setSetting(key, nextValue);
      saveGraphSettings(settingsValue);
    });
    bindings.push(function (nextSettings) {
      input.value = String(nextSettings[key]);
      output.textContent = format(nextSettings[key]);
    });
    header.append(text, output);
    label.append(header, input);
    return label;
  }

  function graphActionButton(label, onClick) {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = label;
    button.addEventListener("click", onClick);
    return button;
  }

  function createKnowledgeGraphViewportActions(controller, graphSidebar) {
    const actions = document.createElement("div");
    actions.className = "knowledge-graph-viewport-actions";
    actions.setAttribute("role", "toolbar");
    actions.setAttribute("aria-label", "Graph viewport controls");
    actions.append(
      graphIconActionButton("Zoom out", "minus", function () { controller.zoomBy(0.82); }),
      graphIconActionButton("Zoom in", "plus", function () { controller.zoomBy(1.22); }),
      graphIconActionButton("Fit graph", "fit", function () { controller.fit(); }),
    );
    if (graphSidebar) {
      const settings = graphIconActionButton("Graph settings", "settings", function () {
        const expanded = settings.getAttribute("aria-expanded") === "true";
        settings.setAttribute("aria-expanded", expanded ? "false" : "true");
        graphSidebar.hidden = expanded;
      });
      settings.className = "knowledge-graph-settings-toggle";
      settings.setAttribute("aria-expanded", "false");
      actions.append(settings);
    }
    return actions;
  }

  function graphIconActionButton(label, icon, onClick) {
    const button = document.createElement("button");
    button.type = "button";
    button.setAttribute("aria-label", label);
    button.title = label;
    button.append(graphViewportIcon(icon));
    button.addEventListener("click", onClick);
    return button;
  }

  function graphViewportIcon(icon) {
    const namespace = "http://www.w3.org/2000/svg";
    const svg = document.createElementNS(namespace, "svg");
    svg.setAttribute("viewBox", "0 0 20 20");
    svg.setAttribute("aria-hidden", "true");
    svg.setAttribute("focusable", "false");
    const pathData = {
      minus: ["M5 10h10"],
      plus: ["M5 10h10", "M10 5v10"],
      fit: ["M8 4.5H4.5V8", "M12 4.5h3.5V8", "M8 15.5H4.5V12", "M12 15.5h3.5V12"],
      settings: ["M4 6h12", "M4 10h12", "M4 14h12", "M7 4v4", "M13 8v4", "M8 12v4"],
    };
    (pathData[icon] || []).forEach(function (data) {
      const path = document.createElementNS(namespace, "path");
      path.setAttribute("d", data);
      svg.append(path);
    });
    return svg;
  }

  function createKnowledgeGraphCanvas(canvas, graph, positions, labelsByPath, worldWidth, worldHeight, status, settingsValue) {
    const context = canvas.getContext("2d");
    const settingsState = normalizeGraphSettings(settingsValue);
    const nodeSet = Object.create(null);
    const degree = Object.create(null);
    graph.nodes.forEach(function (node) {
      if (node && typeof node.path === "string") {
        nodeSet[node.path] = true;
        degree[node.path] = 0;
      }
    });
    const links = graph.edges.filter(function (edge) {
      const valid = edge && nodeSet[edge.source] && nodeSet[edge.target] && positions[edge.source] && positions[edge.target];
      if (valid) {
        degree[edge.source] += 1;
        degree[edge.target] += 1;
      }
      return valid;
    });
    const states = graph.nodes.filter(function (node) {
      return node && typeof node.path === "string" && positions[node.path];
    }).map(function (node) {
      const point = positions[node.path];
      const nodeDegree = degree[node.path] || 0;
      const sourcePath = node.sourcePath || node.path;
      return {
        node: node,
        path: node.path,
        group: graphPathGroup(sourcePath),
        label: graphNodeLabel(node, labelsByPath),
        fullLabel: graphNodeFullLabel(node, labelsByPath),
        radius: sourcePath === "index.md" ? 10 : 4.5 + Math.min(5.5, Math.sqrt(nodeDegree) * 1.45),
        degree: nodeDegree,
        baseX: point.x,
        baseY: point.y,
        x: point.x,
        y: point.y,
        z: 0,
        vx: 0,
        vy: 0,
      };
    });
    const stateByPath = Object.create(null);
    states.forEach(function (state) {
      stateByPath[state.path] = state;
    });

    const viewport = {
      width: worldWidth,
      height: worldHeight,
      pixelRatio: 1,
      camera: { x: 0, y: 0, zoom: 1 },
    };
    const runningListeners = [];
    let filter = "";
    let visibleKinds = new Set(states.map(function (state) { return graphNodeCategory(state.node); }));
    let visibleKnowledgeBases = new Set(states.map(function (state) { return String(state.node.knowledgeBase || "").trim(); }).filter(Boolean));
    let activePath = "";
    let keyboardIndex = states.findIndex(function (state) {
      return (state.node.sourcePath || state.path) === "index.md" && (!currentKnowledgeBase || state.node.knowledgeBase === currentKnowledgeBase);
    });
    if (keyboardIndex < 0) {
      keyboardIndex = states.findIndex(function (state) { return (state.node.sourcePath || state.path) === "index.md"; });
    }
    let lastPointer = null;
    let pointerGesture = null;
    let frame = 0;
    let cameraReady = false;
    let running = !motionIsReduced() && states.length <= 250;
    let resizeObserver = null;
    let themeObserver = null;

    if (keyboardIndex < 0) {
      keyboardIndex = 0;
    }

    const visibleStates = function () {
      return states.filter(function (state) {
        const categoryVisible = visibleKinds.has(graphNodeCategory(state.node));
        const knowledgeBase = String(state.node.knowledgeBase || "").trim();
        const knowledgeBaseVisible = !knowledgeBase || visibleKnowledgeBases.has(knowledgeBase);
        const textVisible = !filter || state.path.toLowerCase().includes(filter) || state.fullLabel.toLowerCase().includes(filter);
        return categoryVisible && knowledgeBaseVisible && textVisible;
      });
    };

    const visibleLinks = function (visible) {
      const visiblePaths = new Set(visible.map(function (state) { return state.path; }));
      return links.filter(function (edge) {
        return visiblePaths.has(edge.source) && visiblePaths.has(edge.target);
      });
    };

    const updateStatus = function () {
      const visible = visibleStates();
      const projectionNotice = projectionHasProblems(graph) ? " · " + projectionFailureSummary(graph) : "";
      if (activePath && stateByPath[activePath] && visible.includes(stateByPath[activePath])) {
        const selected = stateByPath[activePath];
        const connectionCount = visibleLinks(visible).filter(function (edge) {
          return edge.source === activePath || edge.target === activePath;
        }).length;
        const connectionLabel = connectionCount + (connectionCount === 1 ? " connection" : " connections");
        status.textContent = connectionLabel + projectionNotice;
        canvas.setAttribute("aria-label", "Selected " + selected.fullLabel + " with " + connectionLabel + ". Use arrow keys to move and Enter to open.");
        return;
      }
      status.textContent = filter || visible.length !== states.length
        ? visible.length + " of " + states.length + (states.length === 1 ? " item" : " items") + projectionNotice
        : states.length + (states.length === 1 ? " item" : " items") + projectionNotice;
      canvas.setAttribute("aria-label", "Interactive graph of Markdown files. Drag to pan, scroll to zoom, use arrow keys to select a note, and press Enter to open it.");
    };

    const setActivePath = function (path) {
      const visible = visibleStates();
      const nextPath = path && stateByPath[path] && visible.includes(stateByPath[path]) ? path : "";
      if (nextPath === activePath) {
        return;
      }
      activePath = nextPath;
      canvas.dataset.activeGraphPath = activePath;
      if (activePath) {
        const activeIndex = states.findIndex(function (state) { return state.path === activePath; });
        if (activeIndex >= 0) {
          keyboardIndex = activeIndex;
        }
      }
      updateStatus();
      invalidate();
    };

    const resizeCanvas = function () {
      if (!context) {
        return;
      }
      const rect = canvas.getBoundingClientRect();
      const nextWidth = Math.max(1, Math.round(rect.width));
      const nextHeight = Math.max(1, Math.round(rect.height));
      const measurable = rect.width > 50 && rect.height > 50;
      const previousWidth = viewport.width;
      const previousHeight = viewport.height;
      viewport.width = nextWidth;
      viewport.height = nextHeight;
      viewport.pixelRatio = Math.max(1, Math.min(window.devicePixelRatio || 1, 2));
      canvas.width = Math.round(nextWidth * viewport.pixelRatio);
      canvas.height = Math.round(nextHeight * viewport.pixelRatio);
      if (!cameraReady && measurable) {
        fitCamera();
        cameraReady = true;
      } else if (cameraReady) {
        viewport.camera.x += (nextWidth - previousWidth) / 2;
        viewport.camera.y += (nextHeight - previousHeight) / 2;
      }
      invalidate();
    };

    const screenPoint = function (event) {
      const rect = canvas.getBoundingClientRect();
      return { x: event.clientX - rect.left, y: event.clientY - rect.top };
    };

    const worldPoint = function (point) {
      return {
        x: (point.x - viewport.camera.x) / viewport.camera.zoom,
        y: (point.y - viewport.camera.y) / viewport.camera.zoom,
      };
    };

    const fitCamera = function () {
      const visible = visibleStates();
      if (!visible.length) {
        return;
      }
      let minX = Infinity;
      let maxX = -Infinity;
      let minY = Infinity;
      let maxY = -Infinity;
      visible.forEach(function (state) {
        minX = Math.min(minX, state.x);
        maxX = Math.max(maxX, state.x);
        minY = Math.min(minY, state.y);
        maxY = Math.max(maxY, state.y);
      });
      const padding = 54;
      const spanX = Math.max(80, maxX - minX);
      const spanY = Math.max(80, maxY - minY);
      const zoom = clamp(Math.min((viewport.width - padding * 2) / spanX, (viewport.height - padding * 2) / spanY), 0.28, 2.4);
      viewport.camera.zoom = zoom;
      viewport.camera.x = viewport.width / 2 - ((minX + maxX) / 2) * zoom;
      viewport.camera.y = viewport.height / 2 - ((minY + maxY) / 2) * zoom;
      invalidate();
    };

    const actualSize = function () {
      const visible = visibleStates();
      if (!visible.length) {
        return;
      }
      const centerX = visible.reduce(function (sum, state) { return sum + state.x; }, 0) / visible.length;
      const centerY = visible.reduce(function (sum, state) { return sum + state.y; }, 0) / visible.length;
      viewport.camera.zoom = 1;
      viewport.camera.x = viewport.width / 2 - centerX;
      viewport.camera.y = viewport.height / 2 - centerY;
      invalidate();
    };

    const zoomAt = function (factor, point) {
      const anchor = point || { x: viewport.width / 2, y: viewport.height / 2 };
      const before = worldPoint(anchor);
      viewport.camera.zoom = clamp(viewport.camera.zoom * factor, 0.24, 4);
      viewport.camera.x = anchor.x - before.x * viewport.camera.zoom;
      viewport.camera.y = anchor.y - before.y * viewport.camera.zoom;
      invalidate();
    };

    const setRunning = function (nextRunning) {
      const normalized = Boolean(nextRunning);
      if (normalized === running) {
        return;
      }
      running = normalized;
      runningListeners.forEach(function (listener) { listener(running); });
      invalidate();
    };

    const resetGraph = function (nextSettings) {
      Object.assign(settingsState, normalizeGraphSettings(nextSettings));
      filter = "";
      activePath = "";
      states.forEach(function (state) {
        state.x = state.baseX;
        state.y = state.baseY;
        state.vx = 0;
        state.vy = 0;
        state.z = 0;
      });
      setRunning(!motionIsReduced());
      updateStatus();
      fitCamera();
    };

    const updatePointerTarget = function (point) {
      lastPointer = point;
      const hit = graphCanvasHitTest(visibleStates(), worldPoint(point), settingsState);
      setActivePath(hit ? hit.path : "");
    };

    canvas.addEventListener("wheel", function (event) {
      event.preventDefault();
      zoomAt(Math.exp(-event.deltaY * 0.0015), screenPoint(event));
    }, { passive: false });
    canvas.addEventListener("pointerdown", function (event) {
      if (event.button !== 0) {
        return;
      }
      const point = screenPoint(event);
      const hit = graphCanvasHitTest(visibleStates(), worldPoint(point), settingsState);
      pointerGesture = {
        pointerID: event.pointerId,
        startX: event.clientX,
        startY: event.clientY,
        cameraX: viewport.camera.x,
        cameraY: viewport.camera.y,
        node: hit,
        nodeX: hit ? hit.x : 0,
        nodeY: hit ? hit.y : 0,
        moved: false,
      };
      canvas.setPointerCapture(event.pointerId);
      canvas.dataset.graphDragging = hit ? "node" : "canvas";
      event.preventDefault();
    });
    canvas.addEventListener("pointermove", function (event) {
      const point = screenPoint(event);
      if (!pointerGesture || pointerGesture.pointerID !== event.pointerId) {
        updatePointerTarget(point);
        return;
      }
      const dx = event.clientX - pointerGesture.startX;
      const dy = event.clientY - pointerGesture.startY;
      pointerGesture.moved = pointerGesture.moved || Math.abs(dx) + Math.abs(dy) > 5;
      if (pointerGesture.node) {
        pointerGesture.node.x = pointerGesture.nodeX + dx / viewport.camera.zoom;
        pointerGesture.node.y = pointerGesture.nodeY + dy / viewport.camera.zoom;
        pointerGesture.node.baseX = pointerGesture.node.x;
        pointerGesture.node.baseY = pointerGesture.node.y;
        pointerGesture.node.vx = 0;
        pointerGesture.node.vy = 0;
        setActivePath(pointerGesture.node.path);
      } else {
        viewport.camera.x = pointerGesture.cameraX + dx;
        viewport.camera.y = pointerGesture.cameraY + dy;
      }
      invalidate();
    });
    const endPointerGesture = function (event) {
      if (!pointerGesture || pointerGesture.pointerID !== event.pointerId) {
        return;
      }
      const activatedNode = pointerGesture.node;
      const shouldOpen = activatedNode && !pointerGesture.moved && event.type === "pointerup";
      pointerGesture = null;
      delete canvas.dataset.graphDragging;
      if (canvas.hasPointerCapture(event.pointerId)) {
        canvas.releasePointerCapture(event.pointerId);
      }
      if (shouldOpen) {
        void openGraphNode(activatedNode.node);
      }
    };
    canvas.addEventListener("pointerup", endPointerGesture);
    canvas.addEventListener("pointercancel", endPointerGesture);
    canvas.addEventListener("pointerleave", function () {
      lastPointer = null;
      if (!pointerGesture && document.activeElement !== canvas) {
        setActivePath("");
      }
    });
    canvas.addEventListener("focus", function () {
      const visible = visibleStates();
      const target = states[keyboardIndex] && visible.includes(states[keyboardIndex]) ? states[keyboardIndex] : visible[0];
      if (target) {
        setActivePath(target.path);
      }
    });
    canvas.addEventListener("blur", function () {
      setActivePath(lastPointer ? activePath : "");
    });
    canvas.addEventListener("keydown", function (event) {
      const visible = visibleStates();
      if (!visible.length) {
        return;
      }
      if (event.key === "Enter" || event.key === " ") {
        if (activePath) {
          event.preventDefault();
          void openGraphNode(stateByPath[activePath].node);
        }
        return;
      }
      if (event.key === "Escape") {
        setActivePath("");
        return;
      }
      if (event.key === "+" || event.key === "=") {
        event.preventDefault();
        zoomAt(1.22);
        return;
      }
      if (event.key === "-") {
        event.preventDefault();
        zoomAt(0.82);
        return;
      }
      if (event.key === "0") {
        event.preventDefault();
        fitCamera();
        return;
      }
      if (event.key !== "ArrowRight" && event.key !== "ArrowDown" && event.key !== "ArrowLeft" && event.key !== "ArrowUp") {
        return;
      }
      event.preventDefault();
      const currentIndex = Math.max(0, visible.findIndex(function (state) { return state.path === activePath; }));
      const direction = event.key === "ArrowRight" || event.key === "ArrowDown" ? 1 : -1;
      const nextIndex = (currentIndex + direction + visible.length) % visible.length;
      keyboardIndex = states.indexOf(visible[nextIndex]);
      setActivePath(visible[nextIndex].path);
    });

    const invalidate = function () {
      if (!frame) {
        frame = window.requestAnimationFrame(tick);
      }
    };

    const tick = function () {
      frame = 0;
      if (!canvas.isConnected || !context) {
        resizeObserver?.disconnect();
        themeObserver?.disconnect();
        window.removeEventListener("openknowledge:knowledge-base-color", invalidate);
        return;
      }
      const visible = visibleStates();
      const currentLinks = visibleLinks(visible);
      if (running) {
        graphCanvasPhysicsStep(visible, currentLinks, stateByPath, activePath, worldWidth, worldHeight, settingsState);
      }
      drawKnowledgeGraphCanvas(context, visible, currentLinks, stateByPath, activePath, viewport, settingsState);
      if (running) {
        invalidate();
      }
    };

    return {
      start: function () {
        if (!context) {
          return;
        }
        resizeObserver = typeof ResizeObserver === "function" ? new ResizeObserver(resizeCanvas) : null;
        resizeObserver?.observe(canvas);
        themeObserver = new MutationObserver(invalidate);
        themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-viewer-theme", "data-viewer-contrast", "style"] });
        window.addEventListener("openknowledge:knowledge-base-color", invalidate);
        resizeCanvas();
        updateStatus();
        invalidate();
      },
      fit: fitCamera,
      actualSize: actualSize,
      zoomBy: function (factor) { zoomAt(factor); },
      isRunning: function () { return running; },
      setRunning: setRunning,
      onRunningChange: function (listener) { runningListeners.push(listener); },
      setFilter: function (value) {
        filter = String(value || "").trim().toLowerCase();
        if (activePath && !visibleStates().includes(stateByPath[activePath])) {
          activePath = "";
        }
        updateStatus();
        fitCamera();
      },
      setVisibleKinds: function (values) {
        visibleKinds = new Set(values);
        setActivePath("");
        updateStatus();
        fitCamera();
      },
      setVisibleKnowledgeBases: function (values) {
        visibleKnowledgeBases = new Set(values);
        setActivePath("");
        updateStatus();
        fitCamera();
      },
      setSetting: function (key, value) {
        if (Object.prototype.hasOwnProperty.call(settingsState, key)) {
          settingsState[key] = value;
          invalidate();
        }
      },
      reset: resetGraph,
    };
  }

  async function openGraphNode(node) {
    if (!node) {
      return;
    }
    const knowledgeBase = String(node.knowledgeBase || "").trim();
    if (node.kind === "claim" && node.url) {
      try {
        await ensureClaimsData(knowledgeBase);
        refreshClaimsViewToggles();
      } catch {
        return;
      }
      try {
        const url = new URL(node.url, window.location.href);
        const claim = claimByID(url.searchParams.get("claim") || "", knowledgeBase);
        if (claim) {
          selectedClaimKey = claimIdentity(claim);
          setClaimsViewRequested(true, true);
        }
      } catch {
        // Keep the graph usable when a custom claim URL is malformed.
      }
      return;
    }
    if (node.kind === "entity") {
      try {
        await ensureClaimsData(knowledgeBase);
        refreshClaimsViewToggles();
      } catch {
        return;
      }
      const entityID = String(node.path || "").replace(/^.*entity:/, "");
      const claim = claimsData.claims.find(function (candidate) {
        return candidate.knowledgeBase === knowledgeBase && (candidate.subject?.id === entityID || candidate.object?.ref === entityID);
      });
      if (claim) {
        selectedClaimKey = claimIdentity(claim);
        setClaimsViewRequested(true, true);
      }
      return;
    }
    openTarget(node.sourcePath || node.path, true, shouldOpenBeside(false), "", activePanel(), knowledgeBase);
  }

  function graphCanvasPhysicsStep(states, links, stateByPath, activePath, width, height, settingsValue) {
    const active = activePath ? stateByPath[activePath] : null;
    const centerStrength = settingsValue.centerForce / 100;
    const repelStrength = settingsValue.repelForce / 100;
    const linkStrength = settingsValue.linkForce / 100;
    states.forEach(function (state) {
      const targetZ = state === active ? 1 : 0;
      const basePull = 0.004 + centerStrength * (state === active ? 0.042 : 0.026);
      state.vx += (state.baseX - state.x) * basePull;
      state.vy += (state.baseY - state.y) * basePull;
      state.z += (targetZ - state.z) * 0.095;
    });
    const hoverStrength = active ? graphEaseInOut(active.z) : 0;

    links.forEach(function (edge) {
      const source = stateByPath[edge.source];
      const target = stateByPath[edge.target];
      if (!source || !target) {
        return;
      }
      const dx = target.x - source.x || 0.01;
      const dy = target.y - source.y || 0.01;
      const distance = Math.max(1, Math.sqrt(dx * dx + dy * dy));
      const connected = active && (edge.source === active.path || edge.target === active.path);
      const desired = 104 + (connected ? 22 * hoverStrength : 0);
      const force = (distance - desired) * (connected ? 0.0017 : 0.0011) * linkStrength;
      const nx = dx / distance;
      const ny = dy / distance;
      source.vx += nx * force;
      source.vy += ny * force;
      target.vx -= nx * force;
      target.vy -= ny * force;
    });

    const neighborWindow = states.length > 320 ? 18 : states.length;
    for (let i = 0; i < states.length; i += 1) {
      for (let j = i + 1; j < Math.min(states.length, i + neighborWindow); j += 1) {
        const a = states[i];
        const b = states[j];
        const dx = b.x - a.x || 0.01;
        const dy = b.y - a.y || 0.01;
        const distance = Math.max(1, Math.sqrt(dx * dx + dy * dy));
        const nx = dx / distance;
        const ny = dy / distance;
        const activePair = active && (a === active || b === active);
        const desired = activePair ? 68 + 52 * hoverStrength : 34 + (a.radius + b.radius) * (settingsValue.nodeSize / 100);
        if (distance < desired) {
          const push = Math.min(activePair ? 3.1 : 1.7, (desired - distance) * (activePair ? 0.018 : 0.011) * repelStrength);
          if (a !== active) {
            a.vx -= nx * push;
            a.vy -= ny * push;
          }
          if (b !== active) {
            b.vx += nx * push;
            b.vy += ny * push;
          }
        }
      }
    }

    states.forEach(function (state) {
      state.x += state.vx;
      state.y += state.vy;
      state.x = clamp(state.x, 24, width - 24);
      state.y = clamp(state.y, 24, height - 24);
      graphLimitVelocity(state, active ? 5.5 : 4.2);
      state.vx *= active ? 0.58 : 0.66;
      state.vy *= active ? 0.58 : 0.66;
      if (Math.abs(state.vx) < 0.018) {
        state.vx = 0;
      }
      if (Math.abs(state.vy) < 0.018) {
        state.vy = 0;
      }
    });
  }

  function graphEaseInOut(value) {
    const t = clamp(value, 0, 1);
    return t * t * (3 - 2 * t);
  }

  function graphLimitVelocity(state, maxVelocity) {
    const speed = Math.sqrt(state.vx * state.vx + state.vy * state.vy);
    if (speed <= maxVelocity || speed <= 0) {
      return;
    }
    const scale = maxVelocity / speed;
    state.vx *= scale;
    state.vy *= scale;
  }

  function drawKnowledgeGraphCanvas(context, states, links, stateByPath, activePath, viewport, settingsValue) {
    const active = activePath ? stateByPath[activePath] : null;
    const theme = graphCanvasTheme();
    const camera = viewport.camera;
    const nodeScale = settingsValue.nodeSize / 100;
    const linkScale = settingsValue.linkThickness / 100;
    const groupIndexes = Object.create(null);
    Array.from(new Set(states.map(function (state) { return state.node.knowledgeBase || state.group; }))).sort().forEach(function (group, index) {
      groupIndexes[group] = index;
    });

    context.setTransform(viewport.pixelRatio, 0, 0, viewport.pixelRatio, 0, 0);
    context.clearRect(0, 0, viewport.width, viewport.height);
    context.save();
    context.translate(camera.x, camera.y);
    context.scale(camera.zoom, camera.zoom);
    context.lineCap = "round";
    context.lineJoin = "round";

    links.forEach(function (edge) {
      const source = stateByPath[edge.source];
      const target = stateByPath[edge.target];
      if (!source || !target) {
        return;
      }
      const connected = active && (edge.source === active.path || edge.target === active.path);
      const color = connected ? theme.edgeActive : active ? theme.edgeMuted : theme.edge;
      const lineWidth = (connected ? 2.1 : 0.9) * linkScale;
      context.beginPath();
      context.moveTo(source.x, source.y);
      context.lineTo(target.x, target.y);
      context.strokeStyle = color;
      context.lineWidth = lineWidth;
      context.stroke();
      if (settingsValue.arrows) {
        graphDrawArrow(context, source, target, target.radius * nodeScale + 3, color, lineWidth);
      }
    });

    states.slice().sort(function (a, b) {
      return a.z - b.z;
    }).forEach(function (state) {
      const activeNode = state === active;
      const scale = nodeScale * (1 + state.z * 0.32);
      const radius = state.radius * scale;
      const colorGroup = state.node.knowledgeBase || state.group;
      const groupColor = state.node.knowledgeBase ? knowledgeBaseColor(state.node.knowledgeBase) : theme.groups[groupIndexes[colorGroup] % theme.groups.length];
      const nodeColor = settingsValue.colorGroups ? groupColor : theme.node;
      context.save();
      context.globalAlpha = 1;
      context.beginPath();
      context.arc(state.x, state.y - state.z * 4, radius, 0, Math.PI * 2);
      context.fillStyle = activeNode && !settingsValue.colorGroups ? theme.nodeActive : nodeColor;
      context.fill();
      if (activeNode) {
        context.strokeStyle = theme.nodeActiveRing;
        context.lineWidth = 2.4 / Math.max(0.55, camera.zoom);
        context.stroke();
      }

      const sourcePath = state.node.sourcePath || state.path;
      const labelImportance = Math.min(0.5, state.degree * 0.09) + (sourcePath === "index.md" ? 0.35 : 0);
      const reveal = clamp((camera.zoom + labelImportance - (0.68 + settingsValue.labelThreshold * 0.006)) / 0.28, 0, 1);
      if (activeNode || reveal > 0.02) {
        const label = activeNode ? state.fullLabel : state.label;
        context.globalAlpha = activeNode ? 1 : reveal;
        context.font = (activeNode ? "650 13px" : "500 11.5px") + " " + theme.fontBody;
        context.textBaseline = "middle";
        context.textAlign = "center";
        const labelY = state.y + radius + 13 + state.z * 2;
        context.fillStyle = state.node.knowledgeBase ? knowledgeBaseColor(state.node.knowledgeBase) : (activeNode ? theme.labelActive : theme.label);
        context.fillText(label, state.x, labelY);
      }
      context.restore();
    });
    context.restore();
  }

  function graphDrawArrow(context, source, target, targetRadius, color, lineWidth) {
    const dx = target.x - source.x;
    const dy = target.y - source.y;
    const distance = Math.sqrt(dx * dx + dy * dy);
    if (distance < targetRadius + 8) {
      return;
    }
    const nx = dx / distance;
    const ny = dy / distance;
    const tipX = target.x - nx * targetRadius;
    const tipY = target.y - ny * targetRadius;
    const size = 5 + lineWidth;
    context.beginPath();
    context.moveTo(tipX, tipY);
    context.lineTo(tipX - nx * size - ny * size * 0.68, tipY - ny * size + nx * size * 0.68);
    context.lineTo(tipX - nx * size + ny * size * 0.68, tipY - ny * size - nx * size * 0.68);
    context.closePath();
    context.fillStyle = color;
    context.fill();
  }

  function graphCanvasTheme() {
    const highContrastMode = document.documentElement.dataset.viewerContrast === "high";
    const accentRGB = themeValue("--ok-color-accent-rgb", "15, 122, 77").split(",").map(function (component) {
      return clamp(Number(component.trim()) || 0, 0, 255);
    });
    const hsl = graphRGBToHSL(accentRGB[0], accentRGB[1], accentRGB[2]);
    const darkMode = getComputedStyle(document.documentElement).colorScheme.includes("dark");
    const lightness = darkMode ? 67 : 43;
    const hueOffsets = [0, 42, -42, 86, -86, 164];
    const groups = hueOffsets.map(function (offset, index) {
      const saturation = clamp(hsl.s + (index % 2 === 0 ? 4 : -8), 46, 82);
      return "hsl(" + ((hsl.h + offset + 360) % 360) + " " + saturation + "% " + clamp(lightness + (index % 3 - 1) * 5, 32, 76) + "%)";
    });
    const text = themeValue("--ok-color-text", "#202322");
    return {
      fontBody: themeValue("--ok-font-body", "Inter, ui-sans-serif, system-ui, sans-serif"),
      edge: highContrastMode ? text : themeValue("--ok-color-graph-edge", "rgba(128, 138, 133, .25)"),
      edgeMuted: highContrastMode ? themeValue("--ok-color-muted", "#707773") : themeValue("--ok-color-graph-edge-muted", "rgba(128, 138, 133, .11)"),
      edgeActive: themeValue("--ok-color-graph-edge-active", "rgba(15, 122, 77, .78)"),
      node: highContrastMode ? text : groups[0],
      nodeActive: themeValue("--ok-color-graph-node-active-border", "#0f7a4d"),
      nodeActiveRing: themeValue("--ok-color-graph-node-bg", "#f8f8f8"),
      label: highContrastMode ? text : themeValue("--ok-color-graph-label", "#5f6b66"),
      labelActive: highContrastMode ? text : themeValue("--ok-color-graph-label-active", "#26302c"),
      groups: highContrastMode ? [text] : groups,
    };
  }

  function graphRGBToHSL(red, green, blue) {
    const r = red / 255;
    const g = green / 255;
    const b = blue / 255;
    const max = Math.max(r, g, b);
    const min = Math.min(r, g, b);
    const delta = max - min;
    let hue = 0;
    if (delta) {
      if (max === r) {
        hue = 60 * (((g - b) / delta) % 6);
      } else if (max === g) {
        hue = 60 * ((b - r) / delta + 2);
      } else {
        hue = 60 * ((r - g) / delta + 4);
      }
    }
    const lightness = (max + min) / 2;
    const saturation = delta ? delta / (1 - Math.abs(2 * lightness - 1)) : 0;
    return { h: (hue + 360) % 360, s: saturation * 100, l: lightness * 100 };
  }

  function themeValue(name, fallback) {
    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return value || fallback;
  }

  function graphCanvasHitTest(states, point, settingsValue) {
    for (let index = states.length - 1; index >= 0; index -= 1) {
      const state = states[index];
      const dx = point.x - state.x;
      const dy = point.y - state.y;
      const radius = state.radius * (settingsValue.nodeSize / 100) * (1 + state.z * 0.32) + 7;
      if (dx * dx + dy * dy <= radius * radius) {
        return state;
      }
    }
    return null;
  }

  function graphLayoutPositions(graph, width, height, labelsByPath) {
    const nodes = graph.nodes.filter(function (node) {
      return node && typeof node.path === "string" && node.path.length > 0;
    });
    const positions = Object.create(null);
    if (nodes.length === 0) {
      return positions;
    }
    if (nodes.length === 1) {
      positions[nodes[0].path] = { x: width / 2, y: height / 2 };
      return positions;
    }
    if (nodes.length > 260) {
      return graphScalableLayoutPositions(nodes, width, height);
    }
    if (nodes.length > 250) {
      return graphGridPositions(nodes, width, height);
    }

    const center = { x: width / 2, y: height / 2 };
    const nodeSet = Object.create(null);
    const degree = Object.create(null);
    nodes.forEach(function (node) {
      nodeSet[node.path] = true;
      degree[node.path] = 0;
    });

    const links = [];
    graph.edges.forEach(function (edge) {
      if (!edge || !nodeSet[edge.source] || !nodeSet[edge.target]) {
        return;
      }
      links.push(edge);
      degree[edge.source] += 1;
      degree[edge.target] += 1;
    });

    const groupCenters = graphGroupCenters(nodes, width, height);
    const states = nodes.map(function (node) {
      const sourcePath = node.sourcePath || node.path;
      const group = graphPathGroup(sourcePath);
      const groupCenter = groupCenters[group] || center;
      const hash = graphHash(node.path);
      const angle = ((hash % 360) / 360) * Math.PI * 2;
      const spread = 26 + (hash % 74);
      return {
        node: node,
        group: group,
        label: graphNodeLabel(node, labelsByPath),
        radius: sourcePath === "index.md" ? 16 : 10,
        x: groupCenter.x + Math.cos(angle) * spread,
        y: groupCenter.y + Math.sin(angle) * spread,
        vx: 0,
        vy: 0,
      };
    });
    const stateByPath = Object.create(null);
    states.forEach(function (state) {
      stateByPath[state.node.path] = state;
    });

    for (let iteration = 0; iteration < 220; iteration += 1) {
      for (let i = 0; i < states.length; i += 1) {
        for (let j = i + 1; j < states.length; j += 1) {
          const a = states[i];
          const b = states[j];
          const dx = b.x - a.x || 0.01;
          const dy = b.y - a.y || 0.01;
          const distance = Math.max(9, Math.sqrt(dx * dx + dy * dy));
          const force = Math.min(42, 6200 / (distance * distance));
          const nx = dx / distance;
          const ny = dy / distance;
          a.vx -= nx * force;
          a.vy -= ny * force;
          b.vx += nx * force;
          b.vy += ny * force;
        }
      }

      links.forEach(function (edge) {
        const source = stateByPath[edge.source];
        const target = stateByPath[edge.target];
        if (!source || !target) {
          return;
        }
        const dx = target.x - source.x || 0.01;
        const dy = target.y - source.y || 0.01;
        const distance = Math.max(1, Math.sqrt(dx * dx + dy * dy));
        const linkedDegree = Math.max(1, Math.min(degree[edge.source], degree[edge.target]));
        const desired = Math.max(90, 152 - linkedDegree * 7);
        const force = (distance - desired) * 0.014;
        const nx = dx / distance;
        const ny = dy / distance;
        source.vx += nx * force;
        source.vy += ny * force;
        target.vx -= nx * force;
        target.vy -= ny * force;
      });

      applyGraphCollisionForces(states, 0.072);

      states.forEach(function (state) {
        const groupCenter = groupCenters[state.group] || center;
        const nodeDegree = degree[state.node.path] || 0;
        const centerPull = (state.node.sourcePath || state.node.path) === "index.md" ? 0.04 : 0.002 + Math.min(nodeDegree, 8) * 0.0008;
        const groupPull = nodeDegree > 0 ? 0.006 : 0.018;
        state.vx += (center.x - state.x) * centerPull;
        state.vy += (center.y - state.y) * centerPull;
        state.vx += (groupCenter.x - state.x) * groupPull;
        state.vy += (groupCenter.y - state.y) * groupPull;
        state.x += state.vx;
        state.y += state.vy;
        graphClampState(state, width, height);
        state.vx *= 0.62;
        state.vy *= 0.62;
      });
    }

    fitGraphLayout(states, width, height);
    resolveGraphCollisions(states, width, height);
    states.forEach(function (state) {
      positions[state.node.path] = { x: state.x, y: state.y };
    });
    return positions;
  }

  function graphScalableLayoutPositions(nodes, width, height) {
    const positions = Object.create(null);
    const groups = new Map();
    nodes.forEach(function (node) {
      const key = String(node.knowledgeBase || graphPathGroup(node.sourcePath || node.path) || ".");
      const group = groups.get(key) || [];
      group.push(node);
      groups.set(key, group);
    });
    const orderedGroups = Array.from(groups.entries()).sort(function (left, right) {
      return right[1].length - left[1].length || left[0].localeCompare(right[0]);
    });
    const columns = Math.max(1, Math.ceil(Math.sqrt(orderedGroups.length * (width / height))));
    const rows = Math.max(1, Math.ceil(orderedGroups.length / columns));
    const cellWidth = width / columns;
    const cellHeight = height / rows;
    orderedGroups.forEach(function (entry, groupIndex) {
      const groupNodes = entry[1].slice().sort(function (left, right) {
        return String(left.path).localeCompare(String(right.path));
      });
      const centerX = cellWidth * (groupIndex % columns + 0.5);
      const centerY = cellHeight * (Math.floor(groupIndex / columns) + 0.5);
      const spacing = Math.max(18, Math.min(38, Math.sqrt((cellWidth * cellHeight) / Math.max(1, groupNodes.length)) * 0.72));
      groupNodes.forEach(function (node, index) {
        const angle = index * 2.399963229728653;
        const radius = spacing * Math.sqrt(index);
        positions[node.path] = {
          x: clamp(centerX + Math.cos(angle) * radius, 28, width - 28),
          y: clamp(centerY + Math.sin(angle) * radius, 28, height - 28),
        };
      });
    });
    return positions;
  }

  function graphGridPositions(nodes, width, height) {
    const positions = Object.create(null);
    const sorted = nodes.slice().sort(function (left, right) {
      const leftGroup = graphPathGroup(left.sourcePath || left.path);
      const rightGroup = graphPathGroup(right.sourcePath || right.path);
      if (leftGroup !== rightGroup) {
        return leftGroup.localeCompare(rightGroup);
      }
      return left.path.localeCompare(right.path);
    });
    const columns = Math.max(1, Math.ceil(Math.sqrt(sorted.length * (width / height))));
    const rows = Math.max(1, Math.ceil(sorted.length / columns));
    const margin = 36;
    const columnWidth = Math.max(1, (width - margin * 2) / Math.max(1, columns - 1));
    const rowHeight = Math.max(1, (height - margin * 2) / Math.max(1, rows - 1));
    sorted.forEach(function (node, index) {
      positions[node.path] = {
        x: margin + (index % columns) * columnWidth,
        y: margin + Math.floor(index / columns) * rowHeight,
      };
    });
    return positions;
  }

  function graphGroupCenters(nodes, width, height) {
    const counts = Object.create(null);
    nodes.forEach(function (node) {
      const group = graphPathGroup(node.sourcePath || node.path);
      counts[group] = (counts[group] || 0) + 1;
    });
    const groups = Object.keys(counts).sort(function (a, b) {
      if (counts[b] === counts[a]) {
        return a.localeCompare(b);
      }
      return counts[b] - counts[a];
    });
    const centers = Object.create(null);
    if (groups.length === 0) {
      return centers;
    }

    const columns = Math.max(1, Math.ceil(Math.sqrt(groups.length * (width / height))));
    const rows = Math.max(1, Math.ceil(groups.length / columns));
    const cellWidth = width / columns;
    const cellHeight = height / rows;
    groups.forEach(function (group, index) {
      const column = index % columns;
      const row = Math.floor(index / columns);
      const hash = graphHash(group);
      const jitterX = ((hash % 31) - 15) * 0.9;
      const jitterY = (((hash >> 5) % 31) - 15) * 0.9;
      centers[group] = {
        x: cellWidth * (column + 0.5) + jitterX,
        y: cellHeight * (row + 0.5) + jitterY,
      };
    });
    return centers;
  }

  function graphPathGroup(path) {
    const parts = graphPathParts(path);
    if (parts.length <= 1) {
      return ".";
    }
    if (parts.length >= 3) {
      return parts.slice(0, 2).join("/");
    }
    return parts[0];
  }

  function fitGraphLayout(states, width, height) {
    let minX = Infinity;
    let maxX = -Infinity;
    let minY = Infinity;
    let maxY = -Infinity;
    states.forEach(function (state) {
      minX = Math.min(minX, state.x);
      maxX = Math.max(maxX, state.x);
      minY = Math.min(minY, state.y);
      maxY = Math.max(maxY, state.y);
    });
    const paddingX = 74;
    const paddingY = 58;
    const spanX = Math.max(1, maxX - minX);
    const spanY = Math.max(1, maxY - minY);
    const scale = Math.min((width - paddingX * 2) / spanX, (height - paddingY * 2) / spanY, 3.2);
    const sourceCenterX = (minX + maxX) / 2;
    const sourceCenterY = (minY + maxY) / 2;
    const targetCenterX = width / 2;
    const targetCenterY = height / 2;
    states.forEach(function (state) {
      state.x = clamp(targetCenterX + (state.x - sourceCenterX) * scale, paddingX, width - paddingX);
      state.y = clamp(targetCenterY + (state.y - sourceCenterY) * scale, paddingY, height - paddingY);
      graphClampState(state, width, height);
    });
  }

  function applyGraphCollisionForces(states, strength) {
    const boxes = states.map(graphNodeCollisionBox);
    for (let i = 0; i < states.length; i += 1) {
      for (let j = i + 1; j < states.length; j += 1) {
        const overlap = graphBoxOverlap(boxes[i], boxes[j]);
        if (!overlap) {
          continue;
        }
        const a = states[i];
        const b = states[j];
        const dx = b.x - a.x || 0.01;
        const dy = b.y - a.y || 0.01;
        if (overlap.x < overlap.y) {
          const push = overlap.x * strength * Math.sign(dx);
          a.vx -= push;
          b.vx += push;
        } else {
          const push = overlap.y * strength * Math.sign(dy);
          a.vy -= push;
          b.vy += push;
        }
      }
    }
  }

  function resolveGraphCollisions(states, width, height) {
    for (let iteration = 0; iteration < 96; iteration += 1) {
      let moved = false;
      const boxes = states.map(graphNodeCollisionBox);
      for (let i = 0; i < states.length; i += 1) {
        for (let j = i + 1; j < states.length; j += 1) {
          const overlap = graphBoxOverlap(boxes[i], boxes[j]);
          if (!overlap) {
            continue;
          }
          const a = states[i];
          const b = states[j];
          const dx = b.x - a.x || 0.01;
          const dy = b.y - a.y || 0.01;
          if (overlap.x < overlap.y) {
            const push = (overlap.x / 2 + 2.5) * Math.sign(dx);
            a.x -= push;
            b.x += push;
          } else {
            const push = (overlap.y / 2 + 2.5) * Math.sign(dy);
            a.y -= push;
            b.y += push;
          }
          graphClampState(a, width, height);
          graphClampState(b, width, height);
          moved = true;
        }
      }
      if (!moved) {
        return;
      }
    }
  }

  function graphNodeCollisionBox(state) {
    const labelWidth = graphLabelWidth(state.label);
    const labelTop = state.y + ((state.node.sourcePath || state.node.path) === "index.md" ? 19 : 15);
    const labelBottom = labelTop + 20;
    const halfWidth = Math.max(state.radius + 10, labelWidth / 2 + 11);
    return {
      left: state.x - halfWidth,
      right: state.x + halfWidth,
      top: Math.min(state.y - state.radius - 6, labelTop),
      bottom: Math.max(state.y + state.radius + 6, labelBottom),
    };
  }

  function graphLabelWidth(label) {
    return Math.max(20, String(label || "").length * 7.2);
  }

  function graphBoxOverlap(a, b) {
    const x = Math.min(a.right, b.right) - Math.max(a.left, b.left);
    if (x <= 0) {
      return null;
    }
    const y = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
    if (y <= 0) {
      return null;
    }
    return { x: x, y: y };
  }

  function graphClampState(state, width, height) {
    const box = graphNodeCollisionBox(state);
    if (box.left < 14) {
      state.x += 14 - box.left;
    }
    if (box.right > width - 14) {
      state.x -= box.right - (width - 14);
    }
    if (box.top < 18) {
      state.y += 18 - box.top;
    }
    if (box.bottom > height - 18) {
      state.y -= box.bottom - (height - 18);
    }
  }

  function graphHash(value) {
    let hash = 2166136261;
    const text = String(value || "");
    for (let index = 0; index < text.length; index += 1) {
      hash ^= text.charCodeAt(index);
      hash = Math.imul(hash, 16777619);
    }
    return hash >>> 0;
  }

  function graphUniqueNodeLabels(nodes) {
    const groups = Object.create(null);
    nodes.forEach(function (node) {
      if (!node || typeof node.path !== "string") {
        return;
      }
      const base = graphNodeBaseLabel(node);
      if (!groups[base]) {
        groups[base] = [];
      }
      groups[base].push(node);
    });

    const labels = Object.create(null);
    Object.keys(groups).forEach(function (base) {
      const peers = groups[base];
      if (peers.length === 1) {
        labels[peers[0].path] = base;
        return;
      }
      const peerPaths = peers.map(function (node) { return node.sourcePath || node.path; });
      peers.forEach(function (node) {
        labels[node.path] = graphShortestUniquePathSuffix(node.sourcePath || node.path, peerPaths);
      });
    });
    return labels;
  }

  function graphNodeBaseLabel(node) {
    const title = String(node.title || "").trim();
    if (title && title.toLowerCase() !== "index") {
      return title;
    }
    return graphPathDisplayName(node.sourcePath || node.path);
  }

  function graphPathDisplayName(path) {
    const parts = graphPathParts(path);
    if (parts.length === 0) {
      return String(path || "");
    }
    const last = parts[parts.length - 1];
    if (last.toLowerCase() === "index" && parts.length > 1) {
      return parts.slice(-2).join("/");
    }
    return last;
  }

  function graphShortestUniquePathSuffix(path, peers) {
    const parts = graphPathParts(path);
    for (let length = 1; length <= parts.length; length += 1) {
      const suffix = parts.slice(-length).join("/");
      const unique = peers.every(function (peer) {
        return peer === path || graphPathParts(peer).slice(-length).join("/") !== suffix;
      });
      if (unique) {
        return suffix;
      }
    }
    return parts.join("/") || String(path || "");
  }

  function graphPathParts(path) {
    return String(path || "").replace(/\.md$/i, "").split("/").filter(Boolean);
  }

  function graphNodeLabel(node, labelsByPath) {
    const label = graphNodeFullLabel(node, labelsByPath);
    return label.length > 22 ? label.slice(0, 21) + "..." : label;
  }

  function graphNodeFullLabel(node, labelsByPath) {
    return labelsByPath[node.path] || graphNodeBaseLabel(node);
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
        staticHTMLAliases(note.htmlPath).forEach(function (alias) {
          if (indexed[alias] === undefined) {
            indexed[alias] = note.path;
          }
        });
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

  function staticHTMLAliases(htmlPath) {
    const normalized = normalizeStaticPath(htmlPath);
    const aliases = [];
    addStaticHTMLAlias(aliases, normalized);
    if (/\.html$/i.test(normalized)) {
      const extensionless = normalized.slice(0, -5);
      addStaticHTMLAlias(aliases, extensionless);
      if (/\/index\.html$/i.test(normalized)) {
        addStaticHTMLAlias(aliases, normalized.slice(0, normalized.length - "index.html".length));
      } else if (/^index\.html$/i.test(normalized)) {
        addStaticHTMLAlias(aliases, "");
      }
    }
    aliases.slice().forEach(function (alias) {
      addStaticHTMLAlias(aliases, alias.toLowerCase());
    });
    return aliases;
  }

  function addStaticHTMLAlias(aliases, value) {
    const normalized = normalizeStaticPath(value);
    if (!aliases.includes(normalized)) {
      aliases.push(normalized);
    }
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
      return staticNotePathForHTMLPath(sourceDirectory + withoutFragment);
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

    return staticNotePathForHTMLPath(staticRelativeHTMLPathFromURL(url));
  }

  function staticNotePathForHTMLPath(htmlPath) {
    const normalized = normalizeStaticPath(htmlPath);
    return staticNotePathByHTML[normalized] || staticNotePathByHTML[normalized.toLowerCase()] || null;
  }

  function staticRelativeHTMLPathFromURL(url) {
    const currentPath = document.querySelector("[data-note-path]")?.dataset.notePath || currentStack()[0] || "index.md";
    let currentURLPath = safeDecodePath(window.location.pathname);
    let targetURLPath = safeDecodePath(url.pathname);
    const rootPrefix = staticRootPrefixFromCurrentURL(currentURLPath, currentPath);
    if (rootPrefix && targetURLPath.toLowerCase().startsWith(rootPrefix.toLowerCase())) {
      targetURLPath = targetURLPath.slice(rootPrefix.length);
    }
    return normalizeStaticPath(targetURLPath);
  }

  function staticRootPrefixFromCurrentURL(currentURLPath, currentPath) {
    const currentPathLower = String(currentURLPath || "").toLowerCase();
    const aliases = staticHTMLAliases(staticHTMLPath(currentPath))
      .filter(Boolean)
      .sort(function (a, b) {
        return b.length - a.length;
      });
    for (const alias of aliases) {
      const suffixes = ["/" + alias, "/" + alias + "/"];
      for (const suffix of suffixes) {
        if (currentPathLower.endsWith(suffix.toLowerCase())) {
          return currentURLPath.slice(0, currentURLPath.length - suffix.length + 1);
        }
      }
    }
    return currentURLPath.slice(0, currentURLPath.lastIndexOf("/") + 1);
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
    const shared = window.OpenKnowledgeStaticData?.editors;
    if (Array.isArray(shared) && shared.length) {
      return shared;
    }
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

    if (name === "github") {
      const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
      path.setAttribute("d", "M12 .5a12 12 0 0 0-3.79 23.39c.6.11.82-.26.82-.58v-2.17c-3.34.73-4.04-1.42-4.04-1.42-.55-1.39-1.34-1.76-1.34-1.76-1.09-.75.08-.73.08-.73 1.2.08 1.84 1.24 1.84 1.24 1.07 1.83 2.8 1.3 3.49.99.11-.78.42-1.3.76-1.6-2.67-.3-5.47-1.33-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.13-.3-.54-1.52.11-3.18 0 0 1.01-.32 3.3 1.23a11.4 11.4 0 0 1 6 0c2.29-1.55 3.3-1.23 3.3-1.23.65 1.66.24 2.88.12 3.18.77.84 1.23 1.91 1.23 3.22 0 4.61-2.81 5.63-5.48 5.92.43.37.81 1.1.81 2.22v3.29c0 .32.22.69.83.58A12 12 0 0 0 12 .5Z");
      svg.append(path);
      return svg;
    }

    if (name === "volume") {
      const speaker = document.createElementNS("http://www.w3.org/2000/svg", "path");
      speaker.setAttribute("d", "M11 5 6 9H3v6h3l5 4V5Z");
      const sound = document.createElementNS("http://www.w3.org/2000/svg", "path");
      sound.setAttribute("d", "M15.5 8.5a5 5 0 0 1 0 7M18 6a8.5 8.5 0 0 1 0 12");
      svg.append(speaker, sound);
      return svg;
    }

    if (name === "pause") {
      const left = document.createElementNS("http://www.w3.org/2000/svg", "path");
      left.setAttribute("d", "M8 5v14");
      const right = document.createElementNS("http://www.w3.org/2000/svg", "path");
      right.setAttribute("d", "M16 5v14");
      svg.append(left, right);
      return svg;
    }

    if (name === "play") {
      const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
      path.setAttribute("d", "m8 5 11 7-11 7V5Z");
      svg.append(path);
      return svg;
    }

    if (name === "stop") {
      const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
      path.setAttribute("d", "M7 7h10v10H7z");
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

  function narrationIsSupported() {
    return typeof window.speechSynthesis !== "undefined" && typeof window.SpeechSynthesisUtterance === "function";
  }

  function createNarrationControls(path) {
    const fragment = document.createDocumentFragment();
    const toggle = document.createElement("button");
    toggle.className = "note-narration";
    toggle.type = "button";
    toggle.dataset.noteNarration = "";
    toggle.hidden = true;
    toggle.setAttribute("aria-label", "Listen to " + path);
    toggle.setAttribute("aria-pressed", "false");
    toggle.title = "Listen to this page";
    toggle.append(controlIcon("volume", "note-narration-icon"));

    const stop = document.createElement("button");
    stop.className = "note-narration note-narration-stop";
    stop.type = "button";
    stop.dataset.noteNarrationStop = "";
    stop.hidden = true;
    stop.setAttribute("aria-label", "Stop narration of " + path);
    stop.title = "Stop narration";
    stop.append(controlIcon("stop", "note-narration-icon"));

    const status = document.createElement("span");
    status.className = "sr-only";
    status.dataset.noteNarrationStatus = "";
    status.setAttribute("role", "status");
    status.setAttribute("aria-live", "polite");

    fragment.append(toggle, stop, status);
    return fragment;
  }

  function narrationText(panel) {
    const body = panel.querySelector(":scope > .note-body");
    if (!body || body.classList.contains("asset-code") || body.classList.contains("asset-text")) {
      return "";
    }
    const readable = body.cloneNode(true);
    readable.querySelectorAll([
      "[data-frontmatter]",
      "[data-okf-annotation]",
      ".ok-agent-footer",
      ".ok-agent-context",
      ".ok-table-tools",
      ".ok-mermaid",
      "pre",
      "script",
      "style",
      "noscript",
      "button",
      "input",
      "select",
      "textarea",
      "[hidden]",
      "[aria-hidden='true']",
    ].join(",")).forEach(function (element) {
      element.remove();
    });
    readable.querySelectorAll("br").forEach(function (element) {
      element.replaceWith(document.createTextNode(". "));
    });
    readable.querySelectorAll("h1, h2, h3, h4, h5, h6, p, li, blockquote, dt, dd, th, td, figcaption").forEach(function (element) {
      element.append(document.createTextNode(". "));
    });
    return (readable.textContent || "")
      .replace(/\s+/g, " ")
      .replace(/\s+([,.;:!?])/g, "$1")
      .replace(/([.!?]){2,}/g, "$1")
      .trim();
  }

  function narrationChunks(text) {
    const maxLength = 1600;
    const chunks = [];
    let remaining = text;
    while (remaining.length > maxLength) {
      let splitAt = remaining.lastIndexOf(" ", maxLength);
      if (splitAt < maxLength * 0.6) {
        splitAt = maxLength;
      }
      chunks.push(remaining.slice(0, splitAt).trim());
      remaining = remaining.slice(splitAt).trim();
    }
    if (remaining) {
      chunks.push(remaining);
    }
    return chunks;
  }

  function announceNarration(panel, message) {
    const status = panel?.querySelector("[data-note-narration-status]");
    if (status) {
      status.textContent = message;
    }
  }

  function syncNarrationControls() {
    panels().forEach(function (panel) {
      const toggle = panel.querySelector("[data-note-narration]");
      const stop = panel.querySelector("[data-note-narration-stop]");
      if (!toggle || !stop) {
        return;
      }
      const active = narrationState.panel === panel && narrationState.status !== "idle";
      const paused = active && narrationState.status === "paused";
      toggle.setAttribute("aria-pressed", active ? "true" : "false");
      toggle.setAttribute("aria-label", (paused ? "Resume narration of " : active ? "Pause narration of " : "Listen to ") + panel.dataset.notePath);
      toggle.title = paused ? "Resume narration" : active ? "Pause narration" : "Listen to this page";
      toggle.dataset.narrationState = active ? narrationState.status : "idle";
      toggle.replaceChildren(controlIcon(paused ? "play" : active ? "pause" : "volume", "note-narration-icon"));
      if (!active && document.activeElement === stop) {
        toggle.focus();
      }
      stop.hidden = !active;
    });
  }

  function finishNarration(message) {
    const panel = narrationState.panel;
    narrationState.token += 1;
    narrationState.panel = null;
    narrationState.chunks = [];
    narrationState.index = 0;
    narrationState.status = "idle";
    syncNarrationControls();
    announceNarration(panel, message);
  }

  function speakNarrationChunk(token) {
    if (token !== narrationState.token || narrationState.status === "idle") {
      return;
    }
    if (narrationState.index >= narrationState.chunks.length) {
      finishNarration("Narration finished.");
      return;
    }
    const utterance = new window.SpeechSynthesisUtterance(narrationState.chunks[narrationState.index]);
    utterance.onend = function () {
      if (token !== narrationState.token) {
        return;
      }
      narrationState.index += 1;
      speakNarrationChunk(token);
    };
    utterance.onerror = function () {
      if (token === narrationState.token) {
        finishNarration("Narration could not continue.");
      }
    };
    window.speechSynthesis.speak(utterance);
  }

  function startNarration(panel) {
    const text = narrationText(panel);
    if (!text) {
      announceNarration(panel, "This page has no reader text to narrate.");
      return;
    }
    if (narrationState.panel) {
      narrationState.token += 1;
      window.speechSynthesis.cancel();
    }
    narrationState.token += 1;
    narrationState.panel = panel;
    narrationState.chunks = narrationChunks(text);
    narrationState.index = 0;
    narrationState.status = "speaking";
    const token = narrationState.token;
    syncNarrationControls();
    announceNarration(panel, "Narrating " + panel.dataset.notePath + ".");
    speakNarrationChunk(token);
  }

  function toggleNarration(panel) {
    if (narrationState.panel !== panel || narrationState.status === "idle") {
      startNarration(panel);
      return;
    }
    if (narrationState.status === "paused") {
      window.speechSynthesis.resume();
      narrationState.status = "speaking";
      announceNarration(panel, "Narration resumed.");
    } else {
      window.speechSynthesis.pause();
      narrationState.status = "paused";
      announceNarration(panel, "Narration paused.");
    }
    syncNarrationControls();
  }

  function stopNarration(panel, message) {
    if (!narrationState.panel || (panel && narrationState.panel !== panel)) {
      return;
    }
    narrationState.token += 1;
    window.speechSynthesis.cancel();
    finishNarration(message || "Narration stopped.");
  }

  function bindNarration(panel) {
    const toggle = panel.querySelector("[data-note-narration]");
    const stop = panel.querySelector("[data-note-narration-stop]");
    if (!toggle || !stop || toggle.dataset.narrationBound === "true") {
      return;
    }
    toggle.dataset.narrationBound = "true";
    if (!narrationIsSupported() || !narrationText(panel)) {
      return;
    }
    toggle.hidden = false;
    toggle.addEventListener("click", function (event) {
      event.stopPropagation();
      toggleNarration(panel);
    });
    stop.addEventListener("click", function (event) {
      event.stopPropagation();
      stopNarration(panel);
    });
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

  function createSourceButton(notePath, sourceURL) {
    if (!sourceURL) {
      return null;
    }
    const sourceLink = document.createElement("a");
    sourceLink.className = "source-open";
    sourceLink.href = sourceURL;
    sourceLink.dataset.sourceOpen = "";
    sourceLink.dataset.directLink = "true";
    sourceLink.target = "_blank";
    sourceLink.rel = "noreferrer";
    sourceLink.title = "Open on GitHub";
    sourceLink.setAttribute("aria-label", "Open " + notePath + " on GitHub");
    sourceLink.append(controlIcon("github", "source-icon"));
    return sourceLink;
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

  function setSettingsOpen(open) {
    if (!settingsTrigger || !settingsMenu) {
      return;
    }
    settingsMenu.hidden = !open;
    settingsTrigger.setAttribute("aria-expanded", open ? "true" : "false");
  }

  function ensureSettingsResetButton() {
    if (!settingsMenu) {
      return null;
    }
    const existing = settingsMenu.querySelector("[data-settings-reset]");
    if (existing) {
      return existing;
    }
    const footer = document.createElement("div");
    footer.className = "viewer-settings-footer";
    const reset = document.createElement("button");
    reset.type = "button";
    reset.className = "viewer-settings-reset";
    reset.dataset.settingsReset = "";
    reset.textContent = "Reset to defaults";
    footer.append(reset);
    settingsMenu.append(footer);
    return reset;
  }

  function bindViewerSettings() {
    if (!settings || !settingsTrigger || !settingsMenu || settings.dataset.settingsBound === "true") {
      return;
    }
    settings.dataset.settingsBound = "true";
    let preference = readThemePreference();
    let frontmatterVisible = readFrontmatterPreference();
    let accessibilityPreference = readAccessibilityPreference();
    applyThemePreference(preference);
    applyFrontmatterPreference(frontmatterVisible);
    applyAccessibilityPreference(accessibilityPreference);
    const resetButton = ensureSettingsResetButton();
    if (resetButton) {
      resetButton.addEventListener("click", function () {
        preference = normalizeThemePreference({ preset: defaultThemePreset, custom: customThemeDefaults });
        frontmatterVisible = true;
        accessibilityPreference = normalizeAccessibilityPreference(defaultAccessibilityPreference);
        navigationMode = defaultNavigationMode;
        saveThemePreference(preference);
        saveFrontmatterPreference(frontmatterVisible);
        saveAccessibilityPreference(accessibilityPreference);
        saveNavigationModePreference(navigationMode);
        applyThemePreference(preference);
        applyFrontmatterPreference(frontmatterVisible);
        applyAccessibilityPreference(accessibilityPreference);
        applyNavigationMode(navigationMode);
      });
    }

    settingsTrigger.addEventListener("click", function (event) {
      event.preventDefault();
      event.stopPropagation();
      setSettingsOpen(settingsMenu.hidden);
    });
    settingsTrigger.addEventListener("keydown", function (event) {
      if (event.key !== "ArrowDown" && event.key !== "Enter" && event.key !== " ") {
        return;
      }
      event.preventDefault();
      setSettingsOpen(true);
      const selected = settingsMenu.querySelector("[data-theme-option].is-selected") || settingsMenu.querySelector("[data-theme-option]");
      if (selected) {
        selected.focus();
      }
    });

    settingsMenu.addEventListener("click", function (event) {
      const option = closestElement(event.target, "[data-theme-option]");
      if (!option) {
        return;
      }
      event.preventDefault();
      preference = normalizeThemePreference({ preset: option.dataset.themeOption, custom: preference.custom });
      saveThemePreference(preference);
      applyThemePreference(preference);
    });
    settingsMenu.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        event.preventDefault();
        setSettingsOpen(false);
        settingsTrigger.focus();
        return;
      }
      if (event.key !== "Enter" && event.key !== " ") {
        return;
      }
      const option = closestElement(event.target, "[data-theme-option]");
      if (!option) {
        return;
      }
      event.preventDefault();
      preference = normalizeThemePreference({ preset: option.dataset.themeOption, custom: preference.custom });
      saveThemePreference(preference);
      applyThemePreference(preference);
    });

    settingsMenu.querySelectorAll("[data-theme-custom-value]").forEach(function (input) {
      input.addEventListener("input", function () {
        const key = input.dataset.themeCustomValue;
        if (!customThemeDefaults[key] || !isHexColor(input.value)) {
          return;
        }
        const custom = Object.assign({}, preference.custom);
        custom[key] = input.value.toLowerCase();
        preference = normalizeThemePreference({ preset: "custom", custom: custom });
        saveThemePreference(preference);
        applyThemePreference(preference);
      });
    });
    if (frontmatterVisibility) {
      frontmatterVisibility.addEventListener("change", function () {
        frontmatterVisible = frontmatterVisibility.checked;
        saveFrontmatterPreference(frontmatterVisible);
        applyFrontmatterPreference(frontmatterVisible);
      });
    }

    function updateAccessibilityPreference(key, value) {
      accessibilityPreference = normalizeAccessibilityPreference(Object.assign({}, accessibilityPreference, { [key]: value }));
      saveAccessibilityPreference(accessibilityPreference);
      applyAccessibilityPreference(accessibilityPreference);
    }

    if (accessibilityFont) {
      accessibilityFont.addEventListener("change", function () {
        updateAccessibilityPreference("font", accessibilityFont.value);
      });
    }
    if (accessibilitySize) {
      accessibilitySize.addEventListener("change", function () {
        updateAccessibilityPreference("size", accessibilitySize.value);
      });
    }
    if (accessibilitySpacing) {
      accessibilitySpacing.addEventListener("change", function () {
        updateAccessibilityPreference("spacing", accessibilitySpacing.value);
      });
    }
    if (accessibilityMotion) {
      accessibilityMotion.addEventListener("change", function () {
        updateAccessibilityPreference("motion", accessibilityMotion.value);
      });
    }
    if (readableLineLength) {
      readableLineLength.addEventListener("change", function () {
        updateAccessibilityPreference("readableLineLength", readableLineLength.checked);
      });
    }
    if (highContrast) {
      highContrast.addEventListener("change", function () {
        updateAccessibilityPreference("highContrast", highContrast.checked);
      });
    }
    if (underlineLinks) {
      underlineLinks.addEventListener("change", function () {
        updateAccessibilityPreference("underlineLinks", underlineLinks.checked);
      });
    }
  }

  function activePanel() {
    return stackEl.querySelector(".note-panel.is-active-panel");
  }

  function focusedPanel() {
    return closestElement(document.activeElement, "[data-note-path]");
  }

  function closeablePanel() {
    return focusedPanel() || activePanel();
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
    syncKnowledgeTrees(panel.dataset.notePath, false, panel.dataset.knowledgeBase);
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

  function ensurePanelResizeHandles(panel) {
    if (!panel || panel.dataset.resizeHandlesBound === "true") {
      return;
    }
    panel.dataset.resizeHandlesBound = "true";
    ["left", "right"].forEach(function (edge) {
      const handle = document.createElement("button");
      handle.type = "button";
      handle.className = "note-resize-handle note-resize-handle-" + edge;
      handle.dataset.panelResizeHandle = edge;
      handle.setAttribute("aria-label", "Resize note panel from the " + edge);
      handle.title = "Resize panel";
      handle.addEventListener("pointerdown", startPanelResize);
      handle.addEventListener("keydown", function (event) {
        resizePanelWithKeyboard(panel, edge, event);
      });
      panel.append(handle);
    });
    syncPanelResizeHandles(panel);
    panel.addEventListener("scroll", function () {
      syncPanelResizeHandles(panel);
    }, { passive: true });
  }

  function syncPanelResizeHandles(panel) {
    if (!panel) {
      return;
    }
    panel.style.setProperty("--note-panel-scroll-top", Math.max(0, panel.scrollTop || 0) + "px");
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

  function currentStackTargets() {
    return panels().map(function (panel) {
      return noteTarget(panel.dataset.notePath, panel.dataset.knowledgeBase);
    });
  }

  function serializeStackTarget(target, baseKnowledgeBase) {
    const normalized = typeof target === "string" ? noteTarget(target, baseKnowledgeBase) : noteTarget(target?.path, target?.knowledgeBase);
    if (!normalized.knowledgeBase || normalized.knowledgeBase === baseKnowledgeBase) {
      return normalized.path.startsWith("@") ? "@" + normalized.path : normalized.path;
    }
    return "@" + encodeURIComponent(normalized.knowledgeBase) + "/" + normalized.path;
  }

  function parseStackTarget(value, baseKnowledgeBase) {
    const raw = String(value || "");
    if (raw.startsWith("@@")) {
      return noteTarget(raw.slice(1), baseKnowledgeBase);
    }
    if (!raw.startsWith("@")) {
      return noteTarget(raw, baseKnowledgeBase);
    }
    const slash = raw.indexOf("/");
    if (slash < 2) {
      return noteTarget(raw, baseKnowledgeBase);
    }
    try {
      return noteTarget(raw.slice(slash + 1), decodeURIComponent(raw.slice(1, slash)));
    } catch {
      return noteTarget(raw.slice(slash + 1), raw.slice(1, slash));
    }
  }

  function stackFromLocation() {
    const params = new URLSearchParams(window.location.search);
    if (params.get("empty") === "1") {
      return [];
    }
    const base = noteTargetFromHref(window.location.href) || currentStackTargets()[0] || noteTarget("index.md");
    return [base].concat(params.getAll("stack").filter(Boolean).map(function (value) {
      return parseStackTarget(value, base.knowledgeBase);
    }));
  }

  function highlightFromLocation() {
    return highlightFromHref(window.location.href);
  }

  function highlightFromHref(href) {
    let url;
    try {
      url = new URL(href, window.location.href);
    } catch {
      return "";
    }
    return (url.searchParams.get("ok-highlight") || "").trim();
  }

  function stackURL(paths, highlightText) {
    const targets = paths.map(function (target) {
      return typeof target === "string" ? noteTarget(target) : noteTarget(target?.path, target?.knowledgeBase);
    });
    if (!targets.length) {
      const emptyURL = new URL(fileURL("index.md", currentKnowledgeBase), window.location.href);
      emptyURL.searchParams.set("empty", "1");
      return emptyURL;
    }

    const first = targets[0];
    const url = new URL(fileURL(first.path || "index.md", first.knowledgeBase), window.location.href);
    targets.slice(1).forEach(function (target) {
      url.searchParams.append("stack", serializeStackTarget(target, first.knowledgeBase));
    });
    if (highlightText) {
      url.searchParams.set("ok-highlight", highlightText);
    }
    return url;
  }

  function updateWorkspaceState() {
    const panelCount = panels().length;
    const isEmpty = panelCount === 0;
    const showClaims = claimsViewIsVisible();
    const showGraph = !showClaims && (graphViewRequested || isEmpty);
    if (showGraph && !knowledgeGraphRendered) {
      knowledgeGraphRendered = true;
      void ensureKnowledgeGraphData().then(renderKnowledgeGraph).catch(function () {
        knowledgeGraphRendered = false;
        renderKnowledgeGraph();
      });
    }
    const graphWasVisible = document.documentElement.dataset.viewerView === "graph";
    if (showGraph && !graphWasVisible) {
      const graphSidebar = document.querySelector("[data-knowledge-graph-sidebar]");
      const graphSettingsToggle = document.querySelector(".knowledge-graph-settings-toggle");
      if (graphSidebar) {
        graphSidebar.hidden = true;
      }
      graphSettingsToggle?.setAttribute("aria-expanded", "false");
    }
    workspace.classList.toggle("is-empty", isEmpty);
    workspace.classList.toggle("is-graph-view", showGraph);
    workspace.classList.toggle("is-claims-view", showClaims);
    workspace.classList.toggle("is-single-panel", !showGraph && !showClaims && panelCount === 1);
    workspace.classList.toggle("is-multi-panel", !showGraph && !showClaims && panelCount > 1);
    if (emptyState) {
      emptyState.hidden = !showGraph;
    }
    if (claimsWorkspace) {
      claimsWorkspace.hidden = !showClaims;
    }
    if (graphViewToggle) {
      graphViewToggle.dataset.active = showGraph ? "true" : "false";
      graphViewToggle.setAttribute("aria-pressed", showGraph ? "true" : "false");
      if (showGraph) {
        graphViewToggle.setAttribute("aria-current", "page");
      } else {
        graphViewToggle.removeAttribute("aria-current");
      }
    }
    const sidebarGraphToggle = fileSidebar?.querySelector("[data-sidebar-graph-toggle]");
    if (sidebarGraphToggle) {
      sidebarGraphToggle.dataset.active = showGraph ? "true" : "false";
      sidebarGraphToggle.setAttribute("aria-pressed", showGraph ? "true" : "false");
      if (showGraph) {
        sidebarGraphToggle.setAttribute("aria-current", "page");
      } else {
        sidebarGraphToggle.removeAttribute("aria-current");
      }
    }
    if (documentsViewToggle) {
      documentsViewToggle.dataset.active = showGraph || showClaims ? "false" : "true";
      if (showGraph || showClaims) {
        documentsViewToggle.removeAttribute("aria-current");
      } else {
        documentsViewToggle.setAttribute("aria-current", "page");
      }
    }
    claimsViewToggles.forEach(function (toggle) {
      const active = showClaims && claimsToggleKnowledgeBase(toggle) === activeClaimsKnowledgeBase;
      toggle.dataset.active = active ? "true" : "false";
      toggle.setAttribute("aria-pressed", active ? "true" : "false");
      if (active) {
        toggle.setAttribute("aria-current", "page");
      } else {
        toggle.removeAttribute("aria-current");
      }
    });
    stackEl.hidden = showGraph || showClaims;
    document.documentElement.dataset.viewerView = showClaims ? "claims" : showGraph ? "graph" : "notes";
    panels().forEach(applyPanelWidth);
    ensureActivePanel();
    updateCloseLinks();
    updateSpacePanState();
    queueWorkspaceRailUpdate();
  }

  function updateCloseLinks() {
    const paths = currentStackTargets();
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

  function maxWorkspaceScroll() {
    return Math.max(0, workspace.scrollWidth - workspace.clientWidth);
  }

  function canShowWorkspaceRail() {
    return Boolean(scrollRail && scrollTrack && scrollThumb && panels().length > 1 && maxWorkspaceScroll() > 1 && !workspace.classList.contains("is-graph-view"));
  }

  function queueWorkspaceRailUpdate() {
    if (workspaceRailFrame) {
      return;
    }
    workspaceRailFrame = window.requestAnimationFrame(function () {
      workspaceRailFrame = 0;
      updateWorkspaceRail();
    });
  }

  function updateWorkspaceRail() {
    if (!scrollRail || !scrollTrack || !scrollThumb) {
      return;
    }

    if (!canShowWorkspaceRail()) {
      scrollRail.hidden = true;
      scrollRail.setAttribute("aria-hidden", "true");
      scrollThumb.style.width = "";
      scrollThumb.style.setProperty("--thumb-x", "0px");
      scrollThumb.setAttribute("aria-valuemax", "0");
      scrollThumb.setAttribute("aria-valuenow", "0");
      return;
    }

    scrollRail.hidden = false;
    scrollRail.setAttribute("aria-hidden", "false");

    const trackWidth = scrollTrack.getBoundingClientRect().width;
    if (trackWidth <= 0) {
      return;
    }
    const maxScroll = maxWorkspaceScroll();
    const thumbWidth = clamp(trackWidth * (workspace.clientWidth / workspace.scrollWidth), 44, trackWidth);
    const maxThumbX = Math.max(0, trackWidth - thumbWidth);
    const thumbX = maxScroll > 0 ? (workspace.scrollLeft / maxScroll) * maxThumbX : 0;

    scrollThumb.style.width = thumbWidth + "px";
    scrollThumb.style.setProperty("--thumb-x", clamp(thumbX, 0, maxThumbX) + "px");
    scrollThumb.setAttribute("aria-valuemax", String(Math.round(maxScroll)));
    scrollThumb.setAttribute("aria-valuenow", String(Math.round(workspace.scrollLeft)));
  }

  function scrollWorkspaceFromRail(clientX, thumbOffset, geometry) {
    if (!geometry && !canShowWorkspaceRail()) {
      return;
    }
    const trackRect = geometry ? null : scrollTrack.getBoundingClientRect();
    const thumbRect = geometry ? null : scrollThumb.getBoundingClientRect();
    const trackLeft = geometry ? geometry.trackLeft : trackRect.left;
    const maxThumbX = geometry ? geometry.maxThumbX : Math.max(0, trackRect.width - thumbRect.width);
    const maxScroll = geometry ? geometry.maxScroll : maxWorkspaceScroll();
    const thumbX = clamp(clientX - trackLeft - thumbOffset, 0, maxThumbX);
    workspace.scrollLeft = maxThumbX > 0 ? (thumbX / maxThumbX) * maxScroll : 0;
  }

  function updateTitle() {
    if (claimsViewIsVisible()) {
      document.title = (activeClaimsKnowledgeBase ? "Claims in " + activeClaimsKnowledgeBase : "Claims") + " - Open Knowledge";
      return;
    }
    if (graphViewIsVisible()) {
      document.title = "Graph view - Open Knowledge";
      return;
    }
    const all = panels();
    const currentPanel = activePanel() || all[all.length - 1];
    if (!currentPanel) {
      document.title = "Knowledge base - Open Knowledge";
      return;
    }
    const title = currentPanel?.dataset.noteTitle || currentPanel?.dataset.notePath || "Open Knowledge";
    document.title = title + " - Open Knowledge";
  }

  function updateHistory(paths, pushHistory, highlightText) {
    const nextURL = stackURL(paths, highlightText);
    const state = { stack: paths };
    if (pushHistory) {
      window.history.pushState(state, "", nextURL);
    } else {
      window.history.replaceState(state, "", nextURL);
    }
  }

  function scheduleMermaidThemeRender() {
    closeMermaidViewport();
    window.clearTimeout(mermaidThemeTimer);
    mermaidThemeTimer = window.setTimeout(function () {
      enhanceMermaid(stackEl, true);
    }, 60);
  }

  function mermaidThemeValue(styles, name, fallback) {
    return styles.getPropertyValue(name).trim() || fallback;
  }

  function mermaidConfiguration() {
    const styles = window.getComputedStyle(document.documentElement);
    const page = mermaidThemeValue(styles, "--ok-color-page", "#ffffff");
    const surface = mermaidThemeValue(styles, "--ok-color-surface", page);
    const text = mermaidThemeValue(styles, "--ok-color-text", "#202322");
    const muted = mermaidThemeValue(styles, "--ok-color-muted", "#707773");
    const accent = mermaidThemeValue(styles, "--ok-color-accent", "#0a4a9c");
    const border = mermaidThemeValue(styles, "--ok-color-border", "#e3e6e4");
    return {
      startOnLoad: false,
      securityLevel: "strict",
      suppressErrorRendering: true,
      logLevel: "fatal",
      theme: "base",
      themeVariables: {
        background: page,
        primaryColor: surface,
        primaryTextColor: text,
        primaryBorderColor: border,
        lineColor: muted,
        secondaryColor: page,
        secondaryTextColor: text,
        secondaryBorderColor: border,
        tertiaryColor: surface,
        tertiaryTextColor: text,
        tertiaryBorderColor: border,
        mainBkg: surface,
        nodeBorder: accent,
        clusterBkg: page,
        clusterBorder: border,
        titleColor: text,
        edgeLabelBackground: surface,
        textColor: text,
        fontFamily: mermaidThemeValue(styles, "--ok-font-body", "sans-serif")
      },
      flowchart: {
        htmlLabels: false
      }
    };
  }

  function mermaidSourceBlocks(scope) {
    const blocks = [];
    if (scope?.matches?.("[data-mermaid-source]")) {
      blocks.push(scope);
    }
    scope?.querySelectorAll?.("[data-mermaid-source]").forEach(function (block) {
      blocks.push(block);
    });
    return blocks;
  }

  function prepareMermaidBlock(sourceBlock) {
    const existing = closestElement(sourceBlock, "[data-mermaid-diagram]");
    if (existing) {
      return existing;
    }

    const diagram = document.createElement("figure");
    diagram.className = "ok-mermaid";
    diagram.dataset.mermaidDiagram = "";

    const output = document.createElement("div");
    output.className = "ok-mermaid-output";
    output.dataset.mermaidOutput = "";
    output.setAttribute("role", "img");
    output.setAttribute("aria-label", "Mermaid diagram");
    output.hidden = true;

    const error = document.createElement("p");
    error.className = "ok-mermaid-error";
    error.dataset.mermaidError = "";
    error.setAttribute("role", "status");
    error.textContent = "This Mermaid diagram could not be rendered. Its source is shown below.";
    error.hidden = true;

    sourceBlock.parentNode.insertBefore(diagram, sourceBlock);
    diagram.append(output, error, sourceBlock);
    diagram._openKnowledgeMermaidSource = String(sourceBlock.querySelector("code")?.textContent || sourceBlock.textContent || "");
    return diagram;
  }

  function mermaidDiagramLabel(diagram) {
    const panel = closestElement(diagram, "[data-note-path]");
    const title = panel?.querySelector(".note-body h1")?.textContent?.trim() || panel?.dataset.noteTitle || "document";
    const diagrams = Array.from(panel?.querySelectorAll("[data-mermaid-diagram]") || [diagram]);
    return "Mermaid diagram " + (Math.max(0, diagrams.indexOf(diagram)) + 1) + " in " + title;
  }

  function enhanceMermaid(scope, force) {
    if (!window.mermaid || typeof window.mermaid.render !== "function") {
      return;
    }
    const diagrams = mermaidSourceBlocks(scope).map(prepareMermaidBlock).filter(function (diagram) {
      const state = diagram.dataset.mermaidState || "";
      return force || (state !== "queued" && state !== "rendering" && state !== "rendered");
    });
    if (!diagrams.length) {
      return;
    }

    const requestID = ++mermaidRequestID;
    diagrams.forEach(function (diagram) {
      diagram.dataset.mermaidState = "queued";
      diagram._openKnowledgeMermaidRequest = requestID;
    });
    mermaidRenderQueue = mermaidRenderQueue.catch(function () {
      // A malformed diagram must not prevent later diagrams from rendering.
    }).then(function () {
      return renderMermaidBatch(diagrams, requestID);
    });
  }

  async function renderMermaidBatch(diagrams, requestID) {
    try {
      window.mermaid.initialize(mermaidConfiguration());
    } catch {
      diagrams.forEach(function (diagram) {
        const sourceBlock = diagram.querySelector("[data-mermaid-source]");
        const output = diagram.querySelector("[data-mermaid-output]");
        const error = diagram.querySelector("[data-mermaid-error]");
        if (diagram._openKnowledgeMermaidRequest !== requestID || !sourceBlock || !output || !error) {
          return;
        }
        output.hidden = true;
        sourceBlock.hidden = false;
        error.hidden = false;
        diagram.dataset.mermaidState = "error";
      });
      return;
    }
    for (const diagram of diagrams) {
      if (diagram._openKnowledgeMermaidRequest !== requestID) {
        continue;
      }
      const sourceBlock = diagram.querySelector("[data-mermaid-source]");
      const output = diagram.querySelector("[data-mermaid-output]");
      const error = diagram.querySelector("[data-mermaid-error]");
      if (!sourceBlock || !output || !error) {
        continue;
      }

      diagram.dataset.mermaidState = "rendering";
      output.setAttribute("aria-busy", "true");
      try {
        const result = await window.mermaid.render("ok-mermaid-" + (++mermaidRenderID), diagram._openKnowledgeMermaidSource);
        if (diagram._openKnowledgeMermaidRequest !== requestID) {
          continue;
        }
        output.innerHTML = result.svg;
        if (typeof result.bindFunctions === "function") {
          result.bindFunctions(output);
        }
        output.hidden = false;
        sourceBlock.hidden = true;
        error.hidden = true;
        bindMermaidViewport(output, mermaidDiagramLabel(diagram));
        diagram.dataset.mermaidState = "rendered";
      } catch {
        if (diagram._openKnowledgeMermaidRequest !== requestID) {
          continue;
        }
        if (diagram.dataset.mermaidRendered !== "true") {
          output.hidden = true;
          sourceBlock.hidden = false;
          error.hidden = false;
        }
        diagram.dataset.mermaidState = "error";
      } finally {
        output.removeAttribute("aria-busy");
      }
      if (diagram.dataset.mermaidState === "rendered") {
        diagram.dataset.mermaidRendered = "true";
      }
    }
  }

  function enhanceTables(scope) {
    scope.querySelectorAll("table").forEach(function (table) {
      if (table.dataset.okTableEnhanced === "true") {
        return;
      }
      const headerRow = table.tHead?.rows?.[0];
      const body = table.tBodies?.[0];
      if (!headerRow || !body) {
        return;
      }
      const headers = Array.prototype.slice.call(headerRow.cells);
      const rows = Array.prototype.slice.call(body.rows);
      if (!headers.length || !rows.length) {
        return;
      }

      table.classList.add("ok-table");
      table.dataset.okTable = "";
      table.dataset.okTableEnhanced = "true";
      rows.forEach(function (row, index) {
        row.dataset.okTableOriginalIndex = String(index);
      });

      const wrapper = ensureTableWrapper(table);
      const state = {
        query: "",
        filters: headers.map(function () { return ""; }),
        sortColumn: -1,
        sortDirection: "asc"
      };
      const count = document.createElement("span");
      count.className = "ok-table-count";
      count.dataset.okTableCount = "";

      function applyTableState() {
        let visible = 0;
        rows.forEach(function (row) {
          const matchesQuery = !state.query || normalizeTableText(row.textContent).includes(state.query);
          const matchesFilters = state.filters.every(function (filter, column) {
            return !filter || normalizeTableText(tableCellText(row.cells[column])) === filter;
          });
          const shown = matchesQuery && matchesFilters;
          row.hidden = !shown;
          if (shown) {
            visible += 1;
          }
        });
        count.textContent = visible === rows.length
          ? rowCountLabel(rows.length)
          : visible + " / " + rowCountLabel(rows.length);
      }

      headers.forEach(function (header, column) {
        bindSortableTableHeader(header, body, headers, rows, state, column);
      });

      const controls = createTableControls(headers, rows, state, count, applyTableState);
      wrapper.insertBefore(controls, wrapper.firstChild);
      applyTableState();
    });
  }

  function ensureTableWrapper(table) {
    let wrapper = closestElement(table, "[data-ok-table-wrap]");
    if (!wrapper) {
      wrapper = document.createElement("div");
      wrapper.className = "ok-table-wrap";
      wrapper.dataset.okTableWrap = "";
      const scroller = document.createElement("div");
      scroller.className = "ok-table-scroller";
      table.parentNode.insertBefore(wrapper, table);
      wrapper.append(scroller);
      scroller.append(table);
      return wrapper;
    }

    if (!closestElement(table, ".ok-table-scroller")) {
      const scroller = document.createElement("div");
      scroller.className = "ok-table-scroller";
      table.parentNode.insertBefore(scroller, table);
      scroller.append(table);
    }
    return wrapper;
  }

  function createTableControls(headers, rows, state, count, applyTableState) {
    const controls = document.createElement("div");
    controls.className = "ok-table-tools";
    controls.dataset.okTableControls = "";

    const search = document.createElement("input");
    search.className = "ok-table-search";
    search.type = "search";
    search.placeholder = "Filter table";
    search.setAttribute("aria-label", "Filter table rows");
    search.addEventListener("input", function () {
      state.query = normalizeTableText(search.value);
      applyTableState();
    });
    controls.append(search);

    const filterList = document.createElement("div");
    filterList.className = "ok-table-filter-list";
    const filterSelects = [];
    let filterLabel;
    let clearFilters;

    function updateFilterMenuState() {
      const activeFilters = state.filters.filter(Boolean).length;
      if (filterLabel) {
        filterLabel.textContent = activeFilters ? "Filters (" + activeFilters + ")" : "Filters";
      }
      if (clearFilters) {
        clearFilters.disabled = activeFilters === 0;
      }
    }

    headers.forEach(function (header, column) {
      const values = tableColumnFilterValues(rows, column);
      if (values.length < 2 || values.length > 30) {
        return;
      }

      const select = document.createElement("select");
      const label = tableCellText(header) || "Column " + (column + 1);
      select.setAttribute("aria-label", "Filter by " + label);
      const all = document.createElement("option");
      all.value = "";
      all.textContent = label + ": All";
      select.append(all);

      values.forEach(function (value) {
        const option = document.createElement("option");
        option.value = normalizeTableText(value);
        option.textContent = value;
        select.append(option);
      });

      select.addEventListener("change", function () {
        state.filters[column] = select.value;
        updateFilterMenuState();
        applyTableState();
      });
      filterSelects.push(select);
      filterList.append(select);
    });

    if (filterList.children.length) {
      const menu = document.createElement("details");
      menu.className = "ok-table-filter-menu";
      const trigger = document.createElement("summary");
      trigger.className = "ok-table-filter-trigger";
      trigger.setAttribute("role", "button");
      trigger.setAttribute("aria-label", "Table filters");
      filterLabel = document.createElement("span");
      filterLabel.textContent = "Filters";
      trigger.append(filterLabel);
      const panel = document.createElement("div");
      panel.className = "ok-table-filter-panel";
      clearFilters = document.createElement("button");
      clearFilters.className = "ok-table-clear";
      clearFilters.type = "button";
      clearFilters.textContent = "Clear filters";
      clearFilters.addEventListener("click", function () {
        state.filters = state.filters.map(function () { return ""; });
        filterSelects.forEach(function (select) {
          select.value = "";
        });
        updateFilterMenuState();
        applyTableState();
        if (filterSelects[0]) {
          filterSelects[0].focus();
        }
      });
      panel.append(filterList, clearFilters);
      menu.append(trigger, panel);
      menu.addEventListener("keydown", function (event) {
        if (event.key !== "Escape") {
          return;
        }
        menu.open = false;
        trigger.focus();
      });
      controls.append(menu);
      updateFilterMenuState();
    }

    controls.append(count);
    return controls;
  }

  function bindSortableTableHeader(header, body, headers, rows, state, column) {
    header.dataset.okTableSort = "";
    header.tabIndex = 0;
    header.setAttribute("aria-label", "Sort by " + (tableCellText(header) || "column " + (column + 1)));
    if (!header.querySelector(".ok-table-sort-indicator")) {
      const indicator = document.createElement("span");
      indicator.className = "ok-table-sort-indicator";
      indicator.setAttribute("aria-hidden", "true");
      header.append(indicator);
    }

    function activate(event) {
      if (event && closestElement(event.target, "a[href], button, input, textarea, select, [contenteditable='true']")) {
        return;
      }
      sortTableRows(body, headers, rows, state, column);
    }

    header.addEventListener("click", activate);
    header.addEventListener("keydown", function (event) {
      if (event.key !== "Enter" && event.key !== " ") {
        return;
      }
      event.preventDefault();
      activate(event);
    });
  }

  function sortTableRows(body, headers, rows, state, column) {
    const direction = state.sortColumn === column && state.sortDirection === "asc" ? "desc" : "asc";
    state.sortColumn = column;
    state.sortDirection = direction;

    headers.forEach(function (header) {
      header.removeAttribute("aria-sort");
      header.removeAttribute("data-sort-direction");
    });
    headers[column].setAttribute("aria-sort", direction === "asc" ? "ascending" : "descending");
    headers[column].dataset.sortDirection = direction;

    const multiplier = direction === "asc" ? 1 : -1;
    rows.sort(function (left, right) {
      const compared = compareTableValues(tableCellText(left.cells[column]), tableCellText(right.cells[column]));
      if (compared !== 0) {
        return compared * multiplier;
      }
      return Number(left.dataset.okTableOriginalIndex || 0) - Number(right.dataset.okTableOriginalIndex || 0);
    });
    rows.forEach(function (row) {
      body.append(row);
    });
  }

  function tableColumnFilterValues(rows, column) {
    const seen = new Set();
    const values = [];
    rows.forEach(function (row) {
      const value = tableCellText(row.cells[column]);
      const normalized = normalizeTableText(value);
      if (!normalized || seen.has(normalized) || value.length > 80) {
        return;
      }
      seen.add(normalized);
      values.push(value);
    });
    return values.sort(function (left, right) {
      return compareTableValues(left, right);
    });
  }

  function compareTableValues(left, right) {
    const leftText = normalizeTableText(left);
    const rightText = normalizeTableText(right);
    if (!leftText && rightText) {
      return 1;
    }
    if (leftText && !rightText) {
      return -1;
    }
    const leftNumber = tableNumber(leftText);
    const rightNumber = tableNumber(rightText);
    if (leftNumber !== null && rightNumber !== null && leftNumber !== rightNumber) {
      return leftNumber - rightNumber;
    }
    return leftText.localeCompare(rightText, undefined, { numeric: true, sensitivity: "base" });
  }

  function tableNumber(value) {
    const normalized = String(value || "").replace(/,/g, "");
    if (!/^[+-]?\d+(?:\.\d+)?%?$/.test(normalized)) {
      return null;
    }
    return Number(normalized.replace(/%$/, ""));
  }

  function tableCellText(cell) {
    return String(cell?.textContent || "").replace(/\s+/g, " ").trim();
  }

  function normalizeTableText(value) {
    return tableCellText({ textContent: String(value || "") }).toLocaleLowerCase();
  }

  function rowCountLabel(count) {
    return count + (count === 1 ? " row" : " rows");
  }

  function updateActiveLinks() {
    const all = panels();
    all.forEach(function (panel) {
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
    const currentPanel = activePanel() || all[all.length - 1];
    syncKnowledgeTrees(currentPanel?.dataset.notePath || "", false, currentPanel?.dataset.knowledgeBase);
  }

  function scrollToPanel(panel) {
    setActivePanel(panel);
    window.requestAnimationFrame(function () {
      panel.scrollIntoView({
        block: "nearest",
        inline: "end",
        behavior: motionIsReduced() ? "auto" : "smooth"
      });
      panel.focus({ preventScroll: true });
    });
  }

  function clearSearchHighlights(scope) {
    const root = scope || document;
    root.querySelectorAll("mark.ok-search-highlight").forEach(function (mark) {
      const parent = mark.parentNode;
      if (!parent) {
        return;
      }
      mark.replaceWith.apply(mark, Array.prototype.slice.call(mark.childNodes));
      parent.normalize();
    });
  }

  function applySearchHighlight(panel, highlightText) {
    clearSearchHighlights(stackEl);
    const text = String(highlightText || "").trim();
    const body = panel?.querySelector(".note-body");
    if (!body || !text) {
      return;
    }
    window.requestAnimationFrame(function () {
      const range = searchHighlightRange(body, text);
      if (!range) {
        return;
      }
      const mark = document.createElement("mark");
      mark.className = "ok-search-highlight";
      mark.dataset.searchHighlight = "";
      mark.append(range.extractContents());
      range.insertNode(mark);
      mark.scrollIntoView({
        block: "center",
        inline: "nearest",
        behavior: motionIsReduced() ? "auto" : "smooth"
      });
      panel.focus({ preventScroll: true });
    });
  }

  function searchHighlightRange(root, text) {
    const needle = normalizeHighlightText(text);
    if (!needle) {
      return null;
    }
    const haystack = normalizedTextPositions(root);
    const index = haystack.text.indexOf(needle);
    if (index < 0) {
      return null;
    }
    let start = index;
    let end = index + needle.length;
    while (start < end && haystack.text[start] === " ") {
      start++;
    }
    while (end > start && haystack.text[end - 1] === " ") {
      end--;
    }
    const first = haystack.positions[start];
    const last = haystack.positions[end - 1];
    if (!first || !last) {
      return null;
    }
    const range = document.createRange();
    range.setStart(first.node, first.start);
    range.setEnd(last.node, last.end);
    return range;
  }

  function normalizedTextPositions(root) {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    const parts = [];
    const positions = [];
    let previousSpace = true;
    let node;
    while ((node = walker.nextNode())) {
      if (closestElement(node.parentElement, "[hidden], [aria-hidden='true']")) {
        continue;
      }
      const value = node.nodeValue || "";
      for (let index = 0; index < value.length; index += 1) {
        const normalized = normalizeHighlightCharacter(value[index]);
        if (normalized) {
          parts.push(normalized);
          positions.push({ node: node, start: index, end: index + 1 });
          previousSpace = false;
          continue;
        }
        if (!previousSpace) {
          parts.push(" ");
          positions.push({ node: node, start: index, end: index + 1 });
        }
        previousSpace = true;
      }
    }
    while (parts.length && parts[parts.length - 1] === " ") {
      parts.pop();
      positions.pop();
    }
    return { text: parts.join(""), positions: positions };
  }

  function normalizeHighlightText(value) {
    const parts = [];
    let previousSpace = true;
    Array.from(String(value || "")).forEach(function (character) {
      const normalized = normalizeHighlightCharacter(character);
      if (normalized) {
        parts.push(normalized);
        previousSpace = false;
        return;
      }
      if (!previousSpace) {
        parts.push(" ");
      }
      previousSpace = true;
    });
    while (parts.length && parts[parts.length - 1] === " ") {
      parts.pop();
    }
    return parts.join("");
  }

  function normalizeHighlightCharacter(character) {
    const normalized = String(character || "").toLocaleLowerCase().normalize("NFD").replace(/[\u0300-\u036f]/g, "");
    return /^[\p{Letter}\p{Number}]$/u.test(normalized) ? normalized : "";
  }

  async function fetchNote(path, knowledgeBase) {
    if (isStaticBundle()) {
      const note = staticNotesByPath[path];
      if (!note) {
        throw new Error("Could not open " + path);
      }
      return note;
    }

    const response = await fetch(apiURL(path, knowledgeBase), {
      headers: { "Accept": "application/json" }
    });
    if (!response.ok) {
      throw new Error("Could not open " + path);
    }
    return response.json();
  }

  async function liveReloadPathExists(target) {
    const normalized = typeof target === "string" ? noteTarget(target) : noteTarget(target?.path, target?.knowledgeBase);
    const response = await fetch(apiURL(normalized.path, normalized.knowledgeBase), {
      cache: "no-store",
      headers: { "Accept": "application/json" }
    });
    if (response.status === 404) {
      return false;
    }
    if (!response.ok) {
      throw new Error("Could not verify " + normalized.path);
    }
    return true;
  }

  async function prepareLiveReload(revision) {
    if (isStaticBundle()) {
      return;
    }
    const stack = currentStackTargets();
    const active = activePanel();
    const activeTarget = active ? noteTarget(active.dataset.notePath, active.dataset.knowledgeBase) : stack[0] || null;
    const activeKey = activeTarget ? serializeStackTarget(activeTarget, "") : "";
    const panelScrollTop = {};
    panels().forEach(function (panel) {
      if (panel.dataset.notePath) {
        panelScrollTop[serializeStackTarget(noteTarget(panel.dataset.notePath, panel.dataset.knowledgeBase), "")] = panel.scrollTop || 0;
      }
    });
    const survivors = [];
    for (const target of stack) {
      if (await liveReloadPathExists(target)) {
        survivors.push(target);
      }
    }
    const fallback = noteTarget("index.md", currentKnowledgeBase);
    if (!survivors.length && await liveReloadPathExists(fallback)) {
      survivors.push(fallback);
    }
    const survivorKeys = survivors.map(function (target) { return serializeStackTarget(target, ""); });
    const nextActivePath = survivorKeys.includes(activeKey) ? activeKey : survivorKeys[0] || "";
    saveLiveReloadState({
      version: 1,
      revision: String(revision || ""),
      createdAt: Date.now(),
      stack: survivors,
      activePath: nextActivePath,
      view: workspaceViewName(),
      claim: selectedClaimKey,
      claimsKnowledgeBase: activeClaimsKnowledgeBase,
      workspaceScrollLeft: workspace.scrollLeft || 0,
      panelScrollTop: panelScrollTop
    });
    if (!survivors.length) {
      window.location.assign((linkPrefix || "") + "/");
      return;
    }
    updateHistory(survivors, false, highlightFromLocation());
    window.location.reload();
  }

  function restoreLiveReloadSession(state) {
    if (!state) {
      return;
    }
    const matchingActive = panels().find(function (panel) {
      const key = serializeStackTarget(noteTarget(panel.dataset.notePath, panel.dataset.knowledgeBase), "");
      return key === state.activePath || panel.dataset.notePath === state.activePath;
    });
    if (matchingActive) {
      setActivePanel(matchingActive);
    }
    if (state.claim && claimByKey(state.claim)) {
      selectedClaimKey = state.claim;
    }
    if (state.claimsKnowledgeBase) {
      activeClaimsKnowledgeBase = state.claimsKnowledgeBase;
    }
    claimsViewRequested = state.view === "claims" && claimsViewCanOpen(claimsData, activeClaimsKnowledgeBase);
    setGraphViewRequested(state.view === "graph", false);
    if (claimsViewRequested) {
      setClaimsViewRequested(true, false);
    }
    window.requestAnimationFrame(function () {
      window.requestAnimationFrame(function () {
        const scrollByPath = state.panelScrollTop && typeof state.panelScrollTop === "object" ? state.panelScrollTop : {};
        panels().forEach(function (panel) {
          const key = serializeStackTarget(noteTarget(panel.dataset.notePath, panel.dataset.knowledgeBase), "");
          const requested = Number(scrollByPath[key] || scrollByPath[panel.dataset.notePath] || 0);
          panel.scrollTop = clamp(requested, 0, Math.max(0, panel.scrollHeight - panel.clientHeight));
        });
        const requestedWorkspace = Number(state.workspaceScrollLeft || 0);
        workspace.scrollLeft = clamp(requestedWorkspace, 0, maxWorkspaceScroll());
        queueWorkspaceRailUpdate();
      });
    });
  }

  function createPanel(data, animate) {
    const panel = document.createElement("article");
    panel.className = "document note-panel" + (animate && stackMotionIsEnabled() ? " is-entering" : "");
    panel.dataset.notePath = data.path;
    panel.dataset.noteTitle = data.title || data.path;
    const panelKnowledgeBase = String(data.knowledgeBase || currentKnowledgeBase || "").trim();
    if (panelKnowledgeBase) {
      panel.dataset.knowledgeBase = panelKnowledgeBase;
    }
    panel.tabIndex = -1;

    const chrome = document.createElement("div");
    chrome.className = "note-chrome";

    chrome.append(createNoteBreadcrumbs(data.path, panelKnowledgeBase));

    const actions = document.createElement("div");
    actions.className = "note-actions";
    const sourceButton = createSourceButton(data.path, data.sourceURL);
    if (sourceButton) {
      actions.append(sourceButton);
    } else if (!isStaticBundle()) {
      actions.append(createEditorPicker());
    }
    actions.append(createNarrationControls(data.path));

    const closeButton = document.createElement("a");
    closeButton.className = "note-close";
    closeButton.href = "#";
    closeButton.dataset.closePanel = "";
    closeButton.setAttribute("role", "button");
    closeButton.setAttribute("aria-label", "Close " + data.path);
    closeButton.title = "Close note (" + panelCloseShortcut.label + ")";
    closeButton.setAttribute("aria-keyshortcuts", panelCloseShortcut.ariaKeyShortcut);
    closeButton.append(controlIcon("x", "note-close-icon"));
    actions.append(closeButton);
    chrome.append(actions);

    const body = document.createElement("div");
    const assetKind = data.kind === "code" || data.kind === "text" ? data.kind : "";
    body.className = "note-body" + (assetKind ? " asset-" + assetKind : "");
    const frontmatterAndBody = (data.frontmatter || "") + data.body;
    body.innerHTML = data.claims ? (data.frontmatter || "") + data.claims + data.body : frontmatterAndBody;

    panel.append(chrome, body);
    bindPanel(panel);
    return panel;
  }

  function integratePanelFrontmatter(panel) {
    const frontmatter = panel.querySelector(".note-body > [data-frontmatter]");
    const chrome = panel.querySelector(":scope > .note-chrome");
    const summary = frontmatter?.querySelector(":scope > .ok-frontmatter-summary");
    const content = frontmatter?.querySelector(":scope > .ok-frontmatter-body");
    if (!frontmatter || !chrome || !summary || !content || frontmatter.dataset.panelIntegrated === "true") {
      return;
    }

    frontmatter.dataset.panelIntegrated = "true";
    frontmatter.classList.add("is-panel-integrated");
    chrome.after(frontmatter);
  }

  function integratePanelClaims(panel) {
    const claims = panel.querySelector(".note-body > [data-claims-panel]");
    const chrome = panel.querySelector(":scope > .note-chrome");
    const frontmatter = panel.querySelector(":scope > [data-frontmatter]");
    if (!claims || !chrome || claims.dataset.panelIntegrated === "true") {
      return;
    }
    claims.dataset.panelIntegrated = "true";
    claims.classList.add("is-panel-integrated");
    if (frontmatter) {
      frontmatter.after(claims);
    } else {
      chrome.after(claims);
    }
  }

  function findPanelAnchor(panel, id) {
    return Array.prototype.find.call(panel.querySelectorAll("[id]"), function (candidate) {
      return candidate.id === id;
    }) || null;
  }

  function bindClaimSectionMarkers(panel) {
    const grouped = new Map();
    panel.querySelectorAll("[data-claim-section-ref]").forEach(function (claim) {
      const anchor = String(claim.dataset.claimSectionRef || "").replace(/^#/, "");
      if (!anchor) {
        return;
      }
      const claims = grouped.get(anchor) || [];
      claims.push(claim);
      grouped.set(anchor, claims);
    });
    grouped.forEach(function (claims, anchor) {
      let target = findPanelAnchor(panel, anchor);
      if (!target) {
        return;
      }
      if (!/^H[1-6]$/.test(target.tagName)) {
        const next = target.nextElementSibling;
        if (next && /^H[1-6]$/.test(next.tagName)) {
          target = next;
        }
      }
      if (target.querySelector(":scope > [data-claim-section-marker]")) {
        return;
      }
      const marker = createClaimsElement("button", "ok-claim-section-marker", claims.length + (claims.length === 1 ? " claim" : " claims"));
      marker.type = "button";
      marker.dataset.claimSectionMarker = "";
      marker.addEventListener("click", function () {
        const panelDisclosure = panel.querySelector("[data-claims-panel]");
        if (panelDisclosure) {
          panelDisclosure.open = true;
        }
        claims[0].scrollIntoView({ block: "nearest", behavior: motionIsReduced() ? "auto" : "smooth" });
        claims[0].querySelector("summary, button")?.focus({ preventScroll: true });
      });
      target.append(marker);
    });
  }

  function updateLinkBehaviorHints(scope) {
    const root = scope || document;
    root.querySelectorAll("[data-tree-path], .search-result[href]").forEach(function (link) {
      link.title = navigationModeTitle();
    });
    root.querySelectorAll(".note-body a[href]").forEach(function (link) {
      const panel = link.closest("[data-note-path]");
      if (panel && notePathFromHref(link.getAttribute("href") || link.href, panel.dataset.notePath)) {
        link.title = navigationModeTitle();
      }
    });
  }

  function bindPanel(panel) {
    renderPanelBreadcrumbs(panel);
    integratePanelFrontmatter(panel);
    integratePanelClaims(panel);
    bindClaimSectionMarkers(panel);
    applyPanelWidth(panel);
    ensurePanelResizeHandles(panel);
    syncPanelCloseShortcut(panel);
    panel.querySelectorAll("[data-editor-picker]").forEach(bindEditorPicker);
    bindNarration(panel);
    enhanceMermaid(panel, false);
    enhanceTables(panel);
    updateLinkBehaviorHints(panel);

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

  function syncPanelCloseShortcut(panel) {
    const shortcutSystem = window.OpenKnowledgeShortcuts;
    const closeButton = panel.querySelector("[data-close-panel]");
    if (!shortcutSystem || !closeButton) {
      return;
    }
    const label = shortcutSystem.format(panelCloseShortcut);
    closeButton.title = "Close note (" + label + ")";
    closeButton.setAttribute("aria-keyshortcuts", shortcutSystem.ariaKeyShortcut(panelCloseShortcut));
  }

  function createErrorPanel(path, error, knowledgeBase) {
    const message = document.createElement("p");
    message.className = "note-error";
    const detail = error instanceof Error ? error.message : "";
    message.textContent = detail === "Failed to fetch"
      ? "Could not reach the local viewer server while opening " + path + ". Restart openknowledge view and refresh this page."
      : detail || "Could not open " + path;
    return createPanel({
      title: "Not found",
      path,
      knowledgeBase,
      body: message.outerHTML
    }, true);
  }

  async function panelForPath(path, animate, knowledgeBase) {
    try {
      const data = await fetchNote(path, knowledgeBase);
      if (!data.knowledgeBase && knowledgeBase) {
        data.knowledgeBase = knowledgeBase;
      }
      return createPanel(data, animate);
    } catch (error) {
      return createErrorPanel(path, error, knowledgeBase);
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

  function stackMotionIsEnabled() {
    return !mobileSidebar.matches && !motionIsReduced();
  }

  function canUseStackTransition() {
    return stackMotionIsEnabled() && typeof document.startViewTransition === "function";
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
      if (transition.updateCallbackDone) {
        await transition.updateCallbackDone;
      }
      try {
        await transition.finished;
      } catch {
        // Browser-driven transition aborts should not surface as app errors.
      }
    } finally {
      clearEnteringPanels();
      document.body.classList.remove("is-view-transitioning");
    }
  }

  function clearStack() {
    panels().forEach(function (panel) {
      stopNarration(panel);
      panel.remove();
    });
    updateWorkspaceState();
    updateActiveLinks();
    updateTitle();
  }

  function trimAfter(index) {
    panels().slice(index + 1).forEach(function (panel) {
      stopNarration(panel);
      panel.remove();
    });
    updateWorkspaceState();
    updateActiveLinks();
    updateTitle();
  }

  async function openInitialNote(targetPath, pushHistory, highlightText, targetKnowledgeBase) {
    const panel = await panelForPath(targetPath, true, targetKnowledgeBase);
    await runStackTransition(function () {
      clearStack();
      appendPanel(panel);
      updateHistory(currentStackTargets(), pushHistory, highlightText);
    });
    applySearchHighlight(panel, highlightText);
  }

  async function openTarget(targetPath, pushHistory, openBeside, highlightText, sourcePanel, targetKnowledgeBase) {
    if (graphViewRequested) {
      setGraphViewRequested(false, false);
    }
    if (claimsViewRequested) {
      claimsViewRequested = false;
      updateWorkspaceState();
    }
    const source = sourcePanel || activePanel();
    if (!source) {
      await openInitialNote(targetPath, pushHistory, highlightText, targetKnowledgeBase);
      return;
    }
    const normalizedTargetKnowledgeBase = String(targetKnowledgeBase || currentKnowledgeBase || "").trim();
    if (!openBeside && source.dataset.notePath === targetPath && String(source.dataset.knowledgeBase || currentKnowledgeBase || "").trim() === normalizedTargetKnowledgeBase) {
      setActivePanel(source);
      scrollToPanel(source);
      updateHistory(currentStackTargets(), pushHistory, highlightText);
      applySearchHighlight(source, highlightText);
      return;
    }
    if (openBeside) {
      await openFromPanel(source, targetPath, pushHistory, highlightText, targetKnowledgeBase);
      return;
    }
    await replaceFromPanel(source, targetPath, pushHistory, highlightText, targetKnowledgeBase);
  }

  async function replaceFromPanel(sourcePanel, targetPath, pushHistory, highlightText, targetKnowledgeBase) {
    const panel = await panelForPath(targetPath, true, targetKnowledgeBase);
    clearSearchHighlights(stackEl);
    await runStackTransition(function () {
      const all = panels();
      let sourceIndex = all.indexOf(sourcePanel);
      if (sourceIndex < 0) {
        sourceIndex = Math.max(0, all.length - 1);
      }
      all.slice(sourceIndex).forEach(function (item) {
        stopNarration(item);
        item.remove();
      });
      appendPanel(panel);
      updateHistory(currentStackTargets(), pushHistory, highlightText);
    });
    applySearchHighlight(panel, highlightText);
  }

  async function closePanel(panel, pushHistory) {
    const before = panels();
    const index = before.indexOf(panel);
    let nextPanel;

    await runStackTransition(function () {
      stopNarration(panel);
      panel.remove();

      const remaining = panels();
      updateWorkspaceState();
      updateActiveLinks();
      updateTitle();
      updateHistory(currentStackTargets(), pushHistory);

      if (!remaining.length) {
        return;
      }

      nextPanel = remaining[Math.min(Math.max(index - 1, 0), remaining.length - 1)];
      setActivePanel(nextPanel);
    });

    if (!nextPanel) {
      return;
    }
    scrollToPanel(nextPanel);
  }

  async function openFromPanel(sourcePanel, targetPath, pushHistory, highlightText, targetKnowledgeBase) {
    const panel = await panelForPath(targetPath, true, targetKnowledgeBase);
    clearSearchHighlights(stackEl);
    await runStackTransition(function () {
      const all = panels();
      let sourceIndex = all.indexOf(sourcePanel);
      if (sourceIndex < 0) {
        sourceIndex = all.length - 1;
      }

      trimAfter(sourceIndex);
      appendPanel(panel);

      updateHistory(currentStackTargets(), pushHistory, highlightText);
    });
    applySearchHighlight(panel, highlightText);
  }

  async function restoreStack(paths, highlightText) {
    const loadedPanels = [];
    for (const target of paths) {
      const normalized = typeof target === "string" ? noteTarget(target) : noteTarget(target?.path, target?.knowledgeBase);
      loadedPanels.push(await panelForPath(normalized.path, false, normalized.knowledgeBase));
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
    applySearchHighlight(activePanel(), highlightText);
  }

  const finePointer = window.matchMedia("(hover: hover) and (pointer: fine)");
  let workspaceDrag = null;
  let railDrag = null;
  let panelResize = null;
  let sidebarResize = null;
  let spacePanPressed = false;
  let suppressWorkspaceClickUntil = 0;

  function currentSidebarWidth() {
    const renderedWidth = fileSidebar?.getBoundingClientRect().width || 0;
    return renderedWidth > 0 ? renderedWidth : appliedSidebarWidth(sidebarWidth);
  }

  function startSidebarResize(event) {
    if (!sidebarResizeHandle || mobileSidebar.matches || event.button !== 0) {
      return;
    }
    sidebarResize = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startWidth: currentSidebarWidth(),
      moved: false
    };
    document.body.classList.add("is-sidebar-resizing");
    window.addEventListener("pointermove", updateSidebarResize);
    window.addEventListener("pointerup", stopSidebarResize);
    window.addEventListener("pointercancel", stopSidebarResize);
    window.addEventListener("blur", cancelSidebarResize);
    event.preventDefault();
    try {
      sidebarResizeHandle.setPointerCapture(event.pointerId);
    } catch {
      // Pointer capture can fail if the pointer is already released.
    }
  }

  function updateSidebarResize(event) {
    if (!sidebarResize || event.pointerId !== sidebarResize.pointerId) {
      return;
    }
    const deltaX = event.clientX - sidebarResize.startX;
    sidebarResize.moved = sidebarResize.moved || Math.abs(deltaX) > 2;
    setSidebarWidth(sidebarResize.startWidth + deltaX);
    event.preventDefault();
  }

  function finishSidebarResize(pointerId) {
    if (!sidebarResize) {
      return;
    }
    const resized = sidebarResize.moved;
    try {
      if (pointerId !== undefined) {
        sidebarResizeHandle?.releasePointerCapture(pointerId);
      }
    } catch {
      // Pointer capture can already be released by the browser.
    }
    sidebarResize = null;
    document.body.classList.remove("is-sidebar-resizing");
    window.removeEventListener("pointermove", updateSidebarResize);
    window.removeEventListener("pointerup", stopSidebarResize);
    window.removeEventListener("pointercancel", stopSidebarResize);
    window.removeEventListener("blur", cancelSidebarResize);
    if (resized) {
      saveSidebarWidth();
    }
  }

  function stopSidebarResize(event) {
    if (!sidebarResize || event.pointerId !== sidebarResize.pointerId) {
      return;
    }
    finishSidebarResize(event.pointerId);
  }

  function cancelSidebarResize() {
    finishSidebarResize();
  }

  function resizeSidebarWithKeyboard(event) {
    const key = (event.key || "").toLowerCase();
    const step = event.shiftKey ? 64 : 24;
    let nextWidth = appliedSidebarWidth(sidebarWidth);
    if (key === "arrowleft") {
      nextWidth -= step;
    } else if (key === "arrowright") {
      nextWidth += step;
    } else if (key === "home") {
      nextWidth = minSidebarWidth();
    } else if (key === "end") {
      nextWidth = maxSidebarWidth();
    } else {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    setSidebarWidth(nextWidth);
    saveSidebarWidth();
  }

  function isSpacePanKey(event) {
    return event.code === "Space" || event.key === " " || event.key === "Spacebar";
  }

  function isEditableTarget(target) {
    return Boolean(closestElement(target, "input, textarea, select, [contenteditable='true']"));
  }

  function isInteractiveShortcutTarget(target) {
    return Boolean(closestElement(target, "a[href], button, input, textarea, select, [contenteditable='true'], [role='button']"));
  }

  function canUseSpacePanShortcut() {
    return finePointer.matches && panels().length > 1 && !workspace.classList.contains("is-empty");
  }

  function isSpacePanActive() {
    return spacePanPressed && canUseSpacePanShortcut();
  }

  function updateSpacePanState() {
    workspace.classList.toggle("is-space-panning", isSpacePanActive());
  }

  function startSpacePan(event) {
    if (!isSpacePanKey(event) || event.defaultPrevented || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || isInteractiveShortcutTarget(event.target)) {
      return;
    }
    if (!canUseSpacePanShortcut()) {
      return;
    }
    spacePanPressed = true;
    updateSpacePanState();
    event.preventDefault();
  }

  function stopSpacePan(event) {
    if (!isSpacePanKey(event) || !spacePanPressed) {
      return;
    }
    spacePanPressed = false;
    updateSpacePanState();
    event.preventDefault();
  }

  function cancelSpacePan() {
    spacePanPressed = false;
    updateSpacePanState();
  }

  function suppressNextWorkspaceClick() {
    suppressWorkspaceClickUntil = Date.now() + 350;
    window.setTimeout(function () {
      if (Date.now() >= suppressWorkspaceClickUntil) {
        suppressWorkspaceClickUntil = 0;
      }
    }, 360);
  }

  function consumeSuppressedWorkspaceClick(event) {
    if (!suppressWorkspaceClickUntil || Date.now() > suppressWorkspaceClickUntil) {
      return false;
    }
    suppressWorkspaceClickUntil = 0;
    event.preventDefault();
    event.stopPropagation();
    return true;
  }

  function currentPanelWidth(panel) {
    return panel.getBoundingClientRect().width || savedPanelWidth(panel) || defaultPanelWidth();
  }

  function setPanelWidth(panel, width) {
    const nextWidth = normalizePanelWidth(width, panel);
    if (!nextWidth || !panel) {
      return null;
    }
    panel.style.setProperty("--note-panel-width", nextWidth + "px");
    panel.dataset.panelWidth = String(nextWidth);
    if (panel.dataset.notePath) {
      panelWidths[panel.dataset.notePath] = nextWidth;
    }
    queueWorkspaceRailUpdate();
    return nextWidth;
  }

  function panelResizeWidthChange(edge, deltaX, centered) {
    const directionalDelta = edge === "left" ? -deltaX : deltaX;
    return directionalDelta * (centered ? 2 : 1);
  }

  function resizePanelWithKeyboard(panel, edge, event) {
    const key = (event.key || "").toLowerCase();
    const currentWidth = currentPanelWidth(panel);
    const step = event.shiftKey ? 64 : 24;
    let nextWidth = currentWidth;
    if (key === "arrowleft") {
      nextWidth += edge === "left" ? step : -step;
    } else if (key === "arrowright") {
      nextWidth += edge === "right" ? step : -step;
    } else if (key === "home") {
      nextWidth = minPanelWidth();
    } else if (key === "end") {
      nextWidth = maxPanelWidth(panel);
    } else {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    const storedWidth = setPanelWidth(panel, nextWidth);
    if (!storedWidth) {
      return;
    }
    if (edge === "left" && !isSingleCenteredPanel(panel)) {
      workspace.scrollLeft += storedWidth - currentWidth;
    }
    savePanelWidths();
  }

  function startPanelResize(event) {
    const handle = closestElement(event.target, "[data-panel-resize-handle]");
    const panel = handle?.closest("[data-note-path]");
    if (!handle || !panel || event.button !== 0) {
      return;
    }
    panelResize = {
      pointerId: event.pointerId,
      panel: panel,
      handle: handle,
      edge: handle.dataset.panelResizeHandle === "left" ? "left" : "right",
      centered: isSingleCenteredPanel(panel),
      startX: event.clientX,
      startWidth: currentPanelWidth(panel),
      startScrollLeft: workspace.scrollLeft,
      moved: false
    };
    setActivePanel(panel);
    panel.classList.add("is-panel-resizing");
    document.body.classList.add("is-panel-resizing");
    window.addEventListener("pointermove", updatePanelResize);
    window.addEventListener("pointerup", stopPanelResize);
    window.addEventListener("pointercancel", stopPanelResize);
    window.addEventListener("blur", cancelPanelResize);
    event.preventDefault();
    event.stopPropagation();
    try {
      handle.setPointerCapture(event.pointerId);
    } catch {
      // Pointer capture can fail if the pointer is already released.
    }
  }

  function updatePanelResize(event) {
    if (!panelResize || event.pointerId !== panelResize.pointerId) {
      return;
    }
    const deltaX = event.clientX - panelResize.startX;
    if (Math.abs(deltaX) > 2) {
      panelResize.moved = true;
    }
    const requestedWidth = panelResize.startWidth + panelResizeWidthChange(panelResize.edge, deltaX, panelResize.centered);
    const nextWidth = setPanelWidth(panelResize.panel, requestedWidth);
    if (!nextWidth) {
      return;
    }
    if (panelResize.edge === "left" && !panelResize.centered) {
      workspace.scrollLeft = panelResize.startScrollLeft + (nextWidth - panelResize.startWidth);
    }
    event.preventDefault();
  }

  function finishPanelResize(pointerId) {
    if (!panelResize) {
      return;
    }
    const resized = panelResize.moved;
    panelResize.panel.classList.remove("is-panel-resizing");
    try {
      if (pointerId !== undefined) {
        panelResize.handle.releasePointerCapture(pointerId);
      }
    } catch {
      // Pointer capture can already be released by the browser.
    }
    panelResize = null;
    document.body.classList.remove("is-panel-resizing");
    window.removeEventListener("pointermove", updatePanelResize);
    window.removeEventListener("pointerup", stopPanelResize);
    window.removeEventListener("pointercancel", stopPanelResize);
    window.removeEventListener("blur", cancelPanelResize);
    if (resized) {
      savePanelWidths();
      suppressNextWorkspaceClick();
    }
  }

  function stopPanelResize(event) {
    if (!panelResize || event.pointerId !== panelResize.pointerId) {
      return;
    }
    finishPanelResize(event.pointerId);
  }

  function cancelPanelResize() {
    finishPanelResize();
  }

  function canStartWorkspaceDrag(event) {
    const pointerType = event.pointerType || "mouse";
    if (pointerType !== "mouse" || !finePointer.matches || event.button !== 0 || panels().length < 2) {
      return false;
    }
    if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
      return false;
    }
    if (isSpacePanActive()) {
      return !isEditableTarget(event.target);
    }
    return !closestElement(event.target, "[data-note-path], a, button, input, textarea, select, [contenteditable='true'], [role='button']");
  }

  function startWorkspaceDrag(event) {
    if (!canStartWorkspaceDrag(event)) {
      return;
    }
    const fromSpacePan = isSpacePanActive();
    workspaceDrag = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startScrollLeft: workspace.scrollLeft,
      moved: false,
      fromSpacePan: fromSpacePan
    };
    workspace.classList.add("is-drag-scrolling");
    if (fromSpacePan) {
      event.preventDefault();
    }
    try {
      workspace.setPointerCapture(event.pointerId);
    } catch {
      // Pointer capture can fail if the pointer is already released.
    }
  }

  function updateWorkspaceDrag(event) {
    if (!workspaceDrag || event.pointerId !== workspaceDrag.pointerId) {
      return;
    }
    const deltaX = event.clientX - workspaceDrag.startX;
    if (Math.abs(deltaX) < 3 && !workspaceDrag.moved) {
      return;
    }
    workspaceDrag.moved = true;
    workspace.scrollLeft = workspaceDrag.startScrollLeft - deltaX;
    event.preventDefault();
  }

  function stopWorkspaceDrag(event) {
    if (!workspaceDrag || event.pointerId !== workspaceDrag.pointerId) {
      return;
    }
    const drag = workspaceDrag;
    try {
      workspace.releasePointerCapture(event.pointerId);
    } catch {
      // Pointer capture can already be released by the browser.
    }
    if (drag.moved || drag.fromSpacePan) {
      suppressNextWorkspaceClick();
    }
    workspaceDrag = null;
    workspace.classList.remove("is-drag-scrolling");
  }

  function startRailDrag(event) {
    if (!canShowWorkspaceRail() || event.button !== 0) {
      return;
    }
    const trackRect = scrollTrack.getBoundingClientRect();
    const thumbRect = scrollThumb.getBoundingClientRect();
    railDrag = {
      pointerId: event.pointerId,
      thumbOffset: clamp(event.clientX - thumbRect.left, 0, thumbRect.width),
      trackLeft: trackRect.left,
      maxThumbX: Math.max(0, trackRect.width - thumbRect.width),
      maxScroll: maxWorkspaceScroll()
    };
    scrollRail.classList.add("is-rail-dragging");
    workspace.classList.add("is-rail-dragging");
    window.addEventListener("pointermove", updateRailDrag);
    window.addEventListener("pointerup", stopRailDrag);
    window.addEventListener("pointercancel", stopRailDrag);
    window.addEventListener("blur", cancelRailDrag);
    scrollWorkspaceFromRail(event.clientX, railDrag.thumbOffset, railDrag);
    event.preventDefault();
    try {
      scrollThumb.setPointerCapture(event.pointerId);
    } catch {
      // Pointer capture can fail if the pointer is already released.
    }
  }

  function startRailTrackJump(event) {
    if (!canShowWorkspaceRail() || event.button !== 0 || closestElement(event.target, "[data-workspace-scroll-thumb]")) {
      return;
    }
    const thumbRect = scrollThumb.getBoundingClientRect();
    scrollWorkspaceFromRail(event.clientX, thumbRect.width / 2);
    event.preventDefault();
  }

  function updateRailDrag(event) {
    if (!railDrag || event.pointerId !== railDrag.pointerId) {
      return;
    }
    scrollWorkspaceFromRail(event.clientX, railDrag.thumbOffset, railDrag);
    event.preventDefault();
  }

  function finishRailDrag(pointerId) {
    const releasedPointerId = pointerId ?? railDrag.pointerId;
    try {
      scrollThumb.releasePointerCapture(releasedPointerId);
    } catch {
      // Pointer capture can already be released by the browser.
    }
    railDrag = null;
    scrollRail.classList.remove("is-rail-dragging");
    workspace.classList.remove("is-rail-dragging");
    window.removeEventListener("pointermove", updateRailDrag);
    window.removeEventListener("pointerup", stopRailDrag);
    window.removeEventListener("pointercancel", stopRailDrag);
    window.removeEventListener("blur", cancelRailDrag);
  }

  function stopRailDrag(event) {
    if (!railDrag || event.pointerId !== railDrag.pointerId) {
      return;
    }
    finishRailDrag(event.pointerId);
  }

  function cancelRailDrag() {
    if (!railDrag) {
      return;
    }
    finishRailDrag();
  }

  function scrollRailWithKeyboard(event) {
    if (!canShowWorkspaceRail()) {
      return;
    }
    const smallStep = Math.max(48, workspace.clientWidth * 0.12);
    const largeStep = Math.max(120, workspace.clientWidth * 0.72);
    let nextScroll = workspace.scrollLeft;
    const key = (event.key || "").toLowerCase();
    if (key === "arrowleft") {
      nextScroll -= smallStep;
    } else if (key === "arrowright") {
      nextScroll += smallStep;
    } else if (key === "pageup") {
      nextScroll -= largeStep;
    } else if (key === "pagedown") {
      nextScroll += largeStep;
    } else if (key === "home") {
      nextScroll = 0;
    } else if (key === "end") {
      nextScroll = maxWorkspaceScroll();
    } else {
      return;
    }
    event.preventDefault();
    workspace.scrollLeft = clamp(nextScroll, 0, maxWorkspaceScroll());
  }

  applySidebarWidth(sidebarWidth);
  setSidebarOpen(!mobileSidebar.matches || document.body.classList.contains("is-sidebar-open"));
  mobileSidebar.addEventListener("change", function (event) {
    setSidebarOpen(!event.matches);
  });

  workspace.addEventListener("pointerdown", startWorkspaceDrag);
  workspace.addEventListener("pointermove", updateWorkspaceDrag);
  workspace.addEventListener("pointerup", stopWorkspaceDrag);
  workspace.addEventListener("pointercancel", stopWorkspaceDrag);
  workspace.addEventListener("scroll", queueWorkspaceRailUpdate);
  window.addEventListener("keydown", startSpacePan, true);
  window.addEventListener("keyup", stopSpacePan, true);
  window.addEventListener("blur", cancelSpacePan);
  window.addEventListener("resize", function () {
    applySidebarWidth(sidebarWidth);
    queueWorkspaceRailUpdate();
  });

  if (scrollTrack && scrollThumb) {
    scrollTrack.addEventListener("pointerdown", startRailTrackJump);
    scrollThumb.addEventListener("pointerdown", startRailDrag);
    scrollThumb.addEventListener("keydown", scrollRailWithKeyboard);
  }

  workspace.addEventListener("click", async function (event) {
    if (consumeSuppressedWorkspaceClick(event)) {
      return;
    }

    const clickedPanel = closestElement(event.target, "[data-note-path]");
    if (clickedPanel) {
      setActivePanel(clickedPanel);
    }

    const openClaimButton = closestElement(event.target, "[data-open-claim]");
    if (openClaimButton) {
      event.preventDefault();
      const sourcePanel = openClaimButton.closest("[data-note-path]");
      const sourceKnowledgeBase = String(sourcePanel?.dataset.knowledgeBase || currentKnowledgeBase || "");
      try {
        await ensureClaimsData(sourceKnowledgeBase);
        refreshClaimsViewToggles();
      } catch {
        return;
      }
      const claim = claimByID(openClaimButton.dataset.openClaim, sourceKnowledgeBase);
      if (claim) {
        selectedClaimKey = claimIdentity(claim);
        setClaimsViewRequested(true, false);
        syncWorkspaceViewURL(true);
      }
      return;
    }

    const claimDocument = closestElement(event.target, "[data-claim-document-path]");
    if (claimDocument) {
      const targetKnowledgeBase = String(claimDocument.dataset.knowledgeBase || "");
      event.preventDefault();
      claimsViewRequested = false;
      setGraphViewRequested(false, false);
      openTarget(claimDocument.dataset.claimDocumentPath, true, false, "", activePanel(), targetKnowledgeBase);
      return;
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

    const sourceReference = closestElement(event.target, ".ok-source-ref a[href^=\"#\"]");
    if (sourceReference) {
      const panel = sourceReference.closest("[data-note-path]");
      const anchor = decodeURIComponent((sourceReference.getAttribute("href") || "").slice(1));
      const target = panel
        ? Array.prototype.find.call(panel.querySelectorAll("[id]"), function (candidate) {
            return candidate.id === anchor;
          })
        : null;
      if (target) {
        const sourceLedger = target.closest("[data-source-ledger]");
        if (sourceLedger) {
          sourceLedger.open = true;
        }
        event.preventDefault();
        target.scrollIntoView({ block: "nearest" });
        return;
      }
    }

    const treeLink = closestElement(event.target, "[data-tree-path]");
    const graphLink = closestElement(event.target, "[data-graph-path]");
    if (treeLink || graphLink) {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.altKey) {
        return;
      }
      event.preventDefault();
      openTarget(treeLink?.dataset.treePath || graphLink.dataset.graphPath, true, shouldOpenBeside(event.shiftKey), "", activePanel(), treeLink?.dataset.knowledgeBase || graphLink?.dataset.knowledgeBase);
      return;
    }

    const link = closestElement(event.target, "a[href]");
    if (!link || link.dataset.directLink === "true") {
      return;
    }
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.altKey) {
      return;
    }

    const sourcePanel = link.closest("[data-note-path]");
    if (!sourcePanel) {
      return;
    }

    const target = noteTargetFromHref(link.getAttribute("href") || link.href, sourcePanel.dataset.notePath, sourcePanel.dataset.knowledgeBase);
    if (!target) {
      return;
    }

    event.preventDefault();
    openTarget(target.path, true, shouldOpenBeside(event.shiftKey), "", sourcePanel, target.knowledgeBase);
  });

  workspace.addEventListener("keydown", function (event) {
    if (event.key !== "Enter" || !event.shiftKey || event.metaKey || event.ctrlKey || event.altKey) {
      return;
    }
    const link = closestElement(event.target, ".note-body a[href]");
    const sourcePanel = link?.closest("[data-note-path]");
    if (!link || !sourcePanel) {
      return;
    }
    const target = noteTargetFromHref(link.getAttribute("href") || link.href, sourcePanel.dataset.notePath, sourcePanel.dataset.knowledgeBase);
    if (!target) {
      return;
    }
    event.preventDefault();
    openTarget(target.path, true, shouldOpenBeside(true), "", sourcePanel, target.knowledgeBase);
  });

  workspace.addEventListener("focusin", function (event) {
    const focusedPanel = closestElement(event.target, "[data-note-path]");
    if (focusedPanel) {
      setActivePanel(focusedPanel);
    }
  });

  if (window.OpenKnowledgeShortcuts) {
    window.OpenKnowledgeShortcuts.register(panelCloseShortcut);
  }

  if (sidebarToggle) {
    const sidebarShortcut = { id: "viewer.sidebar.toggle", code: "KeyS", metaOrCtrlKey: true, altKey: true, label: "⌘⌥S", ariaKeyShortcut: "Meta+Alt+S", run: toggleSidebar };
    const shortcutSystem = window.OpenKnowledgeShortcuts;
    sidebarToggle.addEventListener("click", toggleSidebar);
    if (shortcutSystem) {
      shortcutSystem.register(sidebarShortcut);
      const label = shortcutSystem.format(sidebarShortcut);
      sidebarToggle.title = "File explorer (" + label + ")";
      sidebarToggle.setAttribute("aria-keyshortcuts", shortcutSystem.ariaKeyShortcut(sidebarShortcut));
      document.querySelectorAll("[data-sidebar-shortcut]").forEach(function (element) {
        element.textContent = label;
      });
    }
  }
  if (sidebarResizeHandle) {
    sidebarResizeHandle.addEventListener("pointerdown", startSidebarResize);
    sidebarResizeHandle.addEventListener("keydown", resizeSidebarWithKeyboard);
  }
  if (fileSidebar) {
    fileSidebar.addEventListener("click", function (event) {
      const treeLink = closestElement(event.target, "[data-tree-path]");
      const link = treeLink || closestElement(event.target, "a[href]");
      if (!link || link.dataset.directLink === "true") {
        return;
      }
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.altKey) {
        return;
      }
      const targetKnowledgeBase = String(treeLink?.dataset.knowledgeBase || "").trim();
      const targetPath = treeLink?.dataset.treePath || notePathFromHref(link.getAttribute("href") || link.href);
      if (!targetPath) {
        return;
      }
      event.preventDefault();
      closeSearchResults(link);
      openTarget(targetPath, true, shouldOpenBeside(event.shiftKey), "", activePanel(), targetKnowledgeBase);
      if (mobileSidebar.matches) {
        setSidebarOpen(false);
      }
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

  window.addEventListener("popstate", async function () {
    const params = new URL(window.location.href).searchParams;
    const requestedView = params.get("view");
    graphViewRequested = requestedView === "graph";
    const wantsClaimsView = requestedView === "claims";
    if (wantsClaimsView) {
      await ensureClaimsData(params.get("knowledge-base") || currentKnowledgeBase).catch(function () {});
      refreshClaimsViewToggles();
    }
    claimsViewRequested = wantsClaimsView && claimsViewCanOpen(claimsData, activeClaimsKnowledgeBase);
    const requestedClaim = claimByID(params.get("claim") || "", params.get("knowledge-base") || currentKnowledgeBase);
    if (requestedClaim) {
      selectedClaimKey = claimIdentity(requestedClaim);
    }
    const paths = stackFromLocation();
    await restoreStack(paths, highlightFromLocation());
    if (claimsViewRequested) {
      setClaimsViewRequested(true, false);
    }
  });

  window.addEventListener("beforeunload", function () {
    stopNarration(null, "");
  });

  document.addEventListener("click", function (event) {
    const searchResult = closestElement(event.target, ".search-result[href]");
    if (searchResult) {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.altKey) {
        return;
      }
      const target = noteTargetFromHref(searchResult.getAttribute("href") || searchResult.href, "", searchResult.dataset.knowledgeBase);
      if (target) {
        event.preventDefault();
        const shiftOverride = event.shiftKey || searchResult.dataset.openBeside === "true";
        const openBeside = shouldOpenBeside(shiftOverride);
        delete searchResult.dataset.openBeside;
        closeSearchResults(searchResult);
        openTarget(target.path, true, openBeside, highlightFromHref(searchResult.getAttribute("href") || searchResult.href), activePanel(), target.knowledgeBase);
        return;
      }
    }

    if (!closestElement(event.target, "[data-editor-picker]")) {
      closeEditorMenus();
    }
    if (!closestElement(event.target, "[data-viewer-settings]")) {
      setSettingsOpen(false);
    }
  });
  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape") {
      closeEditorMenus();
      setSettingsOpen(false);
      setSidebarOpen(false);
    }
  });

  window.OpenKnowledgeViewerLiveReload = Object.freeze({ prepare: prepareLiveReload });

  async function initializeViewer() {
    const requestedStack = stackFromLocation();
    const requestedHighlight = highlightFromLocation();
    organizeSidebarControls();
    bindNavigationMode();
    bindDocumentsView();
    bindClaimsView();
    bindClaimsWorkspace();
    bindGraphView();
    bindViewerSettings();
    prepareKnowledgeBases();
    prepareKnowledgeTrees();
    panels().forEach(bindPanel);
    const initialParams = new URL(window.location.href).searchParams;
    if (initialParams.get("view") === "claims") {
      await ensureClaimsData(initialParams.get("knowledge-base") || currentKnowledgeBase).catch(function () {});
      refreshClaimsViewToggles();
    }
    const initialClaim = claimByID(initialParams.get("claim") || "", initialParams.get("knowledge-base") || currentKnowledgeBase);
    if (initialClaim) {
      selectedClaimKey = claimIdentity(initialClaim);
    }
    claimsViewRequested = initialParams.get("view") === "claims" && claimsViewCanOpen(claimsData, activeClaimsKnowledgeBase);
    graphViewRequested = initialParams.get("view") === "graph";
    ensureActivePanel();
    const initialPanel = panels()[0];
    const requestedInitial = requestedStack[0];
    const initialMatches = requestedStack.length === 1 && initialPanel && requestedInitial
      && requestedInitial.path === initialPanel.dataset.notePath
      && String(requestedInitial.knowledgeBase || "") === String(initialPanel.dataset.knowledgeBase || currentKnowledgeBase || "");
    if (!initialMatches) {
      window.history.replaceState({ stack: requestedStack }, "", window.location.href);
      await restoreStack(requestedStack, requestedHighlight);
    } else {
      window.history.replaceState({ stack: requestedStack }, "", window.location.href);
      updateWorkspaceState();
      updateActiveLinks();
      updateTitle();
      applySearchHighlight(activePanel(), requestedHighlight);
    }
    if (claimsViewRequested) {
      setClaimsViewRequested(true, false);
      syncWorkspaceViewURL(false);
    }
    restoreLiveReloadSession(liveReloadRestoreState);
    window.dispatchEvent(new CustomEvent("openknowledge:viewer-ready"));
  }

  void initializeViewer();
})();
