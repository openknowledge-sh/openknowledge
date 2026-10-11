import { el, tf, kf, P, E, clamp, lerp, typing, segHTML, segText, headline, riseChars, svg, drawPath, esc, addStyle, blink } from '../engine.js';
import { place } from './kit.js';

addStyle(`
.loop-hl { position: absolute; left: 0; right: 0; top: 150px; text-align: center; }
.lnode { position: absolute; left: 0; top: 0; }
.lnode-in { position: relative; left: -50%; top: -50%; padding: 16px 22px; border-radius: 8px; background: #080b10; border: 1px solid #2c3d57;
  font: 500 24px/1 var(--mono); color: var(--text); white-space: pre; }
.lnode-in .k { display: block; font-size: 13px; letter-spacing: .16em; text-transform: uppercase; color: var(--quiet); margin-bottom: 9px; }
.loop-core { position: absolute; left: 960px; top: 610px; width: 150px; height: 150px; margin: -75px 0 0 -75px; }
.comet { position: absolute; left: 0; top: 0; width: 16px; height: 16px; margin: -8px 0 0 -8px; border-radius: 50%; background: #fff;
  box-shadow: 0 0 16px 6px rgba(154,194,255,.95), 0 0 50px 16px rgba(40,120,255,.6); }
.value { position: absolute; left: 0; right: 0; top: 452px; text-align: center; font: 500 150px/1 var(--mono); letter-spacing: -0.055em; white-space: pre; }
.value small { display: block; margin-top: 34px; font-size: 22px; letter-spacing: .18em; text-transform: uppercase; color: var(--quiet); }
.whiteout { position: absolute; inset: 0; background: radial-gradient(circle at 50% 50%, #fff, #9ac2ff 45%, #2878ff 100%); }

.cta-logo { position: absolute; left: 960px; top: 236px; width: 170px; height: 170px; margin: -85px 0 0 -85px; }
.cta-glow { position: absolute; left: 960px; top: 236px; width: 700px; height: 700px; margin: -350px 0 0 -350px; border-radius: 50%;
  background: radial-gradient(circle, rgba(40,120,255,.35), rgba(40,120,255,.08) 40%, transparent 65%); }
.cta-hl { position: absolute; left: 0; right: 0; top: 360px; text-align: center; }
.cta-hl .headline { font-size: 100px; }
.install { position: absolute; left: 350px; top: 640px; width: 1220px; border: 1px solid #3a3a3a; border-radius: 6px; background: #090b0e; overflow: hidden; }
.install-row { display: flex; justify-content: space-between; align-items: center; padding: 22px 28px; font: 500 28px/1.2 var(--mono); white-space: pre; }
.install-row + .install-row { border-top: 1px solid #262626; }
.install-row .copy { font-size: 16px; color: var(--quiet); border: 1px solid #3a3a3a; border-radius: 5px; padding: 8px 12px; }
.cta-row { position: absolute; left: 0; right: 0; top: 860px; display: flex; justify-content: center; align-items: center; gap: 34px; }
.cta-btn { display: inline-flex; align-items: center; gap: 40px; padding: 22px 28px; background: #66a0ff; border-radius: 5px; color: #07162d; font: 500 30px/1 var(--mono); white-space: pre; }
.cta-note { font: 500 20px/1 var(--mono); letter-spacing: .16em; text-transform: uppercase; color: var(--quiet); white-space: pre; }
.blackout { position: absolute; inset: 0; background: #000; }
`);

export default function finale(ctx) {
  loop(ctx);
  cta(ctx);
}

