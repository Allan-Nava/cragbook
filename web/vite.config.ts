import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import { readFileSync } from "node:fs";

// The logo's one copy is assets/logo.svg (also the README's): served at /logo.svg in development
// and emitted into dist/ by the build.
const logo = () => readFileSync(new URL("../assets/logo.svg", import.meta.url));

// `pnpm dev` proxies /api to the Go backend on :8080; `pnpm build` writes dist/, which the
// backend serves with CRAGBOOK_WEB_DIR=web/dist.
export default defineConfig({
  plugins: [react(), {
    name: "cragbook-logo",
    configureServer(server) {
      server.middlewares.use("/logo.svg", (_req, res) => {
        res.setHeader("Content-Type", "image/svg+xml");
        res.end(logo());
      });
    },
    generateBundle() {
      this.emitFile({ type: "asset", fileName: "logo.svg", source: logo() });
    },
  }],
  server: { proxy: { "/api": "http://localhost:8080" } },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    coverage: {
      provider: "v8",
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/**/*.test.{ts,tsx}", "src/test/**", "src/main.tsx"],
      reporter: ["text-summary"],
      // The coverage measured when the floors were set, rounded down: raise them, never lower them.
      thresholds: { statements: 97, branches: 87, functions: 96, lines: 98 },
    },
  },
});
