import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import type { Item } from "@/lib/api";
import { parseProgress } from "@/lib/api";
import { intersect } from "@/test-observers";

// The fixes of issues #181 to #185 (openspec/changes/fix-web-ui-audit-design-2026-11-08).

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
afterEach(() => vi.unstubAllGlobals());

const at = (p: string) =>
  render(
    <MemoryRouter initialEntries={[p]}>
      <AppRoutes />
    </MemoryRouter>,
  );

const t0 = new Date(Date.now() - 5 * 60_000).toISOString();
const t1 = new Date(Date.now() - 60_000).toISOString();
const waiting = (n: number, title: string, over: Partial<Item> = {}): Item => ({
  n, state: "waiting", ref: `gh:o/r#${n}`, project: "o/r", number: n, title, url: "https://github.test/o/r/issues/" + n,
  host: "desk", waiting_since: t0, reason: "question", question: "q", updated_at: t0, notes: 0, ...over,
});
const listing = (...items: Item[]) => ({ items, errors: [], fetched_at: t0 });
const asked = { author: "agent", created_at: t0, body: "どちらにしますか", agent: true, kind: "waiting" };
const detailOf = (it: Item, over: Record<string, unknown> = {}) => ({
  ref: it.ref, title: it.title, url: it.url, waiting: true, host: "desk", state: "waiting", reason: it.reason, since: t0,
  post: "q", author: "me", created_at: t0, body: "本文", comments: [asked], ...over,
});

const progressBody =
  "### 実行の進捗\n\n- 12:01 1分 Read 2 ｜ 設定を読んでいます\n- 12:02 2分 Edit 1・Read 2 ｜ 一覧の並びを直しています\n\n<!-- switchboard:progress -->\n";

describe("the progress of a run (#185)", () => {
  it("is read line by line, the words after the first separator", () => {
    expect(parseProgress(progressBody)).toEqual([
      { head: "12:01 1分 Read 2", words: "設定を読んでいます" },
      { head: "12:02 2分 Edit 1・Read 2", words: "一覧の並びを直しています" },
    ]);
    expect(parseProgress("ただの文")).toBeNull();
  });

  it("shows the latest words as the card, and every line in a folded log", async () => {
    const run = { ...waiting(1, "Running"), state: "running" as const, running_since: t0 };
    routes({
      "GET /api/inbox": () => json(200, listing(run)),
      "GET /api/issue?ref=gh%3Ao%2Fr%231": () =>
        json(200, detailOf(run, { waiting: false, state: "running", comments: [{ author: "agent", created_at: t1, body: progressBody, agent: true, kind: "progress" }] })),
    });
    at("/issue/gh%3Ao%2Fr%231");
    const thread = await screen.findByRole("list", { name: "スレッド" });
    // The latest words lead the card, and come again as the last line of the log.
    const card = (await within(thread).findAllByText("一覧の並びを直しています"))[0];
    expect(card.closest("details")).toBeNull();
    expect(card.closest("li")).toHaveTextContent("12:02 2分 Edit 1・Read 2");
    const log = within(thread).getByText("進捗の記録（2行）").closest("details")!;
    expect(log).not.toHaveAttribute("open");
    expect(within(log).getByText("設定を読んでいます")).toBeInTheDocument();
    expect(within(thread).queryByRole("heading", { name: "実行の進捗" })).toBeNull();
  });
});

