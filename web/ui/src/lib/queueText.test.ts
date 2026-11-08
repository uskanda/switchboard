import { describe, expect, it } from "vitest";
import { queueText } from "@/lib/api";

describe("queueText", () => {
  it("names the slot a queued issue waits on", () => {
    expect(queueText("ja", "max-parallel")).toBe("端末の上限に達しています");
    expect(queueText("ja", "project-max-parallel")).toBe("プロジェクトの上限に達しています");
  });

  it("names a pause of the host with the time it ends, in local time", () => {
    const until = new Date(2026, 10, 3, 18, 30).toISOString();
    expect(queueText("ja", "limit", until)).toBe("利用上限に達しています（11/3 18:30まで）");
    expect(queueText("ja", "quota", until)).toBe("APIのクオータに達しています（11/3 18:30まで）");
  });

  it("leaves the time out when there is none", () => {
    expect(queueText("ja", "limit")).toBe("利用上限に達しています");
    expect(queueText("ja", "quota", null)).toBe("APIのクオータに達しています");
  });
});

describe("queueText in English", () => {
  it("names the slot and the pause, with the time it ends", () => {
    const until = new Date(2026, 10, 3, 18, 30).toISOString();
    expect(queueText("en", "max-parallel")).toBe("Host limit reached");
    expect(queueText("en", "project-max-parallel")).toBe("Project limit reached");
    expect(queueText("en", "limit", until)).toBe("Usage limit reached (until Nov 3, 18:30)");
    expect(queueText("en", "quota", until)).toBe("API quota reached (until Nov 3, 18:30)");
    expect(queueText("en", "quota")).toBe("API quota reached");
  });

  it("pads the hour as every English time does, and leaves the Japanese one unpadded", () => {
    const until = new Date(2026, 10, 3, 9, 5).toISOString();
    expect(queueText("en", "limit", until)).toBe("Usage limit reached (until Nov 3, 09:05)");
    expect(queueText("ja", "limit", until)).toBe("利用上限に達しています（11/3 9:05まで）");
  });
});
