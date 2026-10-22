// The shapes the Go API answers with (internal/transport/httpapi/dto.go); they change together.
export type Status = "done" | "project" | "todo";
/** "4" for 4 and under, "8" for 8 and over, "" for no grade. */
export type Level = "4" | "5" | "6" | "7" | "8" | "";

export interface Crag {
  id: string;
  name: string;
  municipality: string | null;
  provinceCode: string | null;
  latitude: number;
  longitude: number;
  aspect: string | null;
  elevation: number | null;
  routes: number;
  done: number;
  osm: string;
}

export interface Ascent {
  status: Status;
  climbedOn: string | null;
  notes: string;
  updatedAt: string;
}

export interface Route {
  id: string;
  name: string | null;
  grade: string | null;
  level: Level;
  length: number | null;
  bolts: number | null;
  pitches: number | null;
  sector: string | null;
  gone: boolean;
  osm: string;
  ascent: Ascent | null;
}

export interface CragDetail extends Crag {
  routeList: Route[];
}

export interface DiaryEntry extends Route {
  cragId: string;
  cragName: string;
}

export interface ImportSummary {
  crags: number;
  routes: number;
  failed: string[];
}

/** What PUT /api/routes/{id}/ascent takes. */
export interface AscentInput {
  status: Status;
  climbedOn: string | null;
  notes: string;
}
