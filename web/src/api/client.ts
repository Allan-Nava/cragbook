// The port the components talk to, and its HTTP adapter. Components get it from ApiContext, so
// tests hand them a fake and nothing in them knows about fetch.
import { createContext, useContext } from "react";
import type { AscentInput, Crag, CragDetail, DiaryEntry, ImportSummary, Route } from "../domain/types";

export interface CragbookApi {
  crags(query: { q?: string; withRoutes?: boolean }): Promise<Crag[]>;
  crag(id: string): Promise<CragDetail>;
  recordAscent(routeId: string, input: AscentInput): Promise<Route>;
  clearAscent(routeId: string): Promise<void>;
  diary(): Promise<DiaryEntry[]>;
  runImport(): Promise<ImportSummary>;
}

/** An answer that is not 2xx, with the API's own message. */
export class ApiError extends Error {
  constructor(readonly status: number, message: string) {
    super(message);
  }
}

type Fetch = (input: string, init?: RequestInit) => Promise<Response>;

export class HttpApi implements CragbookApi {
  constructor(private readonly base = "", private readonly fetchFn: Fetch = (i, o) => fetch(i, o)) {}

  private async call<T>(path: string, init?: RequestInit): Promise<T> {
    const res = await this.fetchFn(this.base + path, init);
    if (!res.ok) {
      let msg = res.statusText;
      try { msg = ((await res.json()) as { error?: string }).error ?? msg; } catch { /* not JSON */ }
      throw new ApiError(res.status, msg);
    }
    return (res.status === 204 ? undefined : await res.json()) as T;
  }

  crags({ q = "", withRoutes = false }: { q?: string; withRoutes?: boolean }) {
    const p = new URLSearchParams();
    if (q) p.set("q", q);
    if (withRoutes) p.set("withRoutes", "true");
    const qs = p.toString();
    return this.call<Crag[]>(`/api/crags${qs ? "?" + qs : ""}`);
  }
  crag(id: string) { return this.call<CragDetail>(`/api/crags/${encodeURIComponent(id)}`); }
  recordAscent(routeId: string, input: AscentInput) {
    return this.call<Route>(`/api/routes/${encodeURIComponent(routeId)}/ascent`, {
      method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input),
    });
  }
  clearAscent(routeId: string) {
    return this.call<void>(`/api/routes/${encodeURIComponent(routeId)}/ascent`, { method: "DELETE" });
  }
  diary() { return this.call<DiaryEntry[]>("/api/diary"); }
  runImport() { return this.call<ImportSummary>("/api/import", { method: "POST" }); }
}

export const ApiContext = createContext<CragbookApi>(new HttpApi());
export const useApi = () => useContext(ApiContext);
