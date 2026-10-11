import { afterEach, expect, test, vi } from "vitest";
import { createViewerWebMCPTools, registerWebMCPTools } from "./webmcp.js";
import { DocumentDraft } from "./editor-state.js";

afterEach(() => vi.restoreAllMocks());

function fixture(editor) {
  const pages = [
    { path: "index.md", knowledgeBase: "alpha", title: "Alpha", url: "https://wiki.test/alpha/index.html" },
    { path: "index.md", knowledgeBase: "beta", title: "Beta", url: "https://wiki.test/beta/index.html" },
    { path: "notes/test.md", knowledgeBase: "alpha", title: "Test", url: "https://wiki.test/alpha/notes/test.html" },
  ];
  const adapter = {
    editor,
    currentKnowledgeBase: "alpha",
    getPages: () => pages,
    readPage: vi.fn(async (page) => `${page.knowledgeBase}: document content`),
    openPage: vi.fn(async () => {}),
    search: vi.fn(async () => [
      { path: "index.md", knowledgeBase: "beta", snippet: "Beta match" },
      { path: "private.md", knowledgeBase: "beta", snippet: "Excluded" },
      { path: "notes/test.md", snippet: "Current base" },
    ]),
  };
  const tools = createViewerWebMCPTools(adapter);
  return { adapter, tools, call: (name, args, context) => tools.find((tool) => tool.name === `openknowledge_${name}`).execute(args, context) };
}

function editorFixture() {
  const draft = new DocumentDraft({ path: "index.md", text: "# Shared\nHuman text\n", revision: "source-1" }, "shared-document");
  const editor = {
    open: vi.fn(async () => draft.summary()),
    getState: vi.fn((offset, maxChars) => ({ ...draft.summary(), text: draft.text.slice(offset, offset + maxChars) })),
    applyEdit: vi.fn((args) => { draft.apply(args); return draft.summary(); }),
    undo: vi.fn((revision) => { draft.undo(revision); return draft.summary(); }),
    save: vi.fn(async (revision) => { draft.assertRevision(revision); return draft.summary(); }),
  };
  return { ...fixture(editor), draft, editor };
}

test("paginates the catalog and reads the selected knowledge base without changing the viewer", async () => {
  const { call, adapter } = fixture();
  const first = await call("list_pages", { limit: 1 });
  expect(first).toMatchObject({ total: 3, nextOffset: 1 });
  expect((await call("list_pages", { offset: first.nextOffset, limit: 2 })).nextOffset).toBeNull();
  expect((await call("list_pages", { knowledgeBase: "beta" })).pages).toHaveLength(1);
  const read = await call("read_page", { path: "index.md", knowledgeBase: "beta", maxChars: 5 });
  expect(read).toMatchObject({ text: "beta:", nextOffset: 5 });
  const rest = await call("read_page", { path: "index.md", knowledgeBase: "beta", offset: read.nextOffset });
  expect(read.text + rest.text).toBe("beta: document content");
  expect(rest.nextOffset).toBeNull();
  expect(adapter.openPage).not.toHaveBeenCalled();
});

test("rejects guessed paths, unknown bases and invalid arguments before accessing the backend", async () => {
  const { call, adapter } = fixture();
  for (const args of [
    { path: "../secret.md" }, { path: "index.md", knowledgeBase: "unknown" },
    { path: "https://other.test/index.md" }, { path: "index.md", maxChars: 50001 },
    { path: "index.md", offset: -1 }, { path: "index.md", offset: 0.5 },
    { path: "index.md", extra: true }, { path: "index.md", knowledgeBase: null },
  ]) await expect(call("read_page", args)).rejects.toThrow();
  await expect(call("open_page", { path: "index.md", beside: "true" })).rejects.toThrow();
  await expect(call("open_page", { path: "missing.md" })).rejects.toThrow();
  await expect(call("search", { query: " " })).rejects.toThrow();
  await expect(call("search", { query: "word", limit: 31 })).rejects.toThrow();
  await expect(call("list_pages", { limit: null })).rejects.toThrow();
  await expect(call("read_page", {})).rejects.toThrow();
  expect(adapter.readPage).not.toHaveBeenCalled();
  expect(adapter.openPage).not.toHaveBeenCalled();
  expect(adapter.search).not.toHaveBeenCalled();
});

