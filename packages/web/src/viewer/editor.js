import { DocumentDraft, documentDiff } from "./editor-state.js";
import { createRichEditor } from "./rich-editor.js";

const identity = (page) => JSON.stringify([page.knowledgeBase || "", page.path]);

export function createViewerEditor(adapter) {
  const storageKey = "openknowledge.editor.drafts.v2";
  const storageIdentity = (page) => JSON.stringify([adapter.storageScopeForPage(page), page.path]);
  const sessions = new Map();
  const loading = new Map();
  const bindings = new WeakMap();
  let recovered = {};
  try { recovered = JSON.parse(sessionStorage.getItem(storageKey) || "{}"); } catch { /* Browser storage is optional. */ }
  if (!recovered || typeof recovered !== "object" || Array.isArray(recovered)) recovered = {};

  function persist() {
    for (const session of sessions.values()) {
      const key = storageIdentity(session.page);
      const draft = session.draft;
      if (draft.dirty) recovered[key] = { path: draft.path, knowledgeBase: session.page.knowledgeBase, text: draft.text, baseText: draft.baseText, revision: draft.sourceRevision, lineEnding: draft.lineEnding };
      else delete recovered[key];
    }
    try { sessionStorage.setItem(storageKey, JSON.stringify(recovered)); }
    catch {
      for (const session of sessions.values()) if (session.draft.dirty) session.message = "Draft recovery is unavailable. Keep this tab open until you save.";
    }
  }

  function hasRecoveredDrafts() {
    return Object.entries(recovered).some(([key, page]) => {
      try { return key === storageIdentity(page); } catch { return false; }
    });
  }

  function pageFor(panel) {
    return { path: panel.dataset.notePath, knowledgeBase: panel.dataset.knowledgeBase || adapter.currentKnowledgeBase || "" };
  }

  function current() {
    const panel = adapter.activePanel();
    const session = panel && sessions.get(identity(pageFor(panel)));
    if (!session || !panel.querySelector("[data-document-editor]")) throw new Error("Open a document with openknowledge_open_editor first.");
    return session;
  }

  async function request(page, payload, signal) {
    const url = new URL(adapter.documentURL(page.path, page.knowledgeBase), window.location.href);
    url.searchParams.set("path", page.path);
    const response = await fetch(url, {
      method: payload ? "POST" : "GET", signal,
      headers: { Accept: "application/json", ...(payload ? { "Content-Type": "application/json" } : {}) },
      ...(payload ? { body: JSON.stringify(payload) } : {}),
    });
    let data;
    try { data = await response.json(); } catch { throw new Error("The editor could not read the server response. Restart the local viewer with the updated CLI."); }
    if (!response.ok) {
      const error = new Error(data.error || "The document request failed.");
      error.details = data;
      throw error;
    }
    return data;
  }

  function bindPanel(panel) {
    if (adapter.staticMode || !panel.isConnected || !/\.(md|markdown)$/i.test(panel.dataset.notePath || "") || panel.querySelector(".note-error")) return;
    if (!bindings.has(panel)) {
      bindings.set(panel, open(pageFor(panel), panel, undefined, { focus: false }).catch((error) => {
        bindings.delete(panel);
        if (panel.isConnected && error.code !== "READ_ONLY") showPanelError(panel, error.message);
      }));
    }
    return bindings.get(panel);
  }

  async function open(page, panel = adapter.activePanel(), signal, { focus = true } = {}) {
    if (adapter.staticMode) throw new Error("Static exports are read-only.");
    if (!panel || identity(pageFor(panel)) !== identity(page)) throw new Error("Open this page in the viewer first.");
    const key = identity(page);
    if (!sessions.has(key)) {
      if (!loading.has(key)) loading.set(key, request(page, undefined, signal).then((document) => {
        assertWritable(document);
        const draft = new DocumentDraft(document);
        const previous = recovered[storageIdentity(page)];
        if (previous && previous.path === page.path && typeof previous.text === "string" && typeof previous.baseText === "string" && typeof previous.revision === "string") {
          draft.baseText = previous.baseText;
          draft.sourceRevision = previous.revision;
          if (previous.lineEnding === "\n" || previous.lineEnding === "\r\n") draft.lineEnding = previous.lineEnding;
          draft.apply({ draftRevision: draft.revision, start: 0, end: draft.text.length, text: previous.text }, "recovery");
          draft.observeSource(document);
        }
        sessions.set(key, { page, draft, views: new Set(), message: previous ? "Recovered your draft in this tab." : "", saving: null });
      }).finally(() => loading.delete(key)));
      await loading.get(key);
    } else {
      const session = sessions.get(key);
      if (session.saving) await session.saving;
      const document = await request(page, undefined, signal);
      assertWritable(document);
      if (document.revision !== session.draft.sourceRevision) {
        if (session.draft.dirty) session.draft.observeSource(document);
        else session.draft.useSource(document);
      }
    }
    signal?.throwIfAborted();
    const session = sessions.get(key);
    if (!panel.isConnected) throw new Error("The document panel changed. Open the editor again.");
    attach(session, panel);
    render(session);
    if (focus) panel.querySelector(".document-rich-content")?.focus();
    return state(session);
  }

  function attach(session, panel) {
    if (panel.querySelector("[data-document-editor]")) return;
    panel.querySelector("[data-editor-error]")?.remove();
    const root = document.createElement("section");
    root.className = "document-editor";
    root.dataset.documentEditor = "";
    root.setAttribute("aria-label", `Document editor: ${session.page.path}`);
    const toolbar = document.createElement("div");
    toolbar.className = "document-editor-toolbar";
    const undo = makeButton("Undo");
    const redo = makeButton("Redo");
    const changes = makeButton("Changes");
    const save = makeButton("Save", "document-editor-save");
    const status = document.createElement("p");
    status.className = "document-editor-status";
    status.setAttribute("role", "status");
    toolbar.append(status, undo, redo, changes, save);
    const richHost = document.createElement("div");
    const preview = document.createElement("details");
    preview.className = "document-saved-preview";
    const previewLabel = document.createElement("summary");
    previewLabel.textContent = "Preview saved page";
    const previewBody = document.createElement("div");
    previewBody.className = "note-body";
    preview.append(previewLabel, previewBody);
    const refreshPreview = async () => {
      if (!preview.open) return;
      previewBody.textContent = "Loading saved page…";
      try { await adapter.renderPreview(session.page, previewBody); }
      catch { previewBody.textContent = "Could not load the saved page. Close and reopen the preview to retry."; }
    };
    preview.addEventListener("toggle", () => { void refreshPreview(); });
    const diff = document.createElement("pre");
    diff.className = "document-editor-diff";
    diff.setAttribute("aria-label", "Changes from the saved version");
    diff.hidden = true;
    const conflict = document.createElement("div");
    conflict.className = "document-editor-conflict";
    conflict.hidden = true;
    const conflictText = document.createElement("p");
    conflictText.textContent = "The saved file changed. Your draft is preserved. Compare the versions before saving.";
    const compare = makeButton("Compare saved file");
    const useSaved = makeButton("Load saved version");
    useSaved.title = "Replace the draft with the current saved file. Undo restores your draft.";
    const comparison = document.createElement("pre");
    comparison.hidden = true;
    comparison.className = "document-editor-diff";
    comparison.setAttribute("aria-label", "Current saved file compared with your draft");
    conflict.append(conflictText, compare, useSaved, comparison);
    root.append(toolbar, conflict, preview, richHost, diff);
    panel.querySelector(":scope > .note-body").hidden = true;
    panel.classList.add("has-document-editor");
    panel.append(root);
    const ui = { root, panel, undo, redo, changes, save, useSaved, status, rich: null, diff, conflict, comparison, refreshPreview };
    session.views.add(ui);
    const act = (operation) => {
      try { operation(); session.richSelection = { scope: "source" }; session.message = ""; persist(); render(session); }
      catch (error) { session.message = error.message; render(session); }
    };
    ui.rich = createRichEditor({
      element: richHost, text: session.draft.text,
      documentURL: adapter.fileURL(session.page.path, session.page.knowledgeBase),
      onOpenLink: (href) => adapter.openLink(session.page, href),
      onChange(text) {
        try {
          session.draft.apply({ draftRevision: session.draft.revision, start: 0, end: session.draft.text.length, text }, "human");
          session.message = ""; persist(); render(session);
        } catch (error) { session.message = error.message; render(session); throw error; }
      },
      onSelection(selection) {
        session.draft.select(selection.start, selection.end);
        session.richSelection = { scope: selection.selectionScope, selectedText: selection.selectedText };
      },
      onUndo: () => act(() => session.draft.undo(session.draft.revision)),
      onRedo: () => act(() => session.draft.redo(session.draft.revision)),
      onSave: () => { void saveSession(session).catch(() => {}); },
    });
    undo.addEventListener("click", () => act(() => session.draft.undo(session.draft.revision)));
    redo.addEventListener("click", () => act(() => session.draft.redo(session.draft.revision)));
    changes.addEventListener("click", () => { diff.hidden = !diff.hidden; render(session); });
    save.addEventListener("click", () => { void saveSession(session).catch(() => {}); });
    compare.addEventListener("click", () => { comparison.hidden = !comparison.hidden; render(session); });
    useSaved.addEventListener("click", () => act(() => session.draft.useSource(session.draft.conflict)));
    render(session);
  }

  function render(session) {
    const draft = session.draft;
    for (const ui of session.views) {
      if (!ui.root.isConnected) { ui.rich?.destroy(); session.views.delete(ui); continue; }
      ui.panel.dataset.editorDirty = String(draft.dirty);
      ui.rich?.setText(draft.text);
      ui.undo.disabled = !draft.undoStack.length;
      ui.redo.disabled = !draft.redoStack.length;
      ui.save.disabled = Boolean(session.saving) || !draft.dirty || Boolean(draft.conflict);
      ui.useSaved.disabled = Boolean(session.saving);
      ui.save.textContent = session.saving ? "Saving…" : "Save";
      ui.changes.textContent = ui.diff.hidden ? "Changes" : "Hide changes";
      ui.changes.setAttribute("aria-expanded", String(!ui.diff.hidden));
      const status = session.saving ? "Saving…" : draft.dirty ? (draft.lastAuthor === "agent" ? "Agent changes · Not saved" : "Not saved") : "Saved";
      ui.status.textContent = [status, session.message].filter(Boolean).join(" · ");
      ui.conflict.hidden = !draft.conflict;
      if (!ui.diff.hidden) ui.diff.textContent = documentDiff(draft.baseText, draft.text);
      if (draft.conflict && !ui.comparison.hidden) ui.comparison.textContent = documentDiff(draft.conflict.text, draft.text);
    }
  }

  async function saveSession(session, signal) {
    if (session.saving) throw new Error("A save is already in progress.");
    if (session.draft.conflict) throw new Error("The saved file changed. Resolve the conflict in the editor first.");
    if (!session.draft.dirty) return state(session);
    const snapshot = session.draft.snapshot();
    const expectedRevision = session.draft.sourceRevision;
    session.saving = request(session.page, { path: session.page.path, text: session.draft.sourceText(snapshot.text), expectedRevision }, signal);
    render(session);
    try {
      const saved = await session.saving;
      session.draft.markSaved(snapshot, saved.revision);
      for (const ui of session.views) if (ui.root.isConnected) void ui.refreshPreview();
      session.message = (saved.validation?.warnings || []).map((warning) => warning.message).join(" ");
      return state(session);
    } catch (error) {
      if (error.details?.current) session.draft.observeSource(error.details.current);
      session.message = error.details?.diagnostics?.map((item) => `Line ${item.line || 1}: ${item.message}`).join(" ") || error.message;
      throw error;
    } finally {
      session.saving = null;
      persist(); render(session);
    }
  }

  function state(session, offset = 0, maxChars = 20000) {
    const draft = session.draft;
    const end = Math.min(draft.text.length, offset + maxChars);
    return {
      ...draft.summary(), knowledgeBase: session.page.knowledgeBase, open: true,
      text: draft.text.slice(offset, end), offset, nextOffset: end < draft.text.length ? end : null,
      selection: { ...draft.selection, ...session.richSelection, selectedText: session.richSelection?.selectedText?.slice(0, maxChars), text: draft.text.slice(draft.selection.start, draft.selection.end).slice(0, maxChars) },
    };
  }

  window.addEventListener("beforeunload", (event) => {
    persist();
    if (hasRecoveredDrafts()) { event.preventDefault(); event.returnValue = ""; }
  });

  return {
    bindPanel, open,
    getState(offset, maxChars) { return state(current(), offset, maxChars); },
    applyEdit(args) {
      const session = current();
      session.draft.apply(args); session.richSelection = { scope: "source", selectedText: args.text }; session.message = ""; persist(); render(session);
      return session.draft.summary();
    },
    undo(draftRevision) {
      const session = current();
      session.draft.undo(draftRevision); session.richSelection = { scope: "source" }; persist(); render(session);
      return session.draft.summary();
    },
    save(draftRevision, signal) {
      const session = current(); session.draft.assertRevision(draftRevision);
      return saveSession(session, signal);
    },
    async handleSourceChanges() {
      // A clean viewer must still reload its catalog, metadata and deleted
      // panels. Only unsaved drafts require keeping this editor instance alive.
      if (!Array.from(sessions.values()).some((session) => session.draft.dirty || session.saving) && !hasRecoveredDrafts()) return false;
      const active = Array.from(sessions.values()).filter((session) => session.draft.dirty || Array.from(session.views).some((ui) => ui.root.isConnected));
      if (!active.length && !hasRecoveredDrafts()) return false;
      await Promise.all(active.map(async (session) => {
        try {
          if (session.saving) await session.saving.catch(() => {});
          const document = await request(session.page);
          if (session.draft.dirty) session.draft.observeSource(document);
          else if (document.revision !== session.draft.sourceRevision) {
            session.draft.useSource(document);
            session.richSelection = { scope: "source" };
            session.message = "";
          }
        } catch (error) { session.message = `Could not check the saved file: ${error.message}`; }
        render(session);
      }));
      return true;
    },
  };
}

function assertWritable(document) {
  if (document.writable) return;
  const error = new Error("This knowledge base is read-only.");
  error.code = "READ_ONLY";
  throw error;
}

function makeButton(text, className = "") {
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = text;
  button.className = className;
  return button;
}

function showPanelError(panel, message) {
  let status = panel.querySelector("[data-editor-error]");
  if (!status) {
    status = document.createElement("p"); status.dataset.editorError = "";
    status.className = "document-editor-status"; status.setAttribute("role", "alert");
    panel.querySelector(".note-chrome")?.after(status);
  }
  status.textContent = message;
}
