import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "@/App";
import { setLanguageChoice } from "@/lib/i18n";

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

afterEach(() => {
  vi.unstubAllGlobals();
  setLanguageChoice("auto");
});

/** limited is a listing whose GitHub is rate limited until 16:49 in the browser's time zone. */
const limited = () =>
  json({
    items: [],
    errors: [{ forge: "github", message: "API rate limit exceeded", retry_at: new Date(2026, 10, 14, 16, 49).toISOString() }],
    fetched_at: "",
  });

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

  it("show when a rate-limited forge is read again, in the browser's time", async () => {
    answer(limited);
    at("/");
    expect(await screen.findByText("githubのAPI制限中です。16:49に読み直します")).toBeInTheDocument();
    expect(screen.queryByText(/API rate limit exceeded/)).toBeNull();
  });

  it("show the rate limit in English", async () => {
    setLanguageChoice("en");
    answer(limited);
    at("/");
    expect(await screen.findByText("github API rate limit reached. Reading it again at 16:49")).toBeInTheDocument();
  });

  it("show the forge's message for an error that is no rate limit", async () => {
    setLanguageChoice("en");
    answer(() => json({ items: [], errors: [{ forge: "gitlab", message: "connection refused" }], fetched_at: "" }));
    at("/");
    expect(await screen.findByText("Could not read gitlab: connection refused")).toBeInTheDocument();
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
