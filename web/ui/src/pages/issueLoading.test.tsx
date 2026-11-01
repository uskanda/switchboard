import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

/** Reads of /api/issue wait until the test answers them, one by one, oldest first. */
function serveSlowly(inbox: unknown) {
  const pending: { ref: string; answer: (body: unknown) => void }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string) => {
      if (input === "/api/inbox") return new Response(JSON.stringify(inbox), { status: 200 });
      const m = /^\/api\/issue\?ref=(.*)$/.exec(input);
      if (m) {
        return new Promise<Response>((resolve) =>
          pending.push({ ref: decodeURIComponent(m[1]), answer: (body) => resolve(new Response(JSON.stringify(body), { status: 200 })) }),
        );
      }
      return new Response(JSON.stringify({ code: "not_found", message: input }), { status: 404 });
    }),
  );
  return {
    pending,
    async answer(body: unknown) {
      const next = pending.shift();
      if (!next) throw new Error("no read is waiting");
      await act(async () => next.answer(body));
    },
  };
}

class QuietEventSource {
  onerror: (() => void) | null = null;
  addEventListener() {}
  close() {}
}

beforeEach(() => vi.stubGlobal("EventSource", QuietEventSource));
afterEach(() => vi.unstubAllGlobals());

const t0 = "2026-10-31T10:00:00Z";
const item = (over: Record<string, unknown> = {}) => ({
  n: 1, state: "waiting", ref: "gh:o/r#12", project: "o/r", number: 12, title: "Add a flag", url: "https://github.test/o/r/issues/12",
  host: "laptop", waiting_since: t0, reason: "question", question: "Which default?", updated_at: t0, notes: 0, ...over,
});
const listing = { items: [item(), item({ n: 2, ref: "gh:o/r#13", number: 13, title: "Fix CI" })], errors: [], fetched_at: t0 };
const detail = (over: Record<string, unknown> = {}) => ({
  ref: "gh:o/r#12", title: "Add a flag", url: "https://github.test/o/r/issues/12", waiting: true, host: "laptop",
  state: "waiting", reason: "question", since: t0, post: "Which default?", author: "me", created_at: t0,
  body: "Please add --dry-run.", comments: [], ...over,
});

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

describe("reading an issue", () => {
  it("says it is loading until the first read arrives", async () => {
    const forge = serveSlowly(listing);
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByRole("status", { name: "読み込んでいます" })).toBeInTheDocument();
    await forge.answer(detail());
    expect(await screen.findByRole("heading", { level: 2, name: "Add a flag" })).toBeInTheDocument();
    expect(screen.queryByRole("status", { name: "読み込んでいます" })).not.toBeInTheDocument();
  });

  it("shows what was read last time while it reads the issue again, and says so", async () => {
    const forge = serveSlowly(listing);
    at("/issue/gh%3Ao%2Fr%2312");
    await forge.answer(detail());
    await screen.findByRole("heading", { level: 2, name: "Add a flag" });
    cleanup();

    at("/issue/gh%3Ao%2Fr%2312");
    // Before the forge answers, the last read is on the page, marked as being brought up to date.
    expect(screen.getByRole("heading", { level: 2, name: "Add a flag" })).toBeInTheDocument();
    expect(await screen.findByRole("status", { name: "更新を取得中" })).toBeInTheDocument();
    expect(screen.queryByRole("status", { name: "読み込んでいます" })).not.toBeInTheDocument();

    await forge.answer(detail({ title: "Add a --dry-run flag" }));
    expect(await screen.findByRole("heading", { level: 2, name: "Add a --dry-run flag" })).toBeInTheDocument();
    expect(screen.queryByRole("status", { name: "更新を取得中" })).not.toBeInTheDocument();
  });

  it("switches to the cached issue at once when another row is picked", async () => {
    const forge = serveSlowly(listing);
    at("/issue/gh%3Ao%2Fr%2312");
    await forge.answer(detail());
    await screen.findByRole("heading", { level: 2, name: "Add a flag" });

    await userEvent.click((await screen.findAllByRole("link", { name: /Fix CI/ }))[0]);
    expect(await screen.findByRole("status", { name: "読み込んでいます" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 2, name: "Add a flag" })).not.toBeInTheDocument();
    await forge.answer(detail({ ref: "gh:o/r#13", title: "Fix CI" }));
    expect(await screen.findByRole("heading", { level: 2, name: "Fix CI" })).toBeInTheDocument();

    await userEvent.click(screen.getAllByRole("link", { name: /Add a flag/ })[0]);
    expect(await screen.findByRole("heading", { level: 2, name: "Add a flag" })).toBeInTheDocument();
    expect(screen.getByRole("status", { name: "更新を取得中" })).toBeInTheDocument();
  });

  it("keeps the last read shown when reading it again fails", async () => {
    const forge = serveSlowly(listing);
    at("/issue/gh%3Ao%2Fr%2312");
    await forge.answer(detail());
    await screen.findByRole("heading", { level: 2, name: "Add a flag" });
    cleanup();

    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: string) =>
        input === "/api/inbox"
          ? new Response(JSON.stringify(listing), { status: 200 })
          : new Response(JSON.stringify({ code: "unavailable", message: "down" }), { status: 503 }),
      ),
    );
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByRole("alert")).toHaveTextContent("前回読んだ内容を表示しています");
    expect(screen.getByRole("heading", { level: 2, name: "Add a flag" })).toBeInTheDocument();
    expect(screen.queryByRole("status", { name: "更新を取得中" })).not.toBeInTheDocument();
  });
});
