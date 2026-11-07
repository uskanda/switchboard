import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

type Answer = { status: number; body?: unknown };

/** answer makes fetch reply by "METHOD path"; routes can be changed between requests. */
function answer(routes: Record<string, Answer>) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      const a = routes[key];
      if (!a) return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
      return new Response(a.status === 204 ? null : JSON.stringify(a.body), { status: a.status });
    }),
  );
  return routes;
}

afterEach(() => vi.unstubAllGlobals());

/** Where shows the path the router is at, so a test can tell which issue is open. */
function Where() {
  return <span data-testid="where">{decodeURIComponent(useLocation().pathname)}</span>;
}

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
      <Where />
    </MemoryRouter>,
  );
}

const where = () => screen.getByTestId("where").textContent;
const path = (n: number) => `/issue/gh:o/r#${n}`;
const url = (n: number) => `/issue/gh%3Ao%2Fr%23${n}`;

const t0 = new Date(Date.now() - 5 * 60_000).toISOString();
const item = (n: number, over: Record<string, unknown> = {}) => ({
  n, state: "waiting", ref: `gh:o/r#${n}`, project: "o/r", number: n, title: `Issue ${n}`, url: `https://github.test/o/r/issues/${n}`,
  host: "laptop", waiting_since: t0, reason: "question", question: "Which default?", ...over,
});
const listing = (...items: unknown[]) => ({ status: 200, body: { items, errors: [], fetched_at: t0 } });
const detail = (n: number, over: Record<string, unknown> = {}) => ({
  status: 200,
  body: {
    ref: `gh:o/r#${n}`, title: `Issue ${n}`, url: `https://github.test/o/r/issues/${n}`, waiting: true, host: "laptop",
    state: "waiting", reason: "question", since: t0, post: "Which default?", author: "me", created_at: t0,
    body: "Please add --dry-run.", comments: [], ...over,
  },
});

/** routes answers the list of items and the read of each of them, and takes replies. */
function routes(...items: ReturnType<typeof item>[]) {
  const r: Record<string, Answer> = { "GET /api/inbox": listing(...items), "POST /api/reply": { status: 204 } };
  for (const it of items) {
    r[`GET /api/issue?ref=${encodeURIComponent(it.ref)}`] = detail(it.number, it.state === "waiting" ? {} : { waiting: false, state: it.state });
  }
  return r;
}

async function reply(user: ReturnType<typeof userEvent.setup>, n: number) {
  expect(await screen.findByRole("heading", { level: 2, name: `Issue ${n}` })).toBeInTheDocument();
  await user.type(await screen.findByLabelText("返答"), "Use B.");
  await user.click(screen.getByRole("button", { name: "返答を送る" }));
  await screen.findByText("送りました。エージェントが作業を再開します。");
}

describe("the next wait", () => {
  it("takes the focus once a reply is sent and opens the next wait with Enter", async () => {
    answer(routes(item(12), item(13)));
    const user = userEvent.setup();
    at(url(12));
    await reply(user, 12);
    const next = await screen.findByRole("button", { name: /次の待ちへ/ });
    expect(next).toHaveAccessibleName(/Issue 13/);
    await waitFor(() => expect(next).toHaveFocus());
    await user.keyboard("{Enter}");
    await waitFor(() => expect(where()).toBe(path(13)));
    expect(await screen.findByRole("heading", { level: 2, name: "Issue 13" })).toBeInTheDocument();
  });

  it("says there is no other wait and leaves the focus on what says it was sent", async () => {
    answer(routes(item(12)));
    const user = userEvent.setup();
    at(url(12));
    await reply(user, 12);
    expect(await screen.findByText("他に待ちはありません")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /次の待ちへ/ })).not.toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("送りました。エージェントが作業を再開します。")).toHaveFocus());
  });

  it("goes round from the last wait to the first", async () => {
    answer(routes(item(12), item(13), item(14)));
    const user = userEvent.setup();
    at(url(14));
    await reply(user, 14);
    expect(await screen.findByRole("button", { name: /次の待ちへ/ })).toHaveAccessibleName(/Issue 12/);
  });

  it("skips a wait just answered that is still on the list", async () => {
    answer(routes(item(12), item(13), item(14)));
    const user = userEvent.setup();
    at(url(12));
    await reply(user, 12);
    await user.click(await screen.findByRole("button", { name: /次の待ちへ/ }));
    await reply(user, 13);
    expect(await screen.findByRole("button", { name: /次の待ちへ/ })).toHaveAccessibleName(/Issue 14/);
  });
});

