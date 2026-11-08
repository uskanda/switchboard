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

type ModuleInfo = { isEntry: boolean; importedIds: readonly string[] } | null;

/**
 * staticModules lists the modules the entry reaches by static imports alone. Only these go into the vendor chunks:
 * what is loaded with import() (Mermaid and everything it needs, for the diagram of a question) is left to Rollup's
 * own chunks, so a page without a diagram never loads it.
 */
function staticModules(ids: Iterable<string>, info: (id: string) => ModuleInfo): Set<string> {
  const seen = new Set<string>();
  const todo = [...ids].filter((id) => info(id)?.isEntry);
  while (todo.length > 0) {
    const id = todo.pop()!;
    if (seen.has(id)) continue;
    seen.add(id);
    todo.push(...(info(id)?.importedIds ?? []));
  }
  return seen;
}

let reached: Set<string> | null = null;

// The build goes into the Go module's embedded directory; switchboard-web serves it.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  build: {
    outDir: "../internal/assets/dist",
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: (id, { getModuleIds, getModuleInfo }) => {
          reached ??= staticModules(getModuleIds(), getModuleInfo);
          if (!reached.has(id)) return undefined;
          return Object.keys(vendorChunks).find((name) => vendorChunks[name].test(id));
        },
      },
    },
  },
  server: { proxy: { "/api": "http://127.0.0.1:8783" } },
  test: { environment: "jsdom", globals: true, setupFiles: ["./src/test-language.ts", "./src/test-setup.ts"], css: false },
});
