import { Editor, mergeAttributes } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import { Markdown } from "@tiptap/markdown";
import { TableKit } from "@tiptap/extension-table";
import TaskList from "@tiptap/extension-task-list";
import TaskItem from "@tiptap/extension-task-item";
import Image from "@tiptap/extension-image";
import Placeholder from "@tiptap/extension-placeholder";
import { createMarkdownBridge, ProtectedMarkdownBlock, MarkdownSourceAttributes } from "./rich-markdown.js";

const BLOCKS = [
  { name: "Text", hint: "Plain paragraph", icon: "¶", keywords: "paragraph odstavec text", run: (chain) => chain.setParagraph() },
  ...[1, 2, 3].map((level) => ({ name: `Heading ${level}`, hint: `${"#".repeat(level)} + space`, icon: `H${level}`, keywords: `heading nadpis h${level}`, run: (chain) => chain.setHeading({ level }) })),
  { name: "Bullet list", hint: "A simple list", icon: "•", keywords: "bullet list seznam odrážky", run: (chain) => chain.toggleBulletList() },
  { name: "Numbered list", hint: "An ordered list", icon: "1.", keywords: "number list seznam čísla", run: (chain) => chain.toggleOrderedList() },
  { name: "To-do list", hint: "Track a task", icon: "☑", keywords: "task todo úkol check", run: (chain) => chain.toggleTaskList() },
  { name: "Quote", hint: "Highlight a quotation", icon: "❝", keywords: "quote citace", run: (chain) => chain.toggleBlockquote() },
  { name: "Code", hint: "A code block", icon: "<>", keywords: "code kód", run: (chain) => chain.toggleCodeBlock() },
  { name: "Divider", hint: "Separate sections", icon: "—", keywords: "divider horizontal oddělovač", run: (chain) => chain.setHorizontalRule() },
  { name: "Table", hint: "Three columns with a header", icon: "▦", keywords: "table tabulka", run: (chain) => chain.insertTable({ rows: 3, cols: 3, withHeaderRow: true }) },
];

function imageDisplayURL(source, documentURL) {
  try {
    const page = new URL(documentURL, window.location.href);
    const marker = page.pathname.indexOf("/file/");
    const prefix = marker < 0 ? "/" : page.pathname.slice(0, marker + 1);
    const url = new URL(source, page);
    if (source.startsWith("/") && !source.startsWith("//") && !source.startsWith(prefix + "raw/")) {
      url.pathname = prefix + "raw" + url.pathname;
    } else if (url.origin === page.origin && url.pathname.startsWith(prefix + "file/")) {
      url.pathname = prefix + "raw/" + url.pathname.slice((prefix + "file/").length);
    }
    return /^(https?:|data:)$/.test(url.protocol) ? url.href : "";
  } catch { return ""; }
}

