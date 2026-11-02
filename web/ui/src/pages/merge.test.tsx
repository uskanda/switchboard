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

const base = { project: "o/r", url: "https://github.test/o/r/issues/50", waiting_since: null, question: "?", reason: "" };
const done = {
  ...base, n: 1, state: "done", ref: "gh:o/r#50", number: 50, title: "Finished", host: "desk",
  done_since: new Date(Date.now() - 2 * 3600_000).toISOString(), change_url: "https://github.test/o/r/pull/9", change_number: 9, change_sha: "abc",
};
const listing = { status: 200, body: { items: [done], errors: [], fetched_at: "" } };
const detail = {
  status: 200,
  body: {
    ref: "gh:o/r#50", title: "Finished", url: "https://github.test/o/r/issues/50", waiting: false, done: true, host: "desk",
    state: "done", reason: "?", since: done.done_since, post: "### The work is done",
    change_url: "https://github.test/o/r/pull/9", change_number: 9, change_sha: "abc",
  },
};
const detailPath = "GET /api/issue?ref=gh%3Ao%2Fr%2350";
const statusPath = "GET /api/merge-status?ref=gh%3Ao%2Fr%2350&sha=abc";

type Check = { name: string; status: "success" | "failure" | "running" | "other"; url: string };

/** status is an answer of /api/merge-status: an open change without a conflict and without checks, unless changed. */
function status(over: { head_moved?: boolean; mergeable?: string; ci?: string; checks?: Check[]; state?: string } = {}): Answer {
  return { status: 200, body: { state: "open", head_moved: false, mergeable: "clean", ci: "none", checks: [], ...over } };
}

/** openDialog opens the merge dialog of the list's row and returns it. */
async function openDialog() {
  await userEvent.click((await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0]);
  return screen.findByRole("dialog", { name: "PR #9をマージしますか" });
}

