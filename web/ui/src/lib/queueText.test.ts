import { describe, expect, it } from "vitest";
import { queueText } from "@/lib/api";

describe("queueText", () => {
  it("names the slot a queued issue waits on", () => {
    expect(queueText("max-parallel")).toBe("端末の上限に達しています");
    expect(queueText("project-max-parallel")).toBe("プロジェクトの上限に達しています");
  });

  it("names a pause of the host with the time it ends, in local time", () => {
    const until = new Date(2026, 10, 3, 18, 30).toISOString();
    expect(queueText("limit", until)).toBe("利用上限に達しています（11/3 18:30まで）");
    expect(queueText("quota", until)).toBe("APIのクオータに達しています（11/3 18:30まで）");
  });

  it("leaves the time out when there is none", () => {
    expect(queueText("limit")).toBe("利用上限に達しています");
    expect(queueText("quota", null)).toBe("APIのクオータに達しています");
  });
});
