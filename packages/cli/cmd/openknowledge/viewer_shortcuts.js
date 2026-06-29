(function () {
  if (window.OpenKnowledgeShortcuts) {
    return;
  }

  const shortcuts = [];
  const platform = (window.navigator && (window.navigator.platform || window.navigator.userAgentData?.platform)) || "";
  const isMacPlatform = /\b(Mac|iPhone|iPad|iPod)\b/.test(platform);

  function register(shortcut) {
    if (!shortcut || !shortcut.id || typeof shortcut.run !== "function") {
      return function () {};
    }
    unregister(shortcut.id);
    shortcuts.push(Object.assign({ preventDefault: true, allowEditable: false }, shortcut));
    return function () {
      unregister(shortcut.id);
    };
  }

  function unregister(id) {
    const index = shortcuts.findIndex(function (shortcut) {
      return shortcut.id === id;
    });
    if (index >= 0) {
      shortcuts.splice(index, 1);
    }
  }

  function isEditableTarget(target) {
    return Boolean(target && target.closest && target.closest("input, textarea, select, [contenteditable='true']"));
  }

  function matchesShortcut(shortcut, event) {
    if (event.defaultPrevented || (!shortcut.allowEditable && isEditableTarget(event.target))) {
      return false;
    }
    if (shortcut.code) {
      if (event.code !== shortcut.code) {
        return false;
      }
    } else if (String(event.key || "").toLowerCase() !== String(shortcut.key || "").toLowerCase()) {
      return false;
    }
    if (!matchesPrimaryModifiers(shortcut, event)) {
      return false;
    }
    if (event.altKey !== Boolean(shortcut.altKey)) {
      return false;
    }
    if (event.shiftKey !== Boolean(shortcut.shiftKey)) {
      return false;
    }
    return typeof shortcut.when !== "function" || shortcut.when(event);
  }

  function matchesPrimaryModifiers(shortcut, event) {
    if (shortcut.primaryKey) {
      return isMacPlatform ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey;
    }
    if (shortcut.metaOrCtrlKey) {
      return event.metaKey || event.ctrlKey;
    }
    return event.metaKey === Boolean(shortcut.metaKey) && event.ctrlKey === Boolean(shortcut.ctrlKey);
  }

  function handleKeydown(event) {
    for (let index = shortcuts.length - 1; index >= 0; index--) {
      const shortcut = shortcuts[index];
      if (!matchesShortcut(shortcut, event)) {
        continue;
      }
      if (shortcut.preventDefault !== false) {
        event.preventDefault();
      }
      shortcut.run(event);
      return;
    }
  }

  function keyLabel(shortcut) {
    const raw = shortcut.key || shortcut.code || "";
    const key = String(raw).replace(/^Key/, "").replace(/^Digit/, "");
    if (key === " ") {
      return "Space";
    }
    return key.length === 1 ? key.toUpperCase() : key;
  }

  function format(shortcut) {
    if (shortcut.label) {
      return shortcut.label;
    }
    const parts = [];
    if (shortcut.primaryKey || shortcut.metaOrCtrlKey) {
      parts.push(isMacPlatform ? "⌘" : "Ctrl");
    } else {
      if (shortcut.metaKey) {
        parts.push(isMacPlatform ? "⌘" : "Meta");
      }
      if (shortcut.ctrlKey) {
        parts.push("Ctrl");
      }
    }
    if (shortcut.altKey) {
      parts.push(isMacPlatform ? "⌥" : "Alt");
    }
    if (shortcut.shiftKey) {
      parts.push(isMacPlatform ? "⇧" : "Shift");
    }
    parts.push(keyLabel(shortcut));
    return isMacPlatform ? parts.join("") : parts.join("+");
  }

  function ariaKeyShortcut(shortcut) {
    if (shortcut.ariaKeyShortcut) {
      return shortcut.ariaKeyShortcut;
    }
    const parts = [];
    if (shortcut.primaryKey) {
      parts.push(isMacPlatform ? "Meta" : "Control");
    } else if (shortcut.metaOrCtrlKey) {
      parts.push(isMacPlatform ? "Meta" : "Control");
    } else {
      if (shortcut.metaKey) {
        parts.push("Meta");
      }
      if (shortcut.ctrlKey) {
        parts.push("Control");
      }
    }
    if (shortcut.altKey) {
      parts.push("Alt");
    }
    if (shortcut.shiftKey) {
      parts.push("Shift");
    }
    parts.push(keyLabel(shortcut));
    return parts.join("+");
  }

  window.OpenKnowledgeShortcuts = {
    register: register,
    unregister: unregister,
    format: format,
    ariaKeyShortcut: ariaKeyShortcut,
    isEditableTarget: isEditableTarget,
    isMacPlatform: isMacPlatform
  };

  document.addEventListener("keydown", handleKeydown);
})();
