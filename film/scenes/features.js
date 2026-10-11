import { el, tf, kf, P, E, clamp, lerp, hash, typing, scramble, svg, drawPath, esc, addStyle, blink } from '../engine.js';
import { place, offsetIn, term, titleBlock, panelIn, S } from './kit.js';

addStyle(`
/* create */
.dir-label { position: absolute; font: 500 22px/1 var(--mono); color: var(--muted); white-space: pre; }
.dir-label b { color: #e6c18c; font-weight: 500; }
.doc-card { position: absolute; width: 240px; background: #0a0d12; border: 1px solid #222a35; border-radius: 8px; overflow: hidden;
  box-shadow: 0 24px 60px rgba(0,0,0,.6); }
.doc-card .dh { padding: 12px 14px; font: 500 15px/1 var(--mono); color: var(--quiet); border-bottom: 1px solid #161c25; }
.doc-card .fm { overflow: hidden; background: rgba(102,160,255,.1); border-bottom: 1px dashed rgba(102,160,255,.4); }
.doc-card .fm div, .doc-card .db div { font: 500 14px/1.6 var(--mono); white-space: pre; }
.doc-card .fm div { color: var(--blue-light); padding: 0 14px; }
.doc-card .fm div:first-child { padding-top: 10px; } .doc-card .fm div:last-child { padding-bottom: 10px; }
.doc-card .db { padding: 12px 14px 14px; color: var(--text); }
.doc-card .db div:first-child { color: var(--blue-soft); }

/* find */
.code-card .cc-head { display: flex; justify-content: space-between; padding: 15px 20px; border-bottom: 1px solid #161c25; font: 500 18px/1 var(--mono); color: var(--quiet); }
.code-card .cc-head b { color: #e6c18c; font-weight: 500; }
.code-card .cc-body { position: relative; padding: 16px 0; }
.code-row { display: flex; font: 500 19px/1.6 var(--mono); white-space: pre; position: relative; z-index: 1; }
.code-row .ln { width: 64px; text-align: right; padding-right: 20px; color: #4a525e; flex: none; }
.code-row .tx { color: var(--text); }
.code-row .tx.fm { color: #7f8896; }
.code-row .tx.h { color: var(--blue-soft); }
.band { position: absolute; left: 0; right: 0; background: linear-gradient(90deg, rgba(102,160,255,.22), rgba(102,160,255,.08)); border-left: 3px solid var(--blue-soft); transform-origin: 50% 0; }
.band-tag { position: absolute; right: 16px; }

/* connect */
.hub { position: absolute; left: 960px; top: 640px; width: 220px; height: 220px; margin: -110px 0 0 -110px; border-radius: 50%;
  background: radial-gradient(circle at 50% 40%, #0d1626, #05070a 70%); border: 1px solid #2c3d57; display: grid; place-items: center;
  box-shadow: 0 0 80px rgba(40,120,255,.25); }
.hub img { width: 96px; height: 96px; margin-top: -18px; }
.hub span { position: absolute; bottom: 38px; font: 500 22px/1 var(--mono); color: #e6c18c; }
.hub-ring { position: absolute; left: 960px; top: 640px; width: 220px; height: 220px; margin: -110px 0 0 -110px; border-radius: 50%; border: 1px solid rgba(102,160,255,.5); }
.sat { position: absolute; left: 0; top: 0; }
.sat-in { position: relative; left: -50%; top: -50%; padding: 14px 20px; border-radius: 8px; background: #0a0e14; border: 1px solid #263244; white-space: pre; text-align: center; }
.sat-k { font: 500 13px/1 var(--mono); letter-spacing: .16em; text-transform: uppercase; color: var(--quiet); margin-bottom: 9px; }
.sat-n { font: 500 25px/1 var(--mono); color: var(--text); }
.cfg { position: absolute; left: 680px; top: 800px; width: 560px; padding: 18px 24px; font: 500 19px/1.55 var(--mono); white-space: pre; color: var(--text); }

/* update */
.steps { position: absolute; display: flex; flex-direction: column; gap: 22px; }
.step { display: flex; align-items: center; gap: 18px; font: 500 23px/1.2 var(--mono); color: #4d5560; white-space: pre; }
.step .num { width: 40px; height: 40px; border-radius: 6px; border: 1px solid #2a313b; display: grid; place-items: center; font-size: 18px; flex: none; }
.step.on { color: var(--text); } .step.on .num { background: var(--blue-soft); border-color: var(--blue-soft); color: #07162d; }
.step.done { color: var(--muted); } .step.done .num { border-color: rgba(72,221,120,.5); color: var(--green); }

/* check */
.src-card .sc-body { padding: 16px 20px; font: 500 19px/1.6 var(--mono); white-space: pre; }
.del { color: #ff8a85; background: rgba(255,75,69,.1); display: block; margin: 0 -20px; padding: 0 20px; }
.addl { color: var(--green); background: rgba(72,221,120,.09); display: block; margin: 0 -20px; padding: 0 20px; }
.pr-title { padding: 18px 22px 6px; font: 500 22px/1.3 var(--mono); color: var(--text); white-space: pre; }
.pr-title .o-dim { margin-left: 10px; }
.pr-meta { padding: 0 22px 16px; font: 500 16px/1 var(--mono); color: var(--quiet); border-bottom: 1px solid #161c25; }
.pr-row { display: flex; gap: 14px; padding: 13px 22px; font: 500 19px/1.2 var(--mono); white-space: pre; border-bottom: 1px solid #12171e; }
.pr-row .ic { width: 22px; flex: none; }
.pr-row .k { width: 300px; flex: none; color: var(--text); }
.pr-row .v { color: var(--quiet); }

/* ship */
.out-card { position: absolute; width: 400px; height: 112px; padding: 20px 22px; }
.out-card .on { font: 500 25px/1 var(--mono); color: #e6c18c; }
.out-card .od { margin-top: 14px; font: 500 18px/1 var(--mono); color: var(--quiet); }
.gen { position: absolute; transform: translate(-50%, 0); text-align: center; white-space: pre; }
.gen .gl { font: 500 22px/1 var(--mono); color: var(--text); margin-bottom: 18px; }
.gen .gd { width: 22px; height: 22px; border-radius: 50%; margin: 0 auto; background: #0a0e14; border: 2px solid #3a4658; }
.gen .chip { margin-top: 18px; font-size: 17px; padding: 7px 11px; }
.active-pill { position: absolute; top: 0; left: 0; transform: translate(-50%, 0); font: 500 15px/1 var(--mono); letter-spacing: .14em; text-transform: uppercase;
  color: #07162d; background: var(--blue-soft); padding: 8px 12px; border-radius: 5px; box-shadow: 0 0 30px rgba(102,160,255,.6); white-space: pre; }
.caption { position: absolute; font: 500 22px/1 var(--mono); letter-spacing: .12em; text-transform: uppercase; color: var(--quiet); white-space: pre; }
.caption b { color: var(--blue-soft); font-weight: 500; }
`);

