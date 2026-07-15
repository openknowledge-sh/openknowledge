import path from "node:path";
import { createWebServer } from "./server.mjs";
import { distRoot, exportWiki, webRoot } from "./wiki-export.mjs";

const root = process.env.OPENKNOWLEDGE_WEB_ROOT === "dist" ? distRoot : webRoot;
const port = Number(process.env.PORT || 4173);
const host = process.env.HOST || "127.0.0.1";
const refreshWiki = process.env.OPENKNOWLEDGE_WEB_EXPORT_WIKI !== "0";

if (refreshWiki) {
  await exportWiki(path.join(distRoot, "wiki"));
}

const server = createWebServer({
  root,
  fallbackRoot: root === distRoot ? "" : distRoot,
});

server.listen(port, host, () => {
  const address = server.address();
  const listeningPort = typeof address === "object" && address ? address.port : port;
  console.log(`Open Knowledge web: http://${host}:${listeningPort}`);
});
