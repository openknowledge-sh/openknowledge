#!/usr/bin/env node
// Procedural score and sound design for the explainer.
// Reads film/out/cues.json (exported by the animation) and writes film/out/soundtrack.wav.
// 120 BPM, D major. Everything is synthesized here: no samples, no external assets.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const ROOT = path.dirname(fileURLToPath(import.meta.url));
const { duration: DUR, cues } = JSON.parse(fs.readFileSync(path.join(ROOT, 'out', 'cues.json'), 'utf8'));
const SR = 48000;
const N = Math.ceil(DUR * SR);
const BEAT = 0.5;

// ------------------------------------------------------------ buses
const bus = () => [new Float32Array(N), new Float32Array(N)];
const MUSIC = bus(), PAD = bus(), DRUMS = bus(), SFX = bus(), REV = bus(), DLY = bus();

function put(b, i, v, pan = 0) {
  if (i < 0 || i >= N) return;
  const a = ((pan + 1) * Math.PI) / 4;
  b[0][i] += v * Math.cos(a);
  b[1][i] += v * Math.sin(a);
}
// Write to a bus with reverb/delay sends.
function emit(b, i, v, pan, rev = 0, dly = 0) {
  put(b, i, v, pan);
  if (rev) put(REV, i, v * rev, pan);
  if (dly) put(DLY, i, v * dly, pan);
}

