const consentKey = "openknowledge.analytics.consent";
const anonymousIDKey = "openknowledge.analytics.id";
const googleMeasurementID = "G-62SWM7FC2J";
export function initializeAnalytics() {
    const notice = document.querySelector("[data-analytics-consent]");
    const accept = document.querySelector("[data-analytics-accept]");
    const decline = document.querySelector("[data-analytics-decline]");
    const settings = document.querySelector("[data-analytics-settings]");
    const consent = storedValue(consentKey);
    initializeGoogleAnalytics(consent === "granted");
    settings?.addEventListener("click", () => {
        removeValue(consentKey);
        removeValue(anonymousIDKey);
        updateGoogleAnalyticsConsent("denied");
        clearGoogleAnalyticsCookies();
        if (notice)
            notice.hidden = false;
    });
    if (consent === "granted") {
        trackWebEvent("web_page_viewed");
        return;
    }
    if (consent === "denied" || !notice || !accept || !decline)
        return;
    notice.hidden = false;
    accept.addEventListener("click", () => {
        storeValue(consentKey, "granted");
        updateGoogleAnalyticsConsent("granted");
        notice.hidden = true;
        trackWebEvent("web_page_viewed");
    });
    decline.addEventListener("click", () => {
        storeValue(consentKey, "denied");
        removeValue(anonymousIDKey);
        updateGoogleAnalyticsConsent("denied");
        clearGoogleAnalyticsCookies();
        notice.hidden = true;
    });
}
export function trackWebEvent(eventName) {
    trackGoogleAnalyticsEvent(eventName);
    if (storedValue(consentKey) !== "granted")
        return;
    try {
        const event = {
            schema_version: "1",
            event_name: eventName,
            event_id: randomID(),
            occurred_at: new Date().toISOString(),
            surface: "web",
            anonymous_id: anonymousID(),
            page_group: "home",
        };
        if (eventName === "setup_prompt_copied")
            event.interaction = "setup_prompt";
        const body = JSON.stringify({ schema_version: "1", events: [event] });
        if (navigator.sendBeacon) {
            const sent = navigator.sendBeacon("/api/telemetry", new Blob([body], { type: "application/json" }));
            if (sent)
                return;
        }
        void fetch("/api/telemetry", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body,
            keepalive: true,
        }).catch(() => { });
    }
    catch {
        // Analytics never affects the page interaction.
    }
}
function initializeGoogleAnalytics(hasAnalyticsConsent) {
    const gtag = googleTag();
    gtag("consent", "default", googleConsentState("denied"));
    gtag("set", "ads_data_redaction", true);
    gtag("set", "url_passthrough", false);
    if (hasAnalyticsConsent)
        gtag("consent", "update", googleConsentState("granted"));
    gtag("js", new Date());
    gtag("config", googleMeasurementID, {
        allow_google_signals: false,
        allow_ad_personalization_signals: false,
        send_page_view: true,
    });
    if (document.querySelector(`script[data-google-analytics="${googleMeasurementID}"]`))
        return;
    const script = document.createElement("script");
    script.async = true;
    script.src = `https://www.googletagmanager.com/gtag/js?id=${googleMeasurementID}`;
    script.dataset.googleAnalytics = googleMeasurementID;
    document.head.append(script);
}
function googleConsentState(analyticsStorage) {
    return {
        ad_storage: "denied",
        ad_user_data: "denied",
        ad_personalization: "denied",
        analytics_storage: analyticsStorage,
    };
}
function updateGoogleAnalyticsConsent(analyticsStorage) {
    googleTag()("consent", "update", googleConsentState(analyticsStorage));
}
function trackGoogleAnalyticsEvent(eventName) {
    if (eventName !== "setup_prompt_copied")
        return;
    googleTag()("event", eventName, { page_group: "home" });
}
function googleTag() {
    window.dataLayer = window.dataLayer || [];
    if (typeof window.gtag !== "function") {
        window.gtag = function () {
            window.dataLayer.push(arguments);
        };
    }
    return window.gtag;
}
function clearGoogleAnalyticsCookies() {
    try {
        const domain = window.location.hostname.split(".").slice(-2).join(".");
        for (const cookie of document.cookie.split(";")) {
            const name = cookie.split("=", 1)[0].trim();
            if (name !== "_ga" && !name.startsWith("_ga_"))
                continue;
            document.cookie = `${name}=; Max-Age=0; Path=/; SameSite=Lax`;
            if (domain.includes("."))
                document.cookie = `${name}=; Max-Age=0; Path=/; Domain=.${domain}; SameSite=Lax`;
        }
    }
    catch {
        // Consent revocation remains effective when cookie cleanup is unavailable.
    }
}
function anonymousID() {
    const existing = storedValue(anonymousIDKey);
    if (existing)
        return existing;
    const created = randomID();
    storeValue(anonymousIDKey, created);
    return created;
}
function randomID() {
    if (typeof crypto.randomUUID === "function")
        return crypto.randomUUID();
    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    return Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
}
function storedValue(key) {
    try {
        return window.localStorage.getItem(key) || "";
    }
    catch {
        return "";
    }
}
function storeValue(key, value) {
    try {
        window.localStorage.setItem(key, value);
    }
    catch {
        // Consent and analytics storage are best-effort.
    }
}
function removeValue(key) {
    try {
        window.localStorage.removeItem(key);
    }
    catch {
        // Consent and analytics storage are best-effort.
    }
}
