import { describe, expect, it, vi } from "vitest";
import type { Comment, Detail } from "@/lib/api";
import { addedCards, cardsOf, revealAndScroll, shown, targetOf, threadOf } from "@/lib/scrollTarget";

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

describe("threadOf", () => {
  /** shape writes the thread as the index of each card, and a round as [question, ...replies]. */
  const shape = (comments: Comment[]) => threadOf(comments).map((it) => (it.kind === "round" ? [it.question.i, ...it.replies.map((r) => r.i)] : it.i));

  it("folds every wait before the latest one with the people's replies after it", () => {
    const comments = [
      agent(0, "waiting"), person(1), agent(2, "running"),
      agent(3, "waiting"), person(4), person(5), agent(6, "running"), agent(7, "progress"),
      agent(8, "waiting"), person(9),
    ];
    expect(shape(comments)).toEqual([[0, 1], 2, [3, 4, 5], 6, 7, 8, 9]);
  });

  it("makes a round of a wait with no reply, and leaves the latest wait open", () => {
    const comments = [agent(1, "waiting"), agent(2, "running"), agent(3, "waiting")];
    expect(shape(comments)).toEqual([[0], 1, 2]);
    const latest = threadOf(comments)[2];
    expect(latest.kind !== "round" && latest.latest).toBe(true);
  });

  it("ends a round at the next post of the agent, whatever it is", () => {
    expect(shape([agent(1, "waiting"), person(2), agent(3, "done"), person(4), agent(5, "waiting")])).toEqual([[0, 1], 2, 3, 4]);
  });

  it("leaves a thread with one wait or none as shown draws it", () => {
    const comments = [person(1), agent(2, "running"), agent(3, "progress", "a"), agent(4, "progress", "b"), agent(5, "waiting")];
    expect(shape(comments)).toEqual(shown(comments).map((s) => s.i));
  });

  it("does not change the cards the thread draws", () => {
    const comments = [agent(1, "waiting"), person(2), agent(3, "running"), agent(4, "waiting")];
    expect(cardsOf(comments).map((c) => c.index)).toEqual([0, 1, 2, 3]);
  });
});

describe("addedCards", () => {
  it("gives the indexes of the cards a read adds, a rewritten progress card among them", () => {
    const before = cardsOf([agent(1, "running"), agent(2, "progress", "a")]);
    const after = cardsOf([agent(1, "running"), agent(2, "progress", "a"), agent(3, "progress", "b"), person(4)]);
    expect(addedCards(before, after)).toEqual([2, 3]);
    expect(addedCards(after, after)).toEqual([]);
  });

  it("finds a card a read rewrote inside a folded round", () => {
    const before = cardsOf([agent(1, "waiting"), person(2, "a"), agent(3, "running"), agent(4, "waiting")]);
    const after = cardsOf([agent(1, "waiting"), person(2, "b"), agent(3, "running"), agent(4, "waiting")]);
    expect(addedCards(before, after)).toEqual([1]);
  });
});

describe("revealAndScroll", () => {
  it("opens the folded round the card is in, then scrolls to it", () => {
    document.body.innerHTML = `<ol><li><details><summary>s</summary><ol><li data-index="1">a</li></ol></details></li><li data-index="2">b</li></ol>`;
    const scroll = vi.fn();
    Element.prototype.scrollIntoView = scroll;
    revealAndScroll(document.body, 1);
    expect(document.querySelector("details")!.open).toBe(true);
    expect(scroll).toHaveBeenCalledWith({ block: "start" });
    expect(scroll.mock.contexts[0]).toBe(document.querySelector('[data-index="1"]'));
  });

  it("scrolls to a card outside a round without opening one, and does nothing for a missing card", () => {
    document.body.innerHTML = `<ol><li><details><summary>s</summary><ol><li data-index="1">a</li></ol></details></li><li data-index="2">b</li></ol>`;
    const scroll = vi.fn();
    Element.prototype.scrollIntoView = scroll;
    revealAndScroll(document.body, 2);
    expect(document.querySelector("details")!.open).toBe(false);
    expect(scroll).toHaveBeenCalledTimes(1);
    revealAndScroll(document.body, 9);
    revealAndScroll(null, 2);
    expect(scroll).toHaveBeenCalledTimes(1);
  });
});
