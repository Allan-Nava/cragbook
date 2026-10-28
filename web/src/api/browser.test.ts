import { describe, expect, it } from "vitest";
import { ApiError } from "./client";
import { BrowserApi, osmUrl } from "./browser";

// A made-up cragcast, not real places: two crags with routes, one without, one whose file fails.
const BASE = "https://cragcast.test/api/v1/";
type Files = Record<string, unknown>;

function cragcast(): Files {
  return {
    "crags.json": { meta: { version: 1 }, crags: [
      { id: "osm-node-1", name: "Parete di Prova", municipality: "Valfinta", provinceCode: "VF", latitude: 45.9, longitude: 9.4,
        aspect: "S", elevation: 600, url: "crags/osm-node-1.json" },
      { id: "osm-node-9", name: "Sasso Vuoto", municipality: null, provinceCode: null, latitude: 45.8, longitude: 9.3,
        aspect: null, elevation: null, url: "crags/osm-node-9.json" },
      { id: "osm-way-5", name: "Altra Parete", municipality: "Borgo Finto", provinceCode: "VF", latitude: 45.7, longitude: 9.2,
        aspect: "E", elevation: 900, url: "crags/osm-way-5.json" },
    ] },
    "crags/osm-node-1.json": { crag: { id: "osm-node-1", sectors: [{ id: "osm-way-2", name: "Settore Alto" }], routes: [
      { id: "osm-node-3", name: "Diedro", grade: "6A+", length: 25, bolts: 11, pitches: 1, sector: "osm-way-2" },
      { id: "osm-node-4", name: "Placca", grade: "5c", length: null, bolts: null, pitches: null, sector: null },
      { id: "osm-node-6", name: null, grade: "V+", length: null, bolts: null, pitches: null, sector: "osm-way-404" },
    ] } },
    "crags/osm-node-9.json": { crag: { id: "osm-node-9", sectors: [], routes: [] } },
    "crags/osm-way-5.json": { crag: { id: "osm-way-5", sectors: [], routes: [
      { id: "osm-node-7", name: "Tetto", grade: "7a", length: 20, bolts: 9, pitches: 2, sector: null },
    ] } },
  };
}

function setup(files = cragcast(), now = new Date(2026, 9, 23, 15)) {
  const data = new Map<string, string>();
  const store = { getItem: (k: string) => data.get(k) ?? null, setItem: (k: string, v: string) => void data.set(k, v) };
  const fetched: string[] = [];
  const fetchFn = async (url: string) => {
    fetched.push(url);
    const path = url.slice(BASE.length);
    return path in files ? new Response(JSON.stringify(files[path])) : new Response("", { status: 404, statusText: "Not Found" });
  };
  return { api: new BrowserApi(BASE, store, fetchFn, () => now), files, fetched, store };
}

