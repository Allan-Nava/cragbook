// The demo's CragbookApi: no server. The browser imports the crags and routes from cragcast's API
// v1 as the Go importer does — a few crags at a time, a crag that fails left as it was, routes
// no longer listed marked gone, never deleted — and keeps them, with the diary, in localStorage.
// The answers and errors are the HTTP API's; the rules are domain/rules.ts.
import { ApiError, type CragbookApi } from "./client";
import { MAX_QUERY, compareAscents, compareRoutes, levelOf, newAscent, parseGrade } from "../domain/rules";
import type { Ascent, AscentInput, Crag, CragDetail, DiaryEntry, ImportSummary, Route } from "../domain/types";

export const CRAGCAST = "https://allan-nava.github.io/cragcast/api/v1/";
const WORKERS = 6;
const CATALOGUE = "cragbook.catalogue.v1", DIARY = "cragbook.diary.v1";

type StoredCrag = Omit<Crag, "routes" | "done" | "osm">;
type StoredRoute = { id: string; cragId: string; name: string | null; grade: string | null; rank: number | null;
  length: number | null; bolts: number | null; pitches: number | null; sector: string | null; gone: boolean };
type Catalogue = { crags: StoredCrag[]; routes: StoredRoute[] };

type Fetch = (input: string) => Promise<Response>;
type Store = Pick<Storage, "getItem" | "setItem">;

/** The object's page on OpenStreetMap, as climbing.OSMURL. */
export function osmUrl(id: string): string {
  const m = /^osm-(node|way|relation)-(.+)$/.exec(id);
  return m ? `https://www.openstreetmap.org/${m[1]}/${m[2]}` : "";
}

export class BrowserApi implements CragbookApi {
  private importing: Promise<ImportSummary> | null = null;

  constructor(private readonly base = CRAGCAST, private readonly store: Store = localStorage,
              private readonly fetchFn: Fetch = i => fetch(i), private readonly now = () => new Date()) {}

  private read<T>(key: string, empty: T): T {
    const raw = this.store.getItem(key);
    return raw ? (JSON.parse(raw) as T) : empty;
  }
  private diaryMap() { return this.read<Record<string, Ascent>>(DIARY, {}); }

  /** The catalogue, imported first if this browser has none. */
  private async catalogue(): Promise<Catalogue> {
    const c = this.read<Catalogue | null>(CATALOGUE, null);
    if (c) return c;
    await (this.importing ?? this.runImport());
    return this.read<Catalogue>(CATALOGUE, { crags: [], routes: [] });
  }

  private route(r: StoredRoute, ascents: Record<string, Ascent>): Route {
    return { id: r.id, name: r.name, grade: r.grade, level: levelOf(r.rank), length: r.length, bolts: r.bolts,
      pitches: r.pitches, sector: r.sector, gone: r.gone, osm: osmUrl(r.id), ascent: ascents[r.id] ?? null };
  }

  private summary(c: StoredCrag, routes: StoredRoute[], ascents: Record<string, Ascent>): Crag {
    const live = routes.filter(r => r.cragId === c.id && !r.gone);
    return { ...c, routes: live.length, done: live.filter(r => ascents[r.id]?.status === "done").length, osm: osmUrl(c.id) };
  }

  async crags({ q = "", withRoutes = false }: { q?: string; withRoutes?: boolean }): Promise<Crag[]> {
    const query = q.trim().toLowerCase();
    if ([...query].length > MAX_QUERY) throw new ApiError(400, `invalid query: a search of more than ${MAX_QUERY} characters`);
    const { crags, routes } = await this.catalogue();
    const ascents = this.diaryMap();
    return crags
      .filter(c => !query || c.name.toLowerCase().includes(query) || (c.municipality ?? "").toLowerCase().includes(query))
      .map(c => this.summary(c, routes, ascents))
      .filter(c => !withRoutes || c.routes > 0)
      .sort((a, b) => (a.name + a.id < b.name + b.id ? -1 : 1));
  }

  async crag(id: string): Promise<CragDetail> {
    const { crags, routes } = await this.catalogue();
    const c = crags.find(c => c.id === id);
    if (!c) throw new ApiError(404, "crag not found");
    const ascents = this.diaryMap();
    const list = routes.filter(r => r.cragId === id && (!r.gone || ascents[r.id])).sort(compareRoutes);
    return { ...this.summary(c, routes, ascents), routeList: list.map(r => this.route(r, ascents)) };
  }

