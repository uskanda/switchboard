import { afterEach, describe, expect, it } from "vitest";
import type { Item } from "@/lib/api";
import { baseOf, loadMarks, overlay, pendingTimeout, saveMarks, settle, storageKey, type Mark } from "@/lib/pending";

const item = (over: Partial<Item>): Item => ({
  ref: "gh:o/r#1",
  project: "o/r",
  number: 1,
  title: "One",
  url: "https://github.test/o/r/issues/1",
  host: "desk",
  waiting_since: "2026-11-01T00:00:00Z",
  reason: "plan",
  question: "?",
  state: "waiting",
  ...over,
});

const waiting = item({});
const done = item({ ref: "gh:o/r#2", number: 2, state: "done", done_since: "2026-11-01T01:00:00Z", reason: "" });
const at = 1_000_000;

const reply: Mark = { ref: waiting.ref, kind: "reply", at, base: baseOf(waiting) };
const mergeMark: Mark = { ref: done.ref, kind: "merge", at, base: baseOf(done) };
const create: Mark = {
  ref: "gl:nas/sb#7",
  kind: "create",
  at,
  base: null,
  draft: { title: "Add a flag", url: "https://gitlab.test/nas/sb/-/issues/7", project: "nas/sb", number: 7 },
};

afterEach(() => sessionStorage.clear());

describe("settle", () => {
  it("keeps a mark while the issue stays as it was", () => {
    const out = settle([reply, mergeMark, create], [waiting, done], at + 1000);
    expect(out.keep).toEqual([reply, mergeMark, create]);
    expect(out.settled).toEqual([]);
    expect(out.expired).toEqual([]);
  });

  it("settles a mark once the issue's state changes", () => {
    const out = settle([reply], [{ ...waiting, state: "running", running_since: "2026-11-02T00:00:00Z" }], at + 1000);
    expect(out.keep).toEqual([]);
    expect(out.settled).toEqual([reply]);
  });

  it("settles a mark once only the start of the state changes", () => {
    const out = settle([reply], [{ ...waiting, waiting_since: "2026-11-02T00:00:00Z" }], at + 1000);
    expect(out.settled).toEqual([reply]);
  });

  it("settles a mark once the issue leaves the list", () => {
    const out = settle([mergeMark], [waiting], at + 1000);
    expect(out.settled).toEqual([mergeMark]);
  });

  it("settles a new issue's mark once the issue shows on the list", () => {
    const out = settle([create], [item({ ref: create.ref, state: "running" })], at + 1000);
    expect(out.settled).toEqual([create]);
  });

  it("expires a mark after ten minutes", () => {
    const out = settle([reply, create], [waiting], at + pendingTimeout);
    expect(out.keep).toEqual([]);
    expect(out.expired).toEqual([reply, create]);
  });

  it("only expires marks before the first listing", () => {
    expect(settle([reply], null, at + 1000).keep).toEqual([reply]);
    expect(settle([reply], null, at + pendingTimeout).expired).toEqual([reply]);
  });
});

describe("overlay", () => {
  it("shows a replied issue as running and pending, since the mark", () => {
    const [row] = overlay([waiting], [reply]);
    expect(row.state).toBe("running");
    expect(row.pending).toBe("reply");
    expect(row.running_since).toBe(new Date(at).toISOString());
  });

  it("keeps a merged issue done and marks it pending", () => {
    const [row] = overlay([done], [mergeMark]);
    expect(row.state).toBe("done");
    expect(row.pending).toBe("merge");
  });

  it("adds a running row for a new issue not on the list yet, with no machine", () => {
    const rows = overlay([waiting], [create]);
    expect(rows).toHaveLength(2);
    expect(rows[1]).toMatchObject({ ref: "gl:nas/sb#7", title: "Add a flag", url: create.draft!.url, number: 7, host: "", state: "running", pending: "create" });
  });

  it("leaves the other rows as they are", () => {
    expect(overlay([waiting, done], [])).toEqual([waiting, done]);
  });
});

describe("the marks in the tab's storage", () => {
  it("come back as they were saved", () => {
    saveMarks([reply, create]);
    expect(loadMarks()).toEqual([reply, create]);
  });

  it("are dropped when the stored value is not marks", () => {
    sessionStorage.setItem(storageKey, "{not json");
    expect(loadMarks()).toEqual([]);
    sessionStorage.setItem(storageKey, JSON.stringify({ ref: "x" }));
    expect(loadMarks()).toEqual([]);
    sessionStorage.setItem(storageKey, JSON.stringify([reply, { ref: 3, kind: "reply", at: 1, base: "" }, { ...reply, kind: "other" }]));
    expect(loadMarks()).toEqual([reply]);
  });

  it("are removed from the storage when none is left", () => {
    saveMarks([reply]);
    saveMarks([]);
    expect(sessionStorage.getItem(storageKey)).toBeNull();
  });
});
