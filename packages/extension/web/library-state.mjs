export const pageKey = page => JSON.stringify([page.knowledgeBase || '', page.path]);

export function contextForPage(page, selected = '') {
  const text = selected.trim() || page.markdown;
  return {
    title: page.title, knowledgeBase: page.knowledgeBase, path: page.path,
    source: page.sourceURI, revision: page.revision, selection: Boolean(selected.trim()),
    text: text.slice(0, 24000), truncated: text.length > 24000,
  };
}

export function parseDeepLink(value) {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) return null;
  try {
    const url = new URL(value, 'https://openknowledge.local');
    if (url.hash) return null;
    const match = url.pathname.match(/^\/bases\/([^/]+)\/?$/);
    if (!match) return null;
    return { knowledgeBase: decodeURIComponent(match[1]), path: url.searchParams.get('path') || undefined };
  } catch { return null; }
}