describe("the status of the change in the merge dialog", () => {
  it("says it is checking while the status is read, and the merge can still be pressed", async () => {
    let release: (r: Response) => void = () => {};
    vi.stubGlobal(
      "fetch",
      vi.fn((input: string) => {
        if (input === "/api/inbox") return Promise.resolve(new Response(JSON.stringify(listing.body), { status: 200 }));
        return new Promise<Response>((resolve) => (release = resolve));
      }),
    );
    at("/");
    const dialog = await openDialog();
    expect(within(dialog).getByText("確かめています")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "マージする" })).toBeEnabled();
    release(new Response(JSON.stringify(status().body), { status: 200 }));
    expect(await within(dialog).findByText("衝突なし")).toBeInTheDocument();
  });

  it("counts the checks, and links the failing and running ones only", async () => {
    answer({
      "GET /api/inbox": listing,
      [statusPath]: status({
        ci: "ok",
        checks: [
          { name: "test", status: "success", url: "https://ci.test/1" },
          { name: "<b>lint</b>", status: "failure", url: "https://ci.test/2" },
          { name: "build", status: "running", url: "https://ci.test/3" },
          { name: "deploy", status: "other", url: "https://ci.test/4" },
        ],
      }),
    });
    at("/");
    const dialog = await openDialog();
    expect(await within(dialog).findByText("CI：成功 1／失敗 1／実行中 1")).toBeInTheDocument();
    expect(within(dialog).getByRole("link", { name: "<b>lint</b>" })).toHaveAttribute("href", "https://ci.test/2");
    expect(within(dialog).getByRole("link", { name: "build" })).toHaveAttribute("href", "https://ci.test/3");
    expect(within(dialog).queryByRole("link", { name: "test" })).toBeNull();
    expect(within(dialog).queryByRole("link", { name: "deploy" })).toBeNull();
    expect(dialog.querySelector("b")).toBeNull();
  });

  it("does not link a check whose address is not a web page", async () => {
    answer({ "GET /api/inbox": listing, [statusPath]: status({ ci: "ok", checks: [{ name: "evil", status: "failure", url: "javascript:alert(1)" }] }) });
    at("/");
    const dialog = await openDialog();
    expect(await within(dialog).findByText(/evil/)).toBeInTheDocument();
    expect(within(dialog).queryByRole("link", { name: "evil" })).toBeNull();
  });

  it("says when the commit has no CI", async () => {
    answer({ "GET /api/inbox": listing, [statusPath]: status() });
    at("/");
    const dialog = await openDialog();
    expect(await within(dialog).findByText("CIはありません")).toBeInTheDocument();
  });

  it("tells a conflict, and the forge still computing", async () => {
    answer({ "GET /api/inbox": listing, [statusPath]: status({ mergeable: "conflict" }) });
    at("/");
    let dialog = await openDialog();
    expect(await within(dialog).findByText("衝突あり")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "マージする" })).toBeEnabled();
    await userEvent.click(within(dialog).getByRole("button", { name: "やめる" }));

    answer({ "GET /api/inbox": listing, [statusPath]: status({ mergeable: "computing" }) });
    dialog = await openDialog();
    expect(await within(dialog).findByText(/フォージが計算中/)).toBeInTheDocument();
  });

  it("says the change has commits the page did not show", async () => {
    answer({ "GET /api/inbox": listing, [statusPath]: status({ head_moved: true }) });
    at("/");
    const dialog = await openDialog();
    expect(await within(dialog).findByText("画面を読んだ後にコミットが増えています。")).toBeInTheDocument();
  });

  it("names the permissions when the inbox token cannot read CI", async () => {
    answer({ "GET /api/inbox": listing, [statusPath]: status({ ci: "forbidden", mergeable: "conflict" }) });
    at("/");
    const dialog = await openDialog();
    expect(await within(dialog).findByText(/受信箱のトークンにCIを読む権限がありません/)).toHaveTextContent("Actions と Commit statuses の読み取り");
    expect(within(dialog).getByText("衝突あり")).toBeInTheDocument();
  });

  it("says the status could not be read, and the merge can still be sent", async () => {
    const calls = answer({
      "GET /api/inbox": listing,
      [statusPath]: { status: 503, body: { code: "unavailable", message: "down" } },
      "POST /api/merge": { status: 204 },
    });
    at("/");
    const dialog = await openDialog();
    expect(await within(dialog).findByText(/マージの前の状態を読めませんでした/)).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "マージする" }));
    await waitFor(() => expect(calls.some((c) => c.key === "POST /api/merge")).toBe(true));
  });

  it("reads the status again each time the dialog opens", async () => {
    const calls = answer({ "GET /api/inbox": listing, [statusPath]: status() });
    at("/");
    let dialog = await openDialog();
    await within(dialog).findByText("衝突なし");
    await userEvent.click(within(dialog).getByRole("button", { name: "やめる" }));
    dialog = await openDialog();
    await within(dialog).findByText("衝突なし");
    expect(calls.filter((c) => c.key === statusPath)).toHaveLength(2);
  });
});

