import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

type Answer = { status: number; body?: unknown };

/** answer makes fetch reply by "METHOD path" and records the requests. */
function answer(routes: Record<string, Answer>) {
  const calls: { key: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      calls.push({ key, init });
      const a = routes[key];
      if (!a && key === "GET /api/inbox") return new Response(JSON.stringify({ items: [], errors: [], fetched_at: "" }), { status: 200 });
      if (!a && key === "GET /api/projects") return new Response(JSON.stringify({ host: "h", enabled: false, items: [] }), { status: 200 });
      if (!a) return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
      return new Response(a.status === 204 ? null : JSON.stringify(a.body), { status: a.status });
    }),
  );
  return calls;
}

afterEach(() => vi.unstubAllGlobals());

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

const ago = (min: number) => new Date(Date.now() - min * 60_000).toISOString();
const item = (over: Record<string, unknown> = {}) => ({
  n: 1, state: "waiting", ref: "gh:o/r#12", project: "o/r", number: 12, title: "Add a flag", url: "https://github.test/o/r/issues/12",
  host: "laptop", waiting_since: ago(5), reason: "question", question: "Which default?", ...over,
});
const listing = (...items: unknown[]) => ({ status: 200, body: { items, errors: [], fetched_at: "" } });

describe("the header's count of waiting issues", () => {
  it("stands out in the attention color and bold while an issue waits", async () => {
    answer({ "GET /api/inbox": listing(item()) });
    at("/");
    const count = await screen.findByText("待ち 1");
    expect(count).toHaveClass("text-attention", "font-semibold");
  });

  it("stays muted while nothing waits", async () => {
    answer({ "GET /api/inbox": listing(item({ state: "running", running_since: ago(2), progress: "?" })) });
    at("/");
    const count = await screen.findByText("待ち 0");
    expect(count).not.toHaveClass("text-attention");
    expect(count).not.toHaveClass("font-semibold");
  });
});

describe("the times and progress on the list", () => {
  it("say what each time is on a card", async () => {
    answer({
      "GET /api/inbox": listing(
        item(),
        item({ n: undefined, ref: "gh:o/r#13", number: 13, state: "running", running_since: ago(2), progress: "14:24 2分 Bash 3・StructuredOutput 1 ｜ テストを直す" }),
        item({ n: undefined, ref: "gh:o/r#14", number: 14, state: "queued", queued_since: ago(3), reason: "host-max-parallel" }),
        item({ n: 3, ref: "gh:o/r#15", number: 15, state: "done", done_since: ago(60) }),
      ),
    });
    at("/");
    expect((await screen.findAllByText("5分前から待ち"))[0].closest("li")).not.toBeNull();
    expect(screen.getAllByText("2分経過")[0]).toBeInTheDocument();
    expect(screen.getAllByText("3分前から順番待ち")[0]).toBeInTheDocument();
    expect(screen.getAllByText("1時間前に完了")[0]).toBeInTheDocument();
    expect(screen.getAllByText("テストを直す")[0]).toBeInTheDocument();
    expect(screen.queryByText(/StructuredOutput/)).toBeNull();
  });

  it("say the time on the issue's state line", async () => {
    answer({
      "GET /api/inbox": listing(item()),
      "GET /api/issue?ref=gh%3Ao%2Fr%2312": {
        status: 200,
        body: { ref: "gh:o/r#12", title: "Add a flag", url: "https://github.test/o/r/issues/12", waiting: true, host: "laptop", state: "waiting", reason: "question", since: ago(5), post: "Which default?", comments: [] },
      },
    });
    at("/issue/gh%3Ao%2Fr%2312");
    const heading = await screen.findByRole("heading", { level: 2, name: "Add a flag" });
    expect(within(heading.closest<HTMLElement>("[data-pane-head]")!).getByText("5分前から待ち")).toBeInTheDocument();
  });

  it("cut a long title on a card to two lines, with the whole of it in its title", async () => {
    const long = "とても長いタイトル".repeat(20);
    answer({ "GET /api/inbox": listing(item({ title: long })) });
    at("/");
    const titles = await screen.findAllByText(long);
    const card = titles.find((el) => el.closest("li"))!;
    expect(card).toHaveClass("line-clamp-2");
    expect(card).toHaveAttribute("title", long);
  });
});

