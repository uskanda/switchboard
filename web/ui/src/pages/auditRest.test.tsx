import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import type { Item } from "@/lib/api";
import { smarternews80 } from "@/lib/planQuestions.fixture";

// The rest of the fixes of the web inbox audit of 2026-11-07 (openspec/changes/fix-web-ui-audit-rest-2026-11-07),
// issues #172 to #180; #171 is in web/internal/server/auth_test.go and here for the login page's link.

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

/** routes makes fetch answer by "METHOD path" and records the keys asked; the list is empty unless given. */
function routes(r: Record<string, Handler>) {
  const asked: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      asked.push(key);
      const h = r[key];
      if (h) return h(init);
      if (key === "GET /api/inbox") return json(200, { items: [], errors: [], fetched_at: "" });
      return json(404, { code: "not_found", message: key });
    }),
  );
  return asked;
}

const json = (status: number, body?: unknown) => new Response(body === undefined ? null : JSON.stringify(body), { status });

beforeEach(() => {
  FakeEventSource.last = null;
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
  sessionStorage.clear();
});

const at = (p: string) =>
  render(
    <MemoryRouter initialEntries={[p]}>
      <AppRoutes />
    </MemoryRouter>,
  );

const t0 = new Date(Date.now() - 5 * 60_000).toISOString();
const waiting = (n: number, title: string, over: Partial<Item> = {}): Item => ({
  n, state: "waiting", ref: `gh:o/r#${n}`, project: "o/r", number: n, title, url: "https://github.test/o/r/issues/" + n,
  host: "h", waiting_since: t0, reason: "question", question: "q", updated_at: t0, notes: 0, ...over,
});
const detailOf = (it: Item, over: Record<string, unknown> = {}) => ({
  ref: it.ref, title: it.title, url: it.url, waiting: true, host: "h", state: "waiting", reason: it.reason, since: t0,
  post: "q", author: "me", created_at: t0, body: "", comments: [], ...over,
});
const listing = (...items: Item[]) => ({ items, errors: [], fetched_at: t0 });

describe("the login page (#171)", () => {
  it("asks the login to come back to the page that was opened", async () => {
    routes({
      "GET /api/auth": () => json(200, { enabled: true }),
    });
    at("/issue/gh%3Ao%2Fr%2312?action=merge");
    const link = await screen.findByRole("link", { name: "GitLabでログイン" });
    expect(link).toHaveAttribute("href", "/auth/login?next=" + encodeURIComponent("/issue/gh%3Ao%2Fr%2312?action=merge"));
  });

  it("asks for nothing more from the list", async () => {
    routes({ "GET /api/auth": () => json(200, { enabled: true }) });
    at("/");
    expect(await screen.findByRole("link", { name: "GitLabでログイン" })).toHaveAttribute("href", "/auth/login");
  });
});

describe("the scroll on a narrow screen (#172)", () => {
  it("starts an issue at its top, and the list where it was left", async () => {
    const A = waiting(1, "Issue A");
    routes({
      "GET /api/inbox": () => json(200, listing(A)),
      "GET /api/issue?ref=gh%3Ao%2Fr%231": () => json(200, detailOf(A, { waiting: false, state: "running" })),
    });
    const scrollTo = vi.fn();
    vi.stubGlobal("scrollTo", scrollTo);
    at("/");
    const link = (await screen.findAllByRole("link", { name: /Issue A/ }))[0];
    Object.defineProperty(window, "scrollY", { value: 900, configurable: true });
    fireEvent.scroll(window);
    scrollTo.mockClear();
    await userEvent.click(link);
    await screen.findByRole("heading", { level: 2, name: "Issue A" });
    expect(scrollTo).toHaveBeenCalledWith(0, 0);
    Object.defineProperty(window, "scrollY", { value: 300, configurable: true });
    scrollTo.mockClear();
    await userEvent.click(screen.getByRole("link", { name: "← 一覧へ" }));
    await waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 900));
  });
});

describe("the dialog for a new issue (#173)", () => {
  it("does not close on Escape while the issue is being created", async () => {
    let release!: () => void;
    routes({
      "GET /api/issue-targets": () =>
        json(200, { items: [{ ref: "gh:o/r", host: "box" }], starts: [{ kind: "ready", label: "agent:ready" }, { kind: "none", label: "" }] }),
      "POST /api/issues": () => new Promise((res) => (release = () => res(json(503, { code: "unavailable", message: "down" })))),
    });
    at("/");
    await userEvent.click(await screen.findByRole("button", { name: "新規issue" }));
    const dialog = await screen.findByRole("dialog", { name: "新規issue" });
    await waitFor(() => expect(within(dialog).getByRole("combobox", { name: "プロジェクト" })).toHaveTextContent("o/r"));
    await userEvent.type(within(dialog).getByRole("textbox", { name: "タイトル" }), "New");
    await userEvent.click(within(dialog).getByRole("button", { name: "作成する" }));
    await within(dialog).findByRole("button", { name: "作成しています" });
    const cancel = new Event("cancel", { cancelable: true });
    act(() => void dialog.dispatchEvent(cancel));
    expect(cancel.defaultPrevented).toBe(true);
    await act(async () => release());
    expect(await within(dialog).findByRole("alert")).toHaveTextContent("受信箱のサービス");
    const after = new Event("cancel", { cancelable: true });
    act(() => void dialog.dispatchEvent(after));
    expect(after.defaultPrevented).toBe(false);
  });
});

