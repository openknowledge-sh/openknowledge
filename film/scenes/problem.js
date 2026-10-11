import {
  el, tf, kf, P, E, clamp, lerp, hash, typing, headline, riseChars, scramble, svg, drawPath, fmt, esc, addStyle, blink,
} from '../engine.js';

addStyle(`
.chat { position: absolute; left: 340px; top: 330px; width: 1240px; border: 1px solid #222a35; border-radius: 12px;
  background: #06080b; box-shadow: 0 40px 140px rgba(0,0,0,.85); transform-origin: 50% 50%; }
.chat-head { display: flex; justify-content: space-between; padding: 18px 28px; border-bottom: 1px solid #151b23;
  font: 500 18px/1 var(--mono); color: var(--quiet); }
.chat-head b { color: var(--text); font-weight: 500; }
.chat-body { padding: 36px 44px 40px; }
.msg-q { font: 500 38px/1.35 var(--mono); color: var(--text); white-space: pre; min-height: 1.35em; }
.msg-q .gt { color: var(--blue-soft); }
.who-row { display: flex; align-items: center; justify-content: space-between; margin-top: 34px; height: 40px; }
.msg-who { font: 500 17px/1 var(--mono); letter-spacing: .16em; text-transform: uppercase; color: var(--quiet); }
.msg-a { position: relative; margin-top: 12px; font: 500 38px/1.35 var(--mono); color: var(--text); white-space: pre; display: inline-block; min-height: 1.35em; }
.strike { position: absolute; left: -8px; right: -8px; top: 54%; height: 4px; background: var(--red); border-radius: 2px; transform-origin: 0 50%; }
.msg-note { margin-top: 22px; font: 500 23px/1.4 var(--mono); color: var(--quiet); white-space: pre; }
.msg-note .p { color: #e6c18c; }
.cold-hl { position: absolute; left: 0; right: 0; top: 700px; text-align: center; }

.tile { position: absolute; width: 272px; height: 168px; border: 1px solid #1d242e; border-radius: 8px; background: #06080b;
  padding: 12px 14px; transform-origin: 50% 50%; overflow: hidden; }
.tile-head { display: flex; justify-content: space-between; font: 500 13px/1 var(--mono); color: var(--quiet); padding-bottom: 10px;
  border-bottom: 1px solid #141a22; margin-bottom: 10px; }
.tile-head b { color: var(--muted); font-weight: 500; }
.tile-line { font: 500 14px/1.55 var(--mono); white-space: pre; overflow: hidden; color: var(--text); }
.tile-line.ctx { color: #7f8896; }
.tile-line.ctx i { font-style: normal; color: var(--amber); }
.tile-paste { position: absolute; left: 10px; right: 10px; bottom: 12px; height: 52px; border-radius: 4px; background: rgba(102,160,255,.22); }
.again-hl { position: absolute; left: 104px; top: 858px; }
.again-sub { position: absolute; left: 104px; top: 960px; }
.counter { position: absolute; right: 104px; top: 846px; text-align: right; }
.counter .n { font: 500 92px/1 var(--mono); color: var(--blue-soft); letter-spacing: -0.04em; }
.counter .l { font: 500 18px/1 var(--mono); color: var(--quiet); letter-spacing: .16em; text-transform: uppercase; margin-bottom: 14px; }

.scatter { perspective: 1500px; perspective-origin: 50% 50%; }
.frag { position: absolute; left: 960px; top: 540px; transform-style: preserve-3d; }
.frag-in { position: relative; left: -50%; top: -50%; padding: 18px 22px 18px; border: 1px solid #252d38; border-radius: 10px;
  background: #090c11; box-shadow: 0 30px 80px rgba(0,0,0,.7); white-space: pre; }
.frag-kind { font: 500 14px/1 var(--mono); letter-spacing: .16em; text-transform: uppercase; color: var(--dim); margin-bottom: 10px; }
.frag-name { font: 500 26px/1.1 var(--mono); color: var(--text); }
.frag .chip { margin-top: 14px; font-size: 18px; padding: 8px 12px; }
.scatter-hl { position: absolute; left: 0; right: 0; top: 462px; text-align: center; }
.headline .warn { color: var(--amber); }
.singularity { position: absolute; left: 960px; top: 540px; width: 10px; height: 10px; margin: -5px 0 0 -5px; border-radius: 50%;
  background: #fff; box-shadow: 0 0 40px 18px rgba(154,194,255,.9), 0 0 160px 60px rgba(40,120,255,.6); }
`);