describe("the right pane with nothing chosen", () => {
  it("leads to the issue that has waited longest", async () => {
    answer({
      "GET /api/inbox": listing(
        item({ ref: "gh:o/r#12", title: "Newer", waiting_since: ago(5) }),
        item({ n: 2, ref: "gh:o/r#13", number: 13, title: "Older", waiting_since: ago(30), reason: "plan" }),
      ),
    });
    at("/");
    const main = await screen.findByRole("main");
    const link = await within(main).findByRole("link", { name: /Older/ });
    expect(link).toHaveAttribute("href", "/issue/gh%3Ao%2Fr%2313");
    expect(within(main).getByText("最も古い待ち")).toBeInTheDocument();
    expect(within(link).getByText("o/r#13")).toBeInTheDocument();
    expect(within(link).getByText("plan")).toBeInTheDocument();
    expect(within(link).getByText("30分前から待ち")).toBeInTheDocument();
    expect(within(main).queryByText(/Newer/)).toBeNull();
    expect(within(main).queryByText("左の一覧からissueを選んでください")).toBeNull();
  });

  it("asks to pick an issue while nothing waits", async () => {
    answer({ "GET /api/inbox": listing(item({ state: "running", running_since: ago(2), progress: "?" })) });
    at("/");
    const main = await screen.findByRole("main");
    expect(await within(main).findByText("左の一覧からissueを選んでください")).toBeInTheDocument();
    expect(within(main).queryByText("最も古い待ち")).toBeNull();
  });
});

const healthy = { ref: "gh:o/r", path: "/home/me/repos/r", plan_first: false, cloned: true, missing_labels: [] };
const unready = { ref: "gh:o/notes", path: "/home/me/repos/notes", plan_first: true, cloned: false, missing_labels: ["agent:ready"] };

describe("the settings page", () => {
  it("is headed 設定", async () => {
    answer({ "GET /api/instructions": { status: 200, body: { host: "h", items: [] } } });
    at("/settings");
    expect(await screen.findByRole("heading", { level: 1, name: "設定" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "プロジェクト" })).toBeInTheDocument();
  });

  it("puts a project with no problem on one line, after the ones with a problem", async () => {
    answer({
      "GET /api/projects": { status: 200, body: { host: "h", enabled: false, items: [healthy, unready] } },
      "GET /api/instructions": { status: 200, body: { host: "h", items: [] } },
    });
    at("/settings");
    const section = await screen.findByRole("region", { name: "プロジェクト" });
    const rows = await within(section).findAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("o/notes");
    expect(rows[0]).toHaveTextContent("クローンがありません");
    expect(rows[1]).toHaveTextContent("o/r");
    expect(rows[1]).toHaveTextContent("すぐに実装する");
    expect(rows[1]).not.toHaveTextContent("クローン済み");
    expect(rows[1]).not.toHaveTextContent("/home/me/repos/r");
    expect(rows[1]).toHaveAttribute("title", "/home/me/repos/r");
  });
});

const instructions = { status: 200, body: { host: "h", items: [{ target: "", kind: "plan", content: "old" }] } };

