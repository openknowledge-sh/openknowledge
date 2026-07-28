const copyButtons = Array.from(document.querySelectorAll<HTMLButtonElement>(".copy-command"));
const copiedTimers = new WeakMap<HTMLButtonElement, ReturnType<typeof setTimeout>>();
const releaseBadge = document.querySelector<HTMLElement>("[data-release-badge]");
const releaseFormatter = new Intl.DateTimeFormat(undefined, {
  dateStyle: "medium",
});

async function copyText(text: string) {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Fall back below when Clipboard API is blocked by browser permissions.
    }
  }

  const field = document.createElement("textarea");
  field.value = text;
  field.setAttribute("readonly", "");
  field.style.position = "fixed";
  field.style.opacity = "0";
  document.body.append(field);
  field.select();
  document.execCommand("copy");
  field.remove();
}

for (const copy of copyButtons) {
  copy.addEventListener("click", async () => {
    const selector = copy.dataset.copyTarget;
    if (!selector) return;
    const target = document.querySelector<HTMLElement>(selector);
    if (!target) return;

    const label = copy.querySelector<HTMLSpanElement>("span");
    if (!label) return;
    const defaultLabel = label.textContent;

    copy.classList.add("copied");
    label.textContent = "Copied";
    clearTimeout(copiedTimers.get(copy));
    await copyText(target.textContent || "");
    copiedTimers.set(
      copy,
      setTimeout(() => {
        copy.classList.remove("copied");
        label.textContent = defaultLabel;
      }, 1600),
    );
  });
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
    if (value >= 1) {
      return `${value} ${unit}${value === 1 ? "" : "s"} ago`;
    }
  }

  return "just now";
}

async function hydrateReleaseBadge() {
  if (!releaseBadge) return;
  const releaseAPI = releaseBadge.dataset.releaseApi;
  if (!releaseAPI) return;

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
    releaseBadge.setAttribute("aria-label", `Latest Open Knowledge release ${tag}, published ${age.textContent}`);
  } catch {
    // Keep the static releases link when GitHub is unavailable or rate-limited.
  }
}

hydrateReleaseBadge();