describe("a reply just sent (#182)", () => {
  const A = waiting(1, "Issue A");

  async function sendFromA(details: { current: Record<string, unknown> }) {
    routes({
      "GET /api/inbox": () => json(200, listing(A)),
      "GET /api/issue?ref=gh%3Ao%2Fr%231": () => json(200, details.current),
      "POST /api/reply": () => json(204),
    });
    at("/issue/gh%3Ao%2Fr%231");
    await userEvent.type(await screen.findByRole("textbox", { name: "返答" }), "Bにします");
    await userEvent.click(screen.getByRole("button", { name: "返答を送る" }));
    await screen.findByText("送りました。エージェントが作業を再開します。");
  }

  it("shows the reply in the thread at once, and once only after the forge has it", async () => {
    const details = { current: detailOf(A) as Record<string, unknown> };
    await sendFromA(details);
    const thread = screen.getByRole("list", { name: "スレッド" });
    expect(within(thread).getByText("Bにします")).toBeInTheDocument();
    expect(within(thread).getByText("送信済み・フォージへの反映待ち")).toBeInTheDocument();

    details.current = detailOf(A, { comments: [asked, { author: "me", created_at: t1, body: "Bにします", agent: false }] });
    FakeEventSource.last!.emit("inbox", listing({ ...A, notes: 1, updated_at: t1 }));
    await waitFor(() => expect(within(thread).queryByText("送信済み・フォージへの反映待ち")).toBeNull());
    expect(within(thread).getAllByText("Bにします")).toHaveLength(1);
  });

  it("says the issue is being processed, then that the agent took the reply up", async () => {
    const details = { current: detailOf(A) as Record<string, unknown> };
    await sendFromA(details);
    const head = document.querySelector<HTMLElement>("[data-pane-head]")!;
    expect(within(head).getByText("処理中")).toBeInTheDocument();

    details.current = detailOf(A, { waiting: false, state: "running", comments: [asked, { author: "me", created_at: t1, body: "Bにします", agent: false }] });
    FakeEventSource.last!.emit("inbox", listing({ ...A, state: "running" as const, running_since: t1, updated_at: t1, notes: 1 }));
    expect(
      await screen.findByText("返答を受けて、エージェントが作業を再開しました。質問か完了になると、この画面が自動で切り替わります。"),
    ).toBeInTheDocument();
  });
});

describe("the pinned head on a narrow screen (#181)", () => {
  it("shrinks to a back arrow beside the title and the state once scrolled", async () => {
    const A = waiting(1, "A long title of an issue");
    routes({
      "GET /api/inbox": () => json(200, listing(A)),
      "GET /api/issue?ref=gh%3Ao%2Fr%231": () => json(200, detailOf(A)),
    });
    at("/issue/gh%3Ao%2Fr%231");
    const head = await waitFor(() => {
      const h = document.querySelector<HTMLElement>("[data-pane-head]");
      expect(h).not.toBeNull();
      return h!;
    });
    await within(head).findByRole("heading", { level: 2 });
    expect(head).not.toHaveAttribute("data-compact");
    act(() => intersect(document.querySelector("[data-pane-top]")!, false));
    expect(head).toHaveAttribute("data-compact");
    const back = within(head).getByRole("link", { name: "一覧へ" });
    expect(back).toHaveTextContent("←");
    expect(back.parentElement).toContainElement(within(head).getByRole("heading", { level: 2 }));
    expect(within(head).getByRole("heading", { level: 2 })).toHaveClass("max-lg:line-clamp-1");
    expect(within(head).getByText("desk")).toHaveClass("max-lg:hidden");
  });
});

describe("the right pane with nothing chosen (#183)", () => {
  it("counts the sections and the waits by reason", async () => {
    routes({
      "GET /api/inbox": () =>
        json(
          200,
          listing(
            waiting(1, "Q1"),
            waiting(2, "Q2"),
            waiting(3, "C", { reason: "ci" }),
            { ...waiting(4, "R"), state: "running" as const, running_since: t0 },
          ),
        ),
    });
    at("/");
    const summary = await screen.findByRole("region", { name: "受信箱の概要" });
    const count = (term: string) => within(summary).getByText(term).nextElementSibling?.textContent;
    expect(count("返答待ち")).toBe("3件");
    expect(count("実行中")).toBe("1件");
    expect(count("順番待ち")).toBe("0件");
    expect(count("完了")).toBe("0件");
    const reasons = within(summary).getByRole("list", { name: "待ちの理由" });
    const rows = within(reasons).getAllByRole("listitem");
    expect(rows[0]).toHaveTextContent("question質問2件");
    expect(rows[1]).toHaveTextContent("ciCIの失敗1件");
  });
});

describe("the buttons at the top of the list (#184)", () => {
  it("share the row of the heading of the waits", async () => {
    routes({ "GET /api/inbox": () => json(200, listing(waiting(1, "Q1"))) });
    at("/");
    const heading = await screen.findByRole("heading", { name: "返答待ち" });
    const row = heading.parentElement!;
    expect(within(row).getByRole("button", { name: "新規issue" })).toBeInTheDocument();
    const toggle = within(row).getByRole("button", { name: "プロジェクトでまとめる" });
    expect(within(toggle).getByText("まとめる")).toHaveClass("sm:hidden");
  });
});
