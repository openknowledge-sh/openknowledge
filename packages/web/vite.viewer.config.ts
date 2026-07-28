import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";

const root = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  publicDir: false,
  build: {
    emptyOutDir: true,
    lib: {
      entry: path.join(root, "src/viewer/index.ts"),
      name: "OpenKnowledgeViewer",
      formats: ["iife"],
      fileName: () => "viewer.js",
      cssFileName: "viewer",
    },
    outDir: "viewer-dist",
  },
});
