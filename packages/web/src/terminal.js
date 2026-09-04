const journey = document.querySelector("[data-journey]");
const progress = document.querySelector("[data-journey-progress]");
const sections = Array.from(document.querySelectorAll("[data-journey-section]"));

function updateJourneyProgress() {
  if (!journey || !progress) return;
  const rect = journey.getBoundingClientRect();
  const viewportAnchor = window.innerHeight * 0.42;
  const traveled = Math.max(0, viewportAnchor - rect.top);
  const available = Math.max(1, rect.height - viewportAnchor);
  const ratio = Math.min(1, traveled / available);
  progress.style.transform = `scaleY(${ratio})`;
}

const sectionObserver = new IntersectionObserver(
  (entries) => {
    for (const entry of entries) {
      entry.target.classList.toggle("is-visible", entry.isIntersecting);
    }
  },
  { rootMargin: "-18% 0px -62% 0px", threshold: 0 },
);

for (const section of sections) sectionObserver.observe(section);

let animationFrame = 0;
function requestProgressUpdate() {
  if (animationFrame) return;
  animationFrame = window.requestAnimationFrame(() => {
    animationFrame = 0;
    updateJourneyProgress();
  });
}

window.addEventListener("scroll", requestProgressUpdate, { passive: true });
window.addEventListener("resize", requestProgressUpdate);
updateJourneyProgress();
