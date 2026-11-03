import { payloadOf, routeOf } from "./payload";

const plan = { ref: "gh:o/r#20", state: "waiting", reason: "plan", since: "2026-11-02T09:30:00Z" };

describe("payloadOf", () => {
  it("reads the payload as content.data carries it", () => {
    expect(payloadOf(plan)).toEqual(plan);
  });

  it("reads it from a JSON string, as a background task gets it", () => {
    expect(payloadOf({ body: JSON.stringify(plan) })).toEqual(plan);
    expect(payloadOf({ dataString: JSON.stringify(plan) })).toEqual(plan);
  });

  it("is null for anything else", () => {
    expect(payloadOf(undefined)).toBeNull();
    expect(payloadOf({ body: "not json" })).toBeNull();
    expect(payloadOf({ ref: 12 })).toBeNull();
  });
});

describe("routeOf", () => {
  it("opens the issue's page on the notification itself", () => {
    expect(routeOf("expo.modules.notifications.actions.DEFAULT", plan)).toEqual({ kind: "open", path: "/issue/gh%3Ao%2Fr%2320" });
  });

  it("opens the merge dialog on merge", () => {
    const done = { ref: "gh:o/r#12", state: "done", since: "2026-11-02T09:30:00Z", change_number: 9, change_sha: "abc" };
    expect(routeOf("merge", done)).toEqual({ kind: "open", path: "/issue/gh%3Ao%2Fr%2312?action=merge" });
  });

  it("approves only a plan, with the start of its wait", () => {
    expect(routeOf("approve", plan)).toEqual({ kind: "approve", ref: "gh:o/r#20", since: "2026-11-02T09:30:00Z" });
    expect(routeOf("approve", { ...plan, reason: "question" })).toEqual({ kind: "open", path: "/issue/gh%3Ao%2Fr%2320" });
  });
});
