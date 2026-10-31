import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { InboxPage } from "./InboxPage";
import { IssuePage } from "./IssuePage";

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
      <Routes>
        <Route path="/" element={<InboxPage />} />
        <Route path="/issue/:ref" element={<IssuePage />} />
      </Routes>
    </MemoryRouter>,
  );
}

const item = {
  n: 1, ref: "gh:o/r#12", project: "o/r", number: 12, title: "<script>alert(1)</script>",
  url: "https://github.test/o/r/issues/12", host: "laptop", waiting_since: new Date(Date.now() - 3 * 3600_000).toISOString(),
  reason: "question", question: "Which default should --dry-run have?",
};

describe("the inbox", () => {
  it("lists the waiting issues, with issue text as text", async () => {
    answer({ "GET /api/inbox": { status: 200, body: { items: [item, { ...item, n: 2, ref: "gl:nas/sb#6", reason: "error" }], errors: [], fetched_at: "" } } });
    at("/");
    expect(await screen.findAllByText("<script>alert(1)</script>")).not.toHaveLength(0);
    expect(screen.getByRole("heading", { level: 1, name: "受信箱" })).toBeInTheDocument();
    expect(document.querySelector("script")).toBeNull();
    expect(screen.getAllByText("gh:o/r#12")[0].closest("a")).toHaveAttribute("href", "/issue/gh%3Ao%2Fr%2312");
    expect(screen.getAllByText("question")[0]).toBeInTheDocument();
    expect(screen.getAllByText("error")[0]).toBeInTheDocument();
    expect(screen.getAllByText("3時間前")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Which default should --dry-run have?")[0]).toBeInTheDocument();
  });

  it("warns of a forge that could not be read", async () => {
    answer({ "GET /api/inbox": { status: 200, body: { items: [item], errors: [{ forge: "gitlab", message: "connection refused" }], fetched_at: "" } } });
    at("/");
    expect(await screen.findByRole("alert")).toHaveTextContent("gitlab");
    expect(screen.getAllByText("gh:o/r#12")[0]).toBeInTheDocument();
  });

  it("says so when the inbox service is not reachable", async () => {
    answer({ "GET /api/inbox": { status: 503, body: { code: "unavailable", message: "not reachable" } } });
    at("/");
    expect(await screen.findByRole("alert")).toHaveTextContent("受信箱のサービス");
  });

  it("says when nothing waits", async () => {
    answer({ "GET /api/inbox": { status: 200, body: { items: [], errors: [], fetched_at: "" } } });
    at("/");
    expect(await screen.findByText("待ちのissueはありません")).toBeInTheDocument();
  });
});

const detail = {
  ref: "gh:o/r#12", title: "<b>Add a flag</b>", url: "https://github.test/o/r/issues/12", waiting: true,
  host: "laptop", state: "waiting", reason: "permission", since: null,
  post: "### The agent has a question\n\n**Denied**\n\n- Bash: `rm -rf /`\n\n**Question**\n\nWhich default?",
};

describe("one issue", () => {
  it("shows the whole post and the forge link", async () => {
    answer({ "GET /api/issue?ref=gh%3Ao%2Fr%2312": { status: 200, body: detail } });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByRole("heading", { name: "<b>Add a flag</b>" })).toBeInTheDocument();
    expect(screen.getByText(/Bash: `rm -rf \/`/)).toBeInTheDocument();
    expect(screen.getByText(/Which default\?/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /フォージで開く/ })).toHaveAttribute("href", detail.url);
  });

  it("posts the reply and says it was sent", async () => {
    const calls = answer({
      "GET /api/issue?ref=gh%3Ao%2Fr%2312": { status: 200, body: detail },
      "POST /api/reply": { status: 204 },
    });
    at("/issue/gh%3Ao%2Fr%2312");
    const send = await screen.findByRole("button", { name: "返答を送る" });
    expect(send).toBeDisabled();
    await userEvent.type(screen.getByRole("textbox", { name: "返答" }), "Go with B.");
    await userEvent.click(send);
    expect(await screen.findByRole("status")).toHaveTextContent("送りました");
    const post = calls.find((c) => c.key === "POST /api/reply");
    expect(JSON.parse(String(post?.init?.body))).toEqual({ ref: "gh:o/r#12", text: "Go with B." });
    expect(new Headers(post?.init?.headers).get("Content-Type")).toBe("application/json");
  });

  it("says when the issue stopped waiting before the reply", async () => {
    answer({
      "GET /api/issue?ref=gh%3Ao%2Fr%2312": { status: 200, body: detail },
      "POST /api/reply": { status: 409, body: { code: "not_waiting", message: "gh:o/r#12 is not waiting" } },
    });
    at("/issue/gh%3Ao%2Fr%2312");
    await userEvent.type(await screen.findByRole("textbox", { name: "返答" }), "Go with B.");
    await userEvent.click(screen.getByRole("button", { name: "返答を送る" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("待ちではなくなりました"));
  });

  it("offers no reply on an issue that is not waiting", async () => {
    answer({ "GET /api/issue?ref=gh%3Ao%2Fr%2312": { status: 200, body: { ...detail, waiting: false } } });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByText(/待ちではありません/)).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "返答" })).toBeNull();
  });
});

