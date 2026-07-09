(function () {
  const root = document.documentElement;
  const storageKey = "openknowledge.viewer.theme";
  const defaultPreset = "night";
  const presets = ["default", "night", "paper", "ocean", "rose", "custom"];

  function readLocalStorage() {
    try {
      return window.localStorage.getItem(storageKey);
    } catch {
      return null;
    }
  }

  function readCookie() {
    const prefix = encodeURIComponent(storageKey) + "=";
    const parts = document.cookie ? document.cookie.split("; ") : [];
    for (const part of parts) {
      if (!part.startsWith(prefix)) {
        continue;
      }
      try {
        return decodeURIComponent(part.slice(prefix.length));
      } catch {
        return null;
      }
    }
    return null;
  }

  const sources = [readLocalStorage(), readCookie()];
  let preset = defaultPreset;
  for (const source of sources) {
    if (!source) {
      continue;
    }
    try {
      const preference = JSON.parse(source);
      if (preference && presets.includes(preference.preset)) {
        preset = preference.preset;
        break;
      }
    } catch {
      // Ignore malformed storage and preserve the first-run theme.
    }
  }
  root.dataset.viewerTheme = preset;
})();
