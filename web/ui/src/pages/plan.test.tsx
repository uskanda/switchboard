import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

type Answer = { status: number; body?: unknown };

function answer(routes: Record<string, Answer | Answer[]>) {
  const calls: { key: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      calls.push({ key, init });
      const r = routes[key];
      const a = Array.isArray(r) ? r.shift() : r;
      if (!a && key === "GET /api/inbox") return new Response(JSON.stringify({ items: [], errors: [], fetched_at: "" }), { status: 200 });
      if (!a && key === "GET /api/projects") return new Response(JSON.stringify({ host: "ubuntu-a", enabled: false, items: [] }), { status: 200 });
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

const instructions = {
  host: "ubuntu-a",
  items: [
    { target: "", content: "" },
    { target: "gh:o/r", content: "Mind the API names." },
    { target: "gl:nas/sb", content: "" },
  ],
};

describe("the settings", () => {
  it("are linked from the header of the inbox", async () => {
    answer({ "GET /api/inbox": { status: 200, body: { items: [], errors: [], fetched_at: "" } } });
    at("/");
    expect(await screen.findByRole("link", { name: "設定" })).toHaveAttribute("href", "/settings");
  });

  it("show every instruction and the host whose files they are", async () => {
    answer({ "GET /api/instructions": { status: 200, body: instructions } });
    at("/settings");
    expect(await screen.findByRole("textbox", { name: "すべてのプロジェクト" })).toHaveValue("");
    expect(screen.getByRole("textbox", { name: "gh:o/r" })).toHaveValue("Mind the API names.");
    expect(screen.getByRole("textbox", { name: "gl:nas/sb" })).toBeInTheDocument();
    expect(screen.getByText(/のファイルです/)).toHaveTextContent("ubuntu-a");
  });

  it("save one field and say so in a toast", async () => {
    const calls = answer({ "GET /api/instructions": { status: 200, body: instructions }, "POST /api/instructions": { status: 204 } });
    at("/settings");
    const field = await screen.findByRole("textbox", { name: "gl:nas/sb" });
    await userEvent.type(field, "Ask in Japanese.");
    const card = field.closest("section")!;
    await userEvent.click(within(card as HTMLElement).getByRole("button", { name: "保存する" }));
    const status = await screen.findByRole("status");
    expect(status).toHaveTextContent("保存しました");
    expect(status.closest("[data-sonner-toast]")).toHaveClass("toast-success");
    expect(within(card as HTMLElement).queryByText("保存しました")).toBeNull();
    const post = calls.find((c) => c.key === "POST /api/instructions");
    expect(JSON.parse(String(post?.init?.body))).toEqual({ target: "gl:nas/sb", content: "Ask in Japanese." });
    expect(new Headers(post?.init?.headers).get("Content-Type")).toBe("application/json");
  });

  it("show a refusal in its own field and keep the other fields", async () => {
    answer({
      "GET /api/instructions": { status: 200, body: instructions },
      "POST /api/instructions": { status: 422, body: { code: "secret", message: "secret" } },
    });
    at("/settings");
    const global = await screen.findByRole("textbox", { name: "すべてのプロジェクト" });
    await userEvent.type(global, "keep me");
    const field = screen.getByRole("textbox", { name: "gh:o/r" });
    await userEvent.type(field, " ghp_x");
    const card = field.closest("section") as HTMLElement;
    await userEvent.click(within(card).getByRole("button", { name: "保存する" }));
    await waitFor(() => expect(within(card).getByRole("alert")).toHaveTextContent("秘密"));
    expect(global).toHaveValue("keep me");
    expect(field).toHaveValue("Mind the API names. ghp_x");
    expect(screen.getAllByRole("alert")).toHaveLength(1);
  });

  it("say when the inbox service is not reachable", async () => {
    answer({ "GET /api/instructions": { status: 503, body: { code: "unavailable", message: "x" } } });
    at("/settings");
    expect(await screen.findByRole("alert")).toHaveTextContent("受信箱のサービス");
  });
});

const planDetail = {
  ref: "gh:o/r#12", title: "Add a flag", url: "https://github.test/o/r/issues/12", waiting: true,
  host: "laptop", state: "waiting", reason: "plan", since: null, approve_word: "/ok",
  post: "### The plan is ready for approval\n\n1. Add --dry-run.",
};
// The issue on the list: one that left it is not said to have been sent to (it stopped waiting).
const planInbox = {
  status: 200,
  body: { items: [{ n: 1, ref: "gh:o/r#12", project: "o/r", number: 12, title: "Add a flag", url: "u", host: "laptop", waiting_since: null, reason: "plan", question: "" }], errors: [], fetched_at: "" },
};

describe("plan waits", () => {
  it("show the plan and how to approve it", async () => {
    answer({ "GET /api/issue?ref=gh%3Ao%2Fr%2312": { status: 200, body: planDetail } });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByText(/1\. Add --dry-run\./)).toBeInTheDocument();
    expect(screen.getByText("plan")).toBeInTheDocument();
    expect(screen.getByText(/と送ると承認になり/)).toHaveTextContent("/ok");
  });

  it("approve with the approval word alone in one click", async () => {
    const calls = answer({ "GET /api/inbox": planInbox, "GET /api/issue?ref=gh%3Ao%2Fr%2312": { status: 200, body: planDetail }, "POST /api/reply": { status: 204 } });
    at("/issue/gh%3Ao%2Fr%2312");
    const approve = await screen.findByRole("button", { name: "承認する（/ok を送る）" });
    expect(screen.getByRole("button", { name: "返答を送る" })).toBeDisabled();
    await userEvent.click(approve);
    expect(await screen.findByRole("status")).toHaveTextContent("送りました");
    const post = calls.find((c) => c.key === "POST /api/reply");
    expect(JSON.parse(String(post?.init?.body))).toEqual({ ref: "gh:o/r#12", text: "/ok" });
  });

  it("name and send the configured approval word", async () => {
    const calls = answer({
      "GET /api/inbox": planInbox,
      "GET /api/issue?ref=gh%3Ao%2Fr%2312": { status: 200, body: { ...planDetail, approve_word: "/lgtm" } },
      "POST /api/reply": { status: 204 },
    });
    at("/issue/gh%3Ao%2Fr%2312");
    await userEvent.click(await screen.findByRole("button", { name: "承認する（/lgtm を送る）" }));
    await screen.findByRole("status");
    const post = calls.find((c) => c.key === "POST /api/reply");
    expect(JSON.parse(String(post?.init?.body))).toEqual({ ref: "gh:o/r#12", text: "/lgtm" });
  });

  it("do not approve while something is written in the reply", async () => {
    answer({ "GET /api/issue?ref=gh%3Ao%2Fr%2312": { status: 200, body: planDetail } });
    at("/issue/gh%3Ao%2Fr%2312");
    const approve = await screen.findByRole("button", { name: "承認する（/ok を送る）" });
    const field = screen.getByRole("textbox", { name: "返答" });
    await userEvent.type(field, "   ");
    expect(approve).toBeEnabled();
    await userEvent.type(field, "名前を変えて");
    expect(approve).toBeDisabled();
    expect(screen.getByRole("button", { name: "返答を送る" })).toBeEnabled();
  });

  it("do not approve once the issue stops waiting", async () => {
    answer({ "GET /api/issue?ref=gh%3Ao%2Fr%2312": { status: 200, body: { ...planDetail, waiting: false } } });
    at("/issue/gh%3Ao%2Fr%2312");
    expect(await screen.findByText(/1\. Add --dry-run\./)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /承認する/ })).toBeNull();
  });

  it("show the reason of the plan questions on the list", async () => {
    answer({
      "GET /api/inbox": {
        status: 200,
        body: { items: [{ n: 1, ref: "gh:o/r#12", project: "o/r", number: 12, title: "t", url: "u", host: "h", waiting_since: null, reason: "plan-questions", question: "Q1" }], errors: [], fetched_at: "" },
      },
    });
    at("/");
    expect((await screen.findAllByText("plan-questions"))[0]).toBeInTheDocument();
  });
});
