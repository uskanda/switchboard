import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { smarternews80 } from "@/lib/planQuestions.fixture";

/** FakeEventSource stands in for the browser's EventSource; the test sends the stream's events through it. */
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

/** serve answers the issue from a map the test changes as it goes, and keeps the replies posted. */
function serve(details: Record<string, unknown>) {
  const posts: { ref: string; text: string }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      if (key === "GET /api/inbox") return new Response(JSON.stringify(listing(item())), { status: 200 });
      if (key === "POST /api/reply") {
        posts.push(JSON.parse(String(init?.body)));
        return new Response(null, { status: 204 });
      }
      const m = /^GET \/api\/issue\?ref=(.*)$/.exec(key);
      if (m) return new Response(JSON.stringify(details[decodeURIComponent(m[1])]), { status: 200 });
      return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
    }),
  );
  return posts;
}

beforeEach(() => {
  FakeEventSource.last = null;
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => vi.unstubAllGlobals());

const t0 = "2026-11-01T10:00:00Z";
const item = (over: Record<string, unknown> = {}) => ({
  n: 1, state: "waiting", ref: "gh:o/r#80", project: "o/r", number: 80, title: "Filters", url: "https://github.test/o/r/issues/80",
  host: "laptop", waiting_since: t0, reason: "plan-questions", question: "Q1", updated_at: t0, notes: 0, ...over,
});
const listing = (...items: unknown[]) => ({ items, errors: [], fetched_at: t0 });

const asked = (body: string, at = t0) => ({ author: "agent", created_at: at, body, agent: true, kind: "waiting" });
const detail = (over: Record<string, unknown> = {}) => ({
  ref: "gh:o/r#80", title: "Filters", url: "https://github.test/o/r/issues/80", waiting: true, host: "laptop",
  state: "waiting", reason: "plan-questions", since: t0, post: smarternews80, approve_word: "/ok", author: "me",
  created_at: t0, body: "Make filters from a selection.", comments: [asked(smarternews80)], ...over,
});

const at = (path: string) =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );

const open = async () => {
  at("/issue/gh%3Ao%2Fr%2380");
  return screen.findByRole("button", { name: "この内容で返答" });
};
const group = (name: string) => within(screen.getByRole("group", { name }));
const preview = () => screen.getByRole("region", { name: "「この内容で返答」で送る文" }).textContent;

describe("the questions of a plan", () => {
  it("draw each group as radio buttons, with the recommended option chosen", async () => {
    serve({ "gh:o/r#80": detail() });
    await open();
    const q1 = group("Q1の選択肢");
    expect(q1.getAllByRole("radio").map((r) => (r as HTMLInputElement).value)).toEqual(["A", "B", "C", ""]);
    expect(q1.getByRole("radio", { name: /^C AとBの両方/ })).toBeChecked();
    expect(q1.getByRole("radio", { name: "選ばない（補足で答える）" })).not.toBeChecked();
    expect(group("範囲").getByRole("radio", { name: /^A 全体/ })).toBeChecked();
    expect(group("欄").getByRole("radio", { name: /^X タイトルと内容/ })).toBeChecked();
    expect(screen.getByRole("textbox", { name: "Q6の補足" })).toBeInTheDocument();
  });

  it("draw a question without options with its free text only", async () => {
    const body = "### 計画についての質問\n\n❓ **Q1** - **名前**\n\nAPIの名前をどうしますか。\n\n➡️ 今のまま。\n";
    serve({ "gh:o/r#80": detail({ post: body, comments: [asked(body)] }) });
    await open();
    expect(screen.queryAllByRole("radio")).toHaveLength(0);
    expect(screen.getByRole("textbox", { name: "Q1の補足" })).toBeInTheDocument();
    expect(screen.getByText("今のまま。")).toBeInTheDocument();
  });

  it("are not drawn for the questions of an earlier round", async () => {
    const first = "### 計画についての質問\n\n❓ **Q1** - **前**\n\nA: x\nB: y\n\n➡️ A\n";
    serve({ "gh:o/r#80": detail({ comments: [asked(first, "2026-11-01T09:00:00Z"), { author: "me", created_at: t0, body: "Q1 前: B", agent: false }, asked(smarternews80)] }) });
    await open();
    expect(screen.queryByRole("group", { name: "Q1の選択肢" })).toBeInTheDocument();
    expect(screen.getAllByRole("group", { name: "Q1の選択肢" })).toHaveLength(1);
    expect(screen.getByText(/^A: x/)).toBeInTheDocument();
  });

  it("leave the comment and the reply as they were when no question can be read", async () => {
    const body = "### 計画についての質問\n\nどちらにしますか。";
    serve({ "gh:o/r#80": detail({ post: body, comments: [asked(body)] }) });
    at("/issue/gh%3Ao%2Fr%2380");
    expect(await screen.findByText("問いごとの答えを、返答の欄に書いて送ってください。")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "この内容で返答" })).toBeNull();
    expect(screen.getByRole("button", { name: "返答を送る" })).toBeInTheDocument();
    expect(screen.queryByText(/\/ok/)).toBeNull();
  });

  it("leave out the hint on answering with the approval word", async () => {
    serve({ "gh:o/r#80": detail() });
    await open();
    expect(screen.queryByText(/推奨どおりでよければ/)).toBeNull();
    expect(screen.getByText(/この内容で返答」を押します/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /承認する/ })).toBeNull();
  });
});