// ------------------------------------------------------------ 78–86 s
function loop(ctx) {
  const NODES = [
    { k: 'source', n: 'markdown + git', x: 560, y: 420 },
    { k: 'check', n: 'deterministic checks', x: 960, y: 420 },
    { k: 'review', n: 'evidence-backed PR', x: 1360, y: 420 },
    { k: 'publish', n: 'production MCP', x: 1260, y: 800 },
    { k: 'learn', n: 'usage insights', x: 660, y: 800 },
  ];
  const VALUES = [
    ['Plain Markdown.', ''],
    ['Your Git repo.', 'acc'],
    ['Open format.', ''],
    ['Any agent.', 'acc'],
    ['No lock-in.', ''],
    ['Apache-2.0.', 'acc'],
  ];
  const VAL_AT = [4.5, 5.0, 5.5, 6.0, 6.5, 7.0];
  let hl, wires, path, glowPath, nodes = [], core, comet, value, whiteout, marks = [];

  ctx.add({
    id: 'loop',
    post: 0.05,
    build(root, s) {
      hl = headline(root, 'loop-hl h-feature', [[['', 'One lifecycle. '], ['acc', 'No migrations.']]]);
      wires = svg('svg', { class: 'wire', width: 1920, height: 1080 }, root);
      const d = 'M 480 420 L 1440 420 Q 1500 420 1500 480 L 1500 740 Q 1500 800 1440 800 L 480 800 Q 420 800 420 740 L 420 480 Q 420 420 480 420';
      glowPath = svg('path', { d, fill: 'none', stroke: 'rgba(102,160,255,.25)', 'stroke-width': 10, 'stroke-linecap': 'round' }, wires);
      path = svg('path', { d, fill: 'none', stroke: '#66a0ff', 'stroke-width': 2.5, 'stroke-linecap': 'round' }, wires);
      // Direction chevrons along the path.
      for (const [x, y, r] of [[760, 420, 0], [1160, 420, 0], [1500, 610, 90], [960, 800, 180], [420, 610, 270]]) {
        marks.push(svg('path', { d: 'M -7 -8 L 5 0 L -7 8', fill: 'none', stroke: '#9ac2ff', 'stroke-width': 2.5, transform: `translate(${x} ${y}) rotate(${r})` }, wires));
      }
      core = el('img', 'loop-core', root);
      core.src = 'assets/logo-mark.png';
      NODES.forEach((nd, i) => {
        const node = el('div', 'lnode', root);
        el('div', 'lnode-in', node, `<span class="k">${nd.k}</span>${esc(nd.n)}`);
        nodes.push({ ...nd, node, i });
        s.cue('pop', 0.55 + i * 0.16, { v: 0.45, p: i });
      });
      comet = el('div', 'comet', root);
      value = el('div', 'value', root);
      whiteout = el('div', 'whiteout', root);
      s.cue('beam', 1.35, { d: 1.4 });
      s.cue('beam', 2.75, { d: 1.4 });
      VAL_AT.forEach((at, i) => s.cue('stab', at, { i }));
      s.cue('riser', 6.2, { d: 1.8, v: 0.8 });
    },
    update(t) {
      riseChars(hl.chars, t, 0.1, { stagger: 0.016, out: { start: 4.15, stagger: 0.004, dur: 0.3 } });
      const out = P(t, 4.15, 0.4, E.inCubic);
      const dp = P(t, 0.25, 1.1, E.inOutCubic);
      drawPath(path, dp);
      drawPath(glowPath, dp);
      wires.style.opacity = (1 - out).toFixed(3);
      wires.style.transform = `scale(${lerp(1, 0.92, out)})`;
      wires.style.transformOrigin = '960px 610px';
      marks.forEach((m, i) => (m.style.opacity = P(t, 0.9 + i * 0.12, 0.3).toFixed(3)));

      const L = path._len ?? path.getTotalLength();
      const lap = 1.45;
      const cp = (t - 1.35) / lap;
      const startLen = 80 / L;
      let cx = 0, cy = 0;
      if (cp >= 0 && t < 4.2) {
        const pt = path.getPointAtLength(((startLen + cp) % 1) * L);
        cx = pt.x;
        cy = pt.y;
        tf(comet, { x: cx, y: cy, o: Math.min(1, cp * 4) * (1 - P(t, 3.9, 0.3, E.linear)) });
      } else tf(comet, { o: 0 });

      for (const nd of nodes) {
        const a = kf(t, [[0.55 + nd.i * 0.16, { o: 0, s: 0.6 }], [0.9 + nd.i * 0.16, { o: 1, s: 1 }, 'outBack']]);
        const near = cp >= 0 && t < 4.2 ? Math.max(0, 1 - Math.hypot(cx - nd.x, cy - nd.y) / 160) : 0;
        tf(nd.node, { x: lerp(nd.x, 960, out * 0.15), y: lerp(nd.y, 610, out * 0.15), o: a.o * (1 - out), s: a.s * (1 + near * 0.08) });
        const inner = nd.node.firstChild;
        inner.style.borderColor = near > 0.05 ? `rgba(154,194,255,${0.3 + near * 0.7})` : '#2c3d57';
        inner.style.boxShadow = near > 0.05 ? `0 0 ${(40 * near).toFixed(0)}px rgba(102,160,255,${(0.55 * near).toFixed(2)})` : 'none';
      }
      tf(core, { o: P(t, 1.0, 0.5) * (1 - out), s: (0.8 + P(t, 1.0, 0.7, E.outBack) * 0.2) * (1 + Math.sin(t * 2.4) * 0.02), r: Math.sin(t * 0.8) * 3 });

      // Values: one word per beat.
      let k = -1;
      for (let i = 0; i < VAL_AT.length; i++) if (t >= VAL_AT[i]) k = i;
      if (k >= 0 && t < 7.62) {
        const [word, cls] = VALUES[k];
        const since = t - VAL_AT[k];
        value.innerHTML = `<span class="${cls}">${esc(word)}</span>`;
        const punch = P(since, 0, 0.32, E.outExpo);
        tf(value, { s: lerp(1.14, 1, punch), o: lerp(0.2, 1, P(since, 0, 0.06, E.linear)), blur: lerp(12, 0, P(since, 0, 0.14, E.outCubic)), y: lerp(16, 0, punch) });
      } else tf(value, { o: 0 });

      tf(whiteout, kf(t, [[7.55, { o: 0 }], [7.98, { o: 1 }, 'inExpo'], [8.05, { o: 1 }]]));
    },
  });
}

