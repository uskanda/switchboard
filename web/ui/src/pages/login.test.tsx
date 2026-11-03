import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

type Route = { status: number; body?: unknown };

function answer(routes: Record<string, Route>) {
  const calls: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${input}`;
      calls.push(key);
      const r = routes[key];
      if (r) return new Response(r.body === undefined ? null : JSON.stringify(r.body), { status: r.status });
      if (key === "GET /api/inbox") return new Response(JSON.stringify({ items: [], errors: [], fetched_at: "" }), { status: 200 });
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

describe("the login", () => {
  it("shows only the login button to someone not logged in", async () => {
    answer({
      "GET /api/auth": { status: 200, body: { enabled: true, gitlab: "https://gl.example" } },
      "GET /api/inbox": { status: 401, body: { code: "unauthenticated", message: "log in" } },
    });
    at("/");
    const button = await screen.findByRole("link", { name: "GitLabでログイン" });
    expect(button).toHaveAttribute("href", "/auth/login");
    expect(screen.queryByText("設定")).toBeNull();
  });

  it("says who was refused", async () => {
    answer({ "GET /api/auth": { status: 200, body: { enabled: true } } });
    at("/login?error=not_allowed&user=mallory");
    expect(await screen.findByRole("alert")).toHaveTextContent("mallory");
  });

  it("shows the inbox with the user and the logout once logged in", async () => {
    answer({
      "GET /api/auth": { status: 200, body: { enabled: true, user: "alice" } },
      "POST /api/logout": { status: 204 },
    });
    at("/");
    expect(await screen.findByText("alice")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "ログアウト" }));
    expect(await screen.findByRole("link", { name: "GitLabでログイン" })).toBeInTheDocument();
  });

  it("turns to the login page when a call says the session ended", async () => {
    answer({
      "GET /api/auth": { status: 200, body: { enabled: true, user: "alice" } },
      "GET /api/inbox": { status: 401, body: { code: "unauthenticated", message: "log in" } },
    });
    at("/");
    expect(await screen.findByRole("link", { name: "GitLabでログイン" })).toBeInTheDocument();
  });

  it("is not there without inbox.auth", async () => {
    answer({ "GET /api/auth": { status: 200, body: { enabled: false } } });
    at("/");
    expect(await screen.findByText("設定")).toBeInTheDocument();
    expect(screen.queryByText("ログアウト")).toBeNull();
  });
});