describe("an issue just created (#174)", () => {
  it("says the agent has yet to take it up, not that it left the list", async () => {
    const asked = routes({
      "GET /api/issue-targets": () =>
        json(200, { items: [{ ref: "gh:o/r", host: "box" }], starts: [{ kind: "ready", label: "agent:ready" }, { kind: "none", label: "" }] }),
      "POST /api/issues": () => json(201, { ref: "gh:o/r#77", url: "https://github.test/o/r/issues/77" }),
      "GET /api/issue?ref=gh%3Ao%2Fr%2377": () =>
        json(200, { ref: "gh:o/r#77", title: "New", url: "https://github.test/o/r/issues/77", waiting: false, host: "", state: "", reason: "", since: null, post: "", author: "me", created_at: t0, body: "", comments: [] }),
    });
    at("/");
    await userEvent.click(await screen.findByRole("button", { name: "新規issue" }));
    const dialog = await screen.findByRole("dialog", { name: "新規issue" });
    await waitFor(() => expect(within(dialog).getByRole("combobox", { name: "プロジェクト" })).toHaveTextContent("o/r"));
    await userEvent.type(within(dialog).getByRole("textbox", { name: "タイトル" }), "New");
    await userEvent.click(within(dialog).getByRole("button", { name: "作成する" }));
    await userEvent.click((await screen.findAllByRole("link", { name: /New/ }))[0]);
    await screen.findByRole("heading", { level: 2, name: "New" });
    await waitFor(() => expect(asked).toContain("GET /api/issue?ref=gh%3Ao%2Fr%2377"));
    expect(await screen.findByText("作ったばかりのissueです。エージェントが取りかかると、この画面が自動で切り替わります。")).toBeInTheDocument();
    expect(screen.queryByText("このissueは、待ちでも実行中でもなくなりました。")).toBeNull();
    expect(screen.queryByText("このissueは待ちではありません。返答は送れません。")).toBeNull();
  });
});

describe("the settings (#175)", () => {
  it("drops the error once the instructions are read", async () => {
    let first = true;
    routes({
      "GET /api/instructions": () => {
        const was = first;
        first = false;
        return was ? json(503, { code: "unavailable", message: "down" }) : json(200, { host: "h", items: [{ target: "", kind: "plan", content: "" }] });
      },
      "GET /api/projects": () => json(200, { host: "h", enabled: true, items: [] }),
      "POST /api/projects": () => json(201, { project: { ref: "gh:o/n", path: "/p", cloned: false, plan_first: true, missing_labels: [] } }),
    });
    at("/settings");
    expect(await screen.findByText("受信箱のサービス（switchboard inbox serve）に届きません。")).toBeInTheDocument();
    await userEvent.type(await screen.findByRole("textbox", { name: "追加するプロジェクト" }), "gh:o/n");
    await userEvent.click(screen.getByRole("button", { name: "追加する" }));
    await screen.findAllByRole("heading", { name: "計画の実行への追加の指示" });
    expect(screen.queryByText("受信箱のサービス（switchboard inbox serve）に届きません。")).toBeNull();
  });

  it("shows someone not an admin the display section and that only an admin may change the rest, and reads nothing", async () => {
    const asked = routes({ "GET /api/auth": () => json(200, { enabled: true, user: "bob", admin: false }) });
    at("/settings");
    expect(await screen.findByText("プロジェクトと追加の指示を変えられるのは管理者だけです。")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "表示" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "言語" })).toBeInTheDocument();
    expect(asked).not.toContain("GET /api/instructions");
    expect(asked).not.toContain("GET /api/projects");
  });
});