describe("merging a done issue's pull request", () => {
  it("is offered on the list's row, outside the row's link", async () => {
    answer({ "GET /api/inbox": listing });
    at("/");
    const button = (await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0];
    expect(button.closest("a")).toBeNull();
  });

  it("is offered in the issue's pane", async () => {
    answer({ "GET /api/inbox": listing, [detailPath]: detail });
    at("/issue/gh%3Ao%2Fr%2350");
    await screen.findByRole("heading", { level: 2, name: "Finished" });
    const article = screen.getByRole("article");
    expect(within(article).getByRole("button", { name: "PR #9をマージ" })).toBeInTheDocument();
  });

  it("is not offered without the number and the commit of the change", async () => {
    const old = { ...done, change_number: undefined, change_sha: undefined };
    answer({ "GET /api/inbox": { status: 200, body: { items: [old], errors: [], fetched_at: "" } } });
    at("/");
    await screen.findAllByText("Finished");
    expect(screen.queryByRole("button", { name: /マージ/ })).toBeNull();
    expect(screen.getAllByRole("link", { name: "https://github.test/o/r/pull/9" })[0]).toBeInTheDocument();
  });

  it("sends nothing when the dialog is dismissed", async () => {
    const calls = answer({ "GET /api/inbox": listing });
    at("/");
    await userEvent.click((await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0]);
    const dialog = await screen.findByRole("dialog", { name: "PR #9をマージしますか" });
    expect(within(dialog).getByText("PR #9のdraftを解除してマージします。取り消せません。")).toBeInTheDocument();
    expect(within(dialog).getByRole("link", { name: "https://github.test/o/r/pull/9" })).toHaveAttribute("href", "https://github.test/o/r/pull/9");
    await userEvent.click(within(dialog).getByRole("button", { name: "やめる" }));
    expect(dialog).not.toHaveAttribute("open");
    expect(calls.some((c) => c.key === "POST /api/merge")).toBe(false);
  });

  it("sends the issue and the commit once confirmed, then says so in a toast", async () => {
    const calls = answer({ "GET /api/inbox": listing, "POST /api/merge": { status: 204 } });
    at("/");
    await userEvent.click((await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0]);
    const dialog = await screen.findByRole("dialog", { name: "PR #9をマージしますか" });
    await userEvent.click(within(dialog).getByRole("button", { name: "マージする" }));
    const status = await screen.findByRole("status");
    expect(status).toHaveTextContent("PR #9をマージしました");
    expect(status.closest("[data-sonner-toast]")).toHaveClass("toast-success");
    const sent = calls.find((c) => c.key === "POST /api/merge");
    expect(JSON.parse(String(sent?.init?.body))).toEqual({ ref: "gh:o/r#50", sha: "abc" });
    await waitFor(() => expect(dialog).not.toHaveAttribute("open"));
  });

  it("tells a stale page in an error toast", async () => {
    answer({ "GET /api/inbox": listing, "POST /api/merge": { status: 409, body: { code: "stale", message: "the agent pushed def" } } });
    at("/");
    await userEvent.click((await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0]);
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "マージする" }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("画面を読んだ後に変わっています");
    expect(alert).toHaveTextContent("the agent pushed def");
    expect(alert.closest("[data-sonner-toast]")).toHaveClass("toast-error");
  });

  it("tells the forge's refusal in an error toast", async () => {
    answer({ "GET /api/inbox": listing, "POST /api/merge": { status: 409, body: { code: "not_mergeable", message: "Required status check is expected" } } });
    at("/");
    await userEvent.click((await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0]);
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "マージする" }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("フォージがPR #9のマージを断りました");
    expect(alert).toHaveTextContent("Required status check is expected");
  });

  it("tells a token without the permission in an error toast", async () => {
    answer({ "GET /api/inbox": listing, "POST /api/merge": { status: 403, body: { code: "forbidden", message: "the inbox token may not merge" } } });
    at("/");
    await userEvent.click((await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0]);
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "マージする" }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("マージする権限がありません");
    expect(alert).toHaveTextContent("Pull requests と Contents");
  });

  it("asks for the status of the change with the issue and the commit when the dialog opens", async () => {
    const calls = answer({ "GET /api/inbox": listing, [statusPath]: status() });
    at("/");
    await userEvent.click((await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0]);
    await screen.findByText("衝突なし");
    expect(calls.filter((c) => c.key === statusPath)).toHaveLength(1);
  });

  it("names a GitLab merge request by its own sign", async () => {
    const gl = { ...done, ref: "gl:nas/sb#50", project: "nas/sb", change_url: "https://gitlab.test/nas/sb/-/merge_requests/9" };
    answer({ "GET /api/inbox": { status: 200, body: { items: [gl], errors: [], fetched_at: "" } } });
    at("/");
    expect((await screen.findAllByRole("button", { name: "MR !9をマージ" }))[0]).toBeInTheDocument();
  });
});
