import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { isFailure } from "@/lib/api";

function answer(routes: Record<string, unknown>) {
  const calls: { key: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      calls.push({ key, init });
      if (key === "POST /api/reply" && key in routes) return new Response(null, { status: 204 });
      if (key in routes) return new Response(JSON.stringify(routes[key]), { status: 200 });
      if (key === "GET /api/inbox") return new Response(JSON.stringify({ items: [], errors: [], fetched_at: "" }), { status: 200 });
      if (key === "GET /api/projects") return new Response(JSON.stringify({ host: "ubuntu-a", enabled: false, items: [] }), { status: 200 });
      return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
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

const detail = (reason: string) => ({
  ref: "gh:o/r#12", title: "Add a flag", url: "https://github.test/o/r/issues/12", waiting: true,
  host: "laptop", state: "waiting", reason, since: null, approve_word: "/ok",
  post: "### The CI still fails\n\nThe CI of #48 failed.",
});

describe("waits on a failure after the work", () => {
  it("say that a reply runs the agent again and counts the fixes over (ci)", async () => {
    answer({ "GET /api/issue?ref=gh%3Ao%2Fr%2312": detail("ci") });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByText(/The CI of #48 failed\./)).toBeInTheDocument();
    expect(screen.getByText("ci")).toBeInTheDocument();
    expect(screen.getByText(/自動で直す回数/)).toHaveTextContent("返信すると");
    expect(screen.queryByRole("button", { name: /承認する/ })).toBeNull();
  });

  it("say how to run the fix again and that it counts the fixes over (conflict)", async () => {
    answer({ "GET /api/issue?ref=gh%3Ao%2Fr%2312": detail("conflict") });
    at("/issue/gh%3Ao%2Fr%2312");
    const hint = await screen.findByText(/baseとの衝突を自動で直せませんでした/);
    expect(hint).toHaveTextContent("「再実行する」");
    expect(hint).toHaveTextContent("「返答を送る」");
    expect(hint).toHaveTextContent("自動で直す回数を数え直します");
    expect(screen.getByText("conflict")).toBeInTheDocument();
    expect(isFailure("conflict")).toBe(true);
  });

  it("say what to check before replying (push)", async () => {
    answer({ "GET /api/issue?ref=gh%3Ao%2Fr%2312": detail("push") });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByText(/トークンかリモートの設定/)).toBeInTheDocument();
    expect(screen.getByText("push")).toBeInTheDocument();
  });

  it("show the reason push on the list", async () => {
    answer({
      "GET /api/inbox": {
        items: [{ n: 1, ref: "gh:o/r#12", project: "o/r", number: 12, title: "t", url: "u", host: "h", waiting_since: null, reason: "push", question: "" }],
        errors: [],
        fetched_at: "",
      },
    });
    at("/");
    expect((await screen.findAllByText("push"))[0]).toBeInTheDocument();
  });
});

const issue = "GET /api/issue?ref=gh%3Ao%2Fr%2312";
// The issue on the list: one that left it is not said to have been sent to (it stopped waiting).
const inboxOf = (reason: string) => ({
  items: [{ n: 1, ref: "gh:o/r#12", project: "o/r", number: 12, title: "Add a flag", url: "u", host: "laptop", waiting_since: null, reason, question: "" }],
  errors: [],
  fetched_at: "",
});
const errorInbox = inboxOf("error");

describe("waits on a failed run", () => {
  it("offer to run again and say how (error)", async () => {
    answer({ [issue]: detail("error") });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByRole("button", { name: "再実行する" })).toBeEnabled();
    expect(screen.getByText(/実行が失敗しました/)).toHaveTextContent("「再実行する」");
    expect(screen.getByRole("button", { name: "返答を送る" })).toBeDisabled();
  });

  it("send the fixed text to run again in one click", async () => {
    const calls = answer({ "GET /api/inbox": errorInbox, [issue]: detail("error"), "POST /api/reply": null });
    at("/issue/gh%3Ao%2Fr%2312");
    await userEvent.click(await screen.findByRole("button", { name: "再実行する" }));
    expect(await within(screen.getByRole("article")).findByRole("status")).toHaveTextContent("送りました。エージェントが作業を再開します。");
    const posts = calls.filter((c) => c.key === "POST /api/reply");
    expect(posts).toHaveLength(1);
    expect(JSON.parse(String(posts[0].init?.body))).toEqual({ ref: "gh:o/r#12", text: "再実行してください。" });
  });

  it("offer to run the fix of a conflict again (conflict)", async () => {
    answer({ [issue]: detail("conflict") });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByRole("button", { name: "再実行する" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "返答を送る" })).toBeDisabled();
  });

  it("send the fixed text that asks to resolve the conflict in one click", async () => {
    const calls = answer({ "GET /api/inbox": inboxOf("conflict"), [issue]: detail("conflict"), "POST /api/reply": null });
    at("/issue/gh%3Ao%2Fr%2312");
    await userEvent.click(await screen.findByRole("button", { name: "再実行する" }));
    expect(await within(screen.getByRole("article")).findByRole("status")).toHaveTextContent("送りました");
    const posts = calls.filter((c) => c.key === "POST /api/reply");
    expect(posts).toHaveLength(1);
    expect(JSON.parse(String(posts[0].init?.body))).toEqual({ ref: "gh:o/r#12", text: "baseとの衝突を解消して、もう一度pushしてください。" });
  });

  it.each(["error", "conflict"])("send what is written instead, and not run again over it (%s)", async (reason) => {
    const calls = answer({ "GET /api/inbox": inboxOf(reason), [issue]: detail(reason), "POST /api/reply": null });
    at("/issue/gh%3Ao%2Fr%2312");
    const retry = await screen.findByRole("button", { name: "再実行する" });
    await userEvent.type(screen.getByRole("textbox", { name: "返答" }), "テストを先に直して");
    expect(retry).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "返答を送る" }));
    expect(await within(screen.getByRole("article")).findByRole("status")).toHaveTextContent("送りました");
    const post = calls.find((c) => c.key === "POST /api/reply");
    expect(JSON.parse(String(post?.init?.body))).toEqual({ ref: "gh:o/r#12", text: "テストを先に直して" });
  });

  it.each(["verify", "permission", "push", "ci"])("do not offer to run again (%s)", async (reason) => {
    answer({ [issue]: detail(reason) });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByText(/The CI of #48 failed\./)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "再実行する" })).toBeNull();
  });

  it.each([
    ["error", "running"],
    ["error", "queued"],
    ["conflict", "running"],
    ["conflict", "queued"],
  ])("do not run again once the issue stops waiting (%s, %s)", async (reason, state) => {
    answer({ [issue]: { ...detail(reason), waiting: false, state } });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByText(/The CI of #48 failed\./)).toBeInTheDocument();
    const retry = screen.queryByRole("button", { name: "再実行する" });
    if (retry) expect(retry).toBeDisabled();
  });
});
