const releaseFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });
export function relativeReleaseAge(date, now = new Date()) {
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
        if (value >= 1)
            return `${value} ${unit}${value === 1 ? "" : "s"} ago`;
    }
    return "just now";
}
export async function hydrateReleaseBadge(releaseBadge = document.querySelector("[data-release-badge]")) {
    const releaseAPI = releaseBadge?.dataset.releaseApi;
    if (!releaseBadge || !releaseAPI)
        return;
    try {
        const response = await fetch(releaseAPI, {
            headers: { Accept: "application/vnd.github+json" },
        });
        if (!response.ok)
            return;
        const release = (await response.json());
        const tag = String(release.tag_name || "").trim();
        const publishedAt = new Date(release.published_at || release.created_at || "");
        if (!tag || Number.isNaN(publishedAt.getTime()))
            return;
        const version = releaseBadge.querySelector("[data-release-version]");
        const age = releaseBadge.querySelector("[data-release-age]");
        if (!version || !age)
            return;
        version.textContent = tag;
        age.textContent = relativeReleaseAge(publishedAt);
        age.dateTime = publishedAt.toISOString();
        age.title = releaseFormatter.format(publishedAt);
        age.hidden = false;
        releaseBadge.setAttribute("aria-label", `Latest Open Knowledge release ${tag}, published ${age.textContent}`);
    }
    catch {
        // Keep the static releases link when GitHub is unavailable or rate-limited.
    }
}
