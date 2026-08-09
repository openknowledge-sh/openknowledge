import { defineConfig } from "vite";
import { fileURLToPath } from "node:url";

export default defineConfig({
  build: {
    outDir: "dist",
    rollupOptions: {
      input: {
        main: fileURLToPath(new URL("./index.html", import.meta.url)),
        gettingStarted: fileURLToPath(new URL("./getting-started/index.html", import.meta.url)),
        useCases: fileURLToPath(new URL("./use-cases/index.html", import.meta.url)),
        projectDocumentation: fileURLToPath(new URL("./use-cases/project-documentation/index.html", import.meta.url)),
        changelogs: fileURLToPath(new URL("./use-cases/changelogs/index.html", import.meta.url)),
        researchNotes: fileURLToPath(new URL("./use-cases/research-notes/index.html", import.meta.url)),
      },
    },
  },
  server: {
    host: "127.0.0.1",
  },
});
