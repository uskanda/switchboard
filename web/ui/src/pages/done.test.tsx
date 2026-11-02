import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

function answer(routes: Record<string, unknown>) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      if (key in routes) return new Response(JSON.stringify(routes[key]), { status: 200 });
      if (key === "GET /api/inbox") return new Response(JSON.stringify({ items: [], errors: [], fetched_at: "" }), { status: 200 });
      return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
    }),
  );
}

afterEach(() => vi.unstubAllGlobals());

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

const base = { project: "o/r", url: "https://github.test/o/r/issues/1", waiting_since: null, question: "?", reason: "" };
const waiting = { ...base, n: 1, state: "waiting", ref: "gh:o/r#12", number: 12, title: "Waits", host: "laptop", reason: "question", question: "Which?", waiting_since: new Date(Date.now() - 3 * 3600_000).toISOString() };
const running = { ...base, state: "running", ref: "gh:o/r#30", number: 30, title: "Runs", host: "desk", running_since: new Date(Date.now() - 7 * 60_000).toISOString(), progress: "working" };
const queued = { ...base, state: "queued", ref: "gh:o/r#40", number: 40, title: "Queued work", host: "desk", reason: "max-parallel", queued_since: new Date(Date.now() - 5 * 60_000).toISOString() };
const done = { ...base, n: 2, state: "done", ref: "gh:o/r#50", number: 50, title: "Finished", host: "desk", done_since: new Date(Date.now() - 2 * 3600_000).toISOString(), change_url: "https://github.test/o/r/pull/9" };

describe("done issues", () => {
  it("are listed under their own heading after the queued ones, with the pull request", async () => {
    answer({ "GET /api/inbox": { items: [waiting, done, running, queued], errors: [], fetched_at: "" } });
    at("/");
    const head = await screen.findByRole("heading", { level: 2, name: "完了（close待ち）" });
    const q = screen.getByRole("heading", { level: 2, name: "順番待ち" });
    expect(q.compareDocumentPosition(head) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const row = screen.getAllByText("Finished")[0];
    expect(head.compareDocumentPosition(row) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(row.closest("a")).toHaveAttribute("href", "/issue/gh%3Ao%2Fr%2350");
    expect(screen.getAllByRole("link", { name: "https://github.test/o/r/pull/9" })[0]).toHaveAttribute("href", "https://github.test/o/r/pull/9");
    expect(screen.getAllByText("2時間前")[0]).toBeInTheDocument();
    expect(screen.getByLabelText("待ち1件、実行中1件、順番待ち1件、完了1件")).toBeInTheDocument();
  });

  it("leave the header count out while there is none", async () => {
    answer({ "GET /api/inbox": { items: [waiting, running], errors: [], fetched_at: "" } });
    at("/");
    expect(await screen.findByLabelText("待ち1件、実行中1件")).toBeInTheDocument();
  });

  it("take a reply until the issue is closed", async () => {
    const d = {
      ref: "gh:o/r#50", title: "Finished", url: "https://github.test/o/r/issues/50", waiting: false, done: true, host: "desk",
      state: "done", reason: "?", since: new Date(Date.now() - 2 * 3600_000).toISOString(),
      post: "### The work is done\n\nPull request: https://github.test/o/r/pull/9\n\nAdded it.",
    };
    answer({ "GET /api/issue?ref=gh%3Ao%2Fr%2350": d });
    at("/issue/gh%3Ao%2Fr%2350");
    expect(await screen.findByRole("textbox", { name: "返答" })).toBeInTheDocument();
    expect(screen.getAllByText("完了")[0]).toBeInTheDocument();
    expect(screen.queryByText("このissueは待ちではなくなりました。返答は送れません。")).toBeNull();
  });

  it("say a reply was sent while the issue stays done", async () => {
    const d = {
      ref: "gh:o/r#50", title: "Finished", url: "https://github.test/o/r/issues/50", waiting: false, done: true, host: "desk",
      state: "done", reason: "?", since: new Date(Date.now() - 2 * 3600_000).toISOString(), post: "### The work is done",
    };
    answer({ "GET /api/inbox": { items: [done], errors: [], fetched_at: "" }, "GET /api/issue?ref=gh%3Ao%2Fr%2350": d, "POST /api/reply": {} });
    at("/issue/gh%3Ao%2Fr%2350");
    await userEvent.type(await screen.findByRole("textbox", { name: "返答" }), "もう一つ直して");
    await userEvent.click(screen.getByRole("button", { name: "返答を送る" }));
    expect(await screen.findByText("送りました。エージェントが作業を再開します。")).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "返答" })).toBeNull();
  });
});
