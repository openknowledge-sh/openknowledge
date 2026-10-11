import { el, tf, kf, P, E, clamp, scramble, cues, cue } from './engine.js';
import { T, DURATION, FPS, CHAPTERS } from './timeline.js';
import problem from './scenes/problem.js';
import reveal from './scenes/reveal.js';
import features from './scenes/features.js';
import finale from './scenes/finale.js';

const params = new URLSearchParams(location.search);
const RENDER = params.has('render');
if (RENDER) document.body.classList.add('render');

const stage = document.getElementById('stage');

// ------------------------------------------------------------ background
const bg = el('div', '', stage);
bg.id = 'bg';
const glowWarm = el('div', 'glow warm', bg);
const glowBlue = el('div', 'glow', bg);
const grid = el('div', 'grid', bg);
el('div', 'vignette', bg);

function updateBackground(t) {
  const g = kf(t, [
    [0, { o: 0 }],
    [16.05, { o: 0 }],
    [17.6, { o: 0.55 }, 'outCubic'],
    [21, { o: 0.4 }],
    [78, { o: 0.4 }],
    [82.5, { o: 0.65 }],
    [86, { o: 0.25 }],
    [96, { o: 0.25 }],
  ]);
  grid.style.opacity = g.o.toFixed(3);
  const drift = t * 6;
  grid.style.transform = `translate3d(${(drift % 48).toFixed(2)}px, ${((drift * 0.5) % 48).toFixed(2)}px, 0)`;

  const w = kf(t, [
    [0, { o: 0, x: 0, y: 0 }],
    [2, { o: 0.7 }],
    [11, { o: 1, x: -120, y: 80 }],
    [15.9, { o: 1, x: 0, y: 0 }],
    [16.1, { o: 0 }],
  ]);
  tf(glowWarm, { x: w.x, y: w.y, o: w.o });

  const b = kf(t, [
    [0, { o: 0, x: 0, y: 0, s: 0.6 }],
    [16, { o: 0, s: 0.6 }],
    [16.4, { o: 1, s: 1.15 }, 'outExpo'],
    [22, { o: 0.8, s: 1, x: -380, y: 120 }],
    [32, { o: 0.75, x: 380, y: 160 }],
    [42, { o: 0.85, x: -260, y: 60 }],
    [50, { o: 0.8, x: 0, y: 140 }],
    [60, { o: 0.7, x: -420, y: 180 }],
    [70, { o: 0.8, x: 320, y: 100 }],
    [78, { o: 0.9, x: 0, y: 40 }],
    [86, { o: 1, x: 0, y: -60, s: 1.1 }],
    [94.5, { o: 1, s: 1 }],
    [96, { o: 0 }],
  ]);
  tf(glowBlue, b);
}

// ------------------------------------------------------------ scenes
const scenes = [];
const ctx = {
  stage,
  add(def) {
    const root = el('div', `scene scene-${def.id}`, stage);
    const [start, end] = def.window ?? T[def.id];
    const s = { ...def, root, start, end, pre: def.pre ?? 0, post: def.post ?? 0 };
    s.at = (lt) => start + lt;
    s.cue = (type, lt, opts) => cue(type, start + lt, opts);
    def.build(root, s);
    scenes.push(s);
    return s;
  },
};

problem(ctx);
reveal(ctx);
features(ctx);
finale(ctx);

// ------------------------------------------------------------ HUD
const hud = el('div', '', stage);
hud.id = 'hud';
const hudLabel = el('div', 'hud-label', hud);
const hudMark = el('div', 'hud-mark', hud, '<img src="assets/logo-mark.png" alt=""><span>Open Knowledge</span>');
const rail = el('div', 'rail', hud);
const railItems = CHAPTERS.map(([id, num, name]) => {
  const item = el('div', 'rail-item', rail, `${num}  ${name}`);
  const fill = el('div', 'fill', item);
  return { id, num, name, item, fill };
});