export default function features(ctx) {
  create(ctx);
  find(ctx);
  connect(ctx);
  update(ctx);
  check(ctx);
  ship(ctx);
}

const OUT = 9.35;

// ------------------------------------------------------------ 22–32 s  CREATE
function create(ctx) {
  const DOCS = [
    { name: 'overview.md', title: '"Relay overview"', body: ['# Relay overview', '', 'Relay delivers webhooks', 'from our API to …'] },
    { name: 'delivery.md', title: '"Why we retry…"', body: ['# Why we retry webhooks', '', 'Retry failed deliveries', 'for 24 hours with …'] },
    { name: 'operations.md', title: '"Operations"', body: ['# Operations', '', 'Support checks the', 'delivery log first.'] },
  ];
  let title, T, label, chip, cards = [], callouts;

  ctx.add({
    id: 'create',
    post: 0.05,
    build(root, s) {
      title = titleBlock(root, [S('', 'Start from the docs '), S('acc', 'you already have.')], 'okn setup turns ordinary Markdown into a managed knowledge base.', { outAt: OUT });
      T = term(root, s, { x: 120, y: 318, w: 860, title: '~/relay' });
      T.cmd([S('x-exe', 'okn'), S('', ' '), S('x-cmd', 'setup'), S('', ' '), S('x-path', 'Wiki'), S('', ' '), S('x-flag', '--from'), S('', ' '), S('x-path', 'docs'), S('', ' '), S('x-flag', '--mode'), S('', ' '), S('x-arg', 'copy')], 0.7);
      T.out('', 2.3);
      T.out('<span class="o-hi">Setup plan</span>', 2.35, { cue: 'line' });
      T.out('Mode:       copy', 2.5, { cue: 'line' });
      T.out('Documents:  3', 2.62, { cue: 'line' });
      T.out('Changes:', 2.74, { cue: 'line' });
      T.out('    9 create', 2.86, { cue: 'line' });
      T.out('    3 add minimal frontmatter', 2.98, { cue: 'line' });
      T.out('    0 delete', 3.1, { cue: 'line' });
      T.out('', 3.2);
      T.out('<span class="o-soft">Knowledge base created at ./Wiki.</span>', 4.3, { cue: 'success' });
      T.out('', 4.4);
      T.out('Documents       3', 4.55, { cue: 'line' });
      T.out('Validation      <span class="o-ok">READY</span>', 4.78, { cue: 'ding', cueOpts: { p: 0 } });
      T.out('Search index    <span class="o-ok">READY</span> <span class="o-dim">(2 hits)</span>', 5.0, { cue: 'ding', cueOpts: { p: 1 } });

      label = place(el('div', 'dir-label', root), 1040, 330);
      chip = place(el('div', 'chip q', root, 'copy · originals unchanged'), 1470, 318);
      DOCS.forEach((d, i) => {
        const card = place(el('div', 'doc-card', root), 1040 + i * 260, 380);
        el('div', 'dh', card, d.name);
        const fm = el('div', 'fm', card, ['---', 'type: Document', `title: ${d.title}`, '---'].map((l) => `<div>${esc(l)}</div>`).join(''));
        el('div', 'db', card, d.body.map((l) => `<div>${esc(l) || '&nbsp;'}</div>`).join(''));
        cards.push({ card, fm, i });
        s.cue('pop', 0.95 + i * 0.12, { v: 0.4, p: i });
        s.cue('blip', 3.25 + i * 0.16, { v: 0.5, p: i });
      });
      s.cue('scramble', 3.2, { d: 0.5, v: 0.5 });
      s.cue('pop', 3.8, { v: 0.35, p: 5 });
      callouts = place(el('div', 'callouts', root), 1040, 720);
      ['No frontmatter or links required.', 'Shows the plan before it writes.', 'No docs yet? Your agent drafts them.'].forEach((c, i) => {
        el('div', 'callout', callouts, esc(c));
        s.cue('pop', 5.7 + i * 0.5, { v: 0.45, p: 6 + i });
      });
      s.cue('whoosh', OUT, { d: 0.6, v: 0.45 });
    },
    update(t) {
      title.update(t);
      tf(T.root, panelIn(t, 0.45, OUT));
      T.update(t);
      const lp = P(t, 3.2, 0.5, E.linear);
      label.innerHTML = t < 3.2 ? '▸ <b>docs/</b>  <span class="o-dim">3 Markdown files</span>' : `▸ <b>${scramble('Wiki/imported/', lp, t, { seed: 9 })}</b>`;
      tf(label, panelIn(t, 0.8, OUT, { dy: 16 }));
      tf(chip, kf(t, [[3.8, { o: 0, s: 0.8 }], [4.1, { o: 1, s: 1 }, 'outBack'], [OUT, { o: 1, s: 1 }], [OUT + 0.4, { o: 0 }]]));
      for (const c of cards) {
        const p = panelIn(t, 0.95 + c.i * 0.12, OUT + c.i * 0.05, { dy: 50 });
        const g = P(t, 3.25 + c.i * 0.16, 0.45, E.outQuart);
        tf(c.card, { ...p, y: p.y - g * 6 });
        c.fm.style.height = `${(g * 112).toFixed(1)}px`;
        c.card.style.borderColor = g > 0 && g < 1 ? '#3d6fb8' : '#222a35';
      }
      [...callouts.children].forEach((n, i) => tf(n, kf(t, [[5.7 + i * 0.5, { o: 0, x: -20 }], [6.1 + i * 0.5, { o: 1, x: 0 }, 'outCubic'], [OUT + i * 0.05, { o: 1, x: 0 }], [OUT + 0.4 + i * 0.05, { o: 0, x: -40 }]])));
    },
  });
}

