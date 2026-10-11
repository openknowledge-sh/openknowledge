const tabs = [...document.querySelectorAll('[data-stage]')];
function selectTab(tab) {
  for (const item of tabs) {
    const active = item === tab;
    item.setAttribute('aria-selected', String(active));
    item.tabIndex = active ? 0 : -1;
    document.getElementById(item.getAttribute('aria-controls')).hidden = !active;
  }
}
for (const [index, tab] of tabs.entries()) {
  tab.addEventListener('click', () => selectTab(tab));
  tab.addEventListener('keydown', (event) => {
    let next;
    if (event.key === 'ArrowRight') next = (index + 1) % tabs.length;
    if (event.key === 'ArrowLeft') next = (index + tabs.length - 1) % tabs.length;
    if (event.key === 'Home') next = 0;
    if (event.key === 'End') next = tabs.length - 1;
    if (next === undefined) return;
    event.preventDefault();
    selectTab(tabs[next]);
    tabs[next].focus();
  });
}
const copy = document.querySelector('[data-copy]');
copy?.addEventListener('click', async () => {
  const status = document.querySelector('.copy-status');
  try {
    await navigator.clipboard.writeText(document.querySelector('[data-commands]').textContent);
    status.textContent = 'Commands copied.';
  } catch {
    status.textContent = 'Select the commands above to copy them.';
  }
});
// A lightweight particle field: scattered source paths converge into checked context.
const canvas = document.querySelector('[data-field]');
if (canvas) {
  const context = canvas.getContext('2d');
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)');
  let width = 0;
  let height = 0;
  let frame = 0;
  let visible = true;
  function draw(time = 0) {
    if (!context) return;
    context.clearRect(0, 0, width, height);
    const phase = reduced.matches ? 0 : time * 0.00014;
    for (let line = 0; line < 46; line++) {
      const lane = (line - 22.5) / 22.5;
      context.beginPath();
      for (let x = 0; x <= width; x += 5) {
        const t = x / width;
        const spread = Math.pow(Math.abs(t - 0.5) * 2, 1.65);
        const ripple = Math.sin(t * 16 + line * 0.32 + phase) * 13 * spread;
        const y = height / 2 + lane * height * 0.48 * spread + ripple;
        if (x === 0) context.moveTo(x, y); else context.lineTo(x, y);
      }
      context.strokeStyle = `rgba(${line % 3 === 0 ? '194,163,249' : '112,86,172'},${0.12 + (1 - Math.abs(lane)) * 0.24})`;
      context.lineWidth = 0.7;
      context.stroke();
      const t = ((line * 0.137 + phase * 0.12) % 1);
      const spread = Math.pow(Math.abs(t - 0.5) * 2, 1.65);
      const y = height / 2 + lane * height * 0.48 * spread + Math.sin(t * 16 + line * 0.32 + phase) * 13 * spread;
      context.fillStyle = '#c3a7f0';
      context.fillRect(t * width, y, 2, 2);
    }
  }
  function animate(time) {
    draw(time);
    frame = !reduced.matches && visible && !document.hidden ? requestAnimationFrame(animate) : 0;
  }
  function resume() {
    cancelAnimationFrame(frame);
    frame = 0;
    if (visible && !document.hidden) animate(performance.now());
  }
  new ResizeObserver(() => {
    const rect = canvas.getBoundingClientRect();
    width = rect.width;
    height = rect.height;
    const ratio = Math.min(window.devicePixelRatio || 1, 2);
    canvas.width = width * ratio;
    canvas.height = height * ratio;
    context?.setTransform(ratio, 0, 0, ratio, 0, 0);
    draw();
  }).observe(canvas);
  new IntersectionObserver(([entry]) => { visible = entry.isIntersecting; resume(); }).observe(canvas);
  reduced.addEventListener('change', resume);
  document.addEventListener('visibilitychange', resume);
}
