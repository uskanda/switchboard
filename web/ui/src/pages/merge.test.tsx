import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { dismiss } from "@/lib/toast";

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

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

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

  it("puts the focus on the button that backs out when the dialog opens", async () => {
    answer({ "GET /api/inbox": listing });
    at("/");
    await userEvent.click((await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0]);
    const dialog = await screen.findByRole("dialog", { name: "PR #9をマージしますか" });
    expect(within(dialog).getByRole("button", { name: "やめる" })).toHaveFocus();
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
    // The toast, not the row's own status that the merge is being processed.
    const status = (await screen.findByText("PR #9をマージしました")).closest<HTMLElement>("[role=status]")!;
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

  it("says in a toast that a conflicting change is merged after the fix, and stops the button", async () => {
    answer({
      "GET /api/inbox": listing,
      "POST /api/merge": { status: 202, body: { code: "auto_merge_pending", message: "merged once the agent has fixed the conflict" } },
    });
    at("/");
    await userEvent.click((await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0]);
    const dialog = await screen.findByRole("dialog", { name: "PR #9をマージしますか" });
    await userEvent.click(within(dialog).getByRole("button", { name: "マージする" }));
    const status = (await screen.findByText("PR #9は衝突しているため、エージェントが直したあと自動でマージします")).closest<HTMLElement>("[role=status]")!;
    expect(status.closest("[data-sonner-toast]")).toHaveClass("toast-info");
    await waitFor(() => expect(dialog).not.toHaveAttribute("open"));
    const waiting = screen.getAllByRole("button", { name: "PR #9は自動マージ待ち" })[0];
    expect(waiting).toBeDisabled();
    expect(waiting).toHaveTextContent("自動マージ待ち");
  });

  it("names the merge waiting for the fix on a row and in the pane, where it can be cancelled", async () => {
    const waiting = { ...done, auto_merge: { sha: "abc", since: new Date().toISOString() } };
    const calls = answer({
      "GET /api/inbox": { status: 200, body: { items: [waiting], errors: [], fetched_at: "" } },
      [detailPath]: { ...detail, body: { ...detail.body, auto_merge: waiting.auto_merge } },
      "POST /api/automerge/cancel": { status: 204 },
    });
    at("/issue/gh%3Ao%2Fr%2350");
    await screen.findByRole("heading", { level: 2, name: "Finished" });
    const article = screen.getByRole("article");
    expect(within(article).getByText("自動マージ待ち", { selector: "[data-auto-merge]" })).toBeInTheDocument();
    expect(within(article).getByRole("button", { name: "PR #9は自動マージ待ち" })).toBeDisabled();
    const list = screen.getByRole("navigation", { name: /一覧/ });
    expect(within(list).getAllByText("自動マージ待ち", { selector: "[data-auto-merge]" })[0]).toBeInTheDocument();

    await userEvent.click(within(article).getByRole("button", { name: "自動マージを取り消す" }));
    const dialog = await screen.findByRole("dialog", { name: "PR #9の自動マージを取り消しますか" });
    await userEvent.click(within(dialog).getByRole("button", { name: "やめる" }));
    expect(calls.some((c) => c.key === "POST /api/automerge/cancel")).toBe(false);

    await userEvent.click(within(article).getByRole("button", { name: "自動マージを取り消す" }));
    await userEvent.click(within(dialog).getByRole("button", { name: "取り消す" }));
    expect(await screen.findByText("PR #9の自動マージを取り消しました")).toBeInTheDocument();
    const sent = calls.find((c) => c.key === "POST /api/automerge/cancel");
    expect(JSON.parse(String(sent?.init?.body))).toEqual({ ref: "gh:o/r#50" });
    await waitFor(() => expect(dialog).not.toHaveAttribute("open"));
  });

  it("tells each finished automatic merge once", async () => {
    const at1 = new Date(Date.now() - 3600_000).toISOString();
    const results = [
      { ref: "gh:o/r#50", number: 9, outcome: "merged", at: at1 },
      { ref: "gh:o/r#51", number: 10, outcome: "cancelled", reason: "instruction", at: at1 },
    ];
    answer({ "GET /api/inbox": { status: 200, body: { items: [], errors: [], fetched_at: "", auto_merge_results: results } } });
    const first = at("/");
    const merged = (await screen.findByText("PR #9を自動でマージしました")).closest("[data-sonner-toast]");
    expect(merged).toHaveClass("toast-success");
    const stopped = (await screen.findByText("PR #10の自動マージをやめました")).closest<HTMLElement>("[data-sonner-toast]")!;
    expect(stopped).toHaveClass("toast-error");
    expect(stopped).toHaveTextContent("人の指示で実行が始まりました");
    first.unmount();
    dismiss();
    await waitFor(() => expect(screen.queryByText("PR #9を自動でマージしました")).toBeNull());
    at("/");
    await screen.findByRole("navigation", { name: "一覧" });
    await new Promise((r) => setTimeout(r, 50));
    expect(screen.queryByText("PR #9を自動でマージしました")).toBeNull();
  });

  it("names a GitLab merge request by its own sign", async () => {
    const gl = { ...done, ref: "gl:nas/sb#50", project: "nas/sb", change_url: "https://gitlab.test/nas/sb/-/merge_requests/9" };
    answer({ "GET /api/inbox": { status: 200, body: { items: [gl], errors: [], fetched_at: "" } } });
    at("/");
    expect((await screen.findAllByRole("button", { name: "MR !9をマージ" }))[0]).toBeInTheDocument();
  });
});

describe("opening the merge dialog from the address (the app's merge button)", () => {
  it("opens the dialog once the issue is read, and sends nothing until confirmed", async () => {
    const calls = answer({ "GET /api/inbox": listing, [detailPath]: detail });
    at("/issue/gh%3Ao%2Fr%2350?action=merge");
    const dialog = await screen.findByRole("dialog", { name: "PR #9をマージしますか" });
    expect(dialog).toHaveAttribute("open");
    expect(calls.some((c) => c.key === "POST /api/merge")).toBe(false);
    await userEvent.click(within(dialog).getByRole("button", { name: "やめる" }));
    expect(dialog).not.toHaveAttribute("open");
  });

  it("does not open it again after it is closed", async () => {
    answer({ "GET /api/inbox": listing, [detailPath]: detail });
    at("/issue/gh%3Ao%2Fr%2350?action=merge");
    const dialog = await screen.findByRole("dialog", { name: "PR #9をマージしますか" });
    await userEvent.click(within(dialog).getByRole("button", { name: "やめる" }));
    await new Promise((r) => setTimeout(r, 50));
    for (const d of screen.getAllByRole("dialog", { hidden: true })) expect(d).not.toHaveAttribute("open");
  });

  it("tells in an error toast that an issue without the merge cannot be merged", async () => {
    const closed = { status: 200, body: { ...detail.body, done: false, change_number: undefined, change_sha: undefined } };
    answer({ "GET /api/inbox": { status: 200, body: { items: [], errors: [], fetched_at: "" } }, [detailPath]: closed });
    at("/issue/gh%3Ao%2Fr%2350?action=merge");
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("このissueは、ここからはマージできません");
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
