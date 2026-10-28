import { describe, expect, it } from "vitest";
import { compareAscents, compareRoutes, levelOf, newAscent, parseGrade } from "./rules";

// The same cases as the Go domain's tests: the demo must refuse what the server refuses.
describe("grades", () => {
  it("reads French grades and ranks them as climbers do", () => {
    expect(parseGrade(" 6A+ ")).toEqual({ text: "6a+", rank: 37 });
    expect(parseGrade("6")).toEqual({ text: "6", rank: 36 });
    expect(parseGrade("7a")?.rank).toBe(42);
    expect(parseGrade("V+")).toBeNull();
    expect(parseGrade("6d")).toBeNull();
  });

  it("groups them in levels", () => {
    expect([parseGrade("3c")!.rank, parseGrade("5c")!.rank, parseGrade("6c+")!.rank, parseGrade("9a")!.rank, null].map(levelOf))
      .toEqual(["4", "5", "6", "8", ""]);
  });

  it("orders routes easiest first, the ungraded last, then by name and id", () => {
    const r = (id: string, name: string | null, grade: string | null) => ({ id, name, rank: grade ? parseGrade(grade)!.rank : null });
    const sorted = [r("d", null, null), r("c", "Tetto", "7a"), r("b", "placca", "5a"), r("a", "Placca", "5a"), r("e", "Zeta", null)]
      .sort(compareRoutes).map(x => x.id);
    expect(sorted).toEqual(["a", "b", "c", "e", "d"]);
  });
});

describe("an ascent", () => {
  const now = new Date(2026, 9, 23, 15, 0, 0);

  it("is stamped now, its notes trimmed", () => {
    const a = newAscent({ status: "done", climbedOn: "2026-10-23", notes: "  a vista " }, now);
    expect(a).toEqual({ status: "done", climbedOn: "2026-10-23", notes: "a vista", updatedAt: now.toISOString().replace(/\.\d{3}Z$/, "Z") });
    expect(newAscent({ status: "project", climbedOn: "", notes: "" }, now).climbedOn).toBeNull();
  });

  it("refuses what the server refuses, in its words", () => {
    const refused = (input: object) => { try { newAscent(input as never, now); return ""; } catch (e) { return (e as Error).message; } };
    expect(refused({ status: "maybe", climbedOn: null, notes: "" })).toMatch(/status "maybe"/);
    expect(refused({ status: "todo", climbedOn: "2026-10-01", notes: "" })).toBe("invalid ascent: a day only for a route climbed");
    expect(refused({ status: "done", climbedOn: "20/10/2026", notes: "" })).toBe("climbedOn: a date as YYYY-MM-DD");
    expect(refused({ status: "done", climbedOn: "2026-10-24", notes: "" })).toBe("invalid ascent: climbed on 2026-10-24, after today");
    expect(refused({ status: "done", climbedOn: null, notes: "é".repeat(2001) })).toBe("invalid ascent: notes of more than 2000 characters");
    expect(refused({ status: "done", climbedOn: null, notes: "é".repeat(2000) })).toBe("");
  });
});

it("orders the diary: climbed by latest day, undated after, then projects, then to try", () => {
  const a = (id: string, status: "done" | "project" | "todo", climbedOn: string | null, updatedAt = "2026-10-01T00:00:00Z") =>
    ({ id, status, climbedOn, notes: "", updatedAt });
  const sorted = [a("t", "todo", null), a("p1", "project", null, "2026-10-01T00:00:00Z"), a("p2", "project", null, "2026-10-02T00:00:00Z"),
    a("undated", "done", null), a("old", "done", "2026-01-01"), a("new", "done", "2026-10-20"), a("new2", "done", "2026-10-20")]
    .sort(compareAscents).map(x => x.id);
  expect(sorted).toEqual(["new", "new2", "old", "undated", "p2", "p1", "t"]);
});
