import { fileURLToPath, URL } from "node:url";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// Dependencies split out of the app chunk, so no chunk passes Vite's 500 kB warning and an app-only
// change leaves the vendor chunks cached. The rest of node_modules is mostly the react-markdown tree.
const vendorChunks: Record<string, RegExp> = {
  react: /[\\/]node_modules[\\/](react|react-dom|react-router|scheduler)[\\/]/,
  vendor: /[\\/]node_modules[\\/]/,
};

// The build goes into the Go module's embedded directory; switchboard-web serves it.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  build: {
    outDir: "../internal/assets/dist",
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: (id) => Object.keys(vendorChunks).find((name) => vendorChunks[name].test(id)),
      },
    },
  },
  server: { proxy: { "/api": "http://127.0.0.1:8783" } },
  test: { environment: "jsdom", globals: true, setupFiles: ["./src/test-language.ts", "./src/test-setup.ts"], css: false },
});
