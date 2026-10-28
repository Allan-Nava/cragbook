// The Go domain's rules, repeated for the demo, which runs without the server (api/browser.ts):
// grades (climbing/grade.go), the routes' order (climbing.SortRoutes), what an ascent may be
// (diary.NewAscent) and the diary's order (diary.Compare). A rule changed there changes here.
import type { Ascent, AscentInput, Level, Status } from "./types";

export const MAX_NOTES = 2000;
export const MAX_QUERY = 100;

/** A French grade's text and rank (6a = 36, 6a+ = 37 … 7a = 42), or null for anything else. */
export function parseGrade(s: string): { text: string; rank: number } | null {
  const text = s.trim().toLowerCase();
  const m = /^([1-9])([abc]?)(\+?)$/.exec(text);
  if (!m) return null;
  return { text, rank: Number(m[1]) * 6 + (m[2] ? m[2].charCodeAt(0) - 97 : 0) * 2 + (m[3] ? 1 : 0) };
}

export function levelOf(rank: number | null): Level {
  if (rank == null) return "";
  const n = Math.floor(rank / 6);
  return n <= 4 ? "4" : n >= 8 ? "8" : (String(n) as Level);
}

const cmp = <T extends string | number>(a: T, b: T) => (a < b ? -1 : a > b ? 1 : 0);

/** Easiest first, the ungraded last, then by name and id. */
export function compareRoutes(a: { id: string; name: string | null; rank: number | null }, b: typeof a): number {
  return cmp(a.rank ?? Infinity, b.rank ?? Infinity)
    || cmp(a.name?.toLowerCase() ?? "￿", b.name?.toLowerCase() ?? "￿")
    || cmp(a.id, b.id);
}

/** Today as YYYY-MM-DD in the browser's time zone, as the server reads it in its own. */
const dayOf = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

/** An entry stamped now, or the reason it is refused, worded as the server words it. */
export function newAscent(input: AscentInput, now: Date): Ascent {
  const status: Status = input.status;
  if (!["done", "project", "todo"].includes(status)) {
    throw new RangeError(`invalid ascent: status "${status}" is not one of "done", "project", "todo"`);
  }
  const notes = (input.notes ?? "").trim();
  if ([...notes].length > MAX_NOTES) throw new RangeError(`invalid ascent: notes of more than ${MAX_NOTES} characters`);
  const day = input.climbedOn || null;
  if (day) {
    if (status !== "done") throw new RangeError("invalid ascent: a day only for a route climbed");
    if (!/^\d{4}-\d{2}-\d{2}$/.test(day) || Number.isNaN(Date.parse(day))) throw new RangeError("climbedOn: a date as YYYY-MM-DD");
    if (day > dayOf(now)) throw new RangeError(`invalid ascent: climbed on ${day}, after today`);
  }
  return { status, climbedOn: day, notes, updatedAt: now.toISOString().replace(/\.\d{3}Z$/, "Z") };
}

/** The climbed first, the latest day first (undated after), then projects, then to try. */
export function compareAscents(a: Ascent & { id: string }, b: Ascent & { id: string }): number {
  const rank = { done: 0, project: 1, todo: 2 };
  return cmp(rank[a.status], rank[b.status])
    || (a.climbedOn && !b.climbedOn ? -1 : !a.climbedOn && b.climbedOn ? 1 : 0)
    || cmp(b.climbedOn ?? "", a.climbedOn ?? "")
    || cmp(b.updatedAt, a.updatedAt)
    || cmp(a.id, b.id);
}
