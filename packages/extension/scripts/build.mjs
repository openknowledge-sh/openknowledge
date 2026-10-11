import { build } from 'esbuild';
import { mkdir, readFile, writeFile, rename } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export async function buildApp() {
  const output = await build({
    entryPoints: [path.join(root, 'web/app.js')], bundle: true, write: false,
    format: 'iife', target: 'es2022', minify: true,
    outfile: path.join(root, 'dist/app.js'),
  });
  const js = output.outputFiles.find(file => file.path.endsWith('.js')).text;
  const css = output.outputFiles.find(file => file.path.endsWith('.css'))?.text || '';
  const template = await readFile(path.join(root, 'web/index.html'), 'utf8');
  const html = template.replace('<script type="module" src="/app.js"></script>', () => `<style>${css}</style><script>${js.replaceAll('</script', '<\\/script')}</script>`);
  await mkdir(path.join(root, 'dist'), { recursive: true });
  await build({
    entryPoints: [path.join(root, 'web/native-viewer.js')],
    bundle: true, format: 'iife', target: 'es2022', minify: true,
    outfile: path.join(root, 'dist/native-viewer.js'),
  });
  const temporary = path.join(root, `dist/app-${process.pid}.tmp`);
  await writeFile(temporary, html);
  await rename(temporary, path.join(root, 'dist/app.html'));
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) await buildApp();
