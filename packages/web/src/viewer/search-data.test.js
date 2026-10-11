import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { searchStaticNotes } from "./search-data.js";

beforeEach(() => {
  // Plain-text fixtures isolate ranking from the browser's HTML parser.
  vi.stubGlobal("document", {
    createElement: () => ({
      innerHTML: "",
      get textContent() { return this.innerHTML; },
    }),
  });
});

afterEach(() => vi.unstubAllGlobals());

test("ranks specific matching documents before index pages and body-only matches", () => {
  const notes = [
    { path: "context.md", title: "Context", body: "Details about rollback." },
    { path: "index.md", title: "Rollback overview", body: "Start here." },
    { path: "steps.md", title: "Rollback steps", body: "Restore the previous deployment." },
    { path: "rollback.md", title: "Rollback", body: "The rollback checklist." },
    { path: "unrelated.md", title: "Unrelated", body: "No matching information." },
  ];

  const results = searchStaticNotes(notes, "ROLLBACK");

  expect(results.map(result => result.path)).toEqual([
    "rollback.md", "steps.md", "index.md", "context.md",
  ]);
  expect(results[0]).toMatchObject({ title: "Rollback", snippet: "The rollback checklist." });
  expect(notes[0].path).toBe("context.md");
});

test("applies the requested limit after deterministic ranking and defaults to twelve", () => {
  const notes = Array.from({ length: 15 }, (_, index) => ({
    path: `note-${String(15 - index).padStart(2, "0")}.md`,
    title: "Deployment",
    frontmatter: "Owner: operations",
  }));

  expect(searchStaticNotes(notes, "operations")).toHaveLength(12);
  expect(searchStaticNotes(notes, "operations", 2).map(result => result.path))
    .toEqual(["note-01.md", "note-02.md"]);
  expect(searchStaticNotes(notes, "missing", 2)).toEqual([]);
});
