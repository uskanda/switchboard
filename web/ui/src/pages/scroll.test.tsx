import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { intersect, scrolled } from "@/test-observers";

/** FakeEventSource stands in for the browser's EventSource; the test sends the stream's events through it. */
class FakeEventSource {
  static last: FakeEventSource | null = null;
  onerror: (() => void) | null = null;
  private listeners: Record<string, ((e: MessageEvent) => void)[]> = {};
  constructor() {
    FakeEventSource.last = this;
  }
  addEventListener(type: string, fn: (e: MessageEvent) => void) {
    (this.listeners[type] ??= []).push(fn);
  }
  close() {}
  emit(type: string, data: unknown) {
    act(() => this.listeners[type]?.forEach((fn) => fn(new MessageEvent(type, { data: JSON.stringify(data) }))));
  }
}

/** serve answers GET /api/issue from details, which the test changes as it goes. */
function serve(details: Record<string, unknown>, inbox: unknown) {
  const reads: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string) => {
      if (input === "/api/inbox") return new Response(JSON.stringify(inbox), { status: 200 });
      const m = /^\/api\/issue\?ref=(.*)$/.exec(input);
      if (m) {
        reads.push(decodeURIComponent(m[1]));
        return new Response(JSON.stringify(details[decodeURIComponent(m[1])]), { status: 200 });
      }
      return new Response(JSON.stringify({ code: "not_found", message: input }), { status: 404 });
    }),
  );
  return reads;
}

beforeEach(() => {
  FakeEventSource.last = null;
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => vi.unstubAllGlobals());

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

const t0 = "2026-10-31T10:00:00Z";
const at_ = (m: number) => new Date(Date.parse(t0) + m * 60_000).toISOString();
const item = (over: Record<string, unknown> = {}) => ({
  n: 1, state: "waiting", ref: "gh:o/r#12", project: "o/r", number: 12, title: "Add a flag", url: "https://github.test/o/r/issues/12",
  host: "laptop", waiting_since: t0, reason: "question", question: "Which default?", updated_at: t0, notes: 0, ...over,
});
const listing = (...items: unknown[]) => ({ items, errors: [], fetched_at: t0 });
const agent = (m: number, kind: string, body: string) => ({ author: "bot", created_at: at_(m), body, agent: true, kind });
const person = (m: number, body: string) => ({ author: "me", created_at: at_(m), body, agent: false });
const detail = (over: Record<string, unknown> = {}) => ({
  ref: "gh:o/r#12", title: "Add a flag", url: "https://github.test/o/r/issues/12", waiting: true, host: "laptop",
  state: "waiting", reason: "question", since: t0, post: "Which default?", author: "me", created_at: t0,
  body: "Please add --dry-run.", comments: [], ...over,
});
const running = (comments: unknown[]) => detail({ waiting: false, state: "running", reason: "", comments });
const done = (comments: unknown[]) => detail({ waiting: false, done: true, state: "done", reason: "?", comments });

/** card is the thread's card that shows text. */
const card = (text: string) => screen.getByText(text).closest("li")!;
const notice = () => screen.queryByRole("button", { name: "自動更新がありました" });

const asked = [agent(1, "running", "claim"), agent(2, "waiting", "Pick a default."), person(3, "A side note.")];

describe("opening an issue", () => {
  it("scrolls once to the agent's latest question of a waiting issue, even with a comment after it", async () => {
    serve({ "gh:o/r#12": detail({ comments: asked }) }, listing(item()));
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByText("A side note.")).toBeInTheDocument();
    expect(scrolled).toEqual([card("Pick a default.")]);
  });

  it("scrolls to the agent's done report of a done issue, or to the last card without one", async () => {
    serve(
      {
        "gh:o/r#12": done([agent(1, "running", "claim"), agent(2, "done", "The work is done"), person(3, "Thanks.")]),
        "gh:o/r#13": done([agent(1, "running", "claim"), person(2, "First."), person(3, "Last.")]),
      },
      listing(item({ state: "done" }), item({ n: 2, state: "done", ref: "gh:o/r#13", number: 13, title: "Other" })),
    );
    at("/issue/gh%3Ao%2Fr%2312");
    await screen.findByText("Thanks.");
    expect(scrolled).toEqual([card("The work is done")]);
    cleanup();
    scrolled.length = 0;

    at("/issue/gh%3Ao%2Fr%2313");
    await screen.findByText("Last.");
    expect(scrolled).toEqual([card("Last.")]);
  });

  it("does not scroll a running or queued issue", async () => {
    serve(
      {
        "gh:o/r#12": running([agent(1, "running", "claim"), agent(2, "progress", "Halfway.")]),
        "gh:o/r#13": detail({ ref: "gh:o/r#13", waiting: false, state: "queued", reason: "host", comments: [agent(1, "queued", "q"), person(2, "Later.")] }),
      },
      listing(item({ state: "running" }), item({ n: 2, state: "queued", ref: "gh:o/r#13", number: 13 })),
    );
    at("/issue/gh%3Ao%2Fr%2312");
    await screen.findByText("Halfway.");
    cleanup();
    at("/issue/gh%3Ao%2Fr%2313");
    await screen.findByText("Later.");
    expect(scrolled).toEqual([]);
  });

  it("scrolls once when the cached read is drawn, and not again nor with a notice when the read arrives", async () => {
    const details: Record<string, unknown> = { "gh:o/r#12": detail({ comments: asked }) };
    serve(details, listing(item()));
    at("/issue/gh%3Ao%2Fr%2312");
    await screen.findByText("A side note.");
    cleanup();
    scrolled.length = 0;

    details["gh:o/r#12"] = detail({ comments: [...asked, person(4, "Another note.")] });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(scrolled).toEqual([card("Pick a default.")]);
    expect(await screen.findByText("Another note.")).toBeInTheDocument();
    expect(scrolled).toHaveLength(1);
    expect(notice()).not.toBeInTheDocument();
  });
});

