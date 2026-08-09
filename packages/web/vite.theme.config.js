import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";

const root = path.dirname(fileURLToPath(import.meta.url));
const viewerAssetsRoot = path.resolve(root, "../cli/cmd/openknowledge/viewer_assets");

export default defineConfig({
  publicDir: false,
  build: {
    emptyOutDir: false,
    lib: {
      entry: path.join(root, "src/viewer/theme.js"),
      name: "OpenKnowledgeViewerTheme",
      formats: ["iife"],
      fileName: () => "viewer-theme.js",
    },
    outDir: viewerAssetsRoot,
  },
});
