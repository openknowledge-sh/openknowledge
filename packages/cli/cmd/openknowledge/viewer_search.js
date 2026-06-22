
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
      link.href = item.highlightURL || item.url || staticRelativeURL(item.path);
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
