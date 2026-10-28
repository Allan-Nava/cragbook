// cragcast's API v1 as files under e2e/cragcast, for the binary the end-to-end tests run:
// made-up crags, so the tests never reach the real API. Usage: node e2e/fake-cragcast.mjs <port>
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { join, normalize } from "node:path";

const root = join(import.meta.dirname, "cragcast");
const port = Number(process.argv[2] ?? 4181);

createServer(async (req, res) => {
  const path = normalize(new URL(req.url, "http://x").pathname);
  try {
    const body = await readFile(join(root, path));
    res.writeHead(200, { "Content-Type": "application/json" }).end(body);
  } catch {
    res.writeHead(404).end();
  }
}).listen(port, "localhost", () => console.log(`fake cragcast on :${port}`));
