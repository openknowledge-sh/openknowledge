#!/usr/bin/env node
// Render the explainer film.
//
//   node film/render.mjs stills 1.5 4.2 ...   PNG stills + contact sheet in film/out/stills
//   node film/render.mjs cues                 export audio cues to film/out/cues.json
//   node film/render.mjs frames               render every frame (FRAMES_DIR, default film/frames)
//   node film/render.mjs encode               mux frames + film/out/soundtrack.wav into film/out/*.mp4
//   node film/render.mjs all                  cues, soundtrack, frames, encode
//
// Env: WORKERS (default 8), FPS (default 60), FROM/TO seconds, CHROME (executable path).
import { chromium } from 'playwright';
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const ROOT = path.dirname(fileURLToPath(import.meta.url));
const OUT = path.join(ROOT, 'out');
const FRAMES = process.env.FRAMES_DIR || path.join(ROOT, 'frames');
const FPS = +(process.env.FPS || 60);
const WORKERS = +(process.env.WORKERS || Math.min(8, os.cpus().length));
const CHROME = process.env.CHROME || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';

const TYPES = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.png': 'image/png', '.woff2': 'font/woff2', '.wav': 'audio/wav', '.json': 'application/json' };

function serve() {
  return new Promise((resolve) => {
    const srv = http.createServer((req, res) => {
      let p = decodeURIComponent(req.url.split('?')[0]);
      if (p.endsWith('/')) p += 'index.html';
      const f = path.join(ROOT, path.normalize(p));
      if (!f.startsWith(ROOT)) {
        res.writeHead(403);
        res.end();
        return;
      }
      fs.readFile(f, (err, data) => {
        if (err) {
          res.writeHead(404);
          res.end();
          return;
        }
        res.writeHead(200, { 'content-type': TYPES[path.extname(f)] || 'application/octet-stream' });
        res.end(data);
      });
    });
    srv.listen(0, '127.0.0.1', () => resolve(srv));
  });
}

async function launch() {
  const opts = { args: ['--force-color-profile=srgb', '--hide-scrollbars', '--disable-lcd-text', '--font-render-hinting=none'] };
  if (fs.existsSync(CHROME)) opts.executablePath = CHROME;
  return chromium.launch(opts);
}

async function openPage(browser, url) {
  const page = await browser.newPage({ viewport: { width: 1920, height: 1080 }, deviceScaleFactor: 1 });
  page.on('pageerror', (e) => console.error('page error:', e.message));
  page.on('console', (m) => m.type() === 'error' && console.error('console:', m.text()));
  await page.goto(url);
  await page.waitForFunction(() => window.__film);
  await page.evaluate(() => window.__film.ready);
  return page;
}

async function shoot(page, t, file) {
  await page.evaluate((tt) => window.__film.seek(tt), t);
  page._cdp ??= await page.context().newCDPSession(page);
  const { data } = await page._cdp.send('Page.captureScreenshot', {
    format: 'png',
    optimizeForSpeed: true,
    captureBeyondViewport: false,
    clip: { x: 0, y: 0, width: 1920, height: 1080, scale: 1 },
  });
  await fs.promises.writeFile(file, Buffer.from(data, 'base64'));
}

async function withFilm(fn) {
  const srv = await serve();
  const url = `http://127.0.0.1:${srv.address().port}/index.html?render=1`;
  const browser = await launch();
  try {
    return await fn(browser, url);
  } finally {
    await browser.close();
    srv.close();
  }
}

async function stills(times) {
  const dir = path.join(OUT, 'stills');
  fs.rmSync(dir, { recursive: true, force: true });
  fs.mkdirSync(dir, { recursive: true });
  await withFilm(async (browser, url) => {
    const page = await openPage(browser, url);
    for (const t of times) await shoot(page, t, path.join(dir, `t${t.toFixed(2).padStart(6, '0')}.png`));
  });
  const files = fs.readdirSync(dir).filter((f) => f.endsWith('.png')).sort();
  const cols = files.length <= 4 ? files.length : files.length <= 9 ? 3 : 4;
  const rows = Math.ceil(files.length / cols);
  const list = files.map((f) => ['-i', path.join(dir, f)]).flat();
  const pads = cols * rows - files.length;
  const inputs = files.map((_, i) => `[${i}:v]scale=640:360,pad=640:360:0:0[v${i}]`);
  for (let i = 0; i < pads; i++) inputs.push(`color=c=black:s=640x360:d=1[p${i}]`);
  const labels = files.map((_, i) => `[v${i}]`).concat(Array.from({ length: pads }, (_, i) => `[p${i}]`)).join('');
  const layout = Array.from({ length: cols * rows }, (_, i) => `${(i % cols) * 640}_${Math.floor(i / cols) * 360}`).join('|');
  const graph = `${inputs.join(';')};${labels}xstack=inputs=${cols * rows}:layout=${layout}`;
  spawnSync('ffmpeg', ['-v', 'error', '-y', ...list, '-filter_complex', graph, '-frames:v', '1', path.join(dir, 'sheet.png')], { stdio: 'inherit' });
  console.log(`stills: ${files.length} -> ${dir}`);
}