// ------------------------------------------------------------ dsp
let seed = 0x2f6e2b1;
const rnd = () => {
  seed |= 0;
  seed = (seed + 0x6d2b79f5) | 0;
  let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const noise = () => rnd() * 2 - 1;
const mtof = (m) => 440 * Math.pow(2, (m - 69) / 12);
const idx = (t) => Math.round(t * SR);

function blep(t, dt) {
  if (t < dt) {
    t /= dt;
    return t + t - t * t - 1;
  }
  if (t > 1 - dt) {
    t = (t - 1) / dt;
    return t * t + t + t + 1;
  }
  return 0;
}

class SVF {
  constructor() {
    this.ic1 = 0;
    this.ic2 = 0;
  }
  set(cut, q = 0.707) {
    const g = Math.tan((Math.PI * Math.min(Math.max(cut, 20), SR * 0.45)) / SR);
    this.k = 1 / q;
    this.a1 = 1 / (1 + g * (g + this.k));
    this.a2 = g * this.a1;
    this.a3 = g * this.a2;
    return this;
  }
  run(v0) {
    const v3 = v0 - this.ic2;
    const v1 = this.a1 * this.ic1 + this.a2 * v3;
    const v2 = this.ic2 + this.a2 * this.ic1 + this.a3 * v3;
    this.ic1 = 2 * v1 - this.ic1;
    this.ic2 = 2 * v2 - this.ic2;
    this.lp = v2;
    this.bp = v1;
    this.hp = v0 - this.k * v1 - v2;
    return v2;
  }
}

const adsr = (t, dur, a, d, s, r) => {
  if (t < 0) return 0;
  if (t < a) return t / a;
  if (t < a + d) return 1 - ((t - a) / d) * (1 - s);
  if (t < dur) return s;
  if (t < dur + r) return s * (1 - (t - dur) / r);
  return 0;
};

// ------------------------------------------------------------ instruments
function pad(notes, t0, dur, { vol = 0.06, cut = 1800, cutEnd = cut, q = 0.8, a = 0.8, r = 1.6, detune = 9, bus: b = PAD, rev = 0.5 } = {}) {
  const len = dur + r;
  notes.forEach((m, ni) => {
    const f = mtof(m);
    for (let v = 0; v < 3; v++) {
      const cents = (v - 1) * detune + (ni % 2 ? 2 : -2);
      const fv = f * Math.pow(2, cents / 1200);
      const dt = fv / SR;
      const pan = (v - 1) * 0.55 + (ni / notes.length - 0.5) * 0.3;
      const flt = new SVF();
      let ph = rnd();
      const i0 = idx(t0);
      const n = Math.floor(len * SR);
      for (let k = 0; k < n; k++) {
        const t = k / SR;
        if (k % 32 === 0) flt.set((cut + (cutEnd - cut) * Math.min(1, t / dur)) * (1 + 0.12 * Math.sin(t * 0.9 + ni)), q);
        ph += dt;
        if (ph >= 1) ph -= 1;
        const s = 2 * ph - 1 - blep(ph, dt);
        const env = adsr(t, dur, a, 0.01, 1, r);
        emit(b, i0 + k, flt.run(s) * env * vol / 3, pan, rev);
      }
    }
  });
}

function bass(m, t0, dur, vol = 0.22) {
  const f = mtof(m);
  const flt = new SVF();
  let ph = 0, ph2 = 0;
  const i0 = idx(t0);
  const n = Math.floor((dur + 0.08) * SR);
  for (let k = 0; k < n; k++) {
    const t = k / SR;
    if (k % 16 === 0) flt.set(180 + 900 * Math.exp(-t * 14), 1.1);
    ph += f / SR;
    ph2 += f / SR;
    if (ph2 >= 1) ph2 -= 1;
    const saw = 2 * ph2 - 1 - blep(ph2, f / SR);
    const sub = Math.sin(2 * Math.PI * ph);
    const env = Math.min(1, t / 0.004) * (t < dur ? 1 : Math.max(0, 1 - (t - dur) / 0.08));
    const v = Math.tanh((sub * 0.9 + flt.run(saw) * 0.5) * 1.4) * env * vol;
    put(MUSIC, i0 + k, v, 0);
  }
}

function pluck(m, t0, { vol = 0.05, pan = 0, decay = 0.994, bright = 0.5, rev = 0.35, dly = 0.3, len = 1.3 } = {}) {
  const f = mtof(m);
  const L = Math.max(2, Math.round(SR / f));
  const buf = new Float32Array(L);
  const pre = new SVF().set(800 + bright * 6000, 0.7);
  for (let k = 0; k < L; k++) buf[k] = pre.run(noise());
  let p = 0, last = 0;
  const i0 = idx(t0);
  const n = Math.floor(len * SR);
  for (let k = 0; k < n; k++) {
    const v = buf[p];
    const nv = (v + last) * 0.5 * decay;
    last = v;
    buf[p] = nv;
    p = (p + 1) % L;
    const env = Math.min(1, k / 30) * (k > n - 2400 ? (n - k) / 2400 : 1);
    emit(MUSIC, i0 + k, v * vol * env, pan, rev, dly);
  }
}

function kick(t0, vol = 0.5) {
  let ph = 0;
  const i0 = idx(t0);
  const hp = new SVF().set(3000, 0.7);
  for (let k = 0; k < SR * 0.45; k++) {
    const t = k / SR;
    const f = 46 + 120 * Math.exp(-t * 28);
    ph += f / SR;
    const body = Math.sin(2 * Math.PI * ph) * Math.exp(-t * 7.5);
    hp.run(noise());
    const click = hp.hp * Math.exp(-t * 400) * 0.35;
    put(DRUMS, i0 + k, Math.tanh((body + click) * 1.6) * vol, 0);
  }
}

function hat(t0, vol = 0.05, open = false, pan = 0.25) {
  const f = new SVF().set(8000, 0.9);
  const i0 = idx(t0);
  const dec = open ? 11 : 55;
  for (let k = 0; k < SR * (open ? 0.4 : 0.08); k++) {
    const t = k / SR;
    f.run(noise());
    put(DRUMS, i0 + k, f.hp * Math.exp(-t * dec) * vol, pan);
  }
}

function clap(t0, vol = 0.16) {
  const f = new SVF().set(1500, 1.4);
  const i0 = idx(t0);
  for (let k = 0; k < SR * 0.3; k++) {
    const t = k / SR;
    const burst = t < 0.03 ? (Math.floor(t / 0.009) % 2 === 0 ? 1 : 0.4) * Math.exp(-(t % 0.009) * 300) : 0;
    const env = burst + Math.exp(-Math.max(0, t - 0.02) * 22) * (t > 0.02 ? 0.7 : 0);
    f.run(noise());
    emit(DRUMS, i0 + k, f.bp * env * vol, 0, 0.35);
  }
}

function crash(t0, vol = 0.05) {
  const f = new SVF().set(6000, 0.6);
  const i0 = idx(t0);
  for (let k = 0; k < SR * 2.2; k++) {
    const t = k / SR;
    f.run(noise());
    emit(DRUMS, i0 + k, (f.hp * 0.7 + f.bp * 0.3) * Math.exp(-t * 2.1) * vol, noise() * 0.2, 0.4);
  }
}

// ------------------------------------------------------------ sound effects
function bell(f, t0, { vol = 0.08, dec = 2.2, ratio = 3.5, index = 2.2, pan = 0, rev = 0.45, len = 1.8 } = {}) {
  const i0 = idx(t0);
  for (let k = 0; k < SR * len; k++) {
    const t = k / SR;
    const mod = Math.sin(2 * Math.PI * f * ratio * t) * index * Math.exp(-t * 6);
    const v = Math.sin(2 * Math.PI * f * t + mod) * Math.exp(-t * dec) * Math.min(1, t / 0.002);
    emit(SFX, i0 + k, v * vol, pan, rev);
  }
}

function blipTone(f0, f1, t0, len, vol, pan = 0, rev = 0.2, shape = 'sine') {
  const i0 = idx(t0);
  let ph = 0;
  for (let k = 0; k < SR * len; k++) {
    const t = k / SR;
    const f = f0 + (f1 - f0) * Math.min(1, t / (len * 0.6));
    ph += f / SR;
    const s = shape === 'tri' ? 1 - 4 * Math.abs((ph % 1) - 0.5) : Math.sin(2 * Math.PI * ph);
    const env = Math.min(1, t / 0.003) * Math.exp(-t * (6 / len));
    emit(SFX, i0 + k, s * env * vol, pan, rev);
  }
}

function noiseSweep(t0, len, f0, f1, vol, { q = 1.2, pan0 = -0.6, pan1 = 0.6, shape = 'swell', rev = 0.3 } = {}) {
  const f = new SVF();
  const i0 = idx(t0);
  const n = Math.floor(len * SR);
  for (let k = 0; k < n; k++) {
    const x = k / n;
    if (k % 16 === 0) f.set(f0 * Math.pow(f1 / f0, x), q);
    f.run(noise());
    const env = shape === 'swell' ? Math.sin(Math.PI * x) ** 1.5 : shape === 'rise' ? x ** 2.2 : (1 - x) ** 2;
    emit(SFX, i0 + k, f.bp * env * vol, pan0 + (pan1 - pan0) * x, rev);
  }
}

const SCALE = [74, 76, 78, 81, 83, 86, 88, 90, 93, 95]; // D major pentatonic from D5

const FX = {
  key({ t, v = 1, s = 0.5, space }) {
    const f = new SVF().set(2200 + s * 2600, 1.6);
    const i0 = idx(t);
    const thock = 150 + s * 90;
    const pan = (s - 0.5) * 0.5;
    for (let k = 0; k < SR * 0.05; k++) {
      const tt = k / SR;
      f.run(noise());
      const v1 = f.bp * Math.exp(-tt * 900) * 0.9 + Math.sin(2 * Math.PI * thock * tt) * Math.exp(-tt * 90) * 0.5;
      put(SFX, i0 + k, v1 * 0.085 * v * (space ? 0.8 : 1), pan);
    }
  },
  tick({ t, v = 1 }) { blipTone(2400, 2300, t, 0.025, 0.035 * v, 0.2, 0.1); },
  line({ t }) { blipTone(1900, 1850, t, 0.018, 0.022, -0.15, 0.05); },
  count({ t, v = 1 }) { blipTone(3200, 3100, t, 0.01, 0.02 * v, 0.3, 0); },
  send({ t, v = 1 }) {
    blipTone(880, 900, t, 0.07, 0.06 * v, 0, 0.3);
    blipTone(1320, 1340, t + 0.06, 0.1, 0.05 * v, 0, 0.3);
  },
  pop({ t, v = 1, p = 0 }) {
    const f = mtof(SCALE[p % SCALE.length] - 12);
    blipTone(f * 1.5, f, t, 0.09, 0.07 * v, ((p % 5) - 2) * 0.2, 0.25);
  },
  blip({ t, v = 1, p = 0 }) { bell(mtof(SCALE[(p + 2) % SCALE.length]), t, { vol: 0.035 * v, dec: 9, index: 0.8, ratio: 2, len: 0.5 }); },
  ding({ t, p = 0, v = 1 }) { bell(mtof(SCALE[Math.min(p + 1, SCALE.length - 1)]), t, { vol: 0.055 * v, dec: 4.5, index: 1.6, len: 1.4, pan: (p - 3) * 0.12 }); },
  success({ t }) {
    bell(mtof(81), t, { vol: 0.05, dec: 4, index: 1.2, ratio: 2, len: 1.4 });
    bell(mtof(88), t + 0.09, { vol: 0.045, dec: 3.5, index: 1.2, ratio: 2, len: 1.6 });
  },
  alert({ t }) {
    blipTone(mtof(76), mtof(76), t, 0.12, 0.05, -0.1, 0.25, 'tri');
    blipTone(mtof(72), mtof(72), t + 0.13, 0.16, 0.05, 0.1, 0.25, 'tri');
  },
  error({ t, v = 1 }) {
    const i0 = idx(t);
    const f = new SVF().set(1400, 0.9);
    for (let k = 0; k < SR * 0.26; k++) {
      const tt = k / SR;
      const sq = Math.sign(Math.sin(2 * Math.PI * 98 * tt)) + Math.sign(Math.sin(2 * Math.PI * 104 * tt));
      const crush = Math.round(sq * 4) / 4;
      const env = Math.min(1, tt / 0.004) * (tt < 0.2 ? 1 : 1 - (tt - 0.2) / 0.06);
      emit(SFX, i0 + k, f.run(crush) * env * 0.075 * v, 0, 0.15);
    }
  },
  hit({ t }) { FX.boom({ t, v: 0.6 }); },
  boom({ t, v = 1 }) {
    const i0 = idx(t);
    let ph = 0;
    const f = new SVF().set(220, 0.7);
    for (let k = 0; k < SR * 2.6; k++) {
      const tt = k / SR;
      ph += (36 + 60 * Math.exp(-tt * 9)) / SR;
      f.run(noise());
      const v1 = Math.sin(2 * Math.PI * ph) * Math.exp(-tt * 1.6) + f.lp * Math.exp(-tt * 10) * 0.8;
      emit(SFX, i0 + k, Math.tanh(v1 * 1.3) * 0.42 * v, 0, 0.25);
    }
  },
  shimmer({ t }) {
    [86, 93, 98, 100, 105].forEach((m, i) => bell(mtof(m), t + i * 0.035, { vol: 0.022, dec: 1.1, index: 0.6, ratio: 2.01, len: 3.2, pan: (i - 2) * 0.35, rev: 0.9 }));
  },
  scramble({ t, d = 0.8, v = 1 }) {
    for (let x = 0; x < d; x += 0.032) blipTone(1400 + rnd() * 2600, 1400 + rnd() * 2600, t + x, 0.016, 0.018 * v * (1 - (x / d) * 0.5), noise() * 0.6, 0.1);
  },
  whoosh({ t, d = 0.6, v = 1, up }) {
    noiseSweep(t - d * 0.35, d, up ? 300 : 2600, up ? 3200 : 380, 0.11 * v, { q: 0.9 });
  },
  riser({ t, d = 2, v = 1 }) {
    noiseSweep(t, d, 250, 7000, 0.075 * v, { q: 1.6, shape: 'rise', pan0: 0, pan1: 0, rev: 0.4 });
    const i0 = idx(t);
    let ph = 0;
    for (let k = 0; k < SR * d; k++) {
      const x = k / (SR * d);
      ph += (mtof(50) * Math.pow(2, x * 1.5)) / SR;
      if (ph >= 1) ph -= 1;
      emit(SFX, i0 + k, (2 * ph - 1) * x ** 2.5 * 0.018 * v, 0, 0.4);
    }
  },
  suck({ t, d = 1.2 }) { noiseSweep(t, d, 6000, 120, 0.09, { q: 0.8, shape: 'rise', pan0: 0.7, pan1: 0, rev: 0.2 }); },
  beam({ t, d = 0.5 }) {
    const i0 = idx(t);
    let ph = 0;
    for (let k = 0; k < SR * d; k++) {
      const x = k / (SR * d);
      ph += (500 * Math.pow(3, x)) / SR;
      const env = Math.sin(Math.PI * x) * (0.6 + 0.4 * Math.sin(2 * Math.PI * 18 * x * d));
      emit(SFX, i0 + k, Math.sin(2 * Math.PI * ph) * env * 0.022, -0.5 + x, 0.4);
    }
  },
  click({ t }) {
    const f = new SVF().set(3500, 1);
    [0, 0.045].forEach((o, j) => {
      const i0 = idx(t + o);
      for (let k = 0; k < SR * 0.02; k++) {
        f.run(noise());
        put(SFX, i0 + k, f.hp * Math.exp(-(k / SR) * 500) * (j ? 0.07 : 0.11), 0.2);
      }
    });
  },
  stab() {},
  final() {},
};

// ------------------------------------------------------------ score
const CH = {
  D: { pad: [50, 57, 61, 64, 66], root: 38, arp: [62, 66, 69, 73, 76, 73, 69, 66] },
  Bm: { pad: [47, 54, 57, 61, 62], root: 35, arp: [59, 62, 66, 69, 73, 69, 66, 62] },
  G: { pad: [43, 50, 54, 57, 59], root: 31, arp: [55, 59, 62, 66, 69, 66, 62, 59] },
  A: { pad: [45, 52, 59, 61, 64], root: 33, arp: [57, 61, 64, 69, 71, 69, 64, 61] },
};
const PROG = ['D', 'Bm', 'G', 'A'];
const chordAt = (t) => CH[PROG[Math.floor((t - 22) / 4) % 4]];

// Act 1: dark drone, heartbeat, clock.
pad([38, 45, 50, 53, 57, 64], 0, 15.6, { vol: 0.07, cut: 260, cutEnd: 1300, a: 3, r: 0.25, q: 1.1, rev: 0.6 });
pad([62, 63], 9.5, 6.1, { vol: 0.012, cut: 1400, cutEnd: 3000, a: 4, r: 0.25, detune: 18 });
for (let t = 6; t < 15.6; t += 1) {
  const g = 0.16 + ((t - 6) / 10) * 0.22;
  kick(t, g);
  kick(t + 0.2, g * 0.55);
}
for (let t = 6; t < 15.8; t += t < 11 ? 0.5 : 0.25) hat(t + (t < 11 ? 0.25 : 0), 0.022 + (t - 6) * 0.002, false, 0.35);

// Reveal: bloom into D major.
pad(CH.D.pad.concat([69, 73]), 16, 4, { vol: 0.07, cut: 700, cutEnd: 3800, a: 0.35, r: 1.8, rev: 0.7 });
pad(CH.Bm.pad, 20, 2, { vol: 0.06, cut: 2600, cutEnd: 2600, a: 0.6, r: 1.2 });
for (let t = 17; t < 22; t += 0.25) {
  const c = t < 20 ? CH.D : CH.Bm;
  pluck(c.arp[Math.round((t - 17) / 0.25) % 8] + 12, t, { vol: 0.032 * Math.min(1, (t - 16.8) / 2), pan: Math.sin(t * 3) * 0.5, bright: 0.35 });
}

// Groove 22–78.
for (let t = 22; t < 78; t += 4) {
  const c = chordAt(t);
  pad(c.pad, t, 4, { vol: 0.05, cut: 1100 + (t - 22) * 18, cutEnd: 1500 + (t - 22) * 20, a: 0.25, r: 0.9 });
}
for (let t = 22; t < 78; t += BEAT) kick(t, 0.34);
for (let t = 22; t < 78; t += BEAT) hat(t + 0.25, 0.055);
for (let t = 42; t < 78; t += 0.25) if ((t * 4) % 2 === 0) hat(t + 0.125, 0.016, false, -0.3);
for (let t = 32.5; t < 78; t += 1) clap(t, 0.13);
for (let t = 32; t < 78; t += 0.25) {
  const c = chordAt(t);
  const up = t >= 60 && Math.round(t * 4) % 2 === 1 ? 12 : 0;
  bass(c.root + up, t, 0.2, 0.17);
}
for (let t = 22; t < 82.5; t += 0.125) {
  const c = chordAt(Math.min(t, 77.9));
  const step = Math.round((t - 22) / 0.125);
  pluck(c.arp[step % 8] + 12, t, {
    vol: (step % 2 ? 0.018 : 0.028) * (t < 60 ? 1 : 1.15),
    pan: Math.sin(step * 0.7) * 0.6,
    bright: 0.3 + Math.min(0.5, (t - 22) / 110),
    dly: 0.22,
    len: 0.9,
  });
}
[22, 42, 60, 78].forEach((t) => crash(t, 0.045));

// Loop section 78–82.5: keep the pulse, open the filter.
for (let t = 78; t < 82.5; t += 2) pad(CH[t < 80 ? 'G' : 'A'].pad, t, 2, { vol: 0.055, cut: 2400, cutEnd: 4200, a: 0.1, r: 0.6 });
for (let t = 78; t < 82.5; t += BEAT) kick(t, 0.36);
for (let t = 78; t < 82.5; t += 0.25) hat(t + 0.125, 0.03, false, (t * 4) % 2 ? 0.3 : -0.3);
for (let t = 78.5; t < 82.5; t += 1) clap(t, 0.13);
for (let t = 78; t < 82.5; t += 0.25) bass((t < 80 ? CH.G : CH.A).root + (Math.round(t * 4) % 2 ? 12 : 0), t, 0.2, 0.17);
for (let t = 84.5; t < 86; t += t < 85.25 ? 0.125 : 0.0625) clap(t, 0.04 + (t - 84.5) * 0.06);

// Value stabs 82.5–86.
const STABS = ['D', 'Bm', 'G', 'A', 'Bm', 'D'];
for (const c of cues.filter((q) => q.type === 'stab')) {
  const ch = CH[STABS[c.i % STABS.length]];
  kick(c.t, 0.44);
  pad(ch.pad.map((m) => m + 12), c.t, 0.18, { vol: 0.078, cut: 4200, cutEnd: 1200, a: 0.004, r: 0.32, rev: 0.45, bus: MUSIC });
  bass(ch.root, c.t, 0.3, 0.2);
}

// Finale 86–96.
const fin = cues.find((q) => q.type === 'final')?.t ?? 86;
FX.boom({ t: fin, v: 1 });
FX.shimmer({ t: fin + 0.02 });
crash(fin, 0.06);
pad(CH.D.pad.concat([69, 76]), fin, 8.4, { vol: 0.075, cut: 900, cutEnd: 3200, a: 0.12, r: 1.6, rev: 0.75 });
bass(CH.D.root, fin, 1.6, 0.14);
for (let t = fin + 0.5; t < 93.5; t += 0.25) {
  const step = Math.round((t - fin) / 0.25);
  pluck(CH.D.arp[step % 8] + 12, t, { vol: 0.026 * Math.max(0.2, 1 - (t - fin) / 9), pan: Math.sin(step) * 0.5, bright: 0.4, len: 1.6 });
}

// UI sound effects from cues.
for (const c of cues) FX[c.type]?.(c);

// ------------------------------------------------------------ effects + mix
// Sidechain pump on pads and plucks from the groove kicks.
const sc = new Float32Array(N).fill(1);
const kickTimes = [];
for (let t = 22; t < 82.5; t += BEAT) kickTimes.push(t);
for (const kt of kickTimes) {
  const i0 = idx(kt);
  for (let k = 0; k < SR * 0.45; k++) if (i0 + k < N) sc[i0 + k] = Math.min(sc[i0 + k], 1 - 0.45 * Math.exp(-(k / SR) * 9));
}

function freeverb(inp, room = 0.86, damp = 0.35, spread = 23) {
  const combs = [1557, 1617, 1491, 1422, 1277, 1356, 1188, 1116].map((n) => Math.round((n * SR) / 44100));
  const alls = [556, 441, 341, 225].map((n) => Math.round((n * SR) / 44100));
  const out = [new Float32Array(N), new Float32Array(N)];
  for (let ch = 0; ch < 2; ch++) {
    const x = inp[ch];
    const y = out[ch];
    const off = ch ? spread : 0;
    const cb = combs.map((n) => ({ buf: new Float32Array(n + off), p: 0, s: 0 }));
    const ab = alls.map((n) => ({ buf: new Float32Array(n + off), p: 0 }));
    for (let i = 0; i < N; i++) {
      const v = x[i] * 0.015;
      let acc = 0;
      for (const c of cb) {
        const o = c.buf[c.p];
        c.s = o * (1 - damp) + c.s * damp;
        c.buf[c.p] = v + c.s * room;
        c.p = (c.p + 1) % c.buf.length;
        acc += o;
      }
      for (const a of ab) {
        const o = a.buf[a.p];
        a.buf[a.p] = acc + o * 0.5;
        a.p = (a.p + 1) % a.buf.length;
        acc = o - acc;
      }
      y[i] = acc;
    }
  }
  return out;
}

function pingpong(inp, time = 0.375, fb = 0.38) {
  const d = Math.round(time * SR);
  const out = [new Float32Array(N), new Float32Array(N)];
  const lp = [new SVF().set(3200, 0.6), new SVF().set(3200, 0.6)];
  for (let i = d; i < N; i++) {
    const l = inp[0][i - d] + inp[1][i - d] * 0.5 + out[1][i - d] * fb;
    const r = out[0][i - d] * fb;
    out[0][i] = lp[0].run(l);
    out[1][i] = lp[1].run(r);
  }
  return out;
}

console.log('reverb + delay…');
const verb = freeverb(REV);
const delay = pingpong(DLY);

// ------------------------------------------------------------ voice-over
// film/vo/lines.json places cuts of one narration take on the film timeline.
const VO = new Float32Array(N);
const voFile = path.join(ROOT, 'vo', 'lines.json');
if (fs.existsSync(voFile)) {
  const vo = JSON.parse(fs.readFileSync(voFile, 'utf8'));
  for (const l of vo.lines) {
    const len = (l.to - l.from) / l.tempo;
    const af = [
      `atrim=start=${l.from}:end=${l.to}`,
      'asetpts=PTS-STARTPTS',
      `atempo=${l.tempo}`,
      'highpass=f=75',
      'afade=t=in:d=0.015',
      `afade=t=out:st=${Math.max(0, len - 0.09).toFixed(3)}:d=0.09`,
    ].join(',');
    const r = spawnSync('ffmpeg', ['-v', 'error', '-i', path.join(ROOT, 'vo', vo.take), '-af', af, '-ac', '1', '-ar', String(SR), '-f', 'f32le', '-'], { maxBuffer: 1 << 28 });
    if (r.status !== 0) throw new Error(`voice line ${l.line}: ${r.stderr}`);
    const buf = Buffer.from(r.stdout);
    const pcm = new Float32Array(buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength));
    const i0 = idx(l.at);
    for (let k = 0; k < pcm.length && i0 + k < N; k++) VO[i0 + k] += pcm[k];
  }
  // Level the voice so active speech sits well above the music.
  let vs = 0, vn = 0;
  for (let i = 0; i < N; i++) if (Math.abs(VO[i]) > 0.01) (vs += VO[i] * VO[i]), vn++;
  const voGain = 0.16 / Math.sqrt(vs / Math.max(1, vn));
  for (let i = 0; i < N; i++) VO[i] *= voGain;
  console.log(`voice-over: ${vo.lines.length} lines, gain ${voGain.toFixed(2)}`);
}