function updateHUD(t) {
  const m = kf(t, [
    [21.75, { o: 0 }],
    [21.95, { o: 1 }, 'linear'],
    [85.4, { o: 1 }],
    [85.8, { o: 0 }],
  ]);
  tf(hudMark, m);

  const r = kf(t, [
    [22.2, { o: 0, y: 14 }],
    [22.9, { o: 1, y: 0 }, 'outCubic'],
    [77.4, { o: 1, y: 0 }],
    [78, { o: 0, y: 14 }],
  ]);
  tf(rail, r);

  let active = null;
  for (const ch of railItems) {
    const [a, b] = T[ch.id];
    const p = clamp((t - a) / (b - a));
    ch.fill.style.width = `calc(${(p * 100).toFixed(2)}% - ${(p * 14).toFixed(2)}px)`;
    ch.item.className = `rail-item${t >= a && t < b ? ' on' : t >= b ? ' done' : ''}`;
    if (t >= a && t < b) active = { ...ch, since: t - a, len: b - a };
  }
  if (active) {
    const p = P(active.since, 0.15, 0.6, E.linear);
    const out = P(active.since, active.len - 0.45, 0.3, E.linear);
    hudLabel.innerHTML = `<b>${active.num}</b>  /  ${scramble(active.name, p, active.since, { seed: +active.num })}`;
    hudLabel.style.opacity = (P(active.since, 0, 0.2, E.linear) * (1 - out)).toFixed(3);
  } else {
    hudLabel.innerHTML = '';
  }
}

// ------------------------------------------------------------ seek
// Slow camera push-in on the feature scenes.
const DRIFT = new Set(['create', 'find', 'connect', 'update', 'check', 'ship']);
let current = -1;
function seek(t) {
  current = t;
  updateBackground(t);
  for (const s of scenes) {
    const on = t >= s.start - s.pre && t < s.end + s.post;
    if (on !== s._on) {
      s.root.style.display = on ? 'block' : 'none';
      s._on = on;
    }
    if (on) {
      s.update(t - s.start, s);
      if (DRIFT.has(s.id)) {
        const k = clamp((t - s.start) / (s.end - s.start));
        s.root.style.transform = `scale(${(1 + 0.025 * E.inOutQuad(k)).toFixed(5)})`;
      }
    }
  }
  updateHUD(t);
}

// ------------------------------------------------------------ boot
async function ready() {
  await Promise.all([document.fonts.load('500 24px Plex'), document.fonts.load('600 24px Manrope'), document.fonts.load('750 24px Manrope')]);
  await document.fonts.ready;
  await Promise.all([...document.images].map((img) => (img.decode ? img.decode().catch(() => {}) : null)));
  cues.sort((a, b) => a.t - b.t);
  seek(0);
}
const readyPromise = ready();

window.__film = { duration: DURATION, fps: FPS, ready: readyPromise, seek, cues };

if (!RENDER) {
  const fit = () => {
    const s = Math.min(innerWidth / 1920, innerHeight / 1080);
    stage.style.transform = `scale(${s})`;
  };
  addEventListener('resize', fit);
  fit();

  const controls = document.getElementById('controls');
  const btn = el('button', '', controls, 'Play');
  const range = el('input', '', controls);
  range.type = 'range';
  range.min = 0;
  range.max = DURATION;
  range.step = 1 / FPS;
  const clock = el('span', '', controls, '0.00');
  const audio = new Audio('out/soundtrack.wav');
  audio.preload = 'auto';
  let playing = false;
  let t0 = 0;
  let base = 0;

  const show = (t) => {
    seek(t);
    range.value = t;
    clock.textContent = t.toFixed(2);
  };
  const tick = (now) => {
    if (!playing) return;
    const t = audio.readyState >= 2 && !audio.paused ? audio.currentTime : base + (now - t0) / 1000;
    if (t >= DURATION) {
      playing = false;
      btn.textContent = 'Play';
      audio.pause();
      show(DURATION - 1 / FPS);
      return;
    }
    show(t);
    requestAnimationFrame(tick);
  };
  const play = () => {
    playing = !playing;
    btn.textContent = playing ? 'Pause' : 'Play';
    if (playing) {
      base = current >= DURATION - 0.1 ? 0 : Math.max(0, current);
      t0 = performance.now();
      audio.currentTime = base;
      audio.play().catch(() => {});
      requestAnimationFrame(tick);
    } else audio.pause();
  };
  btn.onclick = play;
  range.oninput = () => {
    if (playing) play();
    show(+range.value);
  };
  addEventListener('keydown', (e) => {
    if (e.code === 'Space') {
      e.preventDefault();
      play();
    }
  });
  const start = +(params.get('t') || 0);
  readyPromise.then(() => show(start));
}