// ------------------------------------------------------------ 32–42 s  FIND
function find(ctx) {
  // [class, text, line number]; '' marks a soft-wrapped continuation.
  const FILE = [
    ['fm', '---', '1'], ['fm', 'type: Document', '2'], ['fm', 'title: "Why we retry webhooks"', '3'], ['fm', '---', '4'], ['', '', '5'],
    ['h', '# Why we retry webhooks', '6'], ['', '', '7'],
    ['', 'Retry failed deliveries for 24 hours', '8'], ['', 'with exponential backoff.', ''], ['', '', '9'],
    ['', 'A receiver can be unavailable during', '10'], ['', 'a deploy.', ''], ['', '', '11'],
  ];
  let title, T, card, band, bandTag, hashChip, wires, wire, wire2, pulse, chat, q, ans, cite, grounded, foot;
  const ANSWER = '24 hours, with exponential backoff.';
  let geo = null;

  ctx.add({
    id: 'find',
    post: 0.05,
    build(root, s) {
      title = titleBlock(root, [S('', 'Answers '), S('acc', 'with receipts.')], 'Every result cites its file, its lines, and a content hash.', { outAt: OUT });
      T = term(root, s, { x: 120, y: 318, w: 900, title: '~/relay' });
      T.cmd([S('x-exe', 'okn'), S('', ' '), S('x-cmd', 'search'), S('', ' '), S('x-path', 'Wiki'), S('', ' '), S('x-str', '"how long do we retry?"')], 0.6);
      T.out('', 1.9);
      T.out('<span class="o-hi"># Open Knowledge Context</span>', 1.95, { cue: 'line' });
      T.out('Status: <span class="o-soft">managed</span>', 2.05, { cue: 'line' });
      T.out('Context: 1410 / 2400 estimated tokens', 2.15, { cue: 'line' });
      T.out('Sources: 12', 2.25, { cue: 'line' });
      T.out('', 2.3);
      T.out('<span class="o-hi">## 1. Why we retry webhooks</span>', 2.45, { cue: 'line' });
      const srcLine = T.out('Source:   <span class="x-path src-anchor">imported/delivery.md:6-11</span>', 2.6, { cue: 'line' });
      T.out('<span class="o-muted">Locator:  okf+sha256://3b09…/imported%2Fdelivery.md#78fa…</span>', 2.72, { cue: 'line' });
      T.out('Relation: direct', 2.84, { cue: 'line' });
      T.out('', 2.9);
      T.out('Retry failed deliveries for 24 hours with exponential backoff.', 2.98, { cue: 'line' });
      T._src = srcLine;

      card = place(el('div', 'card code-card', root), 1080, 318, 720);
      el('div', 'cc-head', card, '<b>imported/delivery.md</b><span>OKF 0.2</span>');
      const body = el('div', 'cc-body', card);
      band = el('div', 'band', body);
      for (const [cls, text, ln] of FILE) {
        el('div', 'code-row', body, `<span class="ln">${ln}</span><span class="tx ${cls}">${esc(text) || ' '}</span>`);
      }
      bandTag = el('div', 'chip b band-tag', body, 'lines 6–11');
      hashChip = place(el('div', 'chip q', root, '◆ okf+sha256 locator pins the exact text'), 1080, 818);

      wires = svg('svg', { class: 'wire', width: 1920, height: 1080 }, root);
      wire = svg('path', { fill: 'none', stroke: '#66a0ff', 'stroke-width': 2.5, 'stroke-linecap': 'round' }, wires);
      wire2 = svg('path', { fill: 'none', stroke: '#48dd78', 'stroke-width': 2.5, 'stroke-linecap': 'round' }, wires);
      pulse = el('div', 'pulse-dot', root);

      chat = place(el('div', 'chat', root), 120, 360, 900);
      chat.style.transformOrigin = '0 50%';
      el('div', 'chat-head', chat, '<span><b class="o-soft">●</b>  session 048</span><span>agent + okn search</span>');
      const cb = el('div', 'chat-body', chat);
      q = el('div', 'msg-q', cb, '<span class="gt">›</span> how long do we retry failed webhooks?');
      q.style.fontSize = '30px';
      const row = el('div', 'who-row', cb);
      el('div', 'msg-who', row, 'agent');
      grounded = el('div', 'chip g', row, '✓  grounded');
      ans = el('div', 'msg-a', cb);
      ans.style.fontSize = '32px';
      cite = el('div', 'msg-note', cb, '↳ <span class="p">imported/delivery.md:6-11</span>');
      foot = place(el('div', 'callouts', root), 120, 820);
      el('div', 'callout', foot, 'Local and deterministic. No LLM. No network.');
      root.appendChild(wires);
      root.appendChild(pulse);

      s.cue('beam', 3.2, { d: 0.5 });
      s.cue('blip', 3.6, { v: 0.5, p: 3 });
      s.cue('pop', 3.95, { v: 0.35, p: 4 });
      s.cue('whoosh', 4.95, { d: 0.6, v: 0.5 });
      ANSWER.split(' ').forEach((w, i) => s.cue('tick', 6.0 + i * 0.08, { v: 0.45 }));
      s.cue('tick', 6.9, { v: 0.6 });
      s.cue('beam', 7.0, { d: 0.45 });
      s.cue('success', 7.45);
      s.cue('pop', 8.0, { v: 0.4, p: 6 });
      s.cue('whoosh', OUT, { d: 0.6, v: 0.45 });
    },
    update(t) {
      title.update(t);
      const tIn = panelIn(t, 0.4, 4.9, { exitDx: -80 });
      tf(T.root, tIn);
      T.update(t);
      tf(card, panelIn(t, 0.6, OUT, { dx: 60, dy: 0, exitDx: 60 }));

      if (!geo) {
        const rows = card.querySelectorAll('.code-row');
        const r6 = offsetIn(rows[5], card), r12 = offsetIn(rows[12], card);
        const anchor = T.root.querySelector('.src-anchor');
        const a = offsetIn(anchor, T.root.parentNode);
        geo = { bandTop: rows[5].offsetTop, bandH: rows[12].offsetTop + rows[12].offsetHeight - rows[5].offsetTop, cardY: 318 + r6.y, cardH: r12.y + r12.h - r6.y,
          ax: a.x + a.w + 14, ay: a.y + a.h / 2 };
        band.style.top = `${geo.bandTop}px`;
        band.style.height = `${geo.bandH}px`;
        bandTag.style.top = `${geo.bandTop + 10}px`;
      }
      const bp = P(t, 3.45, 0.45, E.outQuart);
      band.style.transform = `scaleY(${bp})`;
      band.style.opacity = bp > 0 ? 1 : 0;
      tf(bandTag, kf(t, [[3.6, { o: 0, s: 0.7 }], [3.85, { o: 1, s: 1 }, 'outBack']]));
      tf(hashChip, kf(t, [[3.95, { o: 0, y: 10 }], [4.3, { o: 1, y: 0 }, 'outCubic'], [OUT, { o: 1, y: 0 }], [OUT + 0.4, { o: 0 }]]));

      const bx = 1080, by = geo.cardY + geo.cardH / 2;
      const d1 = `M ${geo.ax} ${geo.ay} C ${geo.ax + 220} ${geo.ay}, ${bx - 180} ${by}, ${bx - 4} ${by}`;
      wire.setAttribute('d', d1);
      drawPath(wire, P(t, 3.15, 0.5, E.inOutCubic), 1000);
      wire.style.opacity = (1 - P(t, 4.8, 0.3, E.linear)).toFixed(3);

      // Payoff: the same agent, now grounded.
      tf(chat, kf(t, [[5.1, { o: 0, x: -40, blur: 8 }], [5.7, { o: 1, x: 0, blur: 0 }, 'outQuart'], [OUT, { o: 1, x: 0 }], [OUT + 0.5, { o: 0, x: -60, blur: 8 }, 'inCubic']]));
      const words = ANSWER.split(' ');
      let k = 0;
      while (k < words.length && 6.0 + k * 0.08 <= t) k++;
      ans.textContent = words.slice(0, k).join(' ') || ' ';
      tf(cite, kf(t, [[6.9, { o: 0, x: -10 }], [7.15, { o: 1, x: 0 }, 'outCubic']]));
      tf(grounded, kf(t, [[7.45, { o: 0, s: 0.6 }], [7.75, { o: 1, s: 1 }, 'outBack']]));

      if (t > 6.9) {
        const c = offsetIn(cite.querySelector('.p'), chat);
        const cx = 120 + c.x + c.w + 14, cy = 360 + c.y + c.h / 2;
        wire2.setAttribute('d', `M ${cx} ${cy} C ${cx + 200} ${cy}, ${bx - 200} ${by}, ${bx - 4} ${by}`);
      }
      drawPath(wire2, P(t, 7.0, 0.5, E.inOutCubic), 1000);
      wire2.style.opacity = (1 - P(t, OUT, 0.4, E.linear)).toFixed(3);

      const pp = (t - 3.65) / 1.1;
      if (pp > 0 && pp < 1 && t < 4.8) {
        const L = wire.getTotalLength();
        const pt = wire.getPointAtLength(L * (1 - E.inOutCubic(pp)));
        pulse.style.transform = `translate(${pt.x}px, ${pt.y}px)`;
        pulse.style.opacity = Math.sin(pp * Math.PI).toFixed(3);
      } else pulse.style.opacity = 0;
      tf(foot, kf(t, [[8.0, { o: 0, y: 10 }], [8.4, { o: 1, y: 0 }, 'outCubic'], [OUT, { o: 1, y: 0 }], [OUT + 0.4, { o: 0 }]]));
    },
  });
}

