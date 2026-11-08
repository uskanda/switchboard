import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { setLanguageChoice } from "@/lib/i18n";

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
const statusPath = "GET /api/merge-status?ref=gh%3Ao%2Fr%2350&sha=abc";

type Check = { name: string; status: "success" | "failure" | "running" | "other"; url: string };

/** status is an answer of /api/merge-status: an open change without a conflict and without checks, unless changed. */
function status(over: { head_moved?: boolean; mergeable?: string; ci?: string; checks?: Check[]; state?: string } = {}): Answer {
  return { status: 200, body: { state: "open", head_moved: false, mergeable: "clean", ci: "none", checks: [], ...over } };
}

/** openDialog opens the English merge dialog of the list's row and returns it. */
async function openDialog() {
  await userEvent.click((await screen.findAllByRole("button", { name: "Merge PR #9" }))[0]);
  return screen.findByRole("dialog", { name: "Merge PR #9?" });
}

/** confirmMerge opens the merge dialog of the list's row and presses its merge button. */
async function confirmMerge() {
  const dialog = await openDialog();
  await userEvent.click(within(dialog).getByRole("button", { name: "Merge" }));
  return dialog;
}

describe("merging in English", () => {
  it("asks before merging a pull request, with the focus on the button that backs out", async () => {
    setLanguageChoice("en");
    const calls = answer({ "GET /api/inbox": listing, [statusPath]: status() });
    at("/");
    const button = (await screen.findAllByRole("button", { name: "Merge PR #9" }))[0];
    expect(button).toHaveTextContent("Merge");
    const dialog = await openDialog();
    expect(within(dialog).getByText("This takes PR #9 out of draft and merges it. You cannot undo it.")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toHaveFocus();
    expect(within(dialog).getByRole("region", { name: "Status of PR #9" })).toBeInTheDocument();
    expect(await within(dialog).findByText("No conflicts")).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(dialog).not.toHaveAttribute("open");
    expect(calls.some((c) => c.key === "POST /api/merge")).toBe(false);
  });

  it("says it is checking while the status is read", async () => {
    setLanguageChoice("en");
    vi.stubGlobal(
      "fetch",
      vi.fn((input: string) => {
        if (input === "/api/inbox") return Promise.resolve(new Response(JSON.stringify(listing.body), { status: 200 }));
        return new Promise<Response>(() => {});
      }),
    );
    at("/");
    const dialog = await openDialog();
    expect(within(dialog).getByText("Checking")).toBeInTheDocument();
  });

  it("says the merge went through in a toast", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/inbox": listing, "POST /api/merge": { status: 204 } });
    at("/");
    await confirmMerge();
    const toast = (await screen.findByText("PR #9 was merged")).closest<HTMLElement>("[data-sonner-toast]")!;
    expect(toast).toHaveClass("toast-success");
    expect(toast).toHaveTextContent('The issue leaves the list once "Closes" in the pull or merge request closes it.');
  });

  it("names a GitLab merge request by its own sign", async () => {
    setLanguageChoice("en");
    const gl = { ...done, ref: "gl:nas/sb#50", project: "nas/sb", change_url: "https://gitlab.test/nas/sb/-/merge_requests/9" };
    answer({ "GET /api/inbox": { status: 200, body: { items: [gl], errors: [], fetched_at: "" } } });
    at("/");
    expect((await screen.findAllByRole("button", { name: "Merge MR !9" }))[0]).toBeInTheDocument();
  });
});

