import { Extension, Node } from "@tiptap/core";

const sourceAttribute = "oknBlockId";
const protectedType = "protectedMarkdownBlock";
const blockTypes = ["paragraph", "heading", "blockquote", "bulletList", "orderedList", "taskList", "codeBlock", "horizontalRule", "table", "image", protectedType];

// Keep source identity through ProseMirror transactions. The bridge, rather than
// HTML attributes, owns the original Markdown and decides whether it is intact.
export const MarkdownSourceAttributes = Extension.create({
  name: "markdownSourceAttributes",
  addGlobalAttributes() {
    return [{ types: blockTypes, attributes: { [sourceAttribute]: { default: null, rendered: false } } }];
  },
});

export const ProtectedMarkdownBlock = Node.create({
  name: protectedType,
  group: "block",
  atom: true,
  selectable: true,
  defining: true,
  addAttributes() {
    return {
      raw: { default: "", parseHTML: (element) => element.getAttribute("data-markdown-source") || "", rendered: false },
      reason: { default: "Markdown source", parseHTML: (element) => element.getAttribute("data-markdown-reason") || "Markdown source", rendered: false },
    };
  },
  parseHTML() { return [{ tag: "div[data-protected-markdown]" }]; },
  renderHTML({ node }) {
    return ["div", { "data-protected-markdown": "", "data-markdown-source": node.attrs.raw, "data-markdown-reason": node.attrs.reason, class: "document-protected-block" },
      ["strong", {}, node.attrs.reason],
      ["pre", {}, node.attrs.raw.trim().slice(0, 320)],
      ["small", {}, "Preserved exactly. Your agent can edit this source."],
    ];
  },
  renderMarkdown(node) { return node.attrs.raw || ""; },
});

export function splitMarkdownFrontmatter(text) {
  const firstLine = /^(?:\uFEFF)?---[ \t]*(?:\r\n|\n|\r)/.exec(text);
  if (!firstLine) return { prefix: "", body: text };
  const closing = /^(?:---|\.\.\.)[ \t]*(?:\r\n|\n|\r|$)/gm;
  closing.lastIndex = firstLine[0].length;
  const match = closing.exec(text);
  if (!match) return { prefix: "", body: text };
  const end = match.index + match[0].length;
  return { prefix: text.slice(0, end), body: text.slice(end) };
}

function normalizeLines(text) {
  let normalized = "";
  const offsets = [0];
  for (let index = 0; index < text.length; index++) {
    if (text[index] === "\r") {
      if (text[index + 1] === "\n") index++;
      normalized += "\n";
    } else normalized += text[index];
    offsets.push(index + 1);
  }
  return { normalized, offsets };
}

