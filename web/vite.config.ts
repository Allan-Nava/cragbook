import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// `pnpm dev` proxies /api to the Go backend on :8080; `pnpm build` writes dist/, which the
// backend serves with CRAGBOOK_WEB_DIR=web/dist.
export default defineConfig({
  plugins: [react()],
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