describe("the reply to the questions", () => {
  it("is shown before sending and follows the choices", async () => {
    serve({ "gh:o/r#80": detail() });
    await open();
    expect(preview()).toMatch(/^Q1 操作の入口: C\nQ2 選んだあとの流れ: A\nQ3 ダイアログの既定値: 範囲はA、欄はX\n/);
    await userEvent.click(group("Q2の選択肢").getByRole("radio", { name: /^B / }));
    await userEvent.type(screen.getByRole("textbox", { name: "Q2の補足" }), "件数は出さない");
    expect(preview()).toMatch(/\nQ2 選んだあとの流れ: B。補足：件数は出さない\n/);
  });

  it("is posted as shown, with the reply field as a note on the whole", async () => {
    const posts = serve({ "gh:o/r#80": detail() });
    const send = await open();
    await userEvent.click(group("Q5の選択肢").getByRole("radio", { name: "選ばない（補足で答える）" }));
    await userEvent.type(screen.getByRole("textbox", { name: "返答" }), "急がない");
    const shown = preview();
    await userEvent.click(send);
    expect(await within(screen.getByRole("article")).findByRole("status")).toHaveTextContent("送りました");
    expect(posts).toEqual([{ ref: "gh:o/r#80", text: shown }]);
    expect(shown).toMatch(/\nQ5 Androidアプリの扱い: 回答なし\n/);
    expect(shown).toMatch(/\n\n全体への補足：\n急がない$/);
  });

  it("keeps the plain reply button for the reply field alone", async () => {
    const posts = serve({ "gh:o/r#80": detail() });
    await open();
    await userEvent.type(screen.getByRole("textbox", { name: "返答" }), "全部やり直して");
    await userEvent.click(screen.getByRole("button", { name: "返答を送る" }));
    await within(screen.getByRole("article")).findByRole("status");
    expect(posts).toEqual([{ ref: "gh:o/r#80", text: "全部やり直して" }]);
  });

  it("keeps the choices while the same questions are read again", async () => {
    const details: Record<string, unknown> = { "gh:o/r#80": detail() };
    serve(details);
    await open();
    const es = FakeEventSource.last!;
    es.emit("inbox", listing(item()));
    await userEvent.click(group("Q2の選択肢").getByRole("radio", { name: /^B / }));
    await userEvent.type(screen.getByRole("textbox", { name: "Q2の補足" }), "途中");

    details["gh:o/r#80"] = detail({ title: "Filters again" });
    es.emit("inbox", listing(item({ updated_at: "2026-11-01T10:05:00Z" })));
    expect(await screen.findByRole("heading", { name: "Filters again" })).toBeInTheDocument();
    expect(group("Q2の選択肢").getByRole("radio", { name: /^B / })).toBeChecked();
    expect(screen.getByRole("textbox", { name: "Q2の補足" })).toHaveValue("途中");
  });

  it("starts over with the questions of the next round", async () => {
    const details: Record<string, unknown> = { "gh:o/r#80": detail() };
    serve(details);
    await open();
    const es = FakeEventSource.last!;
    es.emit("inbox", listing(item()));
    await userEvent.type(screen.getByRole("textbox", { name: "Q1の補足" }), "古い");

    const next = "### 計画についての質問\n\n❓ **Q7** - **次**\n\nA: x\nB: y\n\n➡️ B\n";
    details["gh:o/r#80"] = detail({ post: next, comments: [asked(smarternews80), { author: "me", created_at: t0, body: "Q1 …", agent: false }, asked(next, "2026-11-01T11:00:00Z")] });
    es.emit("inbox", listing(item({ updated_at: "2026-11-01T11:00:00Z", notes: 1 })));
    await waitFor(() => expect(screen.getByRole("group", { name: "Q7の選択肢" })).toBeInTheDocument());
    expect(preview()).toBe("Q7 次: B");
  });

  it("can no longer be sent once the issue stops waiting", async () => {
    const details: Record<string, unknown> = { "gh:o/r#80": detail() };
    serve(details);
    await open();
    const es = FakeEventSource.last!;
    es.emit("inbox", listing(item()));
    await userEvent.click(group("Q2の選択肢").getByRole("radio", { name: /^B / }));

    details["gh:o/r#80"] = detail({ waiting: false, state: "running", reason: "" });
    es.emit("inbox", listing(item({ n: undefined, state: "running", progress: "p", updated_at: "2026-11-01T10:05:00Z" })));
    expect(await screen.findByText("このissueは待ちではなくなりました。返答は送れません。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "この内容で返答" })).toBeDisabled();
    expect(group("Q2の選択肢").getByRole("radio", { name: /^B / })).toBeChecked();
  });
});

describe("the approval of a plan", () => {
  const plan = "### 計画の承認のお願い\n\n1. フラグを足す。";
  const planned = (over: Record<string, unknown> = {}) =>
    detail({ reason: "plan", post: plan, comments: [{ ...asked(plan), kind: "waiting" }], ...over });

  it("can no longer be sent once the issue stops waiting, even with the form kept by a draft", async () => {
    const details: Record<string, unknown> = { "gh:o/r#80": planned() };
    serve(details);
    at("/issue/gh%3Ao%2Fr%2380");
    const approve = await screen.findByRole("button", { name: "承認する（/ok を送る）" });
    const es = FakeEventSource.last!;
    es.emit("inbox", listing(item({ reason: "plan" })));
    await userEvent.type(screen.getByRole("textbox", { name: "返答" }), " ");
    expect(approve).toBeEnabled();

    details["gh:o/r#80"] = planned({ waiting: false, state: "running", reason: "plan" });
    es.emit("inbox", listing(item({ reason: "plan", state: "running", updated_at: "2026-11-01T10:05:00Z" })));
    expect(await screen.findByText("このissueは待ちではなくなりました。返答は送れません。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "承認する（/ok を送る）" })).toBeDisabled();
  });
});
