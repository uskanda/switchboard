import { describe, expect, it } from "vitest";
import { asOf, clockText, fullText } from "@/lib/freshness";

// Local times, so the tests hold in any time zone.
const at = (month: number, day: number, h: number, m: number, s = 0) => new Date(2026, month - 1, day, h, m, s);
const now = at(11, 2, 15, 0);

describe("asOf", () => {
  it("takes the later of the listing's time and the last time the updates were live", () => {
    expect(asOf(at(11, 2, 9, 0).toISOString(), at(11, 2, 14, 32).toISOString())).toEqual(at(11, 2, 14, 32));
    expect(asOf(at(11, 2, 14, 40).toISOString(), at(11, 2, 14, 32).toISOString())).toEqual(at(11, 2, 14, 40));
  });

  it("takes the listing's time while the updates were never live", () => {
    expect(asOf(at(11, 2, 9, 5).toISOString(), null)).toEqual(at(11, 2, 9, 5));
  });

  it("gives nothing for a time it cannot read", () => {
    expect(asOf("", null)).toBeNull();
    expect(asOf("not a time", null)).toBeNull();
  });
});

describe("clockText", () => {
  it("gives hours and minutes for today", () => {
    expect(clockText(at(11, 2, 9, 5), now)).toBe("09:05");
  });

  it("leads with the date on another day", () => {
    expect(clockText(at(11, 1, 14, 32), now)).toBe("11/1 14:32");
  });
});

describe("fullText", () => {
  it("gives the date and the time to the second", () => {
    expect(fullText(at(11, 2, 9, 5, 7))).toBe("2026/11/2 09:05:07");
  });
});