// ------------------------------------------------------------ 42–50 s  CONNECT
function connect(ctx) {
  const OUTC = 7.35;
  const SATS = [
    { k: 'mcp', n: 'Claude Code', x: -580, y: -190 },
    { k: 'mcp', n: 'Codex', x: -660, y: 40 },
    { k: 'mcp', n: 'OpenCode', x: -500, y: 250 },
    { k: 'mcp', n: 'any MCP client', x: 520, y: 250 },
    { k: 'webmcp', n: 'browser agent', x: 650, y: 30, two: true },
    { k: 'okn view', n: 'your team', x: 560, y: -190, two: true },
  ];
  let title, hub, ring1, ring2, wires, sats = [], cfg;

  ctx.add({
    id: 'connect',
    post: 0.05,
    build(root, s) {
      title = titleBlock(root, [S('', 'One wiki. '), S('acc', 'Every agent.')], 'Switch tools. Keep your knowledge.', { outAt: OUTC });
      wires = svg('svg', { class: 'wire', width: 1920, height: 1080 }, root);
      ring1 = el('div', 'hub-ring', root);
      ring2 = el('div', 'hub-ring', root);
      hub = el('div', 'hub', root, '<img src="assets/logo-mark.png" alt=""><span>Wiki/</span>');
      SATS.forEach((sat, i) => {
        const node = el('div', 'sat', root);
        el('div', 'sat-in', node, `<div class="sat-k">${sat.k}</div><div class="sat-n">${esc(sat.n)}</div>`);
        const path = svg('path', { fill: 'none', stroke: '#2c3d57', 'stroke-width': 2 }, wires);
        const dots = [0, 1, 2].map(() => el('div', 'pulse-dot', root));
        sats.push({ ...sat, node, path, dots, i });
        s.cue('pop', 0.9 + i * 0.13, { v: 0.45, p: i });
      });
      cfg = el('div', 'card cfg', root, [
        '<span class="x-str">"project-knowledge"</span><span class="x-pun">: {</span>',
        '  <span class="x-key x-cmd">"command"</span><span class="x-pun">:</span> <span class="x-str">"okn"</span><span class="x-pun">,</span>',
        '  <span class="x-cmd">"args"</span><span class="x-pun">: [</span><span class="x-str">"mcp"</span><span class="x-pun">,</span> <span class="x-str">"/path/to/Wiki"</span><span class="x-pun">]</span>',
        '<span class="x-pun">}</span>',
      ].join('\n'));
      s.cue('boom', 0.35, { v: 0.35 });
      s.cue('pop', 3.3, { v: 0.4, p: 7 });
      s.cue('whoosh', OUTC, { d: 0.6, v: 0.45 });
    },
    update(t) {
      title.update(t);
      const hp = kf(t, [[0.3, { o: 0, s: 0.4 }], [0.9, { o: 1, s: 1 }, 'outBack'], [OUTC, { o: 1, s: 1 }], [OUTC + 0.5, { o: 0, s: 0.7 }, 'inCubic']]);
      tf(hub, { ...hp, s: hp.s * (1 + Math.sin(t * 3) * 0.01) });
      [ring1, ring2].forEach((r, j) => {
        const ph = ((t + j * 0.9) % 1.8) / 1.8;
        tf(r, { s: 1 + ph * 1.6, o: (1 - ph) * 0.6 * hp.o });
      });
      const orbit = (t - 4) * 0.012;
      for (const sat of sats) {
        const ang = Math.atan2(sat.y, sat.x) + orbit;
        const rad = Math.hypot(sat.x, sat.y);
        const x = 960 + Math.cos(ang) * rad, y = 640 + Math.sin(ang) * rad;
        const a = kf(t, [[0.9 + sat.i * 0.13, { o: 0, s: 0.5 }], [1.35 + sat.i * 0.13, { o: 1, s: 1 }, 'outBack'], [OUTC, { o: 1, s: 1 }], [OUTC + 0.45, { o: 0, s: 0.8 }]]);
        tf(sat.node, { x, y, s: a.s, o: a.o });
        const hx = 960 + (x - 960) * 0.17, hy = 640 + (y - 640) * 0.3;
        const d = `M ${hx.toFixed(1)} ${hy.toFixed(1)} Q ${(960 + (x - 960) * 0.6).toFixed(1)} ${(640 + (y - 640) * 0.2).toFixed(1)} ${x.toFixed(1)} ${y.toFixed(1)}`;
        sat.path.setAttribute('d', d);
        sat.path._len = undefined;
        drawPath(sat.path, P(t, 0.75 + sat.i * 0.13, 0.55, E.inOutCubic));
        sat.path.style.opacity = a.o;
        const L = sat.path.getTotalLength();
        sat.dots.forEach((dot, j) => {
          const per = 1.3;
          let ph = ((t - 1.6 - sat.i * 0.17 - j * per / 3) % per) / per;
          if (t < 1.6 + sat.i * 0.17 + j * per / 3) ph = -1;
          const back = sat.two && j === 1;
          if (ph < 0 || t > OUTC) {
            dot.style.opacity = 0;
            return;
          }
          const pt = sat.path.getPointAtLength(L * (back ? 1 - ph : ph));
          dot.style.transform = `translate(${pt.x.toFixed(1)}px, ${pt.y.toFixed(1)}px) scale(${back ? 0.8 : 1})`;
          dot.style.opacity = (Math.sin(ph * Math.PI) * 0.95).toFixed(3);
          dot.style.background = back ? '#48dd78' : '#9ac2ff';
        });
      }
      tf(cfg, kf(t, [[3.3, { o: 0, y: 20 }], [3.8, { o: 1, y: 0 }, 'outCubic'], [OUTC, { o: 1, y: 0 }], [OUTC + 0.4, { o: 0, y: 20 }]]));
    },
  });
}