describe("the keys", () => {
  it("move between the waits with j and k, past running and done issues, and stop at the ends", async () => {
    answer(routes(item(12), item(20, { state: "running", running_since: t0 }), item(13), item(30, { state: "done", done_since: t0 })));
    const user = userEvent.setup();
    at("/");
    await screen.findByRole("heading", { level: 2, name: "返答待ち" });
    await screen.findAllByText("Issue 13");
    await user.keyboard("j");
    await waitFor(() => expect(where()).toBe(path(12)));
    await user.keyboard("j");
    await waitFor(() => expect(where()).toBe(path(13)));
    await user.keyboard("j");
    expect(where()).toBe(path(13));
    await user.keyboard("k");
    await waitFor(() => expect(where()).toBe(path(12)));
    await user.keyboard("k");
    expect(where()).toBe(path(12));
  });

  it("opens the last wait with k when no issue is open", async () => {
    answer(routes(item(12), item(13)));
    const user = userEvent.setup();
    at("/");
    await screen.findAllByText("Issue 13");
    await user.keyboard("k");
    await waitFor(() => expect(where()).toBe(path(13)));
  });

  it("are typed into the reply, not taken", async () => {
    answer(routes(item(12), item(13)));
    const user = userEvent.setup();
    at(url(12));
    await screen.findAllByText("Issue 13");
    await user.type(await screen.findByLabelText("返答"), "jk");
    expect(screen.getByLabelText("返答")).toHaveValue("jk");
    expect(where()).toBe(path(12));
  });

  it("are not taken with Ctrl", async () => {
    answer(routes(item(12), item(13)));
    const user = userEvent.setup();
    at(url(12));
    await screen.findAllByText("Issue 13");
    await user.keyboard("{Control>}j{/Control}");
    expect(where()).toBe(path(12));
  });

  it("are not taken while a dialog is open", async () => {
    answer({ ...routes(item(12), item(13)), "GET /api/issue-targets": { status: 200, body: { items: [], starts: [] } } });
    const user = userEvent.setup();
    at(url(12));
    await screen.findAllByText("Issue 13");
    await user.click(screen.getByRole("button", { name: /新規issue/ }));
    await screen.findByRole("dialog");
    document.body.focus();
    await user.keyboard("j");
    expect(where()).toBe(path(12));
  });

  it("move the focus to the reply with r, leaving it empty", async () => {
    answer(routes(item(12)));
    const user = userEvent.setup();
    at(url(12));
    const field = await screen.findByLabelText("返答");
    expect(field).toHaveAttribute("aria-keyshortcuts", expect.stringContaining("r"));
    await user.keyboard("r");
    expect(field).toHaveFocus();
    expect(field).toHaveValue("");
  });

  it("do nothing with r when there is no reply", async () => {
    answer(routes(item(20, { state: "running", running_since: t0 })));
    const user = userEvent.setup();
    at(url(20));
    expect(await screen.findByText("実行中です。質問か完了になると、この画面が自動で切り替わります。")).toBeInTheDocument();
    await user.keyboard("r");
    expect(document.body).toHaveFocus();
  });

  it("are told under the list", async () => {
    answer(routes(item(12)));
    at("/");
    expect(await screen.findByText("j/k：待ちの間を移る　r：返答の欄へ", { normalizer: (s) => s })).toBeInTheDocument();
  });
});