const runningItem = {
  state: "running", ref: "gh:o/r#30", project: "o/r", number: 30, title: "Speed up", url: "https://github.test/o/r/issues/30",
  host: "desk", waiting_since: null, reason: "", question: "", running_since: new Date(Date.now() - 7 * 60_000).toISOString(),
  progress: "13:41 7分 Bash 12・Edit 5 ｜ テストを直している",
};

describe("running issues", () => {
  it("are listed apart from the waiting ones, with the latest progress line", async () => {
    answer({ "GET /api/inbox": { status: 200, body: { items: [{ ...item, state: "waiting" }, runningItem], errors: [], fetched_at: "" } } });
    at("/");
    const heading = await screen.findByRole("heading", { level: 2, name: "実行中" });
    expect(heading).toBeInTheDocument();
    const progress = screen.getAllByText("13:41 7分 Bash 12・Edit 5 ｜ テストを直している")[0];
    expect(heading.compareDocumentPosition(progress) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(progress.closest("a")).toHaveAttribute("href", "/issue/gh%3Ao%2Fr%2330");
    expect(screen.getAllByText("7分")[0]).toBeInTheDocument();
    expect(screen.getAllByText("desk")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Which default should --dry-run have?")[0]).toBeInTheDocument();
  });

  it("leave the waiting list empty when only they exist", async () => {
    answer({ "GET /api/inbox": { status: 200, body: { items: [runningItem], errors: [], fetched_at: "" } } });
    at("/");
    expect(await screen.findByText("待ちのissueはありません")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "実行中" })).toBeInTheDocument();
  });

  it("show the whole progress comment and no reply form", async () => {
    const running = {
      ...detail, waiting: false, state: "running", reason: "?", host: "desk", since: new Date(Date.now() - 7 * 60_000).toISOString(),
      post: "### 実行の進捗\n\n- 13:35 1分 Read 2 ｜ 読む\n- 13:41 7分 Bash 12 ｜ テストを直している",
    };
    answer({ "GET /api/issue?ref=gh%3Ao%2Fr%2312": { status: 200, body: running } });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByText(/13:35 1分 Read 2 ｜ 読む/)).toBeInTheDocument();
    expect(screen.getByText(/13:41 7分 Bash 12/)).toBeInTheDocument();
    expect(screen.getByText("実行中のため返答できません")).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "返答" })).toBeNull();
    expect(screen.queryByText("?")).toBeNull(); // no reason badge
    expect(screen.getByText("7分")).toBeInTheDocument();
  });
});
