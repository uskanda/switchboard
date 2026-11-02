import { describe, expect, it } from "vitest";
import type { Comment, Detail } from "@/lib/api";
import { addedCards, cardsOf, targetOf } from "@/lib/scrollTarget";

const agent = (m: number, kind: string, body = kind): Comment => ({ author: "bot", created_at: `2026-10-31T10:0${m}:00Z`, body, agent: true, kind });
const person = (m: number, body = "note"): Comment => ({ author: "me", created_at: `2026-10-31T10:0${m}:00Z`, body, agent: false });
const detail = (over: Partial<Detail>): Detail => ({
  ref: "gh:o/r#1", title: "t", url: "u", waiting: true, host: "h", state: "waiting", reason: "question", since: null, post: "", comments: [], ...over,
});

describe("targetOf", () => {
  it("is the agent's latest question of a waiting issue", () => {
    expect(targetOf(detail({ comments: [agent(1, "waiting"), agent(2, "running"), agent(3, "waiting"), person(4)] }))).toBe(2);
    expect(targetOf(detail({ comments: [person(1)] }))).toBeNull();
  });

  it("is the latest done report of a done issue, else its last card", () => {
    const done = { waiting: false, done: true, state: "done" };
    expect(targetOf(detail({ ...done, comments: [agent(1, "done"), person(2)] }))).toBe(0);
    expect(targetOf(detail({ ...done, comments: [person(1), agent(2, "progress", "a"), agent(3, "progress", "b"), person(4)] }))).toBe(3);
    expect(targetOf(detail({ ...done, comments: [] }))).toBeNull();
  });

  it("is null for a running or queued issue", () => {
    expect(targetOf(detail({ waiting: false, state: "running", comments: [agent(1, "waiting")] }))).toBeNull();
    expect(targetOf(detail({ waiting: false, state: "queued", comments: [agent(1, "done")] }))).toBeNull();
  });
});

describe("addedCards", () => {
  it("gives the indexes of the cards a read adds, a rewritten progress card among them", () => {
    const before = cardsOf([agent(1, "running"), agent(2, "progress", "a")]);
    const after = cardsOf([agent(1, "running"), agent(2, "progress", "a"), agent(3, "progress", "b"), person(4)]);
    expect(addedCards(before, after)).toEqual([2, 3]);
    expect(addedCards(after, after)).toEqual([]);
  });
});
