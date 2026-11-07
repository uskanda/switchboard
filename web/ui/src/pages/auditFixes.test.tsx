import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import type { Item } from "@/lib/api";

// The fixes of the web inbox audit of 2026-11-07 (openspec/changes/fix-web-ui-audit-2026-11-07).

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

type Handler = (init?: RequestInit) => Promise<Response> | Response;

/** routes makes fetch answer by "METHOD path"; the list is empty unless given. */
function routes(r: Record<string, Handler>) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      const h = r[key];
      if (h) return h(init);
      if (key === "GET /api/inbox") return json(200, { items: [], errors: [], fetched_at: "" });
      return json(404, { code: "not_found", message: key });
    }),
  );
}

const json = (status: number, body?: unknown) => new Response(body === undefined ? null : JSON.stringify(body), { status });

beforeEach(() => {
  FakeEventSource.last = null;
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  localStorage.clear();
});

const at = (p: string) =>
  render(
    <MemoryRouter initialEntries={[p]}>
      <AppRoutes />
    </MemoryRouter>,
  );

const t0 = new Date(Date.now() - 5 * 60_000).toISOString();
const waiting = (n: number, title: string): Item => ({
  n, state: "waiting", ref: `gh:o/r#${n}`, project: "o/r", number: n, title, url: "https://github.test/o/r/issues/" + n,
  host: "h", waiting_since: t0, reason: "question", question: "q", updated_at: t0, notes: 0,
});
const detailOf = (it: Item) => ({
  ref: it.ref, title: it.title, url: it.url, waiting: true, host: "h", state: "waiting", reason: "question", since: t0,
  post: "q", author: "me", created_at: t0, body: "", comments: [],
});
const issueKey = (n: number) => `GET /api/issue?ref=gh%3Ao%2Fr%23${n}`;

describe("a reply in flight while another issue opens", () => {
  const A = waiting(1, "Issue A");
  const B = waiting(2, "Issue B");

  async function sendFromAThenOpenB(answer: () => Promise<Response>) {
    let release!: () => void;
    routes({
      "GET /api/inbox": () => json(200, { items: [A, B], errors: [], fetched_at: t0 }),
      [issueKey(1)]: () => json(200, detailOf(A)),
      [issueKey(2)]: () => json(200, detailOf(B)),
      "POST /api/reply": () => new Promise((res) => (release = () => void answer().then(res))),
    });
    const user = userEvent.setup();
    at("/issue/gh%3Ao%2Fr%232");
    await user.type(await screen.findByRole("textbox", { name: "返答" }), "draft for B");
    await user.click(screen.getAllByRole("link", { name: /Issue A/ })[0]);
    await screen.findByRole("heading", { level: 2, name: "Issue A" });
    await user.type(await screen.findByRole("textbox", { name: "返答" }), "answer A");
    await user.click(screen.getByRole("button", { name: "返答を送る" }));
    await user.click(screen.getAllByRole("link", { name: /Issue B/ })[0]);
    await screen.findByRole("heading", { level: 2, name: "Issue B" });
    await act(async () => release());
    return user;
  }

  it("leaves the shown issue's draft and form alone when the reply is sent", async () => {
    await sendFromAThenOpenB(async () => json(204));
    await waitFor(() => expect(screen.getByRole("textbox", { name: "返答" })).not.toBeDisabled());
    expect(screen.getByRole("textbox", { name: "返答" })).toHaveValue("draft for B");
    expect(screen.queryByText("送りました。エージェントが作業を再開します。")).toBeNull();
  });

  it("drops the draft of the issue the reply went to once it was sent", async () => {
    const user = await sendFromAThenOpenB(async () => json(204));
    await waitFor(() => expect(screen.getByRole("textbox", { name: "返答" })).not.toBeDisabled());
    await user.click(screen.getAllByRole("link", { name: /Issue A/ })[0]);
    await screen.findByRole("heading", { level: 2, name: "Issue A" });
    expect(await screen.findByRole("textbox", { name: "返答" })).toHaveValue("");
  });

  it("tells a failure in a toast, not on the shown issue, and keeps the draft of the issue it went to", async () => {
    const user = await sendFromAThenOpenB(async () => json(503, { code: "unavailable", message: "down" }));
    expect(await screen.findByText(/受信箱のサービス/, { selector: "[data-sonner-toast] *" })).toBeInTheDocument();
    expect(within(screen.getByRole("article")).queryByRole("alert")).toBeNull();
    expect(screen.getByRole("textbox", { name: "返答" })).toHaveValue("draft for B");
    await user.click(screen.getAllByRole("link", { name: /Issue A/ })[0]);
    await screen.findByRole("heading", { level: 2, name: "Issue A" });
    expect(await screen.findByRole("textbox", { name: "返答" })).toHaveValue("answer A");
  });
});

describe("the login state", () => {
  it("shows the login page on a 401 even when the login status could not be read", async () => {
    routes({
      "GET /api/auth": () => json(503, { code: "unavailable", message: "down" }),
      "GET /api/inbox": () => json(401, { code: "unauthenticated", message: "log in" }),
    });
    at("/");
    expect(await screen.findByRole("link", { name: "GitLabでログイン" })).toBeInTheDocument();
  });

  it("stays logged in and says so when the logout fails", async () => {
    routes({
      "GET /api/auth": () => json(200, { enabled: true, user: "alice" }),
      "POST /api/logout": () => json(503, { code: "unavailable", message: "down" }),
    });
    at("/");
    expect(await screen.findByText("alice")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "ログアウト" }));
    expect(await screen.findByText("ログアウトできませんでした")).toBeInTheDocument();
    expect(screen.getByText("alice")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "GitLabでログイン" })).toBeNull();
  });
});