describe("the notice of an update", () => {
  it("shows when a read adds a card, keeps the place, and not when only the state or time moves", async () => {
    const details: Record<string, unknown> = { "gh:o/r#12": detail({ comments: asked }) };
    const reads = serve(details, listing(item()));
    at("/issue/gh%3Ao%2Fr%2312");
    await screen.findByText("A side note.");
    const es = FakeEventSource.last!;
    const before = reads.length;

    details["gh:o/r#12"] = detail({ comments: asked, since: at_(9), reason: "verify" });
    es.emit("inbox", listing(item({ updated_at: at_(9), reason: "verify" })));
    await waitFor(() => expect(reads.length).toBe(before + 1));
    await act(async () => {});
    expect(notice()).not.toBeInTheDocument();

    details["gh:o/r#12"] = detail({ comments: [...asked, person(5, "One more.")] });
    es.emit("inbox", listing(item({ updated_at: at_(10), notes: 1 })));
    expect(await screen.findByText("One more.")).toBeInTheDocument();
    expect(notice()).toBeInTheDocument();
    expect(scrolled).toHaveLength(1);
  });

  it("goes to the question when a run turns into a wait, and goes away once pressed", async () => {
    const claim = agent(1, "running", "claim");
    const details: Record<string, unknown> = { "gh:o/r#12": running([claim, agent(2, "progress", "Halfway.")]) };
    serve(details, listing(item({ state: "running" })));
    at("/issue/gh%3Ao%2Fr%2312");
    await screen.findByText("Halfway.");
    const es = FakeEventSource.last!;

    details["gh:o/r#12"] = detail({ comments: [claim, agent(2, "progress", "Halfway."), agent(3, "progress", "Done with it."), agent(4, "waiting", "Pick a default.")] });
    es.emit("inbox", listing(item({ updated_at: at_(4) })));
    await screen.findByText("Pick a default.");
    await userEvent.click(notice()!);
    expect(scrolled).toEqual([card("Pick a default.")]);
    expect(notice()).not.toBeInTheDocument();
  });

  it("goes to the rewritten progress card of a running issue", async () => {
    const claim = agent(1, "running", "claim");
    const details: Record<string, unknown> = { "gh:o/r#12": running([claim, agent(2, "progress", "Halfway.")]) };
    serve(details, listing(item({ state: "running", progress: "a" })));
    at("/issue/gh%3Ao%2Fr%2312");
    await screen.findByText("Halfway.");

    details["gh:o/r#12"] = running([claim, agent(2, "progress", "Halfway."), agent(3, "progress", "Almost.")]);
    FakeEventSource.last!.emit("inbox", listing(item({ state: "running", progress: "b" })));
    await screen.findByText("Almost.");
    await userEvent.click(notice()!);
    expect(scrolled).toEqual([card("Almost.")]);
  });

  it("goes away when its card comes into view, or when another issue is opened", async () => {
    const details: Record<string, unknown> = {
      "gh:o/r#12": detail({ comments: asked }),
      "gh:o/r#13": detail({ ref: "gh:o/r#13", title: "Other", comments: [] }),
    };
    const two = (over: Record<string, unknown> = {}) => listing(item(over), item({ n: 2, ref: "gh:o/r#13", number: 13, title: "Other" }));
    serve(details, two());
    at("/issue/gh%3Ao%2Fr%2312");
    await screen.findByText("A side note.");
    const es = FakeEventSource.last!;

    details["gh:o/r#12"] = detail({ comments: [...asked, person(5, "One more.")] });
    es.emit("inbox", two({ updated_at: at_(5), notes: 1 }));
    await screen.findByText("One more.");
    expect(notice()).toBeInTheDocument();
    // A waiting issue's notice leads to its question; scrolling that card into view is reading what is new.
    intersect(card("Pick a default."), false);
    expect(notice()).toBeInTheDocument();
    intersect(card("Pick a default."), true);
    expect(notice()).not.toBeInTheDocument();

    details["gh:o/r#12"] = detail({ comments: [...asked, person(5, "One more."), person(6, "And another.")] });
    es.emit("inbox", two({ updated_at: at_(6), notes: 2 }));
    await screen.findByText("And another.");
    expect(notice()).toBeInTheDocument();
    await userEvent.click(screen.getAllByRole("link", { name: /Other/ })[0]);
    await screen.findByRole("heading", { level: 2, name: "Other" });
    expect(notice()).not.toBeInTheDocument();
  });
});

