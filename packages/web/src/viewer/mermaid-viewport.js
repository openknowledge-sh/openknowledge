import { createMermaidViewportControls } from "./mermaid-viewport-controls.js";

const minimumScale = 0.01;
const maximumScale = 8;
let controls;

export function bindMermaidViewport(trigger, label) {
  if ("okMermaidViewportBound" in trigger.dataset) {
    return;
  }
  trigger.dataset.okMermaidViewportBound = "";
  trigger.tabIndex = 0;
  trigger.setAttribute("role", "button");
  trigger.setAttribute("aria-label", label);
  trigger.setAttribute("aria-haspopup", "dialog");
  trigger.setAttribute("aria-keyshortcuts", "Enter Space");
  trigger.setAttribute("title", "Open diagram viewer");
  const open = () => (controls ||= createViewport()).open(trigger, label);
  trigger.addEventListener("click", (event) => {
    if (!event.target?.closest?.("a")) {
      open();
    }
  });
  trigger.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " " || event.code === "Space") {
      event.preventDefault();
      open();
    }
  });
}

export function closeMermaidViewport() {
  controls?.close();
}

function createViewport() {
  const { actual, canvas, closeButton, dialog, fitButton, stage, zoom, zoomIn, zoomOut } =
    createMermaidViewportControls();
  const state = {
    fit: true,
    height: 1,
    original: null,
    pinch: null,
    pointers: new Map(),
    scale: 1,
    trigger: null,
    width: 1,
    x: 0,
    y: 0,
  };

  function update() {
    stage.style.width = state.width + "px";
    stage.style.height = state.height + "px";
    stage.style.transform =
      "translate3d(" + rounded(state.x) + "px, " + rounded(state.y) + "px, 0) scale(" + rounded(state.scale) + ")";
    zoom.textContent = Math.round(state.scale * 100) + "%";
  }

  function fit() {
    const bounds = canvas.getBoundingClientRect();
    const availableWidth = Math.max(1, bounds.width - 48);
    const availableHeight = Math.max(1, bounds.height - 48);
    state.scale = clamp(Math.min(availableWidth / state.width, availableHeight / state.height));
    state.x = (bounds.width - state.width * state.scale) / 2;
    state.y = (bounds.height - state.height * state.scale) / 2;
    state.fit = true;
    update();
  }

  function actualSize() {
    const bounds = canvas.getBoundingClientRect();
    state.scale = 1;
    state.x = (bounds.width - state.width) / 2;
    state.y = (bounds.height - state.height) / 2;
    state.fit = false;
    update();
  }

  function zoomAt(multiplier, clientX, clientY) {
    const nextScale = clamp(state.scale * multiplier);
    const bounds = canvas.getBoundingClientRect();
    const pointX = clientX === undefined ? bounds.width / 2 : clientX - bounds.left;
    const pointY = clientY === undefined ? bounds.height / 2 : clientY - bounds.top;
    const ratio = nextScale / state.scale;
    state.x = pointX - (pointX - state.x) * ratio;
    state.y = pointY - (pointY - state.y) * ratio;
    state.scale = nextScale;
    state.fit = false;
    update();
  }

  function restore() {
    if (!state.original) {
      return;
    }
    state.original.parent.insertBefore(state.original.svg, state.original.next);
    state.original = null;
    state.pinch = null;
    state.pointers.clear();
    delete canvas.dataset.okMermaidPanning;
    const trigger = state.trigger;
    state.trigger = null;
    if (trigger && trigger.isConnected !== false) {
      trigger.focus();
    }
  }

  function close() {
    if (dialog.open) {
      dialog.close();
    }
  }

  function open(trigger, label) {
    const svg = trigger.querySelector("svg");
    if (!svg) {
      return;
    }
    close();
    const size = diagramSize(svg);
    state.width = size.width;
    state.height = size.height;
    state.trigger = trigger;
    state.original = { svg, parent: svg.parentNode, next: svg.nextSibling };
    dialog.setAttribute("aria-label", label);
    stage.append(svg);
    dialog.showModal();
    fit();
    canvas.focus();
  }

  zoomOut.addEventListener("click", () => zoomAt(0.8));
  zoomIn.addEventListener("click", () => zoomAt(1.25));
  actual.addEventListener("click", actualSize);
  fitButton.addEventListener("click", fit);
  closeButton.addEventListener("click", close);
  dialog.addEventListener("close", restore);
  dialog.addEventListener("cancel", (event) => {
    event.preventDefault();
    close();
  });
  dialog.addEventListener("keydown", (event) => {
    const key = event.key.toLowerCase();
    if (key === "+" || key === "=") {
      zoomAt(1.25);
    } else if (key === "-") {
      zoomAt(0.8);
    } else if (key === "0") {
      actualSize();
    } else if (key === "f") {
      fit();
    } else if (key === "escape") {
      close();
    } else if (key.startsWith("arrow")) {
      state.x += key === "arrowleft" ? -40 : key === "arrowright" ? 40 : 0;
      state.y += key === "arrowup" ? -40 : key === "arrowdown" ? 40 : 0;
      state.fit = false;
      update();
    } else {
      return;
    }
    event.preventDefault();
  });
  canvas.addEventListener("wheel", (event) => {
    event.preventDefault();
    zoomAt(event.deltaY < 0 ? 1.1 : 1 / 1.1, event.clientX, event.clientY);
  }, { passive: false });
  canvas.addEventListener("pointerdown", (event) => {
    if (event.button !== 0) {
      return;
    }
    state.pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
    state.pinch = pinchSnapshot(state.pointers);
    state.fit = false;
    canvas.dataset.okMermaidPanning = "";
    canvas.setPointerCapture(event.pointerId);
  });
  canvas.addEventListener("pointermove", (event) => {
    const previous = state.pointers.get(event.pointerId);
    if (!previous) {
      return;
    }
    state.pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
    if (state.pointers.size > 1) {
      const nextPinch = pinchSnapshot(state.pointers);
      if (state.pinch && nextPinch && state.pinch.distance > 0) {
        zoomAt(nextPinch.distance / state.pinch.distance, state.pinch.x, state.pinch.y);
        state.x += nextPinch.x - state.pinch.x;
        state.y += nextPinch.y - state.pinch.y;
      }
      state.pinch = nextPinch;
    } else {
      state.x += event.clientX - previous.x;
      state.y += event.clientY - previous.y;
    }
    update();
  });
  function endPan(event) {
    if (!state.pointers.has(event.pointerId)) {
      return;
    }
    state.pointers.delete(event.pointerId);
    state.pinch = pinchSnapshot(state.pointers);
    if (!state.pointers.size) {
      delete canvas.dataset.okMermaidPanning;
    }
    if (!canvas.hasPointerCapture || canvas.hasPointerCapture(event.pointerId)) {
      canvas.releasePointerCapture(event.pointerId);
    }
  }
  canvas.addEventListener("pointerup", endPan);
  canvas.addEventListener("pointercancel", endPan);
  window.addEventListener("resize", () => {
    if (dialog.open && state.fit) {
      fit();
    }
  });

  return { close, dialog, open };
}

function diagramSize(svg) {
  const viewBox = svg.viewBox?.baseVal;
  const bounds = svg.getBoundingClientRect();
  return {
    width: positive(viewBox?.width) || positive(svg.width?.baseVal?.value) || positive(bounds.width) || 1,
    height: positive(viewBox?.height) || positive(svg.height?.baseVal?.value) || positive(bounds.height) || 1,
  };
}

function positive(value) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? number : 0;
}

function clamp(value) {
  return Math.min(maximumScale, Math.max(minimumScale, value));
}

function rounded(value) {
  return Math.round(value * 1000) / 1000;
}

function pinchSnapshot(pointers) {
  if (pointers.size < 2) {
    return null;
  }
  const [first, second] = Array.from(pointers.values());
  return {
    distance: Math.hypot(second.x - first.x, second.y - first.y),
    x: (first.x + second.x) / 2,
    y: (first.y + second.y) / 2,
  };
}
