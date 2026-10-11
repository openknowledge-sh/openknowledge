const copyStatuses = document.querySelectorAll("[data-copy-status]");
let statusTimer;
let copiedButton;
const copyLabels = new Map([...document.querySelectorAll("[data-copy-command]")].map((button) => [button, button.textContent]));

for (const button of document.querySelectorAll("[data-copy-command]")) {
  button.addEventListener("click", async () => {
    const command = document.getElementById(button.dataset.copyCommand);
    if (!command) return;
    const copyStatus = button.closest(".step-body")?.querySelector("[data-copy-status]");
    if (!copyStatus) return;
    clearTimeout(statusTimer);
    if (copiedButton) { copiedButton.textContent = copyLabels.get(copiedButton); delete copiedButton.dataset.copied; }
    for (const status of copyStatuses) status.textContent = "";
    try {
      await navigator.clipboard.writeText(command.textContent.trim());
      copyStatus.textContent = button.dataset.copyLabel || "Command copied";
      copiedButton = button;
      button.textContent = "Copied";
      button.dataset.copied = "true";
      statusTimer = setTimeout(() => { copyStatus.textContent = ""; button.textContent = copyLabels.get(button); delete button.dataset.copied; copiedButton = null; }, 4000);
    } catch {
      copyStatus.textContent = "Select and copy the command manually.";
    }
  });
}


const steps = [...document.querySelectorAll(".workflow-section")];
if ("IntersectionObserver" in window) {
  const visible = new Set();
  const observer = new IntersectionObserver((entries) => {
    for (const entry of entries) {
      if (entry.isIntersecting) visible.add(entry.target);
      else visible.delete(entry.target);
    }
    const current = steps.find((step) => visible.has(step));
    for (const step of steps) step.classList.toggle("is-current", step === current);
  }, { rootMargin: "-15% 0px -45% 0px" });
  for (const step of steps) observer.observe(step);
}


const film = document.querySelector(".product-film video");
const playFilm = document.querySelector("[data-play-film]");
if (film && playFilm) {
  playFilm.addEventListener("click", async () => {
    try { await film.play(); film.focus(); } catch { playFilm.hidden = true; film.focus(); }
  });
  film.addEventListener("play", () => { playFilm.hidden = true; });
  film.addEventListener("ended", () => { playFilm.hidden = false; });
}
