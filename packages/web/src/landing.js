import { initializeAnalytics } from "./analytics.js";
import { hydrateReleaseBadge } from "./release-badge.js";

const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
const copyStatus = document.querySelector("[data-copy-status]");

// ------------------------------------------------------------ install blocks
const INSTALL = {
  curl: {
    note: "macOS · Linux · SHA-256 verified",
    html: '<span class="x-exe">curl</span> <span class="x-flag">-fsSL</span> <span class="x-path">https://openknowledge.sh/install</span> <span class="x-pun">|</span> <span class="x-cmd">bash</span>',
    text: "curl -fsSL https://openknowledge.sh/install | bash",
  },
  npm: {
    note: "Node 20+ · every release platform",
    html: '<span class="x-exe">npm</span> <span class="x-cmd">install</span> <span class="x-flag">-g</span> <span class="x-path">@openknowledge-sh/openknowledge</span>',
    text: "npm install -g @openknowledge-sh/openknowledge",
  },
};
const installBlocks = [...document.querySelectorAll("[data-install]")];

function selectInstall(method, focus) {
  for (const block of installBlocks) {
    block.dataset.method = method;
    block.querySelector("[data-install-code]").innerHTML = INSTALL[method].html;
    block.querySelector("[data-install-note]").textContent = INSTALL[method].note;
    for (const tab of block.querySelectorAll("[data-install-tab]")) {
      const on = tab.dataset.installTab === method;
      tab.setAttribute("aria-selected", String(on));
      tab.tabIndex = on ? 0 : -1;
      if (on && focus === block) tab.focus();
    }
  }
}

for (const block of installBlocks) {
  const tabs = [...block.querySelectorAll("[data-install-tab]")];
  for (const tab of tabs) {
    tab.addEventListener("click", () => selectInstall(tab.dataset.installTab));
    tab.addEventListener("keydown", (event) => {
      if (event.key !== "ArrowRight" && event.key !== "ArrowLeft") return;
      event.preventDefault();
      const next = tabs[(tabs.indexOf(tab) + (event.key === "ArrowRight" ? 1 : tabs.length - 1)) % tabs.length];
      selectInstall(next.dataset.installTab, block);
    });
  }
}
selectInstall("curl");

// ------------------------------------------------------------ copy
async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}

for (const button of document.querySelectorAll("[data-copy]")) {
  const label = button.querySelector("[data-copy-label]");
  let timer;
  button.addEventListener("click", async () => {
    const block = button.closest("[data-install]");
    const text = INSTALL[block?.dataset.method || "curl"].text;
    const ok = await copyText(text);
    window.clearTimeout(timer);
    if (ok) {
      button.dataset.copied = "";
      label.textContent = "Copied";
      copyStatus.textContent = "Install command copied";
    } else {
      const code = block?.querySelector("[data-install-code]");
      if (code) window.getSelection()?.selectAllChildren(code);
      copyStatus.textContent = "Select the command and copy it manually.";
    }
    timer = window.setTimeout(() => {
      delete button.dataset.copied;
      label.textContent = "Copy";
      copyStatus.textContent = "";
    }, 2400);
  });
}

// ------------------------------------------------------------ hero terminal demo
// Real CLI output from `okn setup` and `okn search`, abbreviated.
const SCRIPT = [
  { cmd: '<span class="x-exe">okn</span> <span class="x-cmd">setup</span> <span class="x-path">Wiki</span> <span class="x-flag">--from</span> <span class="x-path">docs</span> <span class="x-flag">--mode</span> <span class="x-arg">copy</span>' },
  { out: '<span class="o-hi">Setup plan</span>', wait: 420 },
  { out: "Mode:       copy" },
  { out: "Documents:  3" },
  { out: "Changes:    9 create · 3 add minimal frontmatter · 0 delete" },
  { out: '<span class="o-soft">Knowledge base created at ./Wiki.</span>', wait: 520 },
  { out: 'Validation      <span class="o-ok">READY</span>' },
  { out: 'Search index    <span class="o-ok">READY</span> <span class="o-dim">(2 hits)</span>' },
  { out: "", wait: 900 },
  { cmd: '<span class="x-exe">okn</span> <span class="x-cmd">search</span> <span class="x-path">Wiki</span> <span class="x-str">"how long do we retry?"</span>' },
  { out: '<span class="o-hi"># Open Knowledge Context</span>', wait: 380 },
  { out: "Context: 1410 / 2400 estimated tokens" },
  { out: '<span class="o-hi">## 1. Why we retry webhooks</span>' },
  { out: 'Source:   <span class="x-path">imported/delivery.md:6-11</span>', hit: true },
  { out: '<span class="o-muted">Locator:  okf+sha256://3b09…/imported%2Fdelivery.md#78fa…</span>' },
  { out: "Retry failed deliveries for 24 hours with exponential backoff.", receipt: true },
];

