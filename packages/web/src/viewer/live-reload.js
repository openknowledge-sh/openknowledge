(function () {
  if (typeof window.EventSource !== "function") {
    return;
  }

  let revision = "";
  let preparing = false;
  let timer = 0;
  let status;
  const source = new window.EventSource("/api/viewer-events");

  function parseEvent(event) {
    try {
      const parsed = JSON.parse(event.data || "{}");
      return parsed && typeof parsed === "object" ? parsed : {};
    } catch {
      return {};
    }
  }

  function activeKnowledgeBase() {
    return String(document.body.dataset.activeKnowledgeBase || document.querySelector("[data-note-workspace]")?.dataset.knowledgeBase || "").trim();
  }

  function appliesToCurrentKnowledgeBase(event) {
    const aliases = Array.isArray(event.knowledgeBases) ? event.knowledgeBases : [];
    const active = activeKnowledgeBase();
    return !aliases.length || !active || aliases.includes(active);
  }

  function setStatus(message, kind) {
    if (!message) {
      status?.remove();
      status = null;
      return;
    }
    if (!status) {
      status = document.createElement("div");
      status.className = "viewer-live-reload-status";
      status.setAttribute("role", "status");
      document.body.append(status);
    }
    status.dataset.kind = kind || "info";
    status.textContent = message;
  }

  async function applyRevision(nextRevision) {
    const bridge = window.OpenKnowledgeViewerLiveReload;
    if (bridge && typeof bridge.prepare === "function") {
      await bridge.prepare(nextRevision);
      return;
    }
    window.location.reload();
  }

  function schedule(event) {
    const nextRevision = String(event.revision || "");
    if (!nextRevision || nextRevision === revision || preparing || !appliesToCurrentKnowledgeBase(event)) {
      return;
    }
    window.clearTimeout(timer);
    timer = window.setTimeout(async function () {
      preparing = true;
      setStatus("Refreshing changed files…", "info");
      try {
        await applyRevision(nextRevision);
        revision = nextRevision;
      } catch {
        preparing = false;
        setStatus("Live reload could not verify changed files. It will retry after the next change.", "error");
      }
    }, 80);
  }

  source.addEventListener("ready", function (event) {
    const payload = parseEvent(event);
    revision = String(payload.revision || event.lastEventId || revision);
    setStatus("", "");
  });
  source.addEventListener("change", function (event) {
    schedule(parseEvent(event));
  });
  source.addEventListener("status", function (event) {
    const payload = parseEvent(event);
    setStatus(payload.message || "Live reload waits for readable source files.", "waiting");
  });
  source.addEventListener("open", function () {
    if (!preparing) {
      setStatus("", "");
    }
  });
  source.addEventListener("error", function () {
    if (!preparing) {
      setStatus("Live reload disconnected. Reconnecting…", "waiting");
    }
  });
})();
