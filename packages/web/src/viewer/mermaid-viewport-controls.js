function button(text, label, action) {
  const element = document.createElement("button");
  element.type = "button";
  element.textContent = text;
  element.dataset.okMermaidAction = action;
  element.setAttribute("aria-label", label);
  element.setAttribute("title", label);
  return element;
}

export function createMermaidViewportControls() {
  const dialog = document.createElement("dialog");
  dialog.className = "ok-mermaid-viewport";

  const toolbar = document.createElement("div");
  toolbar.className = "ok-mermaid-viewport-toolbar";
  toolbar.setAttribute("role", "toolbar");
  toolbar.setAttribute("aria-label", "Diagram zoom controls");

  const zoomOut = button("−", "Zoom out", "zoom-out");
  const zoomIn = button("+", "Zoom in", "zoom-in");
  const actual = button("100%", "Show diagram at 100%", "actual");
  const fitButton = button("Fit", "Fit diagram to viewport", "fit");
  const closeButton = button("Close", "Close diagram viewer", "close");
  const zoom = document.createElement("output");
  zoom.className = "ok-mermaid-viewport-zoom";
  zoom.dataset.okMermaidZoom = "";
  zoom.setAttribute("aria-label", "Diagram zoom");
  zoom.setAttribute("aria-live", "polite");
  toolbar.append(zoomOut, zoomIn, actual, fitButton, zoom, closeButton);

  const canvas = document.createElement("div");
  canvas.className = "ok-mermaid-viewport-canvas";
  canvas.dataset.okMermaidCanvas = "";
  canvas.tabIndex = 0;
  canvas.setAttribute("aria-label", "Diagram canvas. Drag to pan.");

  const stage = document.createElement("div");
  stage.className = "ok-mermaid-viewport-stage";
  canvas.append(stage);
  dialog.append(toolbar, canvas);
  document.body.append(dialog);

  return { actual, canvas, closeButton, dialog, fitButton, stage, zoom, zoomIn, zoomOut };
}
