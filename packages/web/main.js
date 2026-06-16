const command = document.querySelector("#install-command");
const tabs = Array.from(document.querySelectorAll(".tab"));

for (const tab of tabs) {
  tab.addEventListener("click", () => {
    for (const current of tabs) {
      current.classList.toggle("active", current === tab);
    }
    command.textContent = tab.dataset.command;
  });
}