describe("BrowserApi", () => {
  it("imports on first use and keeps the catalogue", async () => {
    const { api, fetched, store } = setup();
    expect((await api.crags({})).map(c => [c.name, c.routes, c.done, c.osm])).toEqual([
      ["Altra Parete", 1, 0, "https://www.openstreetmap.org/way/5"],
      ["Parete di Prova", 3, 0, "https://www.openstreetmap.org/node/1"],
      ["Sasso Vuoto", 0, 0, "https://www.openstreetmap.org/node/9"],
    ]);
    expect(fetched).toHaveLength(4);
    const again = new BrowserApi(BASE, store, async () => { throw new Error("no network"); });
    expect(await again.crags({ withRoutes: true })).toHaveLength(2);
  });

  it("searches by name or municipality, any case, at most 100 characters", async () => {
    const { api } = setup();
    expect((await api.crags({ q: " FINTO " })).map(c => c.id)).toEqual(["osm-way-5"]);
    expect((await api.crags({ q: "prova", withRoutes: true })).map(c => c.id)).toEqual(["osm-node-1"]);
    await expect(api.crags({ q: "x".repeat(101) })).rejects.toEqual(new ApiError(400, "invalid query: a search of more than 100 characters"));
  });

  it("gives a crag its routes easiest first, with levels, sectors and OSM links", async () => {
    const c = await setup().api.crag("osm-node-1");
    expect(c.routeList.map(r => [r.id, r.grade, r.level, r.sector, r.osm])).toEqual([
      ["osm-node-4", "5c", "5", null, "https://www.openstreetmap.org/node/4"],
      ["osm-node-3", "6a+", "6", "Settore Alto", "https://www.openstreetmap.org/node/3"],
      ["osm-node-6", null, "", null, "https://www.openstreetmap.org/node/6"],
    ]);
    await expect(setup().api.crag("nowhere")).rejects.toEqual(new ApiError(404, "crag not found"));
  });

  it("writes, reads and clears the diary, refusing what the server refuses", async () => {
    const { api } = setup();
    const r = await api.recordAscent("osm-node-3", { status: "done", climbedOn: "2026-10-20", notes: " a vista " });
    expect(r.ascent).toMatchObject({ status: "done", climbedOn: "2026-10-20", notes: "a vista" });
    await api.recordAscent("osm-node-7", { status: "project", climbedOn: null, notes: "" });
    expect((await api.crag("osm-node-1")).done).toBe(1);
    expect((await api.diary()).map(e => [e.id, e.cragName])).toEqual([["osm-node-3", "Parete di Prova"], ["osm-node-7", "Altra Parete"]]);
    await expect(api.recordAscent("osm-node-3", { status: "todo", climbedOn: "2026-10-01", notes: "" }))
      .rejects.toEqual(new ApiError(400, "invalid ascent: a day only for a route climbed"));
    await expect(api.recordAscent("nowhere", { status: "done", climbedOn: null, notes: "" })).rejects.toMatchObject({ status: 404 });
    await api.clearAscent("osm-node-3");
    await api.clearAscent("osm-node-3");
    expect((await api.diary()).map(e => e.id)).toEqual(["osm-node-7"]);
    await expect(api.clearAscent("nowhere")).rejects.toEqual(new ApiError(404, "route not found"));
  });

  it("re-imports without touching the diary: a crag that fails is left as it was, a route gone kept while in the diary", async () => {
    const { api, files } = setup();
    await api.recordAscent("osm-node-4", { status: "done", climbedOn: null, notes: "" });
    const crag1 = files["crags/osm-node-1.json"] as { crag: { routes: { id: string }[] } };
    crag1.crag.routes = crag1.crag.routes.filter(r => r.id !== "osm-node-4" && r.id !== "osm-node-6");
    delete files["crags/osm-way-5.json"];
    const sum = await api.runImport();
    expect(sum).toEqual({ crags: 2, routes: 1, failed: [`osm-way-5: GET ${BASE}crags/osm-way-5.json: 404 Not Found`] });
    const c = await api.crag("osm-node-1");
    expect(c.routes).toBe(1);
    expect(c.done).toBe(0);
    expect(c.routeList.map(r => [r.id, r.gone])).toEqual([["osm-node-4", true], ["osm-node-3", false]]);
    expect((await api.crag("osm-way-5")).routes).toBe(1);
    expect((await api.diary()).map(e => e.id)).toEqual(["osm-node-4"]);
  });

  it("refuses a second import while one runs, a crag file about another crag, and another API version", async () => {
    const { api, files } = setup();
    (files["crags/osm-node-9.json"] as { crag: { id: string } }).crag.id = "osm-node-1";
    const first = api.runImport();
    await expect(api.runImport()).rejects.toEqual(new ApiError(409, "an import is already running"));
    expect((await first).failed).toEqual([`osm-node-9: the file crags/osm-node-9.json is for crag "osm-node-1"`]);
    const v2 = setup({ "crags.json": { meta: { version: 2 }, crags: [] } });
    await expect(v2.api.crags({})).rejects.toEqual(new ApiError(502, "import: the source failed: crags.json: API version 2, cragbook reads version 1"));
  });
});

it("links OSM ids, and nothing else", () => {
  expect(osmUrl("osm-relation-12")).toBe("https://www.openstreetmap.org/relation/12");
  expect(osmUrl("osm-point-1")).toBe("");
});
