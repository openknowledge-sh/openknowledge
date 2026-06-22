
(function () {
  const workspace = document.querySelector("[data-note-workspace]");
  const stackEl = document.querySelector("[data-note-stack]");
  const emptyState = document.querySelector("[data-empty-state]");
  const fileSidebar = document.querySelector("[data-file-sidebar]");
  const sidebarToggle = document.querySelector("[data-sidebar-toggle]");
  const sidebarClose = document.querySelector("[data-sidebar-close]");
  const settings = document.querySelector("[data-viewer-settings]");
  const settingsTrigger = document.querySelector("[data-viewer-settings-trigger]");
  const settingsMenu = document.querySelector("[data-viewer-settings-menu]");
  const customThemeFields = document.querySelector("[data-theme-custom-fields]");
  const scrollRail = document.querySelector("[data-workspace-rail]");
  const scrollTrack = document.querySelector("[data-workspace-scroll-track]");
  const scrollThumb = document.querySelector("[data-workspace-scroll-thumb]");

  if (!workspace || !stackEl) {
    return;
  }

  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  const mobileSidebar = window.matchMedia("(max-width: 680px)");
  const editorStorageKey = "openknowledge.viewer.editorOrder";
  const themeStorageKey = "openknowledge.viewer.theme";
  const linkPrefix = normalizeLinkPrefix(workspace.dataset.linkPrefix || "");
  const panelWidthStorageKey = "openknowledge.viewer.panelWidths." + graphHash(workspace.dataset.noteRoot || linkPrefix || window.location.pathname).toString(36);
  const editorOptions = readEditorOptions();
  const panelWidths = readPanelWidths();
  const staticNotes = readStaticNotes();
  const staticNotesByPath = indexStaticNotes(staticNotes, "path");
  const staticNotePathByHTML = indexStaticNotePathsByHTML(staticNotes);
  const knowledgeGraph = readKnowledgeGraph();
  const themePresets = ["default", "night", "paper", "ocean", "rose", "custom"];
  const customThemeDefaults = {
    page: "#f0f0f0",
    surface: "#ffffff",
    text: "#1f2724",
    muted: "#65736d",
    accent: "#0f7a4d",
    border: "#dfe5e1"
  };
  const customThemeVariables = {
    page: ["--ok-color-page", "--ok-color-viewer-canvas", "--ok-color-viewer-header-bg", "--ok-color-sidebar", "--ok-color-sidebar-header", "--ok-color-search-input-bg", "--ok-color-editor-trigger-bg"],
    surface: ["--ok-color-surface", "--ok-color-note-chrome-bg", "--ok-color-search-popover-bg", "--ok-color-editor-menu-bg", "--ok-color-card-bg", "--ok-color-editor-mark-bg"],
    text: ["--ok-color-text", "--ok-color-document-text", "--ok-color-control-hover-text", "--ok-color-editor-mark-text", "--ok-color-code-block-bg"],
    muted: ["--ok-color-muted", "--ok-color-control-text", "--ok-color-sidebar-text", "--ok-color-tree-text", "--ok-color-editor-trigger-text"],
    accent: ["--ok-color-accent", "--ok-color-accent-strong", "--ok-color-graph-node-active-border"],
    border: ["--ok-color-border", "--ok-color-search-input-border", "--ok-color-card-border", "--ok-color-editor-trigger-border", "--ok-color-editor-menu-border"]
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

  function readThemePreference() {
    const stored = readStoredJSON(themeStorageKey);
    if (stored && typeof stored === "object" && !Array.isArray(stored)) {
      return normalizeThemePreference(stored);
    }
    return normalizeThemePreference({ preset: "default", custom: customThemeDefaults });
  }

  function normalizeThemePreference(value) {
    const preset = themePresets.includes(value.preset) ? value.preset : "default";
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
    const rgb = hexToRGB(background);
    const luminance = (0.2126 * rgb[0] + 0.7152 * rgb[1] + 0.0722 * rgb[2]) / 255;
    return luminance > 0.48 ? "#121715" : "#f3f7f4";
  }

  function applyThemePreference(preference) {
    const normalized = normalizeThemePreference(preference);
    document.documentElement.dataset.viewerTheme = normalized.preset;
    document.documentElement.style.colorScheme = normalized.preset === "night" ? "dark" : "light";
    clearCustomThemeVariables();
    if (normalized.preset === "custom") {
      applyCustomThemeVariables(normalized.custom);
    }
    syncThemeControls(normalized);
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
    document.documentElement.style.setProperty("--ok-color-accent-soft", "rgba(" + accentRGB + ", .13)");
    document.documentElement.style.setProperty("--ok-color-accent-softer", "rgba(" + accentRGB + ", .09)");
    document.documentElement.style.setProperty("--ok-color-accent-selected", "rgba(" + accentRGB + ", .1)");
    document.documentElement.style.setProperty("--ok-color-accent-focus", "rgba(" + accentRGB + ", .12)");
    document.documentElement.style.setProperty("--ok-color-accent-focus-strong", "rgba(" + accentRGB + ", .18)");
    document.documentElement.style.setProperty("--ok-color-accent-border", "rgba(" + accentRGB + ", .35)");
    document.documentElement.style.setProperty("--ok-color-accent-border-strong", "rgba(" + accentRGB + ", .5)");
    document.documentElement.style.setProperty("--ok-color-shadow", "rgba(" + hexToRGB(custom.text).join(", ") + ", .14)");
    document.documentElement.style.setProperty("--ok-color-code-inline-bg", colorMix(custom.surface, custom.accent, 0.1));
    document.documentElement.style.setProperty("--ok-color-code-block-text", readableCodeBlockText(custom.text));
    document.documentElement.style.setProperty("--ok-color-control-hover-bg", colorMix(custom.page, custom.text, 0.08));
    document.documentElement.style.setProperty("--ok-color-sidebar-row", colorMix(custom.page, custom.text, 0.11));
    document.documentElement.style.setProperty("--ok-color-sidebar-tree-hover-bg", colorMix(custom.page, custom.accent, 0.13));
    document.documentElement.style.setProperty("--ok-color-search-result-hover-bg", colorMix(custom.surface, custom.accent, 0.08));
    document.documentElement.style.setProperty("--ok-color-editor-menu-item-hover-bg", colorMix(custom.surface, custom.accent, 0.08));
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
    }
    if (sidebarToggle) {
      sidebarToggle.setAttribute("aria-expanded", open ? "true" : "false");
    }
  }

  function toggleSidebar() {
    setSidebarOpen(!document.body.classList.contains("is-sidebar-open"));
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
    if (!isMarkdownPath(raw)) {
      return null;
    }
    try {
      return decodeURIComponent(raw);
    } catch {
      return raw;
    }
  }

  function encodedNoteURL(prefix, path) {
    return prefix + path.split("/").map(encodeURIComponent).join("/");
  }

  function isMarkdownPath(path) {
    return /\.(md|markdown)$/i.test(String(path || "").split("?")[0].split("#")[0]);
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
    const labelsByPath = graphUniqueNodeLabels(knowledgeGraph.nodes);
    const positions = graphLayoutPositions(knowledgeGraph, width, height, labelsByPath);
    const canvas = document.createElement("canvas");
    canvas.className = "knowledge-graph-canvas";
    canvas.dataset.knowledgeGraphCanvas = "true";
    canvas.width = width;
    canvas.height = height;
    canvas.tabIndex = 0;
    canvas.setAttribute("role", "img");
    canvas.setAttribute("aria-label", "Animated graph of Markdown files. Hover a node to separate nearby notes and highlight direct connections.");
    graphView.append(canvas);
    createKnowledgeGraphCanvas(canvas, knowledgeGraph, positions, labelsByPath, width, height).start();
  }

  function createKnowledgeGraphCanvas(canvas, graph, positions, labelsByPath, width, height) {
    const context = canvas.getContext("2d");
    const nodeSet = Object.create(null);
    graph.nodes.forEach(function (node) {
      if (node && typeof node.path === "string") {
        nodeSet[node.path] = true;
      }
    });
    const links = graph.edges.filter(function (edge) {
      return edge && nodeSet[edge.source] && nodeSet[edge.target] && positions[edge.source] && positions[edge.target];
    });
    const states = graph.nodes.filter(function (node) {
      return node && typeof node.path === "string" && positions[node.path];
    }).map(function (node) {
      const point = positions[node.path];
      const label = graphNodeLabel(node, labelsByPath);
      return {
        node: node,
        path: node.path,
        label: label,
        fullLabel: graphNodeFullLabel(node, labelsByPath),
        radius: node.path === "index.md" ? 16 : 10,
        labelOffset: node.path === "index.md" ? 31 : 25,
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

    let activePath = "";
    let keyboardIndex = states.findIndex(function (state) { return state.path === "index.md"; });
    if (keyboardIndex < 0) {
      keyboardIndex = 0;
    }
    let lastPointer = null;
    let frame = 0;

    const setActivePath = function (path) {
      const nextPath = path && stateByPath[path] ? path : "";
      if (nextPath === activePath) {
        return;
      }
      activePath = nextPath;
      canvas.dataset.activeGraphPath = activePath;
      canvas.style.cursor = activePath ? "pointer" : "default";
      if (activePath) {
        const activeIndex = states.findIndex(function (state) { return state.path === activePath; });
        if (activeIndex >= 0) {
          keyboardIndex = activeIndex;
        }
      }
    };

    const resizeCanvas = function () {
      const pixelRatio = Math.max(1, Math.min(window.devicePixelRatio || 1, 2));
      canvas.width = Math.round(width * pixelRatio);
      canvas.height = Math.round(height * pixelRatio);
      context.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);
    };

    const canvasPoint = function (event) {
      const rect = canvas.getBoundingClientRect();
      if (!rect.width || !rect.height) {
        return { x: 0, y: 0 };
      }
      return {
        x: (event.clientX - rect.left) * (width / rect.width),
        y: (event.clientY - rect.top) * (height / rect.height),
      };
    };

    const updatePointerTarget = function (point) {
      lastPointer = point;
      const hit = graphCanvasHitTest(states, point);
      setActivePath(hit ? hit.path : "");
    };

    canvas.addEventListener("pointermove", function (event) {
      updatePointerTarget(canvasPoint(event));
    });
    canvas.addEventListener("pointerleave", function () {
      lastPointer = null;
      if (document.activeElement !== canvas) {
        setActivePath("");
      }
    });
    canvas.addEventListener("click", function (event) {
      const hit = graphCanvasHitTest(states, canvasPoint(event));
      if (hit) {
        window.location.href = fileURL(hit.path);
      }
    });
    canvas.addEventListener("focus", function () {
      if (states[keyboardIndex]) {
        setActivePath(states[keyboardIndex].path);
      }
    });
    canvas.addEventListener("blur", function () {
      setActivePath(lastPointer ? activePath : "");
    });
    canvas.addEventListener("keydown", function (event) {
      if (!states.length) {
        return;
      }
      if (event.key === "Enter" || event.key === " ") {
        if (activePath) {
          event.preventDefault();
          window.location.href = fileURL(activePath);
        }
        return;
      }
      if (event.key !== "ArrowRight" && event.key !== "ArrowDown" && event.key !== "ArrowLeft" && event.key !== "ArrowUp") {
        return;
      }
      event.preventDefault();
      const direction = event.key === "ArrowRight" || event.key === "ArrowDown" ? 1 : -1;
      keyboardIndex = (keyboardIndex + direction + states.length) % states.length;
      setActivePath(states[keyboardIndex].path);
    });

    const tick = function () {
      if (!canvas.isConnected) {
        return;
      }
      frame = window.requestAnimationFrame(tick);
      graphCanvasPhysicsStep(states, links, stateByPath, activePath, width, height);
      drawKnowledgeGraphCanvas(context, states, links, stateByPath, activePath, width, height);
    };

    return {
      start: function () {
        if (!context) {
          return;
        }
        resizeCanvas();
        drawKnowledgeGraphCanvas(context, states, links, stateByPath, activePath, width, height);
        frame = window.requestAnimationFrame(tick);
      },
      stop: function () {
        if (frame) {
          window.cancelAnimationFrame(frame);
        }
      },
    };
  }

  function graphCanvasPhysicsStep(states, links, stateByPath, activePath, width, height) {
    const active = activePath ? stateByPath[activePath] : null;
    states.forEach(function (state) {
      const targetZ = state === active ? 1 : 0;
      const basePull = state === active ? 0.052 : 0.034;
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
      const desired = 104 + (connected ? 28 * hoverStrength : 0);
      const force = (distance - desired) * (connected ? 0.0015 : 0.0011);
      const nx = dx / distance;
      const ny = dy / distance;
      source.vx += nx * force;
      source.vy += ny * force;
      target.vx -= nx * force;
      target.vy -= ny * force;
    });

    for (let i = 0; i < states.length; i += 1) {
      for (let j = i + 1; j < states.length; j += 1) {
        const a = states[i];
        const b = states[j];
        const dx = b.x - a.x || 0.01;
        const dy = b.y - a.y || 0.01;
        const distance = Math.max(1, Math.sqrt(dx * dx + dy * dy));
        const nx = dx / distance;
        const ny = dy / distance;
        const activePair = active && (a === active || b === active);
        const desired = activePair ? 48 + (64 + graphLabelWidth(active.fullLabel) * 0.2) * hoverStrength : 46;
        if (distance < desired) {
          const push = Math.min(activePair ? 3.2 : 1.6, (desired - distance) * (activePair ? 0.014 + hoverStrength * 0.012 : 0.009));
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

    if (active && hoverStrength > 0.02) {
      const activeBox = graphCanvasNodeBox(active, active.fullLabel);
      states.forEach(function (state) {
        if (state === active) {
          return;
        }
        const overlap = graphBoxOverlap(activeBox, graphCanvasNodeBox(state, state.label));
        if (!overlap) {
          return;
        }
        const dx = state.x - active.x || 0.01;
        const dy = state.y - active.y || 0.01;
        const distance = Math.max(1, Math.sqrt(dx * dx + dy * dy));
        const push = Math.min(3.2, (Math.max(overlap.x, overlap.y) * 0.026 + 0.8) * hoverStrength);
        state.vx += (dx / distance) * push;
        state.vy += (dy / distance) * push;
      });
    }

    states.forEach(function (state) {
      state.x += state.vx;
      state.y += state.vy;
      graphClampState(state, width, height);
      graphLimitVelocity(state, active ? 5.5 : 4.2);
      const damping = active ? 0.58 : 0.66;
      state.vx *= damping;
      state.vy *= damping;
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

  function drawKnowledgeGraphCanvas(context, states, links, stateByPath, activePath, width, height) {
    const active = activePath ? stateByPath[activePath] : null;
    const theme = graphCanvasTheme();
    context.clearRect(0, 0, width, height);
    context.lineCap = "round";
    context.lineJoin = "round";

    links.forEach(function (edge) {
      const source = stateByPath[edge.source];
      const target = stateByPath[edge.target];
      if (!source || !target) {
        return;
      }
      const connected = active && (edge.source === active.path || edge.target === active.path);
      context.beginPath();
      context.moveTo(source.x, source.y);
      context.lineTo(target.x, target.y);
      context.strokeStyle = connected ? theme.edgeActive : active ? theme.edgeMuted : theme.edge;
      context.lineWidth = connected ? 2.35 : 1.05;
      context.stroke();
    });

    states.slice().sort(function (a, b) {
      return a.z - b.z;
    }).forEach(function (state) {
      const activeNode = state === active;
      const scale = 1 + state.z * 0.22;
      const radius = state.radius * scale;
      const label = activeNode ? state.fullLabel : state.label;
      context.save();
      context.globalAlpha = 1;
      context.beginPath();
      context.arc(state.x, state.y - state.z * 6, radius, 0, Math.PI * 2);
      context.fillStyle = theme.nodeBg;
      context.fill();
      context.strokeStyle = activeNode ? theme.nodeActiveBorder : theme.nodeBorder;
      context.lineWidth = activeNode ? 3 : state.path === "index.md" ? 2 : 1.55;
      context.stroke();

      context.font = (activeNode ? "600 13px" : "400 12px") + " " + theme.fontBody;
      context.textBaseline = "middle";
      context.textAlign = graphCanvasTextAlign(state.x, graphLabelWidth(label), width);
      const labelX = context.textAlign === "start" ? Math.max(16, state.x - graphLabelWidth(label) / 2) : context.textAlign === "end" ? Math.min(width - 16, state.x + graphLabelWidth(label) / 2) : state.x;
      const labelY = state.y + state.labelOffset + state.z * 4;
      context.fillStyle = activeNode ? theme.labelActive : theme.label;
      context.fillText(label, labelX, labelY);
      context.restore();
    });
  }

  function graphCanvasTheme() {
    return {
      fontBody: themeValue("--ok-font-body", "Inter, ui-sans-serif, system-ui, sans-serif"),
      edge: themeValue("--ok-color-graph-edge", "rgba(128, 138, 133, .25)"),
      edgeMuted: themeValue("--ok-color-graph-edge-muted", "rgba(128, 138, 133, .11)"),
      edgeActive: themeValue("--ok-color-graph-edge-active", "rgba(15, 122, 77, .78)"),
      nodeBg: themeValue("--ok-color-graph-node-bg", "#f8f8f8"),
      nodeBorder: themeValue("--ok-color-graph-node-border", "#aeb8b2"),
      nodeActiveBorder: themeValue("--ok-color-graph-node-active-border", "#0f7a4d"),
      label: themeValue("--ok-color-graph-label", "#5f6b66"),
      labelActive: themeValue("--ok-color-graph-label-active", "#26302c"),
    };
  }

  function themeValue(name, fallback) {
    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return value || fallback;
  }

  function graphCanvasHitTest(states, point) {
    for (let index = states.length - 1; index >= 0; index -= 1) {
      const state = states[index];
      const dx = point.x - state.x;
      const dy = point.y - state.y;
      const radius = state.radius * (1 + state.z * 0.22) + 6;
      if (dx * dx + dy * dy <= radius * radius) {
        return state;
      }
      if (graphPointInBox(point, graphCanvasNodeBox(state, state.z > 0.6 ? state.fullLabel : state.label))) {
        return state;
      }
    }
    return null;
  }

  function graphCanvasNodeBox(state, label) {
    const labelWidth = graphLabelWidth(label);
    const labelTop = state.y + state.labelOffset - 10;
    const halfWidth = Math.max(state.radius + 8, labelWidth / 2 + 9);
    return {
      left: state.x - halfWidth,
      right: state.x + halfWidth,
      top: Math.min(state.y - state.radius - 8, labelTop),
      bottom: Math.max(state.y + state.radius + 8, labelTop + 22),
    };
  }

  function graphPointInBox(point, box) {
    return point.x >= box.left && point.x <= box.right && point.y >= box.top && point.y <= box.bottom;
  }

  function graphCanvasTextAlign(x, labelWidth, width) {
    if (x - labelWidth / 2 < 16) {
      return "start";
    }
    if (x + labelWidth / 2 > width - 16) {
      return "end";
    }
    return "center";
  }

  function graphStatesConnected(links, activePath, path) {
    if (path === activePath) {
      return true;
    }
    return links.some(function (edge) {
      return (edge.source === activePath && edge.target === path) || (edge.target === activePath && edge.source === path);
    });
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
      const group = graphPathGroup(node.path);
      const groupCenter = groupCenters[group] || center;
      const hash = graphHash(node.path);
      const angle = ((hash % 360) / 360) * Math.PI * 2;
      const spread = 26 + (hash % 74);
      return {
        node: node,
        group: group,
        label: graphNodeLabel(node, labelsByPath),
        radius: node.path === "index.md" ? 16 : 10,
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
        const centerPull = state.node.path === "index.md" ? 0.04 : 0.002 + Math.min(nodeDegree, 8) * 0.0008;
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

  function graphGroupCenters(nodes, width, height) {
    const counts = Object.create(null);
    nodes.forEach(function (node) {
      const group = graphPathGroup(node.path);
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
    const scale = Math.min((width - paddingX * 2) / spanX, (height - paddingY * 2) / spanY, 1.28);
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
    const labelTop = state.y + (state.node.path === "index.md" ? 19 : 15);
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
      const peerPaths = peers.map(function (node) { return node.path; });
      peers.forEach(function (node) {
        labels[node.path] = graphShortestUniquePathSuffix(node.path, peerPaths);
      });
    });
    return labels;
  }

  function graphNodeBaseLabel(node) {
    const title = String(node.title || "").trim();
    if (title && title.toLowerCase() !== "index") {
      return title;
    }
    return graphPathDisplayName(node.path);
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

  function bindViewerSettings() {
    if (!settings || !settingsTrigger || !settingsMenu || settings.dataset.settingsBound === "true") {
      return;
    }
    settings.dataset.settingsBound = "true";
    let preference = readThemePreference();
    applyThemePreference(preference);

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

  function stackFromLocation() {
    const params = new URLSearchParams(window.location.search);
    if (params.get("empty") === "1") {
      return [];
    }
    const base = notePathFromHref(window.location.href) || currentStack()[0] || "index.md";
    return [base].concat(params.getAll("stack").filter(Boolean));
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
    if (!paths.length) {
      const emptyURL = new URL(fileURL("index.md"), window.location.href);
      emptyURL.searchParams.set("empty", "1");
      return emptyURL;
    }

    const url = new URL(fileURL(paths[0] || "index.md"), window.location.href);
    paths.slice(1).forEach(function (path) {
      url.searchParams.append("stack", path);
    });
    if (highlightText) {
      url.searchParams.set("ok-highlight", highlightText);
    }
    return url;
  }

  function updateWorkspaceState() {
    const panelCount = panels().length;
    const isEmpty = panelCount === 0;
    workspace.classList.toggle("is-empty", isEmpty);
    workspace.classList.toggle("is-single-panel", panelCount === 1);
    workspace.classList.toggle("is-multi-panel", panelCount > 1);
    if (emptyState) {
      emptyState.hidden = !isEmpty;
    }
    panels().forEach(applyPanelWidth);
    ensureActivePanel();
    updateCloseLinks();
    updateSpacePanState();
    queueWorkspaceRailUpdate();
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

  function maxWorkspaceScroll() {
    return Math.max(0, workspace.scrollWidth - workspace.clientWidth);
  }

  function canShowWorkspaceRail() {
    return Boolean(scrollRail && scrollTrack && scrollThumb && panels().length > 1 && maxWorkspaceScroll() > 1 && !workspace.classList.contains("is-empty"));
  }

  function queueWorkspaceRailUpdate() {
    window.requestAnimationFrame(updateWorkspaceRail);
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

  function scrollWorkspaceFromRail(clientX, thumbOffset) {
    if (!canShowWorkspaceRail()) {
      return;
    }
    const trackRect = scrollTrack.getBoundingClientRect();
    const thumbRect = scrollThumb.getBoundingClientRect();
    const maxThumbX = Math.max(0, trackRect.width - thumbRect.width);
    const maxScroll = maxWorkspaceScroll();
    const thumbX = clamp(clientX - trackRect.left - thumbOffset, 0, maxThumbX);
    workspace.scrollLeft = maxThumbX > 0 ? (thumbX / maxThumbX) * maxScroll : 0;
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

  function updateHistory(paths, pushHistory, highlightText) {
    const nextURL = stackURL(paths, highlightText);
    const state = { stack: paths };
    if (pushHistory) {
      window.history.pushState(state, "", nextURL);
    } else {
      window.history.replaceState(state, "", nextURL);
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
        behavior: reduceMotion.matches ? "auto" : "smooth"
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
    const sourceButton = createSourceButton(data.path, data.sourceURL);
    if (sourceButton) {
      actions.append(sourceButton);
    } else if (!isStaticBundle()) {
      actions.append(createEditorPicker());
    }

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
    applyPanelWidth(panel);
    ensurePanelResizeHandles(panel);
    panel.querySelectorAll("[data-editor-picker]").forEach(bindEditorPicker);
    enhanceTables(panel);

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

  async function openInitialNote(targetPath, pushHistory, highlightText) {
    const panel = await panelForPath(targetPath, true);
    await runStackTransition(function () {
      clearStack();
      appendPanel(panel);
      updateHistory(currentStack(), pushHistory, highlightText);
    });
    applySearchHighlight(panel, highlightText);
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
    clearSearchHighlights(stackEl);
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

  async function restoreStack(paths, highlightText) {
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
    applySearchHighlight(activePanel(), highlightText);
  }

  const finePointer = window.matchMedia("(hover: hover) and (pointer: fine)");
  let workspaceDrag = null;
  let railDrag = null;
  let panelResize = null;
  let spacePanPressed = false;
  let suppressWorkspaceClickUntil = 0;

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
    const thumbRect = scrollThumb.getBoundingClientRect();
    railDrag = {
      pointerId: event.pointerId,
      thumbOffset: clamp(event.clientX - thumbRect.left, 0, thumbRect.width)
    };
    scrollRail.classList.add("is-rail-dragging");
    window.addEventListener("pointermove", updateRailDrag);
    window.addEventListener("pointerup", stopRailDrag);
    window.addEventListener("pointercancel", stopRailDrag);
    window.addEventListener("blur", cancelRailDrag);
    scrollWorkspaceFromRail(event.clientX, railDrag.thumbOffset);
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
    scrollWorkspaceFromRail(event.clientX, railDrag.thumbOffset);
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

  workspace.addEventListener("pointerdown", startWorkspaceDrag);
  workspace.addEventListener("pointermove", updateWorkspaceDrag);
  workspace.addEventListener("pointerup", stopWorkspaceDrag);
  workspace.addEventListener("pointercancel", stopWorkspaceDrag);
  workspace.addEventListener("scroll", queueWorkspaceRailUpdate);
  window.addEventListener("keydown", startSpacePan, true);
  window.addEventListener("keyup", stopSpacePan, true);
  window.addEventListener("blur", cancelSpacePan);
  window.addEventListener("resize", queueWorkspaceRailUpdate);

  if (scrollTrack && scrollThumb) {
    scrollTrack.addEventListener("pointerdown", startRailTrackJump);
    scrollThumb.addEventListener("pointerdown", startRailDrag);
    scrollThumb.addEventListener("pointermove", updateRailDrag);
    scrollThumb.addEventListener("pointerup", stopRailDrag);
    scrollThumb.addEventListener("pointercancel", stopRailDrag);
    scrollThumb.addEventListener("keydown", scrollRailWithKeyboard);
  }

  workspace.addEventListener("click", function (event) {
    if (consumeSuppressedWorkspaceClick(event)) {
      return;
    }

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
    const sidebarShortcut = { id: "viewer.sidebar.toggle", code: "KeyS", primaryKey: true, altKey: true, run: toggleSidebar };
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

  window.addEventListener("popstate", function () {
    const paths = stackFromLocation();
    restoreStack(paths, highlightFromLocation());
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
        openInitialNote(targetPath, true, highlightFromHref(searchResult.getAttribute("href") || searchResult.href));
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

  const requestedStack = stackFromLocation();
  const requestedHighlight = highlightFromLocation();
  bindViewerSettings();
  renderKnowledgeGraph();
  panels().forEach(bindPanel);
  ensureActivePanel();
  if (requestedStack.length !== 1 || requestedStack[0] !== panels()[0]?.dataset.notePath) {
    window.history.replaceState({ stack: requestedStack }, "", window.location.href);
    restoreStack(requestedStack, requestedHighlight);
  } else {
    window.history.replaceState({ stack: requestedStack }, "", window.location.href);
    updateWorkspaceState();
    updateActiveLinks();
    updateTitle();
    applySearchHighlight(activePanel(), requestedHighlight);
  }
})();