describe("the status of the change in English", () => {
  it("counts the checks and names the state of the listed ones", async () => {
    setLanguageChoice("en");
    answer({
      "GET /api/inbox": listing,
      [statusPath]: status({
        ci: "ok",
        checks: [
          { name: "test", status: "success", url: "https://ci.test/1" },
          { name: "lint", status: "failure", url: "https://ci.test/2" },
          { name: "build", status: "running", url: "https://ci.test/3" },
          { name: "deploy", status: "other", url: "https://ci.test/4" },
        ],
      }),
    });
    at("/");
    const dialog = await openDialog();
    expect(await within(dialog).findByText("CI: 1 passed, 1 failed, 1 running")).toBeInTheDocument();
    expect(within(dialog).getByRole("link", { name: "lint" }).closest("li")).toHaveTextContent("Failed");
    expect(within(dialog).getByRole("link", { name: "build" }).closest("li")).toHaveTextContent("Running");
  });

  it("tells a conflict, commits added later, and a change already merged", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/inbox": listing, [statusPath]: status({ mergeable: "conflict", head_moved: true, state: "merged" }) });
    at("/");
    const dialog = await openDialog();
    expect(await within(dialog).findByText("Has conflicts")).toBeInTheDocument();
    expect(within(dialog).getByText("Commits were added after the page loaded.")).toBeInTheDocument();
    expect(within(dialog).getByText("PR #9 is already merged.")).toBeInTheDocument();
  });

  it("says when the commit has no CI, and when the forge still computes the conflict", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/inbox": listing, [statusPath]: status({ mergeable: "computing" }) });
    at("/");
    const dialog = await openDialog();
    expect(await within(dialog).findByText("No CI")).toBeInTheDocument();
    expect(within(dialog).getByText("The forge is still checking for conflicts.")).toBeInTheDocument();
  });

  it("names the permissions when the inbox token cannot read CI", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/inbox": listing, [statusPath]: status({ ci: "forbidden" }) });
    at("/");
    const dialog = await openDialog();
    expect(
      await within(dialog).findByText("The inbox token has no permission to read CI (on GitHub: read access to Actions and Commit statuses)."),
    ).toBeInTheDocument();
  });

  it("says the status could not be read when the inbox service is down, and the merge can still be pressed", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/inbox": listing, [statusPath]: { status: 503, body: { code: "unavailable", message: "down" } } });
    at("/");
    const dialog = await openDialog();
    expect(await within(dialog).findByText("Could not read the status before the merge. You can still merge.")).toBeInTheDocument();
    expect(within(dialog).getByText("Cannot reach the inbox service (switchboard inbox serve).")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Merge" })).toBeEnabled();
  });
});

describe("a refused merge in English", () => {
  it("tells a stale page with the server's message as it came", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/inbox": listing, "POST /api/merge": { status: 409, body: { code: "stale", message: "the agent pushed def" } } });
    at("/");
    await confirmMerge();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("PR #9 or the issue was updated after the page loaded. Reload the page.");
    expect(alert).toHaveTextContent("the agent pushed def");
  });

  it("tells the forge's refusal", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/inbox": listing, "POST /api/merge": { status: 409, body: { code: "not_mergeable", message: "Required status check is expected" } } });
    at("/");
    await confirmMerge();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("The forge refused to merge PR #9.");
    expect(alert).toHaveTextContent("Required status check is expected");
  });

  it("names the permissions a token without them needs", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/inbox": listing, "POST /api/merge": { status: 403, body: { code: "forbidden", message: "the inbox token may not merge" } } });
    at("/");
    await confirmMerge();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("The inbox token has no permission to merge.");
    expect(alert).toHaveTextContent("On GitHub it needs write access to Pull requests and Contents. On GitLab it needs the api scope.");
  });

  it("tells any other failure with its message", async () => {
    setLanguageChoice("en");
    answer({ "GET /api/inbox": listing, "POST /api/merge": { status: 500, body: { code: "internal", message: "boom" } } });
    at("/");
    await confirmMerge();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not merge PR #9.");
    expect(alert).toHaveTextContent("boom");
  });
});

