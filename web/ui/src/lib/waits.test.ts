import { afterEach, describe, expect, it } from "vitest";
import type { Item } from "@/lib/api";
import { forgetSent, isSentWait, markSent } from "@/lib/sent";
import { ignoreKey, nextWait, stepWait } from "@/lib/waits";

const wait = (n: number, over: Partial<Item> = {}) =>
  ({ ref: `gh:o/r#${n}`, number: n, title: `#${n}`, waiting_since: "2026-11-02T00:00:00Z", reason: "question", ...over }) as Item;

const [a, b, c] = [wait(12), wait(13), wait(14)];

afterEach(() => forgetSent());

describe("nextWait", () => {
  it("is the wait below the current one", () => {
    expect(nextWait([a, b, c], a.ref)?.ref).toBe(b.ref);
  });

  it("goes round from the end to the top", () => {
    expect(nextWait([a, b, c], c.ref)?.ref).toBe(a.ref);
  });

  it("is null when the current wait is the only one", () => {
    expect(nextWait([a], a.ref)).toBeNull();
  });

  it("starts from the top when the current issue is not a wait", () => {
    expect(nextWait([a, b], "gh:o/r#99")?.ref).toBe(a.ref);
  });

  it("skips a wait sent to in this tab", () => {
    markSent(b);
    expect(nextWait([a, b, c], a.ref)?.ref).toBe(c.ref);
  });

  it("is null when every other wait was sent to", () => {
    markSent(a);
    markSent(b);
    expect(nextWait([a, b], a.ref)).toBeNull();
  });

  it("does not skip an issue sent to that waits anew", () => {
    markSent(b);
    const again = wait(13, { waiting_since: "2026-11-02T01:00:00Z" });
    expect(isSentWait(again)).toBe(false);
    expect(nextWait([a, again, c], a.ref)?.ref).toBe(again.ref);
  });

  it("tells a wait sent to by its start, not by the comments added since", () => {
    markSent(b);
    expect(isSentWait({ ...b, updated_at: "2026-11-02T00:05:00Z", notes: 3 })).toBe(true);
  });
});

describe("stepWait", () => {
  it("moves down with 1 and up with -1", () => {
    expect(stepWait([a, b, c], b.ref, 1)?.ref).toBe(c.ref);
    expect(stepWait([a, b, c], b.ref, -1)?.ref).toBe(a.ref);
  });

  it("stops at the ends", () => {
    expect(stepWait([a, b, c], c.ref, 1)).toBeNull();
    expect(stepWait([a, b, c], a.ref, -1)).toBeNull();
  });

  it("opens the first wait down and the last one up when no wait is open", () => {
    expect(stepWait([a, b, c], undefined, 1)?.ref).toBe(a.ref);
    expect(stepWait([a, b, c], undefined, -1)?.ref).toBe(c.ref);
  });

  it("is null when there is no wait", () => {
    expect(stepWait([], undefined, 1)).toBeNull();
  });

  it("does not skip a wait sent to", () => {
    markSent(b);
    expect(stepWait([a, b, c], a.ref, 1)?.ref).toBe(b.ref);
  });
});

describe("ignoreKey", () => {
  const key = (init: KeyboardEventInit, target: EventTarget = document.body) => {
    const e = new KeyboardEvent("keydown", init);
    Object.defineProperty(e, "target", { value: target });
    return e;
  };

  afterEach(() => document.body.replaceChildren());

  it("takes a plain key on the page", () => {
    expect(ignoreKey(key({ key: "j" }))).toBe(false);
  });

  it("ignores a key with a modifier", () => {
    expect(ignoreKey(key({ key: "j", ctrlKey: true }))).toBe(true);
    expect(ignoreKey(key({ key: "j", altKey: true }))).toBe(true);
    expect(ignoreKey(key({ key: "j", metaKey: true }))).toBe(true);
  });

  it("ignores a key typed in a field", () => {
    for (const tag of ["input", "textarea", "select"]) {
      expect(ignoreKey(key({ key: "j" }, document.createElement(tag)))).toBe(true);
    }
    const editable = document.createElement("div");
    editable.contentEditable = "true";
    document.body.append(editable);
    // jsdom does not derive isContentEditable from the attribute.
    Object.defineProperty(editable, "isContentEditable", { value: true });
    expect(ignoreKey(key({ key: "j" }, editable))).toBe(true);
  });

  it("ignores a key while a dialog is open", () => {
    const dialog = document.createElement("dialog");
    dialog.setAttribute("open", "");
    document.body.append(dialog);
    expect(ignoreKey(key({ key: "j" }))).toBe(true);
  });
});
