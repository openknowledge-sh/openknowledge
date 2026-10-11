import { el, tf, kf, P, E, lerp, hash, headline, riseChars, scramble, addStyle } from '../engine.js';

addStyle(`
.flash { position: absolute; inset: 0; background: radial-gradient(circle at 50% 50%, #ffffff 0%, #9ac2ff 22%, rgba(40,120,255,.5) 48%, rgba(0,0,0,0) 80%); mix-blend-mode: screen; }
.shock { position: absolute; left: 960px; top: 420px; width: 100px; height: 100px; margin: -50px 0 0 -50px; border-radius: 50%;
  border: 2px solid rgba(154,194,255,.9); box-shadow: 0 0 30px rgba(102,160,255,.5), inset 0 0 30px rgba(102,160,255,.3); }
.particle { position: absolute; left: 0; top: 0; width: 5px; height: 5px; margin: -2.5px 0 0 -2.5px; border-radius: 3px;
  background: #cfe0ff; box-shadow: 0 0 10px 2px rgba(102,160,255,.8); }
.logo-glow { position: absolute; left: 960px; top: 420px; width: 900px; height: 900px; margin: -450px 0 0 -450px; border-radius: 50%;
  background: radial-gradient(circle, rgba(40,120,255,.45) 0%, rgba(40,120,255,.12) 35%, transparent 65%); }
.logo-big { position: absolute; left: 960px; top: 420px; width: 360px; height: 360px; margin: -180px 0 0 -180px; transform-origin: 50% 50%; }
.wordmark { position: absolute; left: 0; right: 0; top: 590px; text-align: center; font: 500 116px/1 var(--mono); letter-spacing: -0.055em; color: var(--text); white-space: pre; }
.tagline { position: absolute; left: 0; right: 0; top: 748px; text-align: center; }
.tagline .headline { font-size: 46px; letter-spacing: -0.035em; }
.meta { position: absolute; left: 0; right: 0; top: 860px; text-align: center; font: 500 20px/1 var(--mono); letter-spacing: .18em; text-transform: uppercase; color: var(--quiet); white-space: pre; }
.meta b { color: var(--blue-soft); font-weight: 500; }
`);

const N = 170;
const HUD_LOGO = { x: 1590, y: 70, s: 40 / 360 };