test("search preserves backend ranking and base identity while excluding unavailable results", async () => {
  const { call, adapter } = fixture();
  const signal = new AbortController().signal;
  const result = await call("search", { query: "  matches  ", limit: 20 }, { signal });
  expect(adapter.search).toHaveBeenCalledWith("matches", 20, signal);
  expect(result.results.map((page) => [page.knowledgeBase, page.path, page.snippet])).toEqual([
    ["beta", "index.md", "Beta match"], ["alpha", "notes/test.md", "Current base"],
  ]);
});

test("navigation uses the resolved page and reports backend failure without claiming success", async () => {
  const { call, adapter, tools } = fixture();
  const opened = await call("open_page", { path: "index.md", knowledgeBase: "beta", beside: true });
  expect(opened).toMatchObject({ opened: true, knowledgeBase: "beta" });
  expect(adapter.openPage).toHaveBeenCalledWith(expect.objectContaining({ knowledgeBase: "beta" }), true, undefined);
  expect(tools.find((tool) => tool.name === "openknowledge_open_page").annotations.readOnlyHint).toBe(false);
  adapter.openPage.mockRejectedValueOnce(new Error("Offline"));
  await expect(call("open_page", { path: "index.md" })).rejects.toThrow("Offline");
});

test("cancellation prevents initial work and propagates through reads", async () => {
  const { call, adapter } = fixture();
  const controller = new AbortController();
  controller.abort();
  await expect(call("open_page", { path: "index.md" }, { signal: controller.signal })).rejects.toThrow();
  expect(adapter.openPage).not.toHaveBeenCalled();
  const active = new AbortController();
  adapter.readPage.mockImplementationOnce(async (page, signal) => {
    expect(signal).toBe(active.signal);
    active.abort();
    return "Incomplete result";
  });
  await expect(call("read_page", { path: "index.md" }, { signal: active.signal })).rejects.toThrow();
});

test("registers with document.modelContext and rolls back partial registration failures", async () => {
  const { tools } = fixture();
  const modelContext = { registerTool: vi.fn(async () => {}) };
  const legacy = { registerTool: vi.fn() };
  const dispose = await registerWebMCPTools(tools, { modelContext }, { modelContext: legacy });
  expect(modelContext.registerTool).toHaveBeenCalledTimes(4);
  expect(legacy.registerTool).not.toHaveBeenCalled();
  const signal = modelContext.registerTool.mock.calls[0][1].signal;
  expect(signal.aborted).toBe(false);
  dispose();
  expect(signal.aborted).toBe(true);

  vi.spyOn(console, "warn").mockImplementation(() => {});
  modelContext.registerTool.mockClear().mockRejectedValueOnce(new Error("Disabled"));
  await registerWebMCPTools(tools, { modelContext }, {});
  expect(modelContext.registerTool.mock.calls[0][1].signal.aborted).toBe(true);
});

test("supports legacy navigator registration and is inert in unsupported browsers", async () => {
  const { tools } = fixture();
  const modelContext = { registerTool: vi.fn(), unregisterTool: vi.fn() };
  const dispose = await registerWebMCPTools(tools, {}, { modelContext });
  dispose();
  dispose();
  expect(modelContext.unregisterTool.mock.calls.flat()).toEqual(tools.map((tool) => tool.name));
  expect(typeof await registerWebMCPTools(tools, {}, {})).toBe("function");
});

