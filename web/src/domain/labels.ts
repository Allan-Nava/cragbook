// How the page says the domain's values in Italian.
import type { Level, Status } from "./types";

export const LEVELS: { level: Level; label: string; word: string | null }[] = [
  { level: "4", label: "4 e meno", word: "facile" },
  { level: "5", label: "5", word: "medio" },
  { level: "6", label: "6", word: "impegnativo" },
  { level: "7", label: "7", word: "difficile" },
  { level: "8", label: "8 e oltre", word: "estremo" },
  { level: "", label: "Senza grado", word: null },
];

export const STATUS: Record<Status, string> = { done: "Salita", project: "Progetto", todo: "Da provare" };

export const many = (n: number, one: string, more: string) => `${n} ${n === 1 ? one : more}`;

/** "20 ott 2026" from "2026-10-20", read as a calendar day (no time zone shift). */
export function dayLabel(iso: string): string {
  const [y, m, d] = iso.split("-").map(Number);
  return new Date(Date.UTC(y, m - 1, d)).toLocaleDateString("it-IT", { day: "numeric", month: "short", year: "numeric", timeZone: "UTC" });
}

/** Today as YYYY-MM-DD in the browser's time zone: the latest day a route can be climbed on. */
export function today(now = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}
