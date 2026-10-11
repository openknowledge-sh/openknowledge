import { el, tf, kf, P, E, typing, segHTML, segText, headline, riseChars, blink, addStyle } from '../engine.js';

addStyle(`
.feat-hl { position: absolute; left: 120px; top: 116px; }
.feat-sub { position: absolute; left: 120px; top: 218px; white-space: pre; }
`);

export function place(node, x, y, w, h) {
  node.style.position = 'absolute';
  node.style.left = `${x}px`;
  node.style.top = `${y}px`;
  if (w != null) node.style.width = `${w}px`;
  if (h != null) node.style.height = `${h}px`;
  return node;
}

// Offset of node relative to an ancestor, ignoring transforms.
export function offsetIn(node, ancestor) {
  let x = 0, y = 0, n = node;
  while (n && n !== ancestor) {
    x += n.offsetLeft;
    y += n.offsetTop;
    n = n.offsetParent;
  }
  return { x, y, w: node.offsetWidth, h: node.offsetHeight };
}

// A terminal whose command types itself and whose output prints line by line.
export function term(parent, s, { x, y, w, h, title = '~/relay', size = 22 }) {
  const root = place(el('div', 'term', parent), x, y, w, h);
  el('div', 'term-bar', root, `<i></i><i></i><i></i><span>${title}</span>`);
  const body = el('div', 'term-body', root);
  body.style.fontSize = `${size}px`;
  const items = [];
  const T = {
    root,
    body,
    cmd(segs, at, { cps = 30, seed = Math.round(at * 100) + 3, vol = 1 } = {}) {
      const line = el('div', 'term-line', body);
      const ty = typing(segText(segs), s.start + at, { cps, seed, vol });
      items.push({ kind: 'cmd', line, segs, ty, at });
      return line;
    },
    out(html, at, { cue, cueOpts } = {}) {
      const line = el('div', 'term-line', body, html || '&nbsp;');
      items.push({ kind: 'out', line, at });
      if (cue) s.cue(cue, at, cueOpts);
      return line;
    },
    update(t) {
      const abs = t + s.start;
      for (let i = 0; i < items.length; i++) {
        const it = items[i];
        if (it.kind === 'cmd') {
          const n = it.ty.count(abs);
          const next = items.slice(i + 1).find((x) => x.kind === 'out');
          const nextAt = next ? next.at : Infinity;
          const visible = t >= it.at - 0.5;
          const typingNow = abs < it.ty.end;
          const last = n ? it.ty.times[n - 1] - s.start : -1;
          const showCur = visible && t < nextAt && (typingNow || blink(t, last));
          it.line.innerHTML = `<span class="x-prompt">$ </span>${segHTML(it.segs, n)}${showCur ? '<span class="cursor"></span>' : ''}`;
          it.line.style.opacity = visible ? 1 : 0;
        } else {
          const p = P(t, it.at, 0.16, E.outCubic);
          it.line.style.opacity = p.toFixed(3);
          it.line.style.transform = `translateY(${((1 - p) * 6).toFixed(2)}px)`;
        }
      }
    },
  };
  return T;
}

// Feature headline + subline with standard in/out timing.
export function titleBlock(root, parts, subText, { inAt = 0.12, outAt }) {
  const hl = headline(root, 'h-feature feat-hl', [parts]);
  const sub = el('div', 'sub feat-sub', root, subText);
  return {
    update(t) {
      riseChars(hl.chars, t, inAt, { stagger: 0.013, dur: 0.55, out: { start: outAt, stagger: 0.004, dur: 0.3 } });
      tf(sub, kf(t, [
        [inAt + 0.45, { o: 0, y: 12 }],
        [inAt + 0.95, { o: 1, y: 0 }, 'outCubic'],
        [outAt + 0.05, { o: 1, y: 0 }],
        [outAt + 0.4, { o: 0, y: -10 }],
      ]));
    },
  };
}

// Standard panel entrance/exit used across feature scenes.
export function panelIn(t, a, b, { dx = 0, dy = 40, exitDx = -60 } = {}) {
  return kf(t, [
    [a, { o: 0, x: dx, y: dy, blur: 8 }],
    [a + 0.6, { o: 1, x: 0, y: 0, blur: 0 }, 'outQuart'],
    [b, { o: 1, x: 0, y: 0, blur: 0 }],
    [b + 0.5, { o: 0, x: exitDx, y: 0, blur: 8 }, 'inCubic'],
  ]);
}

export const S = (cls, text) => [cls, text];
