import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { GROUP_BY_PROJECT_KEY } from "@/lib/groupPref";

function answer(routes: Record<string, unknown>) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      if (key in routes) return new Response(JSON.stringify(routes[key]), { status: 200 });
      return new Response(JSON.stringify({ code: "not_found", message: key }), { status: 404 });
    }),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  window.localStorage.clear();
});

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

const ago = (min: number) => new Date(Date.now() - min * 60_000).toISOString();
const waiting = (ref: string, title: string, min: number) => {
  const [project, number] = ref.slice(3).split("#");
  return {
    n: 1, state: "waiting", ref, project, number: Number(number), title, url: "https://forge.test/", host: "laptop",
    waiting_since: ago(min), reason: "question", question: "?",
  };
};
// The inbox service gives each section oldest first.
const items = [
  waiting("gh:c/d#1", "Oldest in c/d", 30),
  waiting("gh:a/b#2", "Oldest in a/b", 20),
  waiting("gl:c/d#3", "On GitLab", 15),
  waiting("gh:c/d#4", "Newer in c/d", 10),
  { ...waiting("gh:a/b#5", "Runs", 0), n: undefined, state: "running", waiting_since: null, running_since: ago(5), progress: "working" },
  { ...waiting("gh:a/b#6", "Finished", 0), n: 2, state: "done", waiting_since: null, done_since: ago(60) },
];
const listing = { items, errors: [], fetched_at: ago(0) };

/** section is the list's section under a heading. */
const section = async (name: string) => (await screen.findByRole("heading", { level: 2, name })).closest("section") as HTMLElement;

/** cards is the card list of a section (the table holds the same items for another width). */
const cards = (s: HTMLElement) => s.querySelector(":scope > div:last-of-type") as HTMLElement;

/** shown is the text of an element without the parts hidden from assistive technology, such as the project's icon. */
const shown = (h: HTMLElement) => {
  const copy = h.cloneNode(true) as HTMLElement;
  copy.querySelectorAll("[aria-hidden='true']").forEach((n) => n.remove());
  return copy.textContent;
};

const headings = (el: HTMLElement) => within(el).queryAllByRole("heading", { level: 3 }).map(shown);

describe("the list grouped by project", () => {
  it("puts each section's rows under their project, the one waiting longest first", async () => {
    answer({ "GET /api/inbox": listing });
    at("/");
    const s = cards(await section("返答待ち"));
    expect(headings(s)).toEqual(["c/d GitHub 2件", "a/b GitHub 1件", "c/d GitLab 1件"]);
    const groups = within(s).getAllByRole("list");
    expect(within(groups[0]).getAllByRole("link").map((a) => a.textContent)).toEqual([
      expect.stringContaining("Oldest in c/d"),
      expect.stringContaining("Newer in c/d"),
    ]);
    expect(within(groups[1]).getAllByRole("link")).toHaveLength(1);
    expect(within(groups[2]).getByRole("link")).toHaveTextContent("On GitLab");
  });

  it("gives the table a heading for each project too", async () => {
    answer({ "GET /api/inbox": listing });
    at("/");
    const table = within(await section("返答待ち")).getByRole("table");
    expect(headings(table)).toEqual(["c/d GitHub 2件", "a/b GitHub 1件", "c/d GitLab 1件"]);
  });

  it("heads a section of one project too", async () => {
    answer({ "GET /api/inbox": listing });
    at("/");
    expect(headings(cards(await section("実行中")))).toEqual(["a/b GitHub 1件"]);
    expect(headings(cards(await section("完了（close待ち）")))).toEqual(["a/b GitHub 1件"]);
  });

  it("keeps the selected row marked", async () => {
    answer({ "GET /api/inbox": listing });
    at("/issue/gh%3Aa%2Fb%232");
    const s = cards(await section("返答待ち"));
    expect(within(s).getByRole("link", { current: "page" })).toHaveTextContent("Oldest in a/b");
  });
});

describe("the grouping toggle", () => {
  it("is pressed at first", async () => {
    answer({ "GET /api/inbox": listing });
    at("/");
    expect(await screen.findByRole("button", { name: /プロジェクトでまとめる/ })).toHaveAttribute("aria-pressed", "true");
  });

  it("turns the grouping off in every section, and remembers it", async () => {
    answer({ "GET /api/inbox": listing });
    const user = userEvent.setup();
    const view = at("/");
    await section("返答待ち");
    await user.click(screen.getByRole("button", { name: /プロジェクトでまとめる/ }));
    expect(screen.getByRole("button", { name: /プロジェクトでまとめる/ })).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryAllByRole("heading", { level: 3 })).toHaveLength(0);
    const s = cards(await section("返答待ち"));
    expect(within(s).getAllByRole("link").map((a) => a.textContent)).toEqual([
      expect.stringContaining("Oldest in c/d"),
      expect.stringContaining("Oldest in a/b"),
      expect.stringContaining("On GitLab"),
      expect.stringContaining("Newer in c/d"),
    ]);
    expect(window.localStorage.getItem(GROUP_BY_PROJECT_KEY)).toBe("false");

    view.unmount();
    at("/");
    await section("返答待ち");
    expect(screen.getByRole("button", { name: /プロジェクトでまとめる/ })).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryAllByRole("heading", { level: 3 })).toHaveLength(0);
  });
});
