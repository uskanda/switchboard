import { render, screen } from "@testing-library/react";
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

const base = { project: "o/r", url: "https://github.test/o/r/issues/1", waiting_since: null, question: "?" };
const waiting = { ...base, n: 1, state: "waiting", ref: "gh:o/r#12", number: 12, title: "Waits", host: "laptop", reason: "question", question: "Which?", waiting_since: new Date(Date.now() - 3 * 3600_000).toISOString() };
const running = { ...base, state: "running", ref: "gh:o/r#30", number: 30, title: "Runs", host: "desk", reason: "", running_since: new Date(Date.now() - 7 * 60_000).toISOString(), progress: "working" };
const queued = { ...base, state: "queued", ref: "gh:o/r#40", number: 40, title: "Queued work", host: "desk", reason: "max-parallel", queued_since: new Date(Date.now() - 5 * 60_000).toISOString() };
const queuedOnProject = { ...queued, ref: "gh:o/r#41", number: 41, title: "Also queued", reason: "project-max-parallel" };

describe("queued issues", () => {
  it("are listed under their own heading after the running ones, with the limit reached", async () => {
    answer({ "GET /api/inbox": { items: [waiting, running, queued, queuedOnProject], errors: [], fetched_at: "" } });
    at("/");
    const head = await screen.findByRole("heading", { level: 2, name: "順番待ち" });
    const run = screen.getByRole("heading", { level: 2, name: "実行中" });
    expect(run.compareDocumentPosition(head) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const row = screen.getAllByText("Queued work")[0];
    expect(head.compareDocumentPosition(row) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(row.closest("a")).toHaveAttribute("href", "/issue/gh%3Ao%2Fr%2340");
    expect(screen.getAllByText("端末の上限に達しています")[0]).toBeInTheDocument();
    expect(screen.getAllByText("プロジェクトの上限に達しています")[0]).toBeInTheDocument();
    expect(screen.getAllByText("5分前")[0]).toBeInTheDocument();
    expect(screen.getByText("待ち1件、実行中1件、順番待ち2件")).toBeInTheDocument();
  });

  it("show that one waits for a slot, with its queued comment folded", async () => {
    const d = {
      ref: "gh:o/r#40", title: "Queued work", url: "https://github.test/o/r/issues/40", waiting: false, host: "desk",
      state: "queued", reason: "max-parallel", since: new Date(Date.now() - 5 * 60_000).toISOString(),
      post: "Queued: host desk runs as many issues at once as it may (1).", author: "me", created_at: null, body: "Do it.",
      comments: [{ author: "bot", created_at: new Date().toISOString(), body: "Queued: host desk runs as many issues at once as it may (1).", agent: true, kind: "queued" }],
    };
    answer({ "GET /api/issue?ref=gh%3Ao%2Fr%2340": d });
    at("/issue/gh%3Ao%2Fr%2340");
    expect(await screen.findByText("順番待ちになりました")).toBeInTheDocument();
    expect(screen.getAllByText("順番待ち")[0]).toBeInTheDocument();
    expect(screen.getByText("5分前")).toBeInTheDocument();
    expect(screen.queryByText(/runs as many issues/)).toBeNull();
    expect(screen.getByText("順番待ちのため、まだ返答は要りません")).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "返答" })).toBeNull();
  });

  it("keep the reply form of an issue that still waits for a reply", async () => {
    const d = {
      ref: "gh:o/r#12", title: "Waits", url: "https://github.test/o/r/issues/12", waiting: true, host: "desk",
      state: "queued", reason: "max-parallel", since: new Date(Date.now() - 60_000).toISOString(),
      post: "### The agent has a question\n\n**Question**\n\nWhich?",
    };
    answer({ "GET /api/issue?ref=gh%3Ao%2Fr%2312": d });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByRole("textbox", { name: "返答" })).toBeInTheDocument();
    expect(screen.getAllByText("順番待ち")[0]).toBeInTheDocument();
  });
});
