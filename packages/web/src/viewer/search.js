
(() => {
  const searches = Array.from(document.querySelectorAll(".search"));
  if (searches.length === 0) return;
  const staticNotes = readStaticNotes();
  const primarySearch = document.querySelector("[data-primary-search]") || searches[0];
  const primaryInput = primarySearch?.querySelector(".search-input");
  const shortcutSystem = window.OpenKnowledgeShortcuts;
  const searchShortcut = {
    id: "viewer.search.focus",
    key: "k",
    metaOrCtrlKey: true,
    label: "⌘K",
    allowEditable: true,
    run: () => {
      primaryInput?.focus();
      primaryInput?.select();
    }
  };

  searches.forEach(bindSearch);

  if (shortcutSystem) {
    shortcutSystem.register(searchShortcut);
    document.querySelectorAll("[data-search-shortcut]").forEach((element) => {
      element.textContent = shortcutSystem.format(searchShortcut);
    });
  } else {
    document.addEventListener("keydown", (event) => {
      if ((event.metaKey || event.ctrlKey) && !event.altKey && event.key.toLowerCase() === "k") {
        event.preventDefault();
        primaryInput?.focus();
        primaryInput?.select();
      }
    });
  }

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
    let activeTag = "";

    initializeSearchAccessibility(input, results);
    closeSearch(false);

    if (search === primarySearch) {
      activeTag = tagSearchFromLocation();
      if (activeTag) {
        input.value = activeTag;
        window.requestAnimationFrame(() => {
          input.focus();
          input.select();
          runTagSearch(activeTag);
        });
      }
    }

    input.addEventListener("input", () => {
      activeTag = "";
      clearTagSearchFromLocation();
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
        link.dataset.openBeside = event.shiftKey ? "true" : "false";
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
    document.addEventListener("pointerdown", (event) => {
      if (!results.hidden && !search.contains(event.target)) {
        closeSearch(true);
      }
    });
    document.addEventListener("focusin", (event) => {
      if (!results.hidden && !search.contains(event.target)) {
        closeSearch(true);
      }
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
      renderSearchState(results, status, "Searching…", "loading", setResultsOpen, setActiveResult, true);

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
        renderSearchState(results, status, "Search unavailable.", "error", setResultsOpen, setActiveResult, true, {
          label: "Retry",
          run: runSearch,
        });
      }
    }

    async function runTagSearch(tag) {
      const normalizedTag = String(tag || "").trim();
      if (!normalizedTag) {
        return;
      }
      const requestID = ++sequence;
      const currentPath = document.querySelector("[data-note-path]")?.dataset.notePath || "";
      setActiveResult(-1, false);

      if (staticNotes.length > 0) {
        const items = searchStaticTag(normalizedTag, currentPath);
        renderResults(results, status, items, normalizedTag, setResultsOpen, setActiveResult, {
          emptyStatus: "No other notes tagged \"" + normalizedTag + "\".",
          keepOpenWhenEmpty: true,
          showMatchCounts: false,
          statusText: items.length + " note" + (items.length === 1 ? "" : "s") + " tagged \"" + normalizedTag + "\"",
        });
        return;
      }

      if (controller) controller.abort();
      controller = new AbortController();
      renderSearchState(results, status, "Finding tagged notes…", "loading", setResultsOpen, setActiveResult, true);
      const params = new URLSearchParams({ tag: normalizedTag, limit: "30" });
      if (currentPath) {
        params.set("exclude", currentPath);
      }
      const currentKnowledgeBase = search.dataset.knowledgeBase || document.body.dataset.activeKnowledgeBase || "";
      if (currentKnowledgeBase) {
        params.set("excludeKnowledgeBase", currentKnowledgeBase);
      }
      try {
        const response = await fetch(searchURL + "?" + params.toString(), { signal: controller.signal });
        if (!response.ok) throw new Error("tag search request failed");
        const payload = await response.json();
        if (requestID !== sequence || activeTag !== normalizedTag) {
          return;
        }
        const items = payload.results || [];
        renderResults(results, status, items, normalizedTag, setResultsOpen, setActiveResult, {
          emptyStatus: "No other notes tagged \"" + normalizedTag + "\".",
          keepOpenWhenEmpty: true,
          showMatchCounts: false,
          statusText: items.length + " note" + (items.length === 1 ? "" : "s") + " tagged \"" + normalizedTag + "\"",
        });
      } catch (error) {
        if (error.name === "AbortError") return;
        renderSearchState(results, status, "Tag lookup unavailable.", "error", setResultsOpen, setActiveResult, true, {
          label: "Retry",
          run: () => runTagSearch(normalizedTag),
        });
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
        activeTag = "";
        clearTagSearchFromLocation();
      }
      status.textContent = "";
      results.replaceChildren();
      resetSearchResults(results);
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

  function tagSearchFromLocation() {
    try {
      return (new URL(window.location.href).searchParams.get("ok-tag") || "").trim();
    } catch {
      return "";
    }
  }

  function clearTagSearchFromLocation() {
    let url;
    try {
      url = new URL(window.location.href);
    } catch {
      return;
    }
    if (!url.searchParams.has("ok-tag")) {
      return;
    }
    url.searchParams.delete("ok-tag");
    window.history.replaceState(window.history.state, "", url);
  }

  function renderResults(results, status, items, query, setResultsOpen, setActiveResult, options) {
    const config = options || {};
    const groupedItems = groupSearchResults(items);
    results.replaceChildren();
    resetSearchResults(results);
    results.dataset.mode = query ? "matches" : "initial";
    if (groupedItems.length === 0) {
      const emptyText = config.emptyStatus ?? "No results for \"" + query + "\".";
      if (emptyText) {
        const keepOpenWhenEmpty = config.keepOpenWhenEmpty !== false;
        renderSearchState(results, status, emptyText, "empty", setResultsOpen, setActiveResult, keepOpenWhenEmpty);
      } else {
        status.textContent = "";
        setActiveResult(-1, false);
        setResultsOpen(false);
      }
      return;
    }

    const summary = config.statusText || (groupedItems.length + " document" + (groupedItems.length === 1 ? "" : "s"));
    status.textContent = summary;
    results.dataset.summary = summary;
    results.dataset.hasOptions = "true";
    setResultsOpen(true);
    groupedItems.forEach((item, index) => {
      const link = document.createElement("a");
      link.className = "search-result";
      link.href = item.highlightURL || item.url || staticRelativeURL(item.path);
      link.id = results.id + "-option-" + index;
      link.setAttribute("role", "option");
      link.setAttribute("aria-selected", "false");
      link.title = navigationModeTitle();

      const titleRow = document.createElement("span");
      titleRow.className = "search-result-title-row";
      if (item.knowledgeBase) {
        link.dataset.knowledgeBase = item.knowledgeBase;
        const origin = document.createElement("span");
        origin.className = "search-result-knowledge-base";
        const marker = document.createElement("span");
        marker.className = "search-result-knowledge-base-marker";
        marker.setAttribute("aria-hidden", "true");
        const knowledgeBaseAPI = window.OpenKnowledgeKnowledgeBases;
        if (knowledgeBaseAPI?.color) {
          marker.style.backgroundColor = knowledgeBaseAPI.color(item.knowledgeBase);
          origin.style.color = knowledgeBaseAPI.color(item.knowledgeBase);
        }
        const label = document.createElement("span");
        label.textContent = item.knowledgeBase;
        origin.append(marker, label);
        titleRow.append(origin);
      }
      const title = document.createElement("span");
      title.className = "search-result-title";
      const displayTitle = searchResultTitle(item);
      appendHighlightedText(title, displayTitle, query);
      titleRow.append(title);

      const showPath = item.path && String(item.path).toLocaleLowerCase() !== displayTitle.toLocaleLowerCase();
      if (showPath) {
        const meta = document.createElement("span");
        meta.className = "search-result-meta";
        meta.textContent = item.path;
        titleRow.append(meta);
      }

      if (config.showMatchCounts !== false && query && item.matchCount > 0) {
        const count = document.createElement("span");
        count.className = "search-result-count";
        count.textContent = item.matchCount + " match" + (item.matchCount === 1 ? "" : "es");
        titleRow.append(count);
      }
      link.append(titleRow);

      if (item.snippet) {
        const snippet = document.createElement("span");
        snippet.className = "search-result-snippet";
        appendHighlightedText(snippet, plainSearchExcerpt(item.snippet), query);
        link.append(snippet);
      }

      const accessibleCount = config.showMatchCounts !== false && query && item.matchCount
        ? item.matchCount + " match" + (item.matchCount === 1 ? "" : "es")
        : "";
      link.setAttribute("aria-label", [displayTitle, item.path, item.knowledgeBase, accessibleCount].filter(Boolean).join(", "));

      results.append(link);
    });
    setActiveResult(0, false);
  }

  function renderSearchState(results, status, message, kind, setResultsOpen, setActiveResult, open, action) {
    status.textContent = message;
    results.replaceChildren();
    resetSearchResults(results);
    results.classList.add("is-" + kind);
    results.setAttribute("role", "group");
    results.setAttribute("aria-label", "Search status");

    const state = document.createElement("div");
    state.className = "search-state";
    const text = document.createElement("span");
    text.className = "search-state-message";
    text.textContent = message;
    state.append(text);

    if (action) {
      const button = document.createElement("button");
      button.className = "search-state-action";
      button.type = "button";
      button.textContent = action.label;
      button.addEventListener("click", action.run);
      state.append(button);
    }

    results.append(state);
    setActiveResult(-1, false);
    setResultsOpen(open);
  }

  function resetSearchResults(results) {
    results.classList.remove("is-loading", "is-error", "is-empty");
    delete results.dataset.summary;
    delete results.dataset.hasOptions;
    delete results.dataset.mode;
    results.setAttribute("role", "listbox");
    results.setAttribute("aria-label", "Search results");
  }

  function groupSearchResults(items) {
    const groups = [];
    const byPath = new Map();
    items.forEach((item, index) => {
      const path = item.path || "__result-" + index;
      const key = String(item.knowledgeBase || "") + "\u0000" + path;
      let group = byPath.get(key);
      if (!group) {
        group = Object.assign({}, item, {
          matchCount: 0,
        });
        byPath.set(key, group);
        groups.push(group);
      }
      group.matchCount += 1;
      if (!group.snippet && item.snippet) {
        group.snippet = item.snippet;
      }
    });
    return groups;
  }

  function searchResultTitle(item) {
    const title = String(item.title || item.path || "").trim();
    if (!/^index(?:\.md)?$/i.test(title)) {
      return title;
    }
    const segments = String(item.path || "").split("/").filter(Boolean);
    if (segments[segments.length - 1]?.toLowerCase() === "index.md") {
      segments.pop();
    }
    if (segments.length === 0) {
      return "Home";
    }
    const parent = segments[segments.length - 1]
      .replace(/[-_]+/g, " ")
      .replace(/\b\w/g, (character) => character.toUpperCase());
    return parent + " / Index";
  }

  function plainSearchExcerpt(value) {
    return String(value || "")
      .replace(/!\[([^\]]*)\]\([^)]*\)/g, "$1")
      .replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
      .replace(/(^|\s)#{1,6}\s+/g, "$1")
      .replace(/(^|\s)>\s?/g, "$1")
      .replace(/`{1,3}([^`]+)`{1,3}/g, "$1")
      .replace(/\*\*([^*]+)\*\*/g, "$1")
      .replace(/__([^_]+)__/g, "$1")
      .replace(/[~*_]/g, "")
      .replace(/\s+/g, " ")
      .trim();
  }

  function navigationModeTitle() {
    return document.documentElement.dataset.viewerNavigationMode === "beside"
      ? "Links open beside. Hold Shift to replace the current panel."
      : "Links open in the current panel. Hold Shift to open beside.";
  }

  function appendHighlightedText(container, value, query) {
    const text = String(value || "");
    const needle = String(query || "").trim();
    if (!needle) {
      container.textContent = text;
      return;
    }
    const lowerText = text.toLocaleLowerCase();
    const lowerNeedle = needle.toLocaleLowerCase();
    let offset = 0;
    let match = lowerText.indexOf(lowerNeedle);
    while (match >= 0) {
      if (match > offset) {
        container.append(document.createTextNode(text.slice(offset, match)));
      }
      const mark = document.createElement("mark");
      mark.className = "search-result-highlight";
      mark.textContent = text.slice(match, match + needle.length);
      container.append(mark);
      offset = match + needle.length;
      match = lowerText.indexOf(lowerNeedle, offset);
    }
    if (offset < text.length) {
      container.append(document.createTextNode(text.slice(offset)));
    }
  }

  function defaultSearchResults() {
    const seen = new Set();
    const items = [];
    const links = Array.from(document.querySelectorAll("[data-tree-path]"));
    for (const link of links) {
      const path = link.dataset.treePath || "";
      const knowledgeBase = link.dataset.knowledgeBase || "";
      const key = knowledgeBase + "\u0000" + path;
      if (!path || seen.has(key)) {
        continue;
      }
      seen.add(key);
      const title = link.querySelector(".tree-file-name")?.textContent?.trim() || path;
      items.push({
        path,
        knowledgeBase,
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
    resetSearchResults(results);
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

  function searchStaticNotes(query) {
    const normalizedQuery = normalizeSearchText(query);
    return staticNotes
      .map(function (note) {
        const bodyText = htmlToText(note.body || "");
        const frontmatterText = htmlToText(note.frontmatter || "");
        const title = note.title || note.path || "";
        const path = note.path || "";
        const contentText = [frontmatterText, bodyText].filter(Boolean).join(" ");
        const haystack = normalizeSearchText([title, path, contentText].join(" "));
        const titleMatch = normalizeSearchText(title).includes(normalizedQuery);
        const pathMatch = normalizeSearchText(path).includes(normalizedQuery);
        const contentMatch = haystack.includes(normalizedQuery);
        if (!contentMatch) {
          return null;
        }
        const baseScore = (titleMatch ? 3 : 0) + (pathMatch ? 2 : 0) + 1;
        return {
          path,
          title,
          snippet: staticSnippet(contentText, query),
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

  function searchStaticTag(tag, excludePath) {
    const normalizedTag = normalizeSearchText(tag).trim();
    return staticNotes
      .filter(function (note) {
        return note.path !== excludePath && Array.isArray(note.tags) && note.tags.some(function (candidate) {
          return normalizeSearchText(candidate).trim() === normalizedTag;
        });
      })
      .map(function (note) {
        return {
          path: note.path,
          title: note.title || note.path,
          url: staticRelativeURL(note.path),
          type: "tagged note",
        };
      })
      .sort(function (left, right) {
        return left.path.localeCompare(right.path);
      });
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
