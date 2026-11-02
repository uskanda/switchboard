import { describe, expect, it } from "vitest";
import { clampListWidth, LIST_WIDTH_DEFAULT, LIST_WIDTH_MAX, LIST_WIDTH_MIN, MAIN_WIDTH_MIN, readListWidth } from "@/lib/paneWidth";

describe("clampListWidth", () => {
  it("keeps a width inside the bounds", () => {
    expect(clampListWidth(400, 1440)).toBe(400);
  });

  it("does not go under the minimum", () => {
    expect(clampListWidth(100, 1440)).toBe(LIST_WIDTH_MIN);
  });

  it("does not go over the maximum", () => {
    expect(clampListWidth(2000, 2560)).toBe(LIST_WIDTH_MAX);
  });

  it("leaves the issue pane its minimum on a narrow window", () => {
    expect(clampListWidth(600, 1024)).toBe(1024 - MAIN_WIDTH_MIN);
  });

  it("falls back to the minimum when the window cannot hold both", () => {
    expect(clampListWidth(500, 600)).toBe(LIST_WIDTH_MIN);
  });

  it("rounds to whole pixels", () => {
    expect(clampListWidth(400.6, 1440)).toBe(401);
  });
});

describe("readListWidth", () => {
  it("is the default when nothing is stored", () => {
    expect(readListWidth(null)).toBe(LIST_WIDTH_DEFAULT);
  });

  it("is the default when the stored value is not a number", () => {
    expect(readListWidth("wide")).toBe(LIST_WIDTH_DEFAULT);
  });

  it("is the stored width", () => {
    expect(readListWidth("420")).toBe(420);
  });
});