// Duck music and effects under the voice, with a short look-ahead.
const duck = new Float32Array(N).fill(1);
const duckFx = new Float32Array(N).fill(1);
{
  const LA = Math.round(0.06 * SR);
  const att = Math.exp(-1 / (0.015 * SR)), rel = Math.exp(-1 / (0.4 * SR));
  let env = 0;
  for (let i = 0; i < N; i++) {
    const x = Math.abs(VO[Math.min(N - 1, i + LA)]);
    env = x > env ? att * env + (1 - att) * x : rel * env + (1 - rel) * x;
    const d = Math.min(1, env / 0.04);
    duck[i] = 1 - 0.6 * d;
    duckFx[i] = 1 - 0.4 * d;
  }
}

const master = [new Float32Array(N), new Float32Array(N)];
for (let ch = 0; ch < 2; ch++) {
  const hp = new SVF().set(28, 0.7);
  for (let i = 0; i < N; i++) {
    const t = i / SR;
    let v = (PAD[ch][i] * sc[i] + MUSIC[ch][i] * (0.55 + 0.45 * sc[i]) + DRUMS[ch][i] + verb[ch][i] + delay[ch][i] * 0.55) * duck[i] + SFX[ch][i] * duckFx[i];
    // Silence beat before the reveal.
    if (t > 15.86 && t < 16) v *= Math.max(0, 1 - (t - 15.86) / 0.04);
    v += VO[i] * 0.5;
    // Fade the tail.
    if (t > DUR - 1.8) v *= Math.max(0, (DUR - t) / 1.8) ** 1.5;
    hp.run(v);
    master[ch][i] = hp.hp;
  }
}

