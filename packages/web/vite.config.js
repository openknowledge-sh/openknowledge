import { defineConfig } from "vite";
import { fileURLToPath } from "node:url";

export default defineConfig({
  build: {
    outDir: "dist",
    rollupOptions: {
      input: {
        main: fileURLToPath(new URL("./index.html", import.meta.url)),
        gettingStarted: fileURLToPath(new URL("./getting-started/index.html", import.meta.url)),
        terminal: fileURLToPath(new URL("./terminal/index.html", import.meta.url)),
      },
    },
  },
  server: {
    host: "127.0.0.1",
  },
});
