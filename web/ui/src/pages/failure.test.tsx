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
      if (key === "GET /api/projects") return new Response(JSON.stringify({ host: "ubuntu-a", enabled: false, items: [] }), { status: 200 });
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