// Normalize and soft-limit.
let peak = 0, sum = 0;
for (let ch = 0; ch < 2; ch++) for (let i = 0; i < N; i++) {
  peak = Math.max(peak, Math.abs(master[ch][i]));
  sum += master[ch][i] ** 2;
}
const rms = Math.sqrt(sum / (2 * N));
const gain = Math.min(0.13 / rms, 1.0 / peak);
for (let ch = 0; ch < 2; ch++) for (let i = 0; i < N; i++) master[ch][i] = Math.tanh(master[ch][i] * gain) * 0.93;
console.log(`peak ${peak.toFixed(3)}  rms ${rms.toFixed(4)}  gain ${gain.toFixed(2)}`);

// ------------------------------------------------------------ write wav
const data = Buffer.alloc(N * 4);
for (let i = 0; i < N; i++) {
  data.writeInt16LE(Math.round(Math.max(-1, Math.min(1, master[0][i])) * 32767), i * 4);
  data.writeInt16LE(Math.round(Math.max(-1, Math.min(1, master[1][i])) * 32767), i * 4 + 2);
}
const hdr = Buffer.alloc(44);
hdr.write('RIFF', 0);
hdr.writeUInt32LE(36 + data.length, 4);
hdr.write('WAVE', 8);
hdr.write('fmt ', 12);
hdr.writeUInt32LE(16, 16);
hdr.writeUInt16LE(1, 20);
hdr.writeUInt16LE(2, 22);
hdr.writeUInt32LE(SR, 24);
hdr.writeUInt32LE(SR * 4, 28);
hdr.writeUInt16LE(4, 32);
hdr.writeUInt16LE(16, 34);
hdr.write('data', 36);
hdr.writeUInt32LE(data.length, 40);
const pre = path.join(ROOT, 'out', 'soundtrack.pre.wav');
const final = path.join(ROOT, 'out', 'soundtrack.wav');
fs.writeFileSync(pre, Buffer.concat([hdr, data]));

// Measure loudness, then apply one fixed gain to -16 LUFS with a peak limiter.
// A fixed gain keeps the voice-over-music balance that dynamic normalization would undo.
const probe = spawnSync('ffmpeg', ['-hide_banner', '-nostats', '-i', pre, '-af', 'loudnorm=I=-16:TP=-1.5:print_format=json', '-f', 'null', '-'], { encoding: 'utf8' });
const m = JSON.parse(probe.stderr.slice(probe.stderr.lastIndexOf('{'), probe.stderr.lastIndexOf('}') + 1));
const lift = (-16 - Number(m.input_i)).toFixed(2);
const r2 = spawnSync('ffmpeg', ['-v', 'error', '-y', '-i', pre, '-af', `volume=${lift}dB,alimiter=limit=0.79:attack=4:release=80:level=0`, '-c:a', 'pcm_s16le', final]);
if (r2.status !== 0) throw new Error(`limiter failed: ${r2.stderr}`);
fs.rmSync(pre);
console.log(`soundtrack: ${DUR}s, measured ${m.input_i} LUFS -> -16 LUFS -> ${final}`);
