// Deterministic motion engine. Every visual is a pure function of time, so
// any frame can be rendered in any order and audio cues line up exactly.

export const W = 1920;
export const H = 1080;

export const clamp = (x, a = 0, b = 1) => (x < a ? a : x > b ? b : x);
export const lerp = (a, b, t) => a + (b - a) * t;
export const mix = (a, b, t) => a.map((v, i) => lerp(v, b[i], t));

export const E = {
  linear: (t) => t,
  inQuad: (t) => t * t,
  outQuad: (t) => 1 - (1 - t) * (1 - t),
  inOutQuad: (t) => (t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2),
  inCubic: (t) => t * t * t,
  outCubic: (t) => 1 - Math.pow(1 - t, 3),
  inOutCubic: (t) => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2),
  outQuart: (t) => 1 - Math.pow(1 - t, 4),
  inOutQuart: (t) => (t < 0.5 ? 8 * t * t * t * t : 1 - Math.pow(-2 * t + 2, 4) / 2),
  outQuint: (t) => 1 - Math.pow(1 - t, 5),
  inOutQuint: (t) => (t < 0.5 ? 16 * t ** 5 : 1 - Math.pow(-2 * t + 2, 5) / 2),
  inExpo: (t) => (t === 0 ? 0 : Math.pow(2, 10 * t - 10)),
  outExpo: (t) => (t === 1 ? 1 : 1 - Math.pow(2, -10 * t)),
  inOutExpo: (t) =>
    t === 0 ? 0 : t === 1 ? 1 : t < 0.5 ? Math.pow(2, 20 * t - 10) / 2 : (2 - Math.pow(2, -20 * t + 10)) / 2,
  outBack: (t) => {
    const c1 = 1.70158, c3 = c1 + 1;
    return 1 + c3 * Math.pow(t - 1, 3) + c1 * Math.pow(t - 1, 2);
  },
  outBackSoft: (t) => {
    const c1 = 0.9, c3 = c1 + 1;
    return 1 + c3 * Math.pow(t - 1, 3) + c1 * Math.pow(t - 1, 2);
  },
  outElastic: (t) =>
    t === 0 ? 0 : t === 1 ? 1 : Math.pow(2, -10 * t) * Math.sin((t * 10 - 0.75) * ((2 * Math.PI) / 3)) + 1,
};

// Eased progress of t through [a, a + d].
export const P = (t, a, d, e = E.outCubic) => e(clamp((t - a) / d));
// Fade in over [a, a + din], fade out over [b, b + dout].
export const win = (t, a, din, b, dout, ei = E.outCubic, eo = E.inCubic) =>
  Math.min(P(t, a, din, ei), 1 - P(t, b, dout, eo));

// Integer hash -> [0, 1). Deterministic replacement for Math.random.
export function hash(a, b = 0) {
  let h = Math.imul((a | 0) ^ 0x9e3779b9, 0x85ebca6b) ^ Math.imul((b | 0) + 0x632be5ab, 0xc2b2ae35);
  h ^= h >>> 15;
  h = Math.imul(h, 0x2c1b3c6d);
  h ^= h >>> 12;
  h = Math.imul(h, 0x297a2d39);
  h ^= h >>> 15;
  return (h >>> 0) / 4294967296;
}

export function el(tag, cls, parent, html) {
  const node = document.createElement(tag);
  if (cls) node.className = cls;
  if (html != null) node.innerHTML = html;
  if (parent) parent.appendChild(node);
  return node;
}

export const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

const DEF = { x: 0, y: 0, z: 0, s: 1, r: 0, rx: 0, ry: 0, o: 1, blur: 0 };

// Apply a transform/opacity/blur state to an element.
export function tf(node, p = {}) {
  const q = { ...DEF, ...p };
  const sx = p.sx ?? q.s, sy = p.sy ?? q.s;
  let t = `translate3d(${q.x.toFixed(2)}px,${q.y.toFixed(2)}px,${q.z.toFixed(2)}px)`;
  if (q.rx) t += ` rotateX(${q.rx.toFixed(3)}deg)`;
  if (q.ry) t += ` rotateY(${q.ry.toFixed(3)}deg)`;
  if (q.r) t += ` rotate(${q.r.toFixed(3)}deg)`;
  if (sx !== 1 || sy !== 1) t += ` scale(${sx.toFixed(4)},${sy.toFixed(4)})`;
  node.style.transform = t;
  node.style.opacity = clamp(q.o).toFixed(4);
  node.style.filter = q.blur > 0.15 ? `blur(${q.blur.toFixed(2)}px)` : 'none';
  node.style.visibility = q.o <= 0.001 ? 'hidden' : 'visible';
}

// Keyframes: [[time, {props}, ease?], ...]. Props carry forward between keys.
export function kf(t, frames) {
  let cur = { ...DEF, ...frames[0][1] };
  if (t <= frames[0][0]) return cur;
  for (let i = 1; i < frames.length; i++) {
    const [ft, fp, fe] = frames[i];
    const target = { ...cur, ...fp };
    const pt = frames[i - 1][0];
    if (t < ft) {
      const ease = typeof fe === 'function' ? fe : E[fe || 'inOutCubic'];
      const k = ease(clamp((t - pt) / (ft - pt)));
      const out = {};
      for (const key in target) out[key] = lerp(cur[key] ?? target[key], target[key], k);
      return out;
    }
    cur = target;
  }
  return cur;
}

