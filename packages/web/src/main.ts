const copyButton = document.querySelector<HTMLButtonElement>("[data-copy-setup]");
const copyStatus = document.querySelector<HTMLElement>("[data-copy-status]");
const promptTemplate = document.querySelector<HTMLTemplateElement>("#setup-prompt");
const releaseBadge = document.querySelector<HTMLElement>("[data-release-badge]");
const releaseFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });
let copyResetTimer: number | undefined;

async function copyText(text: string) {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Fall back when browser permissions block the Clipboard API.
    }
  }

  const field = document.createElement("textarea");
  field.value = text;
  field.setAttribute("readonly", "");
  field.style.position = "fixed";
  field.style.opacity = "0";

  try {
    document.body.append(field);
    field.select();
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    field.remove();
  }
}

function relativeReleaseAge(date: Date, now = new Date()) {
  const elapsedSeconds = Math.max(0, Math.round((now.getTime() - date.getTime()) / 1000));
  const units: ReadonlyArray<readonly [string, number]> = [
    ["year", 60 * 60 * 24 * 365],
    ["month", 60 * 60 * 24 * 30],
    ["day", 60 * 60 * 24],
    ["hour", 60 * 60],
    ["minute", 60],
  ];

  for (const [unit, seconds] of units) {
    const value = Math.floor(elapsedSeconds / seconds);
    if (value >= 1) return `${value} ${unit}${value === 1 ? "" : "s"} ago`;
  }

  return "just now";
}

async function hydrateReleaseBadge() {
  const releaseAPI = releaseBadge?.dataset.releaseApi;
  if (!releaseBadge || !releaseAPI) return;

  try {
    const response = await fetch(releaseAPI, {
      headers: { Accept: "application/vnd.github+json" },
    });
    if (!response.ok) return;

    const release = (await response.json()) as {
      tag_name?: string;
      published_at?: string;
      created_at?: string;
    };
    const tag = String(release.tag_name || "").trim();
    const publishedAt = new Date(release.published_at || release.created_at || "");
    if (!tag || Number.isNaN(publishedAt.getTime())) return;

    const version = releaseBadge.querySelector<HTMLElement>("[data-release-version]");
    const age = releaseBadge.querySelector<HTMLTimeElement>("[data-release-age]");
    if (!version || !age) return;

    version.textContent = tag;
    age.textContent = relativeReleaseAge(publishedAt);
    age.dateTime = publishedAt.toISOString();
    age.title = releaseFormatter.format(publishedAt);
    age.hidden = false;
    releaseBadge.setAttribute(
      "aria-label",
      `Latest Open Knowledge release ${tag}, published ${age.textContent}`,
    );
  } catch {
    // Keep the static releases link when GitHub is unavailable or rate-limited.
  }
}

void hydrateReleaseBadge();

if (copyButton && copyStatus && promptTemplate) {
  copyButton.addEventListener("click", async () => {
    const prompt = promptTemplate.content.textContent?.trim() ?? "";
    if (!prompt) return;

    window.clearTimeout(copyResetTimer);
    copyButton.disabled = true;
    copyButton.setAttribute("aria-busy", "true");
    const copied = await copyText(prompt);
    copyButton.disabled = false;
    copyButton.removeAttribute("aria-busy");

    if (copied) {
      copyButton.textContent = "Copied";
      copyButton.dataset.state = "copied";
      copyStatus.textContent = "Paste the prompt into your agent to begin.";
      copyResetTimer = window.setTimeout(() => {
        copyButton.textContent = "Copy setup prompt";
        delete copyButton.dataset.state;
      }, 2400);
      return;
    }

    copyStatus.textContent = "Copy failed. Open the documentation for the setup steps.";
  });
}