// ------------------------------------------------------------ 50–60 s  UPDATE
function update(ctx) {
  const ADD = 'Honor the Retry-After header on HTTP 429 responses.';
  const ICON = {
    doc: '<svg viewBox="0 0 24 24"><path d="M6 3h8l4 4v14H6z"/><path d="M14 3v4h4"/></svg>',
    graph: '<svg viewBox="0 0 24 24"><path d="M3 11l18-8-8 18-2-8z"/></svg>',
    db: '<svg viewBox="0 0 24 24"><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/></svg>',
  };
  let title, viewer, status, btns, ins, diff, pointer, steps, git, gitLine, typed;
  let geo = null;

  ctx.add({
    id: 'update',
    post: 0.05,
    build(root, s) {
      title = titleBlock(root, [S('', 'Your agent proposes. '), S('acc', 'You decide.')], 'Edit together in the local viewer. Review the diff. Then save.', { outAt: OUT });
      viewer = place(el('div', 'viewer', root), 120, 318, 1180, 650);
      viewer.style.transformOrigin = '50% 100%';
      const side = el('div', 'v-side', viewer);
      el('div', 'v-nav on', side, `${ICON.doc} Documents`);
      el('div', 'v-nav', side, `${ICON.graph} Graph`);
      el('div', 'v-nav', side, `${ICON.db} Knowledge bases`);
      el('div', 'v-tree', side, [
        '<div class="v-item dir">⌄ imported</div>',
        '<div class="v-item v-sub">overview.md</div>',
        '<div class="v-item v-sub on">delivery.md</div>',
        '<div class="v-item v-sub">operations.md</div>',
        '<div class="v-item">index.md<span class="v-tag">system</span></div>',
        '<div class="v-item">log.md<span class="v-tag">system</span></div>',
      ].join(''));
      const main = el('div', 'v-main', viewer);
      el('div', 'v-top', main, '<span class="v-kbd">⌘⌥S</span> Wiki <div class="v-search">⌕&nbsp;&nbsp;Search</div>');
      const doc = el('div', 'v-doc', main);
      el('div', 'v-dochead', doc, 'imported / <b>delivery</b><span style="margin-left:auto">Details · 2 fields</span>');
      const bar = el('div', 'v-toolbar', doc);
      status = el('div', 'v-status', bar, 'Saved');
      btns = ['Undo', 'Redo', 'Changes', 'Save'].map((b) => el('div', `v-btn${b === 'Save' ? ' save' : ''}`, bar, b));
      const content = el('div', 'v-content', doc);
      content.innerHTML = '<h1>Why we retry webhooks</h1><h2>Decision</h2><p>Retry failed deliveries for 24 hours with exponential backoff.</p><p>A receiver can be unavailable during a deploy.</p>';
      ins = el('p', '', content);
      diff = el('div', 'v-diff', doc);
      diff.style.bottom = '64px';
      diff.innerHTML = [
        '<span class="hdr">@@ -10,0 +11,2 @@</span>',
        '  A receiver can be unavailable during a deploy.',
        '<span class="add">+</span>',
        `<span class="add">+${esc(ADD)}</span>`,
      ].join('\n');
      pointer = el('div', 'pointer', root, '<svg viewBox="0 0 34 40" width="34" height="40"><path d="M3 2 L3 31 L10.5 24 L15.5 36 L21 33.6 L16 22 L26 22 Z" fill="#fff" stroke="#0b1220" stroke-width="2" stroke-linejoin="round"/></svg>');

      steps = place(el('div', 'steps', root), 1350, 360);
      ['agent edits the draft', 'you review the diff', 'save writes Markdown', 'history stays in Git'].forEach((txt, i) => {
        el('div', 'step', steps, `<span class="num">${i + 1}</span>${txt}`);
      });
      git = place(el('div', 'term', root), 1350, 712, 450);
      el('div', 'term-bar', git, '<i></i><i></i><i></i><span>~/relay</span>');
      gitLine = el('div', 'term-body', git);
      gitLine.style.fontSize = '18px';

      typed = typing(ADD, s.start + 1.7, { cps: 25, seed: 77, vol: 0.55 });
      s.cue('pop', 1.25, { v: 0.4, p: 2 });
      s.cue('whoosh', 4.15, { d: 0.45, v: 0.4, up: 1 });
      s.cue('click', 6.25);
      s.cue('success', 6.35);
      typing('git diff --stat', s.start + 7.05, { cps: 34, seed: 91, vol: 0.6 });
      s.cue('line', 7.6);
      s.cue('whoosh', OUT, { d: 0.6, v: 0.45 });
    },
    update(t) {
      title.update(t);
      tf(viewer, kf(t, [
        [0.3, { o: 0, y: 90, rx: 14, s: 0.96 }],
        [1.2, { o: 1, y: 0, rx: 0, s: 1 }, 'outQuart'],
        [OUT, { o: 1, y: 0, s: 1 }],
        [OUT + 0.5, { o: 0, y: 40, s: 0.97, blur: 8 }, 'inCubic'],
      ]));
      viewer.parentNode.style.perspective = '1600px';

      const abs = t + 50;
      const n = typed.count(abs);
      const typingNow = abs < typed.end + 0.1;
      const saved = t >= 6.3;
      const last = n ? typed.times[n - 1] - 50 : -1;
      const caret = t >= 1.25 && !saved && (typingNow || blink(t, last));
      const tagO = P(t, 1.25, 0.3) * (1 - P(t, 6.3, 0.2));
      const hiO = 1 - P(t, 6.5, 0.6, E.linear);
      ins.innerHTML = `<span class="v-ins" style="background: rgba(72,221,120,${(0.18 * hiO).toFixed(3)}); box-shadow: inset 2px 0 0 rgba(31,157,85,${hiO.toFixed(3)})">${esc(ADD.slice(0, n))}</span>` +
        (t >= 1.25 && !saved ? `<span class="v-caret" style="background:${caret ? "#2878ff" : "transparent"}"><span class="v-agent-tag" style="opacity:${tagO.toFixed(3)}">agent · WebMCP</span></span>` : '');
      ins.style.display = t >= 1.25 ? 'block' : 'none';

      const dirty = t >= 1.7 && !saved;
      status.innerHTML = saved
        ? '<span style="color:#1f9d55">✓</span> Saved'
        : dirty
          ? '<span style="color:#d4920a">●</span> Agent changes · Not saved'
          : 'Saved';
      btns[0].className = `v-btn${dirty ? ' live' : ''}`;
      btns[2].className = `v-btn${dirty || (t > 4.1 && t < 6.5) ? ' live' : ''}`;
      btns[2].textContent = t > 4.1 && t < 6.5 ? 'Hide changes' : 'Changes';
      const press = P(t, 6.15, 0.08, E.linear) * (1 - P(t, 6.25, 0.12, E.linear));
      btns[3].className = `v-btn save${dirty ? ' live' : ''}`;
      btns[3].style.transform = `scale(${1 - press * 0.08})`;

      tf(diff, kf(t, [[4.1, { o: 0, y: 30 }], [4.5, { o: 1, y: 0 }, 'outCubic'], [6.45, { o: 1, y: 0 }], [6.85, { o: 0, y: 30 }, 'inCubic']]));

      if (!geo) {
        const b = offsetIn(btns[3], viewer);
        geo = { sx: 120 + b.x + b.w * 0.55, sy: 318 + b.y + b.h * 0.6 };
      }
      const mv = P(t, 5.2, 0.9, E.inOutCubic);
      const px = lerp(1240, geo.sx, mv) + Math.sin(mv * Math.PI) * -40;
      const py = lerp(930, geo.sy, mv) + Math.sin(mv * Math.PI) * 30;
      tf(pointer, { x: px - 4, y: py - 3, o: kf(t, [[5.05, { o: 0 }], [5.3, { o: 1 }], [7.2, { o: 1 }], [7.6, { o: 0 }]]).o, s: 1 - press * 0.12 });

      const stepAt = [1.4, 4.1, 6.25, 7.05];
      [...steps.children].forEach((node, i) => {
        const on = t >= stepAt[i] && (i === 3 ? t < 99 : t < stepAt[i + 1]);
        node.className = `step${on ? ' on' : t >= (stepAt[i + 1] ?? 99) ? ' done' : ''}`;
        if (t >= (stepAt[i + 1] ?? 99)) node.querySelector('.num').textContent = '✓';
        else node.querySelector('.num').textContent = String(i + 1);
        tf(node, kf(t, [[0.8 + i * 0.1, { o: 0, x: 24 }], [1.3 + i * 0.1, { o: 1, x: 0 }, 'outCubic'], [OUT, { o: 1, x: 0 }], [OUT + 0.4, { o: 0, x: 30 }]]));
      });

      tf(git, kf(t, [[6.8, { o: 0, y: 20 }], [7.1, { o: 1, y: 0 }, 'outCubic'], [OUT, { o: 1, y: 0 }], [OUT + 0.4, { o: 0, y: 20 }]]));
      const gt = 'git diff --stat';
      const gn = clamp(Math.floor((t - 7.05) * 34), 0, gt.length);
      const showOut = t >= 7.6;
      gitLine.innerHTML = `<span class="x-prompt">$ </span><span class="x-exe">${esc(gt.slice(0, gn))}</span>${!showOut ? '<span class="cursor"></span>' : ''}` +
        (showOut ? '\n <span class="x-path">Wiki/imported/delivery.md</span> | 2 <span class="o-ok">++</span>' : '\n ');
    },
  });
}

