const copyButtons = Array.from(document.querySelectorAll(".copy-command"));
const copiedTimers = new WeakMap();
const releaseBadge = document.querySelector("[data-release-badge]");
const releaseFormatter = new Intl.DateTimeFormat(undefined, {
  dateStyle: "medium",
});

async function copyText(text) {
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
    const target = document.querySelector(copy.dataset.copyTarget);
    if (!target) return;

    const label = copy.querySelector("span");
    const defaultLabel = label.textContent;

    copy.classList.add("copied");
    label.textContent = "Copied";
    clearTimeout(copiedTimers.get(copy));
    await copyText(target.textContent);
    copiedTimers.set(
      copy,
      setTimeout(() => {
        copy.classList.remove("copied");
        label.textContent = defaultLabel;
      }, 1600),
    );
  });
}

function relativeReleaseAge(date, now = new Date()) {
  const elapsedSeconds = Math.max(0, Math.round((now.getTime() - date.getTime()) / 1000));
  const units = [
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

  try {
    const response = await fetch(releaseBadge.dataset.releaseApi, {
      headers: { Accept: "application/vnd.github+json" },
    });
    if (!response.ok) return;

    const release = await response.json();
    const tag = String(release.tag_name || "").trim();
    const publishedAt = new Date(release.published_at || release.created_at || "");
    if (!tag || Number.isNaN(publishedAt.getTime())) return;

    const version = releaseBadge.querySelector("[data-release-version]");
    const age = releaseBadge.querySelector("[data-release-age]");
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
