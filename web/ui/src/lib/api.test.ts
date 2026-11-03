import { describe, expect, it } from "vitest";
import { oldestWaiting, progressText, timeText, type Item } from "@/lib/api";

const now = new Date("2026-11-03T12:00:00Z");
const ago = (min: number) => new Date(now.getTime() - min * 60_000).toISOString();

describe("timeText", () => {
  it("says what the time of each state is", () => {
    expect(timeText("running", ago(2), now)).toBe("2分経過");
    expect(timeText("running", ago(63), now)).toBe("1時間3分経過");
    expect(timeText("waiting", ago(5), now)).toBe("5分前から待ち");
    expect(timeText("queued", ago(3), now)).toBe("3分前から順番待ち");
    expect(timeText("done", ago(60), now)).toBe("1時間前に完了");
    expect(timeText("waiting", ago(2 * 24 * 60), now)).toBe("2日前から待ち");
  });

  it("words a time under a minute without 1分未満前", () => {
    expect(timeText("running", ago(0), now)).toBe("始めて1分未満");
    expect(timeText("waiting", ago(0), now)).toBe("待ちになって1分未満");
    expect(timeText("queued", ago(0), now)).toBe("順番待ちになって1分未満");
    expect(timeText("done", ago(0), now)).toBe("完了して1分未満");
  });

  it("keeps the question mark without a time", () => {
    for (const s of ["running", "waiting", "queued", "done"] as const) {
      expect(timeText(s, null, now)).toBe("?");
      expect(timeText(s, undefined, now)).toBe("?");
    }
  });
});

describe("progressText", () => {
  it("shows the words after the first separator", () => {
    expect(progressText("13:41 7分 Bash 12・StructuredOutput 1 ｜ テストを直している")).toBe("テストを直している");
    expect(progressText("13:41 7分 Bash 12 | fixing the tests")).toBe("fixing the tests");
    expect(progressText("13:41 7分 Bash 12 ｜ a | b ｜ c")).toBe("a | b ｜ c");
  });

  it("says the run is working without words yet", () => {
    for (const p of ["14:24 2分 Bash 1 ｜ ?", "14:24 2分 Bash 1 ｜", "14:24 2分 Bash 1", "?", "", undefined]) {
      expect(progressText(p)).toBe("作業しています");
    }
  });
});

describe("oldestWaiting", () => {
  const it0 = (ref: string, over: Partial<Item> = {}): Item => ({
    ref, project: "o/r", number: 1, title: ref, url: "", host: "h", waiting_since: null, reason: "question", question: "", ...over,
  });

  it("is the waiting issue that has waited longest, one without a time last", () => {
    const items = [
      it0("a", { waiting_since: null }),
      it0("b", { waiting_since: ago(5) }),
      it0("c", { waiting_since: ago(30), state: "running" }),
      it0("d", { waiting_since: ago(20) }),
    ];
    expect(oldestWaiting(items)?.ref).toBe("d");
    expect(oldestWaiting([it0("a"), it0("b")])?.ref).toBe("a");
  });

  it("is none without a waiting issue", () => {
    expect(oldestWaiting([it0("a", { state: "running" })])).toBeUndefined();
    expect(oldestWaiting([])).toBeUndefined();
  });
});
