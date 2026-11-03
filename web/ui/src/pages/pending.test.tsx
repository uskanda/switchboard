import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { baseOf, pendingTimeout, storageKey, type Mark } from "@/lib/pending";
import type { Item } from "@/lib/api";

/** FakeEventSource stands in for the browser's EventSource; the test sends the stream's listings through it. */
class FakeEventSource {
  static last: FakeEventSource | null = null;
  onerror: (() => void) | null = null;
  private listeners: Record<string, ((e: MessageEvent) => void)[]> = {};
  constructor(readonly url: string) {
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

type Answer = { status: number; body?: unknown };

/** answer makes fetch reply by "METHOD path". */
function answer(routes: Record<string, Answer>) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      const a = routes[key];
      if (!a) return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
      return new Response(a.status === 204 ? null : JSON.stringify(a.body), { status: a.status });
    }),
  );
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
const waiting: Item = {
  n: 1, state: "waiting", ref: "gh:o/r#12", project: "o/r", number: 12, title: "Add a flag", url: "https://github.test/o/r/issues/12",
  host: "laptop", waiting_since: t0, reason: "question", question: "Which default?", updated_at: t0, notes: 0,
};
const done: Item = {
  n: 2, state: "done", ref: "gh:o/r#50", project: "o/r", number: 50, title: "Finished", url: "https://github.test/o/r/issues/50",
  host: "desk", waiting_since: null, reason: "", question: "?", done_since: t0,
  change_url: "https://github.test/o/r/pull/9", change_number: 9, change_sha: "abc",
};
const listing = (...items: Item[]) => ({ status: 200, body: { items, errors: [], fetched_at: t0 } });
const stream = (...items: Item[]) => FakeEventSource.last!.emit("inbox", { items, errors: [], fetched_at: t0 });

const detail = {
  ref: "gh:o/r#12", title: "Add a flag", url: "https://github.test/o/r/issues/12", waiting: true, host: "laptop",
  state: "waiting", reason: "question", since: t0, post: "Which default?", author: "me", created_at: t0, body: "", comments: [],
};

/** section is the list's section under the heading name. */
const section = (name: string) => screen.getByRole("heading", { level: 2, name }).closest("section")!;

describe("a reply", () => {
  it("moves the issue under running as being processed until the agent takes it up", async () => {
    answer({ "GET /api/inbox": listing(waiting), "GET /api/issue?ref=gh%3Ao%2Fr%2312": { status: 200, body: detail }, "POST /api/reply": { status: 204 } });
    at("/issue/gh%3Ao%2Fr%2312");
    await waitFor(() => expect(document.title).toBe("(1) Add a flag - SwitchBoard Inbox"));
    await userEvent.type(await screen.findByRole("textbox", { name: "返答" }), "Go with B.");
    await userEvent.click(screen.getByRole("button", { name: "返答を送る" }));

    const running = await waitFor(() => section("実行中"));
    expect(within(running).getAllByText("Add a flag").length).toBeGreaterThan(0);
    expect(within(running).getAllByRole("status")[0]).toHaveTextContent("処理中");
    expect(within(running).getAllByText("実行の開始を待っています").length).toBeGreaterThan(0);
    expect(within(running).getAllByText("処理中")[0]).toHaveClass("pending-pulse");
    expect(within(section("返答待ち")).getByText("待ちのissueはありません")).toBeInTheDocument();
    expect(screen.getByText("待ち0件、実行中1件")).toBeInTheDocument();
    await waitFor(() => expect(document.title).toBe("Add a flag - SwitchBoard Inbox"));

    // The same wait listed again keeps the mark; the run taking the issue up ends it.
    stream(waiting);
    expect(within(section("実行中")).getAllByText("実行の開始を待っています").length).toBeGreaterThan(0);
    stream({ ...waiting, state: "running", running_since: "2026-10-31T10:05:00Z", progress: "10:06 1分 Edit 1 ｜ テストを書いています" });
    expect(within(section("実行中")).queryByText("実行の開始を待っています")).toBeNull();
    expect(within(section("実行中")).getAllByText("テストを書いています").length).toBeGreaterThan(0);
    await waitFor(() => expect(sessionStorage.getItem(storageKey)).toBeNull());
  });
});

