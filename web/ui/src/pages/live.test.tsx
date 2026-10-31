import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

/** FakeEventSource stands in for the browser's EventSource; the test sends the stream's events through it. */
class FakeEventSource {
  static last: FakeEventSource | null = null;
  onerror: (() => void) | null = null;
  closed = false;
  private listeners: Record<string, ((e: MessageEvent) => void)[]> = {};
  constructor(readonly url: string) {
    FakeEventSource.last = this;
  }
  addEventListener(type: string, fn: (e: MessageEvent) => void) {
    (this.listeners[type] ??= []).push(fn);
  }
  close() {
    this.closed = true;
  }
  emit(type: string, data: unknown) {
    act(() => this.listeners[type]?.forEach((fn) => fn(new MessageEvent(type, { data: JSON.stringify(data) }))));
  }
  fail() {
    act(() => this.onerror?.());
  }
}

/** issues answers GET /api/issue from a map the test changes as it goes, and counts the reads. */
function serve(details: Record<string, unknown>, inbox: unknown) {
  const reads: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      if (key === "GET /api/inbox") return new Response(JSON.stringify(inbox), { status: 200 });
      const m = /^GET \/api\/issue\?ref=(.*)$/.exec(key);
      if (m) {
        const ref = decodeURIComponent(m[1]);
        reads.push(ref);
        return new Response(JSON.stringify(details[ref]), { status: 200 });
      }
      return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
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
const item = (over: Record<string, unknown> = {}) => ({
  n: 1, state: "waiting", ref: "gh:o/r#12", project: "o/r", number: 12, title: "Add a flag", url: "https://github.test/o/r/issues/12",
  host: "laptop", waiting_since: t0, reason: "question", question: "Which default?", updated_at: t0, notes: 0, ...over,
});
const listing = (...items: unknown[]) => ({ items, errors: [], fetched_at: t0 });

const detail = (over: Record<string, unknown> = {}) => ({
  ref: "gh:o/r#12", title: "Add a flag", url: "https://github.test/o/r/issues/12", waiting: true, host: "laptop",
  state: "waiting", reason: "question", since: t0, post: "Which default?", author: "me", created_at: t0,
  body: "Please add **--dry-run**.", comments: [], ...over,
});

describe("the header", () => {
  it("counts the waiting and running issues and says the updates are live", async () => {
    serve({}, listing());
    at("/");
    const es = FakeEventSource.last!;
    expect(es.url).toBe("/api/events");
    expect(screen.getByText("自動更新に接続しています")).toBeInTheDocument();
    es.emit("inbox", listing(item(), item({ n: 2, ref: "gh:o/r#13" }), item({ n: undefined, state: "running", ref: "gh:o/r#14", progress: "p" })));
    expect(await screen.findByText("待ち 2・実行中 1")).toBeInTheDocument();
    expect(screen.getByText("自動更新中")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "設定" })).toHaveAttribute("href", "/settings");
  });

  it("says the updates are cut while the stream is down", async () => {
    serve({}, listing());
    at("/");
    FakeEventSource.last!.emit("inbox", listing());
    FakeEventSource.last!.emit("unavailable", { code: "unavailable", message: "x" });
    expect(await screen.findByText("自動更新が切れています。再接続しています")).toBeInTheDocument();
    FakeEventSource.last!.emit("inbox", listing());
    expect(await screen.findByText("自動更新中")).toBeInTheDocument();
    FakeEventSource.last!.fail();
    expect(await screen.findByText("自動更新が切れています。再接続しています")).toBeInTheDocument();
  });
});

describe("the list", () => {
  it("takes each listing the stream sends without a reload", async () => {
    serve({}, listing(item()));
    at("/");
    expect((await screen.findAllByText("gh:o/r#12"))[0]).toBeInTheDocument();
    FakeEventSource.last!.emit("inbox", listing(item(), item({ n: 2, ref: "gh:o/r#13", title: "Fix a typo" })));
    expect((await screen.findAllByText("Fix a typo"))[0]).toBeInTheDocument();
    expect(screen.getByText("待ち 2・実行中 0")).toBeInTheDocument();
  });

  it("asks to pick an issue on / and shows the list next to the picked one", async () => {
    serve({ "gh:o/r#12": detail() }, listing(item()));
    const { unmount } = at("/");
    expect(await screen.findByText("左の一覧からissueを選んでください")).toBeInTheDocument();
    unmount();
    at("/issue/gh%3Ao%2Fr%2312");
    const nav = await screen.findByRole("navigation", { name: "一覧" });
    expect((await within(nav).findAllByText("gh:o/r#12"))[0].closest("a")).toHaveAttribute("aria-current", "page");
    expect(await screen.findByRole("heading", { level: 2, name: "Add a flag" })).toBeInTheDocument();
  });
});

