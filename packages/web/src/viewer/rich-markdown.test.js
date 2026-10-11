import { expect, test } from "vitest";
import { getSchema } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import Image from "@tiptap/extension-image";
import { TableKit } from "@tiptap/extension-table";
import TaskList from "@tiptap/extension-task-list";
import TaskItem from "@tiptap/extension-task-item";
import { MarkdownManager } from "@tiptap/markdown";
import { createMarkdownBridge, MarkdownSourceAttributes, ProtectedMarkdownBlock, splitMarkdownFrontmatter } from "./rich-markdown.js";

const extensions = [StarterKit, Image, TableKit, TaskList, TaskItem, MarkdownSourceAttributes, ProtectedMarkdownBlock];
const manager = new MarkdownManager({ extensions });
const schema = getSchema(extensions);
const fixture = () => createMarkdownBridge(manager);
const schemaJSON = (json) => schema.nodeFromJSON(json).toJSON();

test("frontmatter, comments, reference syntax, lists, tables and code survive an untouched rich-editor round trip", () => {
  const source = [
    "---", "title: 'Original: title'", "sources:", "  - id: src", "    url: https://example.com", "---", "",
    "#  Heading", "", "Normal **bold** and _italic_ text.", "", "- one", "- two", "",
    "| A | B |", "| :- | -: |", "| x | y |", "", "- [ ] Todo", "- [x] Done", "",
    "```mermaid", "graph LR", "A --> B", "```", "", "A [[wiki link]] and claim[^src].", "",
    "A [reference][source].", "", "[source]: https://example.com \"Title\"", "", "[^src]: Evidence", "",
    "<!-- okf-annotation: agent-context -->", "", "Editable annotation content.", "", "<!-- /okf-annotation -->", "",
    '<a id="stable-anchor"></a>', "", "<!-- okf-footer: agent-maintenance -->", "", "Last paragraph without final newline",
  ].join("\n");
  const bridge = fixture();
  const json = schemaJSON(bridge.load(source));
  expect(bridge.serialize(json)).toBe(source);
  expect(json.content.filter((node) => node.type === "protectedMarkdownBlock").length).toBeGreaterThanOrEqual(7);
  expect(json.content.some((node) => node.type === "table")).toBe(true);
  expect(json.content.some((node) => node.type === "taskList")).toBe(true);
});

test("editing a supported block preserves YAML bytes and untouched protected blocks including CRLF", () => {
  const prefix = "\uFEFF---\r\ntitle: \"Keep # and :\"\r\nclaims:\r\n  - id: a\r\n---\r\n";
  const source = prefix + "\r\nFirst paragraph.\r\n\r\n<!-- okf-footer: agent-maintenance -->\r\n\r\n[[Exact|Wiki syntax]]\r\n";
  const bridge = fixture();
  const json = schemaJSON(bridge.load(source));
  const paragraph = json.content.find((node) => node.type === "paragraph");
  paragraph.content[0].text = "Changed paragraph.";
  const saved = bridge.serialize(json);
  expect(saved).toBe(source.replace("First paragraph.", "Changed paragraph."));
  expect(saved.startsWith(prefix)).toBe(true);
  expect(splitMarkdownFrontmatter(saved).prefix).toBe(prefix);
});

test("body formatting does not expand or discard source references and OKF annotations", () => {
  const source = "A plain paragraph.\n\nClaim[^evidence] with [reference][id].\n\n[^evidence]: Exact evidence\n\n[id]: ./source.md\n\n<!-- hidden annotation -->\n";
  const bridge = fixture();
  const json = schemaJSON(bridge.load(source));
  json.content[0].content[0].marks = [{ type: "bold" }];
  const saved = bridge.serialize(json);
  expect(saved).toContain("**A plain paragraph.**");
  expect(saved.slice(saved.indexOf("Claim"))).toBe(source.slice(source.indexOf("Claim")));
});

test("unsupported HTML nested in tables or lists stays protected while normal nested lists stay editable", () => {
  const source = "- Parent\n  - Child **bold**\n\n| A | B |\n| - | - |\n| <abbr title=\"Meaning\">Word</abbr> | Plain |\n\n- List with <span>HTML</span>\n";
  const bridge = fixture();
  const json = schemaJSON(bridge.load(source));
  expect(json.content[0].type).toBe("bulletList");
  expect(json.content.filter((node) => node.type === "protectedMarkdownBlock")).toHaveLength(2);
  expect(bridge.serialize(json)).toBe(source);
  json.content[0].content[0].content[0].content[0].text = "Edited parent";
  const saved = bridge.serialize(json);
  expect(saved).toContain('<abbr title="Meaning">Word</abbr>');
  expect(saved).toContain("<span>HTML</span>");
});

test("new, moved, and deleted blocks retain valid separation and do not reuse unrelated original Markdown", () => {
  const bridge = fixture();
  const json = schemaJSON(bridge.load("# Title\n\nOriginal last paragraph"));
  const [heading, paragraph] = json.content;
  json.content = [paragraph, heading, { type: "paragraph", content: [{ type: "text", text: "New paragraph" }] }];
  expect(bridge.serialize(json)).toBe("Original last paragraph\n\n# Title\n\nNew paragraph");
  json.content = [heading];
  expect(bridge.serialize(json)).toBe("# Title\n\n");
});

test("rich selection maps conservatively to UTF-16 Markdown blocks including hidden frontmatter", () => {
  const source = "---\ntitle: Example\n---\n\n# Title\n\nAn emoji 😀 and **bold** text.\n\nLast";
  const bridge = fixture();
  const doc = schema.nodeFromJSON(bridge.load(source));
  const paragraphStart = doc.child(0).nodeSize;
  const selected = bridge.selectionToMarkdown(doc, paragraphStart + 2, paragraphStart + 10);
  expect(selected.selectionScope).toBe("block");
  expect(source.slice(selected.start, selected.end)).toBe("An emoji 😀 and **bold** text.\n\n");
  expect(selected.start).toBe(source.indexOf("An emoji"));
  const collapsed = bridge.selectionToMarkdown(doc, paragraphStart + 3, paragraphStart + 3);
  expect(collapsed).toEqual(selected);
  const spanning = bridge.selectionToMarkdown(doc, 1, paragraphStart + 10);
  expect(source.slice(spanning.start, spanning.end)).toBe("# Title\n\nAn emoji 😀 and **bold** text.\n\n");
  expect(bridge.markdownToSelection(doc, selected.start, selected.end)).toEqual({ from: paragraphStart + 1, to: paragraphStart + doc.child(1).nodeSize - 1, selectionScope: "block" });
  expect(bridge.markdownToSelection(doc, selected.start)).toEqual({ from: paragraphStart + 1, to: paragraphStart + 1, selectionScope: "block" });
});

test("loading an agent revision replaces the source map without inheriting earlier frontmatter or block identities", () => {
  const bridge = fixture();
  bridge.load("---\ntitle: First\n---\n\nFirst");
  const next = "---\ntitle: Agent\n---\n\nNew **agent** document\n";
  const json = schemaJSON(bridge.load(next));
  expect(bridge.serialize(json)).toBe(next);
  expect(bridge.selectionToMarkdown(json, 1, 2).start).toBe(next.indexOf("New"));
});

test("empty documents and incomplete frontmatter remain lossless", () => {
  for (const source of ["", "\n\n", "---\ntitle: Metadata\n---\n", "---\nUnclosed frontmatter", "\r\n\r\n"]) {
    const bridge = fixture();
    expect(bridge.serialize(schemaJSON(bridge.load(source)))).toBe(source);
  }
});