describe("the merge button after an automatic merge is cancelled", () => {
  it("can be pressed again once the list drops auto_merge", async () => {
    const done: Item = {
      n: 2, state: "done", ref: "gh:o/r#50", project: "o/r", number: 50, title: "Finished", url: "u", host: "desk",
      waiting_since: null, reason: "", question: "?", done_since: t0, change_url: "https://x/pull/9", change_number: 9,
      change_sha: "abc", updated_at: t0, notes: 0,
    };
    const detail = {
      ref: done.ref, title: "Finished", url: "u", waiting: false, done: true, host: "desk", state: "done", reason: "",
      since: t0, post: "", author: "me", created_at: t0, body: "", comments: [], change_url: "https://x/pull/9",
      change_number: 9, change_sha: "abc",
    };
    routes({
      "GET /api/inbox": () => json(200, { items: [done], errors: [], fetched_at: t0 }),
      [`GET /api/issue?ref=gh%3Ao%2Fr%2350`]: () => json(200, detail),
      "GET /api/merge-status?ref=gh%3Ao%2Fr%2350&sha=abc": () =>
        json(200, { state: "open", head_moved: false, mergeable: "conflict", ci: "none", checks: [] }),
      "POST /api/merge": () => json(202, { code: "auto_merge_pending", message: "x" }),
      "POST /api/automerge/cancel": () => json(204),
    });
    const user = userEvent.setup();
    at("/issue/gh%3Ao%2Fr%2350");
    await screen.findByRole("heading", { level: 2, name: "Finished" });
    const head = document.querySelector<HTMLElement>("[data-pane-head]")!;
    await user.click(await within(head).findByRole("button", { name: "PR #9をマージ" }));
    await user.click(within(head).getByRole("button", { name: "マージする" }));
    await within(head).findByRole("button", { name: "PR #9は自動マージ待ち" });
    FakeEventSource.last!.emit("inbox", { items: [{ ...done, auto_merge: { sha: "abc", since: t0 } }], errors: [], fetched_at: t0 });
    await user.click(within(head).getByRole("button", { name: "自動マージを取り消す" }));
    await user.click(within(head).getByRole("button", { name: "取り消す" }));
    FakeEventSource.last!.emit("inbox", { items: [done], errors: [], fetched_at: t0 });
    const button = await within(head).findByRole("button", { name: "PR #9をマージ" });
    expect(button).not.toBeDisabled();
  });
});

describe("the usability fixes", () => {
  const done = (n: number): Item => ({
    n, state: "done", ref: `gh:o/r#${n}`, project: "o/r", number: n, title: `Done ${n}`, url: "u", host: "desk",
    waiting_since: null, reason: "", question: "?", done_since: t0, updated_at: t0, notes: 0,
  });

  it("says once, under the heading of the done section, what closing and commenting do", async () => {
    routes({ "GET /api/inbox": () => json(200, { items: [done(1), done(2)], errors: [], fetched_at: t0 }) });
    at("/");
    const heading = await screen.findByRole("heading", { name: "完了（close待ち）" });
    const section = heading.closest("section")!;
    expect(within(section).getAllByText("closeすると一覧から消えます。コメントすると作業を再開します")).toHaveLength(1);
  });

  it("moves the time on without a new listing", async () => {
    const since = new Date(Date.now() - 20_000).toISOString();
    routes({ "GET /api/inbox": () => json(200, { items: [{ ...waiting(1, "Fresh"), waiting_since: since }], errors: [], fetched_at: since }) });
    vi.useFakeTimers({ shouldAdvanceTime: true, toFake: ["setInterval", "clearInterval", "Date"] });
    at("/");
    await screen.findAllByText("待ちになって1分未満");
    act(() => vi.advanceTimersByTime(2 * 60_000));
    expect(screen.getAllByText("2分前から待ち").length).toBeGreaterThan(0);
    expect(screen.queryAllByText("待ちになって1分未満")).toHaveLength(0);
  });

  it("says a reply to a failed CI is a 返答", async () => {
    const ci = { ...waiting(1, "CI"), reason: "ci" };
    routes({
      "GET /api/inbox": () => json(200, { items: [ci], errors: [], fetched_at: t0 }),
      [issueKey(1)]: () => json(200, { ...detailOf(ci), reason: "ci" }),
    });
    at("/issue/gh%3Ao%2Fr%231");
    expect(await screen.findByText(/返答するとエージェントが実行を再開し/)).toBeInTheDocument();
  });

  it("says a running issue's page changes by itself, and names the forge link for the eye and the ear", async () => {
    const run: Item = { ...waiting(1, "Running"), state: "running", running_since: t0, progress: "12:01 Edit ｜ working" };
    routes({
      "GET /api/inbox": () => json(200, { items: [run], errors: [], fetched_at: t0 }),
      [issueKey(1)]: () => json(200, { ...detailOf(run), waiting: false, state: "running" }),
    });
    at("/issue/gh%3Ao%2Fr%231");
    expect(await screen.findByText("実行中です。質問か完了になると、この画面が自動で切り替わります。")).toBeInTheDocument();
    const link = screen.getByRole("link", { name: "フォージで開く（新しいタブ）" });
    expect(link).toHaveTextContent("フォージで開く ↗");
  });
});