describe("the issue", () => {
  it("is read once when it opens, though the first listing comes after", async () => {
    const reads = serve({ "gh:o/r#12": detail() }, listing(item()));
    at("/issue/gh%3Ao%2Fr%2312");
    await screen.findByRole("heading", { level: 2, name: "Add a flag" });
    FakeEventSource.last!.emit("inbox", listing(item()));
    await new Promise((r) => setTimeout(r, 20));
    expect(reads).toEqual(["gh:o/r#12"]);
  });

  it("is read again when its row moves, and not when another row does", async () => {
    const reads = serve({ "gh:o/r#12": detail() }, listing(item()));
    at("/issue/gh%3Ao%2Fr%2312");
    const es = FakeEventSource.last!;
    es.emit("inbox", listing(item()));
    await screen.findByRole("heading", { level: 2, name: "Add a flag" });
    await waitFor(() => expect(reads.length).toBeGreaterThanOrEqual(1));
    const before = reads.length;

    es.emit("inbox", listing(item(), item({ n: 2, ref: "gh:o/r#13" })));
    await new Promise((r) => setTimeout(r, 20));
    expect(reads.length).toBe(before);

    es.emit("inbox", listing(item({ updated_at: "2026-10-31T10:05:00Z" }), item({ n: 2, ref: "gh:o/r#13" })));
    await waitFor(() => expect(reads.length).toBe(before + 1));
  });

  it("keeps the draft when the issue stops waiting, and no longer sends it", async () => {
    const details: Record<string, unknown> = { "gh:o/r#12": detail() };
    serve(details, listing(item()));
    at("/issue/gh%3Ao%2Fr%2312");
    const es = FakeEventSource.last!;
    es.emit("inbox", listing(item()));
    const box = await screen.findByRole("textbox", { name: "返答" });
    await userEvent.type(box, "Go with B.");

    details["gh:o/r#12"] = detail({ waiting: false, state: "running" });
    es.emit("inbox", listing(item({ n: undefined, state: "running", progress: "p", updated_at: "2026-10-31T10:05:00Z" })));
    expect(await screen.findByText("このissueは待ちではなくなりました。返答は送れません。")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "返答" })).toHaveValue("Go with B.");
    expect(screen.getByRole("button", { name: "返答を送る" })).toBeDisabled();
  });

  it("stays shown, marked, when it leaves the list", async () => {
    const reads = serve({ "gh:o/r#12": detail() }, listing(item()));
    at("/issue/gh%3Ao%2Fr%2312");
    const es = FakeEventSource.last!;
    es.emit("inbox", listing(item()));
    await screen.findByRole("heading", { level: 2, name: "Add a flag" });
    await waitFor(() => expect(reads.length).toBeGreaterThanOrEqual(1));
    const before = reads.length;
    es.emit("inbox", listing());
    expect(await screen.findByText("このissueは、待ちでも実行中でもなくなりました。")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Add a flag" })).toBeInTheDocument();
    expect(reads.length).toBe(before);
  });
});

describe("the thread", () => {
  const thread = detail({
    body: 'Please tidy **README**.\n\n<img src=x onerror="alert(1)">\n\n![shot](https://example.test/a.png)\n\nSee [the guide](https://example.test/guide).',
    comments: [
      { author: "bot", created_at: "2026-10-31T10:01:00Z", body: "Started on host desk.", agent: true, kind: "running" },
      { author: "bot", created_at: "2026-10-31T10:02:00Z", body: "### 実行の進捗\n\n- old progress line", agent: true, kind: "progress" },
      { author: "bot", created_at: "2026-10-31T10:03:00Z", body: "### 実行の進捗\n\n- new progress line", agent: true, kind: "progress" },
      { author: "me", created_at: "2026-10-31T10:04:00Z", body: "Use the top one.", agent: false },
      { author: "bot", created_at: "2026-10-31T10:05:00Z", body: "### The agent has a question\n\nWhich README?", agent: true, kind: "waiting" },
    ],
  });

  it("draws Markdown, shows raw HTML as text, and turns images into links", async () => {
    serve({ "gh:o/r#12": thread }, listing(item()));
    at("/issue/gh%3Ao%2Fr%2312");
    const strong = await screen.findByText("README", { selector: "strong" });
    expect(strong).toBeInTheDocument();
    expect(screen.getByText(/<img src=x onerror="alert\(1\)">/)).toBeInTheDocument();
    expect(document.querySelector("img")).toBeNull();
    const image = screen.getByRole("link", { name: "画像：shot" });
    expect(image).toHaveAttribute("href", "https://example.test/a.png");
    const guide = screen.getByRole("link", { name: "the guide" });
    expect(guide).toHaveAttribute("target", "_blank");
    expect(guide).toHaveAttribute("rel", "noreferrer");
  });

  it("marks the agent, folds its claim, and keeps only the latest progress", async () => {
    serve({ "gh:o/r#12": thread }, listing(item()));
    at("/issue/gh%3Ao%2Fr%2312");
    const list = await screen.findByRole("list", { name: "スレッド" });
    expect(within(list).getByText("着手しました")).toBeInTheDocument();
    expect(within(list).queryByText("Started on host desk.")).toBeNull();
    expect(within(list).queryByText("old progress line")).toBeNull();
    expect(within(list).getByText("new progress line")).toBeInTheDocument();
    expect(within(list).getByText("Use the top one.")).toBeInTheDocument();
    // The claim, the progress and the question are the agent's; the person's comment is not marked.
    expect(within(list).getAllByText("エージェント")).toHaveLength(3);
    const question = within(list).getByText("Which README?").closest("[aria-current]");
    expect(question).toHaveAttribute("aria-current", "true");
  });
});