export default function problem(ctx) {
  coldOpen(ctx);
  again(ctx);
  scattered(ctx);
}

// ------------------------------------------------------------ 0–6 s
function coldOpen(ctx) {
  const Q = 'how long do we retry failed webhooks?';
  const A = 'Retries run for 3 days with linear backoff.';
  const words = A.split(' ');
  let cam, panel, q, a, who, chip, note, hl, typeQ;
  const wordAt = (i) => 2.62 + i * 0.085;

  ctx.add({
    id: 'cold',
    post: 0.05,
    build(root, s) {
      cam = el('div', 'layer', root);
      panel = el('div', 'chat', cam);
      el('div', 'chat-head', panel, '<span><b>●</b>  session 001</span><span>new chat</span>');
      const body = el('div', 'chat-body', panel);
      q = el('div', 'msg-q', body);
      const row = el('div', 'who-row', body);
      who = el('div', 'msg-who', row, 'agent');
      chip = el('div', 'chip r', row, '✗  made up');
      a = el('div', 'msg-a', body);
      note = el(
        'div',
        'msg-note',
        body,
        '↳ your docs say: <span class="o-soft">24 hours, exponential backoff</span>  ·  <span class="p">docs/delivery.md</span>',
      );
      hl = headline(root, 'cold-hl', [[['', "Your agent doesn't know"]], [['acc', 'your project.']]]);

      typeQ = typing(Q, 0.62, { cps: 27, seed: 11 });
      s.cue('send', 2.08);
      words.forEach((w, i) => s.cue('tick', wordAt(i), { v: 0.5 }));
      s.cue('error', 3.55);
      s.cue('hit', 4.42);
      s.cue('whoosh', 5.55, { d: 0.5, v: 0.6 });
    },
    update(t) {
      tf(cam, kf(t, [[0, { s: 0.965 }], [6, { s: 1.03 }, 'linear']]));
      tf(panel, kf(t, [
        [0, { o: 0, y: 30, blur: 12 }],
        [0.5, { o: 1, y: 0, blur: 0 }, 'outCubic'],
        [4.3, { y: 0, s: 1 }],
        [5.0, { y: -170, s: 0.9 }, 'inOutQuart'],
        [5.55, { y: -170, s: 0.9, o: 1, blur: 0 }],
        [6.0, { y: -40, s: 0.42, o: 0, blur: 14 }, 'inCubic'],
      ]));

      // Question with blinking cursor.
      const n = typeQ.count(t);
      const typingNow = t < typeQ.end + 0.05;
      const last = n > 0 ? typeQ.times[n - 1] : -1;
      const cur = t < 2.08 && (typingNow ? 1 : blink(t, last)) ? '<span class="cursor"></span>' : '';
      q.innerHTML = `<span class="gt">›</span> ${esc(Q.slice(0, n))}${cur}`;

      // Agent label and thinking dots.
      const whoP = P(t, 2.12, 0.25);
      who.style.opacity = whoP;
      const thinking = t >= 2.12 && t < 2.62;
      const nd = Math.floor((t - 2.12) / 0.12) % 4;
      who.innerHTML = thinking ? `agent  <span class="o-hi">${'·'.repeat(nd)}</span>` : 'agent';

      // Streamed answer.
      let shown = 0;
      while (shown < words.length && wordAt(shown) <= t) shown++;
      a.innerHTML = esc(words.slice(0, shown).join(' ')) + '<div class="strike"></div>';
      const st = a.lastChild;
      const sp = P(t, 3.55, 0.28, E.outQuart);
      st.style.transform = `scaleX(${sp})`;
      st.style.opacity = sp > 0 ? 1 : 0;
      a.style.color = sp > 0 ? `rgba(237,237,237,${lerp(1, 0.42, sp)})` : '';

      tf(chip, kf(t, [[3.55, { o: 0, s: 0.6 }], [3.85, { o: 1, s: 1 }, 'outBack']]));
      tf(note, kf(t, [[3.95, { o: 0, y: 10 }], [4.35, { o: 1, y: 0 }, 'outCubic']]));

      riseChars(hl.chars, t, 4.42, { stagger: 0.016, dur: 0.6, out: { start: 5.5, stagger: 0.004, dur: 0.3 } });
    },
  });
}