describe("the marks of what is being processed (#176)", () => {
  // The mark waits on an issue the list shows as it was when the reply went, so no listing settles it.
  const mark = JSON.stringify([{ ref: "gh:o/r#1", kind: "reply", at: Date.now(), base: JSON.stringify(["waiting", t0]) }]);

  it("go with a logout", async () => {
    sessionStorage.setItem("switchboard.pending", mark);
    routes({
      "GET /api/auth": () => json(200, { enabled: true, user: "alice" }),
      "GET /api/inbox": () => json(200, listing(waiting(1, "A"))),
      "POST /api/logout": () => json(204),
    });
    at("/");
    await screen.findAllByText("処理中");
    expect(sessionStorage.getItem("switchboard.pending")).not.toBeNull();
    await userEvent.click(await screen.findByRole("button", { name: "ログアウト" }));
    await screen.findByRole("link", { name: "GitLabでログイン" });
    expect(sessionStorage.getItem("switchboard.pending")).toBeNull();
  });

  it("go when the session ends", async () => {
    sessionStorage.setItem("switchboard.pending", mark);
    routes({
      "GET /api/auth": () => json(200, { enabled: true, user: "alice" }),
      "GET /api/inbox": () => json(401, { code: "unauthenticated", message: "log in" }),
    });
    at("/");
    await screen.findByRole("link", { name: "GitLabでログイン" });
    expect(sessionStorage.getItem("switchboard.pending")).toBeNull();
  });
});

describe("the results of automatic merges (#177)", () => {
  it("are not told again in a tab once another tab told them", async () => {
    const A = waiting(1, "A");
    const r1 = { ref: "gh:o/r#5", number: 5, outcome: "merged", at: "2026-11-07T10:00:00Z" };
    const r2 = { ref: "gh:o/r#6", number: 6, outcome: "merged", at: "2026-11-07T10:01:00Z" };
    const r3 = { ref: "gh:o/r#7", number: 7, outcome: "merged", at: "2026-11-07T10:02:00Z" };
    routes({ "GET /api/inbox": () => json(200, { ...listing(A), auto_merge_results: [r1] }) });
    at("/");
    expect(await screen.findByText("PR #5を自動でマージしました")).toBeInTheDocument();
    // Another tab told r2 first.
    localStorage.setItem("switchboard.autoMergeTold", JSON.stringify([`${r1.ref}@${r1.at}`, `${r2.ref}@${r2.at}`]));
    FakeEventSource.last!.emit("inbox", { ...listing(A), auto_merge_results: [r1, r2, r3] });
    expect(await screen.findByText("PR #7を自動でマージしました")).toBeInTheDocument();
    expect(screen.queryByText("PR #6を自動でマージしました")).toBeNull();
  });
});

describe("the reason badge (#178)", () => {
  it("names the reason in Japanese after its word", async () => {
    routes({ "GET /api/inbox": () => json(200, listing(waiting(1, "Q"), waiting(2, "C", { reason: "ci" }))) });
    at("/");
    expect((await screen.findAllByText("質問")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("CIの失敗").length).toBeGreaterThan(0);
    expect(screen.getAllByText("question").length).toBeGreaterThan(0);
  });
});

describe("the tables from 640px (#179)", () => {
  it("give each section the same columns", async () => {
    routes({
      "GET /api/inbox": () =>
        json(200, listing(waiting(1, "W"), { ...waiting(2, "R"), state: "running", running_since: t0 })),
    });
    at("/");
    await screen.findAllByText("R");
    const tables = document.querySelectorAll("table");
    expect(tables.length).toBe(2);
    const widths = [...tables].map((t) => [...t.querySelectorAll("col")].map((c) => c.className).join("|"));
    expect(widths[0]).not.toBe("");
    expect(widths[1]).toBe(widths[0]);
    tables.forEach((t) => expect(t).toHaveClass("table-fixed"));
  });
});

describe("the notes of a plan's questions (#180)", () => {
  const plan = waiting(80, "Filters", { reason: "plan-questions" });
  const open = () => {
    routes({
      "GET /api/inbox": () => json(200, listing(plan)),
      "GET /api/issue?ref=gh%3Ao%2Fr%2380": () =>
        json(200, detailOf(plan, { post: smarternews80, approve_word: "/ok", comments: [{ author: "agent", created_at: t0, body: smarternews80, agent: true, kind: "waiting" }] })),
    });
    at("/issue/gh%3Ao%2Fr%2380");
    return screen.findByRole("button", { name: "この内容で返答" });
  };

  it("are folded while an option is chosen, and open on a press", async () => {
    await open();
    expect(screen.queryByRole("textbox", { name: "Q2の補足" })).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Q2に補足を書く" }));
    expect(screen.getByRole("textbox", { name: "Q2の補足" })).toHaveFocus();
  });

  it("open when no option is chosen", async () => {
    await open();
    await userEvent.click(within(screen.getByRole("group", { name: "Q2の選択肢" })).getByRole("radio", { name: "選ばない（補足で答える）" }));
    expect(screen.getByRole("textbox", { name: "Q2の補足" })).toBeInTheDocument();
  });
});
