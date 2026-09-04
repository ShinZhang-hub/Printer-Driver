import { defineConfig } from "vite";

export default defineConfig({
  server: { port: 1422, strictPort: true },
  build: { outDir: "dist", emptyOutDir: true },
});
