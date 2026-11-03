import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { PlanQuestions } from "@/components/PlanQuestions";
import { defaultAnswers, parsePlanQuestions } from "@/lib/planQuestions";
import { smarternews80 } from "@/lib/planQuestions.fixture";

type Answer = { status: number; body?: unknown } | "never";

/** answer makes fetch reply by "METHOD path" ("never" leaves the request pending) and records the requests. */
function answer(routes: Record<string, Answer>) {
  const calls: { key: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      calls.push({ key, init });
      const a = routes[key];
      if (a === "never") return new Promise<Response>(() => {});
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

const t0 = new Date(Date.now() - 5 * 60_000).toISOString();
const item = (over: Record<string, unknown> = {}) => ({
  n: 1, state: "waiting", ref: "gh:o/r#12", project: "o/r", number: 12, title: "Add a flag", url: "https://github.test/o/r/issues/12",
  host: "laptop", waiting_since: t0, reason: "question", question: "Which default?", ...over,
});
const listing = (...items: unknown[]) => ({ status: 200, body: { items, errors: [], fetched_at: t0 } });
const detail = (over: Record<string, unknown> = {}) => ({
  status: 200,
  body: {
    ref: "gh:o/r#12", title: "Add a flag", url: "https://github.test/o/r/issues/12", waiting: true, host: "laptop",
    state: "waiting", reason: "question", since: t0, post: "Which default?", author: "me", created_at: t0,
    body: "Please add --dry-run.", comments: [], ...over,
  },
});

describe("the reply", () => {
  it("keeps what was typed for an issue while another one is open", async () => {
    answer({
      "GET /api/inbox": listing(item(), item({ n: 2, ref: "gh:o/r#13", number: 13, title: "Fix CI" })),
      "GET /api/issue?ref=gh%3Ao%2Fr%2312": detail(),
      "GET /api/issue?ref=gh%3Ao%2Fr%2313": detail({ ref: "gh:o/r#13", title: "Fix CI" }),
    });
    const user = userEvent.setup();
    at("/issue/gh%3Ao%2Fr%2312");
    await user.type(await screen.findByLabelText("返答"), "Use B.");
    await user.click(screen.getAllByRole("link", { name: /Fix CI/ })[0]);
    expect(await screen.findByRole("heading", { level: 2, name: "Fix CI" })).toBeInTheDocument();
    expect(screen.getByLabelText("返答")).toHaveValue("");
    await user.click(screen.getAllByRole("link", { name: /Add a flag/ })[0]);
    expect(await screen.findByRole("heading", { level: 2, name: "Add a flag" })).toBeInTheDocument();
    expect(screen.getByLabelText("返答")).toHaveValue("Use B.");
  });

  it("is sent with Ctrl+Enter", async () => {
    const calls = answer({
      "GET /api/inbox": listing(item()),
      "GET /api/issue?ref=gh%3Ao%2Fr%2312": detail(),
      "POST /api/reply": { status: 204 },
    });
    const user = userEvent.setup();
    at("/issue/gh%3Ao%2Fr%2312");
    await user.type(await screen.findByLabelText("返答"), "Use B.{Control>}{Enter}{/Control}");
    await waitFor(() => expect(calls.some((c) => c.key === "POST /api/reply")).toBe(true));
  });

  // With another wait on the list, the focus goes to the button to it instead (waitNav.test.tsx).
  it("moves the focus to what says it was sent when no other wait is left", async () => {
    answer({
      "GET /api/inbox": listing(item()),
      "GET /api/issue?ref=gh%3Ao%2Fr%2312": detail(),
      "POST /api/reply": { status: 204 },
    });
    const user = userEvent.setup();
    at("/issue/gh%3Ao%2Fr%2312");
    await user.type(await screen.findByLabelText("返答"), "Use B.");
    await user.click(screen.getByRole("button", { name: "返答を送る" }));
    const sent = await screen.findByText("送りました。エージェントが作業を再開します。");
    await waitFor(() => expect(sent).toHaveFocus());
  });

  it("links the hint under its label to the field", async () => {
    answer({ "GET /api/inbox": listing(item()), "GET /api/issue?ref=gh%3Ao%2Fr%2312": detail({ reason: "ci" }) });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByLabelText("返答")).toHaveAccessibleDescription(/CIの失敗を自動で直す回数/);
  });
});

describe("the list", () => {
  it("heads the waiting issues", async () => {
    answer({ "GET /api/inbox": listing(item()) });
    at("/");
    expect(await screen.findByRole("heading", { level: 2, name: "返答待ち" })).toBeInTheDocument();
  });

  it("says a run has no progress yet instead of a question mark", async () => {
    answer({
      "GET /api/inbox": listing(
        item({ state: "running", running_since: t0, progress: "?" }),
        item({ n: 2, ref: "gh:o/r#13", number: 13, state: "running", running_since: t0, progress: "14:24 2分 Bash 1 ｜ ?" }),
      ),
    });
    at("/");
    expect((await screen.findAllByText("進捗はまだありません"))[0]).toBeInTheDocument();
    expect(screen.getAllByText("14:24 2分 Bash 1")[0]).toBeInTheDocument();
    expect(screen.queryByText("?")).toBeNull();
  });

  it("shows the start of a question without its Markdown marks", async () => {
    answer({ "GET /api/inbox": listing(item({ question: "1. **設定（`a.go`）** - `Agent` に…" })) });
    at("/");
    expect((await screen.findAllByText("1. 設定（a.go） - Agent に…"))[0]).toBeInTheDocument();
  });

  it("says it is loading until the first listing arrives", async () => {
    answer({ "GET /api/inbox": "never" });
    at("/");
    expect(await screen.findByRole("status", { name: "読み込んでいます" })).toBeInTheDocument();
  });

  it("names a pull request by its number", async () => {
    const done = item({ state: "done", done_since: t0, change_url: "https://github.test/o/r/pull/9", change_number: 9, change_sha: "abc" });
    answer({ "GET /api/inbox": listing(done) });
    at("/");
    expect((await screen.findAllByRole("link", { name: "PR #9" }))[0]).toHaveAttribute("href", "https://github.test/o/r/pull/9");
  });

  it("draws a card's focus apart from the card, so it differs from the frame of the chosen one", async () => {
    answer({ "GET /api/inbox": listing(item()) });
    at("/");
    const card = within(await screen.findByRole("list")).getAllByRole("link")[0];
    expect(card).toHaveClass("focus-visible:outline-offset-2");
  });
});

describe("the page title", () => {
  it("counts the waiting issues and names the screen", async () => {
    answer({ "GET /api/inbox": listing(item()), "GET /api/issue?ref=gh%3Ao%2Fr%2312": detail() });
    at("/");
    await waitFor(() => expect(document.title).toBe("(1) SwitchBoard Inbox"));
  });

  it("names the issue that is open", async () => {
    answer({ "GET /api/inbox": listing(item()), "GET /api/issue?ref=gh%3Ao%2Fr%2312": detail() });
    at("/issue/gh%3Ao%2Fr%2312");
    await waitFor(() => expect(document.title).toBe("(1) Add a flag - SwitchBoard Inbox"));
  });

  it("names the settings", async () => {
    answer({ "GET /api/inbox": listing(), "GET /api/projects": "never", "GET /api/instructions": "never" });
    at("/settings");
    await waitFor(() => expect(document.title).toBe("設定 - SwitchBoard Inbox"));
  });
});

describe("the favicon", () => {
  const icon = () => document.querySelector<HTMLLinkElement>('link[rel="icon"]')?.getAttribute("href");

  it("is marked while an issue waits", async () => {
    answer({ "GET /api/inbox": listing(item()) });
    at("/");
    await waitFor(() => expect(icon()).toBe("/favicon-waiting.svg"));
  });

  it("is the plain one while nothing waits", async () => {
    answer({ "GET /api/inbox": listing(item({ n: undefined, state: "running", progress: "p" })) });
    at("/");
    await screen.findByText("待ち 0・実行中 1");
    expect(icon()).toBe("/favicon.svg");
  });
});

describe("loading", () => {
  it("says the settings are loading", async () => {
    answer({ "GET /api/inbox": listing(), "GET /api/projects": "never", "GET /api/instructions": "never" });
    at("/settings");
    expect(await screen.findAllByRole("status", { name: "読み込んでいます" })).toHaveLength(2);
  });

  it("says the projects of a new issue are loading", async () => {
    answer({ "GET /api/inbox": listing(), "GET /api/issue-targets": "never" });
    const user = userEvent.setup();
    at("/");
    await user.click(await screen.findByRole("button", { name: "新規issue" }));
    expect(within(screen.getByRole("dialog")).getByRole("status", { name: "読み込んでいます" })).toBeInTheDocument();
  });
});

describe("the thread", () => {
  it("does not name the account the agent posts with", async () => {
    const asked = { author: "bot", created_at: t0, body: "Which default?", agent: true, kind: "waiting" };
    answer({ "GET /api/inbox": listing(item()), "GET /api/issue?ref=gh%3Ao%2Fr%2312": detail({ comments: [asked] }) });
    at("/issue/gh%3Ao%2Fr%2312");
    expect((await screen.findAllByText("エージェント"))[0]).toBeInTheDocument();
    expect(screen.queryByText("bot")).toBeNull();
  });
});

describe("the questions of a plan", () => {
  it("marks the recommended option", () => {
    const parsed = parsePlanQuestions(smarternews80)!;
    const answers = defaultAnswers(parsed.questions);
    render(<PlanQuestions parsed={parsed} answers={answers} disabled={false} onChange={() => {}} />);
    const chosen = screen.getAllByRole("radio", { checked: true }).filter((r) => (r as HTMLInputElement).value !== "");
    expect(chosen.length).toBeGreaterThan(0);
    for (const r of chosen) expect(r.closest("label")).toHaveTextContent("（推奨）");
  });
});
