const consentKey = "openknowledge.analytics.consent";
const anonymousIDKey = "openknowledge.analytics.id";

type WebEventName = "web_page_viewed" | "setup_prompt_copied";

export function initializeAnalytics() {
  const notice = document.querySelector<HTMLElement>("[data-analytics-consent]");
  const accept = document.querySelector<HTMLButtonElement>("[data-analytics-accept]");
  const decline = document.querySelector<HTMLButtonElement>("[data-analytics-decline]");
  const settings = document.querySelector<HTMLButtonElement>("[data-analytics-settings]");
  const consent = storedValue(consentKey);

  settings?.addEventListener("click", () => {
    removeValue(consentKey);
    removeValue(anonymousIDKey);
    if (notice) notice.hidden = false;
  });

  if (consent === "granted") {
    trackWebEvent("web_page_viewed");
    return;
  }
  if (consent === "denied" || !notice || !accept || !decline) return;

  notice.hidden = false;
  accept.addEventListener("click", () => {
    storeValue(consentKey, "granted");
    notice.hidden = true;
    trackWebEvent("web_page_viewed");
  });
  decline.addEventListener("click", () => {
    storeValue(consentKey, "denied");
    removeValue(anonymousIDKey);
    notice.hidden = true;
  });
}

export function trackWebEvent(eventName: WebEventName) {
  if (storedValue(consentKey) !== "granted") return;
  try {
    const event: Record<string, string> = {
      schema_version: "1",
      event_name: eventName,
      event_id: randomID(),
      occurred_at: new Date().toISOString(),
      surface: "web",
      anonymous_id: anonymousID(),
      page_group: "home",
    };
    if (eventName === "setup_prompt_copied") event.interaction = "setup_prompt";
    const body = JSON.stringify({ schema_version: "1", events: [event] });
    if (navigator.sendBeacon) {
      const sent = navigator.sendBeacon("/api/telemetry", new Blob([body], { type: "application/json" }));
      if (sent) return;
    }
    void fetch("/api/telemetry", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
      keepalive: true,
    }).catch(() => {});
  } catch {
    // Analytics never affects the page interaction.
  }
}

function anonymousID() {
  const existing = storedValue(anonymousIDKey);
  if (existing) return existing;
  const created = randomID();
  storeValue(anonymousIDKey, created);
  return created;
}

function randomID() {
  if (typeof crypto.randomUUID === "function") return crypto.randomUUID();
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
}

function storedValue(key: string) {
  try {
    return window.localStorage.getItem(key) || "";
  } catch {
    return "";
  }
}

function storeValue(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // Consent and analytics storage are best-effort.
  }
}

function removeValue(key: string) {
  try {
    window.localStorage.removeItem(key);
  } catch {
    // Consent and analytics storage are best-effort.
  }
}