describe("the automatic merge in English", () => {
  it("says a conflicting change is merged after the fix, and marks the button", async () => {
    setLanguageChoice("en");
    answer({
      "GET /api/inbox": listing,
      "POST /api/merge": { status: 202, body: { code: "auto_merge_pending", message: "merged once the agent has fixed the conflict" } },
    });
    at("/");
    await confirmMerge();
    const toast = (await screen.findByText("PR #9 has conflicts, so it will be merged automatically once the agent fixes them")).closest<HTMLElement>(
      "[data-sonner-toast]",
    )!;
    expect(toast).toHaveTextContent('To stop it, open the issue and press "Cancel auto-merge".');
    const waiting = (await screen.findAllByRole("button", { name: "Auto-merge pending for PR #9" }))[0];
    expect(waiting).toBeDisabled();
    expect(waiting).toHaveTextContent("Auto-merge pending");
  });

  it("names the pending merge in the pane, and cancels it after asking", async () => {
    setLanguageChoice("en");
    const waiting = { ...done, auto_merge: { sha: "abc", since: new Date().toISOString() } };
    const calls = answer({
      "GET /api/inbox": { status: 200, body: { items: [waiting], errors: [], fetched_at: "" } },
      [detailPath]: { ...detail, body: { ...detail.body, auto_merge: waiting.auto_merge } },
      "POST /api/automerge/cancel": { status: 204 },
    });
    at("/issue/gh%3Ao%2Fr%2350");
    await screen.findByRole("heading", { level: 2, name: "Finished" });
    const article = screen.getByRole("article");
    expect(within(article).getByText("Auto-merge pending", { selector: "[data-auto-merge]" })).toBeInTheDocument();

    await userEvent.click(within(article).getByRole("button", { name: "Cancel auto-merge" }));
    const dialog = await screen.findByRole("dialog", { name: "Cancel the auto-merge of PR #9?" });
    expect(
      within(dialog).getByText('PR #9 will not be merged, even once the agent fixes the conflict. To merge it, press "Merge" again.'),
    ).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Keep auto-merge" }));
    expect(calls.some((c) => c.key === "POST /api/automerge/cancel")).toBe(false);

    await userEvent.click(within(article).getByRole("button", { name: "Cancel auto-merge" }));
    await userEvent.click(within(dialog).getByRole("button", { name: "Cancel auto-merge" }));
    expect(await screen.findByText("Auto-merge of PR #9 was cancelled")).toBeInTheDocument();
  });

  it("tells a merge that no longer waits when it is cancelled", async () => {
    setLanguageChoice("en");
    const waiting = { ...done, auto_merge: { sha: "abc", since: new Date().toISOString() } };
    answer({
      "GET /api/inbox": { status: 200, body: { items: [waiting], errors: [], fetched_at: "" } },
      [detailPath]: { ...detail, body: { ...detail.body, auto_merge: waiting.auto_merge } },
      "POST /api/automerge/cancel": { status: 404, body: { code: "not_found", message: "no merge waits" } },
    });
    at("/issue/gh%3Ao%2Fr%2350");
    await screen.findByRole("heading", { level: 2, name: "Finished" });
    const article = screen.getByRole("article");
    await userEvent.click(within(article).getByRole("button", { name: "Cancel auto-merge" }));
    const dialog = await screen.findByRole("dialog", { name: "Cancel the auto-merge of PR #9?" });
    await userEvent.click(within(dialog).getByRole("button", { name: "Cancel auto-merge" }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Auto-merge of PR #9 is no longer pending.");
    expect(alert).toHaveTextContent("It was merged or cancelled. Reload the page.");
  });

  it("tells each finished automatic merge in a toast", async () => {
    setLanguageChoice("en");
    const at1 = new Date(Date.now() - 3600_000).toISOString();
    const results = [
      { ref: "gh:o/r#50", number: 9, outcome: "merged", at: at1 },
      { ref: "gh:o/r#51", number: 10, outcome: "cancelled", reason: "instruction", at: at1 },
    ];
    answer({ "GET /api/inbox": { status: 200, body: { items: [], errors: [], fetched_at: "", auto_merge_results: results } } });
    at("/");
    const merged = (await screen.findByText("PR #9 was merged automatically")).closest<HTMLElement>("[data-sonner-toast]")!;
    expect(merged).toHaveClass("toast-success");
    expect(merged).toHaveTextContent('The issue leaves the list once "Closes" in the pull or merge request closes it.');
    const stopped = (await screen.findByText("Auto-merge of PR #10 was stopped")).closest<HTMLElement>("[data-sonner-toast]")!;
    expect(stopped).toHaveClass("toast-error");
    expect(stopped).toHaveTextContent("A person's instruction started a run. Review the changes, then merge.");
  });
});