// ---------------------------------------------------------------- audio cues
export const cues = [];
export const cue = (type, t, opts = {}) => cues.push({ type, t: Math.round(t * 10000) / 10000, ...opts });

// ---------------------------------------------------------------- typing
// Humanized deterministic typing schedule. Registers a key cue per char.
export function typing(text, start, { cps = 26, jitter = 0.35, seed = 1, sound = true, vol = 1 } = {}) {
  const times = [];
  let t = start;
  for (let i = 0; i < text.length; i++) {
    times.push(t);
    const ch = text[i];
    if (sound) cue('key', t, { v: vol * (ch === ' ' ? 0.6 : 1), s: hash(seed, i), space: ch === ' ' ? 1 : 0 });
    const j = 1 + (hash(seed + 7, i) - 0.5) * 2 * jitter;
    t += (1 / cps) * j * (ch === ' ' ? 1.5 : 1);
  }
  return {
    text,
    times,
    start,
    end: t,
    count(tt) {
      let n = 0;
      while (n < times.length && times[n] <= tt) n++;
      return n;
    },
  };
}

// Segments: [[cls, text], ...] -> HTML of the first n characters.
export function segHTML(segs, n) {
  let out = '';
  let left = n;
  for (const [cls, txt] of segs) {
    if (left <= 0) break;
    const part = txt.slice(0, left);
    left -= part.length;
    out += cls ? `<span class="${cls}">${esc(part)}</span>` : esc(part);
  }
  return out;
}
export const segText = (segs) => segs.map((s) => s[1]).join('');

// ---------------------------------------------------------------- scramble
const GLYPHS = '#%&*+=<>/\\[]{}-_~:;01ABCDEFabcdef';
// Decode text left-to-right. Unrevealed chars keep layout but stay invisible.
export function scramble(text, p, t, { seed = 0, spread = 7 } = {}) {
  const n = text.length;
  const head = p * (n + spread);
  let out = '';
  for (let i = 0; i < n; i++) {
    const c = text[i];
    if (c === ' ') {
      out += ' ';
      continue;
    }
    if (i < head - spread) out += esc(c);
    else if (i < head) {
      const g = GLYPHS[Math.floor(hash(i + seed * 131, Math.floor(t * 30)) * GLYPHS.length)];
      out += `<span class="scr">${esc(g)}</span>`;
    } else out += `<span class="ghost">${esc(c)}</span>`;
  }
  return out;
}

// ---------------------------------------------------------------- headlines
// lines: [[cls, text], ...] per line -> spans per char, for staggered reveals.
export function headline(parent, cls, lines) {
  const root = el('div', `headline ${cls || ''}`, parent);
  const chars = [];
  for (const line of lines) {
    const ln = el('div', 'hl-line', root);
    for (const [c, text] of line) {
      const wrap = el('span', c || '', ln);
      for (const word of text.split(/( )/)) {
        if (word === ' ') {
          wrap.appendChild(document.createTextNode(' '));
          continue;
        }
        if (!word) continue;
        const w = el('span', 'hl-word', wrap);
        for (const ch of word) {
          const s = el('span', 'hl-ch', w);
          s.textContent = ch;
          chars.push(s);
        }
      }
    }
  }
  return { root, chars };
}

// Staggered character rise with blur. Returns nothing; mutates spans.
export function riseChars(chars, t, start, { stagger = 0.018, dur = 0.55, dy = 0.42, blur = 10, out = null } = {}) {
  for (let i = 0; i < chars.length; i++) {
    const p = P(t, start + i * stagger, dur, E.outQuart);
    let o = p;
    let y = (1 - p) * dy;
    let b = (1 - p) * blur;
    if (out) {
      const q = P(t, out.start + i * (out.stagger ?? 0.008), out.dur ?? 0.35, E.inCubic);
      o *= 1 - q;
      y -= q * (out.dy ?? 0.3);
      b += q * (out.blur ?? 8);
    }
    const s = chars[i].style;
    s.opacity = o.toFixed(3);
    s.transform = `translateY(${y.toFixed(3)}em)`;
    s.filter = b > 0.2 ? `blur(${b.toFixed(1)}px)` : 'none';
  }
}

// ---------------------------------------------------------------- svg helpers
export const SVGNS = 'http://www.w3.org/2000/svg';
export function svg(tag, attrs = {}, parent) {
  const node = document.createElementNS(SVGNS, tag);
  for (const k in attrs) node.setAttribute(k, attrs[k]);
  if (parent) parent.appendChild(node);
  return node;
}
// Draw a path from 0..1 using dash offset. Call after the path is in the DOM.
export function drawPath(path, p, len) {
  const L = len ?? (path._len ??= path.getTotalLength());
  path.style.strokeDasharray = `${L} ${L}`;
  path.style.strokeDashoffset = (L * (1 - clamp(p))).toFixed(2);
}

export const fmt = (n, w = 3) => String(Math.floor(n)).padStart(w, '0');

export function addStyle(text) {
  const s = document.createElement('style');
  s.textContent = text;
  document.head.appendChild(s);
}

// Cursor blink that stays solid while typing.
export const blink = (t, lastKey = -1) => (t - lastKey < 0.5 ? 1 : Math.floor((t - lastKey) / 0.53) % 2 === 0 ? 1 : 0);
