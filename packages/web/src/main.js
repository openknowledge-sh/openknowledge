import { initializeAnalytics, trackWebEvent } from "./analytics.js";
import "./site-components.js";
const copyButton = document.querySelector("[data-copy-setup]");
const copyStatus = document.querySelector("[data-copy-status]");
const installCopyButton = document.querySelector("[data-copy-install]");
const installCommand = document.querySelector("[data-install-command]");
const installStatus = document.querySelector("[data-install-status]");
const guideCopyButtons = document.querySelectorAll("[data-copy-command]");
const promptTemplate = document.querySelector("#setup-prompt");
const releaseBadge = document.querySelector("[data-release-badge]");
const releaseFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });
const copyStatusReadyText = copyStatus?.textContent?.trim() ?? "";
let copyResetTimer;
let installCopyResetTimer;
const guideCopyResetTimers = new WeakMap();
async function copyText(text) {
    if (navigator.clipboard?.writeText) {
        try {
            await navigator.clipboard.writeText(text);
            return true;
        }
        catch {
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
    }
    catch {
        return false;
    }
    finally {
        field.remove();
    }
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
        if (value >= 1)
            return `${value} ${unit}${value === 1 ? "" : "s"} ago`;
    }
    return "just now";
}
async function hydrateReleaseBadge() {
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
void hydrateReleaseBadge();
initializeAnalytics();
if (copyButton && copyStatus && promptTemplate) {
    copyButton.addEventListener("click", async () => {
        const prompt = promptTemplate.content.textContent?.trim() ?? "";
        if (!prompt)
            return;
        window.clearTimeout(copyResetTimer);
        copyButton.disabled = true;
        copyButton.setAttribute("aria-busy", "true");
        const copied = await copyText(prompt);
        copyButton.disabled = false;
        copyButton.removeAttribute("aria-busy");
        if (copied) {
            trackWebEvent("setup_prompt_copied");
            copyButton.textContent = "Copied";
            copyButton.dataset.state = "copied";
            copyStatus.textContent = "Paste the prompt into your agent to begin.";
            copyResetTimer = window.setTimeout(() => {
                copyButton.textContent = "Copy setup prompt";
                delete copyButton.dataset.state;
                copyStatus.textContent = copyStatusReadyText;
            }, 2400);
            return;
        }
        copyStatus.textContent = "Copy failed. Open the documentation for the setup steps.";
    });
}
if (installCopyButton && installCommand && installStatus) {
    installCopyButton.addEventListener("click", async () => {
        const command = installCommand.textContent?.trim() ?? "";
        if (!command)
            return;
        window.clearTimeout(installCopyResetTimer);
        installCopyButton.disabled = true;
        installCopyButton.setAttribute("aria-busy", "true");
        const copied = await copyText(command);
        installCopyButton.disabled = false;
        installCopyButton.removeAttribute("aria-busy");
        if (copied) {
            installCopyButton.dataset.state = "copied";
            installCopyButton.setAttribute("aria-label", "Install command copied");
            installStatus.textContent = "Copied. Paste it into your terminal.";
            installCopyResetTimer = window.setTimeout(() => {
                delete installCopyButton.dataset.state;
                installCopyButton.setAttribute("aria-label", "Copy install command");
                installStatus.textContent = "Copy and run in your terminal.";
            }, 2400);
            return;
        }
        installStatus.textContent = "Copy failed. Select the command and copy it manually.";
    });
}
for (const button of guideCopyButtons) {
    const command = button.querySelector("[data-command]");
    const status = button.querySelector("[data-command-status]");
    const readyLabel = button.getAttribute("aria-label") ?? "Copy command";
    if (!command || !status)
        continue;
    button.addEventListener("click", async () => {
        const commandText = command.textContent?.trim() ?? "";
        if (!commandText)
            return;
        const resetTimer = guideCopyResetTimers.get(button);
        if (resetTimer)
            window.clearTimeout(resetTimer);
        button.disabled = true;
        button.setAttribute("aria-busy", "true");
        const copied = await copyText(commandText);
        button.disabled = false;
        button.removeAttribute("aria-busy");
        if (copied) {
            button.dataset.state = "copied";
            button.dataset.feedback = "copied";
            button.setAttribute("aria-label", `${readyLabel.replace(/^Copy /, "")} copied`);
            status.textContent = "Copied";
            guideCopyResetTimers.set(button, window.setTimeout(() => {
                delete button.dataset.feedback;
                button.setAttribute("aria-label", `${readyLabel} again`);
                status.textContent = "Copy again";
            }, 2400));
            return;
        }
        status.textContent = "Select and copy";
    });
}
function coverPlacement(containerWidth, containerHeight, image) {
    const scale = Math.max(containerWidth / image.naturalWidth, containerHeight / image.naturalHeight);
    return {
        height: image.naturalHeight * scale,
        scale,
        width: image.naturalWidth * scale,
        x: (containerWidth - image.naturalWidth * scale) / 2,
        y: (containerHeight - image.naturalHeight * scale) / 2,
    };
}
function solveLinearSystem(rows) {
    const size = rows.length;
    for (let column = 0; column < size; column += 1) {
        let pivot = column;
        for (let row = column + 1; row < size; row += 1) {
            if (Math.abs(rows[row][column]) > Math.abs(rows[pivot][column]))
                pivot = row;
        }
        if (Math.abs(rows[pivot][column]) < 1e-10)
            return null;
        [rows[column], rows[pivot]] = [rows[pivot], rows[column]];
        const divisor = rows[column][column];
        for (let value = column; value <= size; value += 1)
            rows[column][value] /= divisor;
        for (let row = 0; row < size; row += 1) {
            if (row === column)
                continue;
            const factor = rows[row][column];
            for (let value = column; value <= size; value += 1) {
                rows[row][value] -= factor * rows[column][value];
            }
        }
    }
    return rows.map((row) => row[size]);
}
function projectRectangle(width, height, corners) {
    const source = [
        { x: 0, y: 0 },
        { x: width, y: 0 },
        { x: width, y: height },
        { x: 0, y: height },
    ];
    const rows = [];
    for (let index = 0; index < source.length; index += 1) {
        const { x, y } = source[index];
        const { x: targetX, y: targetY } = corners[index];
        rows.push([x, 0, y, 0, 1, 0, -targetX * x, -targetX * y, targetX]);
        rows.push([0, x, 0, y, 0, 1, -targetY * x, -targetY * y, targetY]);
    }
    const values = solveLinearSystem(rows);
    if (!values)
        return "none";
    const [a, b, c, d, e, f, g, h] = values;
    return `matrix3d(${a},${b},0,${g},${c},${d},0,${h},0,0,1,0,${e},${f},0,1)`;
}
function positionTerminal() {
    const stage = document.querySelector("[data-hero-stage]");
    const image = document.querySelector("[data-hero-image]");
    const terminal = document.querySelector("[data-hero-terminal]");
    const powerLight = document.querySelector("[data-hero-power-light]");
    if (!stage || !image || !terminal || !image.naturalWidth || window.innerWidth <= 640)
        return;
    const rect = stage.getBoundingClientRect();
    const placement = coverPlacement(rect.width, rect.height, image);
    const screenCorners = [
        { x: 1137, y: 478 },
        { x: 1373, y: 478 },
        { x: 1376, y: 681 },
        { x: 1137, y: 681 },
    ].map(({ x, y }) => ({
        x: placement.x + x * placement.scale,
        y: placement.y + y * placement.scale,
    }));
    terminal.style.transform = projectRectangle(230, 188, screenCorners);
    if (powerLight) {
        const diameter = Math.max(4, 5 * placement.scale);
        powerLight.style.left = `${placement.x + 1361 * placement.scale - diameter / 2}px`;
        powerLight.style.top = `${placement.y + 713 * placement.scale - diameter / 2 - 4}px`;
        powerLight.style.width = `${diameter}px`;
        powerLight.style.height = `${diameter}px`;
    }
}
function initializeHeroPowerLight() {
    const stage = document.querySelector("[data-hero-stage]");
    const powerLight = document.querySelector("[data-hero-power-light]");
    if (!stage || !powerLight)
        return;
    let isVisible = false;
    const updateAnimation = () => {
        powerLight.classList.toggle("is-active", isVisible && document.visibilityState === "visible");
    };
    const observer = new IntersectionObserver(([entry]) => {
        isVisible = entry.isIntersecting;
        updateAnimation();
    });
    observer.observe(stage);
    document.addEventListener("visibilitychange", updateAnimation);
}
async function initializeHeroCanvas() {
    const stage = document.querySelector("[data-hero-stage]");
    const canvas = document.querySelector("[data-hero-canvas]");
    const image = document.querySelector("[data-hero-image]");
    const content = document.querySelector("[data-hero-content]");
    const terminal = document.querySelector("[data-hero-terminal]");
    if (!stage || !canvas || !image || !content || !terminal)
        return;
    try {
        await image.decode();
    }
    catch {
        if (!image.complete)
            return;
    }
    positionTerminal();
    const terminalObserver = new ResizeObserver(positionTerminal);
    terminalObserver.observe(stage);
    const context = canvas.getContext("2d");
    if (!context || typeof context.drawElementImage !== "function" || window.innerWidth <= 640) {
        return;
    }
    stage.dataset.renderer = "html-canvas";
    canvas.append(content);
    const paint = () => {
        const rect = stage.getBoundingClientRect();
        const pixelRatio = Math.max(1, window.devicePixelRatio || 1);
        const pixelWidth = Math.round(rect.width * pixelRatio);
        const pixelHeight = Math.round(rect.height * pixelRatio);
        if (canvas.width !== pixelWidth)
            canvas.width = pixelWidth;
        if (canvas.height !== pixelHeight)
            canvas.height = pixelHeight;
        context.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);
        context.clearRect(0, 0, rect.width, rect.height);
        const placement = coverPlacement(rect.width, rect.height, image);
        context.drawImage(image, placement.x, placement.y, placement.width, placement.height);
        const bottomFade = context.createLinearGradient(0, rect.height * 0.67, 0, rect.height);
        bottomFade.addColorStop(0, "rgba(5, 7, 11, 0)");
        bottomFade.addColorStop(0.58, "rgba(5, 7, 11, 0.14)");
        bottomFade.addColorStop(1, "#05070b");
        context.fillStyle = bottomFade;
        context.fillRect(0, 0, rect.width, rect.height);
        const contentX = Math.max(32, (rect.width - 1120) / 2);
        const contentY = Math.max(168, Math.min(210, rect.height * 0.23));
        const contentTransform = context.drawElementImage?.(content, contentX, contentY);
        if (contentTransform)
            content.style.transform = contentTransform.toString();
    };
    canvas.addEventListener("paint", paint);
    const resizeObserver = new ResizeObserver(() => {
        positionTerminal();
        canvas.requestPaint?.();
    });
    resizeObserver.observe(stage);
    canvas.requestPaint?.();
}
void initializeHeroCanvas();
initializeHeroPowerLight();
