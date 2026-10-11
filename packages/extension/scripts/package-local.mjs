import { cp, mkdir, writeFile, chmod, stat, realpath } from 'node:fs/promises';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { buildApp } from './build.mjs';
import { packageRoot, repoRoot } from '../server/main.mjs';

const execute = promisify(execFile);

export function localMCPConfig() {
  return {
    $schema: 'https://agent-plugins.org/schemas/1.0.0/mcp.schema.json',
    mcpServers: {
      openknowledge: {
        type: 'stdio',
        command: './bin/' + (process.platform === 'win32' ? 'node.exe' : 'node'),
        args: ['${PLUGIN_ROOT}/server/main.mjs', '--stdio'],
        env: { OPENKNOWLEDGE_BIN: '${PLUGIN_ROOT}/bin/' + (process.platform === 'win32' ? 'openknowledge.exe' : 'openknowledge') },
      },
    },
  };
}

// This local package owns its MCP lifecycle. It ships application code and a
// platform-specific CLI and Node runtime, without knowledge or user registry.
export async function packageLocal({ destination = path.join(packageRoot, 'dist/local-marketplace'), binary = process.env.OPENKNOWLEDGE_BIN, node = process.execPath } = {}) {
  if (Number(process.versions.node.split('.')[0]) < 22) throw new Error('Package the local plugin with Node.js 22 or newer.');
  const root = path.resolve(destination);
  const plugin = path.join(root, 'plugins/openknowledge-local');
  await mkdir(path.join(plugin, 'server'), { recursive: true });
  await mkdir(path.join(plugin, 'dist'), { recursive: true });
  await mkdir(path.join(plugin, 'bin'), { recursive: true });
  await buildApp();
  for (const name of ['plugin.json', 'assets']) await cp(path.join(packageRoot, name), path.join(plugin, name), { recursive: true });
  await cp(path.join(packageRoot, 'dist/app.html'), path.join(plugin, 'dist/app.html'));
  await cp(path.join(packageRoot, 'dist/native-viewer.js'), path.join(plugin, 'dist/native-viewer.js'));
  await build({
    entryPoints: [path.join(packageRoot, 'server/main.mjs')],
    outfile: path.join(plugin, 'server/main.mjs'),
    bundle: true, platform: 'node', format: 'esm', target: 'node22',
    banner: { js: "import { createRequire as createNodeRequire } from 'node:module'; const require = createNodeRequire(import.meta.url);" },
  });
  const packagedNode = path.join(plugin, 'bin', process.platform === 'win32' ? 'node.exe' : 'node');
  const nodeSource = await realpath(node);
  if (!(await stat(nodeSource)).isFile()) throw new Error('The Node runtime must point to an executable file.');
  if (nodeSource !== packagedNode) await cp(nodeSource, packagedNode);
  const packagedBinary = path.join(plugin, 'bin', process.platform === 'win32' ? 'openknowledge.exe' : 'openknowledge');
  if (binary) {
    const source = await realpath(binary);
    if (!(await stat(source)).isFile()) throw new Error('OPENKNOWLEDGE_BIN must point to a CLI executable.');
    if (source !== packagedBinary) await cp(source, packagedBinary);
  } else {
    await execute('go', ['build', '-o', packagedBinary, './packages/cli/cmd/openknowledge'], { cwd: repoRoot, timeout: 120000, maxBuffer: 4 * 1024 * 1024 });
  }
  if (process.platform !== 'win32') {
    await chmod(packagedNode, 0o755);
    await chmod(packagedBinary, 0o755);
  }
  await writeFile(path.join(plugin, 'mcp.json'), JSON.stringify(localMCPConfig(), null, 2) + '\n');
  await mkdir(path.join(root, '.agents/plugins'), { recursive: true });
  await writeFile(path.join(root, '.agents/plugins/marketplace.json'), JSON.stringify({
    name: 'openknowledge-dev',
    interface: { displayName: 'Open Knowledge Development' },
    plugins: [{ name: 'openknowledge-local', source: { source: 'local', path: './plugins/openknowledge-local' }, category: 'Productivity', policy: { installation: 'AVAILABLE', authentication: 'ON_INSTALL' } }],
  }, null, 2) + '\n');
  return { root, plugin, binary: packagedBinary, node: packagedNode };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const result = await packageLocal();
  console.log(`Self-contained local marketplace prepared: ${result.root}`);
  console.log(`MCP runtime: ${result.node}`);
  console.log('Codex starts this plugin automatically. This command does not install or enable it.');
}