describe("an instruction field", () => {
  const prevented = () => {
    const e = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(e);
    return e.defaultPrevented;
  };

  it("marks a change not saved, and asks before the page unloads", async () => {
    answer({ "GET /api/instructions": instructions, "POST /api/instructions": { status: 204 } });
    const user = userEvent.setup();
    at("/settings");
    const field = await screen.findByRole("textbox", { name: "すべてのプロジェクト" });
    expect(screen.queryByText("未保存の変更があります")).toBeNull();
    expect(prevented()).toBe(false);
    await user.type(field, " more");
    expect(screen.getByText("未保存の変更があります")).toBeInTheDocument();
    expect(prevented()).toBe(true);
    await user.click(screen.getByRole("button", { name: "保存する" }));
    await waitFor(() => expect(screen.queryByText("未保存の変更があります")).toBeNull());
    expect(prevented()).toBe(false);
  });

  it("drops the mark when the text is put back as saved", async () => {
    answer({ "GET /api/instructions": instructions });
    const user = userEvent.setup();
    at("/settings");
    const field = await screen.findByRole("textbox", { name: "すべてのプロジェクト" });
    await user.type(field, "x");
    expect(screen.getByText("未保存の変更があります")).toBeInTheDocument();
    await user.type(field, "{Backspace}");
    expect(screen.queryByText("未保存の変更があります")).toBeNull();
  });

  it("keeps a change not saved while another page is open", async () => {
    answer({ "GET /api/instructions": instructions });
    const user = userEvent.setup();
    at("/settings");
    await user.type(await screen.findByRole("textbox", { name: "すべてのプロジェクト" }), " more");
    await user.click(screen.getByRole("link", { name: /SwitchBoard/ }));
    expect(await screen.findByRole("heading", { level: 2, name: "返答待ち" })).toBeInTheDocument();
    expect(prevented()).toBe(false);
    await user.click(screen.getByRole("link", { name: "設定" }));
    expect(await screen.findByRole("textbox", { name: "すべてのプロジェクト" })).toHaveValue("old more");
    expect(screen.getByText("未保存の変更があります")).toBeInTheDocument();
  });
});

const targets = (items: unknown[]) => ({
  status: 200,
  body: { items, starts: [{ kind: "ready", label: "agent:ready" }, { kind: "go", label: "agent:go" }] },
});

async function openNewIssue() {
  at("/");
  await userEvent.click(await screen.findByRole("button", { name: "新規issue" }));
  const dialog = await screen.findByRole("dialog", { name: "新規issue" });
  await waitFor(() => expect(within(dialog).getByRole("combobox", { name: "プロジェクト" })).toHaveTextContent(/担当/));
  return dialog;
}

describe("the ready start of a new issue", () => {
  it("says whether the chosen project starts with a plan", async () => {
    answer({ "GET /api/issue-targets": targets([{ ref: "gh:o/r", host: "a", plan_first: true }, { ref: "gh:o/x", host: "a", plan_first: false }]) });
    const dialog = await openNewIssue();
    expect(within(dialog).getByRole("radio", { name: /agent:ready/ }).closest("label")).toHaveTextContent("このプロジェクトは計画から始めます");
    await userEvent.click(within(dialog).getByRole("combobox", { name: "プロジェクト" }));
    await userEvent.click(await within(dialog).findByRole("option", { name: "o/x GitHub（担当：a）" }));
    expect(within(dialog).getByRole("radio", { name: /agent:ready/ }).closest("label")).toHaveTextContent("このプロジェクトはすぐに実装します");
  });

  it("names each of the three starts of plan_first", async () => {
    answer({
      "GET /api/issue-targets": targets([
        { ref: "gh:o/r", host: "a", plan_first: true, plan_first_mode: "auto" },
        { ref: "gh:o/x", host: "a", plan_first: true, plan_first_mode: "always" },
        { ref: "gh:o/y", host: "a", plan_first: false, plan_first_mode: "never" },
      ]),
    });
    const dialog = await openNewIssue();
    const ready = () => within(dialog).getByRole("radio", { name: /agent:ready/ }).closest("label");
    expect(ready()).toHaveTextContent("このプロジェクトは必要なら計画から始めます");
    await userEvent.click(within(dialog).getByRole("combobox", { name: "プロジェクト" }));
    await userEvent.click(await within(dialog).findByRole("option", { name: "o/x GitHub（担当：a）" }));
    expect(ready()).toHaveTextContent("このプロジェクトは必ず計画から始めます");
    await userEvent.click(within(dialog).getByRole("combobox", { name: "プロジェクト" }));
    await userEvent.click(await within(dialog).findByRole("option", { name: "o/y GitHub（担当：a）" }));
    expect(ready()).toHaveTextContent("このプロジェクトはすぐに実装します");
  });

  it("adds nothing when the inbox service does not say", async () => {
    answer({ "GET /api/issue-targets": targets([{ ref: "gh:o/r", host: "a" }]) });
    const dialog = await openNewIssue();
    expect(within(dialog).queryByText(/このプロジェクトは/)).toBeNull();
  });
});
