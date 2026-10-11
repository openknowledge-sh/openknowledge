export const MAX_DOCUMENT_BYTES = 1024 * 1024;

export class DocumentDraft {
  constructor(document, id = crypto.randomUUID()) {
    this.id = id;
    this.version = 0;
    this.path = document.path;
    this.text = editorText(document.text);
    this.baseText = this.text;
    this.lineEnding = sourceLineEnding(document.text);
    this.sourceRevision = document.revision;
    this.selection = { start: 0, end: 0 };
    this.undoStack = [];
    this.redoStack = [];
    this.conflict = null;
    this.lastAuthor = "";
  }

  get revision() { return `${this.id}:${this.version}`; }
  get dirty() { return this.text !== this.baseText; }

  assertRevision(revision) {
    if (revision !== this.revision) throw new Error("The draft changed. Read the editor state again before editing or saving.");
  }

  snapshot() { return { text: this.text, selection: { ...this.selection } }; }
  sourceText(text = this.text) { return this.lineEnding === "\r\n" ? text.replace(/\n/g, "\r\n") : text; }

  select(start, end) {
    this.selection = { start: Math.min(start, this.text.length), end: Math.min(end, this.text.length) };
  }

  apply({ draftRevision, start, end, text }, author = "agent") {
    this.assertRevision(draftRevision);
    if (!Number.isSafeInteger(start) || !Number.isSafeInteger(end) || start < 0 || end < start || end > this.text.length) {
      throw new Error("The edit range must use valid UTF-16 character offsets in the current draft.");
    }
    if (typeof text !== "string") throw new Error("Replacement text must be a string.");
    text = editorText(text);
    const updated = this.text.slice(0, start) + text + this.text.slice(end);
    if (!isWellFormedText(updated)) throw new Error("The edit splits a Unicode character or contains an incomplete Unicode character. Use complete character boundaries.");
    if (new TextEncoder().encode(this.sourceText(updated)).length > MAX_DOCUMENT_BYTES) throw new Error("Documents can contain at most 1 MiB of UTF-8 text.");
    if (updated === this.text) return;
    this.undoStack.push(this.snapshot());
    trimHistory(this.undoStack);
    this.redoStack = [];
    this.text = updated;
    this.selection = { start, end: start + text.length };
    this.version++;
    this.lastAuthor = author;
  }

  undo(revision) { this.moveHistory(revision, this.undoStack, this.redoStack); }
  redo(revision) { this.moveHistory(revision, this.redoStack, this.undoStack); }

  moveHistory(revision, from, to) {
    this.assertRevision(revision);
    if (!from.length) throw new Error("No change to restore.");
    to.push(this.snapshot());
    const restored = from.pop();
    this.text = restored.text;
    this.selection = restored.selection;
    this.version++;
    this.lastAuthor = "undo";
  }

  markSaved(snapshot, revision) {
    this.baseText = snapshot.text;
    this.sourceRevision = revision;
    this.conflict = null;
  }

  observeSource(document) {
    this.conflict = document.revision === this.sourceRevision ? null : { ...document, lineEnding: sourceLineEnding(document.text), text: editorText(document.text) };
  }

  // The explicit reload action remains undoable, including the user's draft.
  useSource(document) {
    this.apply({ draftRevision: this.revision, start: 0, end: this.text.length, text: document.text }, "reload");
    this.lineEnding = document.lineEnding || sourceLineEnding(document.text);
    this.markSaved({ text: editorText(document.text) }, document.revision);
  }

  summary() {
    return {
      path: this.path, draftRevision: this.revision, sourceRevision: this.sourceRevision,
      dirty: this.dirty, selection: { ...this.selection },
      canUndo: this.undoStack.length > 0, canRedo: this.redoStack.length > 0,
      conflict: Boolean(this.conflict), totalChars: this.text.length,
    };
  }
}

function editorText(text) { return text.replace(/\r\n?/g, "\n"); }
function sourceLineEnding(text) { return text.includes("\r\n") && !text.replace(/\r\n/g, "").includes("\n") ? "\r\n" : "\n"; }

function isWellFormedText(text) {
  for (let index = 0; index < text.length; index++) {
    const code = text.charCodeAt(index);
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = text.charCodeAt(++index);
      if (!(next >= 0xdc00 && next <= 0xdfff)) return false;
    } else if (code >= 0xdc00 && code <= 0xdfff) return false;
  }
  return true;
}

function trimHistory(history) {
  let size = history.reduce((sum, entry) => sum + entry.text.length * 2, 0);
  while (history.length > 1 && (history.length > 50 || size > 8 * MAX_DOCUMENT_BYTES)) {
    size -= history.shift().text.length * 2;
  }
}

export function documentDiff(before, after) {
  if (before === after) return "No changes.";
  const oldLines = before.split("\n");
  const newLines = after.split("\n");
  let start = 0;
  while (start < oldLines.length && start < newLines.length && oldLines[start] === newLines[start]) start++;
  let oldEnd = oldLines.length;
  let newEnd = newLines.length;
  while (oldEnd > start && newEnd > start && oldLines[oldEnd - 1] === newLines[newEnd - 1]) { oldEnd--; newEnd--; }
  return [
    `@@ -${start + 1},${oldEnd - start} +${start + 1},${newEnd - start} @@`,
    ...oldLines.slice(Math.max(0, start - 3), start).map((line) => ` ${line}`),
    ...oldLines.slice(start, oldEnd).map((line) => `− ${line}`),
    ...newLines.slice(start, newEnd).map((line) => `+ ${line}`),
    ...newLines.slice(newEnd, newEnd + 3).map((line) => ` ${line}`),
  ].join("\n");
}