describe("a merge", () => {
  it("keeps the issue under done as being merged, with the button off, until the issue leaves the list", async () => {
    answer({ "GET /api/inbox": listing(done), "POST /api/merge": { status: 204 } });
    at("/");
    await userEvent.click((await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0]);
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "マージする" }));

    const doneSection = section("完了（close待ち）");
    await waitFor(() => expect(within(doneSection).getAllByText("マージ処理中").length).toBeGreaterThan(0));
    expect(within(doneSection).getAllByText("closeされると一覧から消えます").length).toBeGreaterThan(0);
    for (const b of screen.getAllByRole("button", { name: "PR #9をマージ" })) expect(b).toBeDisabled();

    stream();
    expect(screen.queryByText("マージ処理中")).toBeNull();
    await waitFor(() => expect(sessionStorage.getItem(storageKey)).toBeNull());
  });

  it("marks nothing when the merge is refused", async () => {
    answer({ "GET /api/inbox": listing(done), "POST /api/merge": { status: 409, body: { code: "stale", message: "moved" } } });
    at("/");
    await userEvent.click((await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0]);
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "マージする" }));
    await screen.findByRole("alert");
    expect(screen.queryByText("マージ処理中")).toBeNull();
    expect(sessionStorage.getItem(storageKey)).toBeNull();
  });
});

describe("a new issue", () => {
  it("shows a running row of its own until the list has the issue", async () => {
    answer({
      "GET /api/inbox": listing(),
      "GET /api/issue-targets": { status: 200, body: { items: [{ ref: "gl:nas/sb", host: "laptop" }], starts: [{ kind: "ready", label: "agent:ready" }] } },
      "POST /api/issues": { status: 201, body: { ref: "gl:nas/sb#7", url: "https://gitlab.test/nas/sb/-/issues/7" } },
    });
    at("/");
    await userEvent.click(await screen.findByRole("button", { name: "新規issue" }));
    const dialog = await screen.findByRole("dialog", { name: "新規issue" });
    await within(dialog).findByRole("option", { name: /nas\/sb/ });
    await userEvent.type(within(dialog).getByRole("textbox", { name: "タイトル" }), "Add a flag");
    await userEvent.click(within(dialog).getByRole("button", { name: "作成する" }));

    const running = await waitFor(() => section("実行中"));
    expect(within(running).getAllByText("Add a flag").length).toBeGreaterThan(0);
    expect(within(running).getAllByText("実行の開始を待っています").length).toBeGreaterThan(0);
    expect(within(running).getAllByRole("link", { name: /Add a flag/ })[0]).toHaveAttribute("href", "/issue/gl%3Anas%2Fsb%237");

    const real = { ...waiting, ref: "gl:nas/sb#7", project: "nas/sb", number: 7, state: "running" as const, running_since: t0, progress: "10:06 1分 Read 2 ｜ 読んでいます" };
    stream(real);
    expect(within(section("実行中")).queryByText("実行の開始を待っています")).toBeNull();
    expect(within(section("実行中")).getAllByText("読んでいます").length).toBeGreaterThan(0);
  });
});

describe("a mark", () => {
  it("comes back from the tab's storage on a reload", async () => {
    const mark: Mark = { ref: waiting.ref, kind: "reply", at: Date.now(), base: baseOf(waiting) };
    sessionStorage.setItem(storageKey, JSON.stringify([mark]));
    answer({ "GET /api/inbox": listing(waiting) });
    at("/");
    await waitFor(() => expect(within(section("実行中")).getAllByText("実行の開始を待っています").length).toBeGreaterThan(0));
  });

  it("goes after ten minutes, with the issue back where the list has it and a toast that says so", async () => {
    const mark: Mark = { ref: waiting.ref, kind: "reply", at: Date.now() - pendingTimeout + 300, base: baseOf(waiting) };
    sessionStorage.setItem(storageKey, JSON.stringify([mark]));
    answer({ "GET /api/inbox": listing(waiting) });
    at("/");
    await waitFor(() => expect(within(section("実行中")).getAllByText("実行の開始を待っています").length).toBeGreaterThan(0));
    expect(await screen.findByText(/の操作はまだ反映されていません/, {}, { timeout: 2000 })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 2, name: "実行中" })).toBeNull();
    expect(within(section("返答待ち")).getAllByText("Add a flag").length).toBeGreaterThan(0);
    expect(sessionStorage.getItem(storageKey)).toBeNull();
  });
});