export function createRichEditor({ element, text, documentURL, onChange, onSelection, onUndo, onRedo, onSave, onOpenLink }) {
  element.classList.add("document-rich-surface");
  const format = document.createElement("div");
  format.className = "document-rich-format";
  format.setAttribute("role", "toolbar");
  format.setAttribute("aria-label", "Text formatting");
  const canvas = document.createElement("div");
  const menu = document.createElement("div");
  menu.className = "document-block-menu";
  menu.setAttribute("role", "menu");
  menu.setAttribute("aria-label", "Insert block");
  menu.hidden = true;
  element.append(format, canvas, menu);
  let editor;
  let bridge;
  let currentText = text;
  let slash = null;
  let menuIndex = 0;
  let filtered = BLOCKS;
  let manualMenu = false;
  let suppressedSlash = null;
  let syncing = true;

  function button(label, title, action) {
    const control = document.createElement("button");
    control.type = "button";
    control.textContent = label;
    control.title = title;
    control.setAttribute("aria-label", title);
    control.addEventListener("mousedown", (event) => event.preventDefault());
    control.addEventListener("click", action);
    format.append(control);
    return control;
  }

  const addBlock = button("+", "Insert a block", () => {
    manualMenu = !manualMenu;
    slash = null;
    menuIndex = 0;
    updateMenu();
    editor.commands.focus();
  });
  const blockSelect = document.createElement("select");
  blockSelect.setAttribute("aria-label", "Text style");
  for (const [value, label] of [["paragraph", "Text"], ["1", "Heading 1"], ["2", "Heading 2"], ["3", "Heading 3"]]) {
    const option = document.createElement("option"); option.value = value; option.textContent = label; blockSelect.append(option);
  }
  blockSelect.addEventListener("change", () => {
    const chain = editor.chain().focus();
    (blockSelect.value === "paragraph" ? chain.setParagraph() : chain.setHeading({ level: Number(blockSelect.value) })).run();
  });
  format.append(blockSelect);
  const bold = button("B", "Bold (⌘B)", () => editor.chain().focus().toggleBold().run());
  bold.className = "format-bold";
  const italic = button("I", "Italic (⌘I)", () => editor.chain().focus().toggleItalic().run());
  italic.className = "format-italic";
  const strike = button("S", "Strikethrough", () => editor.chain().focus().toggleStrike().run());
  strike.className = "format-strike";
  button("• List", "Bullet list", () => editor.chain().focus().toggleBulletList().run());
  button("Link", "Add or edit link", () => {
    linkForm.hidden = !linkForm.hidden;
    if (!linkForm.hidden) { linkInput.value = editor.getAttributes("link").href || ""; linkInput.focus(); }
  });
  const linkForm = document.createElement("form");
  linkForm.className = "document-link-form";
  linkForm.hidden = true;
  const linkInput = document.createElement("input");
  linkInput.type = "text"; linkInput.placeholder = "https://… or page.md"; linkInput.setAttribute("aria-label", "Link destination");
  const linkSubmit = document.createElement("button"); linkSubmit.type = "submit"; linkSubmit.textContent = "Apply link";
  const linkCancel = document.createElement("button"); linkCancel.type = "button"; linkCancel.textContent = "Cancel";
  linkInput.addEventListener("input", () => linkInput.setCustomValidity(""));
  linkCancel.addEventListener("click", () => { linkForm.hidden = true; editor.commands.focus(); });
  linkForm.append(linkInput, linkSubmit, linkCancel);
  format.after(linkForm);
  linkForm.addEventListener("submit", (event) => {
    event.preventDefault();
    const href = linkInput.value.trim();
    if (/^(?:javascript|data|vbscript):/i.test(href)) { linkInput.setCustomValidity("Use a web address or document path."); linkInput.reportValidity(); return; }
    linkInput.setCustomValidity("");
    const chain = editor.chain().focus().extendMarkRange("link");
    (href ? chain.setLink({ href }) : chain.unsetLink()).run();
    linkForm.hidden = true;
  });

  function selectBlock(block) {
    let chain = editor.chain().focus();
    if (slash) chain = chain.deleteRange({ from: slash.from, to: slash.to });
    slash = null; manualMenu = false; suppressedSlash = null; menu.hidden = true;
    block.run(chain).run();
  }

  function updateMenu() {
    const { $from, empty } = editor.state.selection;
    const prefix = $from.parent.textBetween(0, $from.parentOffset, "", "\ufffc");
    const match = empty && $from.parent.type.name === "paragraph" ? /^\/([^/\n]*)$/.exec(prefix) : null;
    const signature = match ? `${$from.start()}:${prefix}` : null;
    if (!match) suppressedSlash = null;
    const nextSlash = match && signature !== suppressedSlash ? { from: $from.start(), to: $from.pos, query: match[1] } : null;
    if (nextSlash?.query !== slash?.query) menuIndex = 0;
    slash = nextSlash;
    if (!slash && !manualMenu) { menu.hidden = true; return; }
    const query = (slash?.query || "").toLocaleLowerCase();
    filtered = BLOCKS.filter((block) => `${block.name} ${block.keywords}`.toLocaleLowerCase().includes(query));
    menuIndex = Math.min(menuIndex, Math.max(0, filtered.length - 1));
    menu.replaceChildren();
    const caption = document.createElement("div"); caption.className = "document-block-caption"; caption.textContent = filtered.length ? "INSERT BLOCK" : "No matching blocks"; menu.append(caption);
    filtered.forEach((block, index) => {
      const item = document.createElement("button");
      item.type = "button"; item.setAttribute("role", "menuitem");
      item.setAttribute("aria-label", block.name); item.classList.toggle("is-selected", index === menuIndex);
      const icon = document.createElement("span"); icon.className = "document-block-icon"; icon.textContent = block.icon;
      const label = document.createElement("span"); label.textContent = block.name;
      const hint = document.createElement("small"); hint.textContent = block.hint; label.append(hint);
      item.append(icon, label);
      item.addEventListener("mousedown", (event) => event.preventDefault());
      item.addEventListener("click", () => selectBlock(block));
      menu.append(item);
    });
    menu.hidden = false;
    const rect = element.getBoundingClientRect();
    const caret = manualMenu ? addBlock.getBoundingClientRect() : editor.view.coordsAtPos(editor.state.selection.from);
    menu.style.left = `${Math.max(0, Math.min(caret.left - rect.left, rect.width - 260))}px`;
    menu.style.top = `${caret.bottom - rect.top + 6}px`;
  }

  function updateSelection() {
    if (syncing || !bridge) return;
    const { from, to } = editor.state.selection;
    onSelection({ ...bridge.selectionToMarkdown(editor.state.doc, from, to), selectedText: editor.state.doc.textBetween(from, to, "\n") });
    bold.setAttribute("aria-pressed", String(editor.isActive("bold")));
    italic.setAttribute("aria-pressed", String(editor.isActive("italic")));
    strike.setAttribute("aria-pressed", String(editor.isActive("strike")));
    blockSelect.value = editor.isActive("heading") ? String(editor.getAttributes("heading").level) : "paragraph";
    updateMenu();
  }

  editor = new Editor({
    element: canvas,
    extensions: [
      StarterKit.configure({ undoRedo: false, underline: false, trailingNode: false, link: { openOnClick: false, autolink: false } }),
      Markdown.configure({ markedOptions: { gfm: true } }),
      TableKit.configure({ table: { resizable: false } }),
      TaskList, TaskItem.configure({ nested: true }), Image.extend({
        renderHTML({ HTMLAttributes }) {
          // Resolve display URLs against this panel, never the active tab URL.
          // The source attribute in the editor model remains unchanged.
          return ["img", mergeAttributes(this.options.HTMLAttributes, HTMLAttributes, { src: imageDisplayURL(HTMLAttributes.src || "", documentURL) })];
        },
      }),
      Placeholder.configure({ placeholder: "Type / for blocks, or just start writing…" }),
      ProtectedMarkdownBlock, MarkdownSourceAttributes,
    ],
    content: "",
    editorProps: {
      attributes: { class: "document-rich-content", role: "textbox", "aria-multiline": "true", "aria-label": "Document content", spellcheck: "true" },
      handleDOMEvents: {
        click(_view, event) {
          event.stopPropagation();
          const link = event.target.closest?.("a[href]");
          if (link && (event.metaKey || event.ctrlKey)) {
            event.preventDefault();
            onOpenLink?.(link.getAttribute("href"));
            return true;
          }
          return false;
        },
      },
      handleKeyDown(_view, event) {
        // Keep editor keys inside the document, including Escape in its menus.
        event.stopPropagation();
        if ((event.metaKey || event.ctrlKey) && !event.altKey) {
          if (event.key.toLowerCase() === "s") { event.preventDefault(); onSave(); return true; }
          if (event.key.toLowerCase() === "z") { event.preventDefault(); if (event.shiftKey) onRedo(); else onUndo(); return true; }
        }
        if (!menu.hidden) {
          if (event.key === "ArrowDown" || event.key === "ArrowUp") {
            event.preventDefault(); menuIndex = (menuIndex + (event.key === "ArrowDown" ? 1 : -1) + filtered.length) % Math.max(1, filtered.length); updateMenu(); return true;
          }
          if (event.key === "Enter" && filtered.length) { event.preventDefault(); selectBlock(filtered[menuIndex]); return true; }
          if (event.key === "Escape") {
            event.preventDefault(); suppressedSlash = slash ? `${slash.from}:/${slash.query}` : null; slash = null; manualMenu = false; menu.hidden = true; return true;
          }
        }
        return false;
      },
    },
    onUpdate() {
      if (syncing || !bridge) return;
      const nextText = bridge.serialize(editor.getJSON());
      const previous = currentText;
      currentText = nextText;
      try { onChange(nextText); }
      catch { setText(previous, true); return; }
      updateSelection();
    },
    onSelectionUpdate: updateSelection,
  });
  bridge = createMarkdownBridge(editor.markdown);
  editor.commands.setContent(bridge.load(text), { emitUpdate: false });
  syncing = false;

  function setText(value, force = false) {
    if (!force && value === currentText) return;
    syncing = true;
    const position = editor.state.selection.from;
    editor.commands.setContent(bridge.load(value), { emitUpdate: false });
    editor.commands.setTextSelection(Math.min(position, editor.state.doc.content.size));
    currentText = value;
    syncing = false;
    updateMenu();
  }

  return {
    setText,
    focus() { editor.commands.focus(); },
    destroy() { editor.destroy(); },
  };
}
