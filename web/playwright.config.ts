import { defineConfig, devices } from "@playwright/test";

// The page end to end: the Go binary in memory mode serving dist/, importing made-up crags from
// a fake cragcast (e2e/cragcast). One worker, since both projects share the one diary.
const app = 4180, cragcast = 4181;

export default defineConfig({
  testDir: "e2e",
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: { baseURL: `http://localhost:${app}`, locale: "it-IT", trace: "retain-on-failure" },
  projects: [
    { name: "desktop", use: { ...devices["Desktop Chrome"] } },
    { name: "phone", use: { ...devices["Pixel 7"] } },
  ],
  globalSetup: "./e2e/global-setup.ts",
  webServer: [
    { command: `node e2e/fake-cragcast.mjs ${cragcast}`, url: `http://localhost:${cragcast}/api/v1/crags.json`, reuseExistingServer: !process.env.CI },
    {
      command: "go build -o e2e/.out/cragbook ../cmd/cragbook && e2e/.out/cragbook serve",
      url: `http://localhost:${app}/healthz`,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      env: {
        CRAGBOOK_STORE: "memory",
        CRAGBOOK_ADDR: `localhost:${app}`,
        CRAGBOOK_CRAGCAST_URL: `http://localhost:${cragcast}/api/v1/`,
        CRAGBOOK_WEB_DIR: "dist",
      },
    },
  ],
});
