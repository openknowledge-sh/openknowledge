import { normalizeEditorURL } from './editor-link.mjs';

// The UI resource owns the frame origin declared in its CSP. Tool calls can
// come from a different MCP connection with a different local viewer port.
export function createResourceViewer(bootstrap) {
  const viewer = normalizeEditorURL(bootstrap.viewerURL);
  const panel = normalizeEditorURL(bootstrap.panelURL);
  if (viewer.search || viewer.hash || panel.search || panel.hash || !viewer.pathname.endsWith('/') || !panel.pathname.endsWith(viewer.pathname)) {
    throw new Error('The viewer resource has an invalid starting address.');
  }
  const prefix = panel.pathname.slice(0, -viewer.pathname.length);
  if (!prefix || prefix.includes('//')) throw new Error('The viewer resource has an invalid protected address.');
  function projectURL(value, protectedFrame = false) {
    const source = normalizeEditorURL(value);
    if (!source.pathname.startsWith(viewer.pathname)) throw new Error('The requested page is outside this viewer workspace.');
    const target = new URL(protectedFrame ? panel.origin : viewer.origin);
    target.pathname = (protectedFrame ? prefix : '') + source.pathname;
    target.search = source.search;
    target.hash = source.hash;
    return target.href;
  }
  return {
    home: { viewerURL: viewer.href, panelURL: panel.href },
    frameDomains: [panel.origin],
    project(destination) {
      const projected = { ...destination, viewerURL: projectURL(destination.viewerURL), panelURL: projectURL(destination.viewerURL, true) };
      if (destination.editorURL) {
        projected.editorURL = projectURL(destination.editorURL);
        projected.panelEditorURL = projectURL(destination.editorURL, true);
      }
      return projected;
    },
  };
}