// ------------------------------------------------------------ 60–70 s  CHECK
function check(ctx) {
  let title, T, rows = {}, src, pr, prRows = [];
  const PAD = (name, w = 13) => name.padEnd(w, ' ');

  ctx.add({
    id: 'check',
    post: 0.05,
    build(root, s) {
      title = titleBlock(root, [S('', 'Catch stale knowledge '), S('acc', 'before it ships.')], 'Deterministic checks locally. Knowledge CI on every pull request.', { outAt: OUT });
      T = term(root, s, { x: 120, y: 318, w: 1000, title: '~/relay', size: 22 });
      T.cmd([S('x-exe', 'okn'), S('', ' '), S('x-cmd', 'check'), S('', ' '), S('x-path', 'Wiki')], 0.45, { cps: 26 });
      T.out('', 1.05);
      T.out('<span class="o-hi">Knowledge status</span>', 1.1, { cue: 'line' });
      T.out('', 1.15);
      const layers = [
        ['Structure', '6 Markdown files validated.'],
        ['Links', 'Local Markdown links resolve.'],
        ['Freshness', 'No audit findings.'],
        ['Retrieval', '12/12 cases passed.'],
        ['Publication', 'Content checks passed.'],
      ];
      layers.forEach(([name, msg], i) => {
        rows[name] = { line: T.out('', 1.35 + i * 0.24, { cue: 'ding', cueOpts: { p: i } }), name, msg };
      });
      T.out('', 2.6);
      rows.Overall = { line: T.out('', 2.75, { cue: 'ding', cueOpts: { p: 6, v: 1.2 } }), name: 'Overall', msg: '' };

      src = place(el('div', 'card src-card', root), 1160, 318, 640);
      el('div', 'cc-head', src, '<b>src/relay/retry.ts</b><span class="chip a" style="font-size:16px;padding:6px 10px">source changed</span>');
      src.querySelector('.cc-head').style.cssText = 'display:flex;justify-content:space-between;align-items:center;padding:12px 20px;border-bottom:1px solid #161c25;font:500 18px/1 var(--mono);color:var(--quiet)';
      src.querySelector('.cc-head b').style.cssText = 'color:#e6c18c;font-weight:500';
      el('div', 'sc-body', src, '<span class="del">- const RETRY_WINDOW = hours(24)</span><span class="addl">+ const RETRY_WINDOW = hours(72)</span>');

      pr = place(el('div', 'card', root), 1160, 540, 640);
      el('div', 'pr-title', pr, '<span class="o-hi">⑂</span>  docs: update retry window<span class="o-dim">#128</span>');
      el('div', 'pr-meta', pr, 'opened by a maintenance job · 1 file changed');
      [
        ['✓', 'o-ok', 'Open Knowledge checks', 'passed'],
        ['✓', 'o-ok', 'eval · regressions gate', '0 regressed'],
        ['✓', 'o-ok', 'evidence pinned', 'retry.ts'],
        ['●', 'o-warn', 'claim change', 'needs approval'],
      ].forEach(([ic, cls, k, v], i) => {
        const row = el('div', 'pr-row', pr, `<span class="ic ${cls}">${ic}</span><span class="k">${k}</span><span class="v">${v}</span>`);
        prRows.push(row);
        s.cue(i < 3 ? 'ding' : 'alert', 5.45 + i * 0.4, { p: i + 2, v: 0.8 });
      });

      s.cue('alert', 3.5);
      s.cue('error', 4.15, { v: 0.5 });
      s.cue('whoosh', 4.95, { d: 0.5, v: 0.4, up: 1 });
      s.cue('success', 7.45);
      s.cue('ding', 7.95, { p: 2 });
      s.cue('ding', 8.15, { p: 6, v: 1.2 });
      s.cue('whoosh', OUT, { d: 0.6, v: 0.45 });
    },
    update(t) {
      title.update(t);
      tf(T.root, panelIn(t, 0.3, OUT));
      T.update(t);
      const fresh = t >= 4.15 && t < 7.95 ? 'warn' : 'ok';
      const overall = t >= 4.25 && t < 8.15 ? 'warn' : 'ok';
      for (const key of ['Structure', 'Links', 'Freshness', 'Retrieval', 'Publication', 'Overall']) {
        const r = rows[key];
        let st = '<span class="o-ok">READY</span>', msg = r.msg, w = 17;
        if (key === 'Freshness' && fresh === 'warn') {
          st = '<span class="o-warn">NEEDS ATTENTION</span>';
          msg = '2 findings: 1 high, 1 medium, 0 low.';
        }
        if (key === 'Overall' && overall === 'warn') st = '<span class="o-warn">NEEDS ATTENTION</span>';
        const plain = st.replace(/<[^>]+>/g, '');
        r.line.innerHTML = `${PAD(key)}${st}${' '.repeat(Math.max(1, w - plain.length))}<span class="o-muted">${msg}</span>`;
        const flash = key === 'Freshness' ? Math.max(P(t, 4.15, 0.1) * (1 - P(t, 4.25, 0.8)), P(t, 7.95, 0.1) * (1 - P(t, 8.05, 0.8))) : 0;
        r.line.style.background = flash > 0 ? `rgba(${fresh === 'warn' ? '255,194,71' : '72,221,120'},${(flash * 0.16).toFixed(3)})` : '';
      }
      tf(src, kf(t, [[3.4, { o: 0, x: 40 }], [3.85, { o: 1, x: 0 }, 'outQuart'], [OUT, { o: 1, x: 0 }], [OUT + 0.45, { o: 0, x: 60 }]]));
      const glowA = P(t, 3.5, 0.2) * (1 - P(t, 3.7, 1.2));
      src.style.boxShadow = `0 30px 90px rgba(0,0,0,.6), 0 0 ${(60 * glowA).toFixed(0)}px rgba(255,194,71,${(0.5 * glowA).toFixed(3)})`;
      tf(pr, kf(t, [[4.95, { o: 0, y: 50 }], [5.5, { o: 1, y: 0 }, 'outQuart'], [OUT, { o: 1, y: 0 }], [OUT + 0.45, { o: 0, x: 60 }]]));
      prRows.forEach((row, i) => {
        tf(row, kf(t, [[5.45 + i * 0.4, { o: 0, x: 16 }], [5.75 + i * 0.4, { o: 1, x: 0 }, 'outCubic']]));
      });
      const approved = t >= 7.45;
      prRows[3].innerHTML = approved
        ? '<span class="ic o-ok">✓</span><span class="k">claim change</span><span class="v o-soft">approved by a maintainer</span>'
        : '<span class="ic o-warn">●</span><span class="k">claim change</span><span class="v o-warn">needs human approval</span>';
      const ap = P(t, 7.45, 0.1) * (1 - P(t, 7.55, 0.8));
      prRows[3].style.background = ap > 0 ? `rgba(72,221,120,${(ap * 0.18).toFixed(3)})` : '';
    },
  });
}