  async recordAscent(routeId: string, input: AscentInput): Promise<Route> {
    const r = (await this.catalogue()).routes.find(r => r.id === routeId);
    if (!r) throw new ApiError(404, "route not found");
    let a: Ascent;
    try { a = newAscent(input, this.now()); }
    catch (e) { throw new ApiError(400, (e as Error).message); }
    const ascents = { ...this.diaryMap(), [routeId]: a };
    this.store.setItem(DIARY, JSON.stringify(ascents));
    return this.route(r, ascents);
  }

  async clearAscent(routeId: string): Promise<void> {
    if (!(await this.catalogue()).routes.some(r => r.id === routeId)) throw new ApiError(404, "route not found");
    const { [routeId]: _, ...rest } = this.diaryMap();
    this.store.setItem(DIARY, JSON.stringify(rest));
  }

  async diary(): Promise<DiaryEntry[]> {
    const { crags, routes } = await this.catalogue();
    const ascents = this.diaryMap();
    return routes
      .filter(r => ascents[r.id])
      .sort((a, b) => compareAscents({ ...ascents[a.id], id: a.id }, { ...ascents[b.id], id: b.id }))
      .map(r => ({ ...this.route(r, ascents), cragId: r.cragId, cragName: crags.find(c => c.id === r.cragId)?.name ?? r.cragId }));
  }

  /** One import at a time; the diary is never touched by it. */
  runImport(): Promise<ImportSummary> {
    if (this.importing) return Promise.reject(new ApiError(409, "an import is already running"));
    this.importing = this.doImport().finally(() => { this.importing = null; });
    return this.importing;
  }

  private async get<T>(path: string): Promise<T> {
    const url = new URL(path, this.base).toString();
    const res = await this.fetchFn(url);
    if (!res.ok) throw new Error(`GET ${url}: ${res.status} ${res.statusText}`.trim());
    return (await res.json()) as T;
  }

  private async doImport(): Promise<ImportSummary> {
    type Entry = StoredCrag & { url: string };
    let index: { meta: { version: number }; crags: Entry[] };
    try {
      index = await this.get("crags.json");
      if (index.meta?.version !== 1) throw new Error(`crags.json: API version ${index.meta?.version}, cragbook reads version 1`);
    } catch (e) {
      throw new ApiError(502, `import: the source failed: ${(e as Error).message}`);
    }
    const old = this.read<Catalogue>(CATALOGUE, { crags: [], routes: [] });
    const crags = new Map(old.crags.map(c => [c.id, c]));
    const routes = new Map(old.routes.map(r => [r.id, r]));
    const failed: string[] = [];
    let count = 0, routeCount = 0;
    const queue = [...index.crags];
    const work = async () => {
      for (let e = queue.shift(); e; e = queue.shift()) {
        try {
          const read = await this.cragRoutes(e);
          crags.set(e.id, { id: e.id, name: e.name, municipality: e.municipality ?? null, provinceCode: e.provinceCode ?? null,
            latitude: e.latitude, longitude: e.longitude, aspect: e.aspect ?? null, elevation: e.elevation ?? null });
          const listed = new Set(read.map(r => r.id));
          for (const r of routes.values()) if (r.cragId === e.id && !listed.has(r.id)) routes.set(r.id, { ...r, gone: true });
          for (const r of read) routes.set(r.id, r);
          count++;
          routeCount += read.length;
        } catch (err) {
          failed.push(`${e.id}: ${(err as Error).message}`);
        }
      }
    };
    await Promise.all(Array.from({ length: WORKERS }, work));
    this.store.setItem(CATALOGUE, JSON.stringify({ crags: [...crags.values()], routes: [...routes.values()] }));
    return { crags: count, routes: routeCount, failed: failed.sort() };
  }

  /** A crag's file as routes, as the cragcast adapter reads it: a grade not French is dropped. */
  private async cragRoutes(e: { id: string; url: string }): Promise<StoredRoute[]> {
    type File = { crag: { id: string; sectors: { id: string; name: string | null }[];
      routes: { id: string; name: string | null; grade: string | null; length: number | null; bolts: number | null;
        pitches: number | null; sector: string | null }[] } };
    const { crag } = await this.get<File>(e.url);
    if (crag?.id !== e.id) throw new Error(`the file ${e.url} is for crag "${crag?.id}"`);
    const sectors = new Map(crag.sectors.map(s => [s.id, s.name]));
    return crag.routes.map(r => {
      const g = r.grade ? parseGrade(r.grade) : null;
      return { id: r.id, cragId: e.id, name: r.name ?? null, grade: g?.text ?? null, rank: g?.rank ?? null, length: r.length ?? null,
        bolts: r.bolts ?? null, pitches: r.pitches ?? null, sector: r.sector ? sectors.get(r.sector) ?? null : null, gone: false };
    });
  }
}
