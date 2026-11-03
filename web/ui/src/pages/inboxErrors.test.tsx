import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });

/** answer stubs fetch: the inbox answers with inbox, the issue and the settings with what they need. */
function answer(inbox: () => Response) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string) => {
      if (input === "/api/inbox") return inbox();
      if (input.startsWith("/api/issue?")) {
        return json({ ref: "gh:o/r#12", title: "Add a flag", url: "u", waiting: true, host: "laptop", state: "waiting", reason: "question", since: null, approve_word: "/ok", post: "A question." });
      }
      if (input === "/api/projects") return json({ host: "ubuntu-a", enabled: false, items: [] });
      return json({ code: "unavailable", message: "down" }, 503);
    }),
  );
}

const unreachable = () => json({ code: "unavailable", message: "down" }, 503);

afterEach(() => vi.unstubAllGlobals());

function at(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

describe("the inbox errors above the panes", () => {
  it("show that the inbox service is unreachable outside the list, while one issue is open", async () => {
    answer(unreachable);
    at("/issue/gh%3Ao%2Fr%2312");
    const alert = await screen.findByText(/受信箱のサービス（switchboard inbox serve）に届きません/);
    expect(screen.getByRole("navigation", { name: "一覧", hidden: true })).not.toContainElement(alert);
  });

  it("show the unreachable inbox service once on the list", async () => {
    answer(unreachable);
    at("/");
    await screen.findByText(/受信箱のサービス（switchboard inbox serve）に届きません/);
    expect(screen.getAllByText(/受信箱のサービス（switchboard inbox serve）に届きません/)).toHaveLength(1);
    expect(screen.getByRole("navigation", { name: "一覧" })).not.toContainElement(screen.getByRole("alert"));
  });

  it("show a forge that could not be read outside the list", async () => {
    answer(() => json({ items: [], errors: [{ forge: "GitHub", message: "rate limited" }], fetched_at: "" }));
    at("/issue/gh%3Ao%2Fr%2312");
    const alert = await screen.findByText("GitHubを読めませんでした：rate limited");
    expect(screen.getByRole("navigation", { name: "一覧", hidden: true })).not.toContainElement(alert);
  });

  it("do not show the inbox errors on the settings", async () => {
    const item = { n: 1, ref: "gh:o/r#12", project: "o/r", number: 12, title: "Add a flag", url: "u", host: "laptop", waiting_since: null, reason: "question", question: "" };
    answer(() => json({ items: [item], errors: [{ forge: "GitHub", message: "rate limited" }], fetched_at: "" }));
    at("/settings");
    // The waiting count in the title says the listing has arrived.
    await waitFor(() => expect(document.title).toBe("(1) 設定 - SwitchBoard Inbox"));
    expect(screen.queryByText("GitHubを読めませんでした：rate limited")).toBeNull();
  });
});