// ------------------------------------------------------------ 70–78 s  SHIP
function ship(ctx) {
  const OUTS = 7.35;
  const GENS = [
    { g: 'gen 12', h: 'passing', x: 330 },
    { g: 'gen 13', h: 'passing', x: 700 },
    { g: 'gen 14', h: 'passing', x: 1070 },
    { g: 'gen 15', h: 'degraded', x: 1440 },
  ];
  let title, T, outs = [], track, trackFill, gens = [], pill, arc, arcLabel, caption, wires;

  ctx.add({
    id: 'ship',
    post: 0.05,
    build(root, s) {
      title = titleBlock(root, [S('', 'Ship knowledge '), S('acc', 'like code.')], 'Publish only what passes. Every release is immutable and reversible.', { outAt: OUTS });
      T = term(root, s, { x: 120, y: 318, w: 790, title: '~/relay' });
      T.cmd([S('x-exe', 'okn'), S('', ' '), S('x-cmd', 'publish'), S('', ' '), S('x-path', 'Wiki'), S('', ' '), S('x-flag', '--target'), S('', ' '), S('x-arg', 'viewer'), S('', ' '), S('x-flag', '--out'), S('', ' '), S('x-path', './site')], 0.4, { cps: 34 });
      T.out('<span class="o-hi">Publication plan</span>', 1.85, { cue: 'line' });
      T.out('  VIEWER   6 files -> ./site', 2.0, { cue: 'line' });
      T.out('<span class="o-ok">OK</span> Exported HTML', 2.25, { cue: 'success' });
      T.out('wrote 14 files', 2.4, { cue: 'line' });

      [
        ['index.html', 'static viewer'],
        ['llms.txt', 'agent-readable index'],
        ['openknowledge.json', 'connect manifest'],
        ['MCP', 'production runtime · optional'],
      ].forEach(([n, d], i) => {
        const card = place(el('div', 'card out-card', root), 960 + (i % 2) * 440, 318 + Math.floor(i / 2) * 140);
        el('div', 'on', card, n);
        el('div', 'od', card, d);
        outs.push({ card, i });
        s.cue('pop', 2.5 + i * 0.14, { v: 0.45, p: i + 2 });
      });

      wires = svg('svg', { class: 'wire', width: 1920, height: 1080 }, root);
      track = svg('path', { d: 'M 200 754 L 1720 754', stroke: '#222a35', 'stroke-width': 2, fill: 'none' }, wires);
      trackFill = svg('path', { d: 'M 330 754 L 1440 754', stroke: '#66a0ff', 'stroke-width': 2, fill: 'none', opacity: 0.6 }, wires);
      arc = svg('path', { d: 'M 1440 772 C 1440 880, 1070 880, 1070 776', stroke: '#66a0ff', 'stroke-width': 2.5, fill: 'none', 'stroke-dasharray': '1', 'marker-end': '' }, wires);
      GENS.forEach((g, i) => {
        const node = place(el('div', 'gen', root), g.x, 668);
        el('div', 'gl', node, g.g);
        const dot = el('div', 'gd', node);
        const chip = el('div', `chip ${g.h === 'passing' ? 'g' : 'a'}`, node, g.h);
        gens.push({ ...g, node, dot, chip, i });
      });
      pill = el('div', 'active-pill', root, '▾ active');
      arcLabel = place(el('div', 'chip b', root, '↶ rollback'), 1180, 846);
      caption = place(el('div', 'caption', root), 120, 940);
      caption.innerHTML = 'immutable generations  <b>·</b>  explicit health  <b>·</b>  rollback in one command';

      [3.5, 3.85, 4.2, 4.55].forEach((at, i) => s.cue('tick', at, { v: 0.7 }));
      s.cue('alert', 4.85);
      s.cue('beam', 5.35, { d: 0.6 });
      s.cue('success', 6.05);
      s.cue('whoosh', OUTS, { d: 0.6, v: 0.45 });
    },
    update(t) {
      title.update(t);
      tf(T.root, panelIn(t, 0.25, OUTS));
      T.update(t);
      for (const o of outs) {
        tf(o.card, kf(t, [[2.5 + o.i * 0.14, { o: 0, x: -120, s: 0.9, blur: 8 }], [3.0 + o.i * 0.14, { o: 1, x: 0, s: 1, blur: 0 }, 'outQuart'], [OUTS, { o: 1, x: 0 }], [OUTS + 0.45, { o: 0, x: 60 }]]));
      }
      const tl = kf(t, [[3.1, { o: 0 }], [3.4, { o: 1 }], [OUTS, { o: 1 }], [OUTS + 0.45, { o: 0 }]]).o;
      track.style.opacity = tl;
      drawPath(trackFill, P(t, 3.5, 1.05, E.inOutCubic), 1110);
      trackFill.style.opacity = 0.7 * tl;
      for (const g of gens) {
        const at = 3.5 + g.i * 0.35;
        tf(g.node, kf(t, [[at - 0.15, { o: 0, y: 14 }], [at + 0.2, { o: 1, y: 0 }, 'outBack'], [OUTS, { o: 1, y: 0 }], [OUTS + 0.45, { o: 0 }]]));
        const lit = t >= at;
        const degraded = g.h === 'degraded';
        g.dot.style.background = lit ? (degraded ? '#ffc247' : '#66a0ff') : '#0a0e14';
        g.dot.style.borderColor = lit ? (degraded ? '#ffc247' : '#9ac2ff') : '#3a4658';
        g.dot.style.boxShadow = lit ? `0 0 ${degraded ? 26 : 16}px ${degraded ? 'rgba(255,194,71,.7)' : 'rgba(102,160,255,.6)'}` : 'none';
        tf(g.chip, kf(t, [[at + (degraded ? 0.3 : 0.1), { o: 0, s: 0.7 }], [at + (degraded ? 0.6 : 0.4), { o: 1, s: 1 }, 'outBack']]));
      }
      // Active pointer: walks forward, then rolls back from 15 to 14.
      const fw = clamp((t - 3.5) / 1.05);
      const idx = Math.min(3, Math.floor(fw * 3 + 0.0001));
      let ax = lerp(330, 1440, E.inOutCubic(fw));
      const back = P(t, 5.45, 0.6, E.inOutCubic);
      ax = lerp(ax, 1070, back);
      const ay = 620 - Math.sin(back * Math.PI) * 30;
      tf(pill, { x: ax, y: ay, o: kf(t, [[3.4, { o: 0 }], [3.6, { o: 1 }], [OUTS, { o: 1 }], [OUTS + 0.4, { o: 0 }]]).o });
      drawPath(arc, P(t, 5.3, 0.6, E.inOutCubic), 520);
      arc.style.opacity = (1 - P(t, OUTS, 0.4, E.linear)).toFixed(3);
      tf(arcLabel, kf(t, [[5.55, { o: 0, s: 0.7 }], [5.85, { o: 1, s: 1 }, 'outBack'], [OUTS, { o: 1, s: 1 }], [OUTS + 0.4, { o: 0 }]]));
      tf(caption, kf(t, [[6.2, { o: 0, y: 10 }], [6.6, { o: 1, y: 0 }, 'outCubic'], [OUTS, { o: 1, y: 0 }], [OUTS + 0.4, { o: 0 }]]));
    },
  });
}
