import { expect, test } from "vitest";
import { DocumentDraft, documentDiff, MAX_DOCUMENT_BYTES } from "./editor-state.js";

const draft = (text = "First paragraph.\nSecond paragraph.") => new DocumentDraft({ path: "note.md", text, revision: "source-1" }, "session");

test("human and agent edits share one revision and reversible history", () => {
  const document = draft("Hello");
  document.apply({ draftRevision: document.revision, start: 5, end: 5, text: " human" }, "human");
  const humanRevision = document.revision;
  document.apply({ draftRevision: humanRevision, start: 6, end: 11, text: "agent" });
  expect(document.text).toBe("Hello agent");
  expect(document.dirty).toBe(true);
  document.undo(document.revision);
  expect(document.text).toBe("Hello human");
  expect(document.revision).not.toBe(humanRevision);
  document.undo(document.revision);
  expect(document.text).toBe("Hello");
  expect(document.dirty).toBe(false);
  document.redo(document.revision);
  expect(document.text).toBe("Hello human");
});

test("rejects stale or invalid edits without altering text or undo history", () => {
  const document = draft();
  const before = document.snapshot();
  for (const change of [
    { draftRevision: "stale", start: 0, end: 1, text: "x" },
    { draftRevision: document.revision, start: -1, end: 1, text: "x" },
    { draftRevision: document.revision, start: 2, end: 1, text: "x" },
    { draftRevision: document.revision, start: 0, end: 500, text: "x" },
    { draftRevision: document.revision, start: 0.5, end: 1, text: "x" },
  ]) expect(() => document.apply(change)).toThrow();
  expect(document.snapshot()).toEqual(before);
  expect(document.undoStack).toHaveLength(0);
});

test("uses textarea offsets for Unicode and restores CRLF when saving", () => {
  const document = draft("---\r\ntype: Note\r\n---\r\nA 🌲 tree.");
  const start = document.text.indexOf("🌲");
  document.apply({ draftRevision: document.revision, start, end: start + 2, text: "🌳" });
  expect(document.text).toBe("---\ntype: Note\n---\nA 🌳 tree.");
  expect(document.sourceText()).toBe("---\r\ntype: Note\r\n---\r\nA 🌳 tree.");
  document.undo(document.revision);
  expect(document.text).toContain("🌲");
});

test("rejects split or incomplete Unicode characters without changing the draft", () => {
  const document = draft("A 🌲 tree.");
  const before = document.snapshot();
  const start = document.text.indexOf("🌲");
  for (const change of [
    { start, end: start + 1, text: "" },
    { start: start + 1, end: start + 2, text: "" },
    { start: 0, end: 1, text: "🌲".slice(0, 1) },
    { start: 0, end: 1, text: "🌲".slice(1) },
  ]) expect(() => document.apply({ draftRevision: document.revision, ...change })).toThrow("Unicode");
  expect(document.snapshot()).toEqual(before);
  expect(document.undoStack).toHaveLength(0);
  document.apply({ draftRevision: document.revision, start, end: start + 2, text: "" });
  expect(document.text).toBe("A  tree.");
});

test("a completed save preserves edits made while the request was in flight", () => {
  const document = draft("Original");
  document.apply({ draftRevision: document.revision, start: 0, end: 8, text: "First edit" });
  const submitted = document.snapshot();
  document.apply({ draftRevision: document.revision, start: 0, end: 10, text: "Second edit" }, "human");
  document.markSaved(submitted, "source-2");
  expect(document.text).toBe("Second edit");
  expect(document.baseText).toBe("First edit");
  expect(document.sourceRevision).toBe("source-2");
  expect(document.dirty).toBe(true);
});

test("external changes preserve the draft and explicit source reload can be undone", () => {
  const document = draft("Original");
  document.apply({ draftRevision: document.revision, start: 0, end: 8, text: "My unsaved work" });
  const external = { text: "External edit", revision: "source-2" };
  document.observeSource(external);
  expect(document.conflict).toEqual({ ...external, lineEnding: "\n" });
  expect(document.text).toBe("My unsaved work");
  document.useSource(external);
  expect(document.dirty).toBe(false);
  expect(document.conflict).toBeNull();
  document.undo(document.revision);
  expect(document.text).toBe("My unsaved work");
  expect(document.sourceRevision).toBe("source-2");
  expect(document.dirty).toBe(true);
});

test("save is reversible through the same draft history", () => {
  const document = draft("Original");
  document.apply({ draftRevision: document.revision, start: 0, end: 8, text: "Saved edit" });
  document.markSaved(document.snapshot(), "source-2");
  expect(document.dirty).toBe(false);
  document.undo(document.revision);
  expect(document.text).toBe("Original");
  expect(document.dirty).toBe(true);
});

test("enforces UTF-8 byte limits and a fresh edit clears redo", () => {
  const document = draft("A");
  expect(() => document.apply({ draftRevision: document.revision, start: 0, end: 1, text: "é".repeat(MAX_DOCUMENT_BYTES / 2 + 1) })).toThrow("1 MiB");
  document.apply({ draftRevision: document.revision, start: 0, end: 1, text: "B" });
  document.undo(document.revision);
  document.apply({ draftRevision: document.revision, start: 0, end: 1, text: "C" });
  expect(document.redoStack).toHaveLength(0);
});

test("diff shows removed and added lines including insertions and deletions", () => {
  expect(documentDiff("Same", "Same")).toBe("No changes.");
  expect(documentDiff("Before\nOld\nAfter", "Before\nNew\nAfter")).toContain("− Old\n+ New");
  expect(documentDiff("First\nLast", "First\nAdded\nLast")).toContain("+ Added");
  expect(documentDiff("First\nDeleted\nLast", "First\nLast")).toContain("− Deleted");
});
