import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(JSON.stringify({ items: [], errors: [], fetched_at: "" }), { status: 200 })),
  );
});

afterEach(() => vi.unstubAllGlobals());

describe("a URL that no route matches", () => {
  it("shows the header, says the page does not exist and links to the list", async () => {
    render(
      <MemoryRouter initialEntries={["/foo/bar"]}>
        <AppRoutes />
      </MemoryRouter>,
    );
    expect(await screen.findByRole("heading", { name: "ページがありません" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "設定" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "一覧へ戻る" })).toHaveAttribute("href", "/");
    expect(document.title).toBe("ページがありません - SwitchBoard Inbox");
  });
});
