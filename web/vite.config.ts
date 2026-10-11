import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath } from "node:url";

const here = (p: string) => fileURLToPath(new URL(p, import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: [
      // The @next build imports mermaid-to-excalidraw eagerly; the shim makes it lazy (spike 015).
      { find: /^@excalidraw\/mermaid-to-excalidraw$/, replacement: here("./src/mermaid-lazy.ts") },
      { find: /^mermaid-to-excalidraw-real$/, replacement: here("./node_modules/@excalidraw/mermaid-to-excalidraw/dist/index.js") },
    ],
  },
  define: { "process.env.IS_PREACT": JSON.stringify("false") },
  build: { target: "es2022", outDir: "dist", emptyOutDir: true, sourcemap: false },
  server: {
    port: 5173,
    proxy: { "/api": { target: "http://127.0.0.1:4717", changeOrigin: false } },
  },
});
