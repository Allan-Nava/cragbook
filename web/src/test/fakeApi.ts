// An in-memory CragbookApi with made-up crags and routes, not real places.
import type { CragbookApi } from "../api/client";
import { ApiError } from "../api/client";
import type { AscentInput, Crag, CragDetail, Route } from "../domain/types";

const route = (id: string, name: string | null, grade: string | null, level: Route["level"], extra: Partial<Route> = {}): Route => ({
  id, name, grade, level, length: null, bolts: null, pitches: null, sector: null, gone: false,
  osm: `https://www.openstreetmap.org/node/${id}`, ascent: null, ...extra,
});

export function fakeApi(): CragbookApi & { calls: string[] } {
  const crags: Crag[] = [
    { id: "osm-node-1", name: "Test Wall", municipality: "Lecco", provinceCode: "LC", latitude: 45.8, longitude: 9.4,
      aspect: "S", elevation: 600, routes: 4, done: 0, osm: "https://www.openstreetmap.org/node/1" },
    { id: "osm-node-9", name: "Empty Wall", municipality: null, provinceCode: null, latitude: 45.9, longitude: 9.5,
      aspect: null, elevation: null, routes: 0, done: 0, osm: "https://www.openstreetmap.org/node/9" },
  ];
  const routes: Route[] = [
    route("2", "Placca", "4", "4"),
    route("3", "Diedro", "6a+", "6", { length: 25, bolts: 11, sector: "Settore Alto" }),
    route("4", "Tetto", "6c", "6", { pitches: 3 }),
    route("5", null, null, ""),
  ];
  const calls: string[] = [];
  return {
    calls,
    async crags({ q = "", withRoutes = false }) {
      calls.push(`crags ${q}|${withRoutes}`);
      return crags.filter(c => (!withRoutes || c.routes > 0) && c.name.toLowerCase().includes(q.toLowerCase()));
    },
    async crag(id): Promise<CragDetail> {
      const c = crags.find(c => c.id === id);
      if (!c) throw new ApiError(404, "crag not found");
      return { ...c, done: routes.filter(r => r.ascent?.status === "done").length, routeList: id === "osm-node-1" ? routes : [] };
    },
    async recordAscent(id, input: AscentInput) {
      calls.push(`record ${id} ${JSON.stringify(input)}`);
      if (input.notes === "rifiutata") throw new ApiError(400, "invalid ascent: refused by the test");
      const r = routes.find(r => r.id === id)!;
      r.ascent = { ...input, updatedAt: "2026-10-23T15:00:00Z" };
      return r;
    },
    async clearAscent(id) {
      calls.push(`clear ${id}`);
      routes.find(r => r.id === id)!.ascent = null;
    },
    async diary() {
      return routes.filter(r => r.ascent).map(r => ({ ...r, cragId: "osm-node-1", cragName: "Test Wall" }));
    },
    async runImport() {
      calls.push("import");
      return { crags: 220, routes: 116, failed: ["osm-node-7: 502 Bad Gateway"] };
    },
  };
}