test("editor tools change the same human draft, reject stale edits, and share Undo", async () => {
  const { call, draft } = editorFixture();
  const agentRead = await call("get_editor_state", {});
  draft.apply({ draftRevision: draft.revision, start: 9, end: 14, text: "New human" }, "human");
  const humanText = draft.text;
  await expect(call("apply_edit", { draftRevision: agentRead.draftRevision, start: 9, end: 14, text: "Agent" })).rejects.toThrow("draft changed");
  expect(draft.text).toBe(humanText);

  const current = await call("get_editor_state", {});
  expect(current.text).toBe(humanText);
  const applied = await call("apply_edit", { draftRevision: current.draftRevision, start: draft.text.length, end: draft.text.length, text: "Agent addition\n" });
  expect(draft.text).toBe(`${humanText}Agent addition\n`);
  expect(applied).toMatchObject({ dirty: true, canUndo: true });
  await expect(call("save_document", { draftRevision: current.draftRevision })).rejects.toThrow("draft changed");
  await call("undo_edit", { draftRevision: applied.draftRevision });
  expect(draft.text).toBe(humanText);
  expect(draft.sourceRevision).toBe("source-1");
});

test("opening an editor validates inventory and navigates before binding the shared draft", async () => {
  const { call, adapter, editor } = editorFixture();
  const signal = new AbortController().signal;
  await expect(call("open_editor", { path: "../secret.md" })).rejects.toThrow("unavailable");
  await expect(call("open_editor", { path: "index.md", knowledgeBase: "unknown" })).rejects.toThrow("unavailable");
  expect(adapter.openPage).not.toHaveBeenCalled();
  expect(editor.open).not.toHaveBeenCalled();

  await call("open_editor", { path: "index.md", knowledgeBase: "beta" }, { signal });
  const page = adapter.getPages().find((candidate) => candidate.knowledgeBase === "beta");
  expect(adapter.openPage).toHaveBeenCalledWith(page, false, signal);
  expect(editor.open).toHaveBeenCalledWith(page, undefined, signal);
  expect(adapter.openPage.mock.invocationCallOrder[0]).toBeLessThan(editor.open.mock.invocationCallOrder[0]);

  editor.open.mockClear();
  adapter.openPage.mockRejectedValueOnce(new Error("Navigation failed"));
  await expect(call("open_editor", { path: "index.md" })).rejects.toThrow("Navigation failed");
  expect(editor.open).not.toHaveBeenCalled();
});

test("editor reads enforce pagination bounds before accessing unsaved content", async () => {
  const { call, editor, draft } = editorFixture();
  expect((await call("get_editor_state", {})).text).toBe(draft.text);
  expect(editor.getState).toHaveBeenLastCalledWith(0, 20000);
  expect((await call("get_editor_state", { offset: 9, maxChars: 5 })).text).toBe("Human");
  expect(editor.getState).toHaveBeenLastCalledWith(9, 5);
  editor.getState.mockClear();
  for (const args of [{ offset: -1 }, { offset: 1.5 }, { offset: null }, { maxChars: 0 }, { maxChars: 50001 }, { maxChars: "10" }, { path: "index.md" }]) {
    await expect(call("get_editor_state", args)).rejects.toThrow();
  }
  expect(editor.getState).not.toHaveBeenCalled();
});

test("saving forwards the draft revision and cancellation signal without bypassing cancellation", async () => {
  const { call, editor, draft, tools } = editorFixture();
  const signal = new AbortController().signal;
  await call("save_document", { draftRevision: draft.revision }, { signal });
  expect(editor.save).toHaveBeenCalledWith(draft.revision, signal);
  expect(tools.find((tool) => tool.name === "openknowledge_save_document").annotations.readOnlyHint).toBe(false);
  editor.save.mockClear();
  const aborted = new AbortController();
  aborted.abort();
  await expect(call("save_document", { draftRevision: draft.revision }, { signal: aborted.signal })).rejects.toThrow();
  expect(editor.save).not.toHaveBeenCalled();
});

test("browse-only adapters never advertise editor operations", () => {
  expect(fixture().tools.map((tool) => tool.name)).toEqual([
    "openknowledge_list_pages", "openknowledge_search", "openknowledge_read_page", "openknowledge_open_page",
  ]);
});