// ------------------------------------------------------------ 6–11 s
const QUESTIONS = [
  'how long do we retry failed webhooks?',
  'why did we pick postgres?',
  'where do delivery logs live?',
  "what's the retry policy again?",
  'who owns the webhook signer?',
  'how do we version the API?',
  'can we drop the v1 endpoint?',
  'what does error R-429 mean?',
  'how do I run the test suite?',
  'is retry 24h or 3 days?',
  'what are our naming rules?',
  'why exponential backoff?',
];
const CONTEXT = [
  ['<i>context:</i> Relay sends webhooks to', 'customers. retries: 24h, exp…'],
  ['<i>context:</i> we chose postgres for', 'transactional outbox (see adr?)'],
  ['<i>context:</i> conventions are in the', 'old wiki, maybe? ask platform'],
  ['<i>context:</i> API v2 since march.', 'v1 still used by 3 customers'],
];
const TOOLS = ['chat', 'ide', 'cli agent', 'browser', 'chat', 'ide'];

function again(ctx) {
  const COLS = 6, ROWS = 4, TW = 272, TH = 168, GAP = 16, LEFT = 104, TOP = 92;
  const tiles = [];
  let cam, hl, sub, counter, num;

  ctx.add({
    id: 'again',
    post: 0.05,
    build(root, s) {
      cam = el('div', 'layer', root);
      cam.style.transformOrigin = '0 0';
      for (let i = 0; i < COLS * ROWS; i++) {
        const c = i % COLS, r = Math.floor(i / COLS);
        const tile = el('div', 'tile', cam);
        tile.style.left = `${LEFT + c * (TW + GAP)}px`;
        tile.style.top = `${TOP + r * (TH + GAP)}px`;
        const ctxLines = CONTEXT[(i * 3 + r) % CONTEXT.length];
        const qtext = QUESTIONS[i % QUESTIONS.length];
        el('div', 'tile-head', tile, `<span><b>●</b> session ${fmt(i + 1)}</span><span>${TOOLS[(i + r) % TOOLS.length]}</span>`);
        el('div', 'tile-line', tile, `<span class="o-hi">›</span> ${esc(qtext.length > 26 ? qtext.slice(0, 25) + '…' : qtext)}`);
        el('div', 'tile-line ctx', tile, ctxLines[0]);
        el('div', 'tile-line ctx', tile, ctxLines[1]);
        const paste = el('div', 'tile-paste', tile);
        tiles.push({ tile, paste, appear: 6.02 + i * 0.052 - 6, seed: i });
        if (i > 0) s.cue('pop', 0.02 + i * 0.052, { v: 0.35, p: i });
      }
      hl = headline(root, 'again-hl h-feature', [[['', 'So you explain it. '], ['acc', 'Again.']]]);
      sub = el('div', 'sub again-sub', root, 'Every session starts from zero. Every tool forgets.');
      counter = el('div', 'counter', root, '<div class="l">sessions this month</div><div class="n">001</div>');
      num = counter.querySelector('.n');
      s.cue('whoosh', 0.0, { d: 0.9, v: 0.5, up: 1 });
      let prev = 1;
      for (let k = 0; k <= 200; k++) {
        const lt = 0.3 + (k / 200) * 3.4;
        const v = 1 + Math.floor(46 * E.outCubic(k / 200));
        if (v !== prev) {
          s.cue('count', lt, { v: 0.22 });
          prev = v;
        }
      }
      s.cue('whoosh', 4.25, { d: 0.6, v: 0.45 });
    },
    update(t) {
      // Pull back from session 001 to reveal the grid.
      const p = P(t, 0, 1.7, E.outQuart);
      const s0 = 3.4;
      const cx = LEFT + TW / 2, cy = TOP + TH / 2;
      const sc = lerp(s0, 1, p) * lerp(1, 0.975, P(t, 1.7, 3.3, E.linear));
      const dx = lerp(960 - cx * s0, 0, p) + (1 - sc) * 960 * (p >= 1 ? 1 : 0);
      const dy = lerp(540 - cy * s0, 0, p) + (1 - sc) * 420 * (p >= 1 ? 1 : 0);
      cam.style.transform = `translate3d(${dx.toFixed(2)}px, ${dy.toFixed(2)}px, 0) scale(${sc.toFixed(4)})`;

      for (const tl of tiles) {
        const a = P(t, tl.appear, 0.32, E.outBack);
        const fall = P(t, 4.2 + hash(tl.seed, 3) * 0.45, 0.45, E.inCubic);
        const flicker = t > 3.6 && t < 4.6 && hash(tl.seed, Math.floor(t * 20)) > 0.82 ? 0.35 : 1;
        tf(tl.tile, {
          o: Math.min(1, a * 1.2) * (1 - fall) * flicker,
          s: lerp(0.86, 1, a) * lerp(1, 0.7, fall),
          y: fall * (40 + hash(tl.seed, 9) * 60),
          r: fall * (hash(tl.seed, 4) - 0.5) * 16,
          blur: fall * 8,
        });
        const pp = P(t, tl.appear + 0.18, 0.5, E.outQuad);
        tl.paste.style.opacity = (pp > 0 && pp < 1 ? 1 - pp : 0).toFixed(3);
      }

      riseChars(hl.chars, t, 1.25, { stagger: 0.02, out: { start: 4.3, dur: 0.3 } });
      tf(sub, kf(t, [[1.9, { o: 0, y: 14 }], [2.4, { o: 1, y: 0 }, 'outCubic'], [4.35, { o: 1 }], [4.7, { o: 0, y: -10 }]]));
      tf(counter, kf(t, [[0.3, { o: 0, y: 14 }], [0.7, { o: 1, y: 0 }, 'outCubic'], [4.35, { o: 1 }], [4.7, { o: 0 }]]));
      const v = 1 + Math.floor(46 * E.outCubic(clamp((t - 0.3) / 3.4)));
      num.textContent = fmt(v);
    },
  });
}