// ------------------------------------------------------------ 86–96 s
function cta(ctx) {
  const C1 = [['x-exe', 'curl'], ['', ' '], ['x-flag', '-fsSL'], ['', ' '], ['x-path', 'https://openknowledge.sh/install'], ['', ' '], ['x-pun', '|'], ['', ' '], ['x-cmd', 'bash']];
  const C2 = [['x-exe', 'okn'], ['', ' '], ['x-cmd', 'setup']];
  let flash, glow, logo, hl, box, r1, r2, ty1, ty2, row, black;

  ctx.add({
    id: 'cta',
    build(root, s) {
      glow = el('div', 'cta-glow', root);
      logo = el('img', 'cta-logo', root);
      logo.src = 'assets/logo-mark.png';
      const wrap = el('div', 'cta-hl', root);
      hl = headline(wrap, '', [[['', 'Build a knowledge base']], [['acc', 'your agents can use.']]]);
      box = el('div', 'install', root);
      r1 = el('div', 'install-row', box);
      r2 = el('div', 'install-row', box);
      ty1 = typing(segText(C1), s.start + 2.3, { cps: 36, seed: 501 });
      ty2 = typing(segText(C2), s.start + 4.15, { cps: 22, seed: 502 });
      row = el('div', 'cta-row', root, '<div class="cta-btn">openknowledge.sh   →</div><div class="cta-note">Open source  ·  Apache-2.0</div>');
      flash = el('div', 'whiteout', root);
      black = el('div', 'blackout', root);
      s.cue('final', 0.0);
      s.cue('send', 3.95, { v: 0.6 });
      s.cue('send', 4.75, { v: 0.6 });
      s.cue('pop', 5.3, { v: 0.5, p: 9 });
    },
    update(t) {
      tf(flash, kf(t, [[0, { o: 1 }], [0.9, { o: 0 }, 'outQuad']]));
      const a = P(t, 0.05, 1.0, E.outBack);
      tf(logo, { s: lerp(0.4, 1, a) * (1 + Math.sin(t * 1.7) * 0.012), o: P(t, 0, 0.3), r: lerp(-30, 0, P(t, 0, 1.2, E.outCubic)) + Math.sin(t * 0.9) * 2 });
      tf(glow, { s: lerp(0.5, 1, a) * (1 + Math.sin(t * 2.1) * 0.05), o: P(t, 0, 0.6) });
      riseChars(hl.chars, t, 0.55, { stagger: 0.018, dur: 0.65 });
      tf(box, kf(t, [[1.9, { o: 0, y: 24 }], [2.4, { o: 1, y: 0 }, 'outCubic']]));

      const abs = t + 86;
      const n1 = ty1.count(abs), n2 = ty2.count(abs);
      const cur1 = abs < ty2.start - 0.1 && (abs < ty1.end || blink(t, n1 ? ty1.times[n1 - 1] - 86 : -1));
      const cur2 = abs >= ty2.start - 0.1 && (abs < ty2.end || blink(t, n2 ? ty2.times[n2 - 1] - 86 : -1));
      r1.innerHTML = `<span><span class="x-prompt">$ </span>${segHTML(C1, n1)}${cur1 ? '<span class="cursor"></span>' : ''}</span><span class="copy">${abs >= ty1.end + 0.1 ? 'Copied' : 'Copy'}</span>`;
      r2.innerHTML = `<span><span class="x-prompt">$ </span>${segHTML(C2, n2)}${cur2 ? '<span class="cursor"></span>' : ''}</span><span class="copy">Copy</span>`;
      r2.style.opacity = P(t, 3.9, 0.3).toFixed(3);

      tf(row, kf(t, [[5.25, { o: 0, y: 20, s: 0.96 }], [5.8, { o: 1, y: 0, s: 1 }, 'outBack']]));
      tf(black, kf(t, [[8.4, { o: 0 }], [9.85, { o: 1 }, 'inOutQuad']]));
    },
  });
}
