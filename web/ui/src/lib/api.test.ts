import { describe, expect, it } from "vitest";
import { age, autoMergeReason, detailState, elapsed, itemState, oldestWaiting, progressText, stateDiffers, timeText, type Detail, type Item } from "@/lib/api";

const now = new Date("2026-11-03T12:00:00Z");
const ago = (min: number) => new Date(now.getTime() - min * 60_000).toISOString();

describe("timeText", () => {
  it("says what the time of each state is", () => {
    expect(timeText("ja", "running", ago(2), now)).toBe("2分経過");
    expect(timeText("ja", "running", ago(63), now)).toBe("1時間3分経過");
    expect(timeText("ja", "waiting", ago(5), now)).toBe("5分前から待ち");
    expect(timeText("ja", "queued", ago(3), now)).toBe("3分前から順番待ち");
    expect(timeText("ja", "done", ago(60), now)).toBe("1時間前に完了");
    expect(timeText("ja", "waiting", ago(2 * 24 * 60), now)).toBe("2日前から待ち");
  });

  it("words a time under a minute without 1分未満前", () => {
    expect(timeText("ja", "running", ago(0), now)).toBe("始めて1分未満");
    expect(timeText("ja", "waiting", ago(0), now)).toBe("待ちになって1分未満");
    expect(timeText("ja", "queued", ago(0), now)).toBe("順番待ちになって1分未満");
    expect(timeText("ja", "done", ago(0), now)).toBe("完了して1分未満");
  });

  it("keeps the question mark without a time", () => {
    for (const s of ["running", "waiting", "queued", "done"] as const) {
      expect(timeText("ja", s, null, now)).toBe("?");
      expect(timeText("ja", s, undefined, now)).toBe("?");
    }
  });
});

describe("progressText", () => {
  it("shows the words after the first separator", () => {
    expect(progressText("ja", "13:41 7分 Bash 12・StructuredOutput 1 ｜ テストを直している")).toBe("テストを直している");
    expect(progressText("ja", "13:41 7分 Bash 12 | fixing the tests")).toBe("fixing the tests");
    expect(progressText("ja", "13:41 7分 Bash 12 ｜ a | b ｜ c")).toBe("a | b ｜ c");
  });

  it("says the run is working without words yet", () => {
    for (const p of ["14:24 2分 Bash 1 ｜ ?", "14:24 2分 Bash 1 ｜", "14:24 2分 Bash 1", "?", "", undefined]) {
      expect(progressText("ja", p)).toBe("作業しています");
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

describe("the state of a row and of a detail", () => {
  const row = (over: Partial<Item> = {}): Item => ({
    ref: "a", project: "o/r", number: 1, title: "a", url: "", host: "h", waiting_since: null, reason: "question", question: "", ...over,
  });
  const read = (over: Partial<Detail> = {}): Detail => ({
    ref: "a", title: "a", url: "", waiting: true, host: "h", state: "waiting", reason: "question", since: null, post: "", ...over,
  });

  it("tells running, queued, done and waiting apart on a row", () => {
    expect(itemState(row({ state: "running" }))).toBe("running");
    expect(itemState(row({ state: "queued" }))).toBe("queued");
    expect(itemState(row({ state: "done" }))).toBe("done");
    expect(itemState(row({ state: "waiting" }))).toBe("waiting");
    // A back end older than the running list leaves the state out: its items all wait.
    expect(itemState(row())).toBe("waiting");
  });

  it("tells them apart on a detail, a run or a queue before the labels of a wait or a done", () => {
    expect(detailState(read({ state: "running", waiting: true }))).toBe("running");
    expect(detailState(read({ state: "queued", waiting: true }))).toBe("queued");
    expect(detailState(read({ state: "done", waiting: false, done: true }))).toBe("done");
    expect(detailState(read())).toBe("waiting");
  });

  it("differs only on the state, not on the reason", () => {
    expect(stateDiffers(row({ state: "queued" }), read({ state: "running" }))).toBe(true);
    expect(stateDiffers(row({ state: "waiting" }), read({ state: "done", waiting: false, done: true }))).toBe(true);
    expect(stateDiffers(row({ state: "waiting", reason: "question" }), read({ reason: "plan" }))).toBe(false);
    expect(stateDiffers(row({ state: "running" }), read({ state: "running" }))).toBe(false);
  });
});

describe("timeText in English", () => {
  it("says what the time of each state is, in the CLI's short units", () => {
    expect(timeText("en", "running", ago(2), now)).toBe("2m elapsed");
    expect(timeText("en", "running", ago(63), now)).toBe("1h 3m elapsed");
    expect(timeText("en", "waiting", ago(5), now)).toBe("waiting 5m");
    expect(timeText("en", "waiting", ago(2 * 24 * 60), now)).toBe("waiting 2d");
    expect(timeText("en", "queued", ago(3), now)).toBe("queued 3m");
    expect(timeText("en", "done", ago(60), now)).toBe("done 1h ago");
  });

  it("words a time under a minute", () => {
    expect(timeText("en", "running", ago(0), now)).toBe("started just now");
    expect(timeText("en", "waiting", ago(0), now)).toBe("waiting under 1m");
    expect(timeText("en", "queued", ago(0), now)).toBe("queued under 1m");
    expect(timeText("en", "done", ago(0), now)).toBe("done just now");
  });

  it("keeps the question mark without a time", () => {
    expect(timeText("en", "waiting", null, now)).toBe("?");
  });
});

describe("age and elapsed", () => {
  it("are short in English", () => {
    expect(age("en", ago(0), now)).toBe("just now");
    expect(age("en", ago(5), now)).toBe("5m ago");
    expect(age("en", ago(180), now)).toBe("3h ago");
    expect(age("en", ago(3 * 24 * 60), now)).toBe("3d ago");
    expect(elapsed("en", ago(0), now)).toBe("under 1m");
    expect(elapsed("en", ago(7), now)).toBe("7m");
    expect(elapsed("en", ago(125), now)).toBe("2h 5m");
  });

  it("stay as they were in Japanese", () => {
    expect(age("ja", ago(0), now)).toBe("1分未満");
    expect(age("ja", ago(5), now)).toBe("5分前");
    expect(age("ja", ago(180), now)).toBe("3時間前");
    expect(age("ja", ago(3 * 24 * 60), now)).toBe("3日前");
    expect(age("ja", null, now)).toBe("?");
    expect(elapsed("ja", ago(0), now)).toBe("1分未満");
    expect(elapsed("ja", ago(7), now)).toBe("7分");
    expect(elapsed("ja", ago(125), now)).toBe("2時間5分");
    expect(elapsed("ja", undefined, now)).toBe("?");
  });
});

describe("progressText in English", () => {
  it("says the run is working without words yet, and keeps the agent's words as they are", () => {
    expect(progressText("en", "14:24 2分 Bash 1 ｜ ?")).toBe("Working");
    expect(progressText("en", "13:41 7分 Bash 12 ｜ テストを直している")).toBe("テストを直している");
  });
});

describe("autoMergeReason", () => {
  it("explains each reason in the language shown", () => {
    expect(autoMergeReason("ja", "user")).toBe("取り消しました。");
    expect(autoMergeReason("en", "user")).toBe("Cancelled.");
    expect(autoMergeReason("en", "forbidden")).toBe("The inbox token has no permission to merge.");
  });

  it("shows a reason it does not know as it came", () => {
    expect(autoMergeReason("en", "something-new")).toBe("something-new");
    expect(autoMergeReason("en", undefined)).toBe("");
  });
});
