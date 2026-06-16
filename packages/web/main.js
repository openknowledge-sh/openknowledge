const command = document.querySelector("#install-command");
const copy = document.querySelector(".copy-command");
const tabs = Array.from(document.querySelectorAll(".tab"));
let copiedTimer;

for (const tab of tabs) {
  tab.addEventListener("click", () => {
    for (const current of tabs) {
      current.classList.toggle("active", current === tab);
    }
    command.textContent = tab.dataset.command;
  });
}

async function copyText(text) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
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

copy.addEventListener("click", async () => {
  await copyText(command.textContent);
  copy.classList.add("copied");
  copy.querySelector("span").textContent = "Copied";
  clearTimeout(copiedTimer);
  copiedTimer = setTimeout(() => {
    copy.classList.remove("copied");
    copy.querySelector("span").textContent = "Copy";
  }, 1600);
});