async function cues() {
  fs.mkdirSync(OUT, { recursive: true });
  const data = await withFilm(async (browser, url) => {
    const page = await openPage(browser, url);
    return page.evaluate(() => ({ duration: window.__film.duration, cues: window.__film.cues }));
  });
  fs.writeFileSync(path.join(OUT, 'cues.json'), JSON.stringify(data, null, 1));
  console.log(`cues: ${data.cues.length} -> ${path.join(OUT, 'cues.json')}`);
  return data;
}

async function frames() {
  fs.mkdirSync(FRAMES, { recursive: true });
  const duration = 96;
  const from = Math.round(+(process.env.FROM || 0) * FPS);
  const to = Math.round(+(process.env.TO || duration) * FPS);
  const todo = [];
  for (let f = from; f < to; f++) {
    const file = path.join(FRAMES, `f${String(f).padStart(5, '0')}.png`);
    if (!process.env.FORCE && fs.existsSync(file) && fs.statSync(file).size > 0) continue;
    todo.push(f);
  }
  console.log(`frames: ${todo.length} to render with ${WORKERS} workers @ ${FPS} fps`);
  const started = Date.now();
  let done = 0;
  const srv = await serve();
  const url = `http://127.0.0.1:${srv.address().port}/index.html?render=1`;
  {
    let next = 0;
    await Promise.all(
      Array.from({ length: WORKERS }, async () => {
        const browser = await launch();
        const page = await openPage(browser, url);
        while (next < todo.length) {
          const f = todo[next++];
          await shoot(page, f / FPS, path.join(FRAMES, `f${String(f).padStart(5, '0')}.png`));
          done++;
          if (done % 240 === 0) {
            const rate = done / ((Date.now() - started) / 1000);
            console.log(`  ${done}/${todo.length}  ${rate.toFixed(1)} fps  eta ${((todo.length - done) / rate).toFixed(0)} s`);
          }
        }
        if (page._cdp) await page._cdp.detach().catch(() => {});
        await browser.close();
      }),
    );
  }
  console.log(`frames: workers closed after ${((Date.now() - started) / 1000).toFixed(0)} s`);
  srv.closeAllConnections?.();
  srv.close();
  console.log(`frames: done in ${((Date.now() - started) / 1000).toFixed(0)} s`);
}

function encode() {
  const audio = path.join(OUT, 'soundtrack.wav');
  const master = path.join(OUT, 'openknowledge-explainer.mp4');
  const web = path.join(OUT, 'openknowledge-explainer-web.mp4');
  const input = ['-framerate', String(FPS), '-i', path.join(FRAMES, 'f%05d.png')];
  const withAudio = fs.existsSync(audio) ? ['-i', audio] : [];
  const amap = withAudio.length ? ['-map', '0:v', '-map', '1:a', '-c:a', 'aac', '-b:a', '256k', '-shortest'] : [];
  const color = ['-colorspace', 'bt709', '-color_primaries', 'bt709', '-color_trc', 'bt709', '-color_range', 'tv'];
  const vf = ['-vf', 'scale=in_range=full:out_range=tv:out_color_matrix=bt709,format=yuv420p'];
  const run = (args) => {
    const r = spawnSync('ffmpeg', ['-v', 'error', '-stats', '-y', ...args], { stdio: 'inherit' });
    if (r.status !== 0) throw new Error('ffmpeg failed');
  };
  run([...input, ...withAudio, ...amap, ...vf, '-c:v', 'libx264', '-preset', 'slow', '-crf', '14', '-tune', 'animation', ...color, '-movflags', '+faststart', master]);
  run(['-i', master, '-c:v', 'libx264', '-preset', 'slow', '-crf', '23', '-tune', 'animation', '-r', '30', ...color, '-c:a', 'aac', '-b:a', '160k', '-movflags', '+faststart', web]);
  console.log(`encoded: ${master}\n         ${web}`);
}

const [mode, ...rest] = process.argv.slice(2);
if (mode === 'stills') await stills(rest.map(Number));
else if (mode === 'cues') await cues();
else if (mode === 'frames') await frames();
else if (mode === 'encode') encode();
else if (mode === 'all') {
  await cues();
  const r = spawnSync('node', [path.join(ROOT, 'audio.mjs')], { stdio: 'inherit' });
  if (r.status !== 0) process.exit(1);
  await frames();
  encode();
} else {
  console.error('usage: node film/render.mjs stills <t...> | cues | frames | encode | all');
  process.exit(2);
}
