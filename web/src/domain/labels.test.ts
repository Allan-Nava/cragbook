import { describe, expect, it } from "vitest";
import { dayLabel, many, today } from "./labels";

describe("labels", () => {
  it("counts in Italian", () => {
    expect(many(1, "via", "vie")).toBe("1 via");
    expect(many(3, "via", "vie")).toBe("3 vie");
  });
  it("reads a day as a calendar day", () => {
    expect(dayLabel("2026-10-20")).toBe("20 ott 2026");
  });
  it("gives today in the browser's zone", () => {
    expect(today(new Date(2026, 0, 5, 23, 59))).toBe("2026-01-05");
  });
});
