export function searchStaticNotes(notes, query, limit = 12) {
  const normalizedQuery = normalizeSearchText(query);
  return notes
    .map(function (note) {
      const bodyText = htmlToText(note.body || "");
      const frontmatterText = htmlToText(note.frontmatter || "");
      const title = note.title || note.path || "";
      const path = note.path || "";
      const contentText = [frontmatterText, bodyText].filter(Boolean).join(" ");
      const haystack = normalizeSearchText([title, path, contentText].join(" "));
      const titleMatch = normalizeSearchText(title).includes(normalizedQuery);
      const pathMatch = normalizeSearchText(path).includes(normalizedQuery);
      const contentMatch = haystack.includes(normalizedQuery);
      if (!contentMatch) {
        return null;
      }
      const baseScore = (titleMatch ? 3 : 0) + (pathMatch ? 2 : 0) + 1;
      return {
        path,
        title,
        snippet: staticSnippet(contentText, query),
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
    .slice(0, limit);
}

export function normalizeSearchText(value) {
  return String(value || "").toLowerCase();
}

export function htmlToText(html) {
  const element = document.createElement("div");
  element.innerHTML = html;
  return element.textContent || "";
}

export function isIndexMarkdownPath(path) {
  return String(path || "").split("/").pop().toLowerCase() === "index.md";
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
