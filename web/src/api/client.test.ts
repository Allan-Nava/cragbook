import { describe, expect, it } from "vitest";
import { ApiError, HttpApi } from "./client";

function recorder(status: number, body: unknown) {
  const calls: { url: string; init?: RequestInit }[] = [];
  const fetchFn = async (url: string, init?: RequestInit) => {
    calls.push({ url, init });
    return new Response(status === 204 ? null : typeof body === "string" ? body : JSON.stringify(body), { status, statusText: "Nope" });
  };
  return { calls, api: new HttpApi("http://api.test", fetchFn) };
}

describe("HttpApi", () => {
  it("builds the URLs and bodies the Go API expects", async () => {
    const { calls, api } = recorder(200, []);
    await api.crags({ q: "còrna", withRoutes: true });
    await api.crags({});
    await api.crag("osm-node-1");
    await api.recordAscent("osm-node/2", { status: "done", climbedOn: "2026-10-20", notes: "" });
    await api.diary();
    await api.runImport();
    expect(calls.map(c => `${c.init?.method ?? "GET"} ${c.url}`)).toEqual([
      "GET http://api.test/api/crags?q=c%C3%B2rna&withRoutes=true",
      "GET http://api.test/api/crags",
      "GET http://api.test/api/crags/osm-node-1",
      "PUT http://api.test/api/routes/osm-node%2F2/ascent",
      "GET http://api.test/api/diary",
      "POST http://api.test/api/import",
    ]);
    expect(calls[3].init?.body).toBe(`{"status":"done","climbedOn":"2026-10-20","notes":""}`);
  });

  it("answers nothing for 204", async () => {
    const { api } = recorder(204, null);
    expect(await api.clearAscent("osm-node-2")).toBeUndefined();
  });

  it("throws the API's message, or the status text when there is none", async () => {
    await expect(recorder(400, { error: "invalid ascent" }).api.crags({})).rejects.toEqual(new ApiError(400, "invalid ascent"));
    await expect(recorder(502, "<html>").api.crags({})).rejects.toMatchObject({ status: 502, message: "Nope" });
  });
});