// ------------------------------------------------------------ 11–16 s
const FRAGS = [
  { kind: 'markdown', name: 'README.md', tag: ['q', 'last edited 2023'], x: -610, y: -310, z: -120 },
  { kind: 'slack', name: '#eng-webhooks', tag: ['a', 'buried · 212 replies'], x: -130, y: -380, z: -260 },
  { kind: 'markdown', name: 'docs/delivery.md', tag: ['a', 'stale · 14 months'], x: 330, y: -300, z: 0 },
  { kind: 'markdown', name: 'notes/retries-v2.md', tag: ['r', 'conflicts with delivery.md'], x: 690, y: -170, z: 80 },
  { kind: 'markdown', name: 'ADR-0007-postgres.md', tag: ['q', 'linked from nowhere'], x: -640, y: 250, z: 60 },
  { kind: 'old wiki', name: 'Relay (2023)', tag: ['a', 'no owner'], x: -90, y: 330, z: -160 },
  { kind: 'memory', name: "someone's head", tag: ['r', 'on vacation'], x: 540, y: 290, z: -40 },
];

function scattered(ctx) {
  const frags = [];
  let layer, hl, wire, conflict, dot;

  ctx.add({
    id: 'scattered',
    post: 0.02,
    build(root, s) {
      layer = el('div', 'layer scatter', root);
      wire = svg('svg', { class: 'wire', width: 1920, height: 1080, viewBox: '0 0 1920 1080' }, layer);
      conflict = svg('path', { fill: 'none', stroke: '#ff4b45', 'stroke-width': 2, 'stroke-dasharray': '8 8', opacity: 0.8 }, wire);
      FRAGS.forEach((f, i) => {
        const node = el('div', 'frag', layer);
        const inner = el('div', 'frag-in', node);
        el('div', 'frag-kind', inner, f.kind);
        el('div', 'frag-name', inner, esc(f.name));
        const chip = el('div', `chip ${f.tag[0]}`, inner, esc(f.tag[1]));
        frags.push({ ...f, node, chip, i });
        s.cue(f.tag[0] === 'r' ? 'error' : 'pop', 1.3 + i * 0.2, { v: f.tag[0] === 'r' ? 0.45 : 0.4, p: i + 4 });
      });
      hl = headline(root, 'scatter-hl', [
        [['', "And what's written down is"]],
        [['warn', 'scattered, stale, unverifiable.']],
      ]);
      hl.root.style.fontSize = '76px';
      dot = el('div', 'singularity', root);
      s.cue('whoosh', 0.0, { d: 0.8, v: 0.4 });
      s.cue('riser', 2.4, { d: 2.6 });
      s.cue('suck', 3.6, { d: 1.25 });
    },
    update(t) {
      const implode = P(t, 3.55, 1.3, E.inExpo);
      const pos = [];
      for (const f of frags) {
        const enter = P(t, 0.05 + f.i * 0.07, 0.9, E.outQuart);
        const drift = t * 9;
        const x0 = f.x + Math.sin(t * 0.7 + f.i) * 12 + (hash(f.i, 1) - 0.5) * drift;
        const y0 = f.y + Math.cos(t * 0.6 + f.i * 2) * 10 + (hash(f.i, 2) - 0.5) * drift;
        const x = lerp(lerp(0, x0, enter), 0, implode);
        const y = lerp(lerp(0, y0, enter), 0, implode);
        const z = lerp(lerp(-700, f.z, enter), -200, implode);
        const decay = f.tag[0] !== 'q' && t > 1.8 && hash(f.i, Math.floor(t * 14)) > 0.88 ? 0.55 : 1;
        tf(f.node, {
          x, y, z,
          ry: (hash(f.i, 5) - 0.5) * 18 + Math.sin(t * 0.5 + f.i) * 4,
          rx: (hash(f.i, 6) - 0.5) * 10,
          o: enter * decay * (1 - P(t, 4.55, 0.3, E.linear)),
          s: lerp(1, 0.05, implode),
          blur: implode * 14 + (1 - enter) * 10,
        });
        tf(f.chip, kf(t, [[1.3 + f.i * 0.2, { o: 0, s: 0.7 }], [1.55 + f.i * 0.2, { o: 1, s: 1 }, 'outBack']]));
        pos.push({ x: 960 + x, y: 540 + y });
      }
      // Conflict line between delivery.md and retries-v2.md.
      const a = pos[2], b = pos[3];
      conflict.setAttribute('d', `M ${a.x + 40} ${a.y + 40} Q ${(a.x + b.x) / 2 + 30} ${Math.min(a.y, b.y) - 70} ${b.x - 60} ${b.y - 30}`);
      drawPath(conflict, P(t, 2.0, 0.5, E.outCubic), 1200);
      conflict.style.opacity = (0.85 * (1 - implode)).toFixed(3);

      riseChars(hl.chars, t, 0.6, { stagger: 0.014, out: { start: 3.45, stagger: 0.006, dur: 0.5, dy: -0.1, blur: 16 } });
      const sq = P(t, 3.55, 1.2, E.inExpo);
      hl.root.style.transform = `scale(${lerp(1, 0.2, sq)})`;

      const d = kf(t, [[4.2, { o: 0, s: 0.2 }], [4.95, { o: 1, s: 1.6 }, 'inCubic'], [5.0, { o: 1, s: 2.2 }]]);
      tf(dot, d);
    },
  });
}
