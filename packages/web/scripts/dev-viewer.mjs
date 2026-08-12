import { spawn } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { createServer } from "vite";
import { repoRoot, webRoot } from "./wiki-export.mjs";

const viewerAssetPrefix = "/assets/openknowledge/";
const viewerThemePath = path.join(repoRoot, "packages/cli/cmd/openknowledge/viewer_theme.css");
const viewerThemeURL = `/@fs/${viewerThemePath.replaceAll(path.sep, "/")}`;

export function useViteViewerAssets(html) {
  return html
    .replaceAll(
      `<script src="${viewerAssetPrefix}viewer-theme.js"></script>`,
      '<script type="module" src="/src/viewer/theme.js"></script>',
    )
    .replaceAll(
      `<link rel="stylesheet" href="${viewerAssetPrefix}viewer.css">`,
      `<link rel="stylesheet" href="${viewerThemeURL}">\n  <link rel="stylesheet" href="/src/viewer/styles/index.css">`,
    )
    .replaceAll(
      `<script src="${viewerAssetPrefix}viewer.js"></script>`,
      '<script type="module" src="/src/viewer/index.js"></script>',
    )
    .replaceAll(
      `<script src="${viewerAssetPrefix}viewer-live-reload.js"></script>`,
      '<script type="module" src="/src/viewer/live-reload-entry.js"></script>',
    );
}

export function isViteViewerRequest(rawURL = "") {
  const pathname = new URL(rawURL, "http://viewer.local").pathname;
  return (
    pathname.startsWith("/@") ||
    pathname.startsWith("/src/") ||
    pathname.startsWith("/node_modules/") ||
    pathname === "/__vite_ping"
  );
}

function copyResponseHeaders(upstream, response, transformed) {
  for (const [name, value] of upstream.headers) {
    if (transformed && (name === "content-encoding" || name === "content-length")) {
      continue;
    }
    response.setHeader(name, value);
  }
}

function createViewerProxy(backendURL, vite) {
  return async (request, response, next) => {
    if (isViteViewerRequest(request.url)) {
      next();
      return;
    }

    try {
      const target = new URL(request.url || "/", backendURL);
      const headers = { ...request.headers, host: target.host };
      delete headers["accept-encoding"];
      const hasBody = request.method !== "GET" && request.method !== "HEAD";
      const init = {
        method: request.method,
        headers,
        redirect: "manual",
      };
      if (hasBody) {
        init.body = request;
        init.duplex = "half";
      }
      const upstream = await fetch(target, init);
      const isHTML = upstream.headers.get("content-type")?.startsWith("text/html");

      response.statusCode = upstream.status;
      copyResponseHeaders(upstream, response, isHTML);
      if (isHTML) {
        const html = useViteViewerAssets(await upstream.text());
        response.end(await vite.transformIndexHtml(request.url || "/", html));
        return;
      }

      response.end(Buffer.from(await upstream.arrayBuffer()));
    } catch (error) {
      next(error);
    }
  };
}

function startViewer(args) {
  const configuredBinary = process.env.OPENKNOWLEDGE_BIN?.trim();
  const command = configuredBinary || "go";
  const commandArgs = configuredBinary
    ? ["view", "--no-browser", "--host", "127.0.0.1", "--port", "0", ...args]
    : ["run", "./packages/cli/cmd/openknowledge", "view", "--no-browser", "--host", "127.0.0.1", "--port", "0", ...args];
  const child = spawn(command, commandArgs, {
    cwd: repoRoot,
    env: { ...process.env, OPENKNOWLEDGE_TELEMETRY_SUPPRESS: "1" },
    stdio: ["inherit", "pipe", "inherit"],
  });

  let output = "";
  const ready = new Promise((resolve, reject) => {
    const onError = (error) => reject(error);
    const onExit = (code, signal) => {
      reject(new Error(`${command} exited before the viewer started (${signal || `exit code ${code}`})`));
    };
    child.once("error", onError);
    child.once("exit", onExit);
    child.stdout.on("data", (chunk) => {
      const text = chunk.toString();
      process.stdout.write(text);
      output += text;
      const match = output.match(/Open Knowledge view: (https?:\/\/\S+)/);
      if (!match) {
        return;
      }
      child.off("error", onError);
      child.off("exit", onExit);
      resolve(new URL(match[1]));
    });
  });

  return { child, ready };
}

function openBrowser(url) {
  let command = "xdg-open";
  let args = [url];
  if (process.platform === "darwin") {
    command = "open";
  } else if (process.platform === "win32") {
    command = "cmd";
    args = ["/c", "start", "", url];
  }
  const child = spawn(command, args, { detached: true, stdio: "ignore" });
  child.once("error", (error) => {
    console.warn(`Could not open the development viewer: ${error.message}`);
  });
  child.unref();
}

export async function main(args = process.argv.slice(2)) {
  const viewArgs = args[0] === "--" ? args.slice(1) : args;
  const { child: viewer, ready } = startViewer(viewArgs);
  const backendURL = await ready;
  let vite;
  try {
    vite = await createServer({
      root: webRoot,
      appType: "custom",
      server: {
        host: "127.0.0.1",
        port: Number(process.env.OPENKNOWLEDGE_VIEWER_DEV_PORT || 5173),
        fs: { allow: [repoRoot] },
      },
      plugins: [
        {
          name: "openknowledge-viewer-proxy",
          configureServer(server) {
            server.middlewares.use(createViewerProxy(backendURL, server));
          },
        },
      ],
    });
    await vite.listen();
  } catch (error) {
    viewer.kill("SIGTERM");
    await vite?.close();
    throw error;
  }

  const origin = new URL(vite.resolvedUrls.local[0]);
  const devURL = new URL(backendURL.pathname + backendURL.search, origin);
  console.log(`Open Knowledge viewer development: ${devURL}`);
  console.log("Edit packages/web/src/viewer; Vite updates the open page.");
  console.log("Press Ctrl+C to stop.");

  if (!viewArgs.includes("--no-browser")) {
    openBrowser(devURL.href);
  }

  let stopping = false;
  const stop = async (signal) => {
    if (stopping) {
      return;
    }
    stopping = true;
    viewer.kill(signal);
    await vite.close();
  };
  process.once("SIGINT", () => void stop("SIGINT"));
  process.once("SIGTERM", () => void stop("SIGTERM"));
  viewer.once("exit", async (code, signal) => {
    if (stopping) {
      return;
    }
    await stop("SIGTERM");
    process.exitCode = code || (signal ? 1 : 0);
  });
}

if (process.argv[1]?.endsWith("dev-viewer.mjs")) {
  await main();
}