describe("the pinned head of an issue", () => {
  it("holds the way back, the reference, the title and the forge link", async () => {
    serve({ "gh:o/r#12": detail({ comments: asked }) }, listing(item()));
    at("/issue/gh%3Ao%2Fr%2312");
    const title = await screen.findByRole("heading", { level: 2, name: "Add a flag" });
    const head = within(title.closest("[data-pane-head]") as HTMLElement);
    expect(head.getByRole("link", { name: "← 一覧へ" })).toBeInTheDocument();
    expect(head.getByText("gh:o/r#12")).toBeInTheDocument();
    expect(head.getByRole("link", { name: /フォージで開く/ })).toHaveAttribute("href", "https://github.test/o/r/issues/12");
  });

  it("shows the whole title at the top, and two lines of it with the rest in title once scrolled", async () => {
    serve({ "gh:o/r#12": detail({ comments: asked }) }, listing(item()));
    at("/issue/gh%3Ao%2Fr%2312");
    const title = await screen.findByRole("heading", { level: 2, name: "Add a flag" });
    expect(title).not.toHaveClass("line-clamp-2");
    expect(title).not.toHaveAttribute("title");

    const top = document.querySelector("[data-pane-top]")!;
    intersect(top, false);
    expect(title).toHaveClass("line-clamp-2");
    expect(title).toHaveAttribute("title", "Add a flag");
    intersect(top, true);
    expect(title).not.toHaveClass("line-clamp-2");
  });
});
