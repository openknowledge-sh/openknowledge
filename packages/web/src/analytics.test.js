import { afterEach, beforeEach, describe, expect, test } from "vitest";
import { initializeAnalytics, trackWebEvent } from "./analytics.js";

describe("Google Advanced Consent Mode", () => {
  let controls;
  let requests;

  beforeEach(() => {
    controls = Object.fromEntries([
      "[data-analytics-consent]",
      "[data-analytics-accept]",
      "[data-analytics-decline]",
      "[data-analytics-settings]",
    ].map((selector) => [selector, fakeControl()]));
    requests = [];
    const storage = new Map();
    const scripts = [];
    const cookies = new Map();
    const document = {
      head: { append: (script) => scripts.push(script) },
      querySelector(selector) {
        if (selector.startsWith("script[data-google-analytics=")) {
          return scripts.find((script) => script.dataset.googleAnalytics === "G-62SWM7FC2J") || null;
        }
        return controls[selector] || null;
      },
      createElement: () => ({ dataset: {} }),
    };
    Object.defineProperty(document, "cookie", {
      configurable: true,
      get: () => Array.from(cookies, ([name, value]) => `${name}=${value}`).join("; "),
      set: (value) => {
        const [pair, ...attributes] = value.split(";");
        const separator = pair.indexOf("=");
        const name = pair.slice(0, separator).trim();
        const content = pair.slice(separator + 1);
        if (attributes.some((attribute) => attribute.trim().toLowerCase() === "max-age=0")) cookies.delete(name);
        else cookies.set(name, content);
      },
    });
    Object.defineProperties(globalThis, {
      document: { configurable: true, value: document },
      navigator: {
        configurable: true,
        value: { sendBeacon: (url, body) => requests.push({ url, body }) > 0 },
      },
      window: {
        configurable: true,
        value: {
          location: { hostname: "openknowledge.sh" },
          localStorage: {
            getItem: (key) => storage.get(key) || null,
            setItem: (key, value) => storage.set(key, value),
            removeItem: (key) => storage.delete(key),
          },
        },
      },
    });
  });

  afterEach(() => {
    delete globalThis.document;
    delete globalThis.navigator;
    delete globalThis.window;
  });

  test("queues denied defaults before configuration and grants only analytics storage", () => {
    initializeAnalytics();

    const commands = googleCommands();
    const defaultIndex = commands.findIndex(([command, action]) => command === "consent" && action === "default");
    const configIndex = commands.findIndex(([command]) => command === "config");
    expect(defaultIndex).toBeGreaterThanOrEqual(0);
    expect(defaultIndex).toBeLessThan(configIndex);
    expect(commands[defaultIndex][2]).toEqual({
      ad_storage: "denied",
      ad_user_data: "denied",
      ad_personalization: "denied",
      analytics_storage: "denied",
    });
    expect(requests).toHaveLength(0);

    controls["[data-analytics-accept]"].click();
    const granted = googleCommands().findLast(([command, action]) => command === "consent" && action === "update");
    expect(granted[2]).toEqual({
      ad_storage: "denied",
      ad_user_data: "denied",
      ad_personalization: "denied",
      analytics_storage: "granted",
    });
    expect(window.localStorage.getItem("openknowledge.analytics.consent")).toBe("granted");
    expect(window.localStorage.getItem("openknowledge.analytics.id")).toBeTruthy();
    expect(requests).toHaveLength(1);

    trackWebEvent("setup_prompt_copied");
    expect(googleCommands()).toContainEqual(["event", "setup_prompt_copied", { page_group: "home" }]);
    expect(requests).toHaveLength(2);
  });

  test("keeps storage denied and removes analytics state when cookies are refused", () => {
    initializeAnalytics();
    document.cookie = "_ga=test-client";
    document.cookie = "_ga_62SWM7FC2J=test-session";

    controls["[data-analytics-decline]"].click();
    const denied = googleCommands().findLast(([command, action]) => command === "consent" && action === "update");
    expect(denied[2].analytics_storage).toBe("denied");
    expect(window.localStorage.getItem("openknowledge.analytics.consent")).toBe("denied");
    expect(window.localStorage.getItem("openknowledge.analytics.id")).toBeNull();
    expect(document.cookie).not.toContain("_ga");
    expect(requests).toHaveLength(0);
  });
});

function fakeControl() {
  const listeners = new Map();
  return {
    hidden: true,
    addEventListener: (name, listener) => listeners.set(name, listener),
    click: () => listeners.get("click")?.(),
  };
}

function googleCommands() {
  return window.dataLayer.map((entry) => Array.from(entry));
}
