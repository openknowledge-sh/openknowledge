import { defineConfig } from "vite";
import { fileURLToPath } from "node:url";

export default defineConfig({
  build: {
    outDir: "dist",
    rollupOptions: {
      input: {
        main: fileURLToPath(new URL("./index.html", import.meta.url)),
        gettingStarted: fileURLToPath(new URL("./getting-started/index.html", import.meta.url)),
        signal: fileURLToPath(new URL("./signal/index.html", import.meta.url)),
        blueprint: fileURLToPath(new URL("./blueprint/index.html", import.meta.url)),
        terminal: fileURLToPath(new URL("./terminal/index.html", import.meta.url)),
        useCases: fileURLToPath(new URL("./use-cases/index.html", import.meta.url)),
        projectDocs: fileURLToPath(new URL("./use-cases/project-documentation/index.html", import.meta.url)),
        research: fileURLToPath(new URL("./use-cases/research-notes/index.html", import.meta.url)),
        mcpWebMCP: fileURLToPath(new URL("./use-cases/mcp-webmcp/index.html", import.meta.url)),
        agentContext: fileURLToPath(new URL("./use-cases/agent-context/index.html", import.meta.url)),
        changelogs: fileURLToPath(new URL("./use-cases/changelogs/index.html", import.meta.url)),
        knowledgeCI: fileURLToPath(new URL("./use-cases/knowledge-ci/index.html", import.meta.url)),
      },
    },
  },
  server: {
    host: "127.0.0.1",
  },
});
