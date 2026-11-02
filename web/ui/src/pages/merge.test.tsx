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

describe("merging a done issue's pull request", () => {
  it("is offered on the list's row, outside the row's link", async () => {
    answer({ "GET /api/inbox": listing });
    at("/");
    const button = (await screen.findAllByRole("button", { name: "PR #9をマージ" }))[0];
    expect(button.closest("a")).toBeNull();
  });

  it("sits inside the card's frame with the change's link, below and outside the card's link to the issue", async () => {
    answer({ "GET /api/inbox": listing, [detailPath]: detail });
    at("/issue/gh%3Ao%2Fr%2350");
    const buttons = await screen.findAllByRole("button", { name: "PR #9をマージ" });
    const button = buttons.find((b) => b.closest("li"))!;
    const card = button.closest("li")!.firstElementChild as HTMLElement;
    const issueLink = within(card).getAllByRole("link").find((a) => a.getAttribute("href") === "/issue/gh%3Ao%2Fr%2350")!;
    const changeLink = within(card).getByRole("link", { name: "PR #9" });
    expect(card).toContainElement(button);
    expect(issueLink).toHaveAttribute("aria-current", "page");
    expect(issueLink).not.toContainElement(button);
    expect(issueLink).not.toContainElement(changeLink);
    expect(card).toHaveClass("border-primary");
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

  it("names a GitLab merge request by its own sign", async () => {
    const gl = { ...done, ref: "gl:nas/sb#50", project: "nas/sb", change_url: "https://gitlab.test/nas/sb/-/merge_requests/9" };
    answer({ "GET /api/inbox": { status: 200, body: { items: [gl], errors: [], fetched_at: "" } } });
    at("/");
    expect((await screen.findAllByRole("button", { name: "MR !9をマージ" }))[0]).toBeInTheDocument();
  });
});
