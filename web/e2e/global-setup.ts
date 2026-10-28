import type { FullConfig } from "@playwright/test";

// serve imports an empty store in the background: wait until the made-up crags are in.
export default async function globalSetup(config: FullConfig) {
  const base = config.projects[0].use.baseURL;
  for (let i = 0; i < 50; i++) {
    const crags: unknown[] = await fetch(`${base}/api/crags?withRoutes=true`).then(r => r.json());
    if (crags.length > 0) return;
    await new Promise(r => setTimeout(r, 200));
  }
  throw new Error("the first import never landed");
}