export default function reveal(ctx) {
  const parts = [];
  let flash, shock1, shock2, glow, logo, word, tag, meta;

  ctx.add({
    id: 'reveal',
    post: 0.05,
    build(root, s) {
      glow = el('div', 'logo-glow', root);
      for (let i = 0; i < N; i++) {
        const node = el('div', 'particle', root);
        const th = hash(i, 1) * Math.PI * 2;
        const d = 120 + Math.pow(hash(i, 2), 0.7) * 1050;
        parts.push({ node, th, d, i, delay: hash(i, 3) * 0.08 });
      }
      shock1 = el('div', 'shock', root);
      shock2 = el('div', 'shock', root);
      logo = el('img', 'logo-big', root);
      logo.src = 'assets/logo-mark.png';
      logo.alt = '';
      word = el('div', 'wordmark', root);
      tag = el('div', 'tagline', root);
      const hl = headline(tag, '', [[['', 'One knowledge base. '], ['acc', 'For your team and every agent.']]]);
      tag._chars = hl.chars;
      meta = el(
        'div',
        'meta',
        root,
        'Open-source CLI  <b>·</b>  Plain Markdown  <b>·</b>  Your Git repo  <b>·</b>  Any agent',
      );
      flash = el('div', 'flash', root);

      s.cue('boom', 0.0);
      s.cue('shimmer', 0.02);
      s.cue('scramble', 0.75, { d: 0.85 });
      s.cue('pop', 2.9, { v: 0.3, p: 8 });
      s.cue('whoosh', 4.95, { d: 0.9, v: 0.55, up: 1 });
    },
    update(t) {
      tf(flash, kf(t, [[0, { o: 0.95 }], [0.7, { o: 0 }, 'outQuad']]));
      const sw = (node, d) => {
        const p = P(t, d, 1.4, E.outExpo);
        tf(node, { s: lerp(0.1, 26, p), o: (1 - P(t, d, 1.3, E.outQuad)) * (t >= d ? 1 : 0) });
        node.style.borderWidth = `${lerp(3, 0.4, p)}px`;
      };
      sw(shock1, 0);
      sw(shock2, 0.14);

      // Particles: burst outward, decelerate, snap onto the background grid, dissolve into it.
      const gdx = (t + 16) * 6 % 48, gdy = ((t + 16) * 3) % 48;
      for (const p of parts) {
        const b = P(t, p.delay, 1.0, E.outExpo);
        const bx = 960 + Math.cos(p.th) * p.d * b;
        const by = 420 + Math.sin(p.th) * p.d * b * 0.72;
        const gx = Math.round((bx - 24 - gdx) / 48) * 48 + 24 + gdx;
        const gy = Math.round((by - 24 - gdy) / 48) * 48 + 24 + gdy;
        const snap = P(t, 0.75 + hash(p.i, 4) * 0.35, 0.5, E.inOutCubic);
        const x = lerp(bx, gx, snap), y = lerp(by, gy, snap);
        const speed = (1 - b) * (t > p.delay ? 1 : 0);
        const fade = 1 - P(t, 1.25 + hash(p.i, 5) * 0.4, 0.45, E.linear);
        p.node.style.transform = `translate3d(${x.toFixed(1)}px, ${y.toFixed(1)}px, 0) rotate(${(p.th * 57.3).toFixed(1)}deg) scale(${(1 + speed * 7).toFixed(2)}, 1)`;
        p.node.style.opacity = (t >= p.delay ? fade : 0).toFixed(3);
      }

      // Logo arrives, then flies to the HUD position.
      const fly = P(t, 4.95, 0.95, E.inOutQuart);
      const arrive = P(t, 0.02, 0.9, E.outBack);
      const lx = lerp(0, HUD_LOGO.x - 960, fly), ly = lerp(0, HUD_LOGO.y - 420, fly);
      tf(logo, {
        x: lx, y: ly,
        s: lerp(lerp(0.35, 1, arrive), HUD_LOGO.s, fly) * (1 + Math.sin(t * 1.6) * 0.008 * (1 - fly)),
        r: lerp(-28, 0, P(t, 0.02, 1.2, E.outCubic)) + Math.sin(t * 0.9) * 1.5 * (1 - fly),
        o: P(t, 0, 0.25, E.linear) * (1 - P(t, 5.8, 0.12, E.linear)),
        blur: lerp(18, 0, P(t, 0, 0.6, E.outCubic)),
      });
      tf(glow, { x: lx, y: ly, s: lerp(lerp(0.4, 1, arrive), 0.08, fly) * (1 + Math.sin(t * 2.2) * 0.04), o: P(t, 0, 0.4) * (1 - fly) });

      const wp = P(t, 0.75, 0.85, E.linear);
      word.innerHTML = scramble('Open Knowledge', wp, t, { seed: 4, spread: 5 });
      tf(word, kf(t, [[0.7, { o: 0, y: 20 }], [1.1, { o: 1, y: 0 }, 'outCubic'], [4.9, { o: 1, y: 0, blur: 0 }], [5.4, { o: 0, y: -30, blur: 10 }, 'inCubic']]));
      riseChars(tag._chars, t, 1.95, { stagger: 0.012, dur: 0.6, out: { start: 4.95, stagger: 0.003, dur: 0.35 } });
      tf(meta, kf(t, [[2.9, { o: 0, y: 12 }], [3.4, { o: 1, y: 0 }, 'outCubic'], [5.0, { o: 1, y: 0 }], [5.4, { o: 0, y: -16 }]]));
    },
  });
}