function protectionReason(token) {
  if (token.type === "code") return "";
  if (token.type === "html") return "HTML or OKF annotation";
  if (token.type === "def") return "Reference definition";
  if (/\[\[|\]\]|\[\^|\{#[^}]+\}|(?:^|\n)\s*:::/m.test(token.raw || "")) return "Extended Markdown";
  if (token.type === "link" || token.type === "image") {
    // Reference links need their separate definitions and must not be expanded
    // into inline URLs merely because another paragraph changed.
    if (/^!?\[/.test(token.raw || "") && !/\]\(/.test(token.raw || "")) return "Reference link";
  }
  const children = [token.tokens, token.items, token.header, token.rows].filter(Array.isArray).flat(2);
  for (const child of children) {
    const reason = protectionReason(child);
    if (reason) return reason;
  }
  return "";
}

function jsonNodeSize(node) {
  if (node.type === "text") return node.text?.length || 0;
  if (["image", "horizontalRule", "hardBreak", protectedType].includes(node.type)) return 1;
  return 2 + (node.content || []).reduce((total, child) => total + jsonNodeSize(child), 0);
}

/** Preserve untouched source blocks while editing supported Markdown visually. */
export function createMarkdownBridge(manager) {
  let frontmatter = "";
  let leading = "";
  let newline = "\n";
  let nextID = 0;
  const originals = new Map();
  const canonical = (node) => manager.serialize({ type: "doc", content: [node] }).replace(/\n+$/, "");

  function load(text) {
    const parts = splitMarkdownFrontmatter(text);
    frontmatter = parts.prefix;
    leading = "";
    newline = text.includes("\r\n") ? "\r\n" : "\n";
    originals.clear();
    const { normalized, offsets } = normalizeLines(parts.body);
    const content = [];
    let cursor = 0;
    const append = (raw, token, reason = "") => {
      if (!raw) return;
      if (!raw.trim()) {
        const previous = content.at(-1);
        if (previous) {
          const original = originals.get(previous.attrs[sourceAttribute]);
          original.raw += raw;
          if (previous.type === protectedType) previous.attrs.raw += raw;
        } else leading += raw;
        return;
      }
      let node;
      let normalizedSource;
      try {
        if (!reason) {
          const supported = ["heading", "paragraph", "text", "code", "blockquote", "list", "taskList", "hr", "table"].includes(token?.type);
          reason = supported ? protectionReason(token) : "Markdown source";
        }
        if (!reason) {
          const parsed = manager.parse(normalizeLines(raw).normalized.replace(/\n+$/, ""));
          if (parsed.content?.length === 1) {
            node = parsed.content[0];
            normalizedSource = canonical(node);
            if (!normalizedSource.trim()) node = undefined;
          }
        }
      } catch { /* Unrecognized content stays lossless in a protected block. */ }
      if (!node) node = { type: protectedType, attrs: { raw, reason: reason || "Markdown source" } };
      const id = `source-${nextID++}`;
      node.attrs = { ...node.attrs, [sourceAttribute]: id };
      originals.set(id, { raw, canonical: normalizedSource, nodeType: node.type, order: content.length });
      content.push(node);
    };
    const tokens = manager.instance.lexer(normalized);
    for (const token of tokens) {
      const start = normalized.indexOf(token.raw, cursor);
      if (!token.raw || start < 0) {
        append(parts.body.slice(offsets[cursor]), null, "Markdown source");
        cursor = normalized.length;
        break;
      }
      if (start > cursor) append(parts.body.slice(offsets[cursor], offsets[start]), null, "Markdown source");
      const end = start + token.raw.length;
      append(parts.body.slice(offsets[start], offsets[end]), token);
      cursor = end;
    }
    if (cursor < normalized.length) append(parts.body.slice(offsets[cursor]), null, "Markdown source");
    if (!content.length) {
      const node = { type: "paragraph", attrs: { [sourceAttribute]: `source-${nextID++}` } };
      originals.set(node.attrs[sourceAttribute], { raw: "", canonical: "", nodeType: "paragraph", order: 0 });
      content.push(node);
    }
    return { type: "doc", content };
  }

  function materialize(json) {
    let text = frontmatter + leading;
    let previous;
    let position = 0;
    const ranges = [];
    for (const node of json.content || []) {
      const original = originals.get(node.attrs?.[sourceAttribute]);
      let raw;
      let unchanged = false;
      if (node.type === protectedType) {
        raw = node.attrs?.raw || "";
        unchanged = raw === original?.raw;
      } else {
        const rendered = canonical(node);
        unchanged = original?.nodeType === node.type && original.canonical === rendered;
        if (unchanged) raw = original.raw;
        else raw = rendered.replace(/\n/g, newline) + (original?.raw.match(/(?:\r\n|\n|\r)+$/)?.[0] || "");
      }
      if (previous && (!previous.unchanged || !unchanged || previous.order + 1 !== original?.order)) {
        const lineBreaks = (text.match(/(?:\r\n|\n|\r)*$/)?.[0].match(/\r\n|\n|\r/g) || []).length;
        if (lineBreaks < 2) text += newline.repeat(2 - lineBreaks);
      }
      const start = text.length;
      text += raw;
      const size = jsonNodeSize(node);
      ranges.push({ from: position, to: position + size, start, end: text.length });
      position += size;
      previous = { unchanged, order: original?.order };
    }
    return { text, ranges };
  }

  return {
    load,
    serialize(json) { return materialize(json).text; },
    selectionToMarkdown(doc, from, to) {
      const json = typeof doc.toJSON === "function" ? doc.toJSON() : doc;
      const { ranges, text } = materialize(json);
      const selected = ranges.filter((range) => from === to ? from >= range.from && from < range.to : to > range.from && from < range.to);
      if (!selected.length) return { start: text.length, end: text.length, selectionScope: "block" };
      return { start: selected[0].start, end: selected.at(-1).end, selectionScope: "block" };
    },
    markdownToSelection(doc, start, end = start) {
      const json = typeof doc.toJSON === "function" ? doc.toJSON() : doc;
      const { ranges } = materialize(json);
      const selected = ranges.filter((range) => start === end ? start >= range.start && start < range.end : end > range.start && start < range.end);
      const first = selected[0] || ranges.at(-1);
      const last = selected.at(-1) || first;
      if (!first) return { from: 0, to: 0, selectionScope: "block" };
      const from = first.from + (first.to - first.from > 1 ? 1 : 0);
      const to = start === end ? from : last.to - (last.to - last.from > 1 ? 1 : 0);
      return { from, to, selectionScope: "block" };
    },
  };
}