const term = document.querySelector("[data-demo-term]");
const file = document.querySelector("[data-demo-file]");
const replay = document.querySelector("[data-demo-replay]");
let run = 0;
let visible = true;
let waiters = [];

const plain = (html) => {
  const node = document.createElement("span");
  node.innerHTML = html;
  return node.textContent;
};

function sleep(ms, id) {
  return new Promise((resolve) => {
    const start = performance.now();
    const step = () => {
      if (id !== run) return resolve(false);
      if (!visible) return waiters.push(step);
      if (performance.now() - start >= ms) return resolve(true);
      requestAnimationFrame(step);
    };
    step();
  });
}

function line(html, cls = "") {
  const node = document.createElement("div");
  node.className = `nx-line ${cls}`;
  node.innerHTML = html || "&nbsp;";
  term.append(node);
  while (term.children.length > 14) term.firstElementChild.remove();
  return node;
}

function setFile(hit, receipt) {
  file.classList.toggle("is-hit", hit);
  file.classList.toggle("is-receipt", receipt);
}

function renderFinal() {
  term.replaceChildren();
  for (const step of SCRIPT.slice(SCRIPT.findIndex((s) => s.cmd?.includes("search")))) {
    line(step.cmd ? `<span class="x-prompt">$</span> ${step.cmd}` : step.out, step.cmd ? "is-cmd" : "");
  }
  setFile(true, true);
}

async function play() {
  const id = ++run;
  term.replaceChildren();
  setFile(false, false);
  if (reduceMotion.matches) return renderFinal();
  for (const step of SCRIPT) {
    if (step.cmd) {
      const node = line('<span class="x-prompt">$</span> <span data-typed></span><span class="nx-caret"></span>', "is-cmd");
      const target = node.querySelector("[data-typed]");
      const full = plain(step.cmd);
      if (!(await sleep(500, id))) return;
      for (let i = 1; i <= full.length; i++) {
        target.textContent = full.slice(0, i);
        if (!(await sleep(24 + ((i * 37) % 23), id))) return;
      }
      target.innerHTML = step.cmd;
      if (!(await sleep(380, id))) return;
      node.querySelector(".nx-caret")?.remove();
      continue;
    }
    if (!(await sleep(step.wait ?? 110, id))) return;
    line(step.out, "is-out");
    if (step.hit) setFile(true, false);
    if (step.receipt) setFile(true, true);
  }
  if (!(await sleep(7000, id))) return;
  play();
}

if (term && file) {
  new IntersectionObserver(([entry]) => {
    visible = entry.isIntersecting && !document.hidden;
    if (visible) {
      const pending = waiters;
      waiters = [];
      for (const resume of pending) resume();
    }
  }).observe(term);
  replay?.addEventListener("click", () => play());
  reduceMotion.addEventListener("change", () => play());
  play();
}

// ------------------------------------------------------------ command tabs
for (const tabs of document.querySelectorAll("[data-tabs]")) {
  const list = [...tabs.querySelectorAll('[role="tab"]')];
  const select = (tab, focus) => {
    for (const other of list) {
      const on = other === tab;
      other.setAttribute("aria-selected", String(on));
      other.tabIndex = on ? 0 : -1;
      document.getElementById(other.getAttribute("aria-controls")).hidden = !on;
    }
    if (focus) tab.focus();
  };
  for (const tab of list) {
    tab.addEventListener("click", () => select(tab));
    tab.addEventListener("keydown", (event) => {
      const i = list.indexOf(tab);
      const keys = { ArrowDown: i + 1, ArrowRight: i + 1, ArrowUp: i - 1, ArrowLeft: i - 1, Home: 0, End: list.length - 1 };
      if (!(event.key in keys)) return;
      event.preventDefault();
      select(list[(keys[event.key] + list.length) % list.length], true);
    });
  }
}

// ------------------------------------------------------------ intro video
const video = document.querySelector("[data-film]");
const videoPlay = document.querySelector("[data-film-play]");
if (video) {
  // Native controls appear once playback starts, so they never collide with the poster button.
  if (!videoPlay) video.controls = true;
  videoPlay?.addEventListener("click", async () => {
    try {
      await video.play();
    } catch {
      video.controls = true;
    }
    video.focus();
  });
  video.addEventListener("play", () => {
    video.controls = true;
    if (videoPlay) videoPlay.hidden = true;
  });
  video.addEventListener("ended", () => {
    if (videoPlay) videoPlay.hidden = false;
  });
}

void hydrateReleaseBadge();
initializeAnalytics();
