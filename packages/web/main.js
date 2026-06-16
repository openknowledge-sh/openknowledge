const copyButtons = Array.from(document.querySelectorAll(".copy-command"));
const copiedTimers = new WeakMap();

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

    await copyText(target.textContent);
    copy.classList.add("copied");
    label.textContent = "Copied";
    clearTimeout(copiedTimers.get(copy));
    copiedTimers.set(
      copy,
      setTimeout(() => {
        copy.classList.remove("copied");
        label.textContent = defaultLabel;
      }, 1600),
    );
  });
}
